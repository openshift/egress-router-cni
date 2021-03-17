export GO111MODULE=on
unexport GOPATH

include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
     golang.mk \
     targets/openshift/deps.mk \
     targets/openshift/images.mk \
)
