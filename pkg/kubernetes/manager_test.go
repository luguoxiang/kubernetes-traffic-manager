package kubernetes

import (
	"github.com/stretchr/testify/assert"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sort"
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
	statefulsetWatchlist := k8sManager.GetListerWatcher("statefulsets")
	daemonsetWatchlist := k8sManager.GetListerWatcher("daemonsets")

	go k8sManager.WatchPods(stopper, k8sManager)
	go k8sManager.WatchServices(stopper, k8sManager)
	go k8sManager.WatchDeployments(stopper, k8sManager)
	go k8sManager.WatchStatefulSets(stopper, k8sManager)
	go k8sManager.WatchDaemonSets(stopper, k8sManager)

	var deploy v1beta1.Deployment
	deploy.Name = "Comp1"
	deploy.Namespace = "test-ns"
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	deploymentWatchlist.Add(&deploy)

	var stateful apps_v1beta1.StatefulSet
	stateful.Name = "Comp2"
	stateful.Namespace = "test-ns"
	stateful.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	statefulsetWatchlist.Add(&stateful)

	var daemonSet v1beta1.DaemonSet
	daemonSet.Name = "Comp3"
	daemonSet.Namespace = "test-ns"
	daemonSet.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	daemonsetWatchlist.Add(&daemonSet)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	podWatchlist.Add(&pod)

	var service corev1.Service
	service.Namespace = "test-ns"
	service.Spec.Selector = map[string]string{"a": "b"}
	service.Name = "Service1"
	serviceWatchlist.Add(&service)

	var service2 corev1.Service
	service2.Namespace = "test-ns"
	service2.Spec.Selector = map[string]string{"c": "d"}
	service2.Name = "Service2"
	serviceWatchlist.Add(&service2)

	time.Sleep(time.Second)
	k8sManager.Lock()
	pods := k8sManager.GetMatchedResources(NewDeploymentInfo(&deploy), POD_TYPE)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name(), "Comp1-pod")
	assert.Equal(t, pods[0].Namespace(), "test-ns")

	pods = k8sManager.GetMatchedResources(NewDeploymentInfo(&stateful), POD_TYPE)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name(), "Comp1-pod")
	assert.Equal(t, pods[0].Namespace(), "test-ns")

	pods = k8sManager.GetMatchedResources(NewDeploymentInfo(&daemonSet), POD_TYPE)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name(), "Comp1-pod")
	assert.Equal(t, pods[0].Namespace(), "test-ns")

	pods = k8sManager.GetMatchedResources(NewServiceInfo(&service), POD_TYPE)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name(), "Comp1-pod")
	assert.Equal(t, pods[0].Namespace(), "test-ns")

	deployments := k8sManager.GetMatchedResources(NewPodInfo(&pod), DEPLOYMENT_TYPE)
	assert.Equal(t, len(deployments), 3)

	var result []string
	for i := 0; i < 3; i++ {
		result = append(result, deployments[i].Name())
		assert.Equal(t, deployments[i].Namespace(), "test-ns")
	}
	sort.Strings(result)

	assert.True(t, reflect.DeepEqual(result, []string{"Comp1", "Comp2", "Comp3"}))

	services := k8sManager.GetMatchedResources(NewPodInfo(&pod), SERVICE_TYPE)
	assert.Equal(t, len(services), 2)
	result = nil
	for i := 0; i < 2; i++ {
		result = append(result, services[i].Name())
		assert.Equal(t, deployments[i].Namespace(), "test-ns")
	}
	sort.Strings(result)
	assert.True(t, reflect.DeepEqual(result, []string{"Service1", "Service2"}))
	k8sManager.Unlock()
}
