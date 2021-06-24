//+build !windows

package dmsgpty

// Constants related to CLI.
const (
	DefaultCLINet  = "unix"
	DefaultCLIAddr = "/tmp/dmsgpty.sock"
)

// Constants related to dmsg.
const (
	DefaultPort = uint16(22)
	DefaultCmd  = "/bin/bash"
)
