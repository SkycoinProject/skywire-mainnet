// +build windows

package restart

import (
	"os/exec"
)

func attachTTY(_ *exec.Cmd) {
	// not used for Windows
}

func (c *Context) ignoreSignals() {
}
