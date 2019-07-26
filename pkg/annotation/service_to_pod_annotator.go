package annotation

import (
	"github.com/golang/glog"
	envoy "github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

var (
	TRUE_STR  = "true"
	FALSE_STR = "false"

	//annotations also need to be applied on headless service pods
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

	//annotations also need to be applied on ingress pods
	ServiceAnnotations = []string{
		"traffic.tracing.enabled",
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
	annotations := map[string]*string{
		kubernetes.PodHeadlessByService(svc.Name()): nil,
	}
	for _, key := range HeadlessServiceAnnotations {
		if svc.Labels[key] != "" {
			podKey := kubernetes.PodKeyByService(svc.Name(), key[len("traffic."):])
			annotations[podKey] = nil
		}
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

	for _, port := range svc.Ports {
		svc_key := kubernetes.ServicePortProtocol(port.Port)
		protocol := svc.Labels[svc_key]
		if protocol != "" {
			key := kubernetes.PodPortProtcolByService(svc.Name(), port.Port)
			annotations[key] = &protocol
		}

	}

	for _, key := range ServiceAnnotations {
		if svc.Labels[key] != "" {
			headlessValue := svc.Labels[key]
			podKey := kubernetes.PodKeyByService(svc.Name(), key[len("traffic."):])
			annotations[podKey] = &headlessValue
		}
	}

	if svc.ClusterIP == "None" {
		key := kubernetes.PodHeadlessByService(svc.Name())
		annotations[key] = &TRUE_STR

		for _, key := range HeadlessServiceAnnotations {
			if svc.Labels[key] != "" {
				headlessValue := svc.Labels[key]
				podKey := kubernetes.PodKeyByService(svc.Name(), key[len("traffic."):])
				annotations[podKey] = &headlessValue
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
