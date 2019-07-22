package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	accesslog_filter "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/envoy/common"
)

type IngressTlsTcpInfo struct {
	Host      string
	Cluster   string
	TlsSecret string
}

func (info *IngressTlsTcpInfo) GetCluster() string {
	return info.Cluster
}

func (info *IngressTlsTcpInfo) Name() string {
	return fmt.Sprintf("tls_tcp|%s", info.Host)
}
func (info *IngressTlsTcpInfo) Type() string {
	return common.ListenerResource
}

func (info *IngressTlsTcpInfo) CreateRoute() route.Route {
	return route.Route{}
}
func (info *IngressTlsTcpInfo) CreateFilterChain() listener.FilterChain {
	manager := &tcp.TcpProxy{
		StatPrefix: info.Name(),
		AccessLog: []*accesslog_filter.AccessLog{
			&accesslog_filter.AccessLog{
				Name: "envoy.file_access_log",
				ConfigType: &accesslog_filter.AccessLog_TypedConfig{
					TypedConfig: common.CreateAccessLogAny(false),
				},
			},
		},
		ClusterSpecifier: &tcp.TcpProxy_Cluster{
			Cluster: info.Cluster,
		},
	}
	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny tcp.TcpProxy: %s", err.Error())
		panic(err.Error())
	}

	return listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			ServerNames:       []string{info.Host},
			TransportProtocol: "tls",
		},

		Filters: []listener.Filter{{
			Name:       common.TCPProxy,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
		TlsContext: &auth.DownstreamTlsContext{
			CommonTlsContext: &auth.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{{
					Name: info.TlsSecret,
					SdsConfig: &core.ConfigSource{
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
					},
				}},
			},
		},
	}
}

func (info *IngressTlsTcpInfo) String() string {
	return fmt.Sprintf("tls_tcp,%s,cluster=%s", info.Host, info.Cluster)
}
