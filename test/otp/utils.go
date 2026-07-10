package otp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func logf(format string, args ...interface{}) {
	fmt.Fprintf(g.GinkgoWriter, format+"\n", args...)
}

type ocCLI struct {
	execPath         string
	kubeconfig       string
	namespace        string
	withoutNamespace bool
	asAdmin          bool
}

func newOC(namespace string) *ocCLI {
	return &ocCLI{
		execPath:   "oc",
		kubeconfig: os.Getenv("KUBECONFIG"),
		namespace:  namespace,
	}
}

func (c *ocCLI) asAdminCLI() *ocCLI {
	nc := *c
	nc.asAdmin = true
	return &nc
}

func (c *ocCLI) withoutNS() *ocCLI {
	nc := *c
	nc.withoutNamespace = true
	return &nc
}

func (c *ocCLI) run(verb string, args ...string) (string, error) {
	var cmdArgs []string
	if c.kubeconfig != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--kubeconfig=%s", c.kubeconfig))
	}
	if !c.withoutNamespace && c.namespace != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--namespace=%s", c.namespace))
	}
	cmdArgs = append(cmdArgs, verb)
	cmdArgs = append(cmdArgs, args...)

	logf("Running: %s %s", c.execPath, strings.Join(cmdArgs, " "))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.execPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		return outStr, fmt.Errorf("command failed: %w\nstdout: %s\nstderr: %s", err, outStr, errStr)
	}
	return outStr, nil
}

func (c *ocCLI) runAsAdmin(verb string, args ...string) (string, error) {
	return c.asAdminCLI().withoutNS().run(verb, args...)
}

func getClientset() (*kubernetes.Clientset, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

type pingPodResourceNode struct {
	name      string
	namespace string
	nodename  string
	template  string
}

type egressrouterMultipleDst struct {
	name           string
	namespace      string
	reservedip     string
	gateway        string
	destinationip1 string
	destinationip2 string
	destinationip3 string
	template       string
}

type singleRuleBANPPolicyResource struct {
	name       string
	subjectKey string
	subjectVal string
	policyType string
	direction  string
	ruleName   string
	ruleAction string
	ruleKey    string
	ruleVal    string
	template   string
}

type singlePodRuleANPPolicyResource struct {
	name          string
	subjectKey    string
	subjectVal    string
	subjectPodKey string
	subjectPodVal string
	priority      int32
	policyType    string
	direction     string
	ruleName      string
	ruleAction    string
	ruleKey       string
	ruleVal       string
	rulePodKey    string
	rulePodVal    string
	template      string
}

func (pod *pingPodResourceNode) createPingPodNode(oc *ocCLI) {
	err := wait.Poll(3*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", pod.template, "-p", "NAME="+pod.name, "NAMESPACE="+pod.namespace, "NODENAME="+pod.nodename)
		if err1 != nil {
			logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	assertWaitPollNoErr(err, fmt.Sprintf("fail to create pod %v", pod.name))
}

func (egressrouter *egressrouterMultipleDst) createEgressRouterMultipeDst(oc *ocCLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", egressrouter.template, "-p", "NAME="+egressrouter.name, "NAMESPACE="+egressrouter.namespace, "RESERVEDIP="+egressrouter.reservedip, "GATEWAY="+egressrouter.gateway, "DSTIP1="+egressrouter.destinationip1, "DSTIP2="+egressrouter.destinationip2, "DSTIP3="+egressrouter.destinationip3)
		if err1 != nil {
			logf("the err:%v, and try next round", err1)
			return false, nil
		}
		return true, nil
	})
	assertWaitPollNoErr(err, fmt.Sprintf("fail to create egressrouter %v", egressrouter.name))
}

func (banp *singleRuleBANPPolicyResource) createSingleRuleBANP(oc *ocCLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", banp.template, "-p", "NAME="+banp.name,
			"SUBJECTKEY="+banp.subjectKey, "SUBJECTVAL="+banp.subjectVal,
			"POLICYTYPE="+banp.policyType, "DIRECTION="+banp.direction,
			"RULENAME="+banp.ruleName, "RULEACTION="+banp.ruleAction, "RULEKEY="+banp.ruleKey, "RULEVAL="+banp.ruleVal)
		if err1 != nil {
			logf("Error creating resource:%v, and trying again", err1)
			return false, nil
		}
		return true, nil
	})
	assertWaitPollNoErr(err, fmt.Sprintf("Failed to create Baseline Admin Network Policy CR %v", banp.name))
}

func (anp *singlePodRuleANPPolicyResource) createSinglePodRuleANP(oc *ocCLI) {
	err := wait.Poll(5*time.Second, 20*time.Second, func() (bool, error) {
		err1 := applyResourceFromTemplateByAdmin(oc, "--ignore-unknown-parameters=true", "-f", anp.template, "-p", "NAME="+anp.name, "PRIORITY="+strconv.Itoa(int(anp.priority)),
			"SUBJECTKEY="+anp.subjectKey, "SUBJECTVAL="+anp.subjectVal, "SUBJECTPODKEY="+anp.subjectPodKey, "SUBJECTPODVAL="+anp.subjectPodVal,
			"POLICYTYPE="+anp.policyType, "DIRECTION="+anp.direction, "RULENAME="+anp.ruleName, "RULEACTION="+anp.ruleAction,
			"RULEKEY="+anp.ruleKey, "RULEVAL="+anp.ruleVal, "RULEPODKEY="+anp.rulePodKey, "RULEPODVAL="+anp.rulePodVal)
		if err1 != nil {
			logf("Error creating resource:%v, and trying again", err1)
			return false, nil
		}
		return true, nil
	})
	assertWaitPollNoErr(err, fmt.Sprintf("Failed to create Admin Network Policy CR %v", anp.name))
}

func applyResourceFromTemplateByAdmin(oc *ocCLI, parameters ...string) error {
	var configFile string
	err := wait.Poll(3*time.Second, 60*time.Second, func() (bool, error) {
		tmpFile, err := os.CreateTemp("", getRandomString()+"resource-*.json")
		if err != nil {
			return false, nil
		}
		if err := tmpFile.Close(); err != nil {
			logf("failed to close temp file: %v", err)
		}
		processArgs := append(parameters, "-o", "json")
		output, err := oc.runAsAdmin("process", processArgs...)
		if err != nil {
			logf("the err:%v, and try next round", err)
			if rmErr := os.Remove(tmpFile.Name()); rmErr != nil {
				logf("failed to remove temp file: %v", rmErr)
			}
			return false, nil
		}
		if err := os.WriteFile(tmpFile.Name(), []byte(output), 0644); err != nil {
			if rmErr := os.Remove(tmpFile.Name()); rmErr != nil {
				logf("failed to remove temp file: %v", rmErr)
			}
			return false, nil
		}
		configFile = tmpFile.Name()
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("as admin fail to process %v", parameters))

	logf("the file of resource is %s", configFile)
	_, err = oc.runAsAdmin("apply", "-f", configFile)
	if rmErr := os.Remove(configFile); rmErr != nil {
		logf("failed to remove temp resource file %s: %v", configFile, rmErr)
	}
	return err
}

func removeResource(oc *ocCLI, asAdmin bool, withoutNamespace bool, parameters ...string) {
	var err error
	if asAdmin && withoutNamespace {
		_, err = oc.runAsAdmin("delete", parameters...)
	} else {
		_, err = oc.run("delete", parameters...)
	}
	if err != nil && (strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "No resources found")) {
		logf("the resource is deleted already")
		return
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.Poll(3*time.Second, 120*time.Second, func() (bool, error) {
		var e error
		if asAdmin && withoutNamespace {
			_, e = oc.runAsAdmin("get", parameters...)
		} else {
			_, e = oc.run("get", parameters...)
		}
		if e != nil && (strings.Contains(e.Error(), "NotFound") || strings.Contains(e.Error(), "No resources found")) {
			logf("the resource is deleted successfully")
			return true, nil
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("fail to delete resource %v", parameters))
}

func getRandomString() string {
	chars := "abcdefghijklmnopqrstuvwxyz0123456789"
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	buffer := make([]byte, 8)
	for index := range buffer {
		buffer[index] = chars[seed.Intn(len(chars))]
	}
	return string(buffer)
}

func waitPodReady(oc *ocCLI, namespace string, podName string) {
	err := wait.Poll(5*time.Second, 60*time.Second, func() (bool, error) {
		status, err := oc.runAsAdmin("get", "pod", "-n", namespace, podName, "-o=jsonpath={.status.phase}")
		if err != nil {
			logf("the err:%v, wait for pod %v to become ready.", err, podName)
			return false, nil
		}
		ready := []string{"Running", "Ready", "Complete", "Succeeded"}
		for _, s := range ready {
			if s == status {
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		output, descErr := oc.runAsAdmin("describe", "pod", "-n", namespace, podName)
		if descErr != nil {
			logf("failed to describe pod %v: %v", podName, descErr)
		}
		logf("oc describe pod %v:\n%s", podName, output)
	}
	assertWaitPollNoErr(err, fmt.Sprintf("pod %v is not ready", podName))
}

func waitForPodWithLabelReady(oc *ocCLI, ns, label string) error {
	return wait.Poll(5*time.Second, 5*time.Minute, func() (bool, error) {
		status, err := oc.runAsAdmin("get", "pod", "-n", ns, "-l", label, "-ojsonpath={.items[*].status.conditions[?(@.type==\"Ready\")].status}")
		logf("the Ready status of pod is %v", status)
		if err != nil || status == "" {
			logf("failed to get pod status: %v, retrying...", err)
			return false, nil
		}
		if strings.Contains(status, "False") {
			logf("the pod Ready status not met; wanted True but got %v, retrying...", status)
			return false, nil
		}
		return true, nil
	})
}

func createResourceFromFile(oc *ocCLI, ns, file string) {
	_, err := oc.runAsAdmin("create", "-f", file, "-n", ns)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func assertWaitPollNoErr(err error, msg string) {
	o.Expect(err).NotTo(o.HaveOccurred(), msg)
}

func checkPlatform(oc *ocCLI) string {
	output, err := oc.runAsAdmin("get", "infrastructure", "cluster", "-o=jsonpath={.status.platformStatus.type}")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get platform type")
	return strings.ToLower(output)
}

func checkNetworkType(oc *ocCLI) string {
	output, err := oc.runAsAdmin("get", "network.operator", "cluster", "-o=jsonpath={.spec.defaultNetwork.type}")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get network type")
	return strings.ToLower(output)
}

func checkProxy(oc *ocCLI) bool {
	httpProxy, err := oc.runAsAdmin("get", "proxy", "cluster", "-o=jsonpath={.status.httpProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	httpsProxy, err := oc.runAsAdmin("get", "proxy", "cluster", "-o=jsonpath={.status.httpsProxy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	return httpProxy != "" || httpsProxy != ""
}

func checkIPStackType(oc *ocCLI) string {
	svcNetwork, err := oc.runAsAdmin("get", "network.operator", "cluster", "-o=jsonpath={.spec.serviceNetwork}")
	o.Expect(err).NotTo(o.HaveOccurred())
	if strings.Count(svcNetwork, ":") >= 2 && strings.Count(svcNetwork, ".") >= 2 {
		return "dualstack"
	} else if strings.Count(svcNetwork, ":") >= 2 {
		return "ipv6single"
	} else if strings.Count(svcNetwork, ".") >= 2 {
		return "ipv4single"
	}
	return ""
}

func nslookDomainName(domainName string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, domainName)
	o.Expect(err).NotTo(o.HaveOccurred())
	for _, ip := range ips {
		if ip.IP.To4() != nil {
			return ip.IP.String()
		}
	}
	logf("There is no IPv4 address for destination domain %s", domainName)
	return ""
}

func getPrimaryIfaddrFromBMNode(oc *ocCLI, nodeName string) (string, string) {
	primaryIfaddr, err := oc.runAsAdmin("get", "node", nodeName, "-o=jsonpath={.metadata.annotations.k8s\\.ovn\\.org/node-primary-ifaddr}")
	o.Expect(err).NotTo(o.HaveOccurred())
	logf("The primaryIfaddr is %v for node %s", primaryIfaddr, nodeName)
	var ipv4Ifaddr, ipv6Ifaddr string
	tempSlice := strings.Split(primaryIfaddr, "\"")
	ipStackType := checkIPStackType(oc)
	switch ipStackType {
	case "ipv4single":
		o.Expect(len(tempSlice) > 3).Should(o.BeTrue())
		ipv4Ifaddr = tempSlice[3]
	case "dualstack":
		o.Expect(len(tempSlice) > 7).Should(o.BeTrue())
		ipv4Ifaddr = tempSlice[3]
		ipv6Ifaddr = tempSlice[7]
	case "ipv6single":
		o.Expect(len(tempSlice) > 3).Should(o.BeTrue())
		ipv6Ifaddr = tempSlice[3]
	default:
		g.Skip("Skip for not supported IP stack type!! ")
	}
	return ipv4Ifaddr, ipv6Ifaddr
}

func findFreeIPs(oc *ocCLI, nodeName string, number int) []string {
	var freeIPs []string
	platform := checkPlatform(oc)
	if strings.Contains(platform, "baremetal") || strings.Contains(platform, "none") || strings.Contains(platform, "nutanix") || strings.Contains(platform, "kubevirt") || strings.Contains(platform, "powervs") {
		ipv4Sub, _ := getPrimaryIfaddrFromBMNode(oc, nodeName)
		tempSlice := strings.Split(ipv4Sub, "/")
		o.Expect(len(tempSlice) > 1).Should(o.BeTrue())
		preFix, err := strconv.Atoi(tempSlice[1])
		o.Expect(err).NotTo(o.HaveOccurred())
		if preFix > 29 {
			g.Skip("There might be no enough free IPs in current subnet, skip the test!!")
		}
		freeIPs = findUnUsedIPsOnNode(oc, nodeName, ipv4Sub, number)
	} else if strings.Contains(platform, "vsphere") {
		sub1, err := getDefaultSubnet(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		freeIPs = findUnUsedIPs(oc, sub1, number)
	} else {
		sub1 := getIfaddrFromNode(nodeName, oc)
		if len(sub1) == 0 && strings.Contains(platform, "gcp") {
			g.Skip("Skip the tests as no egressIP annotation on this platform nodes!!")
		}
		o.Expect(len(sub1) == 0).NotTo(o.BeTrue())
		freeIPs = findUnUsedIPsOnNode(oc, nodeName, sub1, number)
	}
	return freeIPs
}

func findUnUsedIPsOnNode(oc *ocCLI, nodeName, cidr string, expectedNum int) []string {
	ipRange, err := hosts(cidr)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to parse CIDR %s", cidr))
	var ipUnused []string
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(ipRange), func(i, j int) { ipRange[i], ipRange[j] = ipRange[j], ipRange[i] })
	for _, ip := range ipRange {
		if len(ipUnused) < expectedNum {
			pingCmd := fmt.Sprintf("ping -c1 -W1 %s", ip)
			_, err := debugNodeWithChroot(oc, nodeName, "bash", "-c", pingCmd)
			if err != nil {
				logf("%s is not used on node %s", ip, nodeName)
				ipUnused = append(ipUnused, ip)
			}
		} else {
			break
		}
	}
	return ipUnused
}

func findUnUsedIPs(oc *ocCLI, cidr string, number int) []string {
	ipRange, err := hosts(cidr)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to parse CIDR %s", cidr))
	var ipUnused []string
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(ipRange), func(i, j int) { ipRange[i], ipRange[j] = ipRange[j], ipRange[i] })
	for _, ip := range ipRange {
		if len(ipUnused) < number {
			pingCmd := "ping -c4 -t1 " + ip
			_, err := execCommandInNetworkingPod(oc, pingCmd)
			if err != nil {
				logf("%s is not used!", ip)
				ipUnused = append(ipUnused, ip)
			}
		} else {
			break
		}
	}
	return ipUnused
}

func hosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	logf("in hosts function, ip: %v, ipnet: %v", ip, ipnet)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	if len(ips) < 2 {
		return nil, fmt.Errorf("CIDR %s has fewer than 2 addresses, cannot extract host range", cidr)
	}
	return ips[1 : len(ips)-1], nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func getIPv4Gateway(oc *ocCLI, nodeName string) string {
	cmd := "ip -4 route | grep default | awk '{print $3}'"
	output, err := debugNode(oc, nodeName, "bash", "-c", cmd)
	o.Expect(err).NotTo(o.HaveOccurred())
	re := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
	ips := re.FindAllString(output, -1)
	if len(ips) == 0 {
		return ""
	}
	logf("The default gateway of node %s is %s", nodeName, ips[0])
	return ips[0]
}

func getInterfacePrefix(oc *ocCLI, nodeName string) string {
	defInf, err := getDefaultInterface(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	cmd := fmt.Sprintf("ip -4 -brief a show %s | awk '{print $3}' ", defInf)
	output, err := debugNode(oc, nodeName, "bash", "-c", cmd)
	o.Expect(err).NotTo(o.HaveOccurred())
	logf("IP address for default interface %s is %s", defInf, output)
	sli := strings.Split(output, "/")
	if len(sli) > 1 {
		return strings.Split(sli[1], "\n")[0]
	}
	return "24"
}

func getDefaultInterface(oc *ocCLI) (string, error) {
	getDefaultInterfaceCmd := "/usr/sbin/ip -4 route show default"
	int1, err := execCommandInNetworkingPod(oc, getDefaultInterfaceCmd)
	if err != nil {
		logf("Cannot get default interface, errors: %v", err)
		return "", err
	}
	fields := strings.Fields(int1)
	devIdx := -1
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			devIdx = i + 1
			break
		}
	}
	if devIdx < 0 {
		return "", fmt.Errorf("could not find 'dev' token in route output: %s", int1)
	}
	defInterface := fields[devIdx]
	logf("Get the default interface: %s", defInterface)
	return defInterface, nil
}

func getDefaultSubnet(oc *ocCLI) (string, error) {
	int1, err := getDefaultInterface(oc)
	if err != nil {
		return "", fmt.Errorf("cannot get default interface: %v", err)
	}
	getDefaultSubnetCmd := "/usr/sbin/ip -4 -brief a show " + int1
	subnet1, err := execCommandInNetworkingPod(oc, getDefaultSubnetCmd)
	if err != nil {
		logf("Cannot get default subnet, errors: %v", err)
		return "", err
	}
	fields := strings.Fields(subnet1)
	if len(fields) < 3 {
		return "", fmt.Errorf("unexpected subnet output format: %s", subnet1)
	}
	defSubnet := fields[2]
	logf("Get the default subnet: %s", defSubnet)
	return defSubnet, nil
}

func getIfaddrFromNode(nodeName string, oc *ocCLI) string {
	egressIpconfig, err := oc.runAsAdmin("get", "node", nodeName, "-o=jsonpath={.metadata.annotations.cloud\\.network\\.openshift\\.io/egress-ipconfig}")
	o.Expect(err).NotTo(o.HaveOccurred())
	if len(egressIpconfig) == 0 {
		logf("The node %s doesn't have egressIP annotation", nodeName)
		return ""
	}
	var configs []struct {
		Ifaddr struct {
			IPv4 string `json:"ipv4"`
			IPv6 string `json:"ipv6"`
		} `json:"ifaddr"`
	}
	err = json.Unmarshal([]byte(egressIpconfig), &configs)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to parse egress-ipconfig annotation: %s", egressIpconfig))
	o.Expect(len(configs) > 0).Should(o.BeTrue(), "egress-ipconfig annotation is empty array")
	ifaddr := configs[0].Ifaddr.IPv4
	if ifaddr == "" {
		ifaddr = configs[0].Ifaddr.IPv6
	}
	logf("The subnet of node %s is %v .", nodeName, ifaddr)
	return ifaddr
}

func execCommandInNetworkingPod(oc *ocCLI, command string) (string, error) {
	podName, err := oc.runAsAdmin("get", "pods", "-n", "openshift-ovn-kubernetes", "-l", "app=ovnkube-node", "-o=jsonpath={.items[0].metadata.name}")
	if err != nil {
		logf("Cannot get ovn-kubernetes pods, errors: %v", err)
		return "", err
	}
	msg, err := oc.runAsAdmin("exec", "-n", "openshift-ovn-kubernetes", "-c", "ovnkube-controller", podName, "--", "/bin/sh", "-c", command)
	if err != nil {
		logf("Execute command failed with  err:%v .", err)
		return "", err
	}
	return msg, nil
}

func getReadySchedulableNodes(ctx context.Context) (*corev1.NodeList, error) {
	clientset, err := getClientset()
	if err != nil {
		return nil, err
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var readyNodes []corev1.Node
	for _, node := range nodes.Items {
		if node.Spec.Unschedulable {
			continue
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				readyNodes = append(readyNodes, node)
				break
			}
		}
	}
	nodes.Items = readyNodes
	return nodes, nil
}

func excludeSriovNodes(nodeList *corev1.NodeList) []string {
	var workers []string
	for _, node := range nodeList.Items {
		_, ok := node.Labels["node-role.kubernetes.io/sriov"]
		if !ok {
			logf("node %s is not sriov node,add it to worker list.", node.Name)
			workers = append(workers, node.Name)
		}
	}
	return workers
}

func debugNode(oc *ocCLI, nodeName string, cmd ...string) (string, error) {
	args := []string{"-n", "default", "node/" + nodeName, "--"}
	args = append(args, cmd...)
	return oc.runAsAdmin("debug", args...)
}

func debugNodeWithChroot(oc *ocCLI, nodeName string, cmd ...string) (string, error) {
	args := []string{"-n", "default", "node/" + nodeName, "--"}
	chrootCmd := append([]string{"chroot", "/host"}, cmd...)
	args = append(args, chrootCmd...)
	return oc.runAsAdmin("debug", args...)
}

func getSvcIP(oc *ocCLI, namespace string, svcName string) (string, string) {
	ipStack := checkIPStackType(oc)
	svctype, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.type}")
	o.Expect(err).NotTo(o.HaveOccurred())
	ipFamilyType, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.ipFamilyPolicy}")
	o.Expect(err).NotTo(o.HaveOccurred())
	if (svctype == "ClusterIP") || (svctype == "NodePort") {
		if (ipStack == "ipv6single") || (ipStack == "ipv4single") {
			svcIP, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}")
			o.Expect(err).NotTo(o.HaveOccurred())
			logf("The service %s IP in namespace %s is %q", svcName, namespace, svcIP)
			return svcIP, ""
		} else if (ipStack == "dualstack" && ipFamilyType == "PreferDualStack") || (ipStack == "dualstack" && ipFamilyType == "RequireDualStack") {
			svcIPv4, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}")
			o.Expect(err).NotTo(o.HaveOccurred())
			svcIPv6, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[1]}")
			o.Expect(err).NotTo(o.HaveOccurred())
			ipFamilyPrecedence, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.ipFamilies[0]}")
			o.Expect(err).NotTo(o.HaveOccurred())
			if ipFamilyPrecedence == "IPv4" {
				return svcIPv6, svcIPv4
			}
			svcIPv4, svcIPv6 = svcIPv6, svcIPv4
			return svcIPv6, svcIPv4
		}
		svcIP, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}")
		o.Expect(err).NotTo(o.HaveOccurred())
		return svcIP, ""
	}
	svcIP, err := oc.runAsAdmin("get", "service", "-n", namespace, svcName, "-o=jsonpath={.spec.clusterIPs[0]}")
	o.Expect(err).NotTo(o.HaveOccurred())
	return svcIP, ""
}

func patchReplaceResourceAsAdmin(oc *ocCLI, resource, patch string, nameSpace ...string) {
	var args []string
	if len(nameSpace) > 0 {
		args = []string{resource, "-p", patch, "-n", nameSpace[0], "--type=json"}
	} else {
		args = []string{resource, "-p", patch, "--type=json"}
	}
	_, err := oc.runAsAdmin("patch", args...)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func runHostCmdWithRetries(oc *ocCLI, namespace, podName, command string, interval, timeout time.Duration) (string, error) {
	var output string
	var lastErr error
	err := wait.Poll(interval, timeout, func() (bool, error) {
		out, err := oc.runAsAdmin("exec", "-n", namespace, podName, "--", "bash", "-c", command)
		if err != nil {
			lastErr = err
			return false, nil
		}
		output = out
		return true, nil
	})
	if err != nil {
		return output, fmt.Errorf("command failed after retries: %v (last error: %v)", err, lastErr)
	}
	return output, nil
}

func setupTestNamespace(oc *ocCLI) string {
	nsName := fmt.Sprintf("e2e-egressrouter-%s", getRandomString())
	_, err := oc.runAsAdmin("create", "namespace", nsName)
	o.Expect(err).NotTo(o.HaveOccurred())
	_, err = oc.runAsAdmin("label", "namespace", nsName,
		"pod-security.kubernetes.io/enforce=privileged",
		"pod-security.kubernetes.io/warn=privileged",
		"pod-security.kubernetes.io/audit=privileged",
		"security.openshift.io/scc.podSecurityLabelSync=false",
		"--overwrite")
	o.Expect(err).NotTo(o.HaveOccurred())
	logf("Created test namespace: %s", nsName)
	return nsName
}

func deleteNamespace(oc *ocCLI, ns string) {
	_, err := oc.runAsAdmin("delete", "namespace", ns, "--wait=false")
	if err != nil {
		logf("Warning: failed to delete namespace %s: %v", ns, err)
	}
}
