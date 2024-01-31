package domain

import (
	"errors"
	"runtime"
)

var ErrNotFound = errors.New("record not found")
var ErrNotUnique = errors.New("record not unique")

// GetStackTrace returns a stack trace of the current goroutine. The stack trace has at most 1024 bytes.
func GetStackTrace() string {
	b := make([]byte, 1024)
	n := runtime.Stack(b, false)
	s := string(b[:n])

	return s
}
