package common

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"

	"crypto/md5"
	"encoding/hex"
	"github.com/golang/glog"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
	"reflect"
	"sort"
	"strings"
	"sync"
)

type EnvoyResource interface {
	Name() string
	Type() string
	String() string
}
type EnvoyResourceClonable interface {
	EnvoyResource
	Clone() EnvoyResourceClonable
}
type stream interface {
	Send(*envoy_api_v2.DiscoveryResponse) error
	Recv() (*envoy_api_v2.DiscoveryRequest, error)
}

type ControlPlaneService struct {
	resourceMap map[string]EnvoyResource
	k8sManager  *kubernetes.K8sResourceManager
	cond        *sync.Cond
	versionMap  map[string]string
}

func NewControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *ControlPlaneService {
	return &ControlPlaneService{
		resourceMap: make(map[string]EnvoyResource),
		versionMap:  make(map[string]string),
		k8sManager:  k8sManager,
		cond:        k8sManager.NewCond(),
	}
}

func (cps *ControlPlaneService) GetK8sManager() *kubernetes.K8sResourceManager {
	return cps.k8sManager
}

func (cps *ControlPlaneService) GetResources(resourceNames []string) (map[string]EnvoyResource, string) {
	requested := make(map[string]EnvoyResource)
	var versions []string
	if len(resourceNames) > 0 {
		sort.Strings(resourceNames)
		for _, name := range resourceNames {
			resource := cps.resourceMap[name]
			version := cps.versionMap[name]
			if version == "" || resource == nil {
				glog.Warningf("Could not find requested '%s'", name)
				continue
			}
			requested[name] = resource
			versions = append(versions, version)
		}

	} else {
		for name, version := range cps.versionMap {
			resource := cps.resourceMap[name]
			if version == "" || resource == nil {
				glog.Warningf("Could not find requested %s", name)
				continue
			}
			requested[name] = resource
			versions = append(versions, version)
		}
	}

	switch len(versions) {
	case 0:
		return requested, ""
	case 1:
		return requested, versions[0]
	default:
		sort.Strings(versions)
		hasher := md5.New()
		hasher.Write([]byte(strings.Join(versions, ",")))
		return requested, hex.EncodeToString(hasher.Sum(nil))
	}

}

func (cps *ControlPlaneService) GetResourceNoCopy(name string) (EnvoyResource, string) {
	if !cps.k8sManager.IsLocked() {
		panic("K8sResourceManager should be locked in GetResource")
	}
	resource := cps.resourceMap[name]
	version := cps.versionMap[name]

	return resource, version
}

func (cps *ControlPlaneService) GetResourceClone(name string) (EnvoyResource, string) {
	if !cps.k8sManager.IsLocked() {
		panic("K8sResourceManager should be locked in GetResource")
	}
	resource := cps.resourceMap[name]
	version := cps.versionMap[name]
	if resource == nil {
		return nil, ""
	}
	resourceClone := resource.(EnvoyResourceClonable)
	return resourceClone.Clone(), version
}

func (cps *ControlPlaneService) UpdateResource(resource EnvoyResource, resourceVersion string) {
	if !cps.k8sManager.IsLocked() {
		panic("K8sResourceManager should be locked in ControlPlaneService:UpdateResource")
	}

	name := resource.Name()
	if cps.versionMap[name] == resourceVersion {
		return
	}
	oldResource := cps.resourceMap[name]
	if oldResource != nil && resourceVersion == "" {
		glog.Infof("REMOVE %T %s", resource, resource.String())
		delete(cps.resourceMap, name)
		delete(cps.versionMap, name)

		cps.cond.Broadcast()
		return
	}

	if reflect.DeepEqual(oldResource, resource) {
		return
	}
	if oldResource == nil {
		glog.Infof("ADD %T %s, version=%s", resource, resource.String(), resourceVersion)
	} else {
		glog.Infof("UPDATE %T %s, version=%s", resource, resource.String(), resourceVersion)
	}
	cps.resourceMap[name] = resource

	cps.versionMap[name] = resourceVersion
	cps.cond.Broadcast()
}

type ResponseBuilder func(resourceMap map[string]EnvoyResource, version string, node *core.Node) (*envoy_api_v2.DiscoveryResponse, error)

func (cps *ControlPlaneService) ProcessRequest(req *envoy_api_v2.DiscoveryRequest, builder ResponseBuilder) (*envoy_api_v2.DiscoveryResponse, error) {
	cps.k8sManager.Lock()

	var currentVersion string
	var resourceMap map[string]EnvoyResource
	for {
		resourceMap, currentVersion = cps.GetResources(req.ResourceNames)

		if currentVersion == req.VersionInfo {
			if glog.V(2) {
				glog.Infof("Waiting update on %s for %v, current version=%s", req.TypeUrl, req.ResourceNames, currentVersion)
			}
			cps.cond.Wait()
		} else {
			break
		}
	}

	cps.k8sManager.Unlock()

	return builder(resourceMap, currentVersion, req.Node)
}
