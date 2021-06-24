package stdio

import (
	"errors"
	"io"
	"os"
)

// ErrReleaseNoncaptured is returned on attempt to release a capturer that hasn't started capturing
var ErrReleaseNoncaptured = errors.New("releasing non-captured output")

// OutputCapturer allows to capture output to stdout/stderr and hold it temporarily
// while giving ability to write to stdout through a separate writer
type OutputCapturer interface {
	// CaptureStdout starts capturing all output that is written to stdout/stderr
	// return a separate writer for writing to stdout
	CaptureStdout() (io.Writer, error)
	// Release captured output to the screen, as well as stop capturing any further stout/stderr output
	Release() error
}

// NewCapturer creates a new output capturer
func NewCapturer() OutputCapturer {
	return &outputCapturer{}
}

type outputCapturer struct {
	capturing    bool
	origStdoutFD int
	origStderrFD int
	stdoutReader *os.File
	stderrReader *os.File
	stdoutWriter *os.File
	stderrWriter *os.File
}
