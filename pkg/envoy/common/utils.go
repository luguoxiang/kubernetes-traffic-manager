package common

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
	"strings"
)

func ContainsResource(resourceNames []string, resource string) bool {
	empty := true
	for _, cluster := range resourceNames {
		if resource == cluster {
			return true
		}
		empty = false
	}
	//if resourceNames is empty, all resources is included
	return empty
}

func MakeResource(resources []proto.Message, typeURL string, version string) (*v2.DiscoveryResponse, error) {
	var resoureList []types.Any
	for _, resource := range resources {
		data, err := proto.Marshal(resource)
		if err != nil {
			glog.Error(err.Error())
			return nil, err
		}

		resourceAny := types.Any{
			TypeUrl: typeURL,
			Value:   data,
		}
		resoureList = append(resoureList, resourceAny)
	}

	out := &v2.DiscoveryResponse{
		Nonce:       "0",
		VersionInfo: version,
		Resources:   resoureList,
		TypeUrl:     typeURL,
	}
	return out, nil
}

func CreateAccessLogAny(isHttp bool) *types.Any {
	var format string
	if isHttp {
		format = "[%START_TIME%] %REQ(:METHOD)% %PROTOCOL% %RESPONSE_CODE% %DURATION% %UPSTREAM_HOST% %DOWNSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_REMOTE_ADDRESS%\n"
	} else {
		format = "[%START_TIME%] TCP %BYTES_RECEIVED% %BYTES_SENT% %DURATION% %UPSTREAM_HOST% %DOWNSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_REMOTE_ADDRESS%\n"
	}

	result, err := types.MarshalAny(&accesslog.FileAccessLog{
		Path: "/var/log/access.log",
		AccessLogFormat: &accesslog.FileAccessLog_Format{
			Format: format,
		},
	})
	if err != nil {
		panic(err.Error())
	}
	return result
}

func IngressAttribute(port uint32, attr string) string {
	return fmt.Sprintf("traffic.svc.port.%d.ingress.%s", port, attr)
}

func OutboundClusterName(svc string, ns string, port uint32) string {
	//envoy stat/prometheus will split the name by '.' and use last element as metrices name
	//so this cluster name will have metrics
	//envoy_cluster_outbound_upstream_cx_total{envoy_cluster_name="9080|reviews_default_svc",instance="10.1.73.6:8900",job="traffic-envoy-pods"}
	return fmt.Sprintf("%d|%s|%s.outbound", port, ns, strings.Replace(svc, ".", "_", -1))
}
