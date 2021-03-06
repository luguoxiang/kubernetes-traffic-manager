package ingress

import (
	"fmt"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/listener"
	"sort"
	"strings"
)

type IngressHttpInfo struct {
	listener.HttpListenerConfigInfo
	Host      string
	Path      string
	Service   string
	Namespace string
	Port      uint32
	Secret    string
}

func NewIngressHttpInfo(host string, path string, svc string, ns string, port uint32) *IngressHttpInfo {
	return &IngressHttpInfo{
		Host:      host,
		Path:      path,
		Service:   svc,
		Namespace: ns,
		Port:      port,
	}
}

func IngressName(host string) string {
	if host == "*" {
		return "all_ingress_vh"
	} else {
		return fmt.Sprintf("%s_ingress_vh", strings.Replace(host, ".", "_", -1))
	}
}

func (info *IngressHttpInfo) GetCluster() string {
	return cluster.ServiceClusterName(info.Service, info.Namespace, info.Port)
}

func (info *IngressHttpInfo) Name() string {
	if info.Host == "*" {
		fmt.Sprintf("http|all|%s", info.Path)
	}
	return fmt.Sprintf("http|%s|%s", info.Host, info.Path)
}

func (info *IngressHttpInfo) Type() string {
	return common.ListenerResource
}

func (info *IngressHttpInfo) String() string {
	return info.Name()
}

func (info *IngressHttpInfo) CreateRoute() *route.Route {
	routeAction := info.CreateRouteAction(info.GetCluster())
	return &route.Route{
		Match: &route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: info.Path,
			},
		},
		Action: &route.Route_Route{
			Route: routeAction,
		},
	}
}
func SortIngressHttpInfo(pathList []*IngressHttpInfo) {
	sort.SliceStable(pathList, func(i, j int) bool {
		a := pathList[i]
		b := pathList[j]

		if a.Host != b.Host {
			// * should be last
			if a.Host == "*" {
				return false
			}
			if b.Host == "*" {
				return true
			}

			return a.Host > b.Host
		}
		return a.Path > b.Path
	})
}
