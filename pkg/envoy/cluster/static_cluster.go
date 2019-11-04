package cluster

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"strings"
	"time"
)

type StaticClusterInfo struct {
	ClusterConfigInfo

	IP     string
	Port   uint32
	NodeId string
}

func NewStaticClusterInfo(ip string, port uint32, nodeId string) *StaticClusterInfo {
	info := &StaticClusterInfo{
		IP:     ip,
		Port:   port,
		NodeId: nodeId,
	}
	info.ConnectionTimeout = time.Duration(60*1000) * time.Millisecond
	return info
}

func StaticClusterName(ip string, port uint32) string {
	return fmt.Sprintf("%d|%s.static", port, strings.Replace(ip, ".", "_", -1))
}

func (info *StaticClusterInfo) String() string {
	return fmt.Sprintf("%s:%d,mc=%d,mpr=%d,mr=%d", info.IP, info.Port, info.MaxConnections, info.MaxPendingRequests, info.MaxRequests)
}

func (info *StaticClusterInfo) Name() string {
	return StaticClusterName(info.IP, info.Port)
}

func (info *StaticClusterInfo) Type() string {
	return common.ClusterResource
}

func (info *StaticClusterInfo) CreateCluster() *v2.Cluster {

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

	info.ApplyClusterConfig(result)

	return result
}
