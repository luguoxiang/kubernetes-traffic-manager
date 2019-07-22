# traffic-agent
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

# traffic-control
traffic-control is a control plane implementation of envoy proxy (https://www.envoyproxy.io/). The data plane is group of "envoyproxy/envoy" images attached to k8s pods.





