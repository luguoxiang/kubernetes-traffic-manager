package endpoint

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type EndpointInfo struct {
	PodIP   string
	Weight  uint32
	Version string
	Healthy bool
}

func (info EndpointInfo) String() string {
	return fmt.Sprintf("%s|%d", info.PodIP, info.Weight)
}

func NeedDeploymentToPodAnnotation(key string) bool {
	switch key {
	case "traffic.endpoint.weight":
		return true
	case kubernetes.ENVOY_ENABLED:
		return true
	default:
		return false
	}
}

func (info *EndpointInfo) Config(config map[string]string) {
	info.Weight = 100
	info.Healthy = true
	for k, v := range config {
		if v == "" {
			continue
		}
		if kubernetes.AnnotationHasDeploymentLabel(k) {
			k = kubernetes.DeploymentAnnotationToLabel(k)
		}
		switch k {
		case "traffic.endpoint.weight":
			weight := kubernetes.GetLabelValueUInt32(v)
			if weight > 128 {
				weight = 128
			}
			info.Weight = weight
		}
	}

}

func (info *EndpointInfo) CreateLoadBalanceEndpoint(port uint32) *endpoint.LbEndpoint {
	if info.Weight == 0 {
		return nil
	}
	result := &endpoint.LbEndpoint{
		HostIdentifier: &endpoint.LbEndpoint_Endpoint{
			Endpoint: &endpoint.Endpoint{
				Address: &core.Address{
					Address: &core.Address_SocketAddress{
						SocketAddress: &core.SocketAddress{
							Protocol: core.TCP,
							Address:  info.PodIP,
							PortSpecifier: &core.SocketAddress_PortValue{
								PortValue: port,
							},
						},
					},
				},
			},
		},
		LoadBalancingWeight: &types.UInt32Value{
			Value: info.Weight,
		},
	}

	return result
}
