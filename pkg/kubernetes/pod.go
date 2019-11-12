package kubernetes

import (
	"fmt"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
	"time"
)

type PodInfo struct {
	ResourceVersion string
	name            string
	namespace       string
	PodIP           string
	HostIP          string
	HostNetwork     bool
	Labels          map[string]string
	Annotations     map[string]string
	Containers      []string
}

func (pod *PodInfo) Valid() bool {
	return !pod.HostNetwork && pod.PodIP != ""
}
func (pod *PodInfo) EnvoyDockerId() string {
	return pod.Annotations[ENVOY_PROXY_ANNOTATION]
}

func (pod *PodInfo) NodeId() string {
	return fmt.Sprintf("%s.%s", pod.Name(), pod.Namespace())
}

func (pod *PodInfo) EnvoyEnabled() bool {
	if pod.Labels[ENVOY_ENABLED] != "" {
		//ENVOY_ENABLED label will overide annotation set by deployment label
		return GetLabelValueBool(pod.Labels[ENVOY_ENABLED])
	}
	return GetLabelValueBool(pod.Annotations[ENVOY_ENABLED_BY_DEPLOYMENT])
}

type PodPortInfo struct {
	Protocol int
	//Service   string
	ConfigMap map[string]string
}

func getPort(value string) uint32 {
	port, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(port)
}

func getServiceAndPort(annotation string) (string, uint32) {
	tokens := strings.Split(annotation, ".")
	if len(tokens) < 5 || tokens[0] != "traffic" || tokens[1] != "svc" || tokens[2] == "" || tokens[3] != "port" {
		return "", 0
	}
	port := getPort(tokens[4])
	if port == 0 {
		return "", 0
	}
	return tokens[2], port
}

/**
 * Used for k8s non-headless service.
 * LDS should create a listener for each clusterip:port of the service
 * CDS should create a service cluster for each clusterip:port of the service
 * example:
 * traffic.svc.service1.port.1234=http
 * should return map 1234 => service1 => true
 */

func (pod *PodInfo) GetPortSet() map[uint32]map[string]bool {
	result := make(map[uint32]map[string]bool)
	for k, v := range pod.Annotations {
		if v == "" {
			continue
		}
		service, port := getServiceAndPort(k)
		if port == 0 {
			continue
		}
		if result[port] == nil {
			result[port] = map[string]bool{
				service: true,
			}
		} else {
			result[port][service] = true
		}
	}

	for k, v := range pod.Labels {
		if v == "" {
			continue
		}
		tokens := strings.Split(k, ".")
		if len(tokens) < 3 || tokens[0] != "traffic" || tokens[1] != "port" {
			continue
		}
		port := getPort(tokens[2])
		if port == 0 {
			continue
		}
		if result[port] == nil {
			result[port] = map[string]bool{}
		}
	}
	return result
}

func (pod *PodInfo) collectTargetPort(configMap map[string]string, result map[uint32]*PodPortInfo) {
	for k, v := range configMap {
		tokens := strings.Split(k, ".")
		if len(tokens) != 4 || tokens[0] != "traffic" || tokens[1] != "target" || tokens[2] != "port" {
			continue
		}
		protocol := GetProtocol(v)
		if protocol < 0 {
			continue
		}
		port := getPort(tokens[3])
		if port == 0 {
			continue
		}

		portInfo := result[port]
		if portInfo == nil {
			portInfo = &PodPortInfo{
				ConfigMap: make(map[string]string),
				Protocol:  protocol,
			}
			result[port] = portInfo
		} else {
			if protocol > portInfo.Protocol {
				portInfo.Protocol = protocol
			}
		}

		//if the port has two services's annotations,merge their config
		for k1, v1 := range configMap {
			if !strings.HasPrefix(k1, "traffic.") {
				continue
			}
			if strings.HasPrefix(k1, "traffic.port.") {
				continue
			}
			if strings.HasPrefix(k1, "traffic.target.port.") {
				continue
			}
			portInfo.ConfigMap[k1] = v1
		}

	}
}

/**
 * Used for k8s headless service.
 * LDS should create a listener for each podip:targetPort of the service
 * CDS should create a static cluster for each podip:targetPort of the service
 * example:
 * traffic.svc.service1.attr=value
 * traffic.svc.service1.target.port.5678=http
 * should return map 5678 => PodPortInfo{PROTO_HTTP, traffic.attr=value }
 */
func (pod *PodInfo) GetTargetPortConfig() map[uint32]*PodPortInfo {
	serviceConfig := make(map[string]map[string]string)

	for k, v := range pod.Annotations {
		if v == "" {
			continue
		}
		tokens := strings.Split(k, ".")
		if len(tokens) < 4 || tokens[0] != "traffic" || tokens[1] != "svc" || tokens[2] == "" {
			continue
		}
		service := tokens[2]

		configMap := serviceConfig[service]
		if configMap == nil {
			configMap = make(map[string]string)
			serviceConfig[service] = configMap
		}

		newKey := "traffic" + k[len("traffic.svc.")+len(service):]
		configMap[newKey] = v
	}
	result := make(map[uint32]*PodPortInfo)

	for _, configMap := range serviceConfig {
		pod.collectTargetPort(configMap, result)
	}
	pod.collectTargetPort(pod.Labels, result)
	return result
}

func (pod *PodInfo) GetSelector() map[string]string {
	return pod.Labels
}

func (pod *PodInfo) Type() ResourceType {
	return POD_TYPE
}

func (pod *PodInfo) Name() string {
	return pod.name
}

func (pod *PodInfo) Namespace() string {
	return pod.namespace
}

func (pod *PodInfo) String() string {
	return fmt.Sprintf("Pod %s@%s,EnvoyEnabled=%v",
		pod.name, pod.namespace,
		pod.EnvoyEnabled())
}

func (pod *PodInfo) IsSkip() bool {
	return pod.namespace == "kube-system"
}

func NewPodInfo(pod *v1.Pod) *PodInfo {
	if pod.Status.PodIP == "" {
		return nil
	}

	var containers []string
	for _, container := range pod.Status.ContainerStatuses {
		id := container.ContainerID
		if strings.HasPrefix(id, "docker://") {
			id = id[9:]
		}
		containers = append(containers, id)
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	return &PodInfo{
		PodIP:           pod.Status.PodIP,
		HostIP:          pod.Status.HostIP,
		namespace:       pod.Namespace,
		name:            pod.Name,
		Labels:          pod.Labels,
		Annotations:     pod.Annotations,
		HostNetwork:     pod.Spec.HostNetwork,
		Containers:      containers,
		ResourceVersion: pod.ResourceVersion,
	}
}

func (manager *K8sResourceManager) UpdatePodAnnotation(podInfo *PodInfo, annotation map[string]string) error {
	var err error
	var rawPod *v1.Pod
	for i := 0; i < 3; i++ {
		rawPod, err = manager.ClientSet.CoreV1().Pods(podInfo.Namespace()).Get(podInfo.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if rawPod.Annotations == nil {
			rawPod.Annotations = annotation
		} else {
			changed := false
			for k, v := range annotation {
				if rawPod.Annotations[k] != v {
					rawPod.Annotations[k] = v
					changed = true
				}
			}
			if !changed {
				return nil
			}
		}
		_, err = manager.ClientSet.CoreV1().Pods(podInfo.Namespace()).Update(rawPod)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}

func (manager *K8sResourceManager) RemovePodAnnotation(podInfo *PodInfo, annotationkeys []string) error {
	var err error
	var rawPod *v1.Pod
	for i := 0; i < 3; i++ {
		rawPod, err = manager.ClientSet.CoreV1().Pods(podInfo.Namespace()).Get(podInfo.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if rawPod.Annotations == nil {
			return nil
		}
		changed := false
		for _, key := range annotationkeys {
			_, ok := rawPod.Annotations[key]
			if ok {
				delete(rawPod.Annotations, key)
				changed = true
			}
		}
		if !changed {
			return nil
		}
		_, err = manager.ClientSet.CoreV1().Pods(podInfo.Namespace()).Update(rawPod)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}
