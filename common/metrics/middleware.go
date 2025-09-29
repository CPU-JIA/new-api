package metrics

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
)

// responseWriter wraps gin.ResponseWriter to capture response size
type responseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	rw.body.Write(data)
	return rw.ResponseWriter.Write(data)
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// PrometheusMiddleware creates a middleware for collecting HTTP metrics
func PrometheusMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		startTime := time.Now()

		// Get request size
		var requestSize int64
		if c.Request.Body != nil {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				requestSize = int64(len(body))
				// Restore the body for the next handler
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		// Wrap the response writer to capture response size
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     200, // Default status code
		}
		c.Writer = rw

		// Increment active connections
		metrics := GetMetrics()
		metrics.HTTPActiveConnections.Inc()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(startTime)
		method := c.Request.Method
		path := getRoutePath(c)
		statusCode := rw.statusCode
		responseSize := int64(rw.body.Len())

		metrics.RecordHTTPRequest(method, path, statusCode, duration, requestSize, responseSize)

		// Decrement active connections
		metrics.HTTPActiveConnections.Dec()
	})
}

// getRoutePath extracts the route pattern from gin context
// Returns the matched route pattern instead of the actual path
func getRoutePath(c *gin.Context) string {
	// Try to get the route pattern from gin
	if route := c.FullPath(); route != "" {
		return route
	}
	// Fallback to request path
	return c.Request.URL.Path
}

// RelayMetricsWrapper wraps relay operations with metrics
type RelayMetricsWrapper struct {
	Provider  string
	Model     string
	ChannelID string
	startTime time.Time
}

// NewRelayMetricsWrapper creates a new relay metrics wrapper
func NewRelayMetricsWrapper(provider, model, channelID string) *RelayMetricsWrapper {
	wrapper := &RelayMetricsWrapper{
		Provider:  provider,
		Model:     model,
		ChannelID: channelID,
		startTime: time.Now(),
	}

	// Increment active requests
	GetMetrics().IncrementActiveRequests()

	return wrapper
}

// Success records a successful relay request
func (r *RelayMetricsWrapper) Success() {
	duration := time.Since(r.startTime)
	GetMetrics().RecordRelayRequest(r.Provider, r.Model, r.ChannelID, "success", duration)
	GetMetrics().DecrementActiveRequests()
}

// Error records a failed relay request
func (r *RelayMetricsWrapper) Error(errorType string) {
	duration := time.Since(r.startTime)
	GetMetrics().RecordRelayRequest(r.Provider, r.Model, r.ChannelID, "error", duration)
	GetMetrics().RecordRelayError(r.Provider, r.Model, r.ChannelID, errorType)
	GetMetrics().DecrementActiveRequests()
}

// RecordTokenUsage records token usage for this request
func (r *RelayMetricsWrapper) RecordTokenUsage(tokenType string, count int) {
	GetMetrics().RecordTokenUsage(r.Provider, r.Model, r.ChannelID, tokenType, count)
}

// AuthMetricsMiddleware creates a middleware for collecting auth metrics
func AuthMetricsMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Process request
		c.Next()

		// Record auth metrics based on the response
		if c.Request.URL.Path == "/api/auth/login" {
			status := "success"
			if c.Writer.Status() >= 400 {
				status = "failure"
			}
			GetMetrics().RecordAuthAttempt("password", status)
		}
	})
}

// MetricsConfig holds configuration for metrics collection
type MetricsConfig struct {
	Enabled           bool
	IncludePath       bool
	SkipPaths         []string
	GroupedStatusCode bool
}

// DefaultMetricsConfig returns default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:           true,
		IncludePath:       true,
		SkipPaths:         []string{"/metrics", "/health", "/ping"},
		GroupedStatusCode: true,
	}
}

// ConfigurablePrometheusMiddleware creates a configurable metrics middleware
func ConfigurablePrometheusMiddleware(config *MetricsConfig) gin.HandlerFunc {
	if !config.Enabled {
		return gin.HandlerFunc(func(c *gin.Context) {
			c.Next()
		})
	}

	return gin.HandlerFunc(func(c *gin.Context) {
		// Skip metrics collection for certain paths
		for _, skipPath := range config.SkipPaths {
			if c.Request.URL.Path == skipPath {
				c.Next()
				return
			}
		}

		startTime := time.Now()

		// Get request size
		var requestSize int64
		if c.Request.Body != nil {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				requestSize = int64(len(body))
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
			}
		}

		// Wrap response writer
		rw := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			statusCode:     200,
		}
		c.Writer = rw

		metrics := GetMetrics()
		metrics.HTTPActiveConnections.Inc()

		c.Next()

		// Record metrics
		duration := time.Since(startTime)
		method := c.Request.Method

		var path string
		if config.IncludePath {
			path = getRoutePath(c)
		} else {
			path = "/"
		}

		statusCode := rw.statusCode
		if config.GroupedStatusCode {
			// Group status codes (2xx, 3xx, 4xx, 5xx)
			statusCode = (statusCode / 100) * 100
		}

		responseSize := int64(rw.body.Len())

		metrics.RecordHTTPRequest(method, path, statusCode, duration, requestSize, responseSize)
		metrics.HTTPActiveConnections.Dec()
	})
}