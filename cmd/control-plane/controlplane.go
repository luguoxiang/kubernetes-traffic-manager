package main

import (
	"context"
	"flag"
	"fmt"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/annotation"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy"

	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/endpoint"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/listener"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"google.golang.org/grpc"
	"net"
	"os"
)

const grpcMaxConcurrentStreams = 1000000
const defaultGRPCPort = "18000"

var (
	BuildVersion = "0.1.0"
)

func main() {
	grpcPort := os.Getenv("TRAFFIC_MANAGE_PORT")

	if grpcPort == "" {
		grpcPort = defaultGRPCPort
	}
	flag.Parse()

	ctx := context.Background()

	grpcServer := grpc.NewServer(
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		errInfo := fmt.Sprintf("failed to listen %s", grpcPort)
		glog.Fatal(errInfo)
		panic(errInfo)
	}

	k8sManager, err := kubernetes.NewK8sResourceManager()
	if err != nil {
		panic(err.Error())
	}

	cds := cluster.NewClustersControlPlaneService(k8sManager)
	eds := endpoint.NewEndpointsControlPlaneService(k8sManager)
	lds := listener.NewListenersControlPlaneService(k8sManager)

	serviceToPodAnnotator := annotation.NewServiceToPodAnnotator(k8sManager)
	deploymentToPodAnnotator := annotation.NewDeploymentToPodAnnotator(k8sManager)

	ads := envoy.NewAggregatedDiscoveryService(cds, eds, lds)

	discovery.RegisterAggregatedDiscoveryServiceServer(grpcServer, ads)

	stopper := make(chan struct{})
	defer close(stopper)
	go k8sManager.WatchPods(stopper, k8sManager, eds, cds, lds, deploymentToPodAnnotator, serviceToPodAnnotator)
	go k8sManager.WatchServices(stopper, k8sManager, cds, lds, serviceToPodAnnotator)
	go k8sManager.WatchDeployments(stopper, k8sManager, deploymentToPodAnnotator)
	go k8sManager.WatchStatefulSets(stopper, k8sManager, deploymentToPodAnnotator)
	go k8sManager.WatchDaemonSets(stopper, k8sManager, deploymentToPodAnnotator)

	glog.Infof("grpc server listening %s, version=%s", grpcPort, BuildVersion)
	go func() {
		if err = grpcServer.Serve(lis); err != nil {
			glog.Error(err)
		}
	}()
	<-ctx.Done()

	grpcServer.GracefulStop()
}
