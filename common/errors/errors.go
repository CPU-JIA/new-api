package errors

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeAuthorization ErrorType = "authorization"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeExternal     ErrorType = "external"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeTimeout      ErrorType = "timeout"
	ErrorTypeUnavailable  ErrorType = "unavailable"
)

// AppError represents a structured application error
type AppError struct {
	Type       ErrorType `json:"type"`
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	StatusCode int       `json:"status_code"`
	LocalError bool      `json:"local_error"`
	Err        error     `json:"-"` // Don't expose internal error in JSON
	StackTrace string    `json:"stack_trace,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%s)", e.Message, e.Err.Error(), e.Code)
	}
	return fmt.Sprintf("%s (%s)", e.Message, e.Code)
}

// Unwrap implements the errors.Unwrap interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is implements the errors.Is interface
func (e *AppError) Is(target error) bool {
	var appErr *AppError
	if errors.As(target, &appErr) {
		return e.Code == appErr.Code
	}
	return false
}

// WithStackTrace adds stack trace to the error
func (e *AppError) WithStackTrace() *AppError {
	if e.StackTrace == "" {
		e.StackTrace = getStackTrace()
	}
	return e
}

// WithDetails adds additional details to the error
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// NewAppError creates a new application error
func NewAppError(errType ErrorType, code, message string, statusCode int, localError bool) *AppError {
	return &AppError{
		Type:       errType,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		LocalError: localError,
	}
}

// Wrap wraps an existing error with application error context
func Wrap(err error, errType ErrorType, code, message string, statusCode int, localError bool) *AppError {
	return &AppError{
		Type:       errType,
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		LocalError: localError,
		Err:        err,
	}
}

// getStackTrace captures the current stack trace
func getStackTrace() string {
	buf := make([]byte, 2048)
	runtime.Stack(buf, false)
	return string(buf)
}

// Common error constructors

// ValidationError creates a validation error
func ValidationError(code, message string) *AppError {
	return NewAppError(ErrorTypeValidation, code, message, http.StatusBadRequest, true)
}

// AuthorizationError creates an authorization error
func AuthorizationError(code, message string) *AppError {
	return NewAppError(ErrorTypeAuthorization, code, message, http.StatusUnauthorized, true)
}

// NotFoundError creates a not found error
func NotFoundError(code, message string) *AppError {
	return NewAppError(ErrorTypeNotFound, code, message, http.StatusNotFound, true)
}

// ConflictError creates a conflict error
func ConflictError(code, message string) *AppError {
	return NewAppError(ErrorTypeConflict, code, message, http.StatusConflict, true)
}

// InternalError creates an internal server error
func InternalError(code, message string) *AppError {
	return NewAppError(ErrorTypeInternal, code, message, http.StatusInternalServerError, true).WithStackTrace()
}

// ExternalError creates an external service error
func ExternalError(code, message string) *AppError {
	return NewAppError(ErrorTypeExternal, code, message, http.StatusBadGateway, false)
}

// RateLimitError creates a rate limit error
func RateLimitError(code, message string) *AppError {
	return NewAppError(ErrorTypeRateLimit, code, message, http.StatusTooManyRequests, true)
}

// TimeoutError creates a timeout error
func TimeoutError(code, message string) *AppError {
	return NewAppError(ErrorTypeTimeout, code, message, http.StatusRequestTimeout, false)
}

// UnavailableError creates a service unavailable error
func UnavailableError(code, message string) *AppError {
	return NewAppError(ErrorTypeUnavailable, code, message, http.StatusServiceUnavailable, false)
}

// WrapValidation wraps an error as validation error
func WrapValidation(err error, code, message string) *AppError {
	return Wrap(err, ErrorTypeValidation, code, message, http.StatusBadRequest, true)
}

// WrapInternal wraps an error as internal error
func WrapInternal(err error, code, message string) *AppError {
	return Wrap(err, ErrorTypeInternal, code, message, http.StatusInternalServerError, true).WithStackTrace()
}

// WrapExternal wraps an error as external error
func WrapExternal(err error, code, message string) *AppError {
	return Wrap(err, ErrorTypeExternal, code, message, http.StatusBadGateway, false)
}

// Error code constants
const (
	// Validation errors
	ErrCodeInvalidRequest    = "invalid_request"
	ErrCodeInvalidParameter  = "invalid_parameter"
	ErrCodeMissingParameter  = "missing_parameter"
	ErrCodeInvalidFormat     = "invalid_format"

	// Authorization errors
	ErrCodeUnauthorized      = "unauthorized"
	ErrCodeForbidden         = "forbidden"
	ErrCodeInvalidToken      = "invalid_token"
	ErrCodeExpiredToken      = "expired_token"

	// Resource errors
	ErrCodeNotFound          = "not_found"
	ErrCodeAlreadyExists     = "already_exists"
	ErrCodeConflict          = "conflict"

	// System errors
	ErrCodeInternalError     = "internal_error"
	ErrCodeDatabaseError     = "database_error"
	ErrCodeExternalService   = "external_service_error"
	ErrCodeTimeout           = "timeout"
	ErrCodeUnavailable       = "service_unavailable"

	// Rate limiting
	ErrCodeRateLimit         = "rate_limit_exceeded"
	ErrCodeQuotaExceeded     = "quota_exceeded"

	// AI Provider specific
	ErrCodeModelNotFound     = "model_not_found"
	ErrCodeModelOverloaded   = "model_overloaded"
	ErrCodeInvalidAPIKey     = "invalid_api_key"
	ErrCodeInsufficientQuota = "insufficient_quota"
)