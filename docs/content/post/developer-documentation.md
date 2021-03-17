+++
author = "Daniel Mellado"
title = "Developer Documentation"
date = "2020-10-12"
description = "Developer Docs for Egress Router CNI"
tags = [
    "cni",
    "egress",
    "developer"
]
+++

This article covers how to install and test the Egress Router CNI, both using
cnitool or deploying Openshift and using its official image.

## Using cnitool

[cnitool](https://github.com/containernetworking/cni/tree/master/cnitool) is
a simple program that executes a CNI configuration. It will add or
remove an interface in an already-created network namespace. You can use it in
order to test your egress router cni configuration and development.

Use `hack/build-go.sh` to create a binary of the egress router.

Create a config file in json format called egress-router.conf for the CNI
 plugin, such as:

```json
{
   "cniVersion":"0.4.0",
   "name":"egress-router",
   "type":"egress-router",
   "ip":{
      "addresses":[
         "192.168.10.99/24"
      ],
      "destinations":[
         "10.0.3.0/32"
      ],
      "gateway":"192.168.10.254"
   }
}
```

Then you can call cnitool in order to test your configuration, and it'll output
a nice json back.

```bash
#!/bin/bash

sudo ip netns add testing
sudo CNI_PATH=egress-router-cni/bin NETCONFPATH=. cnitool add egress-router /var/run/netns/testing
```

```json
{
    "cniVersion": "0.4.0",
    "interfaces": [
        {
            "name": "eth0",
            "mac": "1a:9b:80:a1:1e:99",
            "sandbox": "/var/run/netns/testing"
        }
    ],
    "ips": [
        {
            "version": "4",
            "interface": 0,
            "address": "192.168.10.99/24",
            "gateway": "192.168.10.254"
        }
    ],
    "dns": {}
```

Now we can just enter that namespace and check that the macvlan interface and
the iptables rules were applied.

```bash
$ sudo ip netns exec testing ip a
1: lo: <LOOPBACK> mtu 65536 qdisc noop state DOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: eth0@if6: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
    link/ether 1a:9b:80:a1:1e:99 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 192.168.10.99/24 brd 192.168.10.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::189b:80ff:fea1:1e99/64 scope link
       valid_lft forever preferred_lft forever
```

```bash
$ sudo ip netns exec testing iptables -t nat -L
Chain PREROUTING (policy ACCEPT)
target     prot opt source               destination
DNAT       all  --  anywhere             anywhere             to:10.0.3.0

Chain INPUT (policy ACCEPT)
target     prot opt source               destination

Chain OUTPUT (policy ACCEPT)
target     prot opt source               destination

Chain POSTROUTING (policy ACCEPT)
target     prot opt source               destination
SNAT       all  --  anywhere             anywhere             to:192.168.10.99

```

## Using quay.io/openshift image

In order to deploy this into a cluster, you'll have to build the image locally
using the provided `Dockerfile` or rely into the preexising images that are shipped into
Openshift's quay
(https://quay.io/repository/openshift/origin-egress-router-cni).


By default the egress router image is deployed within the CNO's multus
 daemonsets for all Openshift clusters. However, If a custom image is required,
 the `EGRESS_ROUTER_CNI_IMAGE` value can be specified in the
env.sh file for CNO's `hack/run-locally`.

If you don't want to use CNO at all, you can still install the egress router
cni by manually copying it to the CNI binary path, which should be
`/var/lib/cni/bin/`

### How to use Egress-Router-CNI

Egress Router CNI aims to match openshift-sdn's functionality, please refer to
[this nice Openshift blog article](https://www.openshift.com/blog/accessing-external-services-using-egress-router)
for further details on it. In order to make things easier for the legacy router
users, so far the `redirect` mode is supported in egress-router-cni.

In openshift-sdn, the egress router was implemented by adding an annotation to
allow a pod to request a macvlan interface. In contrast, the new egress-router
implementation uses CNI to create the macvlan interface and so can be used
with any network plugin.

(i.e. it's not an ovn-kubernetes. It's a network-plugin-agnostic feature.)

Please take a look at the [enhancement
proposal](https://github.com/openshift/enhancements/blob/master/enhancements/network/egress-router.md)
if you want to get some more detailed info on this.

Assumming that you have the CNI plugin installed, you'd be able to create a
NetworkAttachmentDefinition (NAD) with the CNI configuration alongside, as in the below example.

> NOTE:
> Multus is a requirement for egress-router-cni usage.

The 'NetworkAttachmentDefinition' is used to setup the network attachment, i.e.
secondary interface for the pod, There are two ways to configure the
'NetworkAttachmentDefinition' as following:

* NetworkAttachmentDefinition with json CNI config
* NetworkAttachmentDefinition with CNI config file

#### NetworkAttachmentDefinition with json CNI config

```yaml
---
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: egress-router
spec:
  config: '{
    "cniVersion": "0.4.0",
    "type": "egress-router",
    "name": "egress-router",
    "ip": {
      "addresses": [
        "192.168.123.99"
        ],
      "destinations": [
        "192.168.123.91"
      ],
      "gateway": "192.168.123.1"
      }
    }'
```

This would create the additional network, which would be later used in the pod
with the macvlan interface.

Let's go create the pod!

> NOTE:
> A pod image with iptables is required in order to use it to see the created
> iptables rules, but it is NOT for the egress-router-cni to work.

#### Egress Router Pod

```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-router-pod
  annotations:
    k8s.v1.cni.cncf.io/networks: egress-router
spec:
  containers:
    - name: openshift-egress-router-pod
      command: ["/bin/bash", "-c", "sleep 999999999"]
      image: centos/tools
      securityContext:
        privileged: true

```

If we now check out the annotations from the new pod, we'd be able to
see that it has two interfaces: the default one and another atached to the NAD
that we just created previously.

```bash
Annotations:  k8s.ovn.org/pod-networks:
                {"default":{"ip_addresses":["10.131.0.12/23"],"mac_address":"0a:58:0a:83:00:0c","gateway_ips":["10.131.0.1"],"ip_address":"10.131.0.12/23"...
              k8s.v1.cni.cncf.io/network-status:
                [{
                    "name": "",
                    "interface": "eth0",
                    "ips": [
                        "10.131.0.12"
                    ],
                    "mac": "0a:58:0a:83:00:0c",
                    "default": true,
                    "dns": {}
                },{
                    "name": "default/egress-router",
                    "interface": "net1",
                    "ips": [
                        "10.200.16.0"
                    ],
                    "mac": "a6:e3:20:ae:a9:69",
                    "dns": {}
                }]

```

Also, inside the egress router pod the iptables rules would've been applied,
pretty much in the same way as we showed before.

> NOTE:
> Depending on the iptables version on the pod and the host, some `legacy`
> iptables rules might not be showing from the pod, we'll explain how to check
> that directly from the host.

```bash
[dsal@bkr-hv02  ~]$ oc rsh egress-router-pod
sh-4.2# iptables-save -t nat
Chain PREROUTING (policy ACCEPT)
target     prot opt source               destination
DNAT       all  --  anywhere             anywhere             to:10.0.3.0

Chain INPUT (policy ACCEPT)
target     prot opt source               destination

Chain OUTPUT (policy ACCEPT)
target     prot opt source               destination

Chain POSTROUTING (policy ACCEPT)
target     prot opt source               destination
SNAT       all  --  anywhere             anywhere             to:192.168.10.99
```

In case you don't see any iptables rule from the pod, you can always get them
from the network namespace the pod is running at.

```bash
[dsal@bkr-hv02  ~]$ oc get po -o wide
NAME                       READY   STATUS    RESTARTS   AGE    IP            NODE       NOMINATED NODE   READINESS GATES
egress-router-pod          1/1     Running   0          2m     10.131.0.12   worker-1   <none>           <none>
```

Go to the node where the pod is running and check for the container id.
```bash
ssh worker-1
[core@worker-1 ~]$ sudo crictl ps | grep egress-router-pod
CONTAINER           IMAGE                                                                                                                    CREATED             STATE               NAME                          ATTEMPT             POD ID
847324ac0ee0b       docker.io/centos/tools@sha256:81159542603c2349a276a30cc147045bc642bd84a62c1b427a8d243ef1893e2f                           2 minutes ago       Running             openshift-egress-router-pod   0                   86a48fb69eb7f
```

Get the pid from that container.
```bash
[core@worker-1 ~]$ sudo crictl inspect 847324ac0ee0b | grep pid
    "pid": 302260,
          "pids": {
            "type": "pid"
```

Enter the network namespace.
```bash
[core@worker-1 ~]$ sudo nsenter -n -t 302260
```

Now you'd be able to check the iptables rules from there, even if your pod
isn't running any kind of iptables.
```bash
[root@worker-1 core]# iptables-save -t nat
# Generated by iptables-save v1.8.4 on Fri Dec 11 15:29:48 2020
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
-A PREROUTING -i eth0 -j DNAT --to-destination 10.100.3.0
-A POSTROUTING -o net1 -j SNAT --to-source 10.200.16.0
COMMIT
```
