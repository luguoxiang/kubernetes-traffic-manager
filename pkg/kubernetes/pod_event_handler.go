package kubernetes

import (
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
)

type PodEventHandler interface {
	PodValid(pod *PodInfo) bool
	PodAdded(pod *PodInfo)
	PodDeleted(pod *PodInfo)
	PodUpdated(oldPod, newPod *PodInfo)
}

func (manager *K8sResourceManager) PodValid(info *PodInfo) bool {
	return info.Valid()
}

func (manager *K8sResourceManager) PodAdded(info *PodInfo) {
	manager.addResource(info)
}

func (manager *K8sResourceManager) PodDeleted(info *PodInfo) {
	manager.removeResource(info)
}

func (manager *K8sResourceManager) PodUpdated(oldPod, newPod *PodInfo) {
	manager.PodDeleted(oldPod)
	manager.PodAdded(newPod)
}

func (manager *K8sResourceManager) WatchPods(stopper chan struct{}, handlers ...PodEventHandler) {
	_, controller := cache.NewInformer(
		manager.watchListMap["pods"],
		&v1.Pod{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := NewPodInfo(obj.(*v1.Pod))
				if pod == nil {
					return
				}

				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					if h.PodValid(pod) {
						h.PodAdded(pod)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				pod := NewPodInfo(obj.(*v1.Pod))
				if pod == nil {
					return
				}
				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					if h.PodValid(pod) {
						h.PodDeleted(pod)
					}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldPod := NewPodInfo(oldObj.(*v1.Pod))
				newPod := NewPodInfo(newObj.(*v1.Pod))
				if oldPod == nil && newPod == nil {
					return
				}
				if oldPod != nil && newPod != nil {
					newVersion := newPod.ResourceVersion
					//ignore ResourceVersion diff
					newPod.ResourceVersion = oldPod.ResourceVersion
					if reflect.DeepEqual(oldPod, newPod) {
						return
					}
					newPod.ResourceVersion = newVersion
				}

				manager.Lock()
				defer manager.Unlock()
				for _, h := range handlers {
					oldValid := (oldPod != nil && h.PodValid(oldPod))
					newValid := (newPod != nil && h.PodValid(newPod))
					if !oldValid && newValid {
						h.PodAdded(newPod)
					} else if oldValid && !newValid {
						h.PodDeleted(oldPod)
					} else if oldValid && newValid {
						h.PodUpdated(oldPod, newPod)
					}
				}
			},
		},
	)
	glog.Info("Start watching pods")
	controller.Run(stopper)
	glog.Info("WatchPods terminated")

}
