module github.com/openshift/egress-router-cni

go 1.13

require (
	github.com/containernetworking/cni v0.7.1
	github.com/containernetworking/plugins v0.8.2
	github.com/coreos/go-iptables v0.4.3
	github.com/j-keck/arping v1.0.0
	github.com/vishvananda/netlink v1.0.0
	k8s.io/apimachinery v0.0.0-20190913080033-27d36303b655
	k8s.io/client-go v0.0.0-20190918160344-1fbdaa4c8d90
)
