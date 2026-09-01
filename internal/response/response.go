// Package response writes the JSON envelope shared by every endpoint of the
// service. The shape is dictated by the vue-h5-template frontend client:
//
//	{"code": 0, "message": "ok", "data": {...}, "error": null, "requestId": "..."}
//
// The frontend treats code == 0 as success and any other value as a business
// failure, so the envelope (not the HTTP status alone) is what callers branch on.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/fonghehe/vue-h5-template-business-service/internal/apierr"
)

// Context keys shared by the middleware chain and the response writer. They
// live here (rather than in the middleware package) so that transport helpers
// can read them without creating an import cycle.
const (
	ContextKeyRequestID = "requestID"
	ContextKeyUserID    = "userID"
	ContextKeyUsername  = "username"
	ContextKeyRoles     = "roles"
)

// Envelope is the single response shape returned by every endpoint.
type Envelope struct {
	// Code is the application status code; 0 means success.
	Code int `json:"code"`
	// Message is a human readable, user-safe description.
	Message string `json:"message"`
	// Data holds the payload on success and null on failure.
	Data any `json:"data"`
	// Error carries structured, user-safe error details; null on success.
	Error any `json:"error"`
	// RequestID correlates the response with the access log entry.
	RequestID string `json:"requestId,omitempty"`
}

// OK writes a successful envelope.
func OK(c *gin.Context, data any) {
	write(c, http.StatusOK, apierr.CodeOK, "ok", data, nil)
}

// Created writes a successful envelope with HTTP 201.
func Created(c *gin.Context, data any) {
	write(c, http.StatusCreated, apierr.CodeOK, "created", data, nil)
}

// Fail writes an error envelope derived from err.
// Unknown errors are reported as an opaque internal failure.
func Fail(c *gin.Context, err error) {
	apiErr := apierr.FromError(err)
	if apiErr == nil {
		// The client disconnected before the response was written.
		c.Abort()
		return
	}
	write(c, apiErr.Status, apiErr.Code, apiErr.Message, nil, apiErr.Details)
}

func write(c *gin.Context, status, code int, message string, data, details any) {
	c.JSON(status, Envelope{
		Code:      code,
		Message:   message,
		Data:      data,
		Error:     details,
		RequestID: c.GetString(ContextKeyRequestID),
	})
}
