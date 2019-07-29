package kubernetes

import (
	"fmt"
	"github.com/golang/glog"
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

func (pod *PodInfo) HasHeadlessService() bool {
	for k, v := range pod.Annotations {
		if strings.HasPrefix(k, POD_SERVICE_PREFIX) && strings.HasSuffix(k, ".headless") && GetLabelValueBool(v) {
			return true
		}
	}
	return false
}

type PodPortInfo struct {
	Protocol  string
	Service   string
	ConfigMap map[string]string
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
func (pod *PodInfo) GetPortSet() map[uint32]bool {
	result := make(map[uint32]bool)
	for k, v := range pod.Annotations {
		if v == "" {
			continue
		}

		_, port := GetServiceAndPort(k)
		result[port] = true

	}
	return result
}
func (pod *PodInfo) GetPortConfig() map[uint32]PodPortInfo {
	serviceConfig := make(map[string]map[string]string)
	for k, v := range pod.Annotations {
		if v == "" {
			continue
		}
		tokens := strings.Split(k, ".")
		if len(tokens) < 4 || tokens[0] != "traffic" || tokens[1] != "svc" {
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
	result := make(map[uint32]PodPortInfo)
	for k, v := range pod.Annotations {
		if v == "" {
			continue
		}

		service, port := GetServiceAndPort(k)
		if service != "" && port != 0 {
			podPortInfo := PodPortInfo{
				Service:   service,
				ConfigMap: serviceConfig[service],
				Protocol:  v,
			}
			oldInfo := result[port]
			if oldInfo.ConfigMap != nil {
				glog.Warningf("port %d belongs to more than one services, use %s's config", service)
			}
			result[port] = podPortInfo
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

func (manager *K8sResourceManager) UpdatePodAnnotation(podInfo *PodInfo, annotation map[string]string) error {
	var err error
	var rawPod *v1.Pod
	for i := 0; i < 3; i++ {
		rawPod, err = manager.clientSet.CoreV1().Pods(podInfo.Namespace()).Get(podInfo.Name(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if rawPod.Annotations == nil {
			rawPod.Annotations = annotation
		} else {
			changed := false
			for k, v := range annotation {
				current, ok := rawPod.Annotations[k]
				if ok && current != v {
					rawPod.Annotations[k] = v
					changed = true
				}
			}
			if !changed {
				return nil
			}
		}
		_, err = manager.clientSet.CoreV1().Pods(podInfo.Namespace()).Update(rawPod)
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
		rawPod, err = manager.clientSet.CoreV1().Pods(podInfo.Namespace()).Get(podInfo.Name(), metav1.GetOptions{})
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
		_, err = manager.clientSet.CoreV1().Pods(podInfo.Namespace()).Update(rawPod)
		if err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return err
}
