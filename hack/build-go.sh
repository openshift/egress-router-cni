#!/usr/bin/env bash

set -eu

REPO=github.com/openshift/egress-router-cni
CMD="egress-router"
GLDFLAGS=${GLDFLAGS:-}
GOFLAGS=${GOFLAGS:--mod=vendor}

eval $(go env | grep -e "GOHOSTOS" -e "GOHOSTARCH")

: "${GOOS:=${GOHOSTOS}}"
: "${GOARCH:=${GOHOSTARCH}}"

cd "$(git rev-parse --show-cdup)"

eval $(go env)
GO111MODULE=${GO111MODULE} CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS} -ldflags "${GLDFLAGS}" -o bin/${CMD} ${REPO}/cmd/${CMD}
#CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build ${GOFLAGS} -ldflags "${GLDFLAGS}" -o bin/${cmd} cmd/${cmd}.go
