package errors

import (
	"log"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents the standard error response format
type ErrorResponse struct {
	Error ErrorInfo `json:"error"`
}

// ErrorInfo contains detailed error information
type ErrorInfo struct {
	Type       string `json:"type"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

// ErrorHandler is a Gin middleware for handling errors
func ErrorHandler() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(error); ok {
			HandleError(c, err)
		} else {
			// Handle non-error panics
			HandleError(c, InternalError(ErrCodeInternalError, "Internal server error"))
		}
	})
}

// HandleError handles an error and sends appropriate HTTP response
func HandleError(c *gin.Context, err error) {
	var appErr *AppError

	// Try to convert to AppError
	if !AsAppError(err, &appErr) {
		// Convert standard error to internal AppError
		appErr = WrapInternal(err, ErrCodeInternalError, "Internal server error")
	}

	// Log error if it's internal or has stack trace
	if appErr.Type == ErrorTypeInternal || appErr.StackTrace != "" {
		log.Printf("Internal error: %s\nStack trace: %s", appErr.Error(), appErr.StackTrace)
	}

	// Get request ID if available
	requestID := getRequestID(c)

	// Create response
	response := ErrorResponse{
		Error: ErrorInfo{
			Type:      string(appErr.Type),
			Code:      appErr.Code,
			Message:   appErr.Message,
			Details:   appErr.Details,
			RequestID: requestID,
		},
	}

	// Set status code and send response
	c.JSON(appErr.StatusCode, response)
	c.Abort()
}

// AbortWithError aborts the request with an AppError
func AbortWithError(c *gin.Context, err *AppError) {
	HandleError(c, err)
}

// AbortWithValidationError is a convenience function for validation errors
func AbortWithValidationError(c *gin.Context, code, message string) {
	AbortWithError(c, ValidationError(code, message))
}

// AbortWithAuthorizationError is a convenience function for authorization errors
func AbortWithAuthorizationError(c *gin.Context, code, message string) {
	AbortWithError(c, AuthorizationError(code, message))
}

// AbortWithNotFoundError is a convenience function for not found errors
func AbortWithNotFoundError(c *gin.Context, code, message string) {
	AbortWithError(c, NotFoundError(code, message))
}

// AbortWithInternalError is a convenience function for internal errors
func AbortWithInternalError(c *gin.Context, err error, code, message string) {
	AbortWithError(c, WrapInternal(err, code, message))
}

// AsAppError checks if an error is an AppError and converts it
func AsAppError(err error, target **AppError) bool {
	if appErr, ok := err.(*AppError); ok {
		*target = appErr
		return true
	}
	return false
}

// getRequestID extracts request ID from context
func getRequestID(c *gin.Context) string {
	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// RequestIDMiddleware adds a request ID to the context
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a simple request ID (in production, use a proper UUID)
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// generateRequestID generates a simple request ID
// In production, consider using github.com/google/uuid
func generateRequestID() string {
	// Simple implementation - use timestamp + random suffix
	// In real implementation, use proper UUID generation
	return "req_" + "placeholder" // Placeholder for now
}