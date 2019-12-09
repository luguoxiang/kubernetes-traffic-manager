package kubernetes

import (
	"fmt"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	"strings"
)

type IngressHostInfo struct {
	Host    string
	PathMap map[string]*IngressClusterInfo
	Secret  string
}

type IngressClusterInfo struct {
	Service string
	Port    uint32
	Path    string
}

type IngressInfo struct {
	name            string
	namespace       string
	ResourceVersion string

	HostPathToClusterMap map[string]*IngressHostInfo
}

func NewIngressInfo(ingress *v1beta1.Ingress) *IngressInfo {
	hostPathToClusterMap := map[string]*IngressHostInfo{
		"*": &IngressHostInfo{
			Host:    "*",
			PathMap: map[string]*IngressClusterInfo{},
		},
	}

	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			hostPathToClusterMap[rule.Host] = &IngressHostInfo{
				Host:    rule.Host,
				PathMap: map[string]*IngressClusterInfo{},
			}
		}
	}
	if ingress.Spec.Backend != nil {
		defaultCluster := &IngressClusterInfo{
			Path:    "/",
			Service: ingress.Spec.Backend.ServiceName,
			Port:    uint32(ingress.Spec.Backend.ServicePort.IntVal),
		}
		for _, hostInfo := range hostPathToClusterMap {
			hostInfo.PathMap["/"] = defaultCluster
		}
	}

	for _, tls := range ingress.Spec.TLS {
		for _, host := range tls.Hosts {
			hostInfo := hostPathToClusterMap[host]
			if hostInfo != nil {
				hostInfo.Secret = tls.SecretName
				if strings.Index(hostInfo.Secret, ".") < 0 {
					hostInfo.Secret = fmt.Sprintf("%s.%s", hostInfo.Secret, ingress.Namespace)
				}
			}
		}
	}
	for _, rule := range ingress.Spec.Rules {
		host := rule.Host
		if host == "" {
			host = "*"
		}
		hostInfo := hostPathToClusterMap[host]

		for _, cluster := range rule.HTTP.Paths {
			path := cluster.Path
			if path == "" {
				path = "/"
			}
			hostInfo.PathMap[path] = &IngressClusterInfo{
				Path:    path,
				Service: cluster.Backend.ServiceName,
				Port:    uint32(cluster.Backend.ServicePort.IntVal),
			}
		}

	}

	return &IngressInfo{
		HostPathToClusterMap: hostPathToClusterMap,
		namespace:            ingress.Namespace,
		name:                 ingress.Name,
		ResourceVersion:      ingress.ResourceVersion,
	}
}

func (ingress *IngressInfo) GetServiceAnnotations(hostInfo *IngressHostInfo, clusterInfo *IngressClusterInfo) map[string]string {
	return map[string]string{
		IngressAttrLabel(clusterInfo.Port, "config"): fmt.Sprintf("%s@%s", clusterInfo.Path, hostInfo.Host),
		IngressAttrLabel(clusterInfo.Port, "secret"): hostInfo.Secret,
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
