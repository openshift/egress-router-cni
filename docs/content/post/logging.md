+++
author = "Daniel Mellado"
title = "Logging"
date = "2021-04-22"
description = "Using logging with Egress Router CNI"
tags = [
"cni",
"egress",
"developer"
]
+++

On top of the logging capabilities of Kubernetes, which would end up writing the Egress Router CNI logs within the CRI-O
logs, there's a built-in logging feature that can be enabled when creating the NetworkAttachmentDefinition (NAD).

The default configuration is set for it will write to `/tmp/egress-router-log` using `debug` loglevel, but this can be
overridden.

Sample NAD configuration:
```bash
{
	"cniVersion": "0.4.0",
	"type": "egress-router",
	"name": "egress-router",
	"ip": {
		"addresses": ["192.168.3.10/24"],
		"destinations": ["80 udp 10.100.3.0/30", "8080 tcp 203.0.113.26/30 80", "8443 tcp 203.0.113.27/30 443"],
		"gateway": "192.168.3.1"
	},
	"log_file": "/tmp/egress-router-log",
	"log_level": "debug"
}
```

The log file would contain details on the events processed by the CNI, such as destinations or iptables rules.
```bash
2021-04-22T14:55:34+02:00 [debug] Called CNI ADD
2021-04-22T14:55:34+02:00 [debug] Gateway: 192.168.3.1
2021-04-22T14:55:34+02:00 [debug] IP Source Addresses: [192.168.3.10/24]
2021-04-22T14:55:34+02:00 [debug] IP Destinations: [80 UDP 10.100.3.0/30 8080 TCP 203.0.113.26/30 80 8443 TCP 203.0.113.27/30 443]
2021-04-22T14:55:34+02:00 [debug] Created macvlan interface
2021-04-22T14:55:34+02:00 [debug] Renamed macvlan to "eth0"
2021-04-22T14:55:34+02:00 [debug] Adding route to gateway 192.168.3.1 on macvlan interface
2021-04-22T14:55:34+02:00 [debug] Added new default route with gateway 192.168.3.1
2021-04-22T14:55:34+02:00 [debug] Added iptables rule: iptables -t nat PREROUTING -i eth0 -p UDP --dport 80 -j DNAT --to-destination 10.100.3.0
2021-04-22T14:55:34+02:00 [debug] Added iptables rule: iptables -t nat PREROUTING -i eth0 -p TCP --dport 8080 -j DNAT --to-destination 203.0.113.26:80
2021-04-22T14:55:34+02:00 [debug] Added iptables rule: iptables -t nat PREROUTING -i eth0 -p TCP --dport 8443 -j DNAT --to-destination 203.0.113.27:443
2021-04-22T14:55:34+02:00 [debug] Added iptables rule: iptables -t nat -o eth0 -j SNAT --to-source 192.168.3.10
```