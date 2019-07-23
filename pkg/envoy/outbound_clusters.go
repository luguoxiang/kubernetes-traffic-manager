package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type OutboundClusterInfo struct {
	common.ClusterConfigInfo

	Service   string
	Namespace string
	Port      uint32
}

func NewOutboundClusterInfo(svc *kubernetes.ServiceInfo, port uint32) *OutboundClusterInfo {
	return &OutboundClusterInfo{
		Service:   svc.Name(),
		Namespace: svc.Namespace(),
		Port:      port,
	}
}

func (info *OutboundClusterInfo) String() string {
	return fmt.Sprintf("%s.%s:%d,mr=%d,ct=%v", info.Service, info.Namespace, info.Port, info.MaxRetries, info.ConnectionTimeout)
}

func (info *OutboundClusterInfo) Name() string {
	return common.OutboundClusterName(info.Service, info.Namespace, info.Port)
}

func (info *OutboundClusterInfo) Type() string {
	return common.ClusterResource
}

func (info *OutboundClusterInfo) CreateCluster() *v2.Cluster {
	result := &v2.Cluster{
		Name: info.Name(),
		ClusterDiscoveryType: &v2.Cluster_Type{
			Type: v2.Cluster_EDS,
		},
		EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
			EdsConfig: &core.ConfigSource{
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		},
	}
	info.ApplyClusterConfig(result)
	return result
}
