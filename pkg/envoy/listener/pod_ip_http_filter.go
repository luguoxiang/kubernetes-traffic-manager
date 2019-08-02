package listener

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/types"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

//listener filter for local pod or outbound listener filter for headless service pod
type HttpPodIpFilterInfo struct {
	PodIpFilterInfo
	HttpListenerConfigInfo
	Domains map[string][]string
}

func NewHttpPodIpFilterInfo(pod *kubernetes.PodInfo, port uint32) *HttpPodIpFilterInfo {
	podFilter := NewPodIpFilterInfo(pod, port)
	result := &HttpPodIpFilterInfo{
		PodIpFilterInfo: *podFilter,
	}

	serviceMap := pod.GetPortSet()[port]
	for service, _ := range serviceMap {
		if result.Domains == nil {
			result.Domains = make(map[string][]string)
		}
		cluster := cluster.ServiceClusterName(service, pod.Namespace(), port)
		result.Domains[cluster] = []string{
			fmt.Sprintf("%s:%d", service, port),
			fmt.Sprintf("%s:%d.%s", service, port, pod.Namespace()),
		}
	}
	return result
}

func (info *HttpPodIpFilterInfo) String() string {
	return fmt.Sprintf("%s:%d, tracing=%v", info.podIP, info.port, info.Tracing)
}

func (info *HttpPodIpFilterInfo) CreateVirtualHosts(nodeId string) []route.VirtualHost {
	var virtualHosts []route.VirtualHost

	staticCluster := info.getStaticClusterName(nodeId)

	if nodeId != info.node {
		//If pod ip is used to access the service, should use http Host header to match the target service name
		// so that we can do load balance
		for cluster, domains := range info.Domains {
			if len(info.Domains) == 1 {
				// if there is only one domain, use it without match
				domains = common.ALL_DOMAIN
			}
			virtualHosts = append(virtualHosts, info.CreateVirtualHost(cluster, domains))
		}
		if len(info.Domains) != 1 {
			//if no domain matched, route to static ip
			virtualHosts = append(virtualHosts, info.CreateVirtualHost(staticCluster, common.ALL_DOMAIN))
		}
	} else {
		//ingress cluster should not apply any config
		var noconfig HttpPodIpFilterInfo
		virtualHosts = append(virtualHosts, noconfig.CreateVirtualHost(staticCluster, common.ALL_DOMAIN))
	}

	return virtualHosts
}

func (info *HttpPodIpFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {

	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: info.Name(),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         info.Name(),
				VirtualHosts: info.CreateVirtualHosts(node.Id),
			},
		},
	}
	info.ConfigConnectionManager(manager, node.Id == info.node)

	manager.HttpFilters = append(manager.HttpFilters, &hcm.HttpFilter{
		Name: common.RouterHttpFilter,
	})

	filterConfig, err := types.MarshalAny(manager)
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
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}, nil

}
