package kubernetes

import (
	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	fcache "k8s.io/client-go/tools/cache/testing"
	"sync"
)

func NewFakeK8sResourceManager() *K8sResourceManager {
	glog.Info("Using fake kubernetes client")

	result := &K8sResourceManager{
		ClientSet: fake.NewSimpleClientset(),

		mutex:                &sync.RWMutex{},
		labelTypeResourceMap: make(map[string]ResourcesOnLabel),
		watchListMap:         make(map[string]cache.ListerWatcher),
	}

	for _, resourceType := range []string{"pods", "services", "deployments", "statefulsets", "daemonsets"} {
		result.watchListMap[resourceType] = fcache.NewFakeControllerSource()
	}
	return result
}

func (manager *K8sResourceManager) GetListerWatcher(resourceType string) *fcache.FakeControllerSource {
	return manager.watchListMap[resourceType].(*fcache.FakeControllerSource)
}
