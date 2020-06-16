all: build
.PHONY: all

export GO111MODULE=on
export GOPROXY=https://proxy.golang.org
unexport GOPATH

GO_BUILD_BINDIR = _output
GO_BUILD_PACKAGES = \
    ./cmd/... 
GO_BUILD_PACKAGES_EXPANDED =$(shell GO111MODULE=on $(GO) list $(GO_MOD_FLAGS) $(GO_BUILD_PACKAGES))

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
        golang.mk \
        targets/openshift/deps.mk \
        targets/openshift/images.mk \
)
