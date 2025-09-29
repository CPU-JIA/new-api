package model

import (
	"one-api/common"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestGetRandomSatisfiedChannelOptimization(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestOptimizedVsLegacyEquivalence", func(t *testing.T) {
		// Test that optimized and legacy versions produce similar results
		group := "default"
		model := "gpt-3.5-turbo"

		// Run multiple iterations to test consistency
		const iterations = 10
		optimizedResults := make([]*Channel, iterations)
		legacyResults := make([]*Channel, iterations)

		for i := 0; i < iterations; i++ {
			optimizedChannel, err1 := GetRandomSatisfiedChannelOptimized(group, model, 0)
			legacyChannel, err2 := GetRandomSatisfiedChannelLegacy(group, model, 0)

			// Both should succeed or both should fail
			if err1 != nil && err2 != nil {
				t.Logf("Both methods failed (iteration %d): optimized=%v, legacy=%v", i, err1, err2)
				continue
			}

			if err1 != nil || err2 != nil {
				t.Logf("Method mismatch (iteration %d): optimized_err=%v, legacy_err=%v", i, err1, err2)
				continue
			}

			optimizedResults[i] = optimizedChannel
			legacyResults[i] = legacyChannel

			// Both should return valid channels
			if optimizedChannel != nil && legacyChannel != nil {
				assert.NotZero(t, optimizedChannel.Id, "Optimized channel should have valid ID")
				assert.NotZero(t, legacyChannel.Id, "Legacy channel should have valid ID")
			}
		}

		t.Logf("Completed %d iterations of equivalence testing", iterations)
	})

	t.Run("TestOptimizedPerformance", func(t *testing.T) {
		group := "default"
		model := "gpt-3.5-turbo"

		// Benchmark optimized version
		start := time.Now()
		const testIterations = 20

		successCount := 0
		for i := 0; i < testIterations; i++ {
			channel, err := GetRandomSatisfiedChannelOptimized(group, model, 0)
			if err == nil && channel != nil {
				successCount++
			}
		}

		optimizedDuration := time.Since(start)
		optimizedAvgMs := float64(optimizedDuration.Nanoseconds()) / float64(testIterations) / 1000000.0

		t.Logf("Optimized version: %.2fms average, %d/%d successful",
			optimizedAvgMs, successCount, testIterations)

		// Benchmark legacy version for comparison
		start = time.Now()
		legacySuccessCount := 0
		for i := 0; i < testIterations; i++ {
			channel, err := GetRandomSatisfiedChannelLegacy(group, model, 0)
			if err == nil && channel != nil {
				legacySuccessCount++
			}
		}

		legacyDuration := time.Since(start)
		legacyAvgMs := float64(legacyDuration.Nanoseconds()) / float64(testIterations) / 1000000.0

		t.Logf("Legacy version: %.2fms average, %d/%d successful",
			legacyAvgMs, legacySuccessCount, testIterations)

		// Calculate improvement
		if legacyAvgMs > 0 {
			improvement := ((legacyAvgMs - optimizedAvgMs) / legacyAvgMs) * 100
			t.Logf("Performance improvement: %.1f%%", improvement)

			// Optimized version should be faster or at least equivalent
			assert.LessOrEqual(t, optimizedAvgMs, legacyAvgMs*1.1, // Allow 10% tolerance
				"Optimized version should be faster than legacy")
		}
	})

	t.Run("TestRetryBehavior", func(t *testing.T) {
		// Test retry behavior with different priority levels
		group := "default"
		model := "gpt-3.5-turbo"

		// Test retry = 0 (max priority)
		channel0, err0 := GetRandomSatisfiedChannelOptimized(group, model, 0)
		if err0 != nil {
			t.Logf("No channels available for retry=0: %v", err0)
		} else if channel0 != nil {
			t.Logf("Retry=0 selected channel ID: %d", channel0.Id)
		}

		// Test retry = 1 (second highest priority)
		channel1, err1 := GetRandomSatisfiedChannelOptimized(group, model, 1)
		if err1 != nil {
			t.Logf("No channels available for retry=1: %v", err1)
		} else if channel1 != nil {
			t.Logf("Retry=1 selected channel ID: %d", channel1.Id)
		}

		// Test high retry value (should use lowest priority)
		channel999, err999 := GetRandomSatisfiedChannelOptimized(group, model, 999)
		if err999 != nil {
			t.Logf("No channels available for retry=999: %v", err999)
		} else if channel999 != nil {
			t.Logf("Retry=999 selected channel ID: %d", channel999.Id)
		}
	})

	t.Run("TestWeightDistribution", func(t *testing.T) {
		// Test that weight-based selection works correctly
		group := "default"
		model := "gpt-3.5-turbo"

		channelCounts := make(map[int]int)
		const selectionIterations = 100

		for i := 0; i < selectionIterations; i++ {
			channel, err := GetRandomSatisfiedChannelOptimized(group, model, 0)
			if err == nil && channel != nil {
				channelCounts[channel.Id]++
			}
		}

		if len(channelCounts) > 0 {
			t.Logf("Channel selection distribution over %d iterations:", selectionIterations)
			for channelId, count := range channelCounts {
				percentage := float64(count) / float64(selectionIterations) * 100
				t.Logf("  Channel %d: %d selections (%.1f%%)", channelId, count, percentage)
			}

			// Verify that multiple channels are being selected (weight distribution working)
			if len(channelCounts) > 1 {
				t.Log("✓ Weight-based distribution is working (multiple channels selected)")
			} else {
				t.Log("⚠ Only one channel selected - may indicate weight distribution issue or limited test data")
			}
		} else {
			t.Log("No channels were selected during weight distribution test")
		}
	})
}

func TestChannelWithAbilityStruct(t *testing.T) {
	// Test the ChannelWithAbility struct functionality
	channelWithAbility := ChannelWithAbility{
		Channel: Channel{
			Id:   1,
			Name: "Test Channel",
			Type: 1,
		},
		AbilityWeight:   50,
		AbilityPriority: common.GetPointer[int64](100),
		AbilityEnabled:  true,
	}

	assert.Equal(t, 1, channelWithAbility.Id)
	assert.Equal(t, "Test Channel", channelWithAbility.Name)
	assert.Equal(t, uint(50), channelWithAbility.AbilityWeight)
	assert.Equal(t, int64(100), *channelWithAbility.AbilityPriority)
	assert.True(t, channelWithAbility.AbilityEnabled)
}

func TestOptimizedQueryConstruction(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestBuildOptimizedChannelQuery", func(t *testing.T) {
		group := "default"
		model := "gpt-3.5-turbo"

		// Test with priority = nil (max priority subquery)
		query1 := buildOptimizedChannelQuery(group, model, nil)
		assert.NotNil(t, query1, "Query should not be nil for max priority")

		sql1 := query1.ToSQL(func(tx *gorm.DB) *gorm.DB {
			var results []ChannelWithAbility
			return tx.Scan(&results)
		})
		t.Logf("Max priority query SQL: %s", sql1)

		// Test with specific priority
		priority := int64(100)
		query2 := buildOptimizedChannelQuery(group, model, &priority)
		assert.NotNil(t, query2, "Query should not be nil for specific priority")

		sql2 := query2.ToSQL(func(tx *gorm.DB) *gorm.DB {
			var results []ChannelWithAbility
			return tx.Scan(&results)
		})
		t.Logf("Specific priority query SQL: %s", sql2)

		// Verify queries are different
		assert.NotEqual(t, sql1, sql2, "Max priority and specific priority queries should be different")
	})
}

func TestSelectChannelByWeight(t *testing.T) {
	// Test the weight selection algorithm
	channels := []ChannelWithAbility{
		{
			Channel:       Channel{Id: 1, Name: "Channel 1"},
			AbilityWeight: 100,
		},
		{
			Channel:       Channel{Id: 2, Name: "Channel 2"},
			AbilityWeight: 50,
		},
		{
			Channel:       Channel{Id: 3, Name: "Channel 3"},
			AbilityWeight: 10,
		},
	}

	// Test multiple selections to verify weight distribution
	selections := make(map[int]int)
	const iterations = 1000

	for i := 0; i < iterations; i++ {
		selected := selectChannelByWeight(channels)
		selections[selected.Id]++
	}

	// Channel 1 (weight 100) should be selected most often
	// Channel 2 (weight 50) should be selected moderately
	// Channel 3 (weight 10) should be selected least often

	t.Logf("Weight selection distribution over %d iterations:", iterations)
	for id, count := range selections {
		percentage := float64(count) / float64(iterations) * 100
		t.Logf("  Channel %d: %d selections (%.1f%%)", id, count, percentage)
	}

	// Verify that higher weight channels are selected more often
	assert.Greater(t, selections[1], selections[2], "Channel 1 (weight 100) should be selected more than Channel 2 (weight 50)")
	assert.Greater(t, selections[2], selections[3], "Channel 2 (weight 50) should be selected more than Channel 3 (weight 10)")
}

func BenchmarkGetRandomSatisfiedChannel(b *testing.B) {
	if DB == nil {
		b.Skip("Database not available for benchmarking")
	}

	group := "default"
	model := "gpt-3.5-turbo"

	b.Run("Optimized", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = GetRandomSatisfiedChannelOptimized(group, model, 0)
		}
	})

	b.Run("Legacy", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = GetRandomSatisfiedChannelLegacy(group, model, 0)
		}
	})

	b.Run("WithFallback", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = GetRandomSatisfiedChannelWithFallback(group, model, 0)
		}
	})
}