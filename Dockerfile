FROM golang:1.13 as builder
WORKDIR /go/src/github.com/openshift/egress-router-cni
COPY . .
RUN ./hack/build-go.sh

FROM openshift/origin-base

COPY --from=builder /go/src/github.com/openshift/egress-router-cni/bin/egress-router /egress-router
