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

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target name
# $2 - image ref
# $3 - Dockerfile path
# $4 - context directory for image build
# It will generate target "image-$(1)" for builing the image and binding it as a prerequisite to target "images".

$(call build-image,egress-router,egress-router,./images/egress-router/Dockerfile,.)
