package annotation

import (
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestServiceTracingToPod(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	annotator := NewServiceToPodAnnotator(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")
	serviceWatchlist := k8sManager.GetListerWatcher("services")
	go k8sManager.WatchPods(stopper, k8sManager, annotator)
	go k8sManager.WatchServices(stopper, k8sManager, annotator)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	k8sManager.ClientSet.CoreV1().Pods("test-ns").Create(&pod)
	podWatchlist.Add(&pod)

	var service corev1.Service
	service.Namespace = "test-ns"
	service.Labels = map[string]string{"traffic.tracing.enabled": "true", "traffic.rate.limit": "100"}
	service.Spec.Selector = map[string]string{"c": "d"}
	service.Name = "Service1"
	serviceWatchlist.Add(&service)

	time.Sleep(time.Second)

	pod1, _ := k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.headless"], "")
	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.rate.limit"], "")
	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.tracing.enabled"], "true")
}

func TestServiceHeadlessToPod(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	annotator := NewServiceToPodAnnotator(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")
	serviceWatchlist := k8sManager.GetListerWatcher("services")
	go k8sManager.WatchPods(stopper, k8sManager, annotator)
	go k8sManager.WatchServices(stopper, k8sManager, annotator)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	k8sManager.ClientSet.CoreV1().Pods("test-ns").Create(&pod)
	podWatchlist.Add(&pod)

	var service corev1.Service
	service.Spec.ClusterIP = "None"
	service.Namespace = "test-ns"
	service.Labels = map[string]string{"traffic.tracing.enabled": "true", "traffic.rate.limit": "100"}
	service.Spec.Selector = map[string]string{"c": "d"}
	service.Name = "Service1"
	serviceWatchlist.Add(&service)

	time.Sleep(time.Second)

	pod1, _ := k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.headless"], "true")
	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.rate.limit"], "100")
	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.tracing.enabled"], "true")

	podWatchlist.Modify(pod1)
	time.Sleep(time.Second)

	serviceWatchlist.Delete(&service)

	time.Sleep(time.Second)
	pod1, _ = k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.headless"], "")
	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.rate.limit"], "")
	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.tracing.enabled"], "")
}

func TestServiceToPodUpdate(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	annotator := NewServiceToPodAnnotator(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")
	serviceWatchlist := k8sManager.GetListerWatcher("services")
	go k8sManager.WatchPods(stopper, k8sManager, annotator)
	go k8sManager.WatchServices(stopper, k8sManager, annotator)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	k8sManager.ClientSet.CoreV1().Pods("test-ns").Create(&pod)
	podWatchlist.Add(&pod)

	var service corev1.Service
	service.Namespace = "test-ns"
	service.Labels = map[string]string{"traffic.tracing.enabled": "true"}
	service.Spec.Selector = map[string]string{"c": "d"}
	service.Name = "Service1"
	serviceWatchlist.Add(&service)

	time.Sleep(time.Second)

	pod1, _ := k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.tracing.enabled"], "true")

	podWatchlist.Modify(pod1)
	time.Sleep(time.Second)
	var service1 corev1.Service
	service1.Namespace = "test-ns"
	service1.Labels = map[string]string{}
	service1.Spec.Selector = map[string]string{"c": "d"}
	service1.Name = "Service1"
	serviceWatchlist.Modify(&service1)

	time.Sleep(time.Second)
	pod1, _ = k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.svc.Service1.tracing.enabled"], "")

}
