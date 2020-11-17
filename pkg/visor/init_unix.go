//+build !windows

package visor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/skycoin/dmsg/dmsgpty"
)

const ownerRWX = 0700

// InitDmsgpty initializes dmsgpty.
func InitDmsgpty(ctx context.Context, v *Visor) error {
	report := v.makeReporter("dmsgpty")
	conf := v.conf.Dmsgpty

	if conf == nil {
		v.log.Info("'dmsgpty' is not configured, skipping.")
		return report(nil)
	}

	// Unlink dmsg socket files (just in case).
	if conf.CLINet == "unix" {
		if err := unlinkSocketFiles(v.conf.Dmsgpty.CLIAddr); err != nil {
			return report(err)
		}
	}

	var wl dmsgpty.Whitelist
	if conf.AuthFile == "" {
		wl = dmsgpty.NewMemoryWhitelist()
	} else {
		var err error
		if wl, err = dmsgpty.NewJSONFileWhiteList(v.conf.Dmsgpty.AuthFile); err != nil {
			return report(err)
		}
	}

	// Ensure hypervisors are added to the whitelist.
	if err := wl.Add(v.conf.Hypervisors...); err != nil {
		return report(err)
	}

	dmsgC := v.net.Dmsg()
	if dmsgC == nil {
		return report(errors.New("cannot create dmsgpty with nil dmsg client"))
	}

	pty := dmsgpty.NewHost(dmsgC, wl)

	if ptyPort := conf.Port; ptyPort != 0 {
		cancellableCtx, cancel := context.WithCancel(ctx)
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := pty.ListenAndServe(cancellableCtx, ptyPort); err != nil {
				report(fmt.Errorf("listen and serve stopped: %w", err))
			}
		}()

		v.pushCloseStack("dmsgpty.serve", func() error {
			cancel()
			wg.Wait()
			return report(nil)
		})
	}

	if conf.CLINet != "" {
		if conf.CLINet == "unix" {
			if err := os.MkdirAll(filepath.Dir(conf.CLIAddr), ownerRWX); err != nil {
				return report(fmt.Errorf("failed to prepare unix file for dmsgpty cli listener: %w", err))
			}
		}

		cliL, err := net.Listen(conf.CLINet, conf.CLIAddr)
		if err != nil {
			return report(fmt.Errorf("failed to start dmsgpty cli listener: %w", err))
		}

		cancellableCtx, cancel := context.WithCancel(ctx)
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := pty.ServeCLI(cancellableCtx, cliL); err != nil {
				report(fmt.Errorf("serve cli stopped: %w", err))
			}
		}()

		v.pushCloseStack("dmsgpty.cli", func() error {
			cancel()
			err := report(cliL.Close())
			wg.Wait()
			return err
		})
	}

	return report(nil)
}
