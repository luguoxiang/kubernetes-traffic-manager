package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"strings"
)

type IngressHttpInfo struct {
	Host           string
	PathClusterMap map[string]string
}

func NewIngressHttpInfo(host string) *IngressHttpInfo {
	return &IngressHttpInfo{
		Host:           host,
		PathClusterMap: make(map[string]string),
	}
}

func (info *IngressHttpInfo) Name() string {
	if info.Host == "*" {
		return "ingress-http|all"
	}
	return fmt.Sprintf("http|%s", info.Host)
}

func (info *IngressHttpInfo) Type() string {
	return common.ListenerResource
}

func (info *IngressHttpInfo) String() string {
	return info.Name()
}

func (info *IngressHttpInfo) CreateVirtualHost() route.VirtualHost {
	var name string
	if info.Host == "*" {
		name = "all_ingress_vh"
	} else {
		name = fmt.Sprintf("%s_ingress_vh", strings.Replace(info.Host, ".", "_", -1))
	}
	var routes []route.Route

	for pathPrefix, cluster := range info.PathClusterMap {
		route := route.Route{
			Match: route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: pathPrefix,
				},
			},
			Action: &route.Route_Route{
				Route: &route.RouteAction{
					ClusterSpecifier: &route.RouteAction_Cluster{
						Cluster: cluster,
					},
				},
			},
		}

		routes = append(routes, route)
	}
	return route.VirtualHost{
		Name:    name,
		Domains: []string{info.Host},
		Routes:  routes,
	}
}
