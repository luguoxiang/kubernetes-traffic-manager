# build stage
FROM golang:1.11-alpine AS build-env
RUN apk update
RUN apk add git
RUN apk add curl
ENV PROJECT_DIR /go/src/github.com/luguoxiang/k8s-traffic-manager
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh |sh
RUN mkdir -p ${PROJECT_DIR}/cmd
ENV GOPATH /go
WORKDIR ${PROJECT_DIR}
ADD Gopkg.lock .
ADD Gopkg.toml .
RUN dep ensure -vendor-only -v
ADD cmd cmd
ADD pkg pkg
RUN go build -o traffic-control-plane cmd/control-plane/controlplane.go
RUN go build -o envoy-tools cmd/envoy-tools/envoy-tools.go
RUN go build -o envoy-manager cmd/envoy-manager/envoy-manager.go

# final stage
FROM golang:1.11-alpine
WORKDIR /app
COPY --from=build-env /go/src/github.com/luguoxiang/k8s-traffic-manager/traffic-control-plane /app/
COPY --from=build-env /go/src/github.com/luguoxiang/k8s-traffic-manager/envoy-tools /app/
COPY --from=build-env /go/src/github.com/luguoxiang/k8s-traffic-manager/envoy-manager /app/
ENV https_proxy ""
ENV http_proxy ""

