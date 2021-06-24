//+build !windows

package dmsgpty

import (
	"github.com/creack/pty"
)

func (ui *UI) uiStartSize(ptyC *PtyClient) error {
	return ptyC.StartWithSize(ui.conf.CmdName, ui.conf.CmdArgs, &pty.Winsize{Rows: wsRows, Cols: wsCols})
}
