package kubernetes

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
)

type ServiceEventHandler interface {
	ServiceValid(info *ServiceInfo) bool
	ServiceAdded(svc *ServiceInfo)
	ServiceDeleted(svc *ServiceInfo)
	ServiceUpdated(oldService, newService *ServiceInfo)
}

func (manager *K8sResourceManager) ServiceValid(info *ServiceInfo) bool {
	return true
}

func (manager *K8sResourceManager) ServiceAdded(info *ServiceInfo) {
	manager.addResource(info)

}

func (manager *K8sResourceManager) ServiceDeleted(info *ServiceInfo) {
	manager.removeResource(info)
}

func (manager *K8sResourceManager) ServiceUpdated(oldService, newService *ServiceInfo) {
	manager.ServiceDeleted(oldService)
	manager.ServiceAdded(newService)
}

func (manager *K8sResourceManager) WatchServices(stopper chan struct{}, handlers ...ServiceEventHandler) {
	watchlist := cache.NewListWatchFromClient(
		manager.clientSet.Core().RESTClient(), "services", "",
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Service{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				service := NewServiceInfo(obj.(*v1.Service))

				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					if h.ServiceValid(service) {
						h.ServiceAdded(service)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				service := NewServiceInfo(obj.(*v1.Service))

				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					if h.ServiceValid(service) {
						h.ServiceDeleted(service)
					}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldService := NewServiceInfo(oldObj.(*v1.Service))
				newService := NewServiceInfo(newObj.(*v1.Service))

				newVersion := newService.ResourceVersion
				//ignore ResourceVersion diff
				newService.ResourceVersion = oldService.ResourceVersion
				if reflect.DeepEqual(oldService, newService) {
					return
				}

				newService.ResourceVersion = newVersion
				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					oldValid := h.ServiceValid(oldService)
					newValid := h.ServiceValid(newService)
					if !oldValid && newValid {
						h.ServiceAdded(newService)
					} else if oldValid && !newValid {
						h.ServiceDeleted(oldService)
					} else if oldValid && newValid {
						h.ServiceUpdated(oldService, newService)
					}
				}
			},
		},
	)
	controller.Run(stopper)
}
