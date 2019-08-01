package endpoint

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

const (
	WEIGHT_LABEL            = "traffic.endpoint.weight"
	DEPLOYMENT_WEIGHT_LABEL = kubernetes.POD_DEPLOYMENT_PREFIX + "endpoint.weight"
)

type EndpointInfo struct {
	PodIP   string
	Weight  uint32
	Version string
}

func (info EndpointInfo) String() string {
	return fmt.Sprintf("%s|%d", info.PodIP, info.Weight)
}

func NeedDeploymentToPodAnnotation(key string) bool {
	switch key {
	case WEIGHT_LABEL:
		return true
	case kubernetes.ENVOY_ENABLED:
		return true
	default:
		return false
	}
}

func (info *EndpointInfo) Config(pod *kubernetes.PodInfo) {

	weight := pod.Labels[WEIGHT_LABEL]
	if weight == "" {
		//pod label override deployment label
		weight = pod.Annotations[DEPLOYMENT_WEIGHT_LABEL]
	}
	if weight != "" {
		info.Weight = kubernetes.GetLabelValueUInt32(weight)
		if info.Weight > 128 {
			//should fall into [0, 128]
			info.Weight = 128
		}
	} else {
		info.Weight = 100
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
