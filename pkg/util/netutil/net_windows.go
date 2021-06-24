//+build windows

package netutil

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
)

const (
	defaultNetworkInterfaceCMD = `netsh int ip show config | findstr /r "IP Address.*([0-9]{1,3}\.|){4}"`
)

// DefaultNetworkInterface fetches default network interface name.
func DefaultNetworkInterface() (string, error) {
	cmd := exec.Command("powershell", defaultNetworkInterfaceCMD)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// parse output
	splitLines := strings.Split(string(output), "\n")
	var ips []string

	if len(splitLines) > 0 {
		re := regexp.MustCompile(`\s+`)
		for _, line := range splitLines {
			ipAddr := re.Split(strings.TrimSpace(line), -1)

			if len(ipAddr) > 2 {
				ip := net.ParseIP(ipAddr[2])
				if ip != nil && !ip.IsLoopback() {
					ips = append(ips, ipAddr[2])
				}
			}
		}
	}

	if len(ips) == 0 {
		return "", errors.New("no active ip found")
	}

	// get default network interface based on its ip
	findInterfaceCmd := fmt.Sprintf("Get-NetIpAddress -IPAddress '%s' | %%{$_.InterfaceAlias}", ips[0])
	cmd = exec.Command("powershell", findInterfaceCmd) // nolint:gosec
	output, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("unable to get default interface: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}
