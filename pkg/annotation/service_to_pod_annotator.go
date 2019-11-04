package annotation

import (
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/listener"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
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
	return pod.Valid()
}

func (pa *ServiceToPodAnnotator) removeServiceAnnotationToPod(pod *kubernetes.PodInfo, svc *kubernetes.ServiceInfo) {
	var annotationKeys []string
	for key, _ := range pod.Annotations {
		if kubernetes.AnnotationHasServiceLabel(svc.Name(), key) {
			annotationKeys = append(annotationKeys, key)
		}
	}

	if len(annotationKeys) == 0 {
		return
	}

	err := pa.k8sManager.RemovePodAnnotation(pod, annotationKeys)
	if err != nil {
		glog.Errorf("Remove Pod %s Annotations %v failed: %s", pod.Name(), annotationKeys, err.Error())
	}
}

func (pa *ServiceToPodAnnotator) addServiceAnnotationToPod(pod *kubernetes.PodInfo, svc *kubernetes.ServiceInfo) {
	annotations := make(map[string]string)

	for key, _ := range pod.Annotations {
		if kubernetes.AnnotationHasServiceLabel(svc.Name(), key) {
			//ensure non-exists service annotation being removed
			//existing service annotation will be overided later
			annotations[key] = ""
		}
	}

	for _, port := range svc.Ports {
		svc_key := kubernetes.ServicePortProtocol(port.Port)
		protocol := svc.Labels[svc_key]
		if protocol == "" {
			//annotated by ingress_lds
			protocol = svc.Annotations[svc_key]
		}
		if protocol != "" {
			key := kubernetes.PodPortProtcolByService(svc.Name(), port.Port)
			annotations[key] = protocol

			key = kubernetes.PodTargetPortProtcolByService(svc.Name(), port.TargetPort)
			annotations[key] = protocol
		}

	}

	for key, value := range svc.Labels {
		if value == "" {
			continue
		}
		if cluster.NeedServiceToPodAnnotation(key) || listener.NeedServiceToPodAnnotation(key) {
			podKey := kubernetes.ServiceLabelToPodAnnotation(svc.Name(), key)
			annotations[podKey] = value
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
		pa.addServiceAnnotationToPod(pod, svc)
	}
}

func (*ServiceToPodAnnotator) PodDeleted(pod *kubernetes.PodInfo) {
	//ignore
}
func (pa *ServiceToPodAnnotator) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	//ignore
}

func (manager *ServiceToPodAnnotator) ServiceValid(svc *kubernetes.ServiceInfo) bool {
	return true
}

func (pa *ServiceToPodAnnotator) ServiceAdded(svc *kubernetes.ServiceInfo) {
	for _, resource := range pa.k8sManager.GetMatchedResources(svc, kubernetes.POD_TYPE) {
		pod := resource.(*kubernetes.PodInfo)
		pa.addServiceAnnotationToPod(pod, svc)
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
