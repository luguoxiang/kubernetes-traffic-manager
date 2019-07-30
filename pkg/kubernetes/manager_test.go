package kubernetes

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"
	"time"
)

func TestGetMatchedResources(t *testing.T) {
	k8sManager := NewFakeK8sResourceManager()
	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")
	serviceWatchlist := k8sManager.GetListerWatcher("services")
	deploymentWatchlist := k8sManager.GetListerWatcher("deployments")
	go k8sManager.WatchPods(stopper, k8sManager)
	go k8sManager.WatchServices(stopper, k8sManager)
	go k8sManager.WatchDeployments(stopper, k8sManager)

	var deploy v1beta1.Deployment
	deploy.Name = "Comp1"
	deploy.Namespace = "test-ns"
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	deploymentWatchlist.Add(&deploy)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	podWatchlist.Add(&pod)

	var service corev1.Service
	service.Namespace = "test-ns"
	service.Spec.Selector = map[string]string{"c": "d"}
	service.Name = "Service1"
	serviceWatchlist.Add(&service)

	time.Sleep(time.Second)
	k8sManager.Lock()
	pods := k8sManager.GetMatchedResources(NewDeploymentInfo(&deploy), POD_TYPE)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name(), "Comp1-pod")
	assert.Equal(t, pods[0].Namespace(), "test-ns")

	pods = k8sManager.GetMatchedResources(NewServiceInfo(&service), POD_TYPE)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name(), "Comp1-pod")
	assert.Equal(t, pods[0].Namespace(), "test-ns")

	deployments := k8sManager.GetMatchedResources(NewPodInfo(&pod), DEPLOYMENT_TYPE)
	assert.Equal(t, len(deployments), 1)
	assert.Equal(t, deployments[0].Name(), "Comp1")
	assert.Equal(t, deployments[0].Namespace(), "test-ns")

	services := k8sManager.GetMatchedResources(NewPodInfo(&pod), SERVICE_TYPE)
	assert.Equal(t, len(services), 1)
	assert.Equal(t, services[0].Name(), "Service1")
	assert.Equal(t, services[0].Namespace(), "test-ns")
	k8sManager.Unlock()
}
