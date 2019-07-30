package kubernetes

import (
	"github.com/golang/glog"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	v1beta1 "k8s.io/api/extensions/v1beta1"

	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
)

type DeploymentEventHandler interface {
	DeploymentValid(deployment *DeploymentInfo) bool
	DeploymentAdded(deployment *DeploymentInfo)
	DeploymentDeleted(deployment *DeploymentInfo)
	DeploymentUpdated(oldDeployment, newDeployment *DeploymentInfo)
}

func (manager *K8sResourceManager) DeploymentValid(deployment *DeploymentInfo) bool {
	return true
}
func (manager *K8sResourceManager) DeploymentAdded(deployment *DeploymentInfo) {
	manager.addResource(deployment)
}
func (manager *K8sResourceManager) DeploymentDeleted(deployment *DeploymentInfo) {
	manager.removeResource(deployment)
}
func (manager *K8sResourceManager) DeploymentUpdated(oldDeployment, newDeployment *DeploymentInfo) {
	manager.removeResource(oldDeployment)
	manager.addResource(newDeployment)
}

func (manager *K8sResourceManager) getDeploymentEventHandler(handlers []DeploymentEventHandler) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deployment := NewDeploymentInfo(obj)

			manager.Lock()
			defer manager.Unlock()

			for _, h := range handlers {
				if h.DeploymentValid(deployment) {
					h.DeploymentAdded(deployment)
				}
			}

		},
		DeleteFunc: func(obj interface{}) {
			deployment := NewDeploymentInfo(obj)

			manager.Lock()
			defer manager.Unlock()

			for _, h := range handlers {
				if h.DeploymentValid(deployment) {
					h.DeploymentDeleted(deployment)
				}
			}

		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeployment := NewDeploymentInfo(oldObj)
			newDeployment := NewDeploymentInfo(newObj)
			if reflect.DeepEqual(oldDeployment, newDeployment) {
				return
			}

			manager.Lock()
			defer manager.Unlock()
			for _, h := range handlers {
				oldValid := h.DeploymentValid(oldDeployment)
				newValid := h.DeploymentValid(newDeployment)
				if !oldValid && newValid {
					h.DeploymentAdded(newDeployment)
				} else if oldValid && !newValid {
					h.DeploymentDeleted(oldDeployment)
				} else if oldValid && newValid {
					h.DeploymentUpdated(oldDeployment, newDeployment)
				}
			}
		},
	}

}

func (manager *K8sResourceManager) WatchDeployments(stopper chan struct{}, handlers ...DeploymentEventHandler) {
	_, controller := cache.NewInformer(
		manager.watchListMap["deployments"],
		&v1beta1.Deployment{},
		time.Second*0,
		manager.getDeploymentEventHandler(handlers),
	)
	glog.Info("Start watching deployments")
	controller.Run(stopper)
	glog.Info("WatchDeployments terminated")
}

func (manager *K8sResourceManager) WatchStatefulSets(stopper chan struct{}, handlers ...DeploymentEventHandler) {
	_, controller := cache.NewInformer(
		manager.watchListMap["statefulsets"],
		&apps_v1beta1.StatefulSet{},
		time.Second*0,
		manager.getDeploymentEventHandler(handlers),
	)
	glog.Info("Start watching statefulSets")
	controller.Run(stopper)
	glog.Info("WatchStatefulSets terminated")
}

func (manager *K8sResourceManager) WatchDaemonSets(stopper chan struct{}, handlers ...DeploymentEventHandler) {
	_, controller := cache.NewInformer(
		manager.watchListMap["daemonsets"],
		&v1beta1.DaemonSet{},
		time.Second*0,
		manager.getDeploymentEventHandler(handlers),
	)
	glog.Info("Start watching daemonSets")
	controller.Run(stopper)
	glog.Info("WatchDaemonSets terminated")
}
