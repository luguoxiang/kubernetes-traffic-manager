.PHONY: build build.images push.images

all: build.images push.images

vendor:
	dep ensure -vendor-only -v
	
clean:
	rm -f traffic-control-plane envoy-tools envoy-manager
	
build.client: vendor
	go build -o envoy_client cmd/client-test/envoy_client.go 

build: vendor
	go build -v -o traffic-control-plane cmd/control-plane/controlplane.go
	go build -v  -o envoy-tools cmd/envoy-tools/envoy-tools.go
	go build -v -o envoy-manager cmd/envoy-manager/envoy-manager.go

test: vendor
	go test -v github.com/luguoxiang/kubernetes-traffic-manager/pkg/...

build.images: 
	docker build -t luguoxiang/envoy-manager -f Dockerfile.envoy .
	docker build -t luguoxiang/traffic-control -f Dockerfile.control .

push.images: 
	docker push luguoxiang/traffic-control
	docker push luguoxiang/envoy-manager
