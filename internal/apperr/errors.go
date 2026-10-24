package apperr

import (
	"errors"
	"net/http"
)

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code int, message string, err error) *AppError {
	return &AppError{Code: code, Message: message, Err: err}
}

// Sentinel constructors.
func BadRequest(msg string) *AppError  { return New(http.StatusBadRequest, msg) }
func Unauthorized(msg string) *AppError { return New(http.StatusUnauthorized, msg) }
func Forbidden(msg string) *AppError   { return New(http.StatusForbidden, msg) }
func NotFound(msg string) *AppError    { return New(http.StatusNotFound, msg) }
func Conflict(msg string) *AppError    { return New(http.StatusConflict, msg) }
func Internal(err error) *AppError     { return Wrap(http.StatusInternalServerError, "internal server error", err) }

// HTTPCode extracts the HTTP status code from any error.
// Falls back to 500 for unknown errors.
func HTTPCode(err error) int {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return http.StatusInternalServerError
}

// HTTPMessage extracts the user-facing message from any error.
func HTTPMessage(err error) string {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.Message
	}
	return "internal server error"
}
