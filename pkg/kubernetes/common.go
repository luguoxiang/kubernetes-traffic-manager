package kubernetes

import (
	"fmt"
	"github.com/golang/glog"
	"strconv"
	"strings"
)

type ResourceType int

const (
	SERVICE_TYPE    ResourceType = 1
	DEPLOYMENT_TYPE ResourceType = 2
	POD_TYPE        ResourceType = 3

	ENVOY_ENABLED               = "traffic.envoy.enabled"
	ENVOY_ENABLED_BY_DEPLOYMENT = "traffic.envoy.deployment.enabled"
	ENVOY_PROXY_ANNOTATION      = "traffic.envoy.proxy"
	ENDPOINT_WEIGHT             = "traffic.endpoint.weight"
	ENDPOINT_INBOUND_PODIP      = "traffic.endpoint.inbound.use_podip"
	DEFAULT_WEIGHT              = 100

	POD_SERVICE_PREFIX = "traffic.svc."
)

func GetLabelValueUInt32(value string) uint32 {
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return uint32(i)
}

func GetLabelValueBool(value string) bool {
	return strings.EqualFold(value, "true")
}

func GetLabelValueInt64(value string) int64 {
	if value == "" {
		return 0
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return int64(i)
}

func ServicePortProtocol(port uint32) string {
	return fmt.Sprintf("traffic.port.%d", port)
}

func PodPortProtcolByService(svc string, port uint32) string {
	return fmt.Sprintf("%s%s.port.%d", POD_SERVICE_PREFIX, svc, port)
}

func PodEnvoyByService(svc string) string {
	return fmt.Sprintf("%s%s.envoy", POD_SERVICE_PREFIX, svc)
}

func PodHeadlessByService(svc string) string {
	return fmt.Sprintf("%s%s.headless", POD_SERVICE_PREFIX, svc)
}

func (e ResourceType) String() string {
	switch e {
	case POD_TYPE:
		return "Pod"
	case DEPLOYMENT_TYPE:
		return "Deployment"
	case SERVICE_TYPE:
		return "Service"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(e))
	}
}

type ResourceInfoPointer interface {
	GetSelector() map[string]string
	Namespace() string
	Name() string
	Type() ResourceType
	String() string
}

func (manager *K8sResourceManager) addResource(resource ResourceInfoPointer) {
	if glog.V(2) {
		glog.Infof("add %s", resource.String())
	}
	for k, v := range resource.GetSelector() {
		key := fmt.Sprintf("%s:%s:%s", resource.Namespace(), k, v)

		typeResourceMap := manager.labelTypeResourceMap[key]
		if typeResourceMap == nil {
			typeResourceMap = make(ResourcesOnLabel)
		}

		typeResourceMap[resource.Type()] = append(typeResourceMap[resource.Type()], resource)
		manager.labelTypeResourceMap[key] = typeResourceMap
	}
}

func (manager *K8sResourceManager) removeResource(resource ResourceInfoPointer) {
	if glog.V(2) {
		glog.Infof("remove %s", resource.String())
	}
	for k, v := range resource.GetSelector() {
		key := fmt.Sprintf("%s:%s:%s", resource.Namespace(), k, v)

		typeResourceMap := manager.labelTypeResourceMap[key]
		if typeResourceMap == nil {
			continue
		}

		resources := typeResourceMap[resource.Type()]

		var matched []ResourceInfoPointer
		for _, existResource := range resources {
			if existResource.Name() == resource.Name() {
				continue
			}
			matched = append(matched, existResource)
		}
		typeResourceMap[resource.Type()] = matched
	}
}

func (manager *K8sResourceManager) GetMatchedResources(resource ResourceInfoPointer, matchType ResourceType) []ResourceInfoPointer {
	if !manager.IsLocked() {
		panic("K8sResourceManager should be locked in GetMatchedResources()")
	}
	countMap := make(map[ResourceInfoPointer]*int)
	for k, v := range resource.GetSelector() {
		key := fmt.Sprintf("%s:%s:%s", resource.Namespace(), k, v)
		typeResourceMap := manager.labelTypeResourceMap[key]
		if typeResourceMap == nil {
			return nil
		}
		resources := typeResourceMap[matchType]
		for _, matchResource := range resources {
			if countMap[matchResource] == nil {
				count := 1
				countMap[matchResource] = &count
			} else {
				*countMap[matchResource]++
			}
		}
	}
	//type service < deployment < pod
	returnParent := resource.Type() > matchType
	var result []ResourceInfoPointer
	for matchResource, countPtr := range countMap {
		if returnParent {
			if *countPtr != len(matchResource.GetSelector()) {
				//return services or deployments from pod
				//count should be same with service or deployment selector
				continue
			}
		} else if *countPtr != len(resource.GetSelector()) {
			//return pods from service or deployment
			//count should be same with pod labels
			continue
		}
		result = append(result, matchResource)
	}
	return result
}
