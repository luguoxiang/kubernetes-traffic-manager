package ingress

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	accesslog_filter "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/kubernetes"
	"strings"
	"time"
)

type IngressHttpInfo struct {
	Host       string
	PathPrefix string
}
type IngressHttpClusterInfo struct {
	IngressHttpInfo
	Cluster        string
	RequestTimeout time.Duration
	RetryOn        string
	RetryTimes     uint32
	PrefixRewrite  string
}

type IngressDirectHttpInfo struct {
	IngressHttpInfo
	DirectResponse string
}

type IngressRedirectHttpsInfo struct {
	IngressHttpInfo
}

func NewIngressHttpInfo(svc *kubernetes.ServiceInfo, host string, port uint32, tlsSecret string) IngressInfo {
	pathPrefix := svc.Annotations[common.IngressAttribute(port, "path_prefix")]
	directResponse := svc.Annotations[common.IngressAttribute(port, "direct_response")]

	if directResponse != "" {
		return &IngressDirectHttpInfo{
			IngressHttpInfo: IngressHttpInfo{
				Host:       host,
				PathPrefix: pathPrefix,
			},
			DirectResponse: directResponse,
		}
	}
	prefixRewrite := svc.Annotations[fmt.Sprintf("traffic.svc.%d.ingress.prefix_rewrite", port)]
	cluster := common.OutboundClusterName(svc.Name(), svc.Namespace(), port)
	if !strings.HasPrefix(pathPrefix, "/") {
		pathPrefix = "/" + pathPrefix
	}
	info := &IngressHttpClusterInfo{
		IngressHttpInfo: IngressHttpInfo{
			Host:       host,
			PathPrefix: pathPrefix,
		},
		Cluster:       cluster,
		PrefixRewrite: prefixRewrite,
	}
	for k, v := range svc.Labels {
		switch k {
		case "traffic.envoy.request.timeout":
			info.RequestTimeout = time.Duration(kubernetes.GetLabelValueInt64(v)) * time.Millisecond
		case "traffic.envoy.retries.5xx":
			info.RetryOn = "5xx"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		case "traffic.envoy.retries.connect-failure":
			info.RetryOn = "connect-failure"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		case "traffic.envoy.retries.gateway-error":
			info.RetryOn = "gateway-error"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		case "traffic.envoy.retries.retriable-4xx":
			info.RetryOn = "retriable-4xx"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		}
	}
	if tlsSecret != "" {
		return &IngressTlsHttpInfo{
			IngressHttpClusterInfo: *info,
			TlsSecret:              tlsSecret,
		}
	}
	return info
}

func (info *IngressHttpInfo) GetCluster() string {
	return ""
}

func (info *IngressHttpInfo) CreateRoute() route.Route {
	return route.Route{}
}
func (info *IngressHttpInfo) Name() string {
	if info.Host == "*" {
		return fmt.Sprintf("http|all|%s", info.PathPrefix)
	}
	return fmt.Sprintf("http|%s|%s", info.Host, info.PathPrefix)
}

func (info *IngressHttpInfo) Type() string {
	return common.ListenerResource
}

func (info *IngressHttpInfo) String() string {
	return fmt.Sprintf("http,%s|%s", info.Host, info.PathPrefix)
}
func (info *IngressDirectHttpInfo) CreateRoute() route.Route {
	result := route.Route{
		Match: route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: info.PathPrefix,
			},
		},
	}

	result.Action = &route.Route_DirectResponse{
		DirectResponse: &route.DirectResponseAction{
			Status: 200,
			Body: &core.DataSource{
				Specifier: &core.DataSource_InlineString{
					InlineString: info.DirectResponse,
				},
			},
		},
	}
	return result

}
func (info *IngressRedirectHttpsInfo) CreateRoute() route.Route {
	return route.Route{
		Match: route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: "/",
			},
		},
		Action: &route.Route_Redirect{
			Redirect: &route.RedirectAction{
				SchemeRewriteSpecifier: &route.RedirectAction_HttpsRedirect{
					HttpsRedirect: true,
				},
			},
		},
	}
}

func (info *IngressHttpClusterInfo) GetCluster() string {
	return info.Cluster
}

func (info *IngressHttpClusterInfo) CreateRoute() route.Route {
	result := route.Route{
		Match: route.RouteMatch{
			PathSpecifier: &route.RouteMatch_Prefix{
				Prefix: info.PathPrefix,
			},
		},
	}

	routeAction := &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: info.Cluster,
		},
	}
	if info.PrefixRewrite != "" {
		routeAction.PrefixRewrite = info.PrefixRewrite
	}
	if info.RetryOn != "" {
		routeAction.RetryPolicy = &route.RetryPolicy{
			RetryOn:    info.RetryOn,
			NumRetries: &types.UInt32Value{Value: info.RetryTimes}}
	}

	if info.RequestTimeout > 0 {
		routeAction.Timeout = &info.RequestTimeout
	}
	result.Action = &route.Route_Route{
		Route: routeAction,
	}
	return result
}

func CreateFilterChain(ingressMap map[string][]IngressInfo) listener.FilterChain {
	var virtualHosts []route.VirtualHost
	for host, infoList := range ingressMap {
		var routes []route.Route
		for _, httpIngress := range infoList {
			routes = append(routes, httpIngress.CreateRoute())
		}
		var name string
		if host == "*" {
			name = "all_ingress_vh"
		} else {
			name = fmt.Sprintf("%s_ingress_vh", host)
		}
		virtualHosts = append(virtualHosts, route.VirtualHost{
			Name:    name,
			Domains: []string{host},
			Routes:  routes,
		})
	}

	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: "traffic-ingress",
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: &v2.RouteConfiguration{
				Name:         "traffic-ingress",
				VirtualHosts: virtualHosts,
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

	return listener.FilterChain{
		FilterChainMatch: &listener.FilterChainMatch{},
		Filters: []listener.Filter{{
			Name:       common.HTTPConnectionManager,
			ConfigType: &listener.Filter_TypedConfig{TypedConfig: filterConfig},
		}},
	}

}
