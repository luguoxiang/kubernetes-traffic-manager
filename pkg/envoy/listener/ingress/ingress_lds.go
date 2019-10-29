package ingress

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"os"
	"strconv"
	"strings"
)

type IngressListenerInfo interface {
	common.EnvoyResource
}

type IngressListenersControlPlaneService struct {
	*common.ControlPlaneService
	proxyPort uint32
}

func NewIngressListenersControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *IngressListenersControlPlaneService {

	proxyPortStr := os.Getenv("ENVOY_PROXY_PORT")
	if proxyPortStr == "" {
		panic("env ENVOY_PROXY_PORT is not set")
	}

	proxyPort, err := strconv.ParseInt(proxyPortStr, 10, 32)
	if err != nil {
		panic("wrong ENVOY_PROXY_PORT value:" + err.Error())
	}
	result := &IngressListenersControlPlaneService{
		ControlPlaneService: common.NewControlPlaneService(k8sManager),
		proxyPort:           uint32(proxyPort),
	}

	return result
}

func (cps *IngressListenersControlPlaneService) IngressValid(ingressInfo *kubernetes.IngressInfo) bool {
	return true
}

func getClusterName(svc string, ns string, port uint32) string {

	tokens := strings.Split(svc, ".")
	if len(tokens) > 1 {
		svc = tokens[0]
		ns = tokens[1]
	}
	return cluster.ServiceClusterName(svc, ns, port)
}

func (cps *IngressListenersControlPlaneService) IngressAdded(ingressInfo *kubernetes.IngressInfo) {
	for host, pathMap := range ingressInfo.HostPathToClusterMap {
		info := NewIngressHttpInfo(host)
		for path, clusterInfo := range pathMap {
			cluster := getClusterName(clusterInfo.Service, ingressInfo.Namespace(), clusterInfo.Port)
			info.PathClusterMap[path] = cluster
		}
		cps.UpdateResource(info, ingressInfo.ResourceVersion)
	}
}
func (cps *IngressListenersControlPlaneService) IngressDeleted(ingressInfo *kubernetes.IngressInfo) {
	for host, _ := range ingressInfo.HostPathToClusterMap {
		info := NewIngressHttpInfo(host)
		cps.UpdateResource(info, "")
	}
}
func (cps *IngressListenersControlPlaneService) IngressUpdated(oldIngress, newIngress *kubernetes.IngressInfo) {
	cps.IngressDeleted(oldIngress)
	cps.IngressAdded(newIngress)
}
func (cps *IngressListenersControlPlaneService) CreateHttpFilterChain(virtualHosts []route.VirtualHost) listener.FilterChain {
	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "traffic-ingress",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         "traffic-ingress",
				VirtualHosts: virtualHosts,
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

	return listener.FilterChain{
		Filters: []listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}
}
func (cps *IngressListenersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {

	var virtualHosts []route.VirtualHost

	for _, resource := range resourceMap {
		v := resource.(*IngressHttpInfo)
		virtualHosts = append(virtualHosts, v.CreateVirtualHost())
	}

	var filterChains []listener.FilterChain

	if len(virtualHosts) > 0 {
		filterChains = append(filterChains, cps.CreateHttpFilterChain(virtualHosts))
	}

	l := &v2.Listener{
		Name: "ingress_listener",
		Address: core.Address{
			Address: &core.Address_SocketAddress{
				SocketAddress: &core.SocketAddress{
					Protocol: core.TCP,
					Address:  "0.0.0.0",
					PortSpecifier: &core.SocketAddress_PortValue{
						PortValue: cps.proxyPort,
					},
				},
			},
		},

		FilterChains: filterChains,
	}
	return common.MakeResource([]proto.Message{l}, common.ListenerResource, version)
}
