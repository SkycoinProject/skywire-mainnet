package dmsgpty

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/cipher"
)

// CLI connects with and has ownership over a dmsgpty.Host.
type CLI struct {
	Log  logrus.FieldLogger `json:"-"`
	Net  string             `json:"cli_network"`
	Addr string             `json:"cli_address"`
}

// DefaultCLI returns a CLI with default values.
func DefaultCLI() CLI {
	return CLI{
		Log:  logging.MustGetLogger("dmsgpty-cli"),
		Net:  DefaultCLINet,
		Addr: DefaultCLIAddr,
	}
}

// WhitelistClient returns a client that interacts with the Host's whitelist.
func (cli *CLI) WhitelistClient() (*WhitelistClient, error) {
	conn, err := cli.prepareConn()
	if err != nil {
		return nil, err
	}
	return NewWhitelistClient(conn)
}

// StartLocalPty starts a pty on the host.
func (cli *CLI) StartLocalPty(ctx context.Context, cmd string, args ...string) error {
	conn, err := cli.prepareConn()
	if err != nil {
		return err
	}

	ptyC, err := NewPtyClient(conn)
	if err != nil {
		return err
	}

	restore, err := cli.prepareStdin()
	if err != nil {
		return err
	}
	defer restore()

	return cli.servePty(ctx, ptyC, cmd, args)
}

// StartRemotePty starts a pty on a remote host, proxied via the local pty.
func (cli *CLI) StartRemotePty(ctx context.Context, rPK cipher.PubKey, rPort uint16, cmd string, args ...string) error {
	conn, err := cli.prepareConn()
	if err != nil {
		return err
	}

	ptyC, err := NewProxyClient(conn, rPK, rPort)
	if err != nil {
		return err
	}

	restore, err := cli.prepareStdin()
	if err != nil {
		return err
	}
	defer restore()

	return cli.servePty(ctx, ptyC, cmd, args)
}

// prepareConn prepares a connection with the dmsgpty-host.
func (cli *CLI) prepareConn() (net.Conn, error) {

	// Set defaults.
	if cli.Log == nil {
		cli.Log = logging.MustGetLogger("dmsgpty-cli")
	}
	if cli.Net == "" {
		cli.Net = DefaultCLINet
	}
	if cli.Addr == "" {
		cli.Addr = DefaultCLIAddr
	}

	cli.Log.
		WithField("address", fmt.Sprintf("%s://%s", cli.Net, cli.Addr)).
		Infof("Requesting...")

	conn, err := net.Dial(cli.Net, cli.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dmsgpty-host: %v", err)
	}
	return conn, nil
}

// servePty serves a pty connection via the dmsgpty-host.
func (cli *CLI) servePty(ctx context.Context, ptyC *PtyClient, cmd string, args []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cli.Log.
		WithField("cmd", fmt.Sprint(append([]string{cmd}, args...))).
		Infof("Executing...")

	if err := ptyC.Start(cmd, args...); err != nil {
		return fmt.Errorf("failed to start command on pty: %v", err)
	}

	// Window resize loop.
	go func() {
		defer cancel()
		if err := ptyResizeLoop(ctx, ptyC); err != nil {
			cli.Log.
				WithError(err).
				Warn("Window resize loop closed with error.")
		}
	}()

	// Write loop.
	go func() {
		defer cancel()
		_, _ = io.Copy(ptyC, os.Stdin) //nolint:errcheck
	}()

	// Read loop.
	if _, err := io.Copy(os.Stdout, ptyC); err != nil {
		cli.Log.
			WithError(err).
			Error("Read loop closed with error.")
	}

	return nil
}
