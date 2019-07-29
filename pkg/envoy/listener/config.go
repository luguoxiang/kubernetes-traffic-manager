package listener

import (
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

type HttpListenerConfigInfo struct {
	Tracing        bool
	RequestTimeout time.Duration
	RetryOn        string
	RetryTimes     uint32

	FaultInjectionFixDelayPercentage uint32
	FaultInjectionFixDelay           time.Duration

	FaultInjectionAbortPercentage uint32
	FaultInjectionAbortStatus     uint32

	RateLimitKbps uint64

	HashCookieName string
	HashHeaderName string
	HashCookieTTL  time.Duration
}

func NeedServiceToPodAnnotation(label string, headless bool) bool {
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
		return headless
	default:
		return false
	}
}

func (info *HttpListenerConfigInfo) Config(config map[string]string) {
	info.FaultInjectionAbortStatus = 503
	info.FaultInjectionFixDelay = time.Second

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
			info.HashCookieTTL = time.Duration(kubernetes.GetLabelValueInt64(v)) * time.Millisecond
		case "traffic.tracing.enabled":
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
		case "traffic.rate.limit":
			info.RateLimitKbps = kubernetes.GetLabelValueUInt64(v)
		case "":

		}
	}
}
func (info *HttpListenerConfigInfo) ConfigRouteAction(routeAction *route.RouteAction) {

	if info.HashCookieName != "" {
		cookie := &route.RouteAction_HashPolicy_Cookie{
			Name: info.HashHeaderName,
		}
		if info.HashCookieTTL != 0 {
			cookie.Ttl = &info.HashCookieTTL
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
			NumRetries: &types.UInt32Value{Value: info.RetryTimes}}
	}
	if info.RequestTimeout > 0 {
		routeAction.Timeout = &info.RequestTimeout
	}
}
func (info *HttpListenerConfigInfo) ConfigConnectionManager(manager *hcm.HttpConnectionManager, ingress bool) {

	if info.Tracing {
		if ingress {
			//local inbound tracing
			manager.Tracing = &hcm.HttpConnectionManager_Tracing{
				OperationName: hcm.INGRESS,
			}
		} else {
			manager.Tracing = &hcm.HttpConnectionManager_Tracing{
				OperationName: hcm.EGRESS,
			}
		}
	}
	//headless service will use pod ip egress filter's config, ingress side do not need config
	if ingress {
		return
	}
	faultConfig := &httpfault.HTTPFault{}
	changed := false
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
		filterConfigStruct, err := types.MarshalAny(faultConfig)
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
