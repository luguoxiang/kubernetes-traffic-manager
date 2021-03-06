package main

import (
	"flag"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/docker"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

func main() {
	flag.Parse()
	k8sManager, err := kubernetes.NewK8sResourceManager()
	if err != nil {
		panic(err.Error())
	}
	stopper := make(chan struct{})
	defer close(stopper)

	envoyManager, err := docker.NewEnvoyManager(k8sManager)
	if err != nil {
		panic(err.Error())
	}
	envoyManager.CheckExistingEnvoy()
	k8sManager.WatchPods(stopper, k8sManager, envoyManager)

}
