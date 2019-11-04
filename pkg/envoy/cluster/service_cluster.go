package cluster

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"strings"
)

type ServiceClusterInfo struct {
	ClusterConfigInfo

	Service   string
	Namespace string
	Port      uint32

	LbPolicy int32
}

func ServiceClusterName(svc string, ns string, port uint32) string {
	return fmt.Sprintf("%d|%s|%s.outbound", port, ns, strings.Replace(svc, ".", "_", -1))
}

func NewServiceClusterInfo(svc *kubernetes.ServiceInfo, port uint32) *ServiceClusterInfo {
	return &ServiceClusterInfo{
		Service:   svc.Name(),
		Namespace: svc.Namespace(),
		Port:      port,
	}
}
func (info *ServiceClusterInfo) Config(config map[string]string) {
	info.ClusterConfigInfo.Config(config)

	v := config["traffic.lb.policy"]
	if v != "" {
		info.LbPolicy = v2.Cluster_LbPolicy_value[v]
	}
}

func (info *ServiceClusterInfo) String() string {
	return fmt.Sprintf("%s.%s:%d,mr=%d,ct=%v", info.Service, info.Namespace, info.Port, info.MaxRetries, info.ConnectionTimeout)
}

func (info *ServiceClusterInfo) Name() string {
	return ServiceClusterName(info.Service, info.Namespace, info.Port)
}

func (info *ServiceClusterInfo) Type() string {
	return common.ClusterResource
}

func (info *ServiceClusterInfo) CreateCluster() *v2.Cluster {
	result := &v2.Cluster{
		Name:           info.Name(),
		ConnectTimeout: info.ConnectionTimeout,
		ClusterDiscoveryType: &v2.Cluster_Type{
			Type: v2.Cluster_EDS,
		},
		EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
			EdsConfig: &core.ConfigSource{
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
		},
		LbPolicy: v2.Cluster_LbPolicy(info.LbPolicy),
	}
	info.ApplyClusterConfig(result)
	return result
}
