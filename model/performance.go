package model

import (
	"fmt"
	"one-api/common"
	"time"

	"gorm.io/gorm"
)

// QueryPerformanceMetrics tracks database query performance
type QueryPerformanceMetrics struct {
	QueryName     string        `json:"query_name"`
	ExecutionTime time.Duration `json:"execution_time_ms"`
	RowCount      int64         `json:"row_count"`
	Timestamp     time.Time     `json:"timestamp"`
	QuerySQL      string        `json:"query_sql,omitempty"`
}

// PerformanceBenchmark runs comprehensive database performance tests
func PerformanceBenchmark(db *gorm.DB) map[string]*QueryPerformanceMetrics {
	results := make(map[string]*QueryPerformanceMetrics)

	// Test 1: GetRandomSatisfiedChannel simulation (most critical)
	results["channel_selection"] = benchmarkChannelSelection(db)

	// Test 2: Ability status updates
	results["ability_status_update"] = benchmarkAbilityStatusUpdate(db)

	// Test 3: Channel filtering by status and type
	results["channel_filtering"] = benchmarkChannelFiltering(db)

	// Test 4: Group-based model lookup
	results["group_model_lookup"] = benchmarkGroupModelLookup(db)

	// Test 5: Tag-based operations
	results["tag_operations"] = benchmarkTagOperations(db)

	// Test 6: Complex join queries
	results["complex_joins"] = benchmarkComplexJoins(db)

	return results
}

// benchmarkChannelSelection simulates the critical GetRandomSatisfiedChannel query
func benchmarkChannelSelection(db *gorm.DB) *QueryPerformanceMetrics {
	start := time.Now()

	// Simulate the most critical query pattern from GetRandomSatisfiedChannel
	var abilities []Ability
	query := db.Where("enabled = ? AND priority > ?", true, 0).
		Order("priority DESC, weight DESC").
		Limit(10)

	// Track the SQL being executed
	sqlQuery := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(&abilities)
	})

	err := query.Find(&abilities).Error
	duration := time.Since(start)

	metric := &QueryPerformanceMetrics{
		QueryName:     "Channel Selection (Critical Path)",
		ExecutionTime: duration,
		RowCount:      int64(len(abilities)),
		Timestamp:     time.Now(),
		QuerySQL:      sqlQuery,
	}

	if err != nil {
		common.SysLog(fmt.Sprintf("Benchmark channel_selection error: %v", err))
	}

	return metric
}

// benchmarkAbilityStatusUpdate tests ability status update performance
func benchmarkAbilityStatusUpdate(db *gorm.DB) *QueryPerformanceMetrics {
	start := time.Now()

	// Simulate ability status lookup (common in channel status updates)
	var count int64
	query := db.Model(&Ability{}).Where("channel_id > ? AND enabled = ?", 0, true)

	sqlQuery := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Count(&count)
	})

	err := query.Count(&count).Error
	duration := time.Since(start)

	metric := &QueryPerformanceMetrics{
		QueryName:     "Ability Status Update Lookup",
		ExecutionTime: duration,
		RowCount:      count,
		Timestamp:     time.Now(),
		QuerySQL:      sqlQuery,
	}

	if err != nil {
		common.SysLog(fmt.Sprintf("Benchmark ability_status_update error: %v", err))
	}

	return metric
}

// benchmarkChannelFiltering tests channel filtering performance
func benchmarkChannelFiltering(db *gorm.DB) *QueryPerformanceMetrics {
	start := time.Now()

	// Simulate common channel filtering patterns
	var channels []Channel
	query := db.Where("status = ? AND type > ?", 1, 0).
		Order("priority DESC").
		Limit(20)

	sqlQuery := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Find(&channels)
	})

	err := query.Find(&channels).Error
	duration := time.Since(start)

	metric := &QueryPerformanceMetrics{
		QueryName:     "Channel Filtering",
		ExecutionTime: duration,
		RowCount:      int64(len(channels)),
		Timestamp:     time.Now(),
		QuerySQL:      sqlQuery,
	}

	if err != nil {
		common.SysLog(fmt.Sprintf("Benchmark channel_filtering error: %v", err))
	}

	return metric
}

// benchmarkGroupModelLookup tests group-based model availability checks
func benchmarkGroupModelLookup(db *gorm.DB) *QueryPerformanceMetrics {
	start := time.Now()

	// Simulate group-based model lookup (common in API validation)
	var models []string
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	query := db.Model(&Ability{}).
		Where(groupCol+" = ? AND enabled = ?", "default", true).
		Distinct("model")

	sqlQuery := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Pluck("model", &models)
	})

	err := query.Pluck("model", &models).Error
	duration := time.Since(start)

	metric := &QueryPerformanceMetrics{
		QueryName:     "Group Model Lookup",
		ExecutionTime: duration,
		RowCount:      int64(len(models)),
		Timestamp:     time.Now(),
		QuerySQL:      sqlQuery,
	}

	if err != nil {
		common.SysLog(fmt.Sprintf("Benchmark group_model_lookup error: %v", err))
	}

	return metric
}

// benchmarkTagOperations tests tag-based operations performance
func benchmarkTagOperations(db *gorm.DB) *QueryPerformanceMetrics {
	start := time.Now()

	// Simulate tag-based operations (common in batch operations)
	var count int64
	query := db.Model(&Channel{}).Where("tag IS NOT NULL AND status = ?", 1)

	sqlQuery := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Count(&count)
	})

	err := query.Count(&count).Error
	duration := time.Since(start)

	metric := &QueryPerformanceMetrics{
		QueryName:     "Tag-based Operations",
		ExecutionTime: duration,
		RowCount:      count,
		Timestamp:     time.Now(),
		QuerySQL:      sqlQuery,
	}

	if err != nil {
		common.SysLog(fmt.Sprintf("Benchmark tag_operations error: %v", err))
	}

	return metric
}

// benchmarkComplexJoins tests complex join performance (abilities + channels)
func benchmarkComplexJoins(db *gorm.DB) *QueryPerformanceMetrics {
	start := time.Now()

	// Simulate complex joins used in reporting and analytics
	var results []struct {
		ChannelID int    `gorm:"column:channel_id"`
		Model     string `gorm:"column:model"`
		Status    int    `gorm:"column:status"`
	}

	query := db.Table("abilities").
		Select("abilities.channel_id, abilities.model, channels.status").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.enabled = ? AND channels.status = ?", true, 1).
		Limit(50)

	sqlQuery := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Scan(&results)
	})

	err := query.Scan(&results).Error
	duration := time.Since(start)

	metric := &QueryPerformanceMetrics{
		QueryName:     "Complex Join Queries",
		ExecutionTime: duration,
		RowCount:      int64(len(results)),
		Timestamp:     time.Now(),
		QuerySQL:      sqlQuery,
	}

	if err != nil {
		common.SysLog(fmt.Sprintf("Benchmark complex_joins error: %v", err))
	}

	return metric
}

// LogPerformanceMetrics logs performance benchmark results
func LogPerformanceMetrics(metrics map[string]*QueryPerformanceMetrics) {
	common.SysLog("=== Database Performance Benchmark Results ===")

	totalTime := time.Duration(0)
	for name, metric := range metrics {
		totalTime += metric.ExecutionTime
		common.SysLog(fmt.Sprintf("%-25s: %6.2fms (%d rows)",
			name,
			float64(metric.ExecutionTime.Nanoseconds())/1000000.0,
			metric.RowCount,
		))

		if common.DebugEnabled && metric.QuerySQL != "" {
			common.SysLog(fmt.Sprintf("  SQL: %s", metric.QuerySQL))
		}
	}

	common.SysLog(fmt.Sprintf("Total benchmark time: %.2fms",
		float64(totalTime.Nanoseconds())/1000000.0))
	common.SysLog("============================================")
}

// RunPerformanceValidation runs a complete performance validation
func RunPerformanceValidation() {
	if DB == nil {
		common.SysLog("Database not initialized, skipping performance validation")
		return
	}

	common.SysLog("Starting database performance validation...")

	// Run the benchmark suite
	metrics := PerformanceBenchmark(DB)

	// Log the results
	LogPerformanceMetrics(metrics)

	// Check for potential performance issues
	var warnings []string
	for name, metric := range metrics {
		// Flag queries taking longer than reasonable thresholds
		if metric.ExecutionTime.Milliseconds() > 100 {
			warnings = append(warnings, fmt.Sprintf("%s: %dms (slow)",
				name, metric.ExecutionTime.Milliseconds()))
		}
	}

	if len(warnings) > 0 {
		common.SysLog("Performance warnings detected:")
		for _, warning := range warnings {
			common.SysLog("  - " + warning)
		}
	} else {
		common.SysLog("All performance benchmarks passed ✓")
	}
}

// GetIndexUtilizationStats returns index utilization statistics
func GetIndexUtilizationStats(db *gorm.DB) map[string]interface{} {
	stats := make(map[string]interface{})

	if common.UsingMySQL {
		// MySQL index usage statistics
		var indexStats []map[string]interface{}
		db.Raw(`
			SELECT
				TABLE_NAME as table_name,
				INDEX_NAME as index_name,
				CARDINALITY as cardinality,
				CASE WHEN NON_UNIQUE = 0 THEN 'UNIQUE' ELSE 'NON_UNIQUE' END as uniqueness
			FROM INFORMATION_SCHEMA.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME IN ('channels', 'abilities')
			AND INDEX_NAME != 'PRIMARY'
			ORDER BY TABLE_NAME, CARDINALITY DESC
		`).Scan(&indexStats)
		stats["mysql_index_stats"] = indexStats

	} else if common.UsingPostgreSQL {
		// PostgreSQL index usage statistics
		var indexStats []map[string]interface{}
		db.Raw(`
			SELECT
				schemaname,
				tablename,
				indexname,
				idx_tup_read,
				idx_tup_fetch
			FROM pg_stat_user_indexes
			WHERE schemaname = 'public'
			AND tablename IN ('channels', 'abilities')
			ORDER BY tablename, idx_tup_read DESC
		`).Scan(&indexStats)
		stats["postgresql_index_stats"] = indexStats
	}

	stats["timestamp"] = time.Now().Unix()
	return stats
}