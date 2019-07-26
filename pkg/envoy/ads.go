package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	discovery "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/cluster"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/common"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/envoy/listener"
	"strings"
	"time"
)

const (
	IngressNodeId = "traffic-ingress"
	MAX_RPS       = 10
)

type AggregatedDiscoveryService struct {
	cds *cluster.ClustersControlPlaneService
	eds *EndpointsControlPlaneService
	lds *listener.ListenersControlPlaneService
	sds *SecretsControlPlaneService
}

func NewAggregatedDiscoveryService(cds *cluster.ClustersControlPlaneService,
	eds *EndpointsControlPlaneService,
	lds *listener.ListenersControlPlaneService,
	sds *SecretsControlPlaneService) *AggregatedDiscoveryService {
	return &AggregatedDiscoveryService{
		cds: cds, eds: eds, lds: lds, sds: sds,
	}
}
func (ads *AggregatedDiscoveryService) processRequest(req *v2.DiscoveryRequest) (*v2.DiscoveryResponse, error) {
	switch req.TypeUrl {
	case common.EndpointResource:
		return ads.eds.ProcessRequest(req, ads.eds.BuildResource)
	case common.ClusterResource:
		return ads.cds.ProcessRequest(req, ads.cds.BuildResource)
	case common.ListenerResource:
		//always request all resources
		req.ResourceNames = nil
		return ads.lds.ProcessRequest(req, ads.lds.BuildResource)
		//case RouteResource:
	case common.SecretResource:
		return ads.sds.ProcessRequest(req, ads.sds.BuildResource)
	default:
		return nil, fmt.Errorf("Unsupported TypeUrl" + req.TypeUrl)
	}

}
func (ads *AggregatedDiscoveryService) StreamAggregatedResources(stream discovery.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	requestCh := make(chan *v2.DiscoveryRequest)
	go func() {
		for {
			req, err := stream.Recv()
			if err != nil {
				glog.Error(err.Error())
				requestCh <- nil
				return
			}
			if req.Node == nil || req.Node.Id == "" {
				err := fmt.Errorf("Missing node id info, type=%s, resource=%s", req.TypeUrl, strings.Join(req.ResourceNames, ","))
				glog.Error(err.Error())
				continue
			}

			requestCh <- req
		}
	}()

	for _ = range time.Tick(time.Millisecond * 100) {
		req := <-requestCh
		if req == nil {
			break
		}
		go func(req *v2.DiscoveryRequest) {
			if glog.V(2) {
				glog.Infof("Request recevied: type=%s, nonce=%s, version=%s, resource=%s, node=%s",
					req.TypeUrl, req.GetResponseNonce(), req.VersionInfo, strings.Join(req.ResourceNames, ","), req.Node.Id)
			}
			resp, err := ads.processRequest(req)
			if err != nil {
				glog.Errorf("Failed to process %s, version=%s:%s", req.TypeUrl, resp.VersionInfo, err.Error())
			} else {
				if glog.V(2) {
					glog.Infof("Send %s, version=%s", req.TypeUrl, resp.VersionInfo)
				}
				stream.Send(resp)
			}

		}(req)
	}
	return nil
}

func (ads *AggregatedDiscoveryService) DeltaAggregatedResources(discovery.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return nil
}
