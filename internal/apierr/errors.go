// Package apierr defines the stable application error codes and the error type
// carried through the service and transport layers.
//
// Every failure returned by the HTTP API carries three pieces of information:
//
//   - an HTTP status code, used by clients, load balancers and observability tools;
//   - a stable application error code (Code), safe to branch on in client code;
//   - a human readable message, safe to display to end users.
//
// The numeric codes are part of the public API contract: once released they must
// not be reused for a different meaning. Add a new code instead.
package apierr

import (
	"context"
	"errors"
	"net/http"
)

// Application error codes. Zero always means "success"; every failure uses a
// code in the 4xxx (client) or 5xxx (server) range.
const (
	CodeOK = 0

	// 4000–4099: the caller can usually fix the request.
	CodeBadRequest   = 4000
	CodeValidation   = 4001
	CodeUnauthorized = 4010
	CodeForbidden    = 4030
	CodeNotFound     = 4040
	CodeConflict     = 4090

	// 4200–4299: request rejected to protect the service.
	CodeRateLimited     = 4290
	CodePayloadTooLarge = 4130

	// 5000–5999: the caller cannot fix these; alert on them.
	CodeInternal    = 5000
	CodeUnavailable = 5030
)

// Error is the canonical service error.
type Error struct {
	// Code is the stable application error code.
	Code int
	// Message is safe to return to the client.
	Message string
	// Status is the HTTP status code used for the response.
	Status int
	// Details carries optional structured, user-safe context (e.g. invalid fields).
	Details any
	// Cause is the underlying error; it is logged but never serialised.
	Cause error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap exposes the underlying error to errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Cause }

// New builds an Error from an application code, deriving status and message.
func New(code int, message string) *Error {
	if message == "" {
		message = http.StatusText(statusForCode(code))
	}
	return &Error{Code: code, Message: message, Status: statusForCode(code)}
}

// Wrap annotates an arbitrary error with an application code.
func Wrap(code int, message string, cause error) *Error {
	err := New(code, message)
	err.Cause = cause
	return err
}

// Convenience constructors for the codes used by the service.

func BadRequest(message string) *Error   { return New(CodeBadRequest, message) }
func Validation(message string) *Error   { return New(CodeValidation, message) }
func Unauthorized(message string) *Error { return New(CodeUnauthorized, message) }
func Forbidden(message string) *Error    { return New(CodeForbidden, message) }
func NotFound(message string) *Error     { return New(CodeNotFound, message) }
func Conflict(message string) *Error     { return New(CodeConflict, message) }
func RateLimited(message string) *Error  { return New(CodeRateLimited, message) }
func Internal(message string) *Error     { return New(CodeInternal, message) }
func Unavailable(message string) *Error  { return New(CodeUnavailable, message) }

// statusForCode maps an application code onto an HTTP status code.
func statusForCode(code int) int {
	switch code {
	case CodeOK:
		return http.StatusOK
	case CodeValidation:
		return http.StatusUnprocessableEntity
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodePayloadTooLarge:
		return http.StatusRequestEntityTooLarge
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}

// FromError normalises any error into an *Error so handlers can treat all
// failures uniformly. Unknown failures become an opaque internal error: the
// original message is logged by the middleware but never leaked to clients.
func FromError(err error) *Error {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil // client went away or timed out; nothing to report
	}
	return Wrap(CodeInternal, "Internal server error", err)
}
