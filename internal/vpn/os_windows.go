//+build windows

package vpn

import (
	"fmt"

	"github.com/skycoin/skywire/pkg/util/osutil"
)

const (
	tunSetupCMDFmt    = "netsh interface ip set address name=\"%s\" source=static addr=%s mask=%s gateway=%s"
	tunMTUSetupCMDFmt = "netsh interface ipv4 set subinterface \"%s\" mtu=%d"
	modifyRouteCMDFmt = "route %s %s mask %s %s"
)

// SetupTUN sets the allocated TUN interface up, setting its IP, gateway, netmask and MTU.
func SetupTUN(ifcName, ipCIDR, gateway string, mtu int) error {
	ip, netmask, err := parseCIDR(ipCIDR)
	if err != nil {
		return fmt.Errorf("error parsing IP CIDR: %w", err)
	}

	setupCmd := fmt.Sprintf(tunSetupCMDFmt, ifcName, ip, netmask, gateway)
	if err := osutil.Run("cmd", "/C", setupCmd); err != nil {
		return fmt.Errorf("error running command %s: %w", setupCmd, err)
	}

	mtuSetupCmd := fmt.Sprintf(tunMTUSetupCMDFmt, ifcName, mtu)
	if err := osutil.Run("cmd", "/C", mtuSetupCmd); err != nil {
		return fmt.Errorf("error running command %s: %w", mtuSetupCmd, err)
	}

	return nil
}

// ChangeRoute changes current route to `ipCIDR` to go through the `gateway`
// in the OS routing table.
func ChangeRoute(ipCIDR, gateway string) error {
	return modifyRoutingTable("change", ipCIDR, gateway)
}

// AddRoute adds route to `ipCIDR` through the `gateway` to the OS routing table.
func AddRoute(ipCIDR, gateway string) error {
	return modifyRoutingTable("add", ipCIDR, gateway)
}

// DeleteRoute removes route to `ipCIDR` through the `gateway` from the OS routing table.
func DeleteRoute(ipCIDR, gateway string) error {
	return modifyRoutingTable("delete", ipCIDR, gateway)
}

func modifyRoutingTable(action, ipCIDR, gateway string) error {
	ip, netmask, err := parseCIDR(ipCIDR)
	if err != nil {
		return fmt.Errorf("error parsing IP CIDR: %w", err)
	}

	cmd := fmt.Sprintf(modifyRouteCMDFmt, action, ip, netmask, gateway)
	return osutil.Run("cmd", "/C", cmd)
}
