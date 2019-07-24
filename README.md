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

# Quick start

```
helm install --name kubernetes-traffic-manager helm/kubernetes-traffic-manager
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml
kubectl label deployment productpage-v1 details-v1 ratings-v1 reviews-v1 reviews-v2 reviews-v3 traffic.envoy.enabled=true
kubectl label svc productpage details ratings reviews traffic.port.9080=http
```

# Configuration Labels

| Resource | Labels | Default | Description |
|----------|--------|---------|--------------|
| Pod | traffic.envoy.enabled | false |whether enable envoy docker for pod |
| Pod | traffic.tracing.ingress | true | enable ingress tracing for requests to this pod |
| Pod | traffic.envoy.local.use_podip | false | whether to let envoy access local pod using pod ip instead of 127.0.0.1 |
| Service | traffic.port.(port number)| None| protocol for the port on service (http, tcp)|
| Service | traffic.connection.timeout |  60000 | timeout in miliseconds  |
| Service | traffic.retries.max | 0 | max retries number |
| Service | traffic.connection.max | 0 | max number of connection | 
| Service | traffic.request.max-pending | 0 | max pending requests  |
| Service | traffic.tracing.egress | true | enable tracing for requests to this service | 
| Service | traffic.request.timeout | 0 | timeout in miliseconds |0 |
| Service | traffic.retries.5xx | 0 | number of retries for 5xx error | 
| Service | traffic.retries.connect-failure | 0 | number of retries for connect failure |
| Service | traffic.retries.gateway-error | 0 | number of retries for gateway error |
| Service | traffic.fault.delay.time | 0 | delay time in miliseconds |
| Service | traffic.fault.delay.percentage | 0 | percentage of requests to be delayed for time |
| Service | traffic.fault.abort.status | 0 | abort with http status |
| Service | traffic.fault.abort.percentage | 0 | percentage of requests to be aborted |
| Deployment | traffic.endpoint.weight | 100 | weight value for the pods of this deployment [0-128]  |
| Deployment | traffic.envoy.enabled | false | whether to enable envoy docker for the pods of this deployment |





