package envoy

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/types"
)

type BlackHoleFilterInfo struct {
}

func (info *BlackHoleFilterInfo) String() string {
	return "listener,blackhole"
}

func (info *BlackHoleFilterInfo) Type() string {
	return ListenerResource
}
func (info *BlackHoleFilterInfo) Name() string {
	return "blackhole"
}

func (info *BlackHoleFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "http",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name: "blackhole",
				VirtualHosts: []route.VirtualHost{{
					Name:    "blackhole_vh",
					Domains: []string{"*"},
					Routes: []route.Route{{
						Match: route.RouteMatch{
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
			Name: RouterHttpFilter,
		}},
	}
	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		return listener.FilterChain{}, err
	}
	return listener.FilterChain{
		Filters: []listener.Filter{{
			Name:       HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}, nil
}
