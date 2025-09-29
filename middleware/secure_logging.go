package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"one-api/common"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SecureLoggingConfig holds configuration for secure logging middleware
type SecureLoggingConfig struct {
	// Request logging
	LogRequests         bool     // Log all requests
	LogRequestBody      bool     // Log request body (with masking)
	LogResponseBody     bool     // Log response body (with masking)
	MaxBodySize         int      // Maximum body size to log (in bytes)

	// Sensitive field masking
	SensitiveHeaders    []string // Headers to mask (in addition to defaults)
	SensitiveParams     []string // Query parameters to mask
	SensitiveJSONFields []string // JSON fields to mask in request/response bodies

	// Filtering
	SkipPaths           []string // Paths to skip logging
	SkipMethods         []string // HTTP methods to skip

	// Performance
	AsyncLogging        bool     // Use async logging for performance
}

// DefaultSecureLoggingConfig returns secure default configuration
func DefaultSecureLoggingConfig() *SecureLoggingConfig {
	return &SecureLoggingConfig{
		LogRequests:      true,
		LogRequestBody:   true,
		LogResponseBody:  false, // Can be verbose
		MaxBodySize:      8192,  // 8KB
		SensitiveHeaders: []string{
			"authorization", "x-api-key", "x-auth-token", "cookie",
			"x-forwarded-for", "x-real-ip", "user-agent",
		},
		SensitiveParams: []string{
			"key", "token", "password", "secret", "api_key",
		},
		SensitiveJSONFields: []string{
			"key", "token", "password", "secret", "api_key", "authorization",
			"openai_organization", "base_url",
		},
		SkipPaths: []string{
			"/health", "/metrics", "/ping", "/status",
		},
		SkipMethods: []string{"OPTIONS"},
		AsyncLogging: true,
	}
}

// responseWriter wraps gin.ResponseWriter to capture response body
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseWriter) Write(data []byte) (int, error) {
	// Write to both original writer and capture buffer
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// SecureLoggingMiddleware creates a middleware that logs requests with automatic sensitive data masking
func SecureLoggingMiddleware(config *SecureLoggingConfig) gin.HandlerFunc {
	if config == nil {
		config = DefaultSecureLoggingConfig()
	}

	return func(c *gin.Context) {
		// Skip if logging disabled or path/method should be skipped
		if !config.LogRequests || shouldSkip(c, config) {
			c.Next()
			return
		}

		start := time.Now()

		// Prepare request logging data
		requestData := extractRequestData(c, config)

		// Wrap response writer if response body logging is enabled
		var respWriter *responseWriter
		if config.LogResponseBody {
			respWriter = &responseWriter{
				ResponseWriter: c.Writer,
				body:           &bytes.Buffer{},
			}
			c.Writer = respWriter
		}

		// Process request
		c.Next()

		// Extract response data
		responseData := extractResponseData(c, respWriter, config)

		// Calculate duration
		duration := time.Since(start)
		responseData["duration_ms"] = duration.Milliseconds()

		// Log the API call
		logAPICall(requestData, responseData, config)
	}
}

// shouldSkip checks if request should be skipped from logging
func shouldSkip(c *gin.Context, config *SecureLoggingConfig) bool {
	path := c.Request.URL.Path
	method := c.Request.Method

	// Check skip paths
	for _, skipPath := range config.SkipPaths {
		if strings.HasPrefix(path, skipPath) {
			return true
		}
	}

	// Check skip methods
	for _, skipMethod := range config.SkipMethods {
		if method == skipMethod {
			return true
		}
	}

	return false
}

// extractRequestData extracts and masks sensitive request data
func extractRequestData(c *gin.Context, config *SecureLoggingConfig) map[string]interface{} {
	data := map[string]interface{}{
		"method":     c.Request.Method,
		"path":       c.Request.URL.Path,
		"remote_ip":  c.ClientIP(),
		"user_agent": maskHeader(c.GetHeader("User-Agent")),
		"timestamp":  time.Now().Unix(),
	}

	// Add query parameters (masked)
	if len(c.Request.URL.RawQuery) > 0 {
		data["query_params"] = maskQueryParams(c.Request.URL.Query(), config.SensitiveParams)
	}

	// Add headers (masked)
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			if isSensitiveHeader(key, config.SensitiveHeaders) {
				headers[key] = "****"
			} else {
				headers[key] = maskHeader(values[0])
			}
		}
	}
	data["headers"] = headers

	// Add request body if configured
	if config.LogRequestBody && c.Request.Body != nil {
		if bodyBytes, err := io.ReadAll(c.Request.Body); err == nil {
			if len(bodyBytes) > 0 && len(bodyBytes) <= config.MaxBodySize {
				// Restore body for further processing
				c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

				// Try to parse and mask JSON body
				if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
					if maskedBody := maskJSONBody(bodyBytes, config.SensitiveJSONFields); maskedBody != nil {
						data["request_body"] = maskedBody
					}
				} else {
					// For non-JSON, just include masked string
					data["request_body_text"] = common.MaskLogMessageGlobal(string(bodyBytes))
				}
			}
		}
	}

	return data
}

// extractResponseData extracts and masks sensitive response data
func extractResponseData(c *gin.Context, rw *responseWriter, config *SecureLoggingConfig) map[string]interface{} {
	data := map[string]interface{}{
		"status_code": c.Writer.Status(),
		"size_bytes":  c.Writer.Size(),
	}

	// Add response headers (masked)
	headers := make(map[string]string)
	for key, values := range c.Writer.Header() {
		if len(values) > 0 {
			if isSensitiveHeader(key, config.SensitiveHeaders) {
				headers[key] = "****"
			} else {
				headers[key] = maskHeader(values[0])
			}
		}
	}
	data["headers"] = headers

	// Add response body if configured and captured
	if config.LogResponseBody && rw != nil && rw.body.Len() > 0 {
		bodyBytes := rw.body.Bytes()
		if len(bodyBytes) <= config.MaxBodySize {
			// Try to parse and mask JSON response
			if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
				if maskedBody := maskJSONBody(bodyBytes, config.SensitiveJSONFields); maskedBody != nil {
					data["response_body"] = maskedBody
				}
			} else {
				// For non-JSON, just include masked string
				data["response_body_text"] = common.MaskLogMessageGlobal(string(bodyBytes))
			}
		}
	}

	return data
}

// maskQueryParams masks sensitive query parameters
func maskQueryParams(params map[string][]string, sensitiveParams []string) map[string][]string {
	masked := make(map[string][]string)

	for key, values := range params {
		if isSensitiveParam(key, sensitiveParams) {
			maskedValues := make([]string, len(values))
			for i := range values {
				maskedValues[i] = "****"
			}
			masked[key] = maskedValues
		} else {
			// Still mask using global masker for pattern detection
			maskedValues := make([]string, len(values))
			for i, value := range values {
				maskedValues[i] = common.MaskLogMessageGlobal(value)
			}
			masked[key] = maskedValues
		}
	}

	return masked
}

// maskJSONBody attempts to parse JSON and mask sensitive fields
func maskJSONBody(bodyBytes []byte, sensitiveFields []string) interface{} {
	var parsed interface{}
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		// If not valid JSON, return masked string
		return common.MaskLogMessageGlobal(string(bodyBytes))
	}

	// Apply masking using global masker
	masked := common.MaskJSONGlobal(parsed)
	return masked
}

// isSensitiveHeader checks if a header is considered sensitive
func isSensitiveHeader(header string, additionalSensitive []string) bool {
	header = strings.ToLower(header)

	// Default sensitive headers
	defaultSensitive := []string{
		"authorization", "x-api-key", "x-auth-token", "cookie",
		"proxy-authorization", "x-forwarded-for", "x-real-ip",
	}

	// Check default sensitive headers
	for _, sensitive := range defaultSensitive {
		if header == sensitive {
			return true
		}
	}

	// Check additional sensitive headers
	for _, sensitive := range additionalSensitive {
		if header == strings.ToLower(sensitive) {
			return true
		}
	}

	return false
}

// isSensitiveParam checks if a query parameter is considered sensitive
func isSensitiveParam(param string, additionalSensitive []string) bool {
	param = strings.ToLower(param)

	// Default sensitive parameters
	defaultSensitive := []string{
		"key", "token", "password", "secret", "api_key", "apikey",
		"access_token", "refresh_token", "auth", "authorization",
	}

	// Check default sensitive parameters
	for _, sensitive := range defaultSensitive {
		if param == sensitive {
			return true
		}
	}

	// Check additional sensitive parameters
	for _, sensitive := range additionalSensitive {
		if param == strings.ToLower(sensitive) {
			return true
		}
	}

	return false
}

// maskHeader applies basic masking to header values
func maskHeader(value string) string {
	return common.MaskLogMessageGlobal(value)
}

// logAPICall logs the API call using secure logger
func logAPICall(requestData, responseData map[string]interface{}, config *SecureLoggingConfig) {
	if !common.IsSecureLoggingEnabled() {
		// Fallback to system log if secure logger not available
		common.SysLogMasked("API call: " + requestData["method"].(string) + " " + requestData["path"].(string))
		return
	}

	logger := common.GetSecureLogger()
	if logger == nil {
		return
	}

	// Determine sensitive fields for this specific call
	sensitiveFields := append(config.SensitiveHeaders, config.SensitiveJSONFields...)
	sensitiveFields = append(sensitiveFields, config.SensitiveParams...)

	// Use secure logger's API call logging
	logger.LogAPICall(requestData, responseData, sensitiveFields)
}

// SecureRequestIDMiddleware adds secure request ID tracking
func SecureRequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateSecureRequestID()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)

		// Log request start if secure logging enabled
		if common.IsSecureLoggingEnabled() {
			logger := common.GetSecureLogger()
			if logger != nil {
				logger.LogInfo("request_started", map[string]interface{}{
					"request_id": requestID,
					"method":     c.Request.Method,
					"path":       c.Request.URL.Path,
					"client_ip":  c.ClientIP(),
				})
			}
		}

		c.Next()
	}
}

// generateSecureRequestID generates a secure, masked request ID
func generateSecureRequestID() string {
	// Use current timestamp + random component
	timestamp := time.Now().UnixNano()
	return "req_" + strconv.FormatInt(timestamp%100000000, 36) // Base36 for shorter ID
}