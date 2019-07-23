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

func (pod *PodInfo) EnvoyDockerId() string {
	return pod.Annotations[ENVOY_PROXY_ANNOTATION]
}

func (pod *PodInfo) Weight() uint32 {
	value := pod.Annotations[ENDPOINT_WEIGHT]
	if value != "" {
		result := GetLabelValueUInt32(value)
		if result > 128 {
			return 128
		} else {
			return result
		}
	}

	return DEFAULT_WEIGHT
}

func (pod *PodInfo) EnvoyEnabled() bool {
	if pod.Labels[ENVOY_ENABLED] != "" {
		//ENVOY_ENABLED label will overide annotation set by deployment & service label
		return strings.EqualFold(pod.Labels[ENVOY_ENABLED], "true")
	}

	return strings.EqualFold(pod.Annotations[ENVOY_ENABLED_BY_DEPLOYMENT], "true")
}

func (pod *PodInfo) HasHeadlessService() bool {
	for k, v := range pod.Annotations {
		if strings.HasPrefix(k, POD_SERVICE_PREFIX) && strings.HasSuffix(k, ".headless") && strings.EqualFold(v, "true") {
			return true
		}
	}
	return false
}

type PodPortInfo struct {
	Port     uint32
	Protocol string
	Headless bool
}

func GetServiceAndPort(annotation string) (string, uint32) {
	if strings.HasPrefix(annotation, POD_SERVICE_PREFIX) {
		items := strings.Split(annotation, ".")
		itemLen := len(items)
		if items[itemLen-2] == "port" {
			port, err := strconv.ParseUint(items[itemLen-1], 10, 32)
			if err == nil {
				return items[itemLen-3], uint32(port)
			}
		}
	}
	return "", 0
}

func (pod *PodInfo) IsHeadlessService(service string) bool {
	headlessKey := PodHeadlessByService(service)
	return strings.EqualFold(pod.Annotations[headlessKey], "true")
}

func (pod *PodInfo) GetPortMap() map[uint32]PodPortInfo {
	result := make(map[uint32]PodPortInfo)
	for key, v := range pod.Annotations {
		service, port := GetServiceAndPort(key)
		if service != "" && port != 0 {
			oldInfo := result[port]
			oldInfo.Port = port
			// do not override http protocol and headless
			if !oldInfo.Headless {
				headlessKey := PodHeadlessByService(service)
				oldInfo.Headless = strings.EqualFold(pod.Annotations[headlessKey], "true")
			}
			if v != "" && (oldInfo.Protocol == "" || oldInfo.Protocol == "tcp") {
				oldInfo.Protocol = v
			}
			result[port] = oldInfo
		}
	}
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

func (manager *K8sResourceManager) UpdatePodAnnotation(podInfo *PodInfo, annotation map[string]*string) error {
	var err error
	var rawPod *v1.Pod
	for i := 0; i < 3; i++ {
		rawPod, err = manager.clientSet.CoreV1().Pods(podInfo.Namespace()).Get(podInfo.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		changed := false
		for k, v := range annotation {
			if v != nil && rawPod.Annotations == nil {
				rawPod.Annotations = make(map[string]string)
			}
			current, ok := rawPod.Annotations[k]
			if v == nil && ok {
				delete(rawPod.Annotations, k)
				changed = true
			}
			if v != nil && current != *v {
				rawPod.Annotations[k] = *v
				changed = true
			}
		}
		if !changed {
			return nil
		}
		_, err = manager.clientSet.CoreV1().Pods(podInfo.Namespace()).Update(rawPod)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}
