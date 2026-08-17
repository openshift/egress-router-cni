FROM golang:1.26.6-trixie@sha256:b75d466dd608587fd66cca705a307ba65b889827d06ad61d6a75f0482b51b7c7
ADD . /usr/src/egress-router-cni
RUN mkdir -p $GOPATH/src/github.com/openshift/egress-router-cni
WORKDIR $GOPATH/src/github.com/openshift/egress-router-cni
COPY . .
RUN ./hack/build-go.sh

FROM scratch
COPY --from=0 /go/src/github.com/openshift/egress-router-cni/bin/egress-router /usr/src/egress-router-cni/bin/egress-router
