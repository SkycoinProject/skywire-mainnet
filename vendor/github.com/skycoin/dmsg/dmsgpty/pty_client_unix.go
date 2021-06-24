//+build !windows

package dmsgpty

import (
	"os"

	"github.com/creack/pty"
)

// Start starts the pty.
func (sc *PtyClient) Start(name string, arg ...string) error {
	size, err := pty.GetsizeFull(os.Stdin)
	if err != nil {
		sc.log.WithError(err).Warn("failed to obtain terminal size")
		size = nil
	}
	return sc.StartWithSize(name, arg, size)
}

// StartWithSize starts the pty with a specified size.
func (sc *PtyClient) StartWithSize(name string, arg []string, size *pty.Winsize) error {
	return sc.call("Start", &CommandReq{Name: name, Arg: arg, Size: size}, &empty)
}

// SetPtySize sets the pty size.
func (sc *PtyClient) SetPtySize(size *pty.Winsize) error {
	return sc.call("SetPtySize", size, &empty)
}
