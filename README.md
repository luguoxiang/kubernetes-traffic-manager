# Components
## envoy-manager
Responsible to start/stop envoy container for traffic-control

When user label a service with "traffic.envoy.enabled=true", traffic-agent will start a docker instance of "envoyproxy/envoy" image. 

* Envoy docker git repo: https://github.com/luguoxiang/traffic-envoy
* The docker instance will share corresponding pod's network(docker container network mode).
* The iptable config of the pod network will be changed to redirect all incoming and outcoming traffic to envoy container listen port.
(reference envoy-proxy/iptable_init.sh, the port number is set to 10000 as PROXY_PORT env).
* The pod will be annotated with observability.envoy.enabled=true, observability.envoy.proxy=(docker id)

When user label the service with "traffic.envoy.enabled=false"

* the docker instance will be deleted
* the iptable config will be restored (reference envoy-proxy/iptable_clean.sh)
* the pod will be annotated with traffic.envoy.enabled=false, traffic.envoy.proxy=""

## traffic-control
traffic-control is a control plane implementation of envoy proxy (https://www.envoyproxy.io/). The data plane is group of "envoyproxy/envoy" images attached to k8s pods.

# Configuration
## Labels
| Resource | Labels | Value | Description |
|----------|--------|--------|------------|
| Pod | traffic.endpoint.inbound.use_podip | Bool | if true, envoy will use pod ip instead of 127.0.0.1 to access attached pod |
| Pod | traffic.envoy.enabled | Bool | whether enable envoy docker for pod |
| Service | traffic.port.(port number)| http, tcp | protocol for the port on service, default is tcp |
| Service | traffic.outbound.enabled | Bool | whether other pods can access this service by outbound request |
| Deployment | traffic.endpoint.weight | Number in [0-128] | weight value for the pods of this deployment  |
| Deployment | traffic.envoy.enabled | Bool | hether enable envoy docker for the pods of this deployment |

## Annotations
The annotations are set by control plane, user does not need to set these annotations

| Resource | Annotations | Value | Description |
|----------|-------------|-------|-------------|
| Pod | traffic.svc.(service name).port.(port number) | http, tcp | the pod belonging to a service which define port with protocol |
| Pod | traffic.svc.(service name).headless | Bool | whether the pod belonging to certain headless service |
| Pod | traffic.envoy.deployment.enabled | Bool | whetehr the pod's envoy is enabled by a deployment |
| Pod | traffic.endpoint.weight | Number in [0-128] | whetehr the pod has a weight set by a deployment |
| Pod | traffic.envoy.proxy | envoy docker id | envoy docker id if this pod's envoy is enabled |






