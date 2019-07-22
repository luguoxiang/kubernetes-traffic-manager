package envoy

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/kubernetes"
	"reflect"
)

type ClusterInfo interface {
	common.EnvoyResource
	CreateCluster() *v2.Cluster
}

type ClustersControlPlaneService struct {
	*common.ControlPlaneService
}

func NewClustersControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *ClustersControlPlaneService {
	return &ClustersControlPlaneService{ControlPlaneService: common.NewControlPlaneService(k8sManager)}
}

func (cps *ClustersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {
	var clusters []proto.Message

	for _, resource := range resourceMap {
		clusterInfo := resource.(ClusterInfo)
		serviceCluster := clusterInfo.CreateCluster()
		clusters = append(clusters, serviceCluster)
	}

	return common.MakeResource(clusters, common.ClusterResource, version)
}

func (cps *ClustersControlPlaneService) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return svc.OutboundEnabled()
}

func (cps *ClustersControlPlaneService) ServiceAdded(svc *kubernetes.ServiceInfo) {
	for _, port := range svc.Ports {
		if svc.IsKubeAPIService() {
			cluster := NewByPassClusterInfo(svc, port.Port)
			cluster.configCluster(svc)
			cps.UpdateResource(cluster, svc.ResourceVersion)
		} else {
			cluster := NewOutboundClusterInfo(svc, port.Port)
			cluster.configCluster(svc)
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
