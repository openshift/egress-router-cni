export GO111MODULE=on
unexport GOPATH

include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
     golang.mk \
	 targets/openshift/deps-gomod.mk \
     targets/openshift/deps.mk \
     targets/openshift/images.mk \
)

golangci-lint:
	golangci-lint run --verbose --print-resources-usage --modules-download-mode=vendor --timeout=5m0s
.PHONY: golangci-lint
