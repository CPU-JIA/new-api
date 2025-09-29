package metrics

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler returns the Prometheus metrics HTTP handler
func Handler() gin.HandlerFunc {
	h := promhttp.Handler()
	return gin.WrapH(h)
}

// HealthHandler returns a simple health check handler
func HealthHandler() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "new-api",
		})
	})
}

// RegisterRoutes registers metrics and health routes
func RegisterRoutes(router *gin.Engine) {
	router.GET("/metrics", Handler())
	router.GET("/health", HealthHandler())
	router.GET("/ping", gin.HandlerFunc(func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	}))
}

// SetupMetricsRoutes is a convenience function to set up metrics routes
func SetupMetricsRoutes(router *gin.Engine, config *MetricsConfig) {
	// Initialize metrics if not already done
	if AppMetrics == nil {
		InitMetrics()
	}

	// Add metrics middleware
	router.Use(ConfigurablePrometheusMiddleware(config))

	// Register metrics routes
	RegisterRoutes(router)
}