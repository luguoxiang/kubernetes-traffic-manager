package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"strings"
	"time"
)

type StaticClusterInfo struct {
	IP   string
	Port uint32

	MaxConnections     uint32
	MaxPendingRequests uint32
	MaxRequests        uint32
	ConnectionTimeout  time.Duration
}

func NewStaticClusterInfo(ip string, port uint32) *StaticClusterInfo {
	return &StaticClusterInfo{
		IP:                ip,
		Port:              port,
		ConnectionTimeout: time.Duration(60*1000) * time.Millisecond,
	}
}

func StaticClusterName(ip string, port uint32) string {
	//envoy stat/prometheus will split the name by '.' and use last element as metrices name
	//so this cluster name will have metrics
	//envoy_cluster_static_upstream_rq{envoy_cluster_name="9080|10_1_2_3",envoy_response_code="200",instance="10.1.2.3:8900",job="traffic-envoy-pods"}
	return fmt.Sprintf("%d|%s.static", port, strings.Replace(ip, ".", "_", -1))
}

func (info *StaticClusterInfo) String() string {
	return fmt.Sprintf("cluster,static,%s:%d,mc=%d,mpr=%d,mr=%d", info.IP, info.Port, info.MaxConnections, info.MaxPendingRequests, info.MaxRequests)
}

func (info *StaticClusterInfo) Name() string {
	return StaticClusterName(info.IP, info.Port)
}

func (info *StaticClusterInfo) Type() string {
	return common.ClusterResource
}

func (info *StaticClusterInfo) CreateCluster() *v2.Cluster {
	var threshold cluster.CircuitBreakers_Thresholds
	var hasCircuitBreaker bool
	if info.MaxConnections > 0 {
		threshold.MaxConnections = &types.UInt32Value{
			Value: info.MaxConnections,
		}
		hasCircuitBreaker = true
	}
	if info.MaxPendingRequests > 0 {
		threshold.MaxPendingRequests = &types.UInt32Value{
			Value: info.MaxPendingRequests,
		}
		hasCircuitBreaker = true
	}
	if info.MaxRequests > 0 {
		threshold.MaxRequests = &types.UInt32Value{
			Value: info.MaxRequests,
		}
		hasCircuitBreaker = true
	}

	result := &v2.Cluster{
		Name:           info.Name(),
		ConnectTimeout: info.ConnectionTimeout,
		ClusterDiscoveryType: &v2.Cluster_Type{
			Type: v2.Cluster_STATIC,
		},
		LoadAssignment: &v2.ClusterLoadAssignment{
			ClusterName: info.Name(),
			Endpoints: []endpoint.LocalityLbEndpoints{{
				LbEndpoints: []endpoint.LbEndpoint{{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Protocol: core.TCP,
										Address:  info.IP,
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: uint32(info.Port),
										},
									},
								},
							},
						},
					},
				}},
			}},
		},
	}

	if hasCircuitBreaker {
		result.CircuitBreakers = &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{&threshold},
		}
	}
	return result
}

func (info *StaticClusterInfo) configCluster(pod *kubernetes.PodInfo) {
	for k, v := range pod.Labels {
		switch k {
		case "traffic.envoy.connection.max":
			info.MaxConnections = kubernetes.GetLabelValueUInt32(v)
		case "traffic.envoy.request.max-pending":
			info.MaxPendingRequests = kubernetes.GetLabelValueUInt32(v)
		case "traffic.envoy.request.max":
			info.MaxRequests = kubernetes.GetLabelValueUInt32(v)
		}
	}
}

func (manager *ClustersControlPlaneService) PodValid(pod *kubernetes.PodInfo) bool {
	//Hostnetwork pod should not have envoy enabled, so there will be no inbound cluster for it
	return !pod.HostNetwork && pod.PodIP != ""
}

func (cps *ClustersControlPlaneService) PodAdded(pod *kubernetes.PodInfo) {
	for port, _ := range pod.GetPortMap() {
		cluster := NewStaticClusterInfo(pod.PodIP, port)
		cluster.configCluster(pod)
		cps.UpdateResource(cluster, pod.ResourceVersion)

		cluster = NewStaticClusterInfo("127.0.0.1", port)
		cps.UpdateResource(cluster, "1")
	}
}
func (cps *ClustersControlPlaneService) PodDeleted(pod *kubernetes.PodInfo) {
	for port, _ := range pod.GetPortMap() {
		cluster := NewStaticClusterInfo(pod.PodIP, port)
		cps.UpdateResource(cluster, "")
	}
}
func (cps *ClustersControlPlaneService) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	vistied := make(map[string]bool)
	for port, _ := range newPod.GetPortMap() {
		cluster := NewStaticClusterInfo(newPod.PodIP, port)
		vistied[cluster.Name()] = true
		cluster.configCluster(newPod)
		cps.UpdateResource(cluster, newPod.ResourceVersion)

		cluster = NewStaticClusterInfo("127.0.0.1", port)
		cps.UpdateResource(cluster, "1")
	}

	for port, _ := range oldPod.GetPortMap() {
		cluster := NewStaticClusterInfo(oldPod.PodIP, port)
		if vistied[cluster.Name()] {
			continue
		}
		cps.UpdateResource(cluster, "")
	}
}
