package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/gogo/protobuf/proto"
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
	proxyPort  uint32
	ingressMap map[string]*kubernetes.IngressInfo
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
		ingressMap:          make(map[string]*kubernetes.IngressInfo),
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
	cps.ingressMap[fmt.Sprintf("%s.%s",ingressInfo.Name(), ingressInfo.Namespace())] = ingressInfo
	for _, hostInfo := range ingressInfo.HostPathToClusterMap {
		for _, clusterInfo := range hostInfo.PathMap {
			svc, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())

			cps.GetK8sManager().MergeServiceAnnotation(svc, ns, ingressInfo.GetServiceAnnotations(hostInfo, clusterInfo))
		}
	}
}
func (cps *IngressListenersControlPlaneService) IngressDeleted(ingressInfo *kubernetes.IngressInfo) {
	delete(cps.ingressMap, fmt.Sprintf("%s.%s",ingressInfo.Name(), ingressInfo.Namespace()))
	for _, hostInfo := range ingressInfo.HostPathToClusterMap {
		for _, clusterInfo := range hostInfo.PathMap {
			svc, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())

			cps.GetK8sManager().RemoveServiceAnnotation(svc, ns, ingressInfo.GetServiceAnnotations(hostInfo, clusterInfo))
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
	for _, ingressInfo := range cps.ingressMap {
		for _, hostInfo := range ingressInfo.HostPathToClusterMap {
			for _, clusterInfo := range hostInfo.PathMap {
				name, ns := getNameAndNamespace(clusterInfo.Service, ingressInfo.Namespace())
				if name == svc.Name() && ns ==svc.Namespace() {
					cps.GetK8sManager().MergeServiceAnnotation(name, ns, ingressInfo.GetServiceAnnotations(hostInfo, clusterInfo))
				}
			}
		}
	}
	for _, port := range svc.Ports {
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
		configList := svc.Annotations[kubernetes.IngressAttrLabel(port.Port, "config")]
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

func (cps *IngressListenersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*envoy_api_v2.DiscoveryResponse, error) {

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

	var filterChains []*listener.FilterChain

	for host, pathList := range pathListWithSecret {
		SortIngressHttpInfo(pathList)
		filterChains = append(filterChains, CreateTlsHttpFilterChain(host, pathList))
	}

	if len(pathListWithoutSecret) > 0 {
		SortIngressHttpInfo(pathListWithoutSecret)

		filterChains = append(filterChains, CreateHttpFilterChain(pathListWithoutSecret))
	}

	l := &envoy_api_v2.Listener{
		Name: "ingress_listener",
		Address: &core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.SocketAddress_TCP,
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
