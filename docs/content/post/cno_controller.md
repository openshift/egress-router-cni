+++
author = "Daniel Mellado"
title = "Using Egress Router CNI with a CNO controller"
date = "2021-04-21"
description = "Using Egress Router CNI with its CNO controller and CRD"
tags = [
    "cni",
    "egress",
    "developer"
]
+++

If you're deploying Egress Router CNI on OpenShift, you can leverage on a
controller existing on the Cluster Network Operator (CNO), which is available
starting OpenShift 4.8 on.

You can create an EgressRouter object directly, without all the hassle of going
through the creation of the NetworkAttachmentDefinition and the pod manually.

## EgressRouter CR
```yaml
---
apiVersion: network.operator.openshift.io/v1
kind: EgressRouter
metadata:
  name: egress-router-test
spec:
  addresses: [
    {
      ip: "192.168.3.10",
      gateway: "192.168.3.1",
    },
  ]
  mode: Redirect
  redirect: {
    redirectRules: [
      {
        destinationIP: "10.100.3.0",
        port: 80,
        protocol: UDP,
      },
      {
        destinationIP: "203.0.113.26",
        port: 8080,
        protocol: TCP,
        targetPort: 80
      },
      {
        destinationIP: "203.0.113.27",
        port: 8443,
        protocol: TCP,
        targetPort: 443
      },
    ]
  }

```

> NOTE:
> Logging level and location modification is not supported when using CNO.


You can create an EgressRouter in this way:
```bash
$ oc create -f egress-router-cr.yaml
egressrouter.network.operator.openshift.io/egress-router-test created

$ oc get network-attachment-definition
NAME                    AGE
egress-router-cni-nad   7s

$ oc get deployment
NAME                           READY   UP-TO-DATE   AVAILABLE   AGE
egress-router-cni-deployment   1/1     1            1           13s
```

As a way of double-checking this, you can see that the CNI config provided to the NAD matches the values from your
Custom Resource
```bash
$oc get network-attachment-definition
NAME                    AGE
egress-router-cni-nad   7s
[dmellado@fedora  ~]$ oc describe network-attachment-definition egress-router-cni-nad
Name:         egress-router-cni-nad
Namespace:    default
Labels:       <none>
Annotations:  release.openshift.io/version: 4.8.0-fc.0
API Version:  k8s.cni.cncf.io/v1
Kind:         NetworkAttachmentDefinition
Metadata:
  Creation Timestamp:  2021-04-22T12:53:44Z
  Generation:          1
  Managed Fields:
    API Version:  k8s.cni.cncf.io/v1
    Fields Type:  FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .:
          f:release.openshift.io/version:
        f:ownerReferences:
          .:
          k:{"uid":"74c5d4a0-885d-4b20-b3b2-5f38c4b91d1a"}:
            .:
            f:apiVersion:
            f:controller:
            f:kind:
            f:name:
            f:uid:
      f:spec:
        .:
        f:config:
    Manager:    cluster-network-operator
    Operation:  Update
    Time:       2021-04-22T12:53:44Z
  Owner References:
    API Version:     network.operator.openshift.io/v1
    Controller:      true
    Kind:            EgressRouter
    Name:            egress-router-test
    UID:             74c5d4a0-885d-4b20-b3b2-5f38c4b91d1a
  Resource Version:  62605
  UID:               cea66a49-be5e-4066-ad42-9b8a07cf3a60
Spec:
  Config:  { "cniVersion": "0.4.0", "type": "egress-router", "name": "egress-router-cni-nad", "ip": { "addresses": [ "192.168.3.10/24" ], 
  "destinations": [ "80 UDP 10.100.3.0/30","8080 TCP 203.0.113.26/30 80","8443 TCP 203.0.113.27/30 443" ], "gateway": "192.168.3.1" },
   "log_file": "/tmp/egress-router-log", "log_level": "debug" }
Events:    <none>

```

As mentioned before, this creates the NetworkAttachmentDefinition (NAD) with
the CNI configuration, based on the CR values.

You can also check that the egress router pod is up and running

```bash
$ oc get po -o wide
NAME                                            READY   STATUS    RESTARTS   AGE   IP            NODE                          NOMINATED NODE   READINESS GATES
egress-router-cni-deployment-575465c75c-z2tcd   1/1     Running   0          89s   10.128.6.47   ip-10-0-143-39.ec2.internal   <none>           <none>
```

In case you need to debug this, Egress Router CNI also offers logs, on top of
what you would get from inspecting CRI-O ones, this can also be checked at the
node level.

```bash
$ oc debug node/ip-10-0-143-39.ec2.internal
Creating debug namespace/openshift-debug-node-nfvmn ...
Starting pod/ip-10-0-143-39ec2internal-debug ...
To use host binaries, run `chroot /host`
Pod IP: 10.0.143.39
If you don't see a command prompt, try pressing enter.
sh-4.4# chroot /host
sh-4.4# cat /tmp/egress-router-log
2021-04-21T14:30:47Z [debug] Called CNI ADD
2021-04-21T14:30:47Z [debug] Gateway: 192.168.3.1
2021-04-21T14:30:47Z [debug] IP Source Addresses: [192.168.3.10/24]
2021-04-21T14:30:47Z [debug] IP Destinations: [203.0.113.28/32]
2021-04-21T14:30:47Z [debug] Created macvlan interface
2021-04-21T14:30:47Z [debug] Renamed macvlan to "net1"
2021-04-21T14:30:47Z [debug] Adding route to gateway 192.168.3.1 on macvlan interface
2021-04-21T14:30:47Z [debug] deleted default route {Ifindex: 3 Dst: <nil> Src: <nil> Gw: 10.128.6.1 Flags: [] Table: 254}
2021-04-21T14:30:47Z [debug] Added new default route with gateway 192.168.3.1
2021-04-21T14:30:47Z [debug] Added iptables rule: iptables -t nat PREROUTING -i eth0 -j DNAT --to-destination 203.0.113.28
2021-04-21T14:30:47Z [debug] Added iptables rule: iptables -t nat -o net1 -j SNAT --to-source 192.168.3.10
```

Deleting the EgressRouter object would also delete the associated NAD router
pod

```bash
$ oc delete egressrouter egress-router-test
egressrouter.network.operator.openshift.io "egress-router-test" deleted

$ oc get network-attachment-definition
No resources found in default namespace.

$ oc get po -o wide
NAME                                            READY   STATUS        RESTARTS   AGE    IP            NODE                          NOMINATED NODE   READINESS GATES
egress-router-cni-deployment-575465c75c-z2tcd   0/1     Terminating   0          4m7s   10.128.6.47   ip-10-0-143-39.ec2.internal   <none>           <none>
```

