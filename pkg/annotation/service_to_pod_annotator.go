package annotation

import (
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

var (
	TRUE_STR                   = "true"
	FALSE_STR                  = "false"
	HeadlessServiceAnnotations = []string{
		"traffic.connection.timeout",
		"traffic.retries.max",
		"traffic.connection.max",
		"traffic.request.max-pending"}
)

type ServiceToPodAnnotator struct {
	k8sManager *kubernetes.K8sResourceManager
}

func NewServiceToPodAnnotator(k8sManager *kubernetes.K8sResourceManager) *ServiceToPodAnnotator {
	return &ServiceToPodAnnotator{
		k8sManager: k8sManager,
	}
}

func (pa *ServiceToPodAnnotator) PodValid(pod *kubernetes.PodInfo) bool {
	return true
}

func (pa *ServiceToPodAnnotator) removeServiceAnnotationToPod(pod *kubernetes.PodInfo, svc *kubernetes.ServiceInfo) {
	annotations := map[string]*string{
		kubernetes.PodHeadlessByService(svc.Name()): nil,
	}
	for _, port := range svc.Ports {
		key := kubernetes.PodPortProtcolByService(svc.Name(), port.Port)
		annotations[key] = nil
	}
	err := pa.k8sManager.UpdatePodAnnotation(pod, annotations)
	if err != nil {
		glog.Infof("Annotate pod %s with %v failed: %s", pod.Name(), annotations, err.Error())
	}
}

func (pa *ServiceToPodAnnotator) addServiceAnnotationToPod(pod *kubernetes.PodInfo, svc *kubernetes.ServiceInfo) {
	annotations := map[string]*string{}
	var key string
	for _, port := range svc.Ports {
		var value string
		svc_key := kubernetes.ServicePortProtocol(port.Port)
		if svc.Labels[svc_key] != "" {
			value = svc.Labels[svc_key]
		} else {
			value = "tcp"
		}
		key = kubernetes.PodPortProtcolByService(svc.Name(), port.Port)
		annotations[key] = &value
	}

	if svc.ClusterIP == "None" {
		key = kubernetes.PodHeadlessByService(svc.Name())
		annotations[key] = &TRUE_STR

		for _, key := range HeadlessServiceAnnotations {
			if svc.Labels[key] != "" {
				value := svc.Labels[key]
				annotations[key] = &value
			}
		}
	}

	if len(annotations) == 0 {
		return
	}
	err := pa.k8sManager.UpdatePodAnnotation(pod, annotations)
	if err != nil {
		glog.Infof("Annotate pod %s with %v failed: %s", pod.Name(), annotations, err.Error())
	}
}

func (pa *ServiceToPodAnnotator) PodAdded(pod *kubernetes.PodInfo) {

	for _, resource := range pa.k8sManager.GetMatchedResources(pod, kubernetes.SERVICE_TYPE) {
		svc := resource.(*kubernetes.ServiceInfo)
		if pod.EnvoyEnabled() {
			pa.addServiceAnnotationToPod(pod, svc)
		}
	}
}

func (*ServiceToPodAnnotator) PodDeleted(pod *kubernetes.PodInfo) {
	//ignore
}
func (pa *ServiceToPodAnnotator) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	pa.PodAdded(newPod)
}

func (manager *ServiceToPodAnnotator) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return true
}

func (pa *ServiceToPodAnnotator) ServiceAdded(svc *kubernetes.ServiceInfo) {
	for _, resource := range pa.k8sManager.GetMatchedResources(svc, kubernetes.POD_TYPE) {
		pod := resource.(*kubernetes.PodInfo)
		if pod.EnvoyEnabled() {
			pa.addServiceAnnotationToPod(pod, svc)
		}
	}
}
func (pa *ServiceToPodAnnotator) ServiceDeleted(svc *kubernetes.ServiceInfo) {
	for _, resource := range pa.k8sManager.GetMatchedResources(svc, kubernetes.POD_TYPE) {
		pod := resource.(*kubernetes.PodInfo)
		pa.removeServiceAnnotationToPod(pod, svc)
	}
}

func (pa *ServiceToPodAnnotator) ServiceUpdated(oldService, newService *kubernetes.ServiceInfo) {
	pa.ServiceAdded(newService)
}
