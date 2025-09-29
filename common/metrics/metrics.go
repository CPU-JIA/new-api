package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// Subsystem names
	SubsystemHTTP  = "http"
	SubsystemRelay = "relay"
	SubsystemDB    = "database"
	SubsystemCache = "cache"
	SubsystemAuth  = "auth"
)

// ApplicationMetrics holds all the metrics for the application
type ApplicationMetrics struct {
	// HTTP metrics
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPRequestSize       *prometheus.HistogramVec
	HTTPResponseSize      *prometheus.HistogramVec
	HTTPActiveConnections prometheus.Gauge

	// Relay/AI Provider metrics
	RelayRequestsTotal    *prometheus.CounterVec
	RelayRequestDuration  *prometheus.HistogramVec
	RelayTokensUsed       *prometheus.CounterVec
	RelayErrorsTotal      *prometheus.CounterVec
	RelayActiveRequests   prometheus.Gauge

	// Database metrics
	DBConnections         *prometheus.GaugeVec
	DBOperationsTotal     *prometheus.CounterVec
	DBOperationDuration   *prometheus.HistogramVec

	// Cache metrics
	CacheOperationsTotal  *prometheus.CounterVec
	CacheHitRatio         *prometheus.GaugeVec

	// Authentication metrics
	AuthAttemptsTotal     *prometheus.CounterVec
	AuthTokensIssued      prometheus.Counter
	AuthTokensValidated   *prometheus.CounterVec

	// Business metrics
	UsersActive           prometheus.Gauge
	QuotaUsage           *prometheus.CounterVec
	ChannelsActive       *prometheus.GaugeVec
	ModelsUsage          *prometheus.CounterVec
}

// NewApplicationMetrics creates and registers all application metrics
func NewApplicationMetrics() *ApplicationMetrics {
	return &ApplicationMetrics{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemHTTP,
				Name:      "requests_total",
				Help:      "Total number of HTTP requests",
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: SubsystemHTTP,
				Name:      "request_duration_seconds",
				Help:      "HTTP request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: SubsystemHTTP,
				Name:      "request_size_bytes",
				Help:      "HTTP request size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B to 100MB
			},
			[]string{"method", "path"},
		),
		HTTPResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: SubsystemHTTP,
				Name:      "response_size_bytes",
				Help:      "HTTP response size in bytes",
				Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B to 100MB
			},
			[]string{"method", "path", "status_code"},
		),
		HTTPActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Subsystem: SubsystemHTTP,
				Name:      "active_connections",
				Help:      "Number of active HTTP connections",
			},
		),

		// Relay/AI Provider metrics
		RelayRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemRelay,
				Name:      "requests_total",
				Help:      "Total number of relay requests to AI providers",
			},
			[]string{"provider", "model", "channel_id", "status"},
		),
		RelayRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: SubsystemRelay,
				Name:      "request_duration_seconds",
				Help:      "Relay request duration in seconds",
				Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}, // AI requests can be slow
			},
			[]string{"provider", "model", "channel_id"},
		),
		RelayTokensUsed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemRelay,
				Name:      "tokens_used_total",
				Help:      "Total number of tokens used",
			},
			[]string{"provider", "model", "channel_id", "token_type"}, // token_type: prompt, completion
		),
		RelayErrorsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemRelay,
				Name:      "errors_total",
				Help:      "Total number of relay errors",
			},
			[]string{"provider", "model", "channel_id", "error_type"},
		),
		RelayActiveRequests: promauto.NewGauge(
			prometheus.GaugeOpts{
				Subsystem: SubsystemRelay,
				Name:      "active_requests",
				Help:      "Number of active relay requests",
			},
		),

		// Database metrics
		DBConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Subsystem: SubsystemDB,
				Name:      "connections",
				Help:      "Number of database connections",
			},
			[]string{"state"}, // state: idle, in_use, open
		),
		DBOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemDB,
				Name:      "operations_total",
				Help:      "Total number of database operations",
			},
			[]string{"operation", "table", "status"},
		),
		DBOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Subsystem: SubsystemDB,
				Name:      "operation_duration_seconds",
				Help:      "Database operation duration in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"operation", "table"},
		),

		// Cache metrics
		CacheOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemCache,
				Name:      "operations_total",
				Help:      "Total number of cache operations",
			},
			[]string{"operation", "cache_type", "status"}, // operation: get, set, delete; status: hit, miss, error
		),
		CacheHitRatio: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Subsystem: SubsystemCache,
				Name:      "hit_ratio",
				Help:      "Cache hit ratio",
			},
			[]string{"cache_type"},
		),

		// Authentication metrics
		AuthAttemptsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemAuth,
				Name:      "attempts_total",
				Help:      "Total number of authentication attempts",
			},
			[]string{"method", "status"}, // method: token, password, oauth; status: success, failure
		),
		AuthTokensIssued: promauto.NewCounter(
			prometheus.CounterOpts{
				Subsystem: SubsystemAuth,
				Name:      "tokens_issued_total",
				Help:      "Total number of authentication tokens issued",
			},
		),
		AuthTokensValidated: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Subsystem: SubsystemAuth,
				Name:      "tokens_validated_total",
				Help:      "Total number of token validations",
			},
			[]string{"status"}, // status: valid, invalid, expired
		),

		// Business metrics
		UsersActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "users_active",
				Help: "Number of active users",
			},
		),
		QuotaUsage: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "quota_usage_total",
				Help: "Total quota usage",
			},
			[]string{"user_group", "resource_type"},
		),
		ChannelsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "channels_active",
				Help: "Number of active channels",
			},
			[]string{"provider", "status"}, // status: healthy, unhealthy, disabled
		),
		ModelsUsage: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "models_usage_total",
				Help: "Total model usage",
			},
			[]string{"model", "provider"},
		),
	}
}

// RecordHTTPRequest records metrics for HTTP requests
func (m *ApplicationMetrics) RecordHTTPRequest(method, path string, statusCode int, duration time.Duration, requestSize, responseSize int64) {
	statusStr := strconv.Itoa(statusCode)

	m.HTTPRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	m.HTTPRequestSize.WithLabelValues(method, path).Observe(float64(requestSize))
	m.HTTPResponseSize.WithLabelValues(method, path, statusStr).Observe(float64(responseSize))
}

// RecordRelayRequest records metrics for relay requests
func (m *ApplicationMetrics) RecordRelayRequest(provider, model, channelID, status string, duration time.Duration) {
	m.RelayRequestsTotal.WithLabelValues(provider, model, channelID, status).Inc()
	m.RelayRequestDuration.WithLabelValues(provider, model, channelID).Observe(duration.Seconds())
}

// RecordTokenUsage records token usage metrics
func (m *ApplicationMetrics) RecordTokenUsage(provider, model, channelID, tokenType string, count int) {
	m.RelayTokensUsed.WithLabelValues(provider, model, channelID, tokenType).Add(float64(count))
}

// RecordRelayError records relay error metrics
func (m *ApplicationMetrics) RecordRelayError(provider, model, channelID, errorType string) {
	m.RelayErrorsTotal.WithLabelValues(provider, model, channelID, errorType).Inc()
}

// IncrementActiveRequests increments active relay requests
func (m *ApplicationMetrics) IncrementActiveRequests() {
	m.RelayActiveRequests.Inc()
}

// DecrementActiveRequests decrements active relay requests
func (m *ApplicationMetrics) DecrementActiveRequests() {
	m.RelayActiveRequests.Dec()
}

// RecordAuthAttempt records authentication attempt metrics
func (m *ApplicationMetrics) RecordAuthAttempt(method, status string) {
	m.AuthAttemptsTotal.WithLabelValues(method, status).Inc()
}

// RecordTokenValidation records token validation metrics
func (m *ApplicationMetrics) RecordTokenValidation(status string) {
	m.AuthTokensValidated.WithLabelValues(status).Inc()
}

// Global metrics instance
var AppMetrics *ApplicationMetrics

// InitMetrics initializes the global metrics instance
func InitMetrics() {
	AppMetrics = NewApplicationMetrics()
}

// GetMetrics returns the global metrics instance
func GetMetrics() *ApplicationMetrics {
	if AppMetrics == nil {
		InitMetrics()
	}
	return AppMetrics
}