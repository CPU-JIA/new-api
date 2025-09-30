package controller

import (
	"fmt"
	"net/http"
	"one-api/common"
	"one-api/model"
	"one-api/service"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GetCacheMetricsOverview returns aggregated cache statistics
// GET /api/cache/metrics/overview?period=24h
func GetCacheMetricsOverview(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")

	// Parse period to duration
	endTime := time.Now()
	var startTime time.Time
	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid period. Valid values: 1h, 24h, 7d, 30d",
		})
		return
	}

	// Get aggregated metrics
	summary, err := model.GetPromptCacheMetricsSummary(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get cache metrics: %v", err),
		})
		return
	}

	// Get warmup metrics from CacheWarmer service
	warmerMetrics := service.GetCacheWarmerService().GetMetrics()
	activeWarmupChannels := 0
	for _, m := range warmerMetrics {
		if m.WarmupEnabled {
			activeWarmupChannels++
		}
	}

	// Calculate net savings (assume warmup cost ~$0.001 per request)
	// This is a rough estimate; actual warmup cost tracking would require separate metrics
	totalCostSaved := summary["total_cost_saved"].(float64)
	estimatedWarmupCost := float64(activeWarmupChannels) * 0.001 // Placeholder
	netSavings := totalCostSaved - estimatedWarmupCost

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_requests":          summary["total_requests"],
			"cache_hit_rate":          summary["avg_cache_hit_rate"],
			"total_cost_saved":        totalCostSaved,
			"estimated_warmup_cost":   estimatedWarmupCost,
			"net_savings":             netSavings,
			"active_warmup_channels":  activeWarmupChannels,
			"period":                  period,
			"start_time":              startTime.Unix(),
			"end_time":                endTime.Unix(),
		},
	})
}

// GetCacheMetricsChart returns time-series data for charting
// GET /api/cache/metrics/chart?period=24h&interval=1h
func GetCacheMetricsChart(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")
	interval := c.DefaultQuery("interval", "1h")

	// Parse period
	endTime := time.Now()
	var startTime time.Time
	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid period",
		})
		return
	}

	// Parse interval
	var intervalDuration time.Duration
	switch interval {
	case "1m":
		intervalDuration = 1 * time.Minute
	case "5m":
		intervalDuration = 5 * time.Minute
	case "15m":
		intervalDuration = 15 * time.Minute
	case "1h":
		intervalDuration = 1 * time.Hour
	case "1d":
		intervalDuration = 24 * time.Hour
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid interval. Valid values: 1m, 5m, 15m, 1h, 1d",
		})
		return
	}

	// Generate time buckets
	timestamps := []int64{}
	cacheHitRates := []float64{}
	costSaved := []float64{}

	currentTime := startTime
	for currentTime.Before(endTime) {
		bucketEnd := currentTime.Add(intervalDuration)
		if bucketEnd.After(endTime) {
			bucketEnd = endTime
		}

		// Get metrics for this time bucket
		summary, err := model.GetPromptCacheMetricsSummary(currentTime, bucketEnd)
		if err != nil {
			// Skip this bucket on error
			currentTime = bucketEnd
			continue
		}

		timestamps = append(timestamps, currentTime.Unix())

		hitRate := 0.0
		if summary["avg_cache_hit_rate"] != nil {
			hitRate = summary["avg_cache_hit_rate"].(float64)
		}
		cacheHitRates = append(cacheHitRates, hitRate)

		saved := 0.0
		if summary["total_cost_saved"] != nil {
			saved = summary["total_cost_saved"].(float64)
		}
		costSaved = append(costSaved, saved)

		currentTime = bucketEnd
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"timestamps":      timestamps,
			"cache_hit_rates": cacheHitRates,
			"cost_saved":      costSaved,
			"period":          period,
			"interval":        interval,
		},
	})
}

// GetCacheMetricsByChannels returns aggregated metrics grouped by channel
// GET /api/cache/metrics/channels?period=24h
func GetCacheMetricsByChannels(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")

	// Parse period
	endTime := time.Now()
	var startTime time.Time
	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid period",
		})
		return
	}

	// Get channel-grouped metrics
	channelMetrics, err := model.GetPromptCacheMetricsByChannelGrouped(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get channel metrics: %v", err),
		})
		return
	}

	// Enrich with warmup status from CacheWarmer
	warmerMetrics := service.GetCacheWarmerService().GetMetrics()
	for i, cm := range channelMetrics {
		channelId := cm["channel_id"].(int)
		if wm, ok := warmerMetrics[channelId]; ok {
			channelMetrics[i]["warmup_enabled"] = wm.WarmupEnabled
			channelMetrics[i]["last_warmup"] = wm.LastWarmup.Unix()
			channelMetrics[i]["request_count_5min"] = wm.RequestCount5Min
		} else {
			channelMetrics[i]["warmup_enabled"] = false
			channelMetrics[i]["last_warmup"] = 0
			channelMetrics[i]["request_count_5min"] = 0
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channelMetrics,
		"period":  period,
	})
}

// GetCacheWarmerStatus returns real-time status of CacheWarmer service
// GET /api/cache/warmer/status
func GetCacheWarmerStatus(c *gin.Context) {
	warmerMetrics := service.GetCacheWarmerService().GetMetrics()

	// Convert metrics map to array for easier frontend consumption
	statusArray := []gin.H{}
	for _, m := range warmerMetrics {
		statusArray = append(statusArray, gin.H{
			"channel_id":          m.ChannelID,
			"channel_name":        m.ChannelName,
			"warmup_enabled":      m.WarmupEnabled,
			"request_count_5min":  m.RequestCount5Min,
			"last_request":        m.LastRequest.Unix(),
			"last_warmup":         m.LastWarmup.Unix(),
			"window_start":        m.WindowStart.Unix(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"channels": statusArray,
			"total_channels": len(statusArray),
		},
	})
}

// GetCacheMetricsByUser returns cache metrics for a specific user (admin or self)
// GET /api/cache/metrics/user/:user_id?period=24h
func GetCacheMetricsByUser(c *gin.Context) {
	userIdStr := c.Param("user_id")
	userId, err := strconv.Atoi(userIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid user ID",
		})
		return
	}

	// Check permission: admin or self
	currentRole := c.GetInt("role")
	currentUserId := c.GetInt("id")
	if currentRole < common.RoleAdminUser && currentUserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "Permission denied",
		})
		return
	}

	period := c.DefaultQuery("period", "24h")
	endTime := time.Now()
	var startTime time.Time
	switch period {
	case "1h":
		startTime = endTime.Add(-1 * time.Hour)
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = endTime.Add(-30 * 24 * time.Hour)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid period",
		})
		return
	}

	metrics, err := model.GetPromptCacheMetricsByUser(userId, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get user metrics: %v", err),
		})
		return
	}

	// Calculate summary
	totalRequests := len(metrics)
	totalCostSaved := 0.0
	totalCacheHitRate := 0.0
	for _, m := range metrics {
		totalCostSaved += m.CostSaved
		totalCacheHitRate += m.CacheHitRate
	}
	avgCacheHitRate := 0.0
	if totalRequests > 0 {
		avgCacheHitRate = totalCacheHitRate / float64(totalRequests)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":           userId,
			"total_requests":    totalRequests,
			"total_cost_saved":  totalCostSaved,
			"avg_cache_hit_rate": avgCacheHitRate,
			"period":            period,
			"metrics":           metrics,
		},
	})
}