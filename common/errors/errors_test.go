package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appError *AppError
		expected string
	}{
		{
			name: "Error without underlying error",
			appError: &AppError{
				Code:    "test_error",
				Message: "Test message",
			},
			expected: "Test message (test_error)",
		},
		{
			name: "Error with underlying error",
			appError: &AppError{
				Code:    "test_error",
				Message: "Test message",
				Err:     errors.New("underlying error"),
			},
			expected: "Test message: underlying error (test_error)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.appError.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	appErr := &AppError{
		Code:    "test_error",
		Message: "Test message",
		Err:     underlying,
	}

	result := appErr.Unwrap()
	assert.Equal(t, underlying, result)

	// Test nil case
	appErrNil := &AppError{
		Code:    "test_error",
		Message: "Test message",
	}
	assert.Nil(t, appErrNil.Unwrap())
}

func TestAppError_Is(t *testing.T) {
	err1 := &AppError{Code: "test_error"}
	err2 := &AppError{Code: "test_error"}
	err3 := &AppError{Code: "different_error"}
	standardErr := errors.New("standard error")

	assert.True(t, err1.Is(err2))
	assert.False(t, err1.Is(err3))
	assert.False(t, err1.Is(standardErr))
}

func TestAppError_WithStackTrace(t *testing.T) {
	appErr := &AppError{
		Code:    "test_error",
		Message: "Test message",
	}

	result := appErr.WithStackTrace()
	assert.NotEmpty(t, result.StackTrace)
	assert.Same(t, appErr, result) // Should return same instance

	// Test that calling again doesn't overwrite
	originalTrace := result.StackTrace
	result2 := result.WithStackTrace()
	assert.Equal(t, originalTrace, result2.StackTrace)
}

func TestAppError_WithDetails(t *testing.T) {
	appErr := &AppError{
		Code:    "test_error",
		Message: "Test message",
	}

	details := "Additional details"
	result := appErr.WithDetails(details)
	assert.Equal(t, details, result.Details)
	assert.Same(t, appErr, result) // Should return same instance
}

func TestNewAppError(t *testing.T) {
	errType := ErrorTypeValidation
	code := "test_error"
	message := "Test message"
	statusCode := http.StatusBadRequest
	localError := true

	result := NewAppError(errType, code, message, statusCode, localError)

	assert.Equal(t, errType, result.Type)
	assert.Equal(t, code, result.Code)
	assert.Equal(t, message, result.Message)
	assert.Equal(t, statusCode, result.StatusCode)
	assert.Equal(t, localError, result.LocalError)
	assert.Nil(t, result.Err)
}

func TestWrap(t *testing.T) {
	underlying := errors.New("underlying error")
	errType := ErrorTypeInternal
	code := "test_error"
	message := "Test message"
	statusCode := http.StatusInternalServerError
	localError := true

	result := Wrap(underlying, errType, code, message, statusCode, localError)

	assert.Equal(t, errType, result.Type)
	assert.Equal(t, code, result.Code)
	assert.Equal(t, message, result.Message)
	assert.Equal(t, statusCode, result.StatusCode)
	assert.Equal(t, localError, result.LocalError)
	assert.Equal(t, underlying, result.Err)
}

func TestErrorConstructors(t *testing.T) {
	tests := []struct {
		name       string
		constructor func(string, string) *AppError
		errType    ErrorType
		statusCode int
		localError bool
	}{
		{
			name:        "ValidationError",
			constructor: ValidationError,
			errType:     ErrorTypeValidation,
			statusCode:  http.StatusBadRequest,
			localError:  true,
		},
		{
			name:        "AuthorizationError",
			constructor: AuthorizationError,
			errType:     ErrorTypeAuthorization,
			statusCode:  http.StatusUnauthorized,
			localError:  true,
		},
		{
			name:        "NotFoundError",
			constructor: NotFoundError,
			errType:     ErrorTypeNotFound,
			statusCode:  http.StatusNotFound,
			localError:  true,
		},
		{
			name:        "ConflictError",
			constructor: ConflictError,
			errType:     ErrorTypeConflict,
			statusCode:  http.StatusConflict,
			localError:  true,
		},
		{
			name:        "ExternalError",
			constructor: ExternalError,
			errType:     ErrorTypeExternal,
			statusCode:  http.StatusBadGateway,
			localError:  false,
		},
		{
			name:        "RateLimitError",
			constructor: RateLimitError,
			errType:     ErrorTypeRateLimit,
			statusCode:  http.StatusTooManyRequests,
			localError:  true,
		},
		{
			name:        "TimeoutError",
			constructor: TimeoutError,
			errType:     ErrorTypeTimeout,
			statusCode:  http.StatusRequestTimeout,
			localError:  false,
		},
		{
			name:        "UnavailableError",
			constructor: UnavailableError,
			errType:     ErrorTypeUnavailable,
			statusCode:  http.StatusServiceUnavailable,
			localError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := "test_code"
			message := "test message"

			result := tt.constructor(code, message)

			assert.Equal(t, tt.errType, result.Type)
			assert.Equal(t, code, result.Code)
			assert.Equal(t, message, result.Message)
			assert.Equal(t, tt.statusCode, result.StatusCode)
			assert.Equal(t, tt.localError, result.LocalError)
		})
	}
}

func TestInternalError(t *testing.T) {
	code := "test_code"
	message := "test message"

	result := InternalError(code, message)

	assert.Equal(t, ErrorTypeInternal, result.Type)
	assert.Equal(t, code, result.Code)
	assert.Equal(t, message, result.Message)
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
	assert.True(t, result.LocalError)
	assert.NotEmpty(t, result.StackTrace) // Should have stack trace
}

func TestWrapFunctions(t *testing.T) {
	underlying := errors.New("underlying error")

	tests := []struct {
		name       string
		wrapper    func(error, string, string) *AppError
		errType    ErrorType
		statusCode int
		localError bool
	}{
		{
			name:       "WrapValidation",
			wrapper:    WrapValidation,
			errType:    ErrorTypeValidation,
			statusCode: http.StatusBadRequest,
			localError: true,
		},
		{
			name:       "WrapExternal",
			wrapper:    WrapExternal,
			errType:    ErrorTypeExternal,
			statusCode: http.StatusBadGateway,
			localError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := "test_code"
			message := "test message"

			result := tt.wrapper(underlying, code, message)

			assert.Equal(t, tt.errType, result.Type)
			assert.Equal(t, code, result.Code)
			assert.Equal(t, message, result.Message)
			assert.Equal(t, tt.statusCode, result.StatusCode)
			assert.Equal(t, tt.localError, result.LocalError)
			assert.Equal(t, underlying, result.Err)
		})
	}
}

func TestWrapInternal(t *testing.T) {
	underlying := errors.New("underlying error")
	code := "test_code"
	message := "test message"

	result := WrapInternal(underlying, code, message)

	assert.Equal(t, ErrorTypeInternal, result.Type)
	assert.Equal(t, code, result.Code)
	assert.Equal(t, message, result.Message)
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
	assert.True(t, result.LocalError)
	assert.Equal(t, underlying, result.Err)
	assert.NotEmpty(t, result.StackTrace) // Should have stack trace
}

func TestErrorConstants(t *testing.T) {
	// Test that constants are defined correctly
	assert.Equal(t, "invalid_request", ErrCodeInvalidRequest)
	assert.Equal(t, "unauthorized", ErrCodeUnauthorized)
	assert.Equal(t, "not_found", ErrCodeNotFound)
	assert.Equal(t, "internal_error", ErrCodeInternalError)
	assert.Equal(t, "rate_limit_exceeded", ErrCodeRateLimit)
	assert.Equal(t, "model_not_found", ErrCodeModelNotFound)
}

func TestErrorTypes(t *testing.T) {
	// Test that error types are defined correctly
	assert.Equal(t, ErrorType("validation"), ErrorTypeValidation)
	assert.Equal(t, ErrorType("authorization"), ErrorTypeAuthorization)
	assert.Equal(t, ErrorType("not_found"), ErrorTypeNotFound)
	assert.Equal(t, ErrorType("internal"), ErrorTypeInternal)
	assert.Equal(t, ErrorType("external"), ErrorTypeExternal)
}

func ExampleAppError() {
	// Create a validation error
	err := ValidationError(ErrCodeInvalidRequest, "The request is invalid")

	fmt.Println("Error:", err.Error())
	fmt.Println("Type:", err.Type)
	fmt.Println("Status Code:", err.StatusCode)

	// Wrap an existing error
	originalErr := errors.New("database connection failed")
	wrappedErr := WrapInternal(originalErr, ErrCodeDatabaseError, "Failed to connect to database")

	fmt.Println("Wrapped Error:", wrappedErr.Error())
	fmt.Println("Unwrapped:", errors.Unwrap(wrappedErr))
}

func ExampleAppError_WithDetails() {
	err := ValidationError(ErrCodeInvalidParameter, "Invalid user ID")
	err.WithDetails("User ID must be a positive integer")

	fmt.Println("Error:", err.Error())
	fmt.Println("Details:", err.Details)
}