package listener

import (
	"fmt"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	fault "github.com/envoyproxy/go-control-plane/envoy/config/filter/fault/v2"
	httpfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	_type "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes"
	duration "github.com/golang/protobuf/ptypes/duration"
	wrappers "github.com/golang/protobuf/ptypes/wrappers"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type HttpListenerConfigInfo struct {
	Tracing        bool
	RequestTimeout *duration.Duration
	RetryOn        string
	RetryTimes     uint32

	FaultInjectionFixDelayPercentage uint32
	FaultInjectionFixDelay           *duration.Duration

	FaultInjectionAbortPercentage uint32
	FaultInjectionAbortStatus     uint32

	RateLimitKbps        uint64
	TraceSamplingPercent float64

	HashCookieName string
	HashHeaderName string
	HashCookieTTL  *duration.Duration
}

func NeedServiceToPodAnnotation(label string) bool {
	switch label {
	case "traffic.request.timeout":
		fallthrough
	case "traffic.retries.5xx":
		fallthrough
	case "traffic.retries.connect-failure":
		fallthrough
	case "traffic.retries.gateway-error":
		fallthrough
	case "traffic.fault.delay.time":
		fallthrough
	case "traffic.fault.delay.percentage":
		fallthrough
	case "traffic.fault.abort.status":
		fallthrough
	case "traffic.fault.abort.percentage":
		fallthrough
	case "traffic.rate.limit":
		fallthrough
	case "traffic.tracing.enabled":
		fallthrough
	case "traffic.tracing.sampling":
		return true
	default:
		return false
	}
}

func (info *HttpListenerConfigInfo) Config(config map[string]string) {
	info.FaultInjectionAbortStatus = 503
	info.TraceSamplingPercent = 100
	for k, v := range config {
		if v == "" {
			continue
		}
		switch k {
		case "traffic.hash.cookie.name":
			info.HashCookieName = v
		case "traffic.hash.header.name":
			info.HashHeaderName = v
		case "traffic.hash.cookie.ttl":
			info.HashCookieTTL = &duration.Duration{
				Seconds: kubernetes.GetLabelValueInt64(v),
			}

		case "traffic.tracing.enabled":
			info.Tracing = kubernetes.GetLabelValueBool(v)
		case "traffic.tracing.sampling":
			info.TraceSamplingPercent = kubernetes.GetLabelValueFloat64(v)

		case "traffic.request.timeout":
			value := kubernetes.GetLabelValueInt64(v)
			info.RequestTimeout = &duration.Duration{
				Seconds: value / 1e9,
				Nanos:   int32(value % 1e9),
			}
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
			value := kubernetes.GetLabelValueInt64(v)
			info.FaultInjectionFixDelay = &duration.Duration{
				Seconds: value / 1e9,
				Nanos:   int32(value % 1e9),
			}
		case "traffic.fault.delay.percentage":
			info.FaultInjectionFixDelayPercentage = kubernetes.GetLabelValueUInt32(v)
		case "traffic.fault.abort.status":
			info.FaultInjectionAbortStatus = kubernetes.GetLabelValueUInt32(v)
		case "traffic.fault.abort.percentage":
			info.FaultInjectionAbortPercentage = kubernetes.GetLabelValueUInt32(v)
		case "traffic.rate.limit":
			info.RateLimitKbps = kubernetes.GetLabelValueUInt64(v)

		}
	}
}

func (info *HttpListenerConfigInfo) CreateRouteAction(cluster string) *route.RouteAction {
	routeAction := &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{
			Cluster: cluster,
		},
	}
	if info.HashCookieName != "" {
		cookie := &route.RouteAction_HashPolicy_Cookie{
			Name: info.HashCookieName,
		}
		if info.HashCookieTTL != nil {
			cookie.Ttl = info.HashCookieTTL
		}
		routeAction.HashPolicy = append(routeAction.HashPolicy,
			&route.RouteAction_HashPolicy{
				PolicySpecifier: &route.RouteAction_HashPolicy_Cookie_{
					Cookie: cookie,
				},
			})
	}
	if info.HashHeaderName != "" {
		routeAction.HashPolicy = append(routeAction.HashPolicy,
			&route.RouteAction_HashPolicy{
				PolicySpecifier: &route.RouteAction_HashPolicy_Header_{
					Header: &route.RouteAction_HashPolicy_Header{
						HeaderName: info.HashHeaderName,
					},
				},
			})
	}
	if info.RetryOn != "" {
		routeAction.RetryPolicy = &route.RetryPolicy{
			RetryOn:    info.RetryOn,
			NumRetries: &wrappers.UInt32Value{Value: info.RetryTimes}}
	}
	if info.RequestTimeout != nil {
		routeAction.Timeout = info.RequestTimeout
	}
	return routeAction
}

func (info *HttpListenerConfigInfo) CreateVirtualHost(cluster string, domains []string) *route.VirtualHost {
	return &route.VirtualHost{
		Name:    fmt.Sprintf("%s_vh", cluster),
		Domains: domains,
		Routes: []*route.Route{{
			Match: &route.RouteMatch{
				PathSpecifier: &route.RouteMatch_Prefix{
					Prefix: "/",
				},
			},
			Action: &route.Route_Route{
				Route: info.CreateRouteAction(cluster),
			},
		}},
	}
}

func (info *HttpListenerConfigInfo) ConfigConnectionManager(manager *hcm.HttpConnectionManager) {

	if info.Tracing {
		manager.Tracing = &hcm.HttpConnectionManager_Tracing{
			OperationName: hcm.HttpConnectionManager_Tracing_EGRESS,
		}
		manager.Tracing.OverallSampling = &_type.Percent{
			Value: info.TraceSamplingPercent,
		}
	}

	faultConfig := &httpfault.HTTPFault{}
	changed := false
	if info.FaultInjectionFixDelayPercentage > 0 && info.FaultInjectionFixDelay != nil {
		faultConfig.Delay = &fault.FaultDelay{
			Type: fault.FaultDelay_FIXED,
			FaultDelaySecifier: &fault.FaultDelay_FixedDelay{
				FixedDelay: info.FaultInjectionFixDelay,
			},
			Percentage: &_type.FractionalPercent{
				Numerator:   info.FaultInjectionFixDelayPercentage,
				Denominator: _type.FractionalPercent_HUNDRED,
			},
		}
		changed = true
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
		changed = true
	}
	if info.RateLimitKbps > 0 {
		faultConfig.ResponseRateLimit = &fault.FaultRateLimit{
			LimitType: &fault.FaultRateLimit_FixedLimit_{
				FixedLimit: &fault.FaultRateLimit_FixedLimit{
					LimitKbps: info.RateLimitKbps,
				},
			},
		}
		changed = true
	}
	if changed {
		filterConfigStruct, err := ptypes.MarshalAny(faultConfig)
		if err != nil {
			glog.Warningf("Failed to MarshalAny HTTPFault: %s", err.Error())
			panic(err.Error())
		}

		manager.HttpFilters = []*hcm.HttpFilter{
			&hcm.HttpFilter{
				Name:       common.HttpFaultInjection,
				ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: filterConfigStruct},
			},
		}
	}

}
