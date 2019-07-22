package kubernetes

import (
	"fmt"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"strings"
)

type DeploymentInfo struct {
	name        string
	namespace   string
	realType    string
	selector    map[string]string
	Labels      map[string]string
	Ports       []uint32
	HostNetwork bool
}

func (deployment *DeploymentInfo) EnvoyEnabled() bool {
	if deployment.Labels != nil {
		return strings.EqualFold(deployment.Labels[ENVOY_ENABLED], "true")
	}
	return false
}

func (deployment *DeploymentInfo) String() string {
	return fmt.Sprintf("%s %s@%s, EnvoyEnabled=%v", deployment.realType, deployment.name, deployment.namespace, deployment.EnvoyEnabled())
}

func (deployment *DeploymentInfo) GetSelector() map[string]string {
	return deployment.selector
}

func (deployment *DeploymentInfo) Type() ResourceType {
	return DEPLOYMENT_TYPE
}

func (deployment *DeploymentInfo) Name() string {
	return deployment.name
}

func (deployment *DeploymentInfo) Namespace() string {
	return deployment.namespace
}

func (deployment *DeploymentInfo) addPort(addedPort uint32) bool {
	for _, port := range deployment.Ports {
		if addedPort == port {
			return false
		}
	}
	deployment.Ports = append(deployment.Ports, addedPort)
	return true
}

func NewDeploymentInfo(obj interface{}) *DeploymentInfo {
	var result *DeploymentInfo
	var template *v1.PodTemplateSpec

	switch deployment := obj.(type) {
	case *v1beta1.Deployment:
		result = &DeploymentInfo{
			name:      deployment.Name,
			namespace: deployment.Namespace,
			realType:  "Deployment",
			Labels:    deployment.Labels,
			selector:  deployment.Spec.Selector.MatchLabels,
		}

		template = &deployment.Spec.Template
	case *v1beta1.DaemonSet:
		result = &DeploymentInfo{
			name:      deployment.Name,
			namespace: deployment.Namespace,
			realType:  "DaemonSet",
			Labels:    deployment.Labels,
			selector:  deployment.Spec.Selector.MatchLabels,
		}
		template = &deployment.Spec.Template
	case *apps_v1beta1.StatefulSet:
		result = &DeploymentInfo{
			name:      deployment.Name,
			namespace: deployment.Namespace,
			realType:  "StatefulSet",
			Labels:    deployment.Labels,
			selector:  deployment.Spec.Selector.MatchLabels,
		}
		template = &deployment.Spec.Template
	default:
		panic(fmt.Sprintf("Unexpected type %T", obj))
		return nil
	}

	for _, container := range template.Spec.Containers {
		for _, port := range container.Ports {
			result.addPort(uint32(port.ContainerPort))
		}
	}
	result.HostNetwork = template.Spec.HostNetwork
	return result
}
