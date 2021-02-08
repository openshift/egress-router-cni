# egress-router plugin

## Overview

The `egress-router` plugin creates some sort of
cluster-network-external network interface and assigns a user-provided
static public IP address to it. It is designed for use by the
OpenShift egress-router feature, but may have other uses as well.

## Example configuration

```
{
	"name": "egress-router-1",
	"type": "egress-router",

	"ip": {
		"addresses": ["192.168.1.99/24"]
	}
}

{
	"name": "egress-router-2",
	"type": "egress-router",

	"interfaceArgs": {
		"master": "eth1",
	},
    "ip": {
      "addresses": [
        "192.168.111.200/24"
        ],
      "destinations": [
        "172.217.15.78/32"
      ],
      "gateway": "192.168.111.1"
      }
}

```

## Network configuration reference

* `name` (string, required): the name of the network
* `type` (string, required): `"openshift-egress"`
* `interfaceType` (string, optional): type of interface to create/use.
  * The `macvlan` and `ipvlan` options are available on all platforms (though they are not *useful* on some platforms).
  * On AWS, the `aws-elastic-ip` type is available.
  * If not specified, a default value will be chosen; see below.
* `interfaceArgs` (dictionary, optional): arguments specific to the `interfaceType` (see below).
* `ip` (dictionary, optional): IP configuration arguments:
  * `addresses` (array, required): IP addresses to configure on the interface
  * `gateway` (string, optional): IP address of the next-hop gateway, if it cannot be automatically determined
  * `destinations` (array, optional): list of CIDR blocks that the pod is allowed to connect to via this interface. If not provided, the pod can connect to any destination.


## Interface Types and Platform Support

On bare-metal nodes, `macvlan` is supported for `interfaceType`. For `macvlan`, `interfaceArgs` can include `mode` and `master`. However, you do not need to specify `master` if it can be inferred from the IP address. (That is, if there is exactly 1 network interface on the node whose configured IP is in the same CIDR range as the pod's configured IP, then that interface will automatically be used as the `master`, and the associated gateway will automatically be used as the `gateway`.)

## Routing

The newly-created interface will be made the default route for the pod (with the existing default route being removed). However, the previously-default interface will still be used as the route to the cluster and service networks. Additional routes may also be added as needed. For instance, when using `macvlan`, a route will be added to the master's IP via the pod network, since it would not be accessible via the macvlan interface.
