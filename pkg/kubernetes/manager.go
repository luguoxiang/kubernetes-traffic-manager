package kubernetes

import (
	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"sync"
	"sync/atomic"
)

type ResourcesOnLabel map[ResourceType][]ResourceInfoPointer

type K8sResourceManager struct {
	labelTypeResourceMap map[string]ResourcesOnLabel
	clientSet            kubernetes.Interface
	mutex                *sync.RWMutex
	locked               int32
}

func NewK8sResourceManager() (*K8sResourceManager, error) {

	clientSet, err := getK8sClientSet()
	if err != nil {
		return nil, err
	}

	result := &K8sResourceManager{
		clientSet: clientSet,

		mutex: &sync.RWMutex{},
	}

	result.labelTypeResourceMap = make(map[string]ResourcesOnLabel)
	return result, nil
}
func (manager *K8sResourceManager) NewCond() *sync.Cond {
	return sync.NewCond(manager.mutex)
}
func (manager *K8sResourceManager) Lock() {
	manager.mutex.Lock()
	atomic.AddInt32(&manager.locked, 1)
}
func (manager *K8sResourceManager) Unlock() {
	atomic.StoreInt32(&manager.locked, 0)
	manager.mutex.Unlock()
}

func (manager *K8sResourceManager) IsLocked() bool {
	return atomic.LoadInt32(&manager.locked) != 0
}

func getK8sClientSet() (kubernetes.Interface, error) {
	configPath := os.Getenv("KUBECONFIG")

	var config *rest.Config
	var err error
	if configPath == "" {
		glog.Info("KUBECONFIG: InCluster\n")
		config, err = rest.InClusterConfig()
	} else {
		glog.Infof("KUBECONFIG:%s\n", configPath)
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
	}
	if err != nil {
		return nil, err
	}

	// create the clientset
	return kubernetes.NewForConfig(config)
}

func (manager *K8sResourceManager) PodExists(name string, ns string) (bool, error) {
	_, err := manager.clientSet.CoreV1().Pods(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
