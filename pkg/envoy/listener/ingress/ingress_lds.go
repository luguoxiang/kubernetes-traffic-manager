package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"os"
	"strconv"
	"strings"
)

const (
	INGRESS_PORT_ANNOTATION = "traffic.ingress.port."
)

type IngressListenerInfo interface {
	common.EnvoyResource
}

type IngressListenersControlPlaneService struct {
	*common.ControlPlaneService
	proxyPort uint32
}

func NewIngressListenersControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *IngressListenersControlPlaneService {

	proxyPortStr := os.Getenv("ENVOY_PROXY_PORT")
	if proxyPortStr == "" {
		panic("env ENVOY_PROXY_PORT is not set")
	}

	proxyPort, err := strconv.ParseInt(proxyPortStr, 10, 32)
	if err != nil {
		panic("wrong ENVOY_PROXY_PORT value:" + err.Error())
	}
	result := &IngressListenersControlPlaneService{
		ControlPlaneService: common.NewControlPlaneService(k8sManager),
		proxyPort:           uint32(proxyPort),
	}

	return result
}

func (cps *IngressListenersControlPlaneService) IngressValid(ingressInfo *kubernetes.IngressInfo) bool {
	return true
}

func getNameAndNamespace(svc string, ns string) (string, string) {

	tokens := strings.Split(svc, ".")
	if len(tokens) > 1 {
		svc = tokens[0]
		ns = tokens[1]
	}
	return svc, ns
}

func (cps *IngressListenersControlPlaneService) IngressAdded(ingressInfo *kubernetes.IngressInfo) {
	for host, pathMap := range ingressInfo.HostPathToClusterMap {
		for path, clusterInfo := range pathMap {
			svc, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())
			if path == "" {
				path = "/"
			}
			if host == "*" || host == "" {
				cps.GetK8sManager().MergeServiceAnnotation(svc, ns, fmt.Sprintf("%s%d", INGRESS_PORT_ANNOTATION, clusterInfo.Port), path)
			} else {
				cps.GetK8sManager().MergeServiceAnnotation(svc, ns, fmt.Sprintf("%s%d.host.%s", INGRESS_PORT_ANNOTATION, clusterInfo.Port, host), path)
			}
		}
	}
}
func (cps *IngressListenersControlPlaneService) IngressDeleted(ingressInfo *kubernetes.IngressInfo) {
	for host, pathMap := range ingressInfo.HostPathToClusterMap {
		for path, clusterInfo := range pathMap {
			svc, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())
			if path == "" {
				path = "/"
			}
			if host == "*" || host == "" {
				cps.GetK8sManager().RemoveServiceAnnotation(svc, ns, fmt.Sprintf("%s%d", INGRESS_PORT_ANNOTATION, clusterInfo.Port), path)
			} else {
				cps.GetK8sManager().RemoveServiceAnnotation(svc, ns, fmt.Sprintf("%s%d.host.%s", INGRESS_PORT_ANNOTATION, clusterInfo.Port, host), path)
			}
		}
	}
}
func (cps *IngressListenersControlPlaneService) IngressUpdated(oldIngress, newIngress *kubernetes.IngressInfo) {
	cps.IngressDeleted(oldIngress)
	cps.IngressAdded(newIngress)
}

func (cps *IngressListenersControlPlaneService) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return true
}

func (cps *IngressListenersControlPlaneService) ServiceAdded(svc *kubernetes.ServiceInfo) {
	for k, v := range svc.Annotations {
		if !strings.HasPrefix(k, INGRESS_PORT_ANNOTATION) {
			continue
		}
		k = k[len(INGRESS_PORT_ANNOTATION):]
		tokens := strings.Split(k, ".")
		port := kubernetes.GetLabelValueUInt32(tokens[0])
		var host string
		if len(tokens) == 1 {
			host = "*"
		} else {
			host = strings.Join(tokens[2:], ".")
		}
		for _, path := range strings.Split(v, ",") {
			if path == "" {
				continue
			}
			info := NewIngressHttpInfo(host, path, cluster.ServiceClusterName(svc.Name(), svc.Namespace(), port))
			info.Config(svc.Labels)
			cps.UpdateResource(info, svc.ResourceVersion)
		}
	}
}

func (cps *IngressListenersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	for k, v := range svc.Annotations {
		if !strings.HasPrefix(k, INGRESS_PORT_ANNOTATION) {
			continue
		}
		k = k[len(INGRESS_PORT_ANNOTATION):]
		tokens := strings.Split(k, ".")
		port := kubernetes.GetLabelValueUInt32(tokens[0])
		var host string
		if len(tokens) == 1 {
			host = "*"
		} else {
			host = strings.Join(tokens[2:], ".")
		}
		for _, path := range strings.Split(v, ",") {
			if path == "" {
				continue
			}
			info := NewIngressHttpInfo(host, path, cluster.ServiceClusterName(svc.Name(), svc.Namespace(), port))
			cps.UpdateResource(info, "")
		}
	}
}
func (cps *IngressListenersControlPlaneService) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	cps.ServiceDeleted(oldService)
	cps.ServiceAdded(newService)
}

func (cps *IngressListenersControlPlaneService) CreateHttpFilterChain(virtualHosts []route.VirtualHost) listener.FilterChain {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "traffic-ingress",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         "traffic-ingress",
				VirtualHosts: virtualHosts,
			},
		},

		Tracing: &hcm.HttpConnectionManager_Tracing{
			OperationName: hcm.EGRESS,
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}
	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny HttpConnectionManager: %s", err.Error())
		panic(err.Error())
	}

	return listener.FilterChain{
		Filters: []listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}
}
func (cps *IngressListenersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {

	var virtualHosts []route.VirtualHost

	infoMap := make(map[string]map[string]*IngressHttpInfo)
	for _, resource := range resourceMap {
		v := resource.(*IngressHttpInfo)

		pathMap := infoMap[v.Host]
		if pathMap == nil {
			infoMap[v.Host] = map[string]*IngressHttpInfo{
				v.Path: v,
			}
		} else {
			pathMap[v.Path] = v
		}

	}

	for host, pathMap := range infoMap {
		var name string
		if host == "*" {
			name = "all_ingress_vh"
		} else {
			name = fmt.Sprintf("%s_ingress_vh", strings.Replace(host, ".", "_", -1))
		}
		var routes []route.Route

		for pathPrefix, info := range pathMap {
			routeAction := info.CreateRouteAction(info.Cluster)
			routes = append(routes, route.Route{
				Match: route.RouteMatch{
					PathSpecifier: &route.RouteMatch_Prefix{
						Prefix: pathPrefix,
					},
				},
				Action: &route.Route_Route{
					Route: routeAction,
				},
			})
		}

		virtualHosts = append(virtualHosts, route.VirtualHost{
			Name:    name,
			Domains: []string{host},
			Routes:  routes,
		})
	}
	var filterChains []listener.FilterChain

	if len(virtualHosts) > 0 {
		filterChains = append(filterChains, cps.CreateHttpFilterChain(virtualHosts))
	}

	l := &v2.Listener{
		Name: "ingress_listener",
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: cps.proxyPort,
					},
				},
			},
		},

		FilterChains: filterChains,
	}
	return common.MakeResource([]proto.Message{l}, common.ListenerResource, version)
}
