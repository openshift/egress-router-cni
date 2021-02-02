package macvlan

import (
	"fmt"
	"net"
	"testing"

	egresstest "github.com/openshift/egress-router-cni/pkg/testing"
	netlink_mocks "github.com/openshift/egress-router-cni/pkg/testing/mocks/github.com/vishvananda/netlink"
	"github.com/openshift/egress-router-cni/pkg/types"
	util "github.com/openshift/egress-router-cni/pkg/util"
	util_mocks "github.com/openshift/egress-router-cni/pkg/util/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func TestFillNetConfDefaults(t *testing.T) {
	mockNetLinkOps := new(util_mocks.NetLinkOps)
	mockLink := new(netlink_mocks.Link)
	// below sets the `netLinkOps` in util/net_linux.go to a mock instance for purpose of unit tests execution
	util.SetNetLinkOpMockInst(mockNetLinkOps)

	tests := []struct {
		desc             string
		inpNetConf       *types.NetConf
		inpClusterConf   *types.ClusterConf
		outNetConf       *types.NetConf
		errMatch         error
		netOpsMockHelper []egresstest.TestifyMockHelper
		linkMockHelper   []egresstest.TestifyMockHelper
	}{
		{
			desc:           "empty NetConf and ClusterConf struct",
			inpNetConf:     &types.NetConf{},
			inpClusterConf: &types.ClusterConf{},
			outNetConf: &types.NetConf{
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
			inpNetConf:     &types.NetConf{},
			inpClusterConf: &types.ClusterConf{CloudProvider: "testProvider"},
			errMatch:       fmt.Errorf("must specify explicit interfaceType for cloud provider"),
		},
		{
			desc:           "error: unable to get the default route interface name",
			inpNetConf:     &types.NetConf{},
			inpClusterConf: &types.ClusterConf{},
			errMatch:       fmt.Errorf("unable to get default route interface name"),
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{OnCallMethodName: "RouteListFiltered", OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"}, RetArgList: []interface{}{nil, fmt.Errorf("mock error")}},
			},
		},
		{
			desc:           "error: unable to get MTU on master interface",
			inpNetConf:     &types.NetConf{},
			inpClusterConf: &types.ClusterConf{},
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

func TestGetDefaultRouteInterfaceName(t *testing.T) {
	mockNetLinkOps := new(util_mocks.NetLinkOps)
	mockLink := new(netlink_mocks.Link)


	// below sets the `netLinkOps` in util/net_linux.go to a mock instance for
	// purpose of unit tests execution
	util.SetNetLinkOpMockInst(mockNetLinkOps)

	tests := []struct {
		desc             string
		errMatch         error
		netOpsMockHelper []egresstest.TestifyMockHelper
		linkMockHelper   []egresstest.TestifyMockHelper
	}{
		{
			desc: "mock unable to list routes",
			errMatch: fmt.Errorf("mock unable to list routes"),
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{
					OnCallMethodName: "RouteListFiltered",	OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"},
					RetArgList: []interface{}{nil, fmt.Errorf("mock unable to list routes")}},
			},
		},
		{
			desc: "mock no default route interface found",
			errMatch: fmt.Errorf("no default route interface found"),
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{
					OnCallMethodName: "RouteListFiltered",
					OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"},
					RetArgList: []interface{}{[]netlink.Route{
						{
							LinkIndex: 0,
							Dst: &net.IPNet{
								IP:   net.ParseIP("192.168.1.10"),
								Mask: net.CIDRMask(24, 32)}}}, nil}},
			},
		},
		{
			desc:     "mock get link by index with err",
			errMatch: fmt.Errorf("mock unable to get link by index"),
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{
					OnCallMethodName: "RouteListFiltered",
					OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"},
					RetArgList: []interface{}{[]netlink.Route{{LinkIndex: 0, Dst: nil}}, nil}},
				{
					OnCallMethodName: "LinkByIndex",
					OnCallMethodArgType: []string{"int"},
					RetArgList: []interface{}{nil, fmt.Errorf("mock unable to get link by index")}},
			},
		},
		{
			desc: "mock assert default route interface name",
			errMatch: nil,
			netOpsMockHelper: []egresstest.TestifyMockHelper{
				{
					OnCallMethodName:    "RouteListFiltered",
					OnCallMethodArgType: []string{"int", "*netlink.Route", "uint64"},
					RetArgList:          []interface{}{[]netlink.Route{{LinkIndex: 0, Dst: nil}}, nil},
				},
				{
					OnCallMethodName: "LinkByIndex",
					OnCallMethodArgType: []string{"int"},
					RetArgList: []interface{}{mockLink, nil},
				},
			},
			linkMockHelper: []egresstest.TestifyMockHelper{
				{
					OnCallMethodName: "Attrs",
					OnCallMethodArgType: []string{}, RetArgList: []interface{}{&netlink.LinkAttrs{Name: "dummy0"}},
					CallTimes: 2,
				},
			},

		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d:%s", i, tc.desc), func(t *testing.T) {
			egresstest.ProcessMockFnList(&mockNetLinkOps.Mock, tc.netOpsMockHelper)
			egresstest.ProcessMockFnList(&mockLink.Mock, tc.linkMockHelper)

			_, err := getDefaultRouteInterfaceName()

			t.Log(err)

			if tc.errMatch != nil {
				assert.Contains(t, err.Error(), tc.errMatch.Error())
				mockLink.AssertExpectations(t)
				mockNetLinkOps.AssertExpectations(t)
			}
		})
	}
}
