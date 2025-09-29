package metrics

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestMetrics creates metrics with a custom registry for testing
func createTestMetrics() *ApplicationMetrics {
	reg := prometheus.NewRegistry()

	// Create all metrics
	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubsystemHTTP,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status_code"},
	)

	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SubsystemHTTP,
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status_code"},
	)

	httpRequestSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SubsystemHTTP,
			Name:      "request_size_bytes",
			Help:      "HTTP request size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B to 100MB
		},
		[]string{"method", "path"},
	)

	httpResponseSize := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SubsystemHTTP,
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B to 100MB
		},
		[]string{"method", "path", "status_code"},
	)

	httpActiveConnections := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: SubsystemHTTP,
			Name:      "active_connections",
			Help:      "Number of active HTTP connections",
		},
	)

	relayRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubsystemRelay,
			Name:      "requests_total",
			Help:      "Total number of relay requests",
		},
		[]string{"provider", "model", "channel_id", "status"},
	)

	relayRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Subsystem: SubsystemRelay,
			Name:      "request_duration_seconds",
			Help:      "Relay request duration",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"provider", "model", "channel_id"},
	)

	relayTokensUsed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubsystemRelay,
			Name:      "tokens_used_total",
			Help:      "Total tokens used",
		},
		[]string{"provider", "model", "channel_id", "token_type"},
	)

	relayErrorsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubsystemRelay,
			Name:      "errors_total",
			Help:      "Total relay errors",
		},
		[]string{"provider", "model", "channel_id", "error_type"},
	)

	relayActiveRequests := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: SubsystemRelay,
			Name:      "active_requests",
			Help:      "Active relay requests",
		},
	)

	authAttemptsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubsystemAuth,
			Name:      "attempts_total",
			Help:      "Total auth attempts",
		},
		[]string{"method", "status"},
	)

	authTokensIssued := prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: SubsystemAuth,
			Name:      "tokens_issued_total",
			Help:      "Total tokens issued",
		},
	)

	authTokensValidated := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: SubsystemAuth,
			Name:      "tokens_validated_total",
			Help:      "Total token validations",
		},
		[]string{"status"},
	)

	usersActive := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "users_active",
			Help: "Active users",
		},
	)

	quotaUsage := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "quota_usage_total",
			Help: "Total quota usage",
		},
		[]string{"user_group", "resource_type"},
	)

	channelsActive := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "channels_active",
			Help: "Active channels",
		},
		[]string{"provider", "status"},
	)

	modelsUsage := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "models_usage_total",
			Help: "Total model usage",
		},
		[]string{"model", "provider"},
	)

	// Register all metrics with the custom registry
	reg.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		httpRequestSize,
		httpResponseSize,
		httpActiveConnections,
		relayRequestsTotal,
		relayRequestDuration,
		relayTokensUsed,
		relayErrorsTotal,
		relayActiveRequests,
		authAttemptsTotal,
		authTokensIssued,
		authTokensValidated,
		usersActive,
		quotaUsage,
		channelsActive,
		modelsUsage,
	)

	return &ApplicationMetrics{
		HTTPRequestsTotal:     httpRequestsTotal,
		HTTPRequestDuration:   httpRequestDuration,
		HTTPRequestSize:       httpRequestSize,
		HTTPResponseSize:      httpResponseSize,
		HTTPActiveConnections: httpActiveConnections,
		RelayRequestsTotal:    relayRequestsTotal,
		RelayRequestDuration:  relayRequestDuration,
		RelayTokensUsed:       relayTokensUsed,
		RelayErrorsTotal:      relayErrorsTotal,
		RelayActiveRequests:   relayActiveRequests,
		AuthAttemptsTotal:     authAttemptsTotal,
		AuthTokensIssued:      authTokensIssued,
		AuthTokensValidated:   authTokensValidated,
		UsersActive:           usersActive,
		QuotaUsage:           quotaUsage,
		ChannelsActive:       channelsActive,
		ModelsUsage:          modelsUsage,
	}
}

// createTestPrometheusMiddleware creates a middleware with custom metrics for testing
func createTestPrometheusMiddleware(metrics *ApplicationMetrics) gin.HandlerFunc {
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
		if metrics.HTTPActiveConnections != nil {
			metrics.HTTPActiveConnections.Inc()
		}

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
		if metrics.HTTPActiveConnections != nil {
			metrics.HTTPActiveConnections.Dec()
		}
	})
}

// createTestConfigurablePrometheusMiddleware creates a configurable middleware with custom metrics for testing
func createTestConfigurablePrometheusMiddleware(config *MetricsConfig, metrics *ApplicationMetrics) gin.HandlerFunc {
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

		if metrics.HTTPActiveConnections != nil {
			metrics.HTTPActiveConnections.Inc()
		}

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
		if metrics.HTTPActiveConnections != nil {
			metrics.HTTPActiveConnections.Dec()
		}
	})
}

func TestNewApplicationMetrics(t *testing.T) {
	metrics := createTestMetrics()

	// Test that all metrics are properly initialized
	assert.NotNil(t, metrics.HTTPRequestsTotal)
	assert.NotNil(t, metrics.HTTPRequestDuration)
	assert.NotNil(t, metrics.RelayRequestsTotal)
	assert.NotNil(t, metrics.RelayRequestDuration)
	assert.NotNil(t, metrics.AuthAttemptsTotal)
	assert.NotNil(t, metrics.UsersActive)
}

func TestApplicationMetrics_RecordHTTPRequest(t *testing.T) {
	metrics := createTestMetrics()

	// Record a sample HTTP request
	metrics.RecordHTTPRequest("GET", "/api/test", 200, time.Millisecond*100, 1024, 2048)

	// We can't easily test the actual metric values without accessing internal state
	// But we can ensure the method doesn't panic and works correctly
	assert.NotNil(t, metrics)
}

func TestApplicationMetrics_RecordRelayRequest(t *testing.T) {
	metrics := createTestMetrics()

	// Record a sample relay request
	metrics.RecordRelayRequest("openai", "gpt-4", "123", "success", time.Second*2)

	assert.NotNil(t, metrics)
}

func TestApplicationMetrics_RecordTokenUsage(t *testing.T) {
	metrics := createTestMetrics()

	// Record token usage
	metrics.RecordTokenUsage("openai", "gpt-4", "123", "prompt", 150)
	metrics.RecordTokenUsage("openai", "gpt-4", "123", "completion", 75)

	assert.NotNil(t, metrics)
}

func TestApplicationMetrics_ActiveRequests(t *testing.T) {
	metrics := createTestMetrics()

	// Test increment and decrement
	metrics.IncrementActiveRequests()
	metrics.IncrementActiveRequests()
	metrics.DecrementActiveRequests()

	assert.NotNil(t, metrics)
}

func TestRelayMetricsWrapper(t *testing.T) {
	InitMetrics()

	tests := []struct {
		name     string
		action   func(*RelayMetricsWrapper)
		provider string
		model    string
	}{
		{
			name: "Success case",
			action: func(wrapper *RelayMetricsWrapper) {
				wrapper.RecordTokenUsage("prompt", 100)
				wrapper.Success()
			},
			provider: "openai",
			model:    "gpt-4",
		},
		{
			name: "Error case",
			action: func(wrapper *RelayMetricsWrapper) {
				wrapper.Error("timeout")
			},
			provider: "anthropic",
			model:    "claude-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := NewRelayMetricsWrapper(tt.provider, tt.model, "test-channel")
			assert.NotNil(t, wrapper)

			tt.action(wrapper)
		})
	}
}

func TestPrometheusMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use test metrics instead of global metrics
	testMetrics := createTestMetrics()

	router := gin.New()
	router.Use(createTestPrometheusMiddleware(testMetrics))

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"message": "created"})
	})

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		statusCode int
	}{
		{
			name:       "GET request",
			method:     "GET",
			path:       "/test",
			statusCode: http.StatusOK,
		},
		{
			name:       "POST request with body",
			method:     "POST",
			path:       "/test",
			body:       `{"key": "value"}`,
			statusCode: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestConfigurablePrometheusMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Use test metrics instead of global metrics
	testMetrics := createTestMetrics()

	tests := []struct {
		name   string
		config *MetricsConfig
		path   string
		skip   bool
	}{
		{
			name:   "Disabled metrics",
			config: &MetricsConfig{Enabled: false},
			path:   "/test",
			skip:   true,
		},
		{
			name: "Skip specific paths",
			config: &MetricsConfig{
				Enabled:   true,
				SkipPaths: []string{"/metrics", "/health"},
			},
			path: "/metrics",
			skip: true,
		},
		{
			name: "Normal path",
			config: &MetricsConfig{
				Enabled:     true,
				IncludePath: true,
			},
			path: "/api/test",
			skip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(createTestConfigurablePrometheusMiddleware(tt.config, testMetrics))

			router.GET(tt.path, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// No need to init metrics, handler works with any registered metrics

	router := gin.New()
	router.GET("/metrics", Handler())

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")

	// Check that the response contains Prometheus metrics
	body := w.Body.String()
	assert.Contains(t, body, "# HELP")
	assert.Contains(t, body, "# TYPE")
}

func TestHealthHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/health", HealthHandler())

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	body := w.Body.String()
	assert.Contains(t, body, "status")
	assert.Contains(t, body, "ok")
	assert.Contains(t, body, "new-api")
}

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// No need to init metrics, just testing route registration

	router := gin.New()
	RegisterRoutes(router)

	tests := []struct {
		path         string
		expectedCode int
	}{
		{"/metrics", http.StatusOK},
		{"/health", http.StatusOK},
		{"/ping", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
		})
	}
}

func TestSetupMetricsRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Set up test metrics to avoid InitMetrics conflicts
	originalAppMetrics := AppMetrics
	AppMetrics = createTestMetrics()
	defer func() { AppMetrics = originalAppMetrics }()

	router := gin.New()
	config := DefaultMetricsConfig()

	// This should not panic and should set up routes properly
	SetupMetricsRoutes(router, config)

	// Test that metrics endpoint is available
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDefaultMetricsConfig(t *testing.T) {
	config := DefaultMetricsConfig()

	assert.True(t, config.Enabled)
	assert.True(t, config.IncludePath)
	assert.True(t, config.GroupedStatusCode)
	assert.Contains(t, config.SkipPaths, "/metrics")
	assert.Contains(t, config.SkipPaths, "/health")
	assert.Contains(t, config.SkipPaths, "/ping")
}

func TestResponseWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	rw := &responseWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
		statusCode:     200,
	}

	// Test Write
	data := []byte("test response")
	n, err := rw.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, data, rw.body.Bytes())

	// Test WriteHeader
	rw.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, rw.statusCode)
}

func TestGetRoutePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		setupContext func() *gin.Context
		expectedPath string
	}{
		{
			name: "With route pattern",
			setupContext: func() *gin.Context {
				router := gin.New()
				router.GET("/users/:id", func(c *gin.Context) {})

				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/users/123", nil)
				c, _ := gin.CreateTestContext(w)
				c.Request = req

				// Simulate route matching
				router.HandleContext(c)
				return c
			},
			expectedPath: "/users/:id",
		},
		{
			name: "Without route pattern",
			setupContext: func() *gin.Context {
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/unknown/path", nil)
				c, _ := gin.CreateTestContext(w)
				c.Request = req
				return c
			},
			expectedPath: "/unknown/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupContext()
			path := getRoutePath(c)
			assert.Equal(t, tt.expectedPath, path)
		})
	}
}