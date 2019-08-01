package common

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/golang/glog"
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
