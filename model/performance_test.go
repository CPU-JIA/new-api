package model

import (
	"one-api/common"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseIndexOptimization(t *testing.T) {
	// Skip if database is not available
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestIndexCreation", func(t *testing.T) {
		// Test that critical indexes exist
		criticalIndexes := []struct {
			tableName string
			indexName string
		}{
			{"abilities", "idx_abilities_group_model_enabled_priority_weight"},
			{"abilities", "idx_abilities_channel_enabled"},
			{"channels", "idx_channels_status_type_priority"},
			{"channels", "idx_channels_tag_status"},
		}

		for _, idx := range criticalIndexes {
			exists, err := CheckIndexExists(DB, idx.tableName, idx.indexName)
			require.NoError(t, err, "Failed to check index %s existence", idx.indexName)
			assert.True(t, exists, "Critical index %s should exist", idx.indexName)
		}
	})

	t.Run("TestQueryPerformance", func(t *testing.T) {
		// Run performance benchmarks
		metrics := PerformanceBenchmark(DB)

		// Verify that all benchmarks completed
		expectedBenchmarks := []string{
			"channel_selection",
			"ability_status_update",
			"channel_filtering",
			"group_model_lookup",
			"tag_operations",
			"complex_joins",
		}

		for _, benchmarkName := range expectedBenchmarks {
			metric, exists := metrics[benchmarkName]
			assert.True(t, exists, "Benchmark %s should exist", benchmarkName)
			if exists {
				assert.NotZero(t, metric.ExecutionTime, "Benchmark %s should have execution time", benchmarkName)
				t.Logf("Benchmark %s: %.2fms (%d rows)",
					benchmarkName,
					float64(metric.ExecutionTime.Nanoseconds())/1000000.0,
					metric.RowCount)
			}
		}
	})

	t.Run("TestChannelSelectionPerformance", func(t *testing.T) {
		// Test the critical GetRandomSatisfiedChannel pattern
		start := time.Now()

		var abilities []Ability
		err := DB.Where("enabled = ?", true).
			Order("priority DESC, weight DESC").
			Limit(10).
			Find(&abilities).Error

		duration := time.Since(start)

		require.NoError(t, err, "Channel selection query should not fail")
		t.Logf("Channel selection query took: %.2fms",
			float64(duration.Nanoseconds())/1000000.0)

		// With proper indexes, this should be fast (under 50ms for reasonable data sizes)
		if duration.Milliseconds() > 100 {
			t.Logf("Warning: Channel selection query took %dms, may need index optimization",
				duration.Milliseconds())
		}
	})

	t.Run("TestComplexJoinPerformance", func(t *testing.T) {
		// Test the complex join pattern used in channel-ability operations
		start := time.Now()

		var results []struct {
			ChannelID int    `gorm:"column:channel_id"`
			Model     string `gorm:"column:model"`
			Status    int    `gorm:"column:status"`
		}

		err := DB.Table("abilities").
			Select("abilities.channel_id, abilities.model, channels.status").
			Joins("JOIN channels ON abilities.channel_id = channels.id").
			Where("abilities.enabled = ? AND channels.status = ?", true, 1).
			Limit(20).
			Scan(&results).Error

		duration := time.Since(start)

		require.NoError(t, err, "Complex join query should not fail")
		t.Logf("Complex join query took: %.2fms",
			float64(duration.Nanoseconds())/1000000.0)

		// Log results for verification
		t.Logf("Found %d channel-ability relationships", len(results))
	})

	t.Run("TestIndexUtilization", func(t *testing.T) {
		// Get index utilization statistics
		stats := GetIndexUtilizationStats(DB)
		assert.NotEmpty(t, stats, "Should have index utilization statistics")

		// Verify timestamp is present
		timestamp, exists := stats["timestamp"]
		assert.True(t, exists, "Statistics should have timestamp")
		assert.NotZero(t, timestamp, "Timestamp should not be zero")
	})
}

func TestAbilityQueryOptimization(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestGroupModelLookup", func(t *testing.T) {
		// Test the group-based model lookup pattern
		start := time.Now()

		var models []string
		groupCol := "`group`"
		if common.UsingPostgreSQL {
			groupCol = `"group"`
		}

		err := DB.Model(&Ability{}).
			Where(groupCol+" = ? AND enabled = ?", "default", true).
			Distinct("model").
			Pluck("model", &models).Error

		duration := time.Since(start)

		require.NoError(t, err, "Group model lookup should not fail")
		t.Logf("Group model lookup took: %.2fms (found %d models)",
			float64(duration.Nanoseconds())/1000000.0, len(models))
	})

	t.Run("TestAbilityStatusUpdate", func(t *testing.T) {
		// Test ability status lookup pattern (used in channel updates)
		start := time.Now()

		var count int64
		err := DB.Model(&Ability{}).
			Where("channel_id > ? AND enabled = ?", 0, true).
			Count(&count).Error

		duration := time.Since(start)

		require.NoError(t, err, "Ability status count should not fail")
		t.Logf("Ability status count took: %.2fms (found %d abilities)",
			float64(duration.Nanoseconds())/1000000.0, count)
	})
}

func BenchmarkComplexJoin(b *testing.B) {
	if DB == nil {
		b.Skip("Database not available for benchmarking")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var results []struct {
			ChannelID int    `gorm:"column:channel_id"`
			Model     string `gorm:"column:model"`
			Status    int    `gorm:"column:status"`
		}

		DB.Table("abilities").
			Select("abilities.channel_id, abilities.model, channels.status").
			Joins("JOIN channels ON abilities.channel_id = channels.id").
			Where("abilities.enabled = ? AND channels.status = ?", true, 1).
			Limit(20).
			Scan(&results)
	}
}

func BenchmarkGroupModelLookup(b *testing.B) {
	if DB == nil {
		b.Skip("Database not available for benchmarking")
	}

	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var models []string
		DB.Model(&Ability{}).
			Where(groupCol+" = ? AND enabled = ?", "default", true).
			Distinct("model").
			Pluck("model", &models)
	}
}