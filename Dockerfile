FROM golang:1.19
ADD . /usr/src/egress-router-cni
RUN mkdir -p $GOPATH/src/github.com/openshift/egress-router-cni
WORKDIR $GOPATH/src/github.com/openshift/egress-router-cni
COPY . .
RUN ./hack/build-go.sh

FROM alpine:latest
COPY --from=0 /go/src/github.com/openshift/egress-router-cni/bin/egress-router /usr/src/egress-router-cni/bin/egress-router
