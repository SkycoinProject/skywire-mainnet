//+build windows

package dmsgpty

// Constants related to CLI.
const (
	DefaultCLINet  = "tcp"
	DefaultCLIAddr = "localhost:8083"
)

// Constants related to dmsg.
const (
	DefaultPort = uint16(22)
	DefaultCmd  = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`
)
