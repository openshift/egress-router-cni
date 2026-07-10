export GO111MODULE=on
unexport GOPATH

include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
     golang.mk \
     targets/openshift/deps.mk \
     targets/openshift/images.mk \
)

.PHONY: build-e2e-tests
build-e2e-tests:
	@echo "Building egress-router-cni-tests-ext binary..."
	$(MAKE) -C test build
