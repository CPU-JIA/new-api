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

	// ECP-C1: Defensive Programming - use actual warmup cost from database instead of estimation
	totalCostSaved := summary["total_cost_saved"].(float64)
	actualWarmupCost, err := model.GetWarmupCost(startTime, endTime)
	if err != nil {
		// Fallback to estimation if query fails
		actualWarmupCost = float64(activeWarmupChannels) * 0.001
	}
	netSavings := totalCostSaved - actualWarmupCost

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_requests":         summary["total_requests"],
			"cache_hit_rate":         summary["avg_cache_hit_rate"],
			"active_warmup_channels": activeWarmupChannels,
			"period":                 period,
			"start_time":             startTime.Unix(),
			"end_time":               endTime.Unix(),

			// 🔥 Multi-unit support for cost_saved
			"cost_saved_quota":  totalCostSaved,
			"cost_saved_usd":    common.QuotaToUSD(totalCostSaved),
			"cost_saved_cny":    common.QuotaToCNY(totalCostSaved),
			"cost_saved_tokens": common.QuotaToTokens(totalCostSaved),

			// 🔥 Multi-unit support for net_savings
			"net_savings_quota":  netSavings,
			"net_savings_usd":    common.QuotaToUSD(netSavings),
			"net_savings_cny":    common.QuotaToCNY(netSavings),
			"net_savings_tokens": common.QuotaToTokens(netSavings),

			// 🔥 Warmup cost breakdown (using actual data instead of estimation)
			"warmup_cost_quota":  actualWarmupCost,
			"warmup_cost_usd":    common.QuotaToUSD(actualWarmupCost),
			"warmup_cost_cny":    common.QuotaToCNY(actualWarmupCost),
			"warmup_cost_tokens": common.QuotaToTokens(actualWarmupCost),

			// 🔥 Unit conversion metadata
			"conversion_rates": gin.H{
				"quota_per_usd": common.QuotaPerUnit,
				"usd_to_cny":    common.USDToCNYRate,
			},
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

		// 🔥 Add multi-unit cost data for chart
		costSavedUSD := common.QuotaToUSD(saved)
		costSavedCNY := common.QuotaToCNY(saved)
		costSavedTokens := float64(common.QuotaToTokens(saved))

		// Store in separate arrays (we'll add to response later)
		_ = costSavedUSD
		_ = costSavedCNY
		_ = costSavedTokens

		currentTime = bucketEnd
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"timestamps":      timestamps,
			"cache_hit_rates": cacheHitRates,

			// 🔥 Multi-unit cost data
			"cost_saved_quota":  costSaved,
			"cost_saved_usd":    convertArrayToUSD(costSaved),
			"cost_saved_cny":    convertArrayToCNY(costSaved),
			"cost_saved_tokens": convertArrayToTokens(costSaved),

			"period":            period,
			"interval":          interval,
		},
	})
}

// Helper functions for array conversion
func convertArrayToUSD(quotaArray []float64) []float64 {
	result := make([]float64, len(quotaArray))
	for i, quota := range quotaArray {
		result[i] = common.QuotaToUSD(quota)
	}
	return result
}

func convertArrayToCNY(quotaArray []float64) []float64 {
	result := make([]float64, len(quotaArray))
	for i, quota := range quotaArray {
		result[i] = common.QuotaToCNY(quota)
	}
	return result
}

func convertArrayToTokens(quotaArray []float64) []int {
	result := make([]int, len(quotaArray))
	for i, quota := range quotaArray {
		result[i] = common.QuotaToTokens(quota)
	}
	return result
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

		// Add warmup status
		if wm, ok := warmerMetrics[channelId]; ok {
			channelMetrics[i]["warmup_enabled"] = wm.WarmupEnabled
			channelMetrics[i]["last_warmup"] = wm.LastWarmup.Unix()
			channelMetrics[i]["request_count_5min"] = wm.RequestCount5Min
		} else {
			channelMetrics[i]["warmup_enabled"] = false
			channelMetrics[i]["last_warmup"] = 0
			channelMetrics[i]["request_count_5min"] = 0
		}

		// 🔥 Add multi-unit cost data for each channel
		totalCostSaved := 0.0
		if cost, ok := cm["total_cost_saved"].(float64); ok {
			totalCostSaved = cost
		}
		channelMetrics[i]["cost_saved_quota"] = totalCostSaved
		channelMetrics[i]["cost_saved_usd"] = common.QuotaToUSD(totalCostSaved)
		channelMetrics[i]["cost_saved_cny"] = common.QuotaToCNY(totalCostSaved)
		channelMetrics[i]["cost_saved_tokens"] = common.QuotaToTokens(totalCostSaved)
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
			// 🔥 Optimization 5 & 6: ROI monitoring and TTL configuration fields
			"warmup_count":        m.WarmupCount,
			"consecutive_low_roi": m.ConsecutiveLowROI,
			"optimal_interval":    int(m.OptimalInterval.Seconds()),
			"request_rate":        m.RequestRate,
			"ttl":                 m.TTL,
			"last_roi_check":      m.LastROICheck.Unix(),
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

// GetCachePerformanceAnalysis returns comprehensive cache performance and ROI analysis
// GET /api/cache/performance?period=24h
// ECP-C3: Performance Awareness - single optimized query for all ROI metrics
func GetCachePerformanceAnalysis(c *gin.Context) {
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
			"message": "Invalid period. Valid values: 1h, 24h, 7d, 30d",
		})
		return
	}

	// Get comprehensive ROI metrics from model layer
	roiMetrics, err := model.GetCacheROIMetrics(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get ROI metrics: %v", err),
		})
		return
	}

	// Get warmup service status
	warmerMetrics := service.GetCacheWarmerService().GetMetrics()
	activeWarmupChannels := 0
	totalChannelsTracked := len(warmerMetrics)
	for _, m := range warmerMetrics {
		if m.WarmupEnabled {
			activeWarmupChannels++
		}
	}

	// Extract values for multi-unit conversion
	totalCostSaved := roiMetrics["total_cost_saved"].(float64)
	warmupCost := roiMetrics["warmup_cost"].(float64)
	netSavings := roiMetrics["net_savings"].(float64)
	roi := roiMetrics["roi"].(float64)
	breakEvenPoint := roiMetrics["break_even_point"].(float64)
	isCostEffective := roiMetrics["is_cost_effective"].(bool)
	efficiencyRatio := roiMetrics["efficiency_ratio"].(float64)

	// ECP-C1: Defensive Programming - generate actionable alerts based on metrics
	alerts := []string{}
	if !isCostEffective {
		alerts = append(alerts, "⚠️ 警告: 缓存成本效益为负，预热成本超过节省成本")
	}
	if roi < 1.0 && roi >= 0 {
		alerts = append(alerts, "⚠️ 注意: ROI低于100%，建议优化预热频率或增加用户请求量")
	}
	if roiMetrics["avg_cache_hit_rate"].(float64) < 0.5 {
		alerts = append(alerts, "⚠️ 注意: 缓存命中率低于50%，建议检查padding内容配置")
	}
	if activeWarmupChannels == 0 && totalChannelsTracked > 0 {
		alerts = append(alerts, "ℹ️ 提示: 当前无活跃预热渠道，缓存可能已过期")
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			// Period information
			"period":     period,
			"start_time": startTime.Unix(),
			"end_time":   endTime.Unix(),

			// Core metrics
			"total_requests":     roiMetrics["total_requests"],
			"avg_cache_hit_rate": roiMetrics["avg_cache_hit_rate"],

			// Cost analysis (multi-unit)
			"cost_saved": gin.H{
				"quota":  totalCostSaved,
				"usd":    common.QuotaToUSD(totalCostSaved),
				"cny":    common.QuotaToCNY(totalCostSaved),
				"tokens": common.QuotaToTokens(totalCostSaved),
			},
			"warmup_cost": gin.H{
				"quota":  warmupCost,
				"usd":    common.QuotaToUSD(warmupCost),
				"cny":    common.QuotaToCNY(warmupCost),
				"tokens": common.QuotaToTokens(warmupCost),
			},
			"net_savings": gin.H{
				"quota":  netSavings,
				"usd":    common.QuotaToUSD(netSavings),
				"cny":    common.QuotaToCNY(netSavings),
				"tokens": common.QuotaToTokens(netSavings),
			},

			// ROI indicators
			"roi":                roi * 100, // Convert to percentage
			"roi_formatted":      fmt.Sprintf("%.2f%%", roi*100),
			"break_even_point":   breakEvenPoint,
			"is_cost_effective":  isCostEffective,
			"efficiency_ratio":   efficiencyRatio,

			// Warmup status
			"warmup_status": gin.H{
				"active_channels":      activeWarmupChannels,
				"total_channels_tracked": totalChannelsTracked,
				"coverage_rate":        float64(activeWarmupChannels) / float64(totalChannelsTracked),
			},

			// Actionable insights
			"alerts": alerts,
			"recommendations": generateRecommendations(roiMetrics, activeWarmupChannels, totalChannelsTracked),

			// Token metrics
			"token_metrics": gin.H{
				"total_cache_read_tokens": roiMetrics["total_cache_read_tokens"],
				"total_prompt_tokens":     roiMetrics["total_prompt_tokens"],
				"cache_utilization":       float64(roiMetrics["total_cache_read_tokens"].(int64)) / float64(roiMetrics["total_prompt_tokens"].(int64)),
			},
		},
	})
}

// generateRecommendations generates actionable recommendations based on cache performance
// ECP-B2: KISS - simple rule-based recommendations
func generateRecommendations(roiMetrics map[string]interface{}, activeChannels, totalChannels int) []string {
	recommendations := []string{}

	roi := roiMetrics["roi"].(float64)
	cacheHitRate := roiMetrics["avg_cache_hit_rate"].(float64)
	isCostEffective := roiMetrics["is_cost_effective"].(bool)

	// ROI-based recommendations
	if !isCostEffective {
		recommendations = append(recommendations, "建议禁用低频渠道的预热功能以降低成本")
	} else if roi > 5.0 {
		recommendations = append(recommendations, "✅ 缓存效果极佳，可考虑增加预热覆盖范围")
	} else if roi < 2.0 {
		recommendations = append(recommendations, "建议增加预热间隔时间（当前默认4分钟）")
	}

	// Cache hit rate recommendations
	if cacheHitRate < 0.3 {
		recommendations = append(recommendations, "缓存命中率较低，建议检查padding内容是否与实际请求匹配")
	} else if cacheHitRate > 0.8 {
		recommendations = append(recommendations, "✅ 缓存命中率优秀，继续保持当前配置")
	}

	// Coverage recommendations
	if activeChannels == 0 && totalChannels > 0 {
		recommendations = append(recommendations, "当前无活跃预热，建议增加请求频率或降低预热阈值")
	} else if float64(activeChannels)/float64(totalChannels) < 0.3 {
		recommendations = append(recommendations, "预热覆盖率较低，可考虑降低预热启动阈值（当前默认10请求/5分钟）")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "✅ 系统运行良好，无需调整")
	}

	return recommendations
}