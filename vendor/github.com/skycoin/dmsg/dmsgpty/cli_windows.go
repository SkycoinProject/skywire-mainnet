//+build windows

package dmsgpty

import (
	"context"
	"sync"
	"time"

	"github.com/ActiveState/termtest/conpty"
	"golang.org/x/sys/windows"
)

// ptyResizeLoop informs the remote of changes to the local CLI terminal window size.
func ptyResizeLoop(ctx context.Context, ptyC *PtyClient) error {
	t := time.NewTicker(1 * time.Second)
	mu := sync.RWMutex{}
	var initialSize *windows.Coord
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return nil
		case <-t.C:
			mu.Lock()
			size, err := getSize()
			if err == nil {
				if initialSize == nil {
					initialSize = size
				} else if initialSize.X != size.X || initialSize.Y != size.Y {
					initialSize = size
					if err = ptyC.SetPtySize(initialSize); err != nil {
						mu.Unlock()
						return err
					}
				}
			}
			mu.Unlock()
		}
	}
}

func (cli *CLI) prepareStdin() (restore func(), err error) {
	return conpty.InitTerminal(true)
}
