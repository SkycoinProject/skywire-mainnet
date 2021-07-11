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
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/skywire/pkg/util/osutil"
)

const ownerRWX = 0700

func initDmsgpty(ctx context.Context, v *Visor, log *logging.Logger) error {
	conf := v.conf.Dmsgpty

	if conf == nil {
		log.Info("'dmsgpty' is not configured, skipping.")
		return nil
	}

	// Unlink dmsg socket files (just in case).
	if conf.CLINet == "unix" {
		if err := osutil.UnlinkSocketFiles(v.conf.Dmsgpty.CLIAddr); err != nil {
			return err
		}
	}

	wl := dmsgpty.NewMemoryWhitelist()

	// Ensure hypervisors are added to the whitelist.
	if err := wl.Add(v.conf.Hypervisors...); err != nil {
		return err
	}

	// add itself to the whitelist to allow local pty
	if err := wl.Add(v.conf.PK); err != nil {
		v.log.Errorf("Cannot add itself to the pty whitelist: %s", err)
	}

	dmsgC := v.dmsgC
	if dmsgC == nil {
		err := errors.New("cannot create dmsgpty with nil dmsg client")
		return err
	}

	pty := dmsgpty.NewHost(dmsgC, wl)

	if ptyPort := conf.Port; ptyPort != 0 {
		serveCtx, cancel := context.WithCancel(context.Background())
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			defer wg.Done()
			runtimeErrors := getErrors(ctx)
			if err := pty.ListenAndServe(serveCtx, ptyPort); err != nil {
				runtimeErrors <- fmt.Errorf("listen and serve stopped: %w", err)
			}
		}()

		v.pushCloseStack("router.serve", func() error {
			cancel()
			wg.Wait()
			return nil
		})

	}

	if conf.CLINet != "" {
		if conf.CLINet == "unix" {
			if err := os.MkdirAll(filepath.Dir(conf.CLIAddr), ownerRWX); err != nil {
				err := fmt.Errorf("failed to prepare unix file for dmsgpty cli listener: %w", err)
				return err
			}
		}

		cliL, err := net.Listen(conf.CLINet, conf.CLIAddr)
		if err != nil {
			err := fmt.Errorf("failed to start dmsgpty cli listener: %w", err)
			return err
		}

		serveCtx, cancel := context.WithCancel(context.Background())
		wg := new(sync.WaitGroup)
		wg.Add(1)

		go func() {
			defer wg.Done()
			runtimeErrors := getErrors(ctx)
			if err := pty.ServeCLI(serveCtx, cliL); err != nil {
				runtimeErrors <- fmt.Errorf("serve cli stopped: %w", err)
			}
		}()

		v.pushCloseStack("router.serve", func() error {
			cancel()
			err := cliL.Close()
			wg.Wait()
			return err
		})
	}

	return nil
}
