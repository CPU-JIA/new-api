package errors

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		error          error
		expectedStatus int
		expectedType   string
		expectedCode   string
	}{
		{
			name:           "AppError",
			error:          ValidationError("invalid_input", "Invalid input provided"),
			expectedStatus: http.StatusBadRequest,
			expectedType:   string(ErrorTypeValidation),
			expectedCode:   "invalid_input",
		},
		{
			name:           "Standard error",
			error:          errors.New("something went wrong"),
			expectedStatus: http.StatusInternalServerError,
			expectedType:   string(ErrorTypeInternal),
			expectedCode:   ErrCodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			HandleError(c, tt.error)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedType, response.Error.Type)
			assert.Equal(t, tt.expectedCode, response.Error.Code)
			assert.NotEmpty(t, response.Error.Message)
		})
	}
}

func TestAbortWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	appErr := ValidationError("test_error", "Test message")
	AbortWithError(c, appErr)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.True(t, c.IsAborted())

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, string(ErrorTypeValidation), response.Error.Type)
	assert.Equal(t, "test_error", response.Error.Code)
	assert.Equal(t, "Test message", response.Error.Message)
}

func TestAbortWithValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	AbortWithValidationError(c, "invalid_field", "Field is invalid")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.True(t, c.IsAborted())

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, string(ErrorTypeValidation), response.Error.Type)
	assert.Equal(t, "invalid_field", response.Error.Code)
	assert.Equal(t, "Field is invalid", response.Error.Message)
}

func TestAbortWithAuthorizationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	AbortWithAuthorizationError(c, "unauthorized", "User not authorized")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.True(t, c.IsAborted())

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, string(ErrorTypeAuthorization), response.Error.Type)
	assert.Equal(t, "unauthorized", response.Error.Code)
	assert.Equal(t, "User not authorized", response.Error.Message)
}

func TestAbortWithNotFoundError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	AbortWithNotFoundError(c, "resource_not_found", "Resource not found")

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.True(t, c.IsAborted())

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, string(ErrorTypeNotFound), response.Error.Type)
	assert.Equal(t, "resource_not_found", response.Error.Code)
	assert.Equal(t, "Resource not found", response.Error.Message)
}

func TestAbortWithInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	originalErr := errors.New("database connection failed")
	AbortWithInternalError(c, originalErr, "db_error", "Database error occurred")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.True(t, c.IsAborted())

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, string(ErrorTypeInternal), response.Error.Type)
	assert.Equal(t, "db_error", response.Error.Code)
	assert.Equal(t, "Database error occurred", response.Error.Message)
}

func TestAsAppError(t *testing.T) {
	appErr := ValidationError("test_error", "Test message")
	standardErr := errors.New("standard error")

	var target *AppError

	// Test with AppError
	result := AsAppError(appErr, &target)
	assert.True(t, result)
	assert.Equal(t, appErr, target)

	// Test with standard error
	target = nil
	result = AsAppError(standardErr, &target)
	assert.False(t, result)
	assert.Nil(t, target)
}

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestHeader  string
		expectGenerate bool
	}{
		{
			name:           "With existing request ID",
			requestHeader:  "test-request-id",
			expectGenerate: false,
		},
		{
			name:           "Without request ID",
			requestHeader:  "",
			expectGenerate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Set up request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.requestHeader != "" {
				req.Header.Set("X-Request-ID", tt.requestHeader)
			}
			c.Request = req

			// Execute middleware
			middleware := RequestIDMiddleware()
			middleware(c)

			// Check context
			requestID, exists := c.Get("request_id")
			assert.True(t, exists)

			if tt.expectGenerate {
				assert.NotEmpty(t, requestID)
				assert.NotEqual(t, tt.requestHeader, requestID)
			} else {
				assert.Equal(t, tt.requestHeader, requestID)
			}

			// Check response header
			assert.Equal(t, requestID, w.Header().Get("X-Request-ID"))
		})
	}
}

func TestErrorHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		panicValue     interface{}
		expectedStatus int
	}{
		{
			name:           "Panic with error",
			panicValue:     ValidationError("test_error", "Test message"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Panic with standard error",
			panicValue:     errors.New("something went wrong"),
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Panic with non-error value",
			panicValue:     "string panic",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, router := gin.CreateTestContext(w)

			// Add error handler middleware
			router.Use(ErrorHandler())

			// Add a route that panics
			router.GET("/test", func(c *gin.Context) {
				panic(tt.panicValue)
			})

			// Make request
			req := httptest.NewRequest("GET", "/test", nil)
			c.Request = req
			router.HandleContext(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.NotEmpty(t, response.Error.Type)
			assert.NotEmpty(t, response.Error.Code)
			assert.NotEmpty(t, response.Error.Message)
		})
	}
}

func TestGetRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		setValue   interface{}
		expectedID string
	}{
		{
			name:       "Valid string request ID",
			setValue:   "test-request-id",
			expectedID: "test-request-id",
		},
		{
			name:       "Non-string value",
			setValue:   123,
			expectedID: "",
		},
		{
			name:       "No value set",
			setValue:   nil,
			expectedID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			if tt.setValue != nil {
				c.Set("request_id", tt.setValue)
			}

			result := getRequestID(c)
			assert.Equal(t, tt.expectedID, result)
		})
	}
}