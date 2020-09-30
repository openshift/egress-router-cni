FROM golang:1.13 as builder

ENV GOPATH /go
WORKDIR /go/src/github.com/openshift/egress-router-cni

COPY . .
RUN ./hack/build-go.sh

FROM openshift/origin-base
COPY --from=builder /go/src/github.com/openshift/egress-router-cni/bin/egress-router /egress-router

LABEL io.k8s.display-name="Egress Router CNI" \
      io.k8s.description="CNI Plugin for Egress Router" \
      io.openshift.tags="openshift" \
      maintainer="Daniel Mellado <dmellado@redhat.com>"
