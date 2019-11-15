package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"os"
	"strconv"
	"strings"
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
	for host, hostInfo := range ingressInfo.HostPathToClusterMap {
		for path, clusterInfo := range hostInfo.PathMap {
			svc, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())

			cps.GetK8sManager().MergeServiceAnnotation(svc, ns, map[string]string{
				kubernetes.IngressAttrLabel(clusterInfo.Port, "name"):   ingressInfo.Name(),
				kubernetes.IngressAttrLabel(clusterInfo.Port, "config"): fmt.Sprintf("%s@%s", path, host),
				kubernetes.IngressAttrLabel(clusterInfo.Port, "secret"): hostInfo.Secret,
			})
		}
	}
}
func (cps *IngressListenersControlPlaneService) IngressDeleted(ingressInfo *kubernetes.IngressInfo) {
	for host, hostInfo := range ingressInfo.HostPathToClusterMap {
		for path, clusterInfo := range hostInfo.PathMap {
			svc, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())

			cps.GetK8sManager().RemoveServiceAnnotation(svc, ns, map[string]string{
				kubernetes.IngressAttrLabel(clusterInfo.Port, "name"):   ingressInfo.Name(),
				kubernetes.IngressAttrLabel(clusterInfo.Port, "config"): fmt.Sprintf("%s@%s", path, host),
				kubernetes.IngressAttrLabel(clusterInfo.Port, "secret"): hostInfo.Secret,
			})
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
	for _, port := range svc.Ports {
		if svc.Annotations[kubernetes.IngressAttrLabel(port.Port, "name")] == "" {
			continue
		}
		configList := svc.Annotations[kubernetes.IngressAttrLabel(port.Port, "config")]
		secret := svc.Annotations[kubernetes.IngressAttrLabel(port.Port, "secret")]
		for _, config := range strings.Split(configList, ",") {
			pathHost := strings.Split(config, "@")
			if len(pathHost) != 2 {
				continue
			}
			info := NewIngressHttpInfo(pathHost[1], pathHost[0], svc.Name(), svc.Namespace(), port.Port)
			info.Secret = secret
			info.Config(svc.Labels)
			cps.UpdateResource(info, svc.ResourceVersion)
		}
	}

}

func (cps *IngressListenersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	for _, port := range svc.Ports {
		name := kubernetes.IngressAttrLabel(port.Port, "name")
		if name == "" {
			continue
		}
		configList := kubernetes.IngressAttrLabel(port.Port, "config")
		for _, config := range strings.Split(configList, ",") {
			pathHost := strings.Split(config, "@")
			if len(pathHost) != 2 {
				continue
			}
			info := NewIngressHttpInfo(pathHost[1], pathHost[0], svc.Name(), svc.Namespace(), port.Port)
			cps.UpdateResource(info, "")
		}
	}
}
func (cps *IngressListenersControlPlaneService) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	cps.ServiceDeleted(oldService)
	cps.ServiceAdded(newService)
}

func (cps *IngressListenersControlPlaneService) createFilters(virtualHosts []route.VirtualHost, pathList []*IngressHttpInfo) []listener.Filter {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "traffic-ingress",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         "traffic-ingress",
				VirtualHosts: virtualHosts,
			},
		},

		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}

	var conflictConfig bool
	for index, info := range pathList {
		if index > 0 {
			if info.Service != pathList[0].Service {
				conflictConfig = true
			} else if info.Namespace != pathList[0].Namespace {
				conflictConfig = true
			}
		}
	}
	if !conflictConfig {
		//multiple ingress config in same connection manager is ignored
		pathList[0].ConfigConnectionManager(manager)
	}
	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny HttpConnectionManager: %s", err.Error())
		panic(err.Error())
	}

	return []listener.Filter{{
		Name:       common.HTTPConnectionManager,
		ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
	}}

}

func (cps *IngressListenersControlPlaneService) createHttpFilterChain(pathList []*IngressHttpInfo) listener.FilterChain {
	var virtualHosts []route.VirtualHost
	var routes []route.Route
	for index, info := range pathList {
		if index > 0 && info.Path == pathList[index-1].Path && info.Host == pathList[index-1].Host {
			//ignore same host and path
			continue
		}

		routes = append(routes, info.CreateRoute())
		if index == len(pathList)-1 || info.Host != pathList[index+1].Host {
			virtualHosts = append(virtualHosts, route.VirtualHost{
				Name:    IngressName(info.Host),
				Domains: []string{info.Host},
				Routes:  routes,
			})
			routes = nil
		}
	}

	return listener.FilterChain{
		Filters: cps.createFilters(virtualHosts, pathList),
	}
}
func (cps *IngressListenersControlPlaneService) createTlsHttpFilterChain(host string, pathList []*IngressHttpInfo) listener.FilterChain {
	var routes []route.Route
	secrets := make(map[string]bool)

	for _, info := range pathList {
		routes = append(routes, info.CreateRoute())
		secrets[info.Secret] = true

	}
	virtualHost := route.VirtualHost{
		Name:    IngressName(host),
		Domains: []string{host},
		Routes:  routes,
	}

	var sdsConfig []*auth.SdsSecretConfig
	for secret, _ := range secrets {
		sdsConfig = append(sdsConfig, &auth.SdsSecretConfig{
			Name: secret,
			SdsConfig: &core.ConfigSource{
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		})
	}
	return listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			ServerNames:       []string{host},
			TransportProtocol: "tls",
		},

		Filters: cps.createFilters([]route.VirtualHost{virtualHost}, pathList),

		TlsContext: &auth.DownstreamTlsContext{
			CommonTlsContext: &auth.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: sdsConfig,
			},
		},
	}
}

func (cps *IngressListenersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {

	pathListWithSecret := make(map[string][]*IngressHttpInfo)
	var pathListWithoutSecret []*IngressHttpInfo

	for _, resource := range resourceMap {
		v := resource.(*IngressHttpInfo)
		if v.Secret != "" && v.Host != "*" {
			pathList := pathListWithSecret[v.Host]
			if pathList == nil {
				pathListWithSecret[v.Host] = []*IngressHttpInfo{v}
			} else {
				pathListWithSecret[v.Host] = append(pathList, v)
			}
		} else {
			pathListWithoutSecret = append(pathListWithoutSecret, v)
		}

	}

	var filterChains []listener.FilterChain

	for host, pathList := range pathListWithSecret {
		SortIngressHttpInfo(pathList)
		filterChains = append(filterChains, cps.createTlsHttpFilterChain(host, pathList))
	}

	if len(pathListWithoutSecret) > 0 {
		SortIngressHttpInfo(pathListWithoutSecret)

		filterChains = append(filterChains, cps.createHttpFilterChain(pathListWithoutSecret))
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
