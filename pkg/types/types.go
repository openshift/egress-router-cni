package types

import (
	"github.com/containernetworking/cni/pkg/types"
)

// ClusterConf specifies the Cloud Provider in use
type ClusterConf struct {
	CloudProvider string `json:"cloudProvider"`
}

// NetConf gets the networking configuration for Egress Router CNI
type NetConf struct {
	types.NetConf

	InterfaceType string            `json:"interfaceType"`
	InterfaceArgs map[string]string `json:"interfaceArgs"`

	IP    *IP           `json:"ip"`
	PodIP map[string]IP `json:"podIP"`

	LogFile  string `json:"log_file,omitempty"`
	LogLevel string `json:"log_level.omitempty"`
}

// IP sets the config for the Egress Router CNI pod
type IP struct {
	Addresses    []string `json:"addresses"`
	Gateway      string   `json:"gateway"`
	Destinations []string `json:"destinations"`
}
