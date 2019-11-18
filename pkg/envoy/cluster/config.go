package cluster

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/cluster"
	duration "github.com/golang/protobuf/ptypes/duration"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type ClusterConfigInfo struct {
	MaxRetries         uint32
	MaxConnections     uint32
	MaxPendingRequests uint32
	MaxRequests        uint32
	ConnectionTimeout  *duration.Duration
}

func NeedServiceToPodAnnotation(label string) bool {
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
		return true
	default:
		return false
	}

}

func (info *ClusterConfigInfo) Config(config map[string]string) {
	info.ConnectionTimeout = &duration.Duration{
		Seconds: 60,
	}
	for k, v := range config {
		if v == "" {
			continue
		}
		switch k {
		case "traffic.connection.timeout":
			value := kubernetes.GetLabelValueInt64(v)
			info.ConnectionTimeout =
				&duration.Duration{
					Seconds: value / 1e9,
					Nanos:   int32(value % 1e9),
				}

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

func (info *ClusterConfigInfo) ApplyClusterConfig(clusterInfo *envoy_api_v2.Cluster) {
	var threshold envoy_api_v2_cluster.CircuitBreakers_Thresholds
	var hasCircuitBreaker bool
	if info.MaxConnections > 0 {
		threshold.MaxConnections = &wrappers.UInt32Value{
			Value: info.MaxConnections,
		}
		hasCircuitBreaker = true
	}
	if info.MaxPendingRequests > 0 {
		threshold.MaxPendingRequests = &wrappers.UInt32Value{
			Value: info.MaxPendingRequests,
		}
		hasCircuitBreaker = true
	}
	if info.MaxRequests > 0 {
		threshold.MaxRequests = &wrappers.UInt32Value{
			Value: info.MaxRequests,
		}
		hasCircuitBreaker = true
	}
	if info.MaxRetries > 0 {
		threshold.MaxRetries = &wrappers.UInt32Value{
			Value: info.MaxRetries,
		}
		hasCircuitBreaker = true
	}
	if hasCircuitBreaker {
		clusterInfo.CircuitBreakers = &envoy_api_v2_cluster.CircuitBreakers{
			Thresholds: []*envoy_api_v2_cluster.CircuitBreakers_Thresholds{&threshold},
		}
	}
}
