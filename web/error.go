package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// StatusError is an error with an associated HTTP status code.
type StatusError interface {
	error

	// Status returns the HTTP status code associated with the error.
	Status() int
}

// statusError is a simple implementation of StatusError.
type statusError struct {
	err    error
	status int
}

// Error implements the error interface.
func (s statusError) Error() string {
	return s.err.Error()
}

// Status implements the StatusError interface.
func (s statusError) Status() int {
	return s.status
}

// Unwrap returns the underlying error.
func (s statusError) Unwrap() error {
	return s.err
}

// Error creates a new status error with a text message and status code.
func Error(status int, text string) StatusError {
	return &statusError{
		err:    errors.New(text),
		status: status,
	}
}

// Errorf creates a new formatted status error with a status code.
func Errorf(status int, format string, a ...any) StatusError {
	return &statusError{
		err:    fmt.Errorf(format, a...),
		status: status,
	}
}

// ErrorStatus creates a status error using the corresponding HTTP status text.
// If no status is provided, it defaults to 500 Internal Server Error.
func ErrorStatus(status ...int) StatusError {
	httpStatus := http.StatusInternalServerError
	if len(status) > 0 {
		httpStatus = status[0]
	}
	text := http.StatusText(httpStatus)
	if text == "" {
		text = http.StatusText(http.StatusInternalServerError)
	}
	return Error(httpStatus, strings.ToLower(text))
}

// ErrorWrap wraps an existing error with a status code.
func ErrorWrap(status int, err error) StatusError {
	if err == nil {
		return nil
	}
	return &statusError{
		err:    err,
		status: status,
	}
}
