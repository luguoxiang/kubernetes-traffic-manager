package listener

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	tp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"strings"
)

//listener filter for local pod or outbound listener filter for headless service pod
type PodIpFilterInfo struct {
	podIP            string
	node             string
	port             uint32
	Headless         bool
	LocalAccessPodIP bool
}

func NewPodIpFilterInfo(pod *kubernetes.PodInfo, port uint32, headless bool) *PodIpFilterInfo {
	return &PodIpFilterInfo{
		port:             port,
		podIP:            pod.PodIP,
		LocalAccessPodIP: kubernetes.GetLabelValueBool(pod.Labels[kubernetes.LOCAL_ACCESS_POD_IP]),
		node:             fmt.Sprintf("%s.%s", pod.Name(), pod.Namespace()),
		//the pod belongs to a headless service, need to listen on pod ip
		Headless: headless,
	}
}

func (info *PodIpFilterInfo) String() string {
	return fmt.Sprintf("%s:%d", info.node, info.port)
}

func (info *PodIpFilterInfo) Type() string {
	return common.ListenerResource
}

func (info *PodIpFilterInfo) Name() string {
	return fmt.Sprintf("%d|%s.static", info.port, strings.Replace(info.node, ".", "|", -1))
}

func (info *PodIpFilterInfo) getClusterName(nodeId string) string {
	if nodeId == info.node {
		if info.LocalAccessPodIP {
			return cluster.StaticClusterName(info.podIP, info.port)
		}
		//use local loop interface to access local workload
		return cluster.StaticClusterName(common.LOCALHOST, info.port)
	} else {
		if info.Headless {
			return cluster.StaticClusterName(info.podIP, info.port)
		}
	}
	return ""
}

func (info *PodIpFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	clusterName := info.getClusterName(node.Id)
	if clusterName == "" {
		return listener.FilterChain{}, nil
	}

	filterConfig, err := types.MarshalAny(&tp.TcpProxy{
		StatPrefix: info.Name(),
		ClusterSpecifier: &tp.TcpProxy_Cluster{
			Cluster: clusterName,
		},
	})
	if err != nil {
		return listener.FilterChain{}, err
	}
	return listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			DestinationPort: &types.UInt32Value{Value: info.port},
			PrefixRanges: []*core.CidrRange{&core.CidrRange{
				AddressPrefix: info.podIP,
				PrefixLen:     &types.UInt32Value{Value: 32},
			},
			},
		},
		Filters: []listener.Filter{{
			Name:       common.TCPProxy,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}, nil

}
