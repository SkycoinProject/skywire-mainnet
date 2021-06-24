// +build !arm64,windows

package stdio

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

// DupTo basically just duplicates / capture the handle into the new handle
func DupTo(stdhandle uintptr, handle windows.Handle) error {
	procSetHandle := windows.MustLoadDLL("kernel32.dll").MustFindProc("SetStdHandle")
	r0, _, e1 := syscall.Syscall(procSetHandle.Addr(), 2, stdhandle, uintptr(handle), 0)
	if r0 == 0 {
		if e1 != 0 {
			return error(e1)
		}
		return syscall.EINVAL
	}
	return nil
}

// CaptureStdout captures stdout output
func (oc *outputCapturer) CaptureStdout() (io.Writer, error) {
	oc.capturing = true
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	oldStdout := int(stdoutWriter.Fd())

	if err = DupTo(stdoutWriter.Fd(), windows.Stdout); err != nil {
		return nil, err
	}

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	oldStderr := int(stderrWriter.Fd())

	if err = DupTo(stderrWriter.Fd(), windows.Stderr); err != nil {
		return nil, err
	}

	oc.origStdoutFD = oldStdout
	oc.origStderrFD = oldStderr
	oc.stdoutReader = stdoutReader
	oc.stderrReader = stderrReader
	oc.stdoutWriter = stdoutWriter
	oc.stderrWriter = stderrWriter

	origStdoutWriter := os.NewFile(uintptr(oc.origStdoutFD), "/dev/stdout")
	return origStdoutWriter, nil
}

func (oc *outputCapturer) Release() error {
	if !oc.capturing {
		return ErrReleaseNoncaptured
	}
	if err := oc.stdoutWriter.Close(); err != nil {
		return err
	}

	if err := oc.stderrWriter.Close(); err != nil {
		return err
	}

	if err := windows.Close(windows.Stdout); err != nil {
		return err
	}

	if err := windows.Close(windows.Stderr); err != nil {
		return err
	}

	if err := DupTo(uintptr(oc.origStdoutFD), windows.Stdout); err != nil {
		return err
	}

	if err := DupTo(uintptr(oc.origStderrFD), windows.Stderr); err != nil {
		return err
	}

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer

	if _, err := io.Copy(&stdoutBuffer, oc.stdoutReader); err != nil {
		return err
	}

	if _, err := io.Copy(&stderrBuffer, oc.stderrReader); err != nil {
		return err
	}

	if _, err := fmt.Fprint(os.Stdout, stdoutBuffer.String()); err != nil {
		return err
	}

	if _, err := fmt.Fprint(os.Stderr, stderrBuffer.String()); err != nil {
		return err
	}

	return nil
}
