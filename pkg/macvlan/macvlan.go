package macvlan

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/coreos/go-iptables/iptables"
	"github.com/j-keck/arping"
	"github.com/openshift/egress-router-cni/pkg/util"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/egress-router-cni/pkg/logging"
	"github.com/openshift/egress-router-cni/pkg/types"
)

const (
	IPv4InterfaceArpProxySysctlTemplate = "net.ipv4.conf.%s.proxy_arp"
	DisableIPv6SysctlTemplate           = "net.ipv6.conf.%s.disable_ipv6"
)

func loadNetConf(cluster *types.ClusterConf, bytes []byte) (*types.NetConf, error) {
	conf := &types.NetConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, logging.Errorf("failed to load netconf: %v", err)
	}
	if err := fillNetConfDefaults(conf, cluster); err != nil {
		return nil, err
	}

	return conf, nil
}

// configureIface takes the result of IPAM plugin and
// applies to the ifName interface
func configureIface(ifName string, res *current.Result) error {
	if len(res.Interfaces) == 0 {
		logging.Errorf("no interfaces to configure")
		return fmt.Errorf("no interfaces to configure")
	}

	link, err := netlink.LinkByName(ifName)
	if err != nil {
		logging.Errorf("failed to lookup %q: %v", ifName, err)
		return fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
		logging.Errorf("failed to set %q UP: %v", ifName, err)
		return fmt.Errorf("failed to set %q UP: %v", ifName, err)
	}

	var v4gw, v6gw net.IP
	var has_enabled_ipv6 bool = false
	for _, ipc := range res.IPs {
		if ipc.Interface == nil {
			continue
		}
		intIdx := *ipc.Interface
		if intIdx < 0 || intIdx >= len(res.Interfaces) || res.Interfaces[intIdx].Name != ifName {
			// IP address is for a different interface
			logging.Errorf("failed to add IP addr %v to %q: invalid interface index", ipc, ifName)
			return fmt.Errorf("failed to add IP addr %v to %q: invalid interface index", ipc, ifName)
		}

		// Make sure sysctl "disable_ipv6" is 0 if we are about to add
		// an IPv6 address to the interface
		if !has_enabled_ipv6 && ipc.Version == "6" {
			// Enabled IPv6 for loopback "lo" and the interface
			// being configured
			for _, iface := range [2]string{"lo", ifName} {
				ipv6SysctlValueName := fmt.Sprintf(DisableIPv6SysctlTemplate, iface)

				// Read current sysctl value
				value, err := sysctl.Sysctl(ipv6SysctlValueName)
				if err != nil || value == "0" {
					logging.Errorf("Unable to read sysctl value %s", ipv6SysctlValueName)
					continue
				}

				// Write sysctl to enable IPv6
				_, err = sysctl.Sysctl(ipv6SysctlValueName, "0")
				if err != nil {
					logging.Errorf("failed to enable IPv6 for interface %q (%s=%s): %v", iface, ipv6SysctlValueName, value, err)
					return fmt.Errorf("failed to enable IPv6 for interface %q (%s=%s): %v", iface, ipv6SysctlValueName, value, err)
				}
			}
			has_enabled_ipv6 = true
		}

		addr := &netlink.Addr{IPNet: &ipc.Address, Label: ""}
		if err = netlink.AddrAdd(link, addr); err != nil {
			logging.Errorf("failed to add IP addr %v to %q: %v", ipc, ifName, err)
			return fmt.Errorf("failed to add IP addr %v to %q: %v", ipc, ifName, err)
		}

		gwIsV4 := ipc.Gateway.To4() != nil
		if gwIsV4 && v4gw == nil {
			v4gw = ipc.Gateway
		} else if !gwIsV4 && v6gw == nil {
			v6gw = ipc.Gateway
		}
	}

	if v6gw != nil {
		ip.SettleAddresses(ifName, 10)
	}

	for _, r := range res.Routes {
		routeIsV4 := r.Dst.IP.To4() != nil
		gw := r.GW
		if gw == nil {
			if routeIsV4 && v4gw != nil {
				gw = v4gw
			} else if !routeIsV4 && v6gw != nil {
				gw = v6gw
			}
		}

		if err = ip.AddRoute(&r.Dst, gw, link); err != nil {
			// we skip over duplicate routes as we assume the first one wins
			if !os.IsExist(err) {
				logging.Errorf("failed to add route '%v via %v dev %v': %v", r.Dst, gw, ifName, err)
				return fmt.Errorf("failed to add route '%v via %v dev %v': %v", r.Dst, gw, ifName, err)
			}
		}
	}

	return nil
}

func getDefaultRouteInterfaceName() (string, error) {
	routeToDstIP, err := util.GetNetLinkOps().RouteListFiltered(netlink.FAMILY_ALL, nil, netlink.RT_FILTER_OIF)
	if err != nil {
		return "", err
	}

	for _, v := range routeToDstIP {
		if v.Dst == nil {
			l, err := util.GetNetLinkOps().LinkByIndex(v.LinkIndex)
			if err != nil {
				return "", err
			}
			return l.Attrs().Name, nil
		}
	}
	logging.Errorf("no default route interface found")
	return "", fmt.Errorf("no default route interface found")
}

func fillNetConfDefaults(conf *types.NetConf, cluster *types.ClusterConf) error {
	if conf.LogFile != "" {
		logging.SetLogFile(conf.LogFile)
	}
	if conf.LogLevel != "" {
		logging.SetLogLevel(conf.LogLevel)
	}
	if conf.InterfaceType == "" {
		if cluster.CloudProvider == "" {
			conf.InterfaceType = "macvlan"
		} else {
			logging.Errorf("must specify explicit interfaceType for cloud provider %q", cluster.CloudProvider)
			return fmt.Errorf("must specify explicit interfaceType for cloud provider %q", cluster.CloudProvider)
		}
	}

	switch conf.InterfaceType {
	case "macvlan":
		if conf.InterfaceArgs["master"] == "" {
			defaultRouteInterface, err := getDefaultRouteInterfaceName()
			if err != nil {
				logging.Errorf("unable to get default route interface name: %v", err)
				return fmt.Errorf("unable to get default route interface name: %v", err)
			}
			if conf.InterfaceArgs == nil {
				conf.InterfaceArgs = make(map[string]string)
			}
			conf.InterfaceArgs["master"] = defaultRouteInterface
		}
		if conf.InterfaceArgs["mode"] == "" {
			conf.InterfaceArgs["mode"] = "bridge"
		}
		if conf.InterfaceArgs["mtu"] == "" {
			mtu, err := getMTUByName(conf.InterfaceArgs["master"])
			if err != nil {
				logging.Errorf("unable to get MTU on master interface: %v", err)
				return fmt.Errorf("unable to get MTU on master interface: %v", err)
			}
			conf.InterfaceArgs["mtu"] = strconv.Itoa(mtu)
		}
	}

	return nil
}

func loadIPConfig(ipc *types.IPConfig, podNamespace string) (*types.IP, map[string]types.IP, error) {
	if ipc.Namespace == "" {
		ipc.Namespace = podNamespace
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		logging.Errorf("failed to get in-cluster config")
		return nil, nil, fmt.Errorf("failed to get in-cluster config")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logging.Errorf("failed to get Kubernetes clientset")
		return nil, nil, fmt.Errorf("failed to get Kubernetes clientset")
	}

	cm, err := clientset.CoreV1().ConfigMaps(ipc.Namespace).Get(context.TODO(), ipc.Name, metav1.GetOptions{})
	if err != nil {
		logging.Errorf("failed to get ConfigMap on namespace %s with name %s", ipc.Namespace, ipc.Name)
		return nil, nil, fmt.Errorf("failed to get ConfigMap on namespace %s with name %s", ipc.Namespace, ipc.Name)
	}

	if cm.Data["ip"] != "" {
		if cm.Data["podIP"] != "" {
			logging.Errorf("ConfigMap %s/%s contains both 'ip' and 'podIP'", ipc.Namespace, ipc.Name)
			return nil, nil, fmt.Errorf("ConfigMap %s/%s contains both 'ip' and 'podIP'", ipc.Namespace, ipc.Name)
		}
		ip := &types.IP{}
		if err := json.Unmarshal([]byte(cm.Data["ip"]), ip); err != nil {
			logging.Errorf("failed to parse 'ip' in ConfigMap %s/%s: %v", ipc.Namespace, ipc.Name, err)
			return nil, nil, fmt.Errorf("failed to parse 'ip' in ConfigMap %s/%s: %v", ipc.Namespace, ipc.Name, err)
		}
		return ip, nil, nil
	} else if cm.Data["podIP"] != "" {
		podIP := map[string]types.IP{}
		if err := json.Unmarshal([]byte(cm.Data["podIP"]), &podIP); err != nil {
			logging.Errorf("failed to parse 'podIP' in ConfigMap %s/%s: %v", ipc.Namespace, ipc.Name, err)
			return nil, nil, fmt.Errorf("failed to parse 'podIP' in ConfigMap %s/%s: %v", ipc.Namespace, ipc.Name, err)
		}
		return nil, podIP, nil
	} else {
		return nil, nil, fmt.Errorf("ConfigMap %s/%s contains neither 'ip' nor 'podIP'", ipc.Namespace, ipc.Name)
	}
}

func macvlanCmdDel(args *skel.CmdArgs) error {
	if args.Netns == "" {
		return nil
	}
	logging.Debugf("Called CNI DEL")

	// There is a netns so try to clean up. Delete can be called multiple times
	// so don't return an error if the device is already removed.
	err := ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		if err := ip.DelLinkByName(args.IfName); err != nil {
			if err != ip.ErrLinkNotFound {
				logging.Errorf("CNI DEL failed, link not found: %s", err)
				return err
			}
			logging.Debugf("CNI DEL called")
		}
		return nil
	})

	return err
}

// validatePortRange validates the destination port value provided in the NAD.
// Accepts an argument of type string and returns error if the provided port value is invalid.
func validatePortRange(port string) error {
	dport, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	if dport < 0 || dport > 65535 {
		return fmt.Errorf("Port number out of range %v", port)
	}
	return nil
}

// generateDNATIPTablesRules creates the necessary IPTable rules to DNAT packets to remote destination.
// Accepts an array of strings repsenting allowedDestinations to which the router can talk to.
// Returns an error if invalid user input is detected at any point.
func generateDNATIPTablesRules(ipt *iptables.IPTables, allowedDestinations []string) error {
	if len(allowedDestinations) == 0 {
		logging.Debugf("No destination information has been provided")
		return nil
	}

	for _, allowedDestination := range allowedDestinations {

		destination := strings.Split(allowedDestination, " ")

		if len(destination) == 1 {
			// should be <IPaddress/mask> format

			dest := net.ParseIP(destination[0])

			ipt.Append("nat", "PREROUTING", "-i", "eth0", "-j", "DNAT", "--to-destination", dest.String())
			logging.Debugf("Added iptables rule: iptables -t nat PREROUTING -i eth0 -j DNAT --to-destination %s", dest.String())
		} else if len(destination) == 3 || len(destination) == 4 {
			// should be <localport protocol IPaddress/mask> format

			if err := validatePortRange(destination[0]); err != nil {
				logging.Errorf("Incorrect port number provided %v: %v", destination[0], err)
				return fmt.Errorf("Incorrect port number provided %v: %v", destination[0], err)
			}

			proto := strings.ToLower(destination[1])
			if !(proto == "tcp" || proto == "udp" || proto == "sctp") {
				logging.Errorf("Incorrect protocol provided %v", proto)
				return fmt.Errorf("Incorrect protocol number provided %v", proto)
			}

			dest := net.ParseIP(destination[2])

			if len(destination) == 4 && validatePortRange(destination[3]) == nil {
				// should be <localport protocol IPaddress/mask remoteport> format
				ipt.Append("nat", "PREROUTING", "-i", "eth0", "-p", proto, "--dport", destination[0], "-j", "DNAT", "--to-destination", dest.String()+":"+destination[3])
				logging.Debugf("Added iptables rule: iptables -t nat PREROUTING -i eth0 -p %s --dport %s -j DNAT --to-destination %s", proto, destination[0], dest.String()+":"+destination[3])
				continue
			}

			ipt.Append("nat", "PREROUTING", "-i", "eth0", "-p", proto, "--dport", destination[0], "-j", "DNAT", "--to-destination", dest.String())
			logging.Debugf("Added iptables rule: iptables -t nat PREROUTING -i eth0 -p %s --dport %s -j DNAT --to-destination %s", proto, destination[0], dest.String())
		} else {
			logging.Errorf("Invalid destination provided %v", allowedDestination)
			return fmt.Errorf("Invalid destination provided %v", allowedDestination)
		}
	}
	return nil
}

func macvlanCmdAdd(args *skel.CmdArgs) error {
	n, err := loadNetConf(&types.ClusterConf{}, args.StdinData)
	logging.Debugf("Called CNI ADD")
	if err != nil {
		return err
	}
	logging.Debugf("Gateway: %s", n.IP.Gateway)
	logging.Debugf("IP Source Addresses: %s", n.IP.Addresses)
	logging.Debugf("IP Destinations: %v", n.IP.Destinations)

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	macvlanInterface, err := createMacvlan(n, args.IfName, netns)
	if err != nil {
		return err
	}

	// Delete link if err to avoid link leak in this ns
	defer func() {
		if err != nil {
			netns.Do(func(_ ns.NetNS) error {
				return ip.DelLinkByName(args.IfName)
			})
		}
	}()

	ip, ipnet, err := net.ParseCIDR(n.IP.Addresses[0])
	if err != nil {
		logging.Errorf("unable to parse IP address %q: %v", n.IP.Addresses[0], err)
		return fmt.Errorf("unable to parse IP address %q: %v", n.IP.Addresses[0], err)
	}
	gw := net.ParseIP(n.IP.Gateway)
	allowedDestinations := n.IP.Destinations

	isIPv6 := isIPv6CIDR(ipnet)

	// Assume L2 interface only
	result := &current.Result{CNIVersion: n.CNIVersion, Interfaces: []*current.Interface{macvlanInterface}}
	if isIPv6 {
		result.IPs = append(result.IPs, &current.IPConfig{
			Version: "6",
			Address: net.IPNet{IP: ip, Mask: ipnet.Mask},
			Gateway: gw,
		})
	} else {
		result.IPs = append(result.IPs, &current.IPConfig{
			Version: "4",
			Address: net.IPNet{IP: ip, Mask: ipnet.Mask},
			Gateway: gw,
		})
	}

	for _, ipc := range result.IPs {
		// All addresses apply to the container macvlan interface
		ipc.Interface = current.Int(0)
	}

	err = netns.Do(func(_ ns.NetNS) error {
		// Configure interfaces IPAM
		if err := configureIface(args.IfName, result); err != nil {
			return err
		}

		// Get macvlan interface
		macvlanLink, err := netlink.LinkByName(args.IfName)
		if err != nil {
			logging.Errorf("could not get interface: %v", err)
			return fmt.Errorf("could not get interface: %v", err)
		}

		// Add route to gateway on macvlan interface
		var destIpNet net.IPNet
		if isIPv6 {
			destIpNet = net.IPNet{
				IP:   gw,
				Mask: net.CIDRMask(128, 128),
			}
			logging.Debugf("Adding IPv6 route to gateway %s on macvlan interface", gw)
		} else {
			destIpNet = net.IPNet{
				IP:   gw,
				Mask: net.CIDRMask(32, 32),
			}
			logging.Debugf("Adding IPv4 route to gateway %s on macvlan interface", gw)
		}
		newGatewayRoute := netlink.Route{
			LinkIndex: macvlanLink.Attrs().Index,
			Dst:       &destIpNet,
		}

		if err := netlink.RouteAdd(&newGatewayRoute); err != nil {
			logging.Errorf("failed to add new gateway default route : %v", err)
			return fmt.Errorf("failed to add new gateway default route : %v", err)
		}

		// Get default interface
		existingLink, err := netlink.LinkByName("eth0")
		if err != nil {
			logging.Errorf("couldn't get interface eth0: %v", err)
			return fmt.Errorf("couldn't get interface eth0: %v", err)
		}

		// Enable IP forwarding
		ipFamily := "ipv4"
		if isIPv6 {
			ipFamily = "ipv6"
		}
		_, err = sysctl.Sysctl(fmt.Sprintf("net.%s.ip_forward", ipFamily), "1")
		if err != nil {
			logging.Errorf("failed to enable %s forwarding: %v", ipFamily, err)
			return fmt.Errorf("failed to enable %s forwarding: %v", ipFamily, err)
		}

		// Delete default route
		var routes []netlink.Route
		if isIPv6 {
			routes, _ = netlink.RouteList(existingLink, netlink.FAMILY_V6)
		} else {
			routes, _ = netlink.RouteList(existingLink, netlink.FAMILY_V4)
		}

		for _, r := range routes {
			if r.Dst == nil {
				if err := netlink.RouteDel(&r); err != nil {
					logging.Errorf("failed to delete existing default route : %v", err)
					return fmt.Errorf("failed to delete existing default route : %v", err)
				}
				logging.Debugf("deleted default route %v", r)
			}
		}

		// Create new default route
		newDefaultRoute := netlink.Route{
			LinkIndex: macvlanLink.Attrs().Index,
			Dst:       nil,
			Gw:        gw,
		}

		if err := netlink.RouteAdd(&newDefaultRoute); err != nil {
			// Check if we already have route installed
			if !os.IsExist(err) {
				logging.Errorf("failed to add new default route, gw %v : %v", gw, err)
				return fmt.Errorf("failed to add new default route, gw %v : %v", gw, err)
			}
			logging.Debugf("Use existing route with gateway %v", gw)
		} else {
			logging.Debugf("Added new default route with gateway %v", gw)
		}
		contVeth, err := net.InterfaceByName(args.IfName)
		if err != nil {
			return fmt.Errorf("failed to look up %q: %v", args.IfName, err)
		}

		for _, ipc := range result.IPs {
			if ipc.Version == "4" || ipc.Version == "6" {
				_ = arping.GratuitousArpOverIface(ipc.Address.IP, *contVeth)
			}
		}

		var ipt *iptables.IPTables
		if isIPv6 {
			ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
		} else {
			ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
		}

		if err != nil {
			logging.Errorf("failed to get IPTables: %v", err)
			return fmt.Errorf("failed to get IPTables: %v", err)
		}

		if err := generateDNATIPTablesRules(ipt, allowedDestinations); err != nil {
			logging.Errorf("Invalid destination %v: %v", allowedDestinations, err)
			return fmt.Errorf("Invalid destination %v: %v", allowedDestinations, err)
		}
		ipt.Append("nat", "POSTROUTING", "-o", args.IfName, "-j", "SNAT", "--to-source", ip.String())
		logging.Debugf("Added iptables rule: iptables -t nat -o %s -j SNAT --to-source %s", args.IfName, ip.String())

		return nil
	})
	if err != nil {
		return err
	}

	result.DNS = n.DNS
	return cnitypes.PrintResult(result, n.CNIVersion)
}

func getMTUByName(ifName string) (int, error) {
	link, err := util.GetNetLinkOps().LinkByName(ifName)
	if err != nil {
		logging.Errorf("Failed to get MTU on link: %v", err)
		return 0, err
	}
	return link.Attrs().MTU, nil
}

func modeFromString(s string) (netlink.MacvlanMode, error) {
	switch s {
	case "bridge":
		return netlink.MACVLAN_MODE_BRIDGE, nil
	case "private":
		return netlink.MACVLAN_MODE_PRIVATE, nil
	case "vepa":
		return netlink.MACVLAN_MODE_VEPA, nil
	case "passthru":
		return netlink.MACVLAN_MODE_PASSTHRU, nil
	default:
		return 0, fmt.Errorf("unknown macvlan mode: %q", s)
	}
}

func createMacvlan(conf *types.NetConf, ifName string, netns ns.NetNS) (*current.Interface, error) {
	macvlan := &current.Interface{}

	mode, err := modeFromString(conf.InterfaceArgs["mode"])
	if err != nil {
		return nil, err
	}

	m, err := netlink.LinkByName(conf.InterfaceArgs["master"])
	if err != nil {
		return nil, fmt.Errorf("failed to lookup master %q: %v", conf.InterfaceArgs["master"], err)
	}

	mtu, err := strconv.Atoi(conf.InterfaceArgs["mtu"])
	if err != nil {
		return nil, fmt.Errorf("failed to convert MTU to integer: %v", conf.InterfaceArgs["mtu"])
	}

	// due to kernel bug we have to create with tmpName or it might
	// collide with the name on the host and error out
	tmpName, err := ip.RandomVethName()
	if err != nil {
		return nil, err
	}

	mv := &netlink.Macvlan{
		LinkAttrs: netlink.LinkAttrs{
			MTU:         mtu,
			Name:        tmpName,
			ParentIndex: m.Attrs().Index,
			Namespace:   netlink.NsFd(int(netns.Fd())),
		},
		Mode: mode,
	}

	if err := netlink.LinkAdd(mv); err != nil {
		logging.Errorf("failed to create macvlan: %v", err)
		return nil, fmt.Errorf("failed to create macvlan: %v", err)
	}
	logging.Debugf("Created macvlan interface")

	err = netns.Do(func(_ ns.NetNS) error {
		// TODO: duplicate following lines for ipv6 support, when it will be added in other places
		ipv4SysctlValueName := fmt.Sprintf(IPv4InterfaceArpProxySysctlTemplate, tmpName)
		if _, err := sysctl.Sysctl(ipv4SysctlValueName, "1"); err != nil {
			// remove the newly added link and ignore errors, because we already are in a failed state
			_ = netlink.LinkDel(mv)
			return fmt.Errorf("failed to set proxy_arp on newly added interface %q: %v", tmpName, err)
		}

		err := ip.RenameLink(tmpName, ifName)
		if err != nil {
			_ = netlink.LinkDel(mv)
			logging.Errorf("failed to rename macvlan to %q: %v", ifName, err)
			return fmt.Errorf("failed to rename macvlan to %q: %v", ifName, err)
		}
		logging.Debugf("Renamed macvlan to %q", ifName)
		macvlan.Name = ifName

		// Re-fetch macvlan to get all properties/attributes
		contMacvlan, err := netlink.LinkByName(ifName)
		if err != nil {
			logging.Errorf("failed to refetch macvlan %q: %v", ifName, err)
			return fmt.Errorf("failed to refetch macvlan %q: %v", ifName, err)
		}
		macvlan.Mac = contMacvlan.Attrs().HardwareAddr.String()
		macvlan.Sandbox = netns.Path()

		return nil
	})
	if err != nil {
		return nil, err
	}

	return macvlan, nil
}

func CmdCheck(args *skel.CmdArgs) error {
	return nil
}

func CmdAdd(args *skel.CmdArgs) error {
	return macvlanCmdAdd(args)
}

func CmdDel(args *skel.CmdArgs) error {
	return macvlanCmdDel(args)
}

func isIPv6CIDR(cidr *net.IPNet) bool {
	return cidr.IP != nil && cidr.IP.To4() == nil
}
