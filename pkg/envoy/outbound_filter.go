package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"

	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type OutboundFilterInfo struct {
	clusterIP string
	service   string
	namespace string
	port      uint32
}

func NewOutboundFilterInfo(svc *kubernetes.ServiceInfo, port uint32) *OutboundFilterInfo {
	return &OutboundFilterInfo{
		port:      port,
		clusterIP: svc.ClusterIP,
		service:   svc.Name(),
		namespace: svc.Namespace(),
	}
}

func (info *OutboundFilterInfo) String() string {
	return fmt.Sprintf("%s, clusterIp=%v", info.Name(), info.clusterIP)
}

func (info *OutboundFilterInfo) Type() string {
	return common.ListenerResource
}
func (info *OutboundFilterInfo) Name() string {
	return info.ClusterName()
}

func (info *OutboundFilterInfo) ClusterName() string {
	return common.OutboundClusterName(info.service, info.namespace, info.port)
}

func (info *OutboundFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	tcpProxy := &tcp.TcpProxy{
		StatPrefix: info.Name(),
		ClusterSpecifier: &tcp.TcpProxy_Cluster{
			Cluster: info.ClusterName(),
		},
	}
	filterConfig, err := types.MarshalAny(tcpProxy)
	if err != nil {
		glog.Warningf("MarshalAny tcp.TcpProxy failed: %s", err.Error())
		return listener.FilterChain{}, err
	}

	if info.clusterIP == "" || info.clusterIP == "None" {
		return listener.FilterChain{}, nil
	}
	return listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			DestinationPort: &types.UInt32Value{Value: info.port},
			PrefixRanges: []*core.CidrRange{&core.CidrRange{
				AddressPrefix: info.clusterIP,
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
