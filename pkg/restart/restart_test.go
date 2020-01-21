package restart

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptureContext(t *testing.T) {
	cc := CaptureContext()

	require.Equal(t, DefaultCheckDelay, cc.checkDelay)
	require.Equal(t, os.Args, cc.cmd.Args)
	require.Equal(t, os.Stdout, cc.cmd.Stdout)
	require.Equal(t, os.Stdin, cc.cmd.Stdin)
	require.Equal(t, os.Stderr, cc.cmd.Stderr)
	require.Equal(t, os.Environ(), cc.cmd.Env)
	require.Nil(t, cc.log)
}

func TestContext_RegisterLogger(t *testing.T) {
	cc := CaptureContext()
	require.Nil(t, cc.log)

	logger := logging.MustGetLogger("test")
	cc.RegisterLogger(logger)
	require.Equal(t, logger, cc.log)
}

func TestContext_Start(t *testing.T) {
	cc := CaptureContext()
	assert.NotZero(t, len(cc.cmd.Args))

	t.Run("executable started", func(t *testing.T) {
		cmd := "touch"
		path := "/tmp/test_start"
		cc.cmd = exec.Command(cmd, path) // nolint:gosec
		cc.appendDelay = false

		assert.NoError(t, cc.Start())
		assert.NoError(t, os.Remove(path))
	})

	t.Run("bad args", func(t *testing.T) {
		cmd := "bad_command"
		cc.cmd = exec.Command(cmd) // nolint:gosec

		// TODO(nkryuchkov): Add error text for Windows
		possibleErrors := []string{
			`exec: "bad_command": executable file not found in $PATH`,
		}
		err := cc.Start()
		require.NotNil(t, err)
		assert.Contains(t, possibleErrors, err.Error())
	})

	t.Run("already starting", func(t *testing.T) {
		cmd := "touch"
		path := "/tmp/test_start"
		cc.cmd = exec.Command(cmd, path) // nolint:gosec
		cc.appendDelay = false

		errCh := make(chan error, 1)
		go func() {
			errCh <- cc.Start()
		}()

		err1 := cc.Start()
		err2 := <-errCh
		errors := []error{err1, err2}

		assert.Contains(t, errors, ErrAlreadyStarting)
		assert.Contains(t, errors, nil)

		assert.NoError(t, os.Remove(path))
	})
}

func TestContext_SetCheckDelay(t *testing.T) {
	cc := CaptureContext()
	require.Equal(t, DefaultCheckDelay, cc.checkDelay)

	const oneSecond = 1 * time.Second

	cc.SetCheckDelay(oneSecond)
	require.Equal(t, oneSecond, cc.checkDelay)
}
