package annotation

import (
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type DeploymentToPodAnnotator struct {
	k8sManager *kubernetes.K8sResourceManager
}

func NewDeploymentToPodAnnotator(k8sManager *kubernetes.K8sResourceManager) *DeploymentToPodAnnotator {
	return &DeploymentToPodAnnotator{
		k8sManager: k8sManager,
	}
}

func (annotator *DeploymentToPodAnnotator) PodValid(pod *kubernetes.PodInfo) bool {
	return true
}

func (annotator *DeploymentToPodAnnotator) removeDeploymentAnnotateToPod(pod *kubernetes.PodInfo, deployment *kubernetes.DeploymentInfo) {
	annotations := map[string]*string{
		kubernetes.ENVOY_ENABLED_BY_DEPLOYMENT: nil,
		kubernetes.ENDPOINT_WEIGHT:             nil,
	}
	err := annotator.k8sManager.UpdatePodAnnotation(pod, annotations)
	if err != nil {
		glog.Infof("Annotate pod %s with %v failed: %s", pod.Name(), annotations, err.Error())
	}
}

func (annotator *DeploymentToPodAnnotator) addDeploymentAnnotateToPod(pod *kubernetes.PodInfo, deployment *kubernetes.DeploymentInfo) {
	annotations := map[string]*string{}

	//propagate deployment labels to pod
	value1 := deployment.Labels[kubernetes.ENDPOINT_WEIGHT]
	if value1 != "" {
		annotations[kubernetes.ENDPOINT_WEIGHT] = &value1
	}

	value2 := deployment.Labels[kubernetes.ENVOY_ENABLED]
	if value2 != "" {
		annotations[kubernetes.ENVOY_ENABLED_BY_DEPLOYMENT] = &value2
	}

	if len(annotations) == 0 {
		return
	}
	err := annotator.k8sManager.UpdatePodAnnotation(pod, annotations)
	if err != nil {
		glog.Infof("Annotate pod %s with %v failed: %s", pod.Name(), annotations, err.Error())
	}
}

func (annotator *DeploymentToPodAnnotator) PodAdded(pod *kubernetes.PodInfo) {
	for _, resource := range annotator.k8sManager.GetMatchedResources(pod, kubernetes.DEPLOYMENT_TYPE) {
		deployment := resource.(*kubernetes.DeploymentInfo)
		if pod.EnvoyEnabled() || deployment.EnvoyEnabled() {
			annotator.addDeploymentAnnotateToPod(pod, deployment)
		}
	}
}

func (*DeploymentToPodAnnotator) PodDeleted(pod *kubernetes.PodInfo) {
	//ignore
}
func (annotator *DeploymentToPodAnnotator) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	annotator.PodAdded(newPod)
}

func (annotator *DeploymentToPodAnnotator) DeploymentValid(deployment *kubernetes.DeploymentInfo) bool {
	return true
}
func (annotator *DeploymentToPodAnnotator) DeploymentAdded(deployment *kubernetes.DeploymentInfo) {
	for _, resource := range annotator.k8sManager.GetMatchedResources(deployment, kubernetes.POD_TYPE) {
		pod := resource.(*kubernetes.PodInfo)
		if pod.EnvoyEnabled() || deployment.EnvoyEnabled() {
			annotator.addDeploymentAnnotateToPod(pod, deployment)
		}
	}
}
func (annotator *DeploymentToPodAnnotator) DeploymentDeleted(deployment *kubernetes.DeploymentInfo) {
	for _, resource := range annotator.k8sManager.GetMatchedResources(deployment, kubernetes.POD_TYPE) {
		pod := resource.(*kubernetes.PodInfo)
		annotator.removeDeploymentAnnotateToPod(pod, deployment)
	}
}
func (annotator *DeploymentToPodAnnotator) DeploymentUpdated(oldDeployment, newDeployment *kubernetes.DeploymentInfo) {
	annotator.DeploymentAdded(newDeployment)
}
