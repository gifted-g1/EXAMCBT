// Package httpx holds the API's response envelope and machine-readable error
// codes shared by middleware and handlers.
package httpx

import (
	"context"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error codes the frontend switches on.
const (
	CodeLANRequired      = "LAN_REQUIRED"
	CodeExamNotOpen      = "EXAM_NOT_OPEN"
	CodePINRequired      = "PIN_REQUIRED"
	CodePINInvalid       = "PIN_INVALID"
	CodeDeviceInUse      = "DEVICE_IN_USE"
	CodeStudentElsewhere = "STUDENT_ACTIVE_ELSEWHERE"
	CodeDeviceMismatch   = "DEVICE_MISMATCH"
	CodeAlreadySubmitted = "ALREADY_SUBMITTED"
	CodeNotOnRoster      = "NOT_ON_ROSTER"
	CodeFaceMismatch     = "FACE_MISMATCH"
	CodeNoFace           = "NO_FACE_DETECTED"
	CodeConsentRequired  = "CONSENT_REQUIRED"
	CodeSessionEnded     = "SESSION_ENDED"
	CodeSessionLost      = "SESSION_LOCK_LOST"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeInvalidCreds     = "INVALID_CREDENTIALS"
	CodeRateLimited      = "RATE_LIMITED"
	CodeValidation       = "VALIDATION_FAILED"
	CodeNotFound         = "NOT_FOUND"
	CodeConflict         = "CONFLICT"
	CodeInternal         = "INTERNAL_ERROR"
)

// Error is the body of every non-2xx response.
type Error struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Details any    `json:"details,omitempty"`
}

// Abort stops the chain with a JSON error.
func Abort(c *gin.Context, status int, code, msg string) {
	c.Set(KeyErrCode, code)
	c.AbortWithStatusJSON(status, Error{Error: code, Message: msg})
}

// AbortDetails is Abort with structured details (e.g. field errors).
func AbortDetails(c *gin.Context, status int, code, msg string, details any) {
	c.Set(KeyErrCode, code)
	c.AbortWithStatusJSON(status, Error{Error: code, Message: msg, Details: details})
}

// Internal logs nothing itself (the request logger records the status) and
// never leaks err to the client.
func Internal(c *gin.Context, err error) {
	_ = c.Error(err)
	Abort(c, http.StatusInternalServerError, CodeInternal, "unexpected server error")
}

func BadRequest(c *gin.Context, msg string) {
	Abort(c, http.StatusBadRequest, CodeValidation, msg)
}

func NotFound(c *gin.Context, what string) {
	Abort(c, http.StatusNotFound, CodeNotFound, what+" not found")
}

// Context keys set by middleware.
const (
	KeyClientIP = "es.client_ip"
	KeyUser     = "es.user"
	KeyStudent  = "es.student"
	KeyExam     = "es.exam"
	KeyErrCode  = "es.err_code"
)

type clientIPKey struct{}

// WithClientIP stores the gate-normalised client IP in a request context so
// services can apply per-exam subnet rules without depending on Gin.
func WithClientIP(ctx context.Context, ip net.IP) context.Context {
	return context.WithValue(ctx, clientIPKey{}, ip)
}

// ClientIPFrom returns the IP stored by WithClientIP (nil if absent).
func ClientIPFrom(ctx context.Context) net.IP {
	ip, _ := ctx.Value(clientIPKey{}).(net.IP)
	return ip
}
