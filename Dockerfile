FROM golang:1.12 as builder
WORKDIR /go/src/github.com/openshift/egress-router
COPY . .
RUN ./hack/build-go.sh

FROM openshift/origin-base

COPY --from=builder /go/src/github.com/openshift/egress-router/bin/egress-router /egress-router
