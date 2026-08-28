package clierr

import (
	"errors"
	"fmt"
)

const (
	ExitOK         = 0
	ExitUsage      = 2
	ExitAuth       = 3
	ExitNotFound   = 4
	ExitConflict   = 5
	ExitNetwork    = 6
	ExitServer     = 7
	ExitTimeout    = 8
	ExitValidation = 9
)

type Error struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Hint     string         `json:"hint,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
	ExitCode int            `json:"-"`
	Cause    error          `json:"-"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Cause }

func New(code, message string, exitCode int) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode}
}

func Wrap(code, message string, exitCode int, cause error) *Error {
	return &Error{Code: code, Message: message, ExitCode: exitCode, Cause: cause}
}

func Usage(message string) *Error {
	return New("usage_error", message, ExitUsage)
}

func Validation(message, hint string) *Error {
	return &Error{Code: "validation_failed", Message: message, Hint: hint, ExitCode: ExitValidation}
}

func As(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		if target.ExitCode == 0 {
			target.ExitCode = ExitServer
		}
		return target
	}
	return Wrap("internal_error", fmt.Sprintf("unexpected error: %v", err), ExitServer, err)
}

func Code(err error) int {
	if err == nil {
		return ExitOK
	}
	return As(err).ExitCode
}
