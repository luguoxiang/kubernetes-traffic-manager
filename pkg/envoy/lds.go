package envoy

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"os"
	"reflect"
	"strconv"
)

type ListenerInfo interface {
	EnvoyResource
	CreateFilterChain(node *core.Node) (listener.FilterChain, error)
}

type ListenersControlPlaneService struct {
	*ControlPlaneService
	proxyPort uint32
}

func NewListenersControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *ListenersControlPlaneService {

	proxyPortStr := os.Getenv("ENVOY_PROXY_PORT")
	if proxyPortStr == "" {
		panic("env ENVOY_PROXY_PORT is not set")
	}

	proxyPort, err := strconv.ParseInt(proxyPortStr, 10, 32)
	if err != nil {
		panic("wrong ENVOY_PROXY_PORT value:" + err.Error())
	}
	result := &ListenersControlPlaneService{
		ControlPlaneService: NewControlPlaneService(k8sManager),
		proxyPort:           uint32(proxyPort),
	}
	k8sManager.Lock()
	defer k8sManager.Unlock()
	result.UpdateResource(&BlackHoleFilterInfo{}, "1")
	return result
}

func (cps *ListenersControlPlaneService) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return svc.OutboundEnabled()
}

func (cps *ListenersControlPlaneService) ServiceAdded(svc *kubernetes.ServiceInfo) {
	var info EnvoyResource
	for _, port := range svc.Ports {
		protocol := svc.Protocol(port.Port)
		if svc.IsKubeAPIService() {
			info = NewClusterIpFilterInfo(svc, port.Port)
		} else if protocol == CLUSTER_PROTO_HTTP {
			info = NewHttpClusterIpFilterInfo(svc, port.Port)
		} else if protocol != "" {
			info = NewClusterIpFilterInfo(svc, port.Port)
		} else {
			continue
		}
		cps.UpdateResource(info, svc.ResourceVersion)
	}
}
func (cps *ListenersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	var info EnvoyResource
	for _, port := range svc.Ports {
		protocol := svc.Protocol(port.Port)
		if protocol != "" {
			info = NewClusterIpFilterInfo(svc, port.Port)
			cps.UpdateResource(info, "")
		}

	}
}
func (cps *ListenersControlPlaneService) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	if !reflect.DeepEqual(oldService.Ports, newService.Ports) {
		cps.ServiceDeleted(oldService)
		cps.ServiceAdded(newService)
	} else {
		cps.ServiceAdded(newService)
	}
}

func (manager *ListenersControlPlaneService) PodValid(pod *kubernetes.PodInfo) bool {
	//Hostnetwork pod should not have envoy enabled, so no inbound listener
	return !pod.HostNetwork && pod.PodIP != "" && (pod.EnvoyEnabled() || pod.HasHeadlessService())
}

func (cps *ListenersControlPlaneService) PodAdded(pod *kubernetes.PodInfo) {
	var info EnvoyResource
	for port, portInfo := range pod.GetPortMap() {
		if portInfo.Protocol == CLUSTER_PROTO_HTTP {
			info = NewHttpPodIpFilterInfo(pod, port, portInfo.Headless, portInfo.Tracing)
		} else {
			info = NewPodIpFilterInfo(pod, port, portInfo.Headless)
		}
		cps.UpdateResource(info, pod.ResourceVersion)
	}

}
func (cps *ListenersControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	var info EnvoyResource
	for port, portInfo := range pod.GetPortMap() {
		info = NewPodIpFilterInfo(pod, port, portInfo.Headless)
		cps.UpdateResource(info, "")
	}
}

func (cps *ListenersControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	visited := make(map[string]bool)

	var info EnvoyResource
	for port, portInfo := range newPod.GetPortMap() {
		if portInfo.Protocol == "http" {
			info = NewHttpPodIpFilterInfo(newPod, port, portInfo.Headless, portInfo.Tracing)
		} else {
			info = NewPodIpFilterInfo(newPod, port, portInfo.Headless)
		}
		visited[info.Name()] = true
		cps.UpdateResource(info, newPod.ResourceVersion)
	}

	for port, portInfo := range oldPod.GetPortMap() {
		info = NewPodIpFilterInfo(oldPod, port, portInfo.Headless)
		if visited[info.Name()] {
			continue
		}
		cps.UpdateResource(info, "")
	}
}

func (cps *ListenersControlPlaneService) BuildResource(resourceMap map[string]EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {
	var filterChains []listener.FilterChain

	for _, resource := range resourceMap {
		listenerInfo := resource.(ListenerInfo)

		fc, err := listenerInfo.CreateFilterChain(node)
		if err != nil {
			return nil, err
		}
		if fc.Filters == nil {
			continue
		}
		filterChains = append(filterChains, fc)
		if glog.V(2) {
			glog.Infof("FilterChainMatch %v = %s", fc.FilterChainMatch, listenerInfo.Name())
		}
	}

	l := &v2.Listener{
		Name: "mylistener",
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
		ListenerFilters: []listener.ListenerFilter{
			listener.ListenerFilter{
				Name: ORIGINAL_DST,
			},
		},
	}
	return MakeResource([]proto.Message{l}, ListenerResource, version)
}
