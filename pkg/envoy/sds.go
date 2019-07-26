package envoy

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
	"github.com/luguoxiang/kubernetes-traffic-manager/pkg/kubernetes"
)

type SecretResourceInfo struct {
	cert      []byte
	key       []byte
	name      string
	namespace string
}

func (info *SecretResourceInfo) Name() string {
	return fmt.Sprintf("%s.%s", info.name, info.namespace)
}

func (info *SecretResourceInfo) Type() string {
	return SecretResource
}

func (info *SecretResourceInfo) String() string {
	return fmt.Sprintf("secret %s.%s", info.name, info.namespace)
}

type SecretsControlPlaneService struct {
	*ControlPlaneService
}

func NewSecretsControlPlaneService(k8sManager *kubernetes.K8sResourceManager) *SecretsControlPlaneService {
	return &SecretsControlPlaneService{ControlPlaneService: NewControlPlaneService(k8sManager)}
}

func (*SecretsControlPlaneService) SecretValid(info *kubernetes.SecretInfo) bool {
	return info.Data["tls.crt"] != nil && info.Data["tls.key"] != nil
}

func (sds *SecretsControlPlaneService) SecretAdded(info *kubernetes.SecretInfo) {
	sds.UpdateResource(&SecretResourceInfo{
		name:      info.Name,
		namespace: info.Namespace,
		cert:      info.Data["tls.crt"],
		key:       info.Data["tls.key"],
	}, info.ResourceVersion)
}

func (sds *SecretsControlPlaneService) SecretDeleted(info *kubernetes.SecretInfo) {
	sds.UpdateResource(&SecretResourceInfo{
		name:      info.Name,
		namespace: info.Namespace,
	}, "")
}

func (sds *SecretsControlPlaneService) SecretUpdated(oldSecret, newSecret *kubernetes.SecretInfo) {
	sds.SecretAdded(newSecret)
}

func (sds *SecretsControlPlaneService) BuildResource(resourceMap map[string]EnvoyResource, version string, node *core.Node) (*v2.DiscoveryResponse, error) {
	var secrets []proto.Message

	for _, resource := range resourceMap {
		info := resource.(*SecretResourceInfo)
		secrets = append(secrets, &auth.Secret{
			Name: info.Name(),
			Type: &auth.Secret_TlsCertificate{
				TlsCertificate: &auth.TlsCertificate{
					CertificateChain: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{
							InlineBytes: info.cert,
						},
					},
					PrivateKey: &core.DataSource{
						Specifier: &core.DataSource_InlineBytes{
							InlineBytes: info.key,
						},
					},
				},
			},
		})
	}

	return MakeResource(secrets, SecretResource, version)
}
