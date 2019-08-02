package listener

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"

	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"

	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
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

func (info *HttpClusterIpFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	if info.clusterIP == "" || info.clusterIP == "None" {
		return listener.FilterChain{}, nil
	}
	routeConfig := &v2.RouteConfiguration{
		Name:         info.Name(),
		VirtualHosts: []route.VirtualHost{info.CreateVirtualHost(info.ClusterName(), common.ALL_DOMAIN)},
	}

	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: info.Name(),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
	}
	info.ConfigConnectionManager(manager, false)

	manager.HttpFilters = append(manager.HttpFilters, &hcm.HttpFilter{
		Name: common.RouterHttpFilter,
	})

	filterConfig, err := types.MarshalAny(manager)
	if err != nil {
		glog.Warningf("Failed to MarshalAny HttpConnectionManager: %s", err.Error())
		return listener.FilterChain{}, err
	}

	result := listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{
			DestinationPort: &types.UInt32Value{Value: info.port},
			PrefixRanges: []*core.CidrRange{&core.CidrRange{
				AddressPrefix: info.clusterIP,
				PrefixLen:     &types.UInt32Value{Value: 32},
			},
			},
		},
		Filters: []listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}

	return result, nil
}
