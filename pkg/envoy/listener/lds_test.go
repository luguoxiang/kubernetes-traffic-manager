package listener

import (
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"os"
	"testing"
	"time"
)

func TestPodFilter(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	os.Setenv("ENVOY_PROXY_PORT", "10000")
	lds := NewListenersControlPlaneService(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")

	go k8sManager.WatchPods(stopper, k8sManager, lds)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"traffic.envoy.enabled": "true", "c": "d"}
	pod.Annotations = map[string]string{
		"traffic.svc.Service1.port.8080":        "http",
		"traffic.svc.Service1.target.port.8080": "http",
	}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	podWatchlist.Add(&pod)

	time.Sleep(time.Second)

	result, _ := lds.GetResources([]string{})
	assert.Equal(t, len(result), 2)
	assert.Equal(t, result["blackhole"].Name(), "blackhole")
	assert.Equal(t, result["8080|Comp1-pod|test-ns.static"].Name(), "8080|Comp1-pod|test-ns.static")
}

func TestServiceFilter(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	os.Setenv("ENVOY_PROXY_PORT", "10000")
	lds := NewListenersControlPlaneService(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	serviceWatchlist := k8sManager.GetListerWatcher("services")
	go k8sManager.WatchServices(stopper, k8sManager, lds)

	var service corev1.Service
	service.Namespace = "test-ns"
	service.Labels = map[string]string{"traffic.port.8080": "http"}
	service.Spec.Selector = map[string]string{"c": "d"}
	service.Name = "Service1"
	service.Spec.Ports = []corev1.ServicePort{{Name: "test", Port: 8080}}
	serviceWatchlist.Add(&service)

	time.Sleep(time.Second)

	result, _ := lds.GetResources([]string{})
	assert.Equal(t, len(result), 2)
	assert.Equal(t, result["blackhole"].Name(), "blackhole")
	assert.Equal(t, result["8080|test-ns|Service1.outbound"].Name(), "8080|test-ns|Service1.outbound")
}
