package annotation

import (
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestDeploymentEnvoyToPod(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	annotator := NewDeploymentToPodAnnotator(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")
	deploymentWatchlist := k8sManager.GetListerWatcher("deployments")
	go k8sManager.WatchPods(stopper, k8sManager, annotator)
	go k8sManager.WatchDeployments(stopper, k8sManager, annotator)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	k8sManager.ClientSet.CoreV1().Pods("test-ns").Create(&pod)
	podWatchlist.Add(&pod)

	var deploy v1beta1.Deployment
	deploy.Name = "Comp1"
	deploy.Namespace = "test-ns"
	deploy.Labels = map[string]string{"traffic.envoy.enabled": "true"}
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	deploymentWatchlist.Add(&deploy)

	time.Sleep(time.Second)

	pod1, _ := k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.deployment.envoy.enabled"], "true")
	podWatchlist.Modify(pod1)
	time.Sleep(time.Second)

	var deploy2 v1beta1.Deployment
	deploy2.Name = "Comp1"
	deploy2.Namespace = "test-ns"
	deploy2.Labels = map[string]string{}
	deploy2.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	deploymentWatchlist.Modify(&deploy2)
	time.Sleep(time.Second)

	pod1, _ = k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.deployment.envoy.enabled"], "")
}

func TestDeploymentWeightToPod(t *testing.T) {
	k8sManager := kubernetes.NewFakeK8sResourceManager()
	annotator := NewDeploymentToPodAnnotator(k8sManager)

	stopper := make(chan struct{})
	defer close(stopper)

	podWatchlist := k8sManager.GetListerWatcher("pods")
	deploymentWatchlist := k8sManager.GetListerWatcher("deployments")
	go k8sManager.WatchPods(stopper, k8sManager, annotator)
	go k8sManager.WatchDeployments(stopper, k8sManager, annotator)

	var pod corev1.Pod
	pod.Namespace = "test-ns"
	pod.Labels = map[string]string{"a": "b", "c": "d"}
	pod.Status.PodIP = "10.1.1.1"
	pod.Name = "Comp1-pod"
	k8sManager.ClientSet.CoreV1().Pods("test-ns").Create(&pod)
	podWatchlist.Add(&pod)

	var deploy v1beta1.Deployment
	deploy.Name = "Comp1"
	deploy.Namespace = "test-ns"
	deploy.Labels = map[string]string{"traffic.endpoint.weight": "50"}
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	deploymentWatchlist.Add(&deploy)

	time.Sleep(time.Second)

	pod1, _ := k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.deployment.endpoint.weight"], "50")
	podWatchlist.Modify(pod1)
	time.Sleep(time.Second)

	var deploy2 v1beta1.Deployment
	deploy2.Name = "Comp1"
	deploy2.Namespace = "test-ns"
	deploy2.Labels = map[string]string{"traffic.endpoint.weight": "80"}
	deploy2.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}
	deploymentWatchlist.Modify(&deploy2)
	time.Sleep(time.Second)

	pod1, _ = k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})

	assert.Equal(t, pod1.Annotations["traffic.deployment.endpoint.weight"], "80")
	podWatchlist.Modify(pod1)
	time.Sleep(time.Second)

	deploymentWatchlist.Delete(&deploy2)
	time.Sleep(time.Second)

	pod1, _ = k8sManager.ClientSet.CoreV1().Pods("test-ns").Get("Comp1-pod", metav1.GetOptions{})
	assert.Equal(t, pod1.Annotations["traffic.deployment.endpoint.weight"], "")
}
