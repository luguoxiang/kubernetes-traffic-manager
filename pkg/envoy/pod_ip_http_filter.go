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

//listener filter for local pod or outbound listener filter for headless service pod
type HttpPodIpFilterInfo struct {
	PodIpFilterInfo
	IngressTracing bool
	Domains        map[string][]string
}

func NewHttpPodIpFilterInfo(pod *kubernetes.PodInfo, port uint32, headless bool) ListenerInfo {
	podFilter := NewPodIpFilterInfo(pod, port, headless)
	result := &HttpPodIpFilterInfo{
		PodIpFilterInfo: *podFilter,
		IngressTracing:  true,
	}
	for k, v := range pod.Labels {
		switch k {
		case "traffic.envoy.tracing.ingress":
			result.IngressTracing = kubernetes.GetLabelValueBool(v)
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

func (info *HttpPodIpFilterInfo) String() string {
	return fmt.Sprintf("%s:%d, tracing=%v", info.podIP, info.port, info.IngressTracing)
}

func (info *HttpPodIpFilterInfo) CreateVirtualHosts(nodeId string, podCluserName string) []route.VirtualHost {
	var virtualHosts []route.VirtualHost
	if nodeId != info.node {
		//for headless service, should use http Host header to match the target service name so that we can use
		//cluster ip to route the request.
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
		Name:    fmt.Sprintf("%s_vh", podCluserName),
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
						Cluster: podCluserName,
					},
				},
			},
		}},
	})
	return virtualHosts
}

func (info *HttpPodIpFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	podCluserName := info.getClusterName(node.Id)
	if podCluserName == "" {
		return listener.FilterChain{}, nil
	}

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
				VirtualHosts: info.CreateVirtualHosts(node.Id, podCluserName),
			},
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}
	//headless outbound acess will be traced at target headless service pod
	if info.IngressTracing && node.Id == info.node {
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
