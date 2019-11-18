package listener

import (
	"fmt"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"

	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type ClusterIpFilterInfo struct {
	clusterIP string
	service   string
	namespace string
	port      uint32
}

func NewClusterIpFilterInfo(svc *kubernetes.ServiceInfo, port uint32) *ClusterIpFilterInfo {
	return &ClusterIpFilterInfo{
		port:      port,
		clusterIP: svc.ClusterIP,
		service:   svc.Name(),
		namespace: svc.Namespace(),
	}
}

func (info *ClusterIpFilterInfo) String() string {
	return fmt.Sprintf("%s, clusterIp=%v", info.Name(), info.clusterIP)
}

func (info *ClusterIpFilterInfo) Type() string {
	return common.ListenerResource
}
func (info *ClusterIpFilterInfo) Name() string {
	return info.ClusterName()
}

func (info *ClusterIpFilterInfo) ClusterName() string {
	return cluster.ServiceClusterName(info.service, info.namespace, info.port)
}

func (info *ClusterIpFilterInfo) CreateFilterChain(node *core.Node) (*listener.FilterChain, error) {
	tcpProxy := &tcp.TcpProxy{
		StatPrefix: info.Name(),
		ClusterSpecifier: &tcp.TcpProxy_Cluster{
			Cluster: info.ClusterName(),
		},
	}
	filterConfig, err := ptypes.MarshalAny(tcpProxy)
	if err != nil {
		glog.Warningf("MarshalAny tcp.TcpProxy failed: %s", err.Error())
		return nil, err
	}

	if info.clusterIP == "" || info.clusterIP == "None" {
		return nil, nil
	}
	return &listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			DestinationPort: &wrappers.UInt32Value{Value: info.port},
			PrefixRanges: []*core.CidrRange{&core.CidrRange{
				AddressPrefix: info.clusterIP,
				PrefixLen:     &wrappers.UInt32Value{Value: 32},
			},
			},
		},
		Filters: []*listener.Filter{{
			Name:       common.TCPProxy,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}, nil
}
