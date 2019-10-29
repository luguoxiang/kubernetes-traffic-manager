package kubernetes

import (
	"github.com/golang/glog"
	v1beta1 "k8s.io/api/extensions/v1beta1"

	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
)

type IngressEventHandler interface {
	IngressValid(ingressInfo *IngressInfo) bool
	IngressAdded(ingressInfo *IngressInfo)
	IngressDeleted(ingressInfo *IngressInfo)
	IngressUpdated(oldIngress, newIngress *IngressInfo)
}

func (manager *K8sResourceManager) getIngressEventHandler(handlers []IngressEventHandler) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingressInfo := NewIngressInfo(obj.(*v1beta1.Ingress))
			manager.Lock()
			defer manager.Unlock()

			for _, h := range handlers {
				if h.IngressValid(ingressInfo) {
					h.IngressAdded(ingressInfo)
				}
			}

		},
		DeleteFunc: func(obj interface{}) {
			ingressInfo := NewIngressInfo(obj.(*v1beta1.Ingress))

			manager.Lock()
			defer manager.Unlock()

			for _, h := range handlers {
				if h.IngressValid(ingressInfo) {
					h.IngressDeleted(ingressInfo)
				}
			}

		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldIngress := NewIngressInfo(oldObj.(*v1beta1.Ingress))
			newIngress := NewIngressInfo(newObj.(*v1beta1.Ingress))
			if reflect.DeepEqual(oldIngress, newIngress) {
				return
			}

			manager.Lock()
			defer manager.Unlock()
			for _, h := range handlers {
				oldValid := h.IngressValid(oldIngress)
				newValid := h.IngressValid(newIngress)
				if !oldValid && newValid {
					h.IngressAdded(newIngress)
				} else if oldValid && !newValid {
					h.IngressDeleted(oldIngress)
				} else if oldValid && newValid {
					h.IngressUpdated(oldIngress, newIngress)
				}
			}
		},
	}

}

func (manager *K8sResourceManager) WatchIngresss(stopper chan struct{}, handlers ...IngressEventHandler) {
	_, controller := cache.NewInformer(
		manager.watchListMap["ingresses"],
		&v1beta1.Ingress{},
		time.Second*0,
		manager.getIngressEventHandler(handlers),
	)
	glog.Info("Start watching ingresses")
	controller.Run(stopper)
	glog.Info("WatchIngresss terminated")
}
