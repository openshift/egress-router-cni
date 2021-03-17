module github.com/openshift/egress-router-cni

go 1.13

require (
	github.com/containernetworking/cni v0.8.0
	github.com/containernetworking/plugins v0.8.7
	github.com/coreos/go-iptables v0.4.5
	github.com/j-keck/arping v1.0.0
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/openshift/build-machinery-go v0.0.0-20200512074546-3744767c4131
	github.com/pkg/errors v0.9.1
	github.com/vishvananda/netlink v1.0.0
	github.com/stretchr/testify v1.6.1
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
)
