package cluster

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	endpoint "github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

//used for kubernetes.default service
type ByPassClusterInfo struct {
	ServiceClusterInfo
	ClusterIP string
}

func NewByPassClusterInfo(svc *kubernetes.ServiceInfo, port uint32) *ByPassClusterInfo {
	return &ByPassClusterInfo{
		ServiceClusterInfo: *NewServiceClusterInfo(svc, port),
		ClusterIP:          svc.ClusterIP,
	}
}

func (info *ByPassClusterInfo) String() string {
	return fmt.Sprintf("%s.%s:%d,mr=%d,ct=%v", info.Service, info.Namespace, info.Port, info.MaxRetries, info.ConnectionTimeout)
}

func (info *ByPassClusterInfo) CreateCluster() *envoy_api_v2.Cluster {
	result := &envoy_api_v2.Cluster{
		Name:           info.Name(),
		ConnectTimeout: info.ConnectionTimeout,
		ClusterDiscoveryType: &envoy_api_v2.Cluster_Type{
			Type: envoy_api_v2.Cluster_STATIC,
		},
		LoadAssignment: &envoy_api_v2.ClusterLoadAssignment{
			ClusterName: info.Name(),
			Endpoints: []*endpoint.LocalityLbEndpoints{{
				LbEndpoints: []*endpoint.LbEndpoint{{
					HostIdentifier: &endpoint.LbEndpoint_Endpoint{
						Endpoint: &endpoint.Endpoint{
							Address: &core.Address{
								Address: &core.Address_SocketAddress{
									SocketAddress: &core.SocketAddress{
										Protocol: core.SocketAddress_TCP,
										Address:  info.ClusterIP,
										PortSpecifier: &core.SocketAddress_PortValue{
											PortValue: uint32(info.Port),
										},
									},
								},
							},
						},
					}},
				},
			}},
		},
	}

	info.ApplyClusterConfig(result)
	return result
}
