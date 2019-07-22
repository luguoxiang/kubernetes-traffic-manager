package annotation

import (
	"github.com/golang/glog"
	"github.com/luguoxiang/k8s-traffic-manager/pkg/kubernetes"
)

var (
	TRUE_STR  = "true"
	FALSE_STR = "false"
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
		kubernetes.PodHeadlessAnnotationByServiceKey(svc.Name()): nil,
		kubernetes.PodEnvoyAnnotationByServiceKey(svc.Name()):    nil,
	}
	for _, port := range svc.Ports {
		key := kubernetes.PodPortAnnotationByServiceKey(svc.Name(), port.Port)
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
		svc_key := kubernetes.AnnotationPortProtocol(port.Port)
		key = kubernetes.PodPortAnnotationByServiceKey(svc.Name(), port.Port)
		if svc.Annotations != nil && svc.Annotations[svc_key] != "" {
			value = svc.Annotations[svc_key]
		} else {
			value = "tcp"
		}
		annotations[key] = &value
	}

	if svc.ClusterIP == "None" {
		key = kubernetes.PodHeadlessAnnotationByServiceKey(svc.Name())
		annotations[key] = &TRUE_STR
	}
	key = kubernetes.PodEnvoyAnnotationByServiceKey(svc.Name())
	if svc.EnvoyEnabled() {
		annotations[key] = &TRUE_STR
	} else {
		annotations[key] = &FALSE_STR
	}

	if svc.Labels != nil {
		//propagate service labels to pod
		lvalue := svc.Labels[kubernetes.ENDPOINT_INBOUND_PODIP]
		if lvalue != "" {
			annotations[kubernetes.ENDPOINT_INBOUND_PODIP] = &lvalue
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
		if pod.EnvoyEnabled() || svc.OutboundEnabled() {
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
		if pod.EnvoyEnabled() || svc.OutboundEnabled() {
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
