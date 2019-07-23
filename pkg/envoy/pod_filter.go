package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	accesslog_filter "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	tp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"strings"
)

type PodFilterInfo struct {
	podIP         string
	node          string
	port          uint32
	InboundPodIP  bool
	OutboundPodIP bool
}

func NewPodFilterInfo(pod *kubernetes.PodInfo, port uint32, outboundPodIP bool) *PodFilterInfo {
	inboundPodIP := pod.Labels[kubernetes.ENDPOINT_INBOUND_PODIP]
	return &PodFilterInfo{
		port:         port,
		podIP:        pod.PodIP,
		node:         fmt.Sprintf("%s.%s", pod.Name(), pod.Namespace()),
		InboundPodIP: strings.EqualFold(inboundPodIP, "true"),
		//the pod belongs to a headless service, need to listen on pod ip
		OutboundPodIP: outboundPodIP,
	}
}

func (info *PodFilterInfo) String() string {
	return fmt.Sprintf("listener,tcp,%s:%d", info.node, info.port)
}

func (info *PodFilterInfo) Type() string {
	return common.ListenerResource
}

func (info *PodFilterInfo) Name() string {
	return fmt.Sprintf("%d|%s.static", info.port, strings.Replace(info.node, ".", "_", -1))
}

func (info *PodFilterInfo) getClusterName(nodeId string) string {
	if nodeId == info.node {
		if info.InboundPodIP {
			return StaticClusterName(info.podIP, info.port)
		} else {
			//use local loop interface to access local workload
			return StaticClusterName("127.0.0.1", info.port)
		}
	} else {
		if info.OutboundPodIP {
			return StaticClusterName(info.podIP, info.port)
		}
	}
	return ""
}

func (info *PodFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	clusterName := info.getClusterName(node.Id)
	if clusterName == "" {
		return listener.FilterChain{}, nil
	}

	filterConfig, err := types.MarshalAny(&tp.TcpProxy{
		StatPrefix: info.Name(),
		AccessLog: []*accesslog_filter.AccessLog{
			&accesslog_filter.AccessLog{
				Name: "envoy.file_access_log",
				ConfigType: &accesslog_filter.AccessLog_TypedConfig{
					TypedConfig: common.CreateAccessLogAny(false),
				},
			},
		},
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
