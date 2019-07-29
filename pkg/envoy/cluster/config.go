package cluster

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"time"
)

type ClusterConfigInfo struct {
	MaxRetries         uint32
	MaxConnections     uint32
	MaxPendingRequests uint32
	MaxRequests        uint32
	ConnectionTimeout  time.Duration
}

func NeedServiceToPodAnnotation(label string, headless bool) bool {
	switch label {
	case "traffic.connection.timeout":
		fallthrough
	case "traffic.retries.max":
		fallthrough
	case "traffic.connection.max":
		fallthrough
	case "traffic.request.max-pending":
		fallthrough
	case "traffic.request.max":
		return headless
	default:
		return false
	}

}

func (info *ClusterConfigInfo) Config(config map[string]string) {
	info.ConnectionTimeout = time.Duration(60*1000) * time.Millisecond
	for k, v := range config {
		if v == "" {
			continue
		}
		switch k {
		case "traffic.connection.timeout":
			info.ConnectionTimeout = time.Duration(kubernetes.GetLabelValueInt64(v)) * time.Millisecond
		case "traffic.retries.max":
			info.MaxRetries = kubernetes.GetLabelValueUInt32(v)
		case "traffic.connection.max":
			info.MaxConnections = kubernetes.GetLabelValueUInt32(v)
		case "traffic.request.max-pending":
			info.MaxPendingRequests = kubernetes.GetLabelValueUInt32(v)
		case "traffic.request.max":
			info.MaxRequests = kubernetes.GetLabelValueUInt32(v)
		}
	}
}

func (info *ClusterConfigInfo) ApplyClusterConfig(clusterInfo *v2.Cluster) {
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
	if info.MaxRetries > 0 {
		threshold.MaxRetries = &types.UInt32Value{
			Value: info.MaxRetries,
		}
		hasCircuitBreaker = true
	}
	if hasCircuitBreaker {
		clusterInfo.CircuitBreakers = &cluster.CircuitBreakers{
			Thresholds: []*cluster.CircuitBreakers_Thresholds{&threshold},
		}
	}
}
