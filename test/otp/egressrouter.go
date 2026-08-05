package otp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/egress-router-cni/test/otp/testdata"
)

var _ = g.Describe("[sig-networking][Suite:openshift/egress-router-cni] SDN EgressRouter", func() {
	var oc *ocCLI
	var ns string

	g.BeforeEach(func() {
		oc = newOC("")
		platform := checkPlatform(oc)
		logf("The platform is %v", platform)
		networkType := checkNetworkType(oc)
		acceptedPlatform := strings.Contains(platform, "baremetal")
		if !acceptedPlatform || !strings.Contains(networkType, "ovn") {
			g.Skip("Test cases should be run on BareMetal cluster, skip for other platforms or other non-OVN network plugin!!")
		}
		if checkProxy(oc) {
			g.Skip("This is proxy cluster, skip the test.")
		}
		ns = setupTestNamespace(oc)
		oc = newOC(ns)
	})

	g.AfterEach(func() {
		if ns != "" {
			deleteNamespace(newOC(""), ns)
		}
	})

	g.It("[JIRA:Networking][OTP][Serial] 42340-Egress router redirect mode with multiple destinations [Suite:openshift/egress-router-cni]", func() {
		ipStackType := checkIPStackType(oc)
		g.By("Skip testing on ipv6 single stack cluster")
		if ipStackType == "ipv6single" {
			g.Skip("Skip for single stack cluster!!!")
		}
		var (
			testDataDir                  = testdata.FixturePath()
			egressBaseDir                = filepath.Join(testDataDir, "egressrouter")
			pingPodTemplate              = filepath.Join(testDataDir, "ping-for-pod-template.yaml")
			egressRouterTemplate         = filepath.Join(egressBaseDir, "egressrouter-multiple-destination-template.yaml")
			egressRouterService          = filepath.Join(egressBaseDir, "serive-egressrouter.yaml")
			egressRouterServiceDualStack = filepath.Join(egressBaseDir, "serive-egressrouter-dualstack.yaml")
			url                          = "www.google.com"
		)

		g.By("1. nslookup obtain dns server ip for url")
		destinationIP := nslookDomainName(url)
		logf("ip address from nslookup for %v: %v", url, destinationIP)

		g.By("2. Get gateway for one worker node")
		nodeList, err := getReadySchedulableNodes(context.TODO())
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(nodeList.Items) > 0).Should(o.BeTrue())

		gateway := getIPv4Gateway(oc, nodeList.Items[0].Name)
		o.Expect(gateway).ShouldNot(o.BeEmpty())
		freeIP := findFreeIPs(oc, nodeList.Items[0].Name, 1)
		o.Expect(len(freeIP)).Should(o.Equal(1))
		prefixIP := getInterfacePrefix(oc, nodeList.Items[0].Name)
		o.Expect(prefixIP).ShouldNot(o.BeEmpty())
		reservedIP := fmt.Sprintf("%s/%s", freeIP[0], prefixIP)

		g.By("3. Create egressrouter")
		egressrouter := egressrouterMultipleDst{
			name:           "egressrouter-42340",
			namespace:      ns,
			reservedip:     reservedIP,
			gateway:        gateway,
			destinationip1: destinationIP,
			destinationip2: destinationIP,
			destinationip3: destinationIP,
			template:       egressRouterTemplate,
		}
		egressrouter.createEgressRouterMultipeDst(oc)
		err = waitForPodWithLabelReady(oc, ns, "app=egress-router-cni")
		assertWaitPollNoErr(err, "EgressRouter pod is not ready!")

		g.By("4. Schedule the worker")
		workers := excludeSriovNodes(nodeList)
		o.Expect(len(workers) > 0).Should(o.BeTrue(), fmt.Sprintf("The number of common worker nodes in the cluster is %v ", len(workers)))
		if len(workers) < len(nodeList.Items) {
			logf("There are sriov workers in the cluster, will schedule the egress router pod to a common node.")
			_, err = oc.runAsAdmin("patch", "-n", ns, "deployment/egress-router-cni-deployment", "-p", "{\"spec\":{\"template\":{\"spec\":{\"nodeName\":\""+workers[0]+"\"}}}}", "--type=merge")
			o.Expect(err).NotTo(o.HaveOccurred())
			output, err := oc.runAsAdmin("rollout", "-n", ns, "status", "deployment/egress-router-cni-deployment")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("successfully rolled out"))
		}

		g.By("5. Create service for egress router pod!")
		if ipStackType == "dualstack" {
			createResourceFromFile(oc, ns, egressRouterServiceDualStack)
		} else {
			createResourceFromFile(oc, ns, egressRouterService)
		}

		g.By("6. create hello pod in ns1")
		pod1 := pingPodResourceNode{
			name:      "hello-pod1",
			namespace: ns,
			template:  pingPodTemplate,
		}
		pod1.createPingPodNode(oc)
		waitPodReady(oc, ns, pod1.name)

		g.By("7. Get service IP")
		var svcIPv4 string
		if ipStackType == "dualstack" {
			_, svcIPv4 = getSvcIP(oc, ns, "ovn-egressrouter-multidst-svc")
		} else {
			svcIPv4, _ = getSvcIP(oc, ns, "ovn-egressrouter-multidst-svc")
		}

		g.By("8. Check result,the svc for egessrouter can be accessed")
		_, err = runHostCmdWithRetries(oc, ns, pod1.name, "curl -s "+svcIPv4+":5000 --connect-timeout 10", 5*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to access %s:5000 with error:%v", svcIPv4, err))
		_, err = runHostCmdWithRetries(oc, ns, pod1.name, "curl -s "+svcIPv4+":6000 --connect-timeout 10", 5*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to access %s:6000 with error:%v", svcIPv4, err))
		_, err = runHostCmdWithRetries(oc, ns, pod1.name, "curl -s "+svcIPv4+":80 --connect-timeout 10", 5*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to access %s:80 with error:%v", svcIPv4, err))
	})

	g.It("[JIRA:Networking][OTP][Serial] 81385-BANP and ANP with Egress router in redirect mode with multiple destinations [Suite:openshift/egress-router-cni]", func() {
		ipStackType := checkIPStackType(oc)
		g.By("Skip testing on ipv6 single stack cluster")
		if ipStackType == "ipv6single" {
			g.Skip("Skip for single stack cluster.")
		}
		var (
			testDataDir                  = testdata.FixturePath()
			egressBaseDir                = filepath.Join(testDataDir, "egressrouter")
			pingPodTemplate              = filepath.Join(testDataDir, "ping-for-pod-template.yaml")
			egressRouterTemplate         = filepath.Join(egressBaseDir, "egressrouter-multiple-destination-template.yaml")
			egressRouterService          = filepath.Join(egressBaseDir, "serive-egressrouter.yaml")
			egressRouterServiceDualStack = filepath.Join(egressBaseDir, "serive-egressrouter-dualstack.yaml")
			banpCRTemplate               = filepath.Join(testDataDir, "adminnetworkpolicy", "banp-single-rule-template.yaml")
			anpCRTemplate                = filepath.Join(testDataDir, "adminnetworkpolicy", "anp-single-pod-rule-template.yaml")
			matchLabelKey                = "kubernetes.io/metadata.name"
			podKey                       = "app"
			podVal                       = "egress-router-cni"
			testID                       = "81385"
			url                          = "www.google.com"
		)

		g.By("1. Nslookup obtain DNS server ip for URL.")
		destinationIP := nslookDomainName(url)
		logf("IP address from nslookup for %v: %v", url, destinationIP)

		g.By("2. Get gateway for one worker node.")
		nodeList, err := getReadySchedulableNodes(context.TODO())
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(nodeList.Items) > 0).Should(o.BeTrue())

		gateway := getIPv4Gateway(oc, nodeList.Items[0].Name)
		o.Expect(gateway).ShouldNot(o.BeEmpty())
		freeIP := findFreeIPs(oc, nodeList.Items[0].Name, 1)
		o.Expect(len(freeIP)).Should(o.Equal(1))
		prefixIP := getInterfacePrefix(oc, nodeList.Items[0].Name)
		o.Expect(prefixIP).ShouldNot(o.BeEmpty())
		reservedIP := fmt.Sprintf("%s/%s", freeIP[0], prefixIP)

		g.By("3. Create egressrouter")
		egressrouter := egressrouterMultipleDst{
			name:           "egressrouter-" + testID,
			namespace:      ns,
			reservedip:     reservedIP,
			gateway:        gateway,
			destinationip1: destinationIP,
			destinationip2: destinationIP,
			destinationip3: destinationIP,
			template:       egressRouterTemplate,
		}
		egressrouter.createEgressRouterMultipeDst(oc)
		err = waitForPodWithLabelReady(oc, ns, "app=egress-router-cni")
		assertWaitPollNoErr(err, "EgressRouter pod is not ready!")

		g.By("4. Schedule the router pod")
		workers := excludeSriovNodes(nodeList)
		o.Expect(len(workers) > 0).Should(o.BeTrue(), fmt.Sprintf("The number of common worker nodes in the cluster is %v ", len(workers)))
		if len(workers) < len(nodeList.Items) {
			logf("There are sriov workers in the cluster, will schedule the egress router pod to a common node.")
			_, err = oc.runAsAdmin("patch", "-n", ns, "deployment/egress-router-cni-deployment", "-p", "{\"spec\":{\"template\":{\"spec\":{\"nodeName\":\""+workers[0]+"\"}}}}", "--type=merge")
			o.Expect(err).NotTo(o.HaveOccurred())
			output, err := oc.runAsAdmin("rollout", "-n", ns, "status", "deployment/egress-router-cni-deployment")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(output).To(o.ContainSubstring("successfully rolled out"))
		}

		g.By("5. Create service for egress router pod.")
		if ipStackType == "dualstack" {
			createResourceFromFile(oc, ns, egressRouterServiceDualStack)
		} else {
			createResourceFromFile(oc, ns, egressRouterService)
		}

		g.By("6. Create hello pod in the namespace")
		pod := pingPodResourceNode{
			name:      "hello-pod",
			namespace: ns,
			template:  pingPodTemplate,
		}
		pod.createPingPodNode(oc)
		waitPodReady(oc, ns, pod.name)

		g.By("7. Get service IP")
		var svcIPv4 string
		if ipStackType == "dualstack" {
			_, svcIPv4 = getSvcIP(oc, ns, "ovn-egressrouter-multidst-svc")
		} else {
			svcIPv4, _ = getSvcIP(oc, ns, "ovn-egressrouter-multidst-svc")
		}

		g.By("8. Check the service for egessrouter can be accessed at all the ports")
		portList := []string{"5000", "6000", "80"}
		for _, port := range portList {
			_, err = runHostCmdWithRetries(oc, ns, pod.name, "curl -s "+svcIPv4+":"+port+" --connect-timeout 10", 5*time.Second, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to access %s:%s with error:%v", svcIPv4, port, err))
		}

		g.By("9. Create BANP to deny egress, check the svc for egessrouter cannot be accessed")
		banpCR := singleRuleBANPPolicyResource{
			name:       "default",
			subjectKey: matchLabelKey,
			subjectVal: ns,
			policyType: "egress",
			direction:  "to",
			ruleName:   "default-deny-to-" + ns,
			ruleAction: "Deny",
			ruleKey:    matchLabelKey,
			ruleVal:    ns,
			template:   banpCRTemplate,
		}
		defer removeResource(oc, true, true, "banp", banpCR.name)
		banpCR.createSingleRuleBANP(oc)
		output, err := oc.runAsAdmin("get", "banp")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(output, banpCR.name)).To(o.BeTrue())

		for _, port := range portList {
			_, err = runHostCmdWithRetries(oc, ns, pod.name, "curl -s "+svcIPv4+":"+port+" --connect-timeout 10", 5*time.Second, 15*time.Second)
			o.Expect(err).To(o.HaveOccurred(), fmt.Sprintf("Did not Fail to access %s:%v as expected, error:%v", svcIPv4, port, err))
		}

		g.By("10. Create ANP to allow egress traffic from namespace")
		anpSingleRuleCR := singlePodRuleANPPolicyResource{
			name:          "anp-" + testID,
			subjectKey:    matchLabelKey,
			subjectVal:    ns,
			subjectPodKey: podKey,
			subjectPodVal: podVal,
			priority:      20,
			policyType:    "egress",
			direction:     "to",
			ruleName:      "allow-to-" + ns,
			ruleAction:    "Allow",
			ruleKey:       matchLabelKey,
			ruleVal:       ns,
			rulePodKey:    podKey,
			rulePodVal:    podVal,
			template:      anpCRTemplate,
		}
		defer removeResource(oc, true, true, "anp", anpSingleRuleCR.name)
		anpSingleRuleCR.createSinglePodRuleANP(oc)
		output, err = oc.runAsAdmin("get", "anp")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(output, anpSingleRuleCR.name)).To(o.BeTrue())

		g.By("11. Update ANP to add port to rule and podSelector{} to subject, check the svc for egessrouter can be accessed through allowed ports")
		patchANP := `[{"op": "add", "path": "/spec/egress/0/ports", "value": [{"portNumber": {"protocol": "TCP", "port": 5000}}, {"portNumber": {"protocol": "TCP", "port": 8080}}]}, {"op": "add", "path": "/spec/subject/pods/podSelector", "value": {}}]`
		patchReplaceResourceAsAdmin(oc, "anp/"+anpSingleRuleCR.name, patchANP)
		anpRules, patchErr := oc.runAsAdmin("get", "adminnetworkpolicy", anpSingleRuleCR.name, "-o=jsonpath={.spec}")
		o.Expect(patchErr).NotTo(o.HaveOccurred())
		logf("ANP Rules %s after update", anpRules)

		_, err = runHostCmdWithRetries(oc, ns, pod.name, "curl -s "+svcIPv4+":5000 --connect-timeout 10", 5*time.Second, 30*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Failed to access %s:5000 with error:%v", svcIPv4, err))
		_, err = runHostCmdWithRetries(oc, ns, pod.name, "curl -s "+svcIPv4+":6000 --connect-timeout 10", 5*time.Second, 15*time.Second)
		o.Expect(err).To(o.HaveOccurred(), fmt.Sprintf("Did not fail to access %s:6000 as expected, error:%v", svcIPv4, err))
	})
})
