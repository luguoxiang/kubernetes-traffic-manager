package cluster

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type ClusterInfo interface {
	common.EnvoyResource
	CreateCluster() *envoy_api_v2.Cluster
}

type ClustersControlPlaneService struct {
	*common.ControlPlaneService
}

func NewClustersControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *ClustersControlPlaneService {
	return &ClustersControlPlaneService{ControlPlaneService: common.NewControlPlaneService(k8sManager)}
}

func (cps *ClustersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*envoy_api_v2.DiscoveryResponse, error) {
	var clusters []proto.Message

	for _, resource := range resourceMap {
		clusterInfo := resource.(ClusterInfo)
		serviceCluster := clusterInfo.CreateCluster()
		if serviceCluster.ConnectTimeout == nil {
			panic(fmt.Sprintf("cluster %s connect timeout should not be zero", serviceCluster.Name))
		}
		clusters = append(clusters, serviceCluster)
	}

	return common.MakeResource(clusters, common.ClusterResource, version)
}

func (cps *ClustersControlPlaneService) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return true
}

func (cps *ClustersControlPlaneService) ServiceAdded(svc *kubernetes.ServiceInfo) {
	cps.ServiceUpdated(nil, svc)
}

func (cps *ClustersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	cps.ServiceUpdated(svc, nil)
}
func (cps *ClustersControlPlaneService) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	visited := make(map[string]bool)
	if newService != nil {
		for _, port := range newService.Ports {
			protocol := newService.Protocol(port.Port)
			if protocol == kubernetes.PROTO_DIRECT {
				cluster := NewByPassClusterInfo(newService, port.Port)
				cluster.Config(newService.Labels)
				visited[cluster.Name()] = true
				cps.UpdateResource(cluster, newService.ResourceVersion)
			} else if protocol >= 0 {
				cluster := NewServiceClusterInfo(newService, port.Port)
				cluster.Config(newService.Labels)
				visited[cluster.Name()] = true
				cps.UpdateResource(cluster, newService.ResourceVersion)
			}

		}
	}

	if oldService != nil {
		for _, port := range oldService.Ports {
			cluster := NewServiceClusterInfo(oldService, port.Port)
			if !visited[cluster.Name()] {
				cps.UpdateResource(cluster, "")
			}
		}
	}
}

func (cps *ClustersControlPlaneService) PodValid(pod *kubernetes.PodInfo) bool {
	return pod.Valid()
}

func (cps *ClustersControlPlaneService) PodAdded(pod *kubernetes.PodInfo) {
	cps.PodUpdated(nil, pod)
}
func (cps *ClustersControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	cps.PodUpdated(pod, nil)
}
func (cps *ClustersControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	visited := make(map[string]bool)
	if newPod != nil {
		for port, config := range newPod.GetTargetPortConfig() {
			cluster := NewStaticClusterInfo(newPod.PodIP, port, newPod.NodeId())

			visited[cluster.Name()] = true
			cluster.Config(config.ConfigMap)

			cps.UpdateResource(cluster, newPod.ResourceVersion)
		}
	}

	if oldPod != nil {
		for port, _ := range oldPod.GetTargetPortConfig() {
			cluster := NewStaticClusterInfo(oldPod.PodIP, port, oldPod.NodeId())
			if !visited[cluster.Name()] {
				cps.UpdateResource(cluster, "")
			}
		}
	}

}
