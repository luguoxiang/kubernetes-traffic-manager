package listener

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	fault "github.com/envoyproxy/go-control-plane/envoy/config/filter/fault/v2"
	httpfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	_type "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"time"
)

type HttpClusterIpFilterInfo struct {
	ClusterIpFilterInfo

	Tracing        bool
	RequestTimeout time.Duration
	RetryOn        string
	RetryTimes     uint32

	FaultInjectionFixDelayPercentage uint32
	FaultInjectionFixDelay           time.Duration

	FaultInjectionAbortPercentage uint32
	FaultInjectionAbortStatus     uint32
}

func NewHttpClusterIpFilterInfo(svc *kubernetes.ServiceInfo, port uint32) *HttpClusterIpFilterInfo {
	egressListenerInfo := NewClusterIpFilterInfo(svc, port)
	info := &HttpClusterIpFilterInfo{
		ClusterIpFilterInfo: *egressListenerInfo,
	}

	info.FaultInjectionAbortStatus = 503
	info.FaultInjectionFixDelay = time.Second

	for k, v := range svc.Labels {
		switch k {
		case kubernetes.TRACING_ENABLED:
			info.Tracing = kubernetes.GetLabelValueBool(v)
		case "traffic.request.timeout":
			info.RequestTimeout = time.Duration(kubernetes.GetLabelValueInt64(v)) * time.Millisecond
		case "traffic.retries.5xx":
			info.RetryOn = "5xx"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		case "traffic.retries.connect-failure":
			info.RetryOn = "connect-failure"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		case "traffic.retries.gateway-error":
			info.RetryOn = "gateway-error"
			info.RetryTimes = kubernetes.GetLabelValueUInt32(v)
		case "traffic.fault.delay.time":
			info.FaultInjectionFixDelay = time.Duration(kubernetes.GetLabelValueUInt32(v)) * time.Millisecond
		case "traffic.fault.delay.percentage":
			info.FaultInjectionFixDelayPercentage = kubernetes.GetLabelValueUInt32(v)
		case "traffic.fault.abort.status":
			info.FaultInjectionAbortStatus = kubernetes.GetLabelValueUInt32(v)
		case "traffic.fault.abort.percentage":
			info.FaultInjectionAbortPercentage = kubernetes.GetLabelValueUInt32(v)
		}
	}

	return info
}

func (info *HttpClusterIpFilterInfo) String() string {
	return fmt.Sprintf("%s, %s,tracing=%v", info.Name(), info.clusterIP, info.Tracing)
}

func (info *HttpClusterIpFilterInfo) CreateVirtualHost() route.VirtualHost {
	routeAction := &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: info.ClusterName(),
		},
	}
	if info.RetryOn != "" {
		routeAction.RetryPolicy = &route.RetryPolicy{
			RetryOn:    info.RetryOn,
			NumRetries: &types.UInt32Value{Value: info.RetryTimes}}
	}
	if info.RequestTimeout > 0 {
		routeAction.Timeout = &info.RequestTimeout
	}

	return route.VirtualHost{
		Name:    fmt.Sprintf("%s_vh", info.Name()),
		Domains: []string{"*"},
		Routes: []route.Route{{
			Match: route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: &route.Route_Route{
				Route: routeAction,
			},
		}},
	}

}

func (info *HttpClusterIpFilterInfo) createHttpFilters() []*hcm.HttpFilter {
	httpFilters := []*hcm.HttpFilter{}
	faultConfig := &httpfault.HTTPFault{}
	if info.FaultInjectionFixDelayPercentage > 0 {
		faultConfig.Delay = &fault.FaultDelay{
			Type: fault.FaultDelay_FIXED,
			FaultDelaySecifier: &fault.FaultDelay_FixedDelay{
				FixedDelay: &info.FaultInjectionFixDelay,
			},
			Percentage: &_type.FractionalPercent{
				Numerator:   info.FaultInjectionFixDelayPercentage,
				Denominator: _type.FractionalPercent_HUNDRED,
			},
		}
	}
	if info.FaultInjectionAbortPercentage > 0 {
		faultConfig.Abort = &httpfault.FaultAbort{
			ErrorType: &httpfault.FaultAbort_HttpStatus{
				HttpStatus: info.FaultInjectionAbortStatus,
			},
			Percentage: &_type.FractionalPercent{
				Numerator:   info.FaultInjectionAbortPercentage,
				Denominator: _type.FractionalPercent_HUNDRED,
			},
		}
	}
	if faultConfig.Delay != nil || faultConfig.Abort != nil {
		filterConfigStruct, err := types.MarshalAny(faultConfig)
		if err != nil {
			glog.Warningf("Failed to MarshalAny HTTPFault: %s", err.Error())
			panic(err.Error())
		}
		httpFilters = append(httpFilters, &hcm.HttpFilter{
			Name:       common.HttpFaultInjection,
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: filterConfigStruct},
		})
	}
	httpFilters = append(httpFilters, &hcm.HttpFilter{
		Name: common.RouterHttpFilter,
	})
	return httpFilters
}
func (info *HttpClusterIpFilterInfo) CreateFilterChain(node *core.Node) (listener.FilterChain, error) {
	if info.clusterIP == "" || info.clusterIP == "None" {
		return listener.FilterChain{}, nil
	}
	routeConfig := &v2.RouteConfiguration{
		Name:         info.Name(),
		VirtualHosts: []route.VirtualHost{info.CreateVirtualHost()},
	}

	manager := &hcm.HttpConnectionManager{
		CodecType:  hcm.AUTO,
		StatPrefix: info.Name(),
		RouteSpecifier: &hcm.HttpConnectionManager_RouteConfig{
			RouteConfig: routeConfig,
		},
		HttpFilters: info.createHttpFilters(),
	}

	if info.Tracing {
		manager.Tracing = &hcm.HttpConnectionManager_Tracing{
			OperationName: hcm.EGRESS,
		}
	}

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
