package listener

import (
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
}

func (info *HttpListenerConfigInfo) Config(config map[string]string) {
	info.FaultInjectionAbortStatus = 503
	info.FaultInjectionFixDelay = time.Second

	for k, v := range config {
		switch k {
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
		}
	}
}
