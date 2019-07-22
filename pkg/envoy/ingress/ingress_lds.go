package ingress

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/glog"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/kubernetes"
	"os"
	"strconv"
	"strings"
)

type IngressInfo interface {
	common.EnvoyResource
	GetCluster() string
	CreateRoute() route.Route
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

func (cps *IngressListenersControlPlaneService) doUpdateResource(svc *kubernetes.ServiceInfo, port uint32, resource IngressInfo, version string) bool {
	oldInfo, _ := cps.GetResourceNoCopy(resource.Name())
	if oldInfo != nil {
		oldResource := oldInfo.(IngressInfo)
		oldCluster := oldResource.GetCluster()
		if oldCluster != resource.GetCluster() {
			cps.GetK8sManager().UpdateServiceAnnotation(svc, map[string]*string{
				common.IngressAttribute(port, "conflict"): &oldCluster,
			})
			glog.Warningf("conflict ingress config %s for cluster %s, already assigned for %s",
				resource.Name(), resource.GetCluster(), oldResource.GetCluster())
			return false
		}
	}
	cps.UpdateResource(resource, version)
	return true
}

func (cps *IngressListenersControlPlaneService) configForService(svc *kubernetes.ServiceInfo, add bool) {
	if svc.Annotations == nil {
		return
	}
	var version string
	if add {
		version = svc.ResourceVersion
	}

	for _, portInfo := range svc.Ports {
		port := portInfo.Port
		conflict := svc.Annotations[common.IngressAttribute(port, "conflict")]
		if conflict != "" {
			continue
		}
		hosts := svc.Annotations[common.IngressAttribute(port, "hosts")]
		pathPrefix := svc.Annotations[common.IngressAttribute(port, "path_prefix")]
		var tlsSecret string
		if hosts == "" || hosts == "*" {
			if pathPrefix == "" {
				continue
			}
			hosts = "*"
		} else {
			tlsSecret = svc.Annotations[common.IngressAttribute(port, "tls_secret")]
		}
		hostList := strings.Split(hosts, ",")
		if tlsSecret != "" {
			cluster := common.OutboundClusterName(svc.Name(), svc.Namespace(), port)
			if svc.IsHttp(port) {
				for _, host := range hostList {
					resource := NewIngressHttpInfo(svc, host, port, tlsSecret)
					cps.doUpdateResource(svc, port, resource, version)
				}
			} else {
				for _, host := range hostList {
					resource := &IngressTlsTcpInfo{
						Host:      host,
						Cluster:   cluster,
						TlsSecret: tlsSecret,
					}
					cps.doUpdateResource(svc, port, resource, version)
				}
			}
		} else {
			for _, host := range hostList {
				resource := NewIngressHttpInfo(svc, host, port, "")
				cps.doUpdateResource(svc, port, resource, version)
			}
		}
	}
}
func (cps *IngressListenersControlPlaneService) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return svc.OutboundEnabled()
}
func (cps *IngressListenersControlPlaneService) ServiceAdded(svc *kubernetes.ServiceInfo) {
	cps.configForService(svc, true)
}
func (cps *IngressListenersControlPlaneService) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	cps.configForService(svc, false)
}

func (cps *IngressListenersControlPlaneService) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	cps.configForService(oldService, false)
	cps.configForService(newService, true)
}

func (cps *IngressListenersControlPlaneService) BuildResource(resourceMap map[string]common.EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {
	ingressMap := make(map[string][]IngressInfo)
	tlsIngressMap := make(map[string][]IngressInfo)
	tlsSecretMap := make(map[string]string)

	var filterChains []listener.FilterChain
	for _, resource := range resourceMap {
		switch v := resource.(type) {
		case *IngressTlsTcpInfo:
			filterChains = append(filterChains, v.CreateFilterChain())
			tlsIngressMap[v.Host] = []IngressInfo{}
		case *IngressTlsHttpInfo:
			tlsSecretMap[v.Host] = v.TlsSecret
			tlsIngressMap[v.Host] = append(tlsIngressMap[v.Host], v)
		case *IngressHttpClusterInfo:
			ingressMap[v.Host] = append(ingressMap[v.Host], v)
		case *IngressDirectHttpInfo:
			ingressMap[v.Host] = append(ingressMap[v.Host], v)
		default:
			glog.Warningf("Unexpected resource %T", resource)
			continue
		}
	}
	for host, infoList := range tlsIngressMap {
		ingressMap[host] = []IngressInfo{&IngressRedirectHttpsInfo{}}
		if len(infoList) > 0 {
			filterChains = append(filterChains, CreateTlsFilterChain(host, infoList, tlsSecretMap[host]))
		}
	}
	filterChains = append(filterChains, CreateFilterChain(ingressMap))

	l := &v2.Listener{
		Name: "mylistener",
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

		//		ListenerFilters: []listener.ListenerFilter{
		//			listener.ListenerFilter{
		//				Name: common.TLS_INSPECTOR,
		//			},
		//		},
		FilterChains: filterChains,
	}
	return common.MakeResource([]proto.Message{l}, common.ListenerResource, version)
}
