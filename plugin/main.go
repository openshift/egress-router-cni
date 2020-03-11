package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/coreos/go-iptables/iptables"
	"github.com/j-keck/arping"
	"github.com/vishvananda/netlink"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	IPv4InterfaceArpProxySysctlTemplate = "net.ipv4.conf.%s.proxy_arp"
	DisableIPv6SysctlTemplate           = "net.ipv6.conf.%s.disable_ipv6"
)

type ClusterConf struct {
	CloudProvider string `json:"cloudProvider"`
}

type NetConf struct {
	types.NetConf

	InterfaceType string            `json:"interfaceType"`
	InterfaceArgs map[string]string `json:"interfaceArgs"`

	IP       *IP           `json:"ip"`
	PodIP    map[string]IP `json:"podIP"`
	IPConfig *IPConfig     `json:"ipConfig"`
}

type IP struct {
	Addresses    []string `json:"addresses"`
	Gateway      string   `json:"gateway"`
	Destinations []string `json:"destinations"`
}

type IPConfig struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Overrides *IP    `json:"overrides"`
}

func loadNetConf(cluster *ClusterConf, bytes []byte) (*NetConf, error) {
	conf := &NetConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("failed to load netconf: %v", err)
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
		return fmt.Errorf("no interfaces to configure")
	}

	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", ifName, err)
	}

	if err := netlink.LinkSetUp(link); err != nil {
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
					// FIXME: log warning if unable to read sysctl value
					continue
				}

				// Write sysctl to enable IPv6
				_, err = sysctl.Sysctl(ipv6SysctlValueName, "0")
				if err != nil {
					return fmt.Errorf("failed to enable IPv6 for interface %q (%s=%s): %v", iface, ipv6SysctlValueName, value, err)
				}
			}
			has_enabled_ipv6 = true
		}

		addr := &netlink.Addr{IPNet: &ipc.Address, Label: ""}
		if err = netlink.AddrAdd(link, addr); err != nil {
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
				return fmt.Errorf("failed to add route '%v via %v dev %v': %v", r.Dst, gw, ifName, err)
			}
		}
	}

	return nil
}

func getDefaultRouteInterfaceName() (string, error) {
	routeToDstIP, err := netlink.RouteList(nil, netlink.FAMILY_ALL)
	if err != nil {
		return "", err
	}

	for _, v := range routeToDstIP {
		if v.Dst == nil {
			l, err := netlink.LinkByIndex(v.LinkIndex)
			if err != nil {
				return "", err
			}
			return l.Attrs().Name, nil
		}
	}

	return "", fmt.Errorf("no default route interface found")
}

func fillNetConfDefaults(conf *NetConf, cluster *ClusterConf) error {
	if conf.InterfaceType == "" {
		if cluster.CloudProvider == "" {
			conf.InterfaceType = "macvlan"
		} else {
			return fmt.Errorf("must specify explicit interfaceType for cloud provider %q", cluster.CloudProvider)
		}
	}

	switch conf.InterfaceType {
	case "macvlan":
		if conf.InterfaceArgs["master"] == "" {
			defaultRouteInterface, err := getDefaultRouteInterfaceName()
			if err != nil {
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
				return fmt.Errorf("unable to get MTU on master interface: %v", err)
			}
			conf.InterfaceArgs["mtu"] = strconv.Itoa(mtu)
		}
	}

	return nil
}

func loadIPConfig(ipc *IPConfig, podNamespace string) (*IP, map[string]IP, error) {
	if ipc.Namespace == "" {
		ipc.Namespace = podNamespace
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get in-cluster config")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get Kubernetes clientset")
	}

	cm, err := clientset.CoreV1().ConfigMaps(ipc.Namespace).Get(ipc.Name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get ConfigMap on namespace %s with name %s: %v", ipc.Namespace, ipc.Name)
	}

	if cm.Data["ip"] != "" {
		if cm.Data["podIP"] != "" {
			return nil, nil, fmt.Errorf("ConfigMap %s/%s contains both 'ip' and 'podIP'", ipc.Namespace, ipc.Name)
		}
		ip := &IP{}
		if err := json.Unmarshal([]byte(cm.Data["ip"]), ip); err != nil {
			return nil, nil, fmt.Errorf("failed to parse 'ip' in ConfigMap %s/%s: %v", ipc.Namespace, ipc.Name, err)
		}
		return ip, nil, nil
	} else if cm.Data["podIP"] != "" {
		podIP := map[string]IP{}
		if err := json.Unmarshal([]byte(cm.Data["podIP"]), podIP); err != nil {
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

	// There is a netns so try to clean up. Delete can be called multiple times
	// so don't return an error if the device is already removed.
	err := ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		if err := ip.DelLinkByName(args.IfName); err != nil {
			if err != ip.ErrLinkNotFound {
				return err
			}
		}
		return nil
	})

	return err
}

func macvlanCmdAdd(args *skel.CmdArgs) error {
	n, err := loadNetConf(&ClusterConf{}, args.StdinData)
	if err != nil {
		return err
	}

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
		return fmt.Errorf("unable to parse IP address %q: %v", n.IP.Addresses[0], err)
	}
	gw := net.ParseIP(n.IP.Gateway)
	dest, _, err := net.ParseCIDR(n.IP.Destinations[0])
	// Assume L2 interface only
	result := &current.Result{CNIVersion: n.CNIVersion, Interfaces: []*current.Interface{macvlanInterface}}
	result.IPs = append(result.IPs, &current.IPConfig{
		Version: "4",
		Address: net.IPNet{IP: ip, Mask: ipnet.Mask},
		Gateway: gw,
	})

	for _, ipc := range result.IPs {
		// All addresses apply to the container macvlan interface
		ipc.Interface = current.Int(0)
	}

	err = netns.Do(func(_ ns.NetNS) error {
		if err := configureIface(args.IfName, result); err != nil {
			return err
		}

		contVeth, err := net.InterfaceByName(args.IfName)
		if err != nil {
			return fmt.Errorf("failed to look up %q: %v", args.IfName, err)
		}

		for _, ipc := range result.IPs {
			if ipc.Version == "4" {
				_ = arping.GratuitousArpOverIface(ipc.Address.IP, *contVeth)
			}
		}

		ipt, err := iptables.New()
		if err != nil {
			return fmt.Errorf("failed to get IPTables: %v", err)
		}
		ipt.Append("nat", "PREROUTING", "-i", "eth0", "-j", "DNAT", "--to-destination", dest.String())
		ipt.Append("nat", "POSTROUTING", "-o", args.IfName, "-j", "SNAT", "--to-source", ip.String())

		return nil
	})
	if err != nil {
		return err
	}

	result.DNS = n.DNS

	return types.PrintResult(result, n.CNIVersion)
}

func getMTUByName(ifName string) (int, error) {
	link, err := netlink.LinkByName(ifName)
	if err != nil {
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

func createMacvlan(conf *NetConf, ifName string, netns ns.NetNS) (*current.Interface, error) {
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
		return nil, fmt.Errorf("failed to convert MTU %q to integer: %v")
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
		return nil, fmt.Errorf("failed to create macvlan: %v", err)
	}

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
			return fmt.Errorf("failed to rename macvlan to %q: %v", ifName, err)
		}
		macvlan.Name = ifName

		// Re-fetch macvlan to get all properties/attributes
		contMacvlan, err := netlink.LinkByName(ifName)
		if err != nil {
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

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("egress-router"))
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	macvlanCmdAdd(args)

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	macvlanCmdDel(args)

	return nil
}
