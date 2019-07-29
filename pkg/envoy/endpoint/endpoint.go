package endpoint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
)

type ClusterAssignmentInfo struct {
	Service     string
	Namespace   string
	Port        uint32
	EndpointMap map[string]*EndpointInfo
}

func NewClusterAssignmentInfo(svc string, ns string, port uint32) *ClusterAssignmentInfo {
	return &ClusterAssignmentInfo{
		Service:   svc,
		Namespace: ns,
		Port:      port,
	}
}

func (info *ClusterAssignmentInfo) String() string {
	var ss []string
	for _, ai := range info.EndpointMap {
		ss = append(ss, ai.String())
	}
	return fmt.Sprintf("%s.%s:%d[%s]", info.Service, info.Namespace, info.Port, strings.Join(ss, ","))
}

func (info *ClusterAssignmentInfo) Name() string {
	return cluster.ServiceClusterName(info.Service, info.Namespace, info.Port)
}

func (info *ClusterAssignmentInfo) Type() string {
	return common.EndpointResource
}

func (info *ClusterAssignmentInfo) Clone() common.EnvoyResourceClonable {
	result := &ClusterAssignmentInfo{
		Service:     info.Service,
		Namespace:   info.Namespace,
		Port:        info.Port,
		EndpointMap: make(map[string]*EndpointInfo),
	}
	for k, v := range info.EndpointMap {
		result.EndpointMap[k] = v
	}
	return result
}

func (info *ClusterAssignmentInfo) Version() string {
	var result []string
	for _, assignment := range info.EndpointMap {
		result = append(result, assignment.Version)
	}
	if len(result) == 0 {
		return "0"
	}
	sort.Strings(result)
	return strings.Join(result, "-")
}
