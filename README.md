# Introduction
 kubernetes-traffic-manager is a traffic management tools for kubernetes, it implements envoy (https://www.envoyproxy.io/) control plane data api to support:
 * Simple ingress gateway
 * Weighted load balancing
 * Tracing
 * Fault Injection
 * Circuit breaker
 * Runtime metrics
 
# Installation
```
helm install --name kubernetes-traffic-manager helm/kubernetes-traffic-manager
```

# Ingress gateway

```
# deploy sample application
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml

# deploy k8s ingress resource
kubectl apply -f samples/ingress.yaml

# wait awhile then run
curl -v localhost/reviews/0


```
# Required labels
   When user label a pod or deployment with "traffic.envoy.enabled=true", the related pods' traffic will be managed.
   
   By default, all envoy enabled pods' incoming and outcoming traffic will be blocked. 
   You need to add traffic.port.(port number)=(protocol) labels for service or pod to unblock traffic on certain port.
   The protocol can be http or tcp or direct(bypass envoy load balancing). 
   
   service in ingress configuration will be automatically annotated(not labeled) with http protocol, so no need to label them.
   
   For example, following label wil let envoy enabled pods accessing kubernetes service
   ```
   kubectl label svc kubernetes traffic.port.443=direct
   ```
   
# Runtime metrics
```
#enable envoy to collect runtime metrics
kubectl label deployment reviews-v1 traffic.envoy.enabled=true
kubectl label deployment reviews-v2 traffic.envoy.enabled=true
kubectl label deployment reviews-v3 traffic.envoy.enabled=true

# generate traffic
curl -v localhost/reviews/0

# query runtime metrics
curl localhost/api/v1/label/__name__/values |jq
curl -G http://localhost/api/v1/query --data-urlencode "query=envoy_cluster_outbound_upstream_rq_completed{instance='(Ingress PodIP):8900', envoy_cluster_name='9080|default|reviews'}"|jq
```

# Load Balancing
| Resource | Labels | Default | Description |
|----------|--------|---------|--------------|
| Service | traffic.lb.policy | ROUND_ROBIN | load balance policy: ROUND_ROBIN, LEAST_REQUEST, RING_HASH, RANDOM, MAGLEV | 
| Service | traffic.hash.cookie.name | "" | cookie hash policy |
| Service | traffic.hash.cookie.ttl | 0 | generate cookie with ttl |
| Service | traffic.hash.header.name | "" | http header name for hash policy |
| Pod, Deployment, StatefulSet, DaemonSet | traffic.endpoint.weight | 100 | weight value for related pods [0-128]  |

```
# if reviews is not in ingress configuration, following lable is required
# kubectl label svc reviews traffic.port.9080=http

# Default lb policy is ROUND_ROBIN
kubectl label deployment reviews-v1 traffic.endpoint.weight=100 
kubectl label deployment reviews-v2 traffic.endpoint.weight=10 
kubectl label deployment reviews-v3 traffic.endpoint.weight=0

# The value should be 2, since reviews-v3 has weight 0
curl -G http://localhost/api/v1/query --data-urlencode "query=envoy_cluster_outbound_membership_total{envoy_cluster_name='9080|default|reviews'}"|jq

# repeat many times
curl -v http://localhost/reviews/0

# The value of reviews-v1 and reviews-v2 should be about 10:1
curl -G http://localhost/api/v1/query --data-urlencode "query=envoy_listener_http_static_downstream_rq_xx{envoy_response_code_class='2', instance='review-v1 or review-v2 pod ip}:8900'}"|jq

# Use cookie hash policy
kubectl label svc reviews traffic.lb.policy=RING_HASH
kubectl label svc reviews traffic.hash.cookie.name="mycookie"
kubectl label svc reviews traffic.hash.cookie.ttl="100000"

# delete review-v1 and review-v2 pods to avoid http request being cached

curl -v  http://localhost/reviews/0
# The http response should contains set-cookie, for example:
# set-cookie: mycookie="3acd918773ba09c5"; Max-Age=100; HttpOnly

#following request should always send to same review pod, it should always contain "ratings" or always be without "ratings"
curl -v  -H "Cookie: mycookie=3acd918773ba09c5" http://localhost/reviews/0

kubectl delete -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml
```

Reference:
* https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/load_balancers
* https://www.envoyproxy.io/docs/envoy/latest/api-v2/api/v2/route/route.proto#envoy-api-field-route-routeaction-hash-policy

# Fault Injection

| Resource | Labels | Default | Description |
|----------|--------|---------|--------------|
| Pod, Service | traffic.fault.delay.time | 0 | delay time in miliseconds |
| Pod, Service | traffic.fault.delay.percentage | 0 | percentage of requests to be delayed for time |
| Pod, Service | traffic.fault.abort.status | 0 | abort with http status |
| Pod, Service | traffic.fault.abort.percentage | 0 | percentage of requests to be aborted |
| Pod, Service | traffic.rate.limit | 0 | rate limit number in Kbps on each client |

Ingress does not support Fault Injection
```
kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml

kubectl label svc reviews traffic.port.9080=http
kubectl label deployment traffic-zipkin traffic.envoy.enabled=true

kubectl label svc reviews traffic.fault.delay.time=3000
kubectl label svc reviews traffic.fault.delay.percentage=100

# should delay 3 seconds
kubectl exec traffic-zipkin-694c7884d5-bqdvm -- curl http://reviews:9080/reviews/0

kubectl label svc reviews traffic.fault.abort.status=505
kubectl label svc reviews traffic.fault.abort.percentage=100

# should delay 3 seconds and return http 505
kubectl exec traffic-zipkin-694c7884d5-bqdvm -- curl -v http://reviews:9080/reviews/0

kubectl label svc reviews traffic.fault.abort.percentage-
kubectl label svc reviews traffic.fault.abort.status-
kubectl label svc reviews traffic.fault.delay.percentage-
kubectl label svc reviews traffic.fault.delay.time-
  
 # should return normal
kubectl exec traffic-zipkin-694c7884d5-bqdvm -- curl -v http://reviews:9080/reviews/0

kubectl delete -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml
```

# Tracing 

| Resource | Labels | Default | Description |
|----------|--------|---------|--------------|
| Pod, Service | traffic.tracing.enabled | false | enable tracing for requests to or from envoy enabled pods of this service | 
| Pod, Service | traffic.tracing.sampling | 100 | percentage of tracing sampling (float) |

```
kubectl label deployment traffic-zipkin traffic.envoy.enabled=false --overwrite

kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.0/samples/bookinfo/platform/kube/bookinfo.yaml

kubectl label svc productpage details ratings reviews traffic.port.9080=http

kubectl label deployment productpage-v1 details-v1 ratings-v1 reviews-v1 reviews-v2 reviews-v3 traffic.envoy.enabled=true

kubectl label svc productpage details ratings reviews traffic.tracing.enabled=true
# sampling rate 50%
kubectl label svc productpage details ratings reviews traffic.tracing.sampling=50

#repeat following command 10 times
kubectl exec traffic-zipkin-694c7884d5-bqdvm -- curl -v http://productpage:9080:/productpage

# show traceId, number of returned traceId should be around 5
kubectl exec traffic-zipkin-694c7884d5-x4pn9 curl http://localhost:9411/api/v2/traces?limit=100 | python3 -c "import sys, json; print([x[0]['traceId'] for x in json.load(sys.stdin)])"
```
Like Istio, Applications need to propagate appropriate HTTP headers, reference: https://istio.io/docs/tasks/telemetry/distributed-tracing/overview/

## Check running envoy proxy instances on each node

### Find envoy manager on target node
```
kubectl get pod -o wide
NAME                              READY     STATUS    RESTARTS   AGE       IP             NODE
traffic-envoy-manager-6f7nw       1/1       Running   0          9m        (node ip)  (targert node name)
```
### Check running envoy proxy instances
```
shell> kubectl exec traffic-envoy-manager-6f7nw ./envoy-tools
ID                                                                 |Pod                            |Namespace   |Status                  |State     |
--                                                                 |---                            |---------   |------                  |-----     |
86e56e0dc8e6a734fdb547aab60d9720117231742a32bdeaabc7dea6316a2f7b   |reviews-v3-5df889bcff-xdrwb    |default     |Up Less than a second   |running   |
a55a9126aef6f402e71c6c9ee61c3c0674aef8b71aeb40334fc1154695d80410   |reviews-v2-7ff5966b99-krw9s    |default     |Up 1 second             |running   |

shell> kubectl exec traffic-envoy-manager-6f7nw -- ./envoy-tools -id (prefix of the envoy id) -log
```

## Check envoy configuration
```
kubectl exec traffic-control-89778f5d8-nmvrn -- ./envoy-config --nodeId reviews-v3-5df889bcff-f2hgh.default
```
node id is pod name and pod namespace


# Circuit Breaker
| Resource | Labels | Default | Description |
|----------|--------|---------|--------------|
| Service | traffic.retries.max | 0 | max retries number |
| Service | traffic.connection.max | 0 | max number of connection | 
| Service | traffic.request.max-pending | 0 | max pending requests  |
| Service | traffic.request.max | 0 | max requests  |

# Other Configuration Labels
| Resource | Labels | Default | Description |
|----------|--------|---------|--------------|
| Pod, Deployment, StatefulSet, DaemonSet | traffic.envoy.enabled | false | whether to enable envoy docker for related pods|
| Pod | traffic.envoy.local.use_podip | false | whether to let envoy access local pod using pod ip instead of 127.0.0.1 |
| Pod, Service | traffic.port.(port number)| None| protocol for the port on service (http, tcp, direct)|
| Pod, Service | traffic.request.timeout | 0 | timeout in miliseconds |0 |
| Pod, Service | traffic.retries.5xx | 0 | number of retries for 5xx error | 
| Pod, Service | traffic.retries.connect-failure | 0 | number of retries for connect failure |
| Pod, Service | traffic.retries.gateway-error | 0 | number of retries for gateway error |
| Service | traffic.connection.timeout |  60000 | timeout in miliseconds  |

Note that all the service label configuration requires client pod's envoy enabled.

# Components
## envoy-manager
Responsible to start/stop envoy container for traffic-control

When a pod is envoy enabled, envoy-manager will start a envoy docker instance:
* Envoy docker git repo: https://github.com/luguoxiang/traffic-envoy
* The docker instance will share corresponding pod's network(docker container network mode).
* The iptable config of the pod network will be changed to redirect all incoming and outcoming traffic to envoy container listen port.
* The pod will be annotated with traffic.envoy.proxy=(docker id)

When user label the service with "traffic.envoy.enabled=false"

* the docker instance will be deleted
* the iptable config will be restored
* the pod will be annotated with traffic.envoy.proxy=""

## traffic-control
traffic-control is a control plane implementation of envoy proxy (https://www.envoyproxy.io/). 

