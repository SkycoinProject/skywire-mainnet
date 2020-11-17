package visorerr

import (
	"strconv"
)

// ErrorWithCode is used to combine `ErrCode` with the original error.
type ErrorWithCode struct {
	Err  error
	Code ErrCode
}

// NewErrorWithCode constructs `ErrorWithCode`.
func NewErrorWithCode(err error, code ErrCode) error {
	return &ErrorWithCode{
		Err:  err,
		Code: code,
	}
}

// Error implements `error`.
func (e *ErrorWithCode) Error() string {
	return "code " + strconv.Itoa(int(e.Code)) + ": " + e.Err.Error()
}
