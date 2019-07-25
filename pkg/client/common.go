package client

import (
	"fmt"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	httpfault "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/fault/v2"
	hcm "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"gopkg.in/yaml.v2"
	"strings"
)

func decodeAny(any *types.Any) interface{} {
	var yaml_data []byte
	var pb proto.Message
	switch any.TypeUrl {
	case "type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager":
		pb = &hcm.HttpConnectionManager{}
	case "type.googleapis.com/envoy.config.filter.network.tcp_proxy.v2.TcpProxy":
		pb = &tcp.TcpProxy{}
	case "type.googleapis.com/envoy.config.accesslog.v2.FileAccessLog":
		pb = &accesslog.FileAccessLog{}
	case "type.googleapis.com/envoy.config.filter.http.fault.v2.HTTPFault":
		pb = &httpfault.HTTPFault{}
	default:
		panic(any.TypeUrl)
	}
	err := types.UnmarshalAny(any, pb)
	if err != nil {
		panic(err.Error())
	}
	yaml_data, _ = yaml.Marshal(pb)
	data := make(map[interface{}]interface{})
	yaml.Unmarshal(yaml_data, data)
	return doClean(data)
}
func doClean(in interface{}) interface{} {
	switch typedValue := in.(type) {
	case map[interface{}]interface{}:
		result := make(map[interface{}]interface{})
		for k, v := range typedValue {
			key := strings.ToLower(fmt.Sprint(k))
			if key == "typedconfig" {
				if v == nil {
					return nil
				}
				var any types.Any
				vv := v.(map[interface{}]interface{})
				any.TypeUrl = vv["typeurl"].(string)
				value := vv["value"].([]interface{})

				for _, vvv := range value {
					any.Value = append(any.Value, uint8(vvv.(int)))
				}
				return decodeAny(&any)
			}
			if strings.HasPrefix(key, "xxx_") {
				delete(typedValue, key)
				continue
			}
			v = doClean(v)
			if v == nil {
				continue
			}
			result[k] = v

		}
		if len(result) == 1 {
			for _, skipKey := range []string{"kind", "fields", "structvalue", "string_value", "attributes", "stringvalue", "boolvalue", "listvalue"} {
				if result[skipKey] != nil {
					return result[skipKey]
				}
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []interface{}:
		var result []interface{}
		for _, elem := range typedValue {
			result = append(result, doClean(elem))
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return in
	}
}

func DoPrint(in interface{}) {
	yaml_data, _ := yaml.Marshal(in)
	data := make(map[interface{}]interface{})
	yaml.Unmarshal(yaml_data, data)

	yaml_data, _ = yaml.Marshal(doClean(data))
	fmt.Printf("%s\n", yaml_data)

}
