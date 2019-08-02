package endpoint

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/proto"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type EndpointsControlPlaneService struct {
	*common.ControlPlaneService
}

func NewEndpointsControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *EndpointsControlPlaneService {
	return &EndpointsControlPlaneService{
		ControlPlaneService: common.NewControlPlaneService(k8sManager),
	}
}

func (manager *EndpointsControlPlaneService) PodValid(pod *kubernetes.PodInfo) bool {
	return pod.Valid()
}

func (cps *EndpointsControlPlaneService) addClusterAssignment(pod *kubernetes.PodInfo, clusterAssignment *ClusterAssignmentInfo) {

	envoyResource, _ := cps.GetResourceClone(clusterAssignment.Name())

	if envoyResource != nil {
		clusterAssignment = envoyResource.(*ClusterAssignmentInfo)
	} else if clusterAssignment.EndpointMap == nil {
		clusterAssignment.EndpointMap = make(map[string]*EndpointInfo)
	}

	endpoint := &EndpointInfo{
		PodIP:   pod.PodIP,
		Version: pod.ResourceVersion,
	}
	endpoint.Config(pod)

	key := fmt.Sprintf("%s@%s", pod.Name(), pod.Namespace())
	clusterAssignment.EndpointMap[key] = endpoint

	cps.UpdateResource(clusterAssignment, clusterAssignment.Version())
}

func (cps *EndpointsControlPlaneService) removeClusterAssignment(pod *kubernetes.PodInfo, clusterName string) {
	envoyResource, _ := cps.GetResourceClone(clusterName)
	if envoyResource != nil {
		clusterAssignment := envoyResource.(*ClusterAssignmentInfo)

		key := fmt.Sprintf("%s@%s", pod.Name(), pod.Namespace())
		if clusterAssignment.EndpointMap[key] != nil {
			delete(clusterAssignment.EndpointMap, key)
			cps.UpdateResource(clusterAssignment, clusterAssignment.Version())
		}
	}
}

func (cps *EndpointsControlPlaneService) PodAdded(pod *kubernetes.PodInfo) {
	cps.PodUpdated(nil, pod)

}
func (cps *EndpointsControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	cps.PodUpdated(pod, nil)

}
func (cps *EndpointsControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	visited := make(map[string]bool)
	if newPod != nil {
		for port, serviceMap := range newPod.GetPortSet() {
			for service, _ := range serviceMap {
				clusterAssigment := NewClusterAssignmentInfo(service, newPod.Namespace(), port)
				visited[clusterAssigment.Name()] = true
				cps.addClusterAssignment(newPod, clusterAssigment)
			}
		}
	}

	if oldPod != nil {
		for port, serviceMap := range oldPod.GetPortSet() {
			for service, _ := range serviceMap {
				clusterName := cluster.ServiceClusterName(service, oldPod.Namespace(), port)
				if !visited[clusterName] {
					cps.removeClusterAssignment(oldPod, clusterName)
				}
			}
		}
	}

}

func (cps *EndpointsControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {

	var claList []proto.Message
	for _, resource := range resourceMap {
		assignmentInfo := resource.(*ClusterAssignmentInfo)

		var lbEndpoints []endpoint.LbEndpoint
		for _, endpointInfo := range assignmentInfo.EndpointMap {
			lbEndpoint := endpointInfo.CreateLoadBalanceEndpoint(assignmentInfo.Port)
			if lbEndpoint == nil {
				continue
			}

			lbEndpoints = append(lbEndpoints, *lbEndpoint)
		}

		cla := &v2.ClusterLoadAssignment{
			ClusterName: assignmentInfo.Name(),
			Endpoints: []endpoint.LocalityLbEndpoints{{
				LbEndpoints: lbEndpoints,
			}},
		}
		claList = append(claList, cla)
	}

	return common.MakeResource(claList, common.EndpointResource, version)
}
