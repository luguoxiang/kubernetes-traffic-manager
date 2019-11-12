package main

import (
	"flag"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type hostList []string

var hosts hostList
var name string
var namespace string

func (i *hostList) String() string {
	return "host list"
}

func (i *hostList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {
	flag.Var(&hosts, "host", "host names")
	flag.StringVar(&name, "name", "ingress-secret", "secret name")
	flag.StringVar(&namespace, "namespace", "default", "secret namespace")
	flag.Parse()

	manager, err := common.NewSecretManager()
	if err != nil {
		panic(err.Error())
	}
	certificate, privateKey, err := manager.GenerateGatewaySecret(hosts)
	if err != nil {
		panic(err.Error())
	}

	k8sManager, err := kubernetes.NewK8sResourceManager()
	if err != nil {
		panic(err.Error())
	}
	err = k8sManager.PostSecret(name, namespace, certificate, privateKey)

	if err != nil {
		panic(err.Error())
	}

}
