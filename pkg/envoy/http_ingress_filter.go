package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	accesslog_filter "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"

	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type HttpPodIngressFilterInfo struct {
	PodIngressFilterInfo
	IngressTracing bool
	Domains        map[string][]string
}

func NewHttpPodIngressFilterInfo(pod *kubernetes.PodInfo, port uint32, headless bool) ListenerInfo {
	inBoundInfo := NewPodIngressFilterInfo(pod, port, headless)
	result := &HttpPodIngressFilterInfo{
		PodIngressFilterInfo: *inBoundInfo,
		IngressTracing:       true,
	}
	if !headless {
		for k, v := range pod.Labels {
			switch k {
			case "traffic.envoy.tracing.ingress":
				result.IngressTracing = kubernetes.GetLabelValueBool(v)
			}
		}
	}
	for key, _ := range pod.Annotations {
		service, svcPort := kubernetes.GetServiceAndPort(key)
		if svcPort != port {
			continue
		}
		if service != "" {
			if result.Domains == nil {
				result.Domains = make(map[string][]string)
			}
			cluster := common.OutboundClusterName(service, pod.Namespace(), port)
			result.Domains[cluster] = []string{
				fmt.Sprintf("%s:%d", service, port),
				fmt.Sprintf("%s:%d.%s", service, port, pod.Namespace()),
			}
		}
	}
	return result
}

func (info *HttpPodIngressFilterInfo) String() string {
	return fmt.Sprintf("%s:%d, tracing=%v", info.podIP, info.port, info.IngressTracing)
}

func (info *HttpPodIngressFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	defaultClusterName := info.getClusterName(node.Id)
	if defaultClusterName == "" {
		return listener.FilterChain{}, nil
	}

	var virtualHosts []route.VirtualHost
	if node.Id != info.node {
		for cluster, domains := range info.Domains {
			virtualHosts = append(virtualHosts, route.VirtualHost{
				Name:    fmt.Sprintf("%s_vh", cluster),
				Domains: domains,
				Routes: []route.Route{{
					Match: route.RouteMatch{
						PathSpecifier: &route.RouteMatch_Prefix{
							Prefix: "/",
						},
					},
					Action: &route.Route_Route{
						Route: &route.RouteAction{
							ClusterSpecifier: &route.RouteAction_Cluster{
								Cluster: cluster,
							},
						},
					},
				}},
			})
		}
	}
	virtualHosts = append(virtualHosts, route.VirtualHost{
		Name:    fmt.Sprintf("%s_vh", defaultClusterName),
		Domains: []string{"*"},
		Routes: []route.Route{{
			Match: route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: defaultClusterName,
					},
				},
			},
		}},
	})

	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: info.Name(),
		AccessLog: []*accesslog_filter.AccessLog{
			&accesslog_filter.AccessLog{
				Name: "envoy.file_access_log",
				ConfigType: &accesslog_filter.AccessLog_TypedConfig{
					TypedConfig: common.CreateAccessLogAny(true),
				},
			},
		},
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         info.Name(),
				VirtualHosts: virtualHosts,
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}
	if info.IngressTracing {
		manager.Tracing = &hcm.HttpConnectionManager_Tracing{
			OperationName: hcm.INGRESS,
		}
	}
	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		return listener.FilterChain{}, err
	}
	return listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			DestinationPort: &types.UInt32Value{Value: info.port},
			PrefixRanges: []*core.CidrRange{&core.CidrRange{
				AddressPrefix: info.podIP,
				PrefixLen:     &types.UInt32Value{Value: 32},
			},
			},
		},
		Filters: []listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}, nil

}
