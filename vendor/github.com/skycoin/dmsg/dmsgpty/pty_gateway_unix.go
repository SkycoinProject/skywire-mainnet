//+build !windows

package dmsgpty

import (
	"github.com/creack/pty"
)

// PtyGateway represents a pty gateway, hosted by the pty.SessionServer
type PtyGateway interface {
	Start(req *CommandReq, _ *struct{}) error
	Stop(_, _ *struct{}) error
	Read(reqN *int, respB *[]byte) error
	Write(reqB *[]byte, respN *int) error
	SetPtySize(size *pty.Winsize, _ *struct{}) error
}

// LocalPtyGateway is the gateway to a local pty.

// CommandReq represents a pty command.
type CommandReq struct {
	Name string
	Arg  []string
	Size *pty.Winsize
}

// SetPtySize sets the local pty's window size.
func (g *LocalPtyGateway) SetPtySize(size *pty.Winsize, _ *struct{}) error {
	return g.ses.SetPtySize(size)
}

// SetPtySize sets the remote pty's window size.
func (g *ProxiedPtyGateway) SetPtySize(size *pty.Winsize, _ *struct{}) error {
	return g.ptyC.SetPtySize(size)
}
