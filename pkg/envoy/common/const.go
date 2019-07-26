package common

const (
	typePrefix            = "type.googleapis.com/envoy.api.v2."
	ClusterResource       = typePrefix + "Cluster"
	EndpointResource      = typePrefix + "ClusterLoadAssignment"
	RouteResource         = typePrefix + "RouteConfiguration"
	ListenerResource      = typePrefix + "Listener"
	SecretResource        = typePrefix + "auth.Secret"
	XdsCluster            = "xds_cluster"
	RouterHttpFilter      = "envoy.router"
	HTTPConnectionManager = "envoy.http_connection_manager"
	TCPProxy              = "envoy.tcp_proxy"
	TLS_INSPECTOR         = "envoy.listener.tls_inspector"
	ORIGINAL_DST          = "envoy.listener.original_dst"
	HttpFaultInjection    = "envoy.fault"

	PROTO_DIRECT = "direct"
	PROTO_HTTP   = "http"
)
