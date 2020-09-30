FROM registry.svc.ci.openshift.org/ocp/builder:rhel-8-golang-1.15-openshift-4.6 as rhel8
FROM golang:1.13 as builder
WORKDIR /go/src/github.com/openshift/egress-router-cni
COPY . .
RUN ./hack/build-go.sh

FROM openshift/origin-base
COPY --from=builder /go/src/github.com/openshift/egress-router-cni/bin/egress-router /usr/src/egress-router-cni/bin/egress-router
LABEL io.k8s.display-name="Egress Router CNI" \
      io.k8s.description="CNI Plugin for Egress Router" \
      io.openshift.tags="openshift" \
      maintainer="Daniel Mellado <dmellado@redhat.com>"
