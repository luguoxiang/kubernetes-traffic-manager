package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"time"
)

type OutboundClusterInfo struct {
	Service           string
	Namespace         string
	Port              uint32
	MaxRetries        uint32
	ConnectionTimeout time.Duration
}

func NewOutboundClusterInfo(svc *kubernetes.ServiceInfo, port uint32) *OutboundClusterInfo {
	return &OutboundClusterInfo{
		Service:           svc.Name(),
		Namespace:         svc.Namespace(),
		Port:              port,
		ConnectionTimeout: time.Duration(60*1000) * time.Millisecond,
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

func (info *OutboundClusterInfo) configCluster(svc *kubernetes.ServiceInfo) {
	for k, v := range svc.Labels {
		switch k {
		case "traffic.envoy.connection.timeout":
			info.ConnectionTimeout = time.Duration(kubernetes.GetLabelValueInt64(v)) * time.Millisecond
		case "traffic.envoy.retries.max":
			info.MaxRetries = kubernetes.GetLabelValueUInt32(v)
		}
	}
}

func (info *OutboundClusterInfo) CreateCluster() *v2.Cluster {
	result := &v2.Cluster{
		Name:           info.Name(),
		ConnectTimeout: info.ConnectionTimeout,
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
