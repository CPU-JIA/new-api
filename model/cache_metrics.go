package model

import (
	"time"
)

// PromptCacheMetrics tracks Claude API prompt caching effectiveness and cost savings
// Used for cache analytics dashboard and performance monitoring
// Note: This is different from CacheMetrics in cache_manager.go which tracks internal memory cache
type PromptCacheMetrics struct {
	Id        int       `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at" gorm:"index:idx_prompt_cache_created_at;index:idx_prompt_cache_channel_time,priority:2"`

	// Request identification
	ChannelId   int    `json:"channel_id" gorm:"index:idx_prompt_cache_channel_id;index:idx_prompt_cache_channel_time,priority:1"`
	ChannelName string `json:"channel_name"`
	UserId      int    `json:"user_id" gorm:"index:idx_prompt_cache_user_id"`
	TokenId     int    `json:"token_id" gorm:"index:idx_prompt_cache_token_id"`
	LogId       int    `json:"log_id" gorm:"index:idx_prompt_cache_log_id"` // Reference to Log table
	ModelName   string `json:"model_name" gorm:"index:idx_prompt_cache_model"`

	// Cache token metrics (from Claude API usage response)
	PromptTokens        int `json:"prompt_tokens"`         // Total prompt tokens
	CacheReadTokens     int `json:"cache_read_tokens"`     // Tokens read from cache (0.1x cost)
	CacheCreationTokens int `json:"cache_creation_tokens"` // Tokens written to cache (1.25x cost)
	CompletionTokens    int `json:"completion_tokens"`     // Output tokens

	// Derived metrics
	UncachedTokens int     `json:"uncached_tokens"` // Tokens not cached (1.0x cost)
	CacheHitRate   float64 `json:"cache_hit_rate"`  // cache_read / prompt_tokens

	// Cost analysis (in quota units, not dollars)
	CostWithoutCache float64 `json:"cost_without_cache"` // Hypothetical cost if no cache
	CostWithCache    float64 `json:"cost_with_cache"`    // Actual cost
	CostSaved        float64 `json:"cost_saved"`         // Savings from cache

	// Metadata
	IsWarmup bool `json:"is_warmup"` // True if this is a warmup request from CacheWarmer
}

// TableName specifies the table name for GORM
func (PromptCacheMetrics) TableName() string {
	return "prompt_cache_metrics"
}

// GetPromptCacheMetricsByChannel retrieves cache metrics for a specific channel within a time range
func GetPromptCacheMetricsByChannel(channelId int, startTime, endTime time.Time) ([]PromptCacheMetrics, error) {
	var metrics []PromptCacheMetrics
	err := DB.Where("channel_id = ? AND created_at >= ? AND created_at <= ?",
		channelId, startTime, endTime).
		Order("created_at DESC").
		Find(&metrics).Error
	return metrics, err
}

// GetPromptCacheMetricsByUser retrieves cache metrics for a specific user within a time range
func GetPromptCacheMetricsByUser(userId int, startTime, endTime time.Time) ([]PromptCacheMetrics, error) {
	var metrics []PromptCacheMetrics
	err := DB.Where("user_id = ? AND created_at >= ? AND created_at <= ?",
		userId, startTime, endTime).
		Order("created_at DESC").
		Find(&metrics).Error
	return metrics, err
}

// GetPromptCacheMetricsSummary retrieves aggregated cache statistics for a time range
func GetPromptCacheMetricsSummary(startTime, endTime time.Time) (map[string]interface{}, error) {
	var result struct {
		TotalRequests        int64
		TotalCacheReadTokens int64
		TotalPromptTokens    int64
		TotalCostSaved       float64
		AvgCacheHitRate      float64
	}

	err := DB.Model(&PromptCacheMetrics{}).
		Select(`
			COUNT(*) as total_requests,
			SUM(cache_read_tokens) as total_cache_read_tokens,
			SUM(prompt_tokens) as total_prompt_tokens,
			SUM(cost_saved) as total_cost_saved,
			AVG(cache_hit_rate) as avg_cache_hit_rate
		`).
		Where("created_at >= ? AND created_at <= ? AND is_warmup = ?", startTime, endTime, false).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_requests":          result.TotalRequests,
		"total_cache_read_tokens": result.TotalCacheReadTokens,
		"total_prompt_tokens":     result.TotalPromptTokens,
		"total_cost_saved":        result.TotalCostSaved,
		"avg_cache_hit_rate":      result.AvgCacheHitRate,
	}, nil
}

// GetPromptCacheMetricsByChannelGrouped retrieves aggregated metrics grouped by channel
func GetPromptCacheMetricsByChannelGrouped(startTime, endTime time.Time) ([]map[string]interface{}, error) {
	var results []struct {
		ChannelId            int
		ChannelName          string
		TotalRequests        int64
		TotalCacheReadTokens int64
		TotalPromptTokens    int64
		TotalCostSaved       float64
		AvgCacheHitRate      float64
	}

	err := DB.Model(&PromptCacheMetrics{}).
		Select(`
			channel_id,
			channel_name,
			COUNT(*) as total_requests,
			SUM(cache_read_tokens) as total_cache_read_tokens,
			SUM(prompt_tokens) as total_prompt_tokens,
			SUM(cost_saved) as total_cost_saved,
			AVG(cache_hit_rate) as avg_cache_hit_rate
		`).
		Where("created_at >= ? AND created_at <= ? AND is_warmup = ?", startTime, endTime, false).
		Group("channel_id, channel_name").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// Convert to map slice for JSON serialization
	channelMetrics := make([]map[string]interface{}, len(results))
	for i, r := range results {
		channelMetrics[i] = map[string]interface{}{
			"channel_id":              r.ChannelId,
			"channel_name":            r.ChannelName,
			"total_requests":          r.TotalRequests,
			"total_cache_read_tokens": r.TotalCacheReadTokens,
			"total_prompt_tokens":     r.TotalPromptTokens,
			"total_cost_saved":        r.TotalCostSaved,
			"avg_cache_hit_rate":      r.AvgCacheHitRate,
		}
	}

	return channelMetrics, nil
}

// InsertPromptCacheMetrics inserts a new cache metrics record
func InsertPromptCacheMetrics(metric *PromptCacheMetrics) error {
	return DB.Create(metric).Error
}

// GetWarmupCost retrieves the actual cost of warmup requests within a time range
// ECP-C2: Systematic Error Handling - return error for proper handling
func GetWarmupCost(startTime, endTime time.Time) (float64, error) {
	var result struct {
		TotalWarmupCost float64
	}

	err := DB.Model(&PromptCacheMetrics{}).
		Select("SUM(cost_with_cache) as total_warmup_cost").
		Where("created_at >= ? AND created_at <= ? AND is_warmup = ?", startTime, endTime, true).
		Scan(&result).Error

	if err != nil {
		return 0, err
	}

	return result.TotalWarmupCost, nil
}

// GetCacheROIMetrics calculates comprehensive ROI metrics for cache performance
// ECP-C3: Performance Awareness - optimize query with single aggregation
func GetCacheROIMetrics(startTime, endTime time.Time) (map[string]interface{}, error) {
	var userResult struct {
		TotalRequests       int64
		TotalCostSaved      float64
		TotalCostWithCache  float64
		TotalCostWithout    float64
		AvgCacheHitRate     float64
		TotalCacheReadTokens int64
		TotalPromptTokens   int64
	}

	// Get user request metrics (exclude warmup)
	err := DB.Model(&PromptCacheMetrics{}).
		Select(`
			COUNT(*) as total_requests,
			SUM(cost_saved) as total_cost_saved,
			SUM(cost_with_cache) as total_cost_with_cache,
			SUM(cost_without_cache) as total_cost_without,
			AVG(cache_hit_rate) as avg_cache_hit_rate,
			SUM(cache_read_tokens) as total_cache_read_tokens,
			SUM(prompt_tokens) as total_prompt_tokens
		`).
		Where("created_at >= ? AND created_at <= ? AND is_warmup = ?", startTime, endTime, false).
		Scan(&userResult).Error

	if err != nil {
		return nil, err
	}

	// Get warmup cost
	warmupCost, err := GetWarmupCost(startTime, endTime)
	if err != nil {
		return nil, err
	}

	// Calculate net savings and ROI
	netSavings := userResult.TotalCostSaved - warmupCost
	roi := 0.0
	if warmupCost > 0 {
		roi = (userResult.TotalCostSaved / warmupCost) - 1 // ROI as percentage
	}

	// Calculate break-even analysis
	breakEvenPoint := 0.0
	if userResult.TotalRequests > 0 {
		breakEvenPoint = warmupCost / float64(userResult.TotalRequests)
	}

	return map[string]interface{}{
		// User request metrics
		"total_requests":         userResult.TotalRequests,
		"total_cost_saved":       userResult.TotalCostSaved,
		"total_cost_with_cache":  userResult.TotalCostWithCache,
		"total_cost_without":     userResult.TotalCostWithout,
		"avg_cache_hit_rate":     userResult.AvgCacheHitRate,
		"total_cache_read_tokens": userResult.TotalCacheReadTokens,
		"total_prompt_tokens":    userResult.TotalPromptTokens,

		// Warmup cost
		"warmup_cost": warmupCost,

		// ROI metrics
		"net_savings":      netSavings,
		"roi":              roi,
		"break_even_point": breakEvenPoint,

		// Efficiency indicators
		"is_cost_effective": netSavings > 0,
		"efficiency_ratio":  userResult.TotalCostSaved / (userResult.TotalCostWithCache + warmupCost),
	}, nil
}

// GetCacheTrendMetrics retrieves time-series trend data for cache performance
// Used for detecting cache efficiency degradation over time
// ECP-B2: KISS - simple bucketing by hour for trend analysis
func GetCacheTrendMetrics(startTime, endTime time.Time, bucketSize time.Duration) ([]map[string]interface{}, error) {
	var results []struct {
		TimeBucket      time.Time
		TotalRequests   int64
		AvgCacheHitRate float64
		TotalCostSaved  float64
	}

	// Use hourly buckets by default
	bucketFormat := "datetime(created_at, 'start of hour')"
	if bucketSize >= 24*time.Hour {
		bucketFormat = "date(created_at)"
	}

	err := DB.Model(&PromptCacheMetrics{}).
		Select(bucketFormat+" as time_bucket, COUNT(*) as total_requests, AVG(cache_hit_rate) as avg_cache_hit_rate, SUM(cost_saved) as total_cost_saved").
		Where("created_at >= ? AND created_at <= ? AND is_warmup = ?", startTime, endTime, false).
		Group("time_bucket").
		Order("time_bucket ASC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// Convert to map slice
	trends := make([]map[string]interface{}, len(results))
	for i, r := range results {
		trends[i] = map[string]interface{}{
			"timestamp":        r.TimeBucket.Unix(),
			"total_requests":   r.TotalRequests,
			"avg_cache_hit_rate": r.AvgCacheHitRate,
			"total_cost_saved": r.TotalCostSaved,
		}
	}

	return trends, nil
}