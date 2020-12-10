+++
author = "Daniel Mellado"
title = "Examples"
date = "2020-10-15"
description = "Example configs for Egress Router CNI"
tags = [
    "cni",
    "egress",
    "developer"
]
+++

## Example configurations for both NetworkAttachmentDefinition and pod

### NetworkAttachmentDefinition with json CNI config
```yaml
---
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: egress-router-cni-net
spec:
  config: '{
    "cniVersion": "0.4.0",
    "type": "egress-router",
    "name": "egress-router-cni-net",
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

### NetworkAttachmentDefinition with CNI config file
If the NetworkAttatchmentDefinition has no spec, multus will look for a file
 in the defaultConfDir (‘/etc/cni/multus/net.d’), with the same name that's
 specified in the CNI config.

```yaml
---
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: egress-router-cni-net
```

```bash
$ cat <<EOF > /etc/cni/multus/net.d/egress-router.conf
{
  "cniVersion": "0.4.0",
  "type": "egress-router",
  "name": "egress-router-cni-net",
  "ip": {
    "addresses": [
      "192.168.123.99"
      ],
    "destinations": [
      "192.168.123.91"
    ],
    "gateway": "192.168.123.1"
    }
}
EOF
```
### Egress Router Pod with text annotation
```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-router-pod-text-annotation
  annotations:
    k8s.v1.cni.cncf.io/networks: egress-router-cni-net
spec:
  containers:
  - name: egress-router-pod-test-annotation
    image: docker.io/centos/tools:latest
    command:
    - /sbin/init
```

### Egress Router Pod with interface name
```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-router-pod-interface-name
  annotations:
    k8s.v1.cni.cncf.io/networks: egress-router-cni-net@macvlan0
spec:
  containers:
  - name: egress-router-pod-container
    image: docker.io/centos/tools:latest
    command:
    - /sbin/init
```

### Egress Router Pod with json annotation
```yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: egress-router-pod-json-annotation
  annotations:
    k8s.v1.cni.cncf.io/networks: '[
            { "name" : "egress-router-cni-net" },
    ]'
spec:
  containers:
  - name: egress-router-pod-container
    image: docker.io/centos/tools:latest
    command:
    - /sbin/init
```
