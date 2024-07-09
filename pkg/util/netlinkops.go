//go:build linux
// +build linux

package util

import (
	"github.com/vishvananda/netlink"
	"net"
)

type NetLinkOps interface {
	LinkByName(ifaceName string) (netlink.Link, error)
	LinkByIndex(index int) (netlink.Link, error)
	LinkSetDown(link netlink.Link) error
	LinkSetName(link netlink.Link, newName string) error
	LinkSetUp(link netlink.Link) error
	LinkSetNsFd(link netlink.Link, fd int) error
	LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error
	LinkSetMTU(link netlink.Link, mtu int) error
	LinkSetTxQLen(link netlink.Link, qlen int) error
	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)
	AddrDel(link netlink.Link, addr *netlink.Addr) error
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteDel(route *netlink.Route) error
	RouteAdd(route *netlink.Route) error
	RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error)
	NeighAdd(neigh *netlink.Neigh) error
	NeighList(linkIndex, family int) ([]netlink.Neigh, error)
	ConntrackDeleteFilter(table netlink.ConntrackTableType, family netlink.InetFamily, filter netlink.CustomConntrackFilter) (uint, error)
}

type defaultNetLinkOps struct {
}

var netLinkOps NetLinkOps = &defaultNetLinkOps{}

// SetNetLinkOpMockInst method would be used by unit tests in other packages
func SetNetLinkOpMockInst(mockInst NetLinkOps) {
	netLinkOps = mockInst
}

// GetNetLinkOps will be invoked by functions in other packages that would need access to the netlink library methods.
func GetNetLinkOps() NetLinkOps {
	return netLinkOps
}

func (defaultNetLinkOps) LinkByName(ifaceName string) (netlink.Link, error) {
	return netlink.LinkByName(ifaceName)
}

func (defaultNetLinkOps) LinkByIndex(index int) (netlink.Link, error) {
	return netlink.LinkByIndex(index)
}

func (defaultNetLinkOps) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

func (defaultNetLinkOps) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (defaultNetLinkOps) LinkSetName(link netlink.Link, newName string) error {
	return netlink.LinkSetName(link, newName)
}

func (defaultNetLinkOps) LinkSetNsFd(link netlink.Link, fd int) error {
	return netlink.LinkSetNsFd(link, fd)
}

func (defaultNetLinkOps) LinkSetHardwareAddr(link netlink.Link, hwaddr net.HardwareAddr) error {
	return netlink.LinkSetHardwareAddr(link, hwaddr)
}

func (defaultNetLinkOps) LinkSetMTU(link netlink.Link, mtu int) error {
	return netlink.LinkSetMTU(link, mtu)
}

func (defaultNetLinkOps) LinkSetTxQLen(link netlink.Link, qlen int) error {
	return netlink.LinkSetTxQLen(link, qlen)
}

func (defaultNetLinkOps) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	return netlink.AddrList(link, family)
}

func (defaultNetLinkOps) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrDel(link, addr)
}

func (defaultNetLinkOps) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrAdd(link, addr)
}

func (defaultNetLinkOps) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}

func (defaultNetLinkOps) RouteDel(route *netlink.Route) error {
	return netlink.RouteDel(route)
}

func (defaultNetLinkOps) RouteAdd(route *netlink.Route) error {
	return netlink.RouteAdd(route)
}

func (defaultNetLinkOps) RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error) {
	return netlink.RouteListFiltered(family, filter, filterMask)
}

func (defaultNetLinkOps) NeighAdd(neigh *netlink.Neigh) error {
	return netlink.NeighAdd(neigh)
}

func (defaultNetLinkOps) NeighList(linkIndex, family int) ([]netlink.Neigh, error) {
	return netlink.NeighList(linkIndex, family)
}

func (defaultNetLinkOps) ConntrackDeleteFilter(table netlink.ConntrackTableType, family netlink.InetFamily, filter netlink.CustomConntrackFilter) (uint, error) {
	return netlink.ConntrackDeleteFilter(table, family, filter)
}
