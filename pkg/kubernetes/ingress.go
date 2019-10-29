package kubernetes

import (
	"fmt"
	v1beta1 "k8s.io/api/extensions/v1beta1"
)

type IngressClusterInfo struct {
	Service string
	Port    uint32
}

type IngressInfo struct {
	name            string
	namespace       string
	ResourceVersion string

	DefaultCluster       *IngressClusterInfo
	HostPathToClusterMap map[string]map[string]*IngressClusterInfo
}

func NewIngressInfo(ingress *v1beta1.Ingress) *IngressInfo {
	hostPathToClusterMap := make(map[string]map[string]*IngressClusterInfo)
	var defaultCluster *IngressClusterInfo
	if ingress.Spec.Backend != nil {
		defaultCluster = &IngressClusterInfo{
			Service: ingress.Spec.Backend.ServiceName,
			Port:    uint32(ingress.Spec.Backend.ServicePort.IntVal),
		}
	}
	for _, rule := range ingress.Spec.Rules {
		pathRule := make(map[string]*IngressClusterInfo)

		for _, cluster := range rule.HTTP.Paths {
			pathRule[cluster.Path] = &IngressClusterInfo{
				Service: cluster.Backend.ServiceName,
				Port:    uint32(cluster.Backend.ServicePort.IntVal),
			}
		}
		if rule.Host == "" {
			hostPathToClusterMap["*"] = pathRule
		} else {
			hostPathToClusterMap[rule.Host] = pathRule
		}
	}
	return &IngressInfo{
		DefaultCluster:       defaultCluster,
		HostPathToClusterMap: hostPathToClusterMap,
		namespace:            ingress.Namespace,
		name:                 ingress.Name,
		ResourceVersion:      ingress.ResourceVersion,
	}
}

func (ingress *IngressInfo) GetSelector() map[string]string {
	return nil
}

func (ingress *IngressInfo) Namespace() string {
	return ingress.namespace
}

func (ingress *IngressInfo) Name() string {
	return ingress.name
}

func (ingress *IngressInfo) Type() ResourceType {
	return INGRESS_TYPE
}

func (ingress *IngressInfo) String() string {
	return fmt.Sprintf("Ingress %s@%s",
		ingress.name, ingress.namespace)
}
