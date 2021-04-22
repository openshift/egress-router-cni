+++
author = "Daniel Mellado"
title = "Multiple destinations"
date = "2021-04-22"
description = "Using Egress Router CNI with multiple destinations in redirect mode"
tags = [
"cni",
"egress",
"developer"
]
+++

Egress Router CNI now offers you the possibility of selecting several destinations. On top of that, you would be able to
use these formats as destination.
 * "ip-address/mask"
 * "port protocol ip-address/mask"
 * "port protocol ip-address/mask remote-port"

You can see an example of such configuration below:
```bash
{
	"cniVersion": "0.4.0",
	"type": "egress-router",
	"name": "egress-router",
	"ip": {
		"addresses": ["192.168.3.10/24"],
		"destinations": ["80 UDP 10.100.3.0/30",
		                 "8080 TCP 203.0.113.26/30 80",
		                 "8443 TCP 203.0.113.27/30 443"],
		"gateway": "192.168.3.1"
	},
	"log_file": "/tmp/egress-router-log",
	"log_level": "debug"
}
```