package listener

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/golang/protobuf/ptypes"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
)

type BlackHoleFilterInfo struct {
}

func (info *BlackHoleFilterInfo) String() string {
	return "listener,blackhole"
}

func (info *BlackHoleFilterInfo) Type() string {
	return common.ListenerResource
}
func (info *BlackHoleFilterInfo) Name() string {
	return "blackhole"
}

func (info *BlackHoleFilterInfo) CreateFilterChain(node *core.Node) (*listener.FilterChain, error) {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &envoy_api_v2.RouteConfiguration{
				Name: "blackhole",
				VirtualHosts: []*route.VirtualHost{{
					Name:    "blackhole_vh",
					Domains: []string{"*"},
					Routes: []*route.Route{{
						Match: &route.RouteMatch{
							PathSpecifier: &route.RouteMatch_Prefix{
								Prefix: "/",
							},
						},
						Action: &route.Route_DirectResponse{
							DirectResponse: &route.DirectResponseAction{
								Status: 404,
							},
						},
					},
					},
				},
				},
			},
		},

		HttpFilters: []*hcm.HttpFilter{{
			Name: common.RouterHttpFilter,
		}},
	}
	filterConfig, err := ptypes.MarshalAny(manager)
	if err != nil {
		return nil, err
	}
	return &listener.FilterChain{
		Filters: []*listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}, nil
}
