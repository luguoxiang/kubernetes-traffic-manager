package kubernetes

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetPortSet(t *testing.T) {
	pod := PodInfo{
		Labels: map[string]string{
			"traffic.port.1234": "http",
			"traffic.port.2345": "",
		},
		Annotations: map[string]string{
			"traffic.svc.testsvc.port.3456": "http",
			"traffic.svc.testsvc.port.4567": "",
		},
	}
	result := pod.GetPortSet()
	assert.Equal(t, len(result), 2)
	assert.True(t, result[1234] != nil)
	assert.True(t, result[3456]["testsvc"])

}

func TestGetPortConfig(t *testing.T) {
	pod := PodInfo{
		Labels: map[string]string{
			"traffic.port.1234":            "http",
			"traffic.port.2345":            "",
			"traffic.rate.limit":           "200",
			"traffic.port.6789.rate.limit": "100",
			"traffic.port.5678":            "tcp",
		},
		Annotations: map[string]string{
			"traffic.svc.svc2.port.1234":       "tcp",
			"traffic.svc.svc1.port.3456":       "http",
			"traffic.svc.svc2.tracing.enabled": "true",
			"traffic.svc.svc3.port.4567":       "",
		},
	}
	result := pod.GetPortConfig()
	assert.Equal(t, len(result), 3)
	assert.Equal(t, result[1234].Protocol, PROTO_HTTP)
	assert.Equal(t, result[3456].Protocol, PROTO_HTTP)
	assert.Equal(t, len(result[1234].ConfigMap), 2)
	assert.Equal(t, result[1234].ConfigMap["traffic.tracing.enabled"], "true")
	assert.Equal(t, result[1234].ConfigMap["traffic.rate.limit"], "200")
	assert.Equal(t, len(result[3456].ConfigMap), 0)
	assert.Equal(t, len(result[5678].ConfigMap), 1)
	assert.Equal(t, result[5678].ConfigMap["traffic.rate.limit"], "200")
}
