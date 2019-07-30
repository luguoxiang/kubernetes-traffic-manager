package annotation

import (
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/endpoint"
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
	var annotationKeys []string
	for key, _ := range pod.Annotations {
		if kubernetes.AnnotationHasDeploymentLabel(key) {
			annotationKeys = append(annotationKeys, key)
		}
	}

	if len(annotationKeys) == 0 {
		return
	}

	err := annotator.k8sManager.RemovePodAnnotation(pod, annotationKeys)
	if err != nil {
		glog.Errorf("Remove Pod %s Annotations %v failed: %s", pod.Name(), annotationKeys, err.Error())
	}
}

func (annotator *DeploymentToPodAnnotator) addDeploymentAnnotateToPod(pod *kubernetes.PodInfo, deployment *kubernetes.DeploymentInfo) {
	annotations := make(map[string]string)

	for key, _ := range pod.Annotations {
		if kubernetes.AnnotationHasDeploymentLabel(key) {
			//ensure non-exists deployment annotation being removed
			//existings annotations will be override later
			annotations[key] = ""
		}
	}

	//propagate deployment labels to pod
	for key, value := range deployment.Labels {
		if endpoint.NeedDeploymentToPodAnnotation(key) {
			podKey := kubernetes.DeploymentLabelToPodAnnotation(key)
			annotations[podKey] = value
		}
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
		annotator.addDeploymentAnnotateToPod(pod, deployment)
	}
}

func (*DeploymentToPodAnnotator) PodDeleted(pod *kubernetes.PodInfo) {
	//ignore
}
func (annotator *DeploymentToPodAnnotator) PodUpdated(oldPod, newPod *kubernetes.PodInfo) {
	//ignore
}

func (annotator *DeploymentToPodAnnotator) DeploymentValid(deployment *kubernetes.DeploymentInfo) bool {
	return true
}
func (annotator *DeploymentToPodAnnotator) DeploymentAdded(deployment *kubernetes.DeploymentInfo) {
	for _, resource := range annotator.k8sManager.GetMatchedResources(deployment, kubernetes.POD_TYPE) {
		pod := resource.(*kubernetes.PodInfo)
		annotator.addDeploymentAnnotateToPod(pod, deployment)
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
