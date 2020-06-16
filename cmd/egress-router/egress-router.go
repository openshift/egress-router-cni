package main

import (
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/openshift/egress-router-cni/pkg/macvlan"
)

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("egress-router"))
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

func cmdAdd(args *skel.CmdArgs) error {
	macvlan.CmdAdd(args)

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	macvlan.CmdDel(args)

	return nil
}
