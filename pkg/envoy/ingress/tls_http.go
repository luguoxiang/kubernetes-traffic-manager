package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	accesslog_filter "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/envoy/common"
)

type IngressTlsHttpInfo struct {
	IngressHttpClusterInfo
	TlsSecret string
}

func (info *IngressTlsHttpInfo) Name() string {
	return fmt.Sprintf("tls_http|%s|%s", info.Host, info.PathPrefix)
}

func (info *IngressTlsHttpInfo) String() string {
	return fmt.Sprintf("tls_http,%s|%s,cluster=%s, rewrite=%s", info.Host, info.PathPrefix, info.Cluster, info.PrefixRewrite)
}

func CreateTlsFilterChain(host string, infoList []IngressInfo, tlsSecret string) listener.FilterChain {
	var routes []route.Route
	for _, httpIngress := range infoList {
		routes = append(routes, httpIngress.CreateRoute())
	}
	var name string
	if host == "*" {
		name = "all-ingress"
	} else {
		name = fmt.Sprintf("%s-ingress", host)
	}
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: name,
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name: name,
				VirtualHosts: []route.VirtualHost{{
					Name:    fmt.Sprintf("%s_vh", name),
					Domains: []string{host},
					Routes:  routes,
				}},
			},
		},
		AccessLog: []*accesslog_filter.AccessLog{
			&accesslog_filter.AccessLog{
				Name: "envoy.file_access_log",
				ConfigType: &accesslog_filter.AccessLog_TypedConfig{
					TypedConfig: common.CreateAccessLogAny(true),
				},
			},
		},
		Tracing: &hcm.HttpConnectionManager_Tracing{
			OperationName: hcm.EGRESS,
		},
		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}
	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny HttpConnectionManager: %s", err.Error())
		panic(err.Error())
	}

	result := listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			ServerNames:       []string{host},
			TransportProtocol: "tls",
		},
		Filters: []listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
		TlsContext: &auth.DownstreamTlsContext{
			CommonTlsContext: &auth.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{{
					Name: tlsSecret,
					SdsConfig: &core.ConfigSource{
						ConfigSourceSpecifier: &core.ConfigSource_Ads{
							Ads: &core.AggregatedConfigSource{},
						},
					},
				}},
			},
		},
	}

	return result
}
