package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"reflect"
	"sort"
	"strings"
)

type AssignmentInfo struct {
	PodIP   string
	Weight  uint32
	Version string
}

func (info AssignmentInfo) String() string {
	return fmt.Sprintf("%s|%d", info.PodIP, info.Weight)
}

type EndpointInfo struct {
	Service     string
	Namespace   string
	Port        uint32
	Assignments map[string]*AssignmentInfo
}

func NewEndpointInfo(svc string, ns string, port uint32) *EndpointInfo {
	return &EndpointInfo{
		Service:     svc,
		Namespace:   ns,
		Port:        port,
		Assignments: make(map[string]*AssignmentInfo),
	}
}

func EndpointName(svc string, ns string, port uint32) string {
	return OutboundClusterName(svc, ns, port)
}

func (info *EndpointInfo) String() string {
	var ss []string
	for _, ai := range info.Assignments {
		ss = append(ss, ai.String())
	}
	return fmt.Sprintf("%s.%s:%d[%s]", info.Service, info.Namespace, info.Port, strings.Join(ss, ","))
}

func (info *EndpointInfo) Name() string {
	return OutboundClusterName(info.Service, info.Namespace, info.Port)
}

func (info *EndpointInfo) Type() string {
	return EndpointResource
}

func (info *EndpointInfo) Clone() EnvoyResourceClonable {
	result := &EndpointInfo{
		Service:     info.Service,
		Namespace:   info.Namespace,
		Port:        info.Port,
		Assignments: make(map[string]*AssignmentInfo),
	}
	for k, v := range info.Assignments {
		result.Assignments[k] = v
	}
	return result
}
func (info *EndpointInfo) Version() string {
	var result []string
	for _, assignment := range info.Assignments {
		result = append(result, assignment.Version)
	}
	if len(result) == 0 {
		return "0"
	}
	sort.Strings(result)
	return strings.Join(result, "-")
}

type EndpointsControlPlaneService struct {
	*ControlPlaneService
}

func NewEndpointsControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *EndpointsControlPlaneService {
	return &EndpointsControlPlaneService{
		ControlPlaneService: NewControlPlaneService(k8sManager),
	}
}

func (manager *EndpointsControlPlaneService) PodValid(pod *kubernetes.PodInfo) bool {
	return true
}

func (cps *EndpointsControlPlaneService) addEndpoint(pod *kubernetes.PodInfo, service string, port uint32) {
	name := EndpointName(service, pod.Namespace(), port)
	envoyResource, _ := cps.GetResourceClone(name)

	var endpoint *EndpointInfo
	if envoyResource != nil {
		endpoint = envoyResource.(*EndpointInfo)
	} else {
		endpoint = NewEndpointInfo(service, pod.Namespace(), port)
	}
	weight := pod.Weight()

	key := fmt.Sprintf("%s@%s", pod.Name(), pod.Namespace())
	endpoint.Assignments[key] = &AssignmentInfo{
		PodIP:   pod.PodIP,
		Weight:  weight,
		Version: pod.ResourceVersion,
	}
	cps.UpdateResource(endpoint, endpoint.Version())
}

func (cps *EndpointsControlPlaneService) removeEndpoint(pod *kubernetes.PodInfo, service string, port uint32) {
	name := EndpointName(service, pod.Namespace(), port)
	envoyResource, _ := cps.GetResourceClone(name)
	if envoyResource != nil {
		endpoint := envoyResource.(*EndpointInfo)
		key := fmt.Sprintf("%s@%s", pod.Name(), pod.Namespace())
		if endpoint.Assignments[key] != nil {
			delete(endpoint.Assignments, key)
			cps.UpdateResource(endpoint, endpoint.Version())
		}
	}
}
func (cps *EndpointsControlPlaneService) PodAdded(pod *kubernetes.PodInfo) {
	for key, _ := range pod.Annotations {
		service, port := kubernetes.GetServiceAndPort(key)
		if service != "" && port != 0 {
			cps.addEndpoint(pod, service, port)
		}
	}

}
func (cps *EndpointsControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	for key, _ := range pod.Annotations {
		service, port := kubernetes.GetServiceAndPort(key)
		if service != "" && port != 0 {
			cps.removeEndpoint(pod, service, port)
		}
	}

}
func (cps *EndpointsControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	if reflect.DeepEqual(oldPod.Annotations, newPod.Annotations) {
		return
	}
	for key, _ := range oldPod.Annotations {
		if newPod.Annotations[key] == "" {
			service, port := kubernetes.GetServiceAndPort(key)
			if service != "" && port != 0 {
				cps.removeEndpoint(oldPod, service, port)
			}
		}
	}
	for key, _ := range newPod.Annotations {
		if oldPod.Annotations[key] == "" {
			service, port := kubernetes.GetServiceAndPort(key)
			if service != "" && port != 0 {
				cps.addEndpoint(newPod, service, port)
			}
		}
	}
}

func (cps *EndpointsControlPlaneService) BuildResource(resourceMap map[string]EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {

	var claList []proto.Message
	for _, resource := range resourceMap {
		endpointInfo := resource.(*EndpointInfo)
		cla := &v2.ClusterLoadAssignment{
			ClusterName: endpointInfo.Name(),
			Endpoints: []endpoint.LocalityLbEndpoints{{
				LbEndpoints: []endpoint.LbEndpoint{},
			}},
		}

		var lbEndpoints []endpoint.LbEndpoint
		for _, assignment := range endpointInfo.Assignments {
			if assignment.Weight == 0 {
				continue
			}
			lbEndpoint := endpoint.LbEndpoint{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.TCP,
									Address:  assignment.PodIP,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: endpointInfo.Port,
									},
								},
							},
						},
					},
				},
				LoadBalancingWeight: &types.UInt32Value{
					Value: assignment.Weight,
				},
			}

			lbEndpoints = append(lbEndpoints, lbEndpoint)
		}

		cla.Endpoints[0].LbEndpoints = lbEndpoints
		claList = append(claList, cla)
	}

	return MakeResource(claList, EndpointResource, version)
}
