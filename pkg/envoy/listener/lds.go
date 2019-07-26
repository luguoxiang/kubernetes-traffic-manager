package listener

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"os"
	"reflect"
	"strconv"
)

type ListenerInfo interface {
	common.EnvoyResource
	CreateFilterChain(node *core.Node) (listener.FilterChain, error)
}

type ListenersControlPlaneService struct {
	*common.ControlPlaneService
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
		ControlPlaneService: common.NewControlPlaneService(k8sManager),
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
	for _, port := range svc.Ports {
		protocol := svc.Protocol(port.Port)
		if svc.IsKubeAPIService() {
			info := NewClusterIpFilterInfo(svc, port.Port)
			cps.UpdateResource(info, svc.ResourceVersion)
		} else if protocol == common.PROTO_HTTP {
			info := NewHttpClusterIpFilterInfo(svc, port.Port)
			info.Config(svc.Labels)
			cps.UpdateResource(info, svc.ResourceVersion)
		} else if protocol != "" {
			info := NewClusterIpFilterInfo(svc, port.Port)
			cps.UpdateResource(info, svc.ResourceVersion)
		}
	}
}
func (cps *ListenersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	for _, port := range svc.Ports {
		var info common.EnvoyResource
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
	cps.PodUpdated(nil, pod)
}
func (cps *ListenersControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	cps.PodUpdated(pod, nil)
}

func (cps *ListenersControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	visited := make(map[string]bool)

	if newPod != nil {
		for port, portInfo := range newPod.GetPortConfig() {
			configMap := portInfo.ConfigMap
			headless := (configMap != nil && kubernetes.GetLabelValueBool(configMap[kubernetes.HEADLESS]))
			if portInfo.Protocol == common.PROTO_HTTP {
				info := NewHttpPodIpFilterInfo(newPod, port, headless)
				info.Config(portInfo.ConfigMap)
				visited[info.Name()] = true
				cps.UpdateResource(info, newPod.ResourceVersion)
			} else {
				info := NewPodIpFilterInfo(newPod, port, headless)
				visited[info.Name()] = true
				cps.UpdateResource(info, newPod.ResourceVersion)
			}

		}
	}
	if oldPod != nil {
		for port, _ := range oldPod.GetPortSet() {
			info := NewPodIpFilterInfo(oldPod, port, true)
			if visited[info.Name()] {
				continue
			}
			cps.UpdateResource(info, "")
		}
	}
}

func (cps *ListenersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {
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
				Name: common.ORIGINAL_DST,
			},
		},
	}
	return common.MakeResource([]proto.Message{l}, common.ListenerResource, version)
}
