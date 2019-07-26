package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"reflect"
)

type ClusterInfo interface {
	EnvoyResource
	CreateCluster(nodeId string) *v2.Cluster
}

type ClustersControlPlaneService struct {
	*ControlPlaneService
}

func NewClustersControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *ClustersControlPlaneService {
	return &ClustersControlPlaneService{ControlPlaneService: NewControlPlaneService(k8sManager)}
}

func (cps *ClustersControlPlaneService) BuildResource(resourceMap map[string]EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {
	var clusters []proto.Message

	for _, resource := range resourceMap {
		clusterInfo := resource.(ClusterInfo)
		serviceCluster := clusterInfo.CreateCluster(node.Id)
		if serviceCluster.ConnectTimeout == 0 {
			panic(fmt.Sprintf("cluster %s connect timeout should not be zero", serviceCluster.Name))
		}
		clusters = append(clusters, serviceCluster)
	}

	return MakeResource(clusters, ClusterResource, version)
}

func (cps *ClustersControlPlaneService) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return svc.OutboundEnabled()
}

func (cps *ClustersControlPlaneService) ServiceAdded(svc *kubernetes.ServiceInfo) {
	for _, port := range svc.Ports {
		protocol := svc.Protocol(port.Port)
		if protocol == CLUSTER_PROTO_DIRECT {
			cluster := NewByPassClusterInfo(svc, port.Port)
			cluster.ConfigCluster(svc.Labels)
			cps.UpdateResource(cluster, svc.ResourceVersion)
		} else if protocol != "" {
			cluster := NewOutboundClusterInfo(svc, port.Port)
			cluster.ConfigCluster(svc.Labels)
			cps.UpdateResource(cluster, svc.ResourceVersion)
		}

	}

}
func (cps *ClustersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	for _, port := range svc.Ports {
		cluster := NewOutboundClusterInfo(svc, port.Port)
		cps.UpdateResource(cluster, "")
	}

}
func (cps *ClustersControlPlaneService) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	if !reflect.DeepEqual(oldService.Ports, newService.Ports) {
		cps.ServiceDeleted(oldService)
		cps.ServiceAdded(newService)
	} else {
		cps.ServiceAdded(newService)
	}
}

func (cps *ClustersControlPlaneService) PodValid(pod *kubernetes.PodInfo) bool {
	//Hostnetwork pod should not have envoy enabled, so there will be no inbound cluster for it
	return !pod.HostNetwork && pod.PodIP != ""
}

func (cps *ClustersControlPlaneService) PodAdded(pod *kubernetes.PodInfo) {
	for port, _ := range pod.GetPortMap() {
		cluster := NewStaticLocalClusterInfo(port)
		cps.UpdateResource(cluster, "1")

		cluster = NewStaticClusterInfo(pod.PodIP, port, pod.NodeId())
		cluster.ConfigCluster(pod.Annotations)
		cps.UpdateResource(cluster, pod.ResourceVersion)
	}
}
func (cps *ClustersControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	for port, _ := range pod.GetPortMap() {
		cluster := NewStaticClusterInfo(pod.PodIP, port, pod.NodeId())
		cps.UpdateResource(cluster, "")
	}
}
func (cps *ClustersControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	cps.PodAdded(newPod)
}
