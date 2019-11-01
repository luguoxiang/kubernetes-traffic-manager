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

func (service *ServiceInfo) Protocol(port uint32) int {
	key := ServicePortProtocol(port)
	return GetProtocol(service.Labels[key])
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
	buffer.WriteString(fmt.Sprintf("Service %s@%s Port=", service.name, service.namespace))
	for _, port := range service.Ports {
		key := ServicePortProtocol(port.Port)
		buffer.WriteString(fmt.Sprintf("%d:%s", port.Port, service.Annotations[key]))
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
	if info.Labels == nil {
		info.Labels = map[string]string{}
	}
	if info.Annotations == nil {
		info.Annotations = map[string]string{}
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

func (manager *K8sResourceManager) AddServiceLabel(serviceInfo *ServiceInfo, key string, value string) error {
	var err error
	var rawService *v1.Service
	for i := 0; i < 3; i++ {
		rawService, err = manager.ClientSet.CoreV1().Services(serviceInfo.Namespace()).Get(serviceInfo.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		if rawService.Labels[key] == value {
			return nil
		}
		rawService.Labels[key] = value

		_, err = manager.ClientSet.CoreV1().Services(serviceInfo.Namespace()).Update(rawService)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func (manager *K8sResourceManager) MergeServiceAnnotation(name string, ns string, key string, value string) error {
	var err error
	var rawService *v1.Service
	for i := 0; i < 3; i++ {
		rawService, err = manager.ClientSet.CoreV1().Services(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if rawService.Annotations == nil || rawService.Annotations[key] == "" {
			rawService.Annotations = map[string]string{key: value}
		} else {
			items := strings.Split(rawService.Annotations[key], ",")
			for _, item := range items {
				if item == value {
					return nil
				}
			}
			items = append(items, value)

			rawService.Annotations[key] = strings.Join(items, ",")
		}
		_, err = manager.ClientSet.CoreV1().Services(ns).Update(rawService)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func (manager *K8sResourceManager) RemoveServiceAnnotation(name string, ns string, key string, value string) error {
	var err error
	var rawService *v1.Service
	for i := 0; i < 3; i++ {
		rawService, err = manager.ClientSet.CoreV1().Services(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if rawService.Annotations == nil {
			return nil
		} else {
			var changed bool
			var result []string
			for _, item := range strings.Split(rawService.Annotations[key], ",") {
				if item == value {
					changed = true
					continue
				}
				result = append(result, item)
			}

			if !changed {
				return nil
			}
			rawService.Annotations[key] = strings.Join(result, ",")
		}
		_, err = manager.ClientSet.CoreV1().Services(ns).Update(rawService)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}
