package kubernetes

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"time"
)

const (
	SECRET_TLS_HOST = "traffic.tls.host"
)

type SecretInfo struct {
	Name            string
	Namespace       string
	Labels          map[string]string
	Data            map[string][]byte
	ResourceVersion string
}

func NewSecretInfo(secret *v1.Secret) *SecretInfo {
	return &SecretInfo{
		Name:            secret.Name,
		Namespace:       secret.Namespace,
		Labels:          secret.Labels,
		Data:            secret.Data,
		ResourceVersion: secret.ResourceVersion,
	}

}

type SecretEventHandler interface {
	SecretValid(info *SecretInfo) bool
	SecretAdded(svc *SecretInfo)
	SecretDeleted(svc *SecretInfo)
	SecretUpdated(oldSecret, newSecret *SecretInfo)
}

func (manager *K8sResourceManager) WatchSecrets(stopper chan struct{}, handlers ...SecretEventHandler) {
	watchlist := cache.NewListWatchFromClient(
		manager.ClientSet.Core().RESTClient(), "secrets", "",
		fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.Secret{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				secret := NewSecretInfo(obj.(*v1.Secret))

				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					if h.SecretValid(secret) {
						h.SecretAdded(secret)
					}
				}
			},
			DeleteFunc: func(obj interface{}) {
				secret := NewSecretInfo(obj.(*v1.Secret))

				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					if h.SecretValid(secret) {
						h.SecretDeleted(secret)
					}
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldSecret := NewSecretInfo(oldObj.(*v1.Secret))
				newSecret := NewSecretInfo(newObj.(*v1.Secret))

				newVersion := newSecret.ResourceVersion
				//ignore ResourceVersion diff
				newSecret.ResourceVersion = oldSecret.ResourceVersion
				if reflect.DeepEqual(oldSecret, newSecret) {
					return
				}

				newSecret.ResourceVersion = newVersion
				manager.Lock()
				defer manager.Unlock()

				for _, h := range handlers {
					oldValid := h.SecretValid(oldSecret)
					newValid := h.SecretValid(newSecret)
					if !oldValid && newValid {
						h.SecretAdded(newSecret)
					} else if oldValid && !newValid {
						h.SecretDeleted(oldSecret)
					} else if oldValid && newValid {
						h.SecretUpdated(oldSecret, newSecret)
					}
				}
			},
		},
	)
	controller.Run(stopper)
}
