package kubernetes

import (
	"bytes"
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

type ServicePortInfo struct {
	Port       uint32
	TargetPort uint32
	Name       string
}
type ServiceInfo struct {
	ResourceVersion string
	name            string
	namespace       string
	ClusterIP       string
	selector        map[string]string
	Labels          map[string]string
	Annotations     map[string]string
	Ports           []*ServicePortInfo
}

func (service *ServiceInfo) Type() ResourceType {
	return SERVICE_TYPE
}

func (service *ServiceInfo) IsKubeAPIService() bool {
	return service.Name() == "kubernetes" && service.Namespace() == "default"
}

func (service *ServiceInfo) OutboundEnabled() bool {
	if service.EnvoyEnabled() || service.IsKubeAPIService() {
		return true
	}

	if service.Labels != nil {
		return strings.EqualFold(service.Labels[OUTBOUND_ENABLED], "true")
	}
	return false
}

func (service *ServiceInfo) EnvoyEnabled() bool {
	if service.Labels != nil {
		return strings.EqualFold(service.Labels[ENVOY_ENABLED], "true")
	}
	return false
}

func (service *ServiceInfo) IsHttp(port uint32) bool {
	key := AnnotationPortProtocol(port)
	if service.Annotations != nil {
		return strings.EqualFold(service.Annotations[key], "http")
	}
	return false
}

func (service *ServiceInfo) Name() string {
	return service.name
}

func (service *ServiceInfo) Namespace() string {
	return service.namespace
}

func (service *ServiceInfo) GetSelector() map[string]string {
	return service.selector
}

func (service *ServiceInfo) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("Service %s@%s EnvoyEnabled=%v,Port=", service.name, service.namespace, service.EnvoyEnabled()))
	for _, port := range service.Ports {
		buffer.WriteString(fmt.Sprintf("%d", port.Port))
		buffer.WriteString(" ")
	}

	return buffer.String()
}

func NewServiceInfo(service *v1.Service) *ServiceInfo {

	info := &ServiceInfo{
		name:            service.Name,
		namespace:       service.Namespace,
		selector:        service.Spec.Selector,
		Labels:          service.Labels,
		ClusterIP:       service.Spec.ClusterIP,
		Annotations:     service.Annotations,
		ResourceVersion: service.ResourceVersion,
	}
	for _, port := range service.Spec.Ports {
		var targetPort uint32
		if port.TargetPort.IntVal > 0 {
			targetPort = uint32(port.TargetPort.IntVal)
		}

		info.Ports = append(info.Ports, &ServicePortInfo{
			Name:       port.Name,
			Port:       uint32(port.Port),
			TargetPort: targetPort,
		})
	}

	return info

}

func (manager *K8sResourceManager) UpdateServiceAnnotation(serviceInfo *ServiceInfo, annotation map[string]*string) error {
	var err error
	var rawService *v1.Service
	for i := 0; i < 3; i++ {
		rawService, err = manager.clientSet.CoreV1().Services(serviceInfo.Namespace()).Get(serviceInfo.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changed := false
		for k, v := range annotation {
			if v != nil && rawService.Annotations == nil {
				rawService.Annotations = make(map[string]string)
			}
			current, ok := rawService.Annotations[k]
			if v == nil && ok {
				delete(rawService.Annotations, k)
				changed = true
			}
			if v != nil && current != *v {
				rawService.Annotations[k] = *v
				changed = true
			}
		}
		if !changed {
			return nil
		}
		_, err = manager.clientSet.CoreV1().Services(serviceInfo.Namespace()).Update(rawService)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func (manager *K8sResourceManager) AnnotatePortHTTP(info *ServiceInfo, port *ServicePortInfo) {
	annotationName := AnnotationPortProtocol(port.Port)

	if strings.TrimSpace(info.Annotations[annotationName]) != "" {
		//if the service is already annotated, do not replace the annotation
		return
	}
	manager.Lock()
	defer manager.Unlock()

	protocol := "http"
	go manager.UpdateServiceAnnotation(info, map[string]*string{
		annotationName: &protocol,
	})
}
