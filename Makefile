.PHONY: build build.images push.images

all: build.images push.images

vendor:
	dep ensure -vendor-only -v
	
clean:
	rm -f traffic-control-plane envoy-tools envoy-manager
	
build: vendor
	go build -v -o bin/traffic-control-plane cmd/control-plane/controlplane.go
	go build -v -o bin/envoy-config cmd/envoy-config/envoy-config.go
	go build -v -o bin/envoy-tools cmd/envoy-tools/envoy-tools.go
	go build -v -o bin/envoy-manager cmd/envoy-manager/envoy-manager.go

test: vendor
	go test -v github.com/luguoxiang/kubernetes-traffic-manager/pkg/...

build.images.envoy: 
	docker build -t luguoxiang/envoy-manager -f Dockerfile.envoy .
	docker push luguoxiang/envoy-manager

build.images.control: 
	docker build -t luguoxiang/traffic-control -f Dockerfile.control .
	docker push luguoxiang/traffic-control

build.images: build.images.envoy build.images.control
