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
	podIP string
	node  string
	port  uint32
}

func NewPodIpFilterInfo(pod *kubernetes.PodInfo, port uint32) *PodIpFilterInfo {
	return &PodIpFilterInfo{
		port:  port,
		podIP: pod.PodIP,
		node:  fmt.Sprintf("%s.%s", pod.Name(), pod.Namespace()),
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

func (info *PodIpFilterInfo) getStaticClusterName(nodeId string) string {
	return cluster.StaticClusterName(info.podIP, info.port)
}

func (info *PodIpFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	clusterName := info.getStaticClusterName(node.Id)

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
