package macvlan

import (
	"fmt"
	"testing"

	egresstest "github.com/openshift/egress-router-cni/pkg/testing"
	netlink_mocks "github.com/openshift/egress-router-cni/pkg/testing/mocks/github.com/vishvananda/netlink"
	util "github.com/openshift/egress-router-cni/pkg/util"
	util_mocks "github.com/openshift/egress-router-cni/pkg/util/mocks"
	"github.com/vishvananda/netlink"

	"github.com/stretchr/testify/assert"
)

func TestFillNetConfDefaults(t *testing.T) {
	mockNetLinkOps := new(util_mocks.NetLinkOps)
	mockLink := new(netlink_mocks.Link)
	// below sets the `netLinkOps` in util/net_linux.go to a mock instance for purpose of unit tests execution
	util.SetNetLinkOpMockInst(mockNetLinkOps)

	tests := []struct {
		desc             string
		inpNetConf       *NetConf
		inpClusterConf   *ClusterConf
		outNetConf       *NetConf
		errMatch         error
		netOpsMockHelper []egresstest.TestifyMockHelper
		linkMockHelper   []egresstest.TestifyMockHelper
	}{
		{
			desc:           "empty NetConf and ClusterConf struct",
			inpNetConf:     &NetConf{},
			inpClusterConf: &ClusterConf{},
			outNetConf: &NetConf{
				InterfaceType: "macvlan",
				InterfaceArgs: map[string]string{"master": "eno1", "mode": "bridge", "mtu": "1500"},
			},
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{OnCallMethodName: "RouteListFiltered", OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"}, RetArgList: []interface{}{[]netlink.Route{{LinkIndex: 0}}, nil}},
				{OnCallMethodName: "LinkByIndex", OnCallMethodArgType: []string{"int"}, RetArgList: []interface{}{mockLink, nil}},
				{OnCallMethodName: "LinkByName", OnCallMethodArgType: []string{"string"}, RetArgList: []interface{}{mockLink, nil}},
			},
			linkMockHelper: []egresstest.TestifyMockHelper{
				{OnCallMethodName: "Attrs", OnCallMethodArgType: []string{}, RetArgList: []interface{}{&netlink.LinkAttrs{Name: "eno1"}}},
				{OnCallMethodName: "Attrs", OnCallMethodArgType: []string{}, RetArgList: []interface{}{&netlink.LinkAttrs{MTU: 1500}}},
			},
		},
		{
			desc:           "missing explicit interface type when cloud provider specified",
			inpNetConf:     &NetConf{},
			inpClusterConf: &ClusterConf{"testProvider"},
			errMatch:       fmt.Errorf("must specify explicit interfaceType for cloud provider"),
		},
		{
			desc:			"error: unable to get the default route interface name",
			inpNetConf:     &NetConf{},
			inpClusterConf: &ClusterConf{},
			errMatch:       fmt.Errorf("unable to get default route interface name"),
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{OnCallMethodName: "RouteListFiltered", OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"}, RetArgList: []interface{}{nil, fmt.Errorf("mock error")}},
			},
		},
		{
			desc:			"error: unable to get MTU on master interface",
			inpNetConf:     &NetConf{},
			inpClusterConf: &ClusterConf{},
			errMatch:       fmt.Errorf("unable to get MTU on master interface"),
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{OnCallMethodName: "RouteListFiltered", OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"}, RetArgList: []interface{}{[]netlink.Route{{LinkIndex: 0}}, nil}},
				{OnCallMethodName: "LinkByIndex", OnCallMethodArgType: []string{"int"}, RetArgList: []interface{}{mockLink, nil}},
				{OnCallMethodName: "LinkByName", OnCallMethodArgType: []string{"string"}, RetArgList: []interface{}{nil, fmt.Errorf("mock error")}},
			},
			linkMockHelper: []egresstest.TestifyMockHelper{
				{OnCallMethodName: "Attrs", OnCallMethodArgType: []string{}, RetArgList: []interface{}{&netlink.LinkAttrs{Name: "eno1"}}},
			},
		},
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d:%s", i, tc.desc), func(t *testing.T) {
			egresstest.ProcessMockFnList(&mockNetLinkOps.Mock, tc.netOpsMockHelper)
			egresstest.ProcessMockFnList(&mockLink.Mock, tc.linkMockHelper)

			err := fillNetConfDefaults(tc.inpNetConf, tc.inpClusterConf)

			t.Log(err)
			t.Log(tc.inpClusterConf)
			t.Log(tc.inpNetConf)

			if tc.errMatch != nil {
				assert.Contains(t, err.Error(), tc.errMatch.Error())
			} else {
				assert.Equal(t, tc.inpNetConf, tc.outNetConf)
			}
			mockLink.AssertExpectations(t)
			mockNetLinkOps.AssertExpectations(t)
		})
	}
}
