.PHONY: build build.images push.images

VERSION=1.0

all: build.images push.images

vendor:
	dep ensure -vendor-only -v
	
clean:
	rm -f traffic-control-plane envoy-tools envoy-manager
	
build: vendor
	go build -v -o bin/traffic-control-plane cmd/control-plane/main.go
	go build -v -o bin/envoy-config cmd/envoy-config/main.go
	go build -v -o bin/envoy-tools cmd/envoy-tools/main.go
	go build -v -o bin/envoy-manager cmd/envoy-manager/main.go

test: vendor
	go test -v github.com/luguoxiang/kubernetes-traffic-manager/pkg/...

build.images.envoy: 
	docker build -t luguoxiang/envoy-manager:${VERSION} -f Dockerfile.envoy .
	docker push luguoxiang/envoy-manager:${VERSION}

build.images.control: 
	docker build -t luguoxiang/traffic-control:${VERSION} -f Dockerfile.control .
	docker push luguoxiang/traffic-control:${VERSION}

build.images: build.images.envoy build.images.control
