package listener

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

//listener filter for local pod or outbound listener filter for headless service pod
type HttpPodIpFilterInfo struct {
	PodIpFilterInfo
	HttpListenerConfigInfo
	Domains map[string][]string
}

func NewHttpPodIpFilterInfo(pod *kubernetes.PodInfo, port uint32, headless bool) *HttpPodIpFilterInfo {
	podFilter := NewPodIpFilterInfo(pod, port, headless)
	result := &HttpPodIpFilterInfo{
		PodIpFilterInfo: *podFilter,
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
			cluster := cluster.ServiceClusterName(service, pod.Namespace(), port)
			result.Domains[cluster] = []string{
				fmt.Sprintf("%s:%d", service, port),
				fmt.Sprintf("%s:%d.%s", service, port, pod.Namespace()),
			}
		}
	}
	return result
}

func (info *HttpPodIpFilterInfo) String() string {
	return fmt.Sprintf("%s:%d, tracing=%v", info.podIP, info.port, info.Tracing)
}

func (info *HttpPodIpFilterInfo) CreateVirtualHosts(nodeId string, podCluserName string) []route.VirtualHost {
	var virtualHosts []route.VirtualHost

	if nodeId != info.node {
		//for headless service, should use http Host header to match the target service name so that we can use
		//cluster ip to route the request.
		for cluster, domains := range info.Domains {
			routeAction := &route.RouteAction{
				ClusterSpecifier: &route.RouteAction_Cluster{
					Cluster: cluster,
				},
			}
			info.ConfigRouteAction(routeAction)
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
						Route: routeAction,
					},
				}},
			})
		}
	}
	routeAction := &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: podCluserName,
		},
	}
	//ingress cluster does not need config
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
				Route: routeAction,
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
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         info.Name(),
				VirtualHosts: info.CreateVirtualHosts(node.Id, podCluserName),
			},
		},
	}
	info.ConfigConnectionManager(manager, node.Id == info.node)

	manager.HttpFilters = append(manager.HttpFilters, &hcm.HttpFilter{
		Name: common.RouterHttpFilter,
	})

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
