//+build windows

package vpn

import (
	"bytes"
	"net"
	"regexp"

	"github.com/skycoin/skywire/pkg/util/osutil"
)

const (
	defaultNetworkGatewayCMD = "route PRINT"
)

var redundantWhitespacesCleanupRegex = regexp.MustCompile(`[\s\p{Zs}]{2,}`)

// DefaultNetworkGateway fetches system's default network gateway.
func DefaultNetworkGateway() (net.IP, error) {
	outBytes, err := osutil.RunWithResult("cmd", "/C", defaultNetworkGatewayCMD)
	if err != nil {
		return nil, err
	}

	outBytes = bytes.TrimRight(outBytes, "\n\r")

	lines := bytes.Split(outBytes, []byte{'\n'})
	for _, line := range lines {
		line = bytes.TrimLeft(line, " \t\r\n")
		if !bytes.HasPrefix(line, []byte("0.0.0.0")) {
			continue
		}

		line = bytes.TrimRight(line, " \t\r\n")

		line := redundantWhitespacesCleanupRegex.ReplaceAll(line, []byte{' '})

		lineTokens := bytes.Split(line, []byte{' '})
		if len(lineTokens) < 2 {
			continue
		}

		ip := net.ParseIP(string(lineTokens[2]))
		if ip != nil && ip.To4() != nil {
			return ip, nil
		}
	}

	return nil, errCouldFindDefaultNetworkGateway
}

func setupClientSysPrivileges() (suid int, err error) {
	return 0, nil
}
