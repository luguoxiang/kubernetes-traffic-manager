package listener

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"

	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type HttpClusterIpFilterInfo struct {
	ClusterIpFilterInfo
	HttpListenerConfigInfo
}

func NewHttpClusterIpFilterInfo(svc *kubernetes.ServiceInfo, port uint32) *HttpClusterIpFilterInfo {
	egressListenerInfo := NewClusterIpFilterInfo(svc, port)
	info := &HttpClusterIpFilterInfo{
		ClusterIpFilterInfo: *egressListenerInfo,
	}

	return info
}

func (info *HttpClusterIpFilterInfo) String() string {
	return fmt.Sprintf("%s,%s,tracing=%v", info.Name(), info.clusterIP, info.Tracing)
}

func (info *HttpClusterIpFilterInfo) CreateFilterChain(node *core.Node) (*listener.FilterChain, error) {
	if info.clusterIP == "" || info.clusterIP == "None" {
		return nil, nil
	}
	routeConfig := &envoy_api_v2.RouteConfiguration{
		Name:         info.Name(),
		VirtualHosts: []*route.VirtualHost{info.CreateVirtualHost(info.ClusterName(), common.ALL_DOMAIN)},
	}

	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.HttpConnectionManager_AUTO,
		StatPrefix: info.Name(),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
	}
	info.ConfigConnectionManager(manager)

	manager.HttpFilters = append(manager.HttpFilters, &hcm.HttpFilter{
		Name: common.RouterHttpFilter,
	})

	filterConfig, err := ptypes.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny HttpConnectionManager: %s", err.Error())
		return nil, err
	}

	result := &listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			DestinationPort: &wrappers.UInt32Value{Value: info.port},
			PrefixRanges: []*core.CidrRange{&core.CidrRange{
				AddressPrefix: info.clusterIP,
				PrefixLen:     &wrappers.UInt32Value{Value: 32},
			},
			},
		},
		Filters: []*listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}

	return result, nil
}
