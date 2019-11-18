package ingress

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
)

func createFilters(virtualHosts []*route.VirtualHost, pathList []*IngressHttpInfo) []*listener.Filter {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "traffic-ingress",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy_api_v2.RouteConfiguration{
				Name:         "traffic-ingress",
				VirtualHosts: virtualHosts,
			},
		},

		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}

	var conflictConfig bool
	for index, info := range pathList {
		if index > 0 {
			if info.Service != pathList[0].Service {
				conflictConfig = true
			} else if info.Namespace != pathList[0].Namespace {
				conflictConfig = true
			}
		}
	}
	if !conflictConfig {
		//multiple ingress config in same connection manager is ignored
		pathList[0].ConfigConnectionManager(manager)
	}
	filterConfig, err := ptypes.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny HttpConnectionManager: %s", err.Error())
		panic(err.Error())
	}

	return []*listener.Filter{{
		Name:       common.HTTPConnectionManager,
		ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
	}}

}

func CreateHttpFilterChain(pathList []*IngressHttpInfo) *listener.FilterChain {
	var virtualHosts []*route.VirtualHost
	var routes []*route.Route
	for index, info := range pathList {
		if index > 0 && info.Path == pathList[index-1].Path && info.Host == pathList[index-1].Host {
			//ignore same host and path
			continue
		}

		routes = append(routes, info.CreateRoute())
		if index == len(pathList)-1 || info.Host != pathList[index+1].Host {
			virtualHosts = append(virtualHosts, &route.VirtualHost{
				Name:    IngressName(info.Host),
				Domains: []string{info.Host},
				Routes:  routes,
			})
			routes = nil
		}
	}

	return &listener.FilterChain{
		Filters: createFilters(virtualHosts, pathList),
	}
}

func CreateTlsHttpFilterChain(host string, pathList []*IngressHttpInfo) *listener.FilterChain {
	var routes []*route.Route
	secrets := make(map[string]bool)

	for _, info := range pathList {
		routes = append(routes, info.CreateRoute())
		secrets[info.Secret] = true

	}
	virtualHost := &route.VirtualHost{
		Name:    IngressName(host),
		Domains: []string{host},
		Routes:  routes,
	}

	var sdsConfig []*auth.SdsSecretConfig
	for secret, _ := range secrets {
		sdsConfig = append(sdsConfig, &auth.SdsSecretConfig{
			Name: secret,
			SdsConfig: &core.ConfigSource{
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		})
	}
	return &listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			ServerNames:       []string{host},
			TransportProtocol: "tls",
		},

		Filters: createFilters([]*route.VirtualHost{virtualHost}, pathList),

		TlsContext: &auth.DownstreamTlsContext{
			CommonTlsContext: &auth.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: sdsConfig,
			},
		},
	}
}
