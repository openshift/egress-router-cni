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

	"interfaceType": "ipvlan",
	"interfaceArgs": {
		"master": "eth1",
	},
	"podIP": {
		"db-router": {
			"addresses": ["192.168.3.10/24"],
			"gateway": "192.168.3.1",
			"destinations": ["10.1.2.3/32"]
		},
		"metrics-router": {
			"addresses": ["192.168.3.11/24"]
			"gateway": "192.168.3.1",
		},
		"alpha-*": {
			"addresses": ["192.168.3.12/24"]
			"gateway": "192.168.3.1",
		}
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
* `podIP` (dictionary, optional): map providing per-pod IP configuration. Each map entry has the name of a pod (possibly ending with `"*"`) and a value that is a dictionary like the `ip` argument.
* `ipConfig` (dictionary, optional): pointer to external IP configuration information:
  * `namespace` (string, optional): namespace to look up the IP `ConfigMap` in; defaults to the namespace of the pod if not specified.
  * `name` (string, required): name of the `ConfigMap` containing IP configuration
  * `overrides` (dictionary, optional): dictionary in the format of `ip` providing overrides to the configuration in the `ConfigMap`

Exactly one of `ip`, `podIP`, or `ipConfig` must be provided.

## Interface Types and Platform Support

On bare-metal nodes, `macvlan` and `ipvlan` are supported for `interfaceType`, with `macvlan` being the default. For `macvlan`, `interfaceArgs` can include `mode` and `master`, and for `ipvlan` it can include `master`. However, you do not need to specify `master` if it can be inferred from the IP address. (That is, if there is exactly 1 network interface on the node whose configured IP is in the same CIDR range as the pod's configured IP, then that interface will automatically be used as the `master`, and the associated gateway will automatically be used as the `gateway`.)

On AWS, the default `interfaceType` is `aws-elastic-ip`. The administrator is responsible for allocating the Elastic IP address (and for deallocating it when it is no longer needed). The plugin will handle associating the Elastic IP with the node hosting the pod (and moving it to another node if the pod moves).

## IP Configuration

The configuration must specify exactly one of `ip`, `podIP`, or `ipConfig`. The first two forms configure IP addresses staticly in the network definition, while `ipConfig` allows dynamic configuration.

The value of `ipConfig` must include at least the name (and optionally the namespace) of a `ConfigMap` whose `data` must include either an `ip` entry or a `podIP` entry, in the same format as used by the CNI configuration. (If there are other fields set in the `ConfigMap` they will be ignored.) By default, the `ip`/`podIP` value in the `ConfigMap` will be interpreted just as it would be if it had been in the CNI config directly. However, if the `ipConfig` specifies `overrides`, then:

  1. If `overrides.addresses` is set, then the `ConfigMap` is only allowed to assign `addresses` values that are present in `overrides.addresses`.
  2. If `overrides.gateway` is set, then it is used as the default `gateway` value and the `ConfigMap` is not allowed to specify any other value.
  3. If `overrides.destinations` is set, then it is used as the default `destinations` value, and any `destinations` specified in the `ConfigMap` are intersected with it.

## Routing

The newly-created interface will be made the default route for the pod (with the existing default route being removed). However, the previously-default interface will still be used as the route to the cluster and service networks. Additional routes may also be added as needed. For instance, when using `macvlan`, a route will be added to the master's IP via the pod network, since it would not be accessible via the macvlan interface.

