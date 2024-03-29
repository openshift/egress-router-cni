// Code generated by mockery v2.2.1. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	netlink "github.com/vishvananda/netlink"
)

// Link is an autogenerated mock type for the Link type
type Link struct {
	mock.Mock
}

// Attrs provides a mock function with given fields:
func (_m *Link) Attrs() *netlink.LinkAttrs {
	ret := _m.Called()

	var r0 *netlink.LinkAttrs
	if rf, ok := ret.Get(0).(func() *netlink.LinkAttrs); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*netlink.LinkAttrs)
		}
	}

	return r0
}

// Type provides a mock function with given fields:
func (_m *Link) Type() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
