package annotation

import (
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	envoy "github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/listener"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

var (

	//annotations need to be applied on headless service pods
	HeadlessServiceAnnotations = []string{
		"traffic.connection.timeout",
		"traffic.retries.max",
		"traffic.connection.max",
		"traffic.request.max-pending",

		"traffic.request.timeout",
		"traffic.retries.5xx",
		"traffic.retries.connect-failure",
		"traffic.retries.gateway-error",
		"traffic.fault.delay.time",
		"traffic.fault.delay.percentage",
		"traffic.fault.abort.status",
		"traffic.fault.abort.percentage",
		"traffic.rate.limit",
	}
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

	for _, port := range svc.Ports {
		svc_key := kubernetes.ServicePortProtocol(port.Port)
		protocol := svc.Labels[svc_key]
		if protocol != "" {
			key := kubernetes.PodPortProtcolByService(svc.Name(), port.Port)
			annotations[key] = protocol
		}

	}
	headless := false
	if svc.ClusterIP == "None" {
		key := kubernetes.PodHeadlessByService(svc.Name())
		annotations[key] = "true"
		headless = true
	}
	for key, value := range svc.Labels {
		if value == "" {
			continue
		}
		if cluster.NeedServiceToPodAnnotation(key, headless) || listener.NeedServiceToPodAnnotation(key, headless) {
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
	if svc.IsKubeAPIService() {
		for _, port := range svc.Ports {
			key := kubernetes.ServicePortProtocol(port.Port)
			pa.k8sManager.AddServiceLabel(svc, key, envoy.PROTO_DIRECT)
		}

		return
	}
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
