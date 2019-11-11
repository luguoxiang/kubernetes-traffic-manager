package ingress

import (
	"fmt"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/listener"
)

type IngressHttpInfo struct {
	listener.HttpListenerConfigInfo
	Host    string
	Path    string
	Cluster string
	Secret  string
}

func NewIngressHttpInfo(host string, path string, cluster string) *IngressHttpInfo {
	return &IngressHttpInfo{
		Host:    host,
		Path:    path,
		Cluster: cluster,
	}
}

func (info *IngressHttpInfo) Name() string {
	if info.Host == "*" {
		fmt.Sprintf("http|all|%s", info.Path)
	}
	return fmt.Sprintf("http|%s|%s", info.Host, info.Path)
}

func (info *IngressHttpInfo) Type() string {
	return common.ListenerResource
}

func (info *IngressHttpInfo) String() string {
	return info.Name()
}
