package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

//used for kubernetes.default service
type ByPassClusterInfo struct {
	OutboundClusterInfo
	ClusterIP string
}

func NewByPassClusterInfo(svc *kubernetes.ServiceInfo, port uint32) *ByPassClusterInfo {
	return &ByPassClusterInfo{
		OutboundClusterInfo: *NewOutboundClusterInfo(svc, port),
		ClusterIP:           svc.ClusterIP,
	}
}

func (info *ByPassClusterInfo) String() string {
	return fmt.Sprintf("%s.%s:%d,mr=%d,ct=%v", info.Service, info.Namespace, info.Port, info.MaxRetries, info.ConnectionTimeout)
}

func (info *ByPassClusterInfo) CreateCluster() *v2.Cluster {
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

	if info.MaxRetries > 0 {
		var threshold cluster.CircuitBreakers_Thresholds
		threshold.MaxRetries = &types.UInt32Value{
			Value: info.MaxRetries,
		}
		result.CircuitBreakers = &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{&threshold},
		}
	}
	return result
}
