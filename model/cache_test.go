package model

import (
	"context"
	"fmt"
	"one-api/common"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache(t *testing.T) {
	t.Run("TestMemoryCacheBasicOperations", func(t *testing.T) {
		cache := NewMemoryCache(100, 5*time.Minute)
		defer cache.Close()

		// Test Set and Get
		entry := &CacheEntry{
			Data:      "test data",
			Timestamp: time.Now(),
			TTL:       1 * time.Minute,
			Layer:     L1Layer,
		}

		cache.Set("test_key", entry)

		retrieved, found := cache.Get("test_key")
		assert.True(t, found, "Should find cached entry")
		assert.NotNil(t, retrieved, "Retrieved entry should not be nil")
		assert.Equal(t, "test data", retrieved.Data, "Data should match")

		// Test Delete
		cache.Delete("test_key")
		_, found = cache.Get("test_key")
		assert.False(t, found, "Should not find deleted entry")
	})

	t.Run("TestMemoryCacheExpiration", func(t *testing.T) {
		cache := NewMemoryCache(100, 100*time.Millisecond)
		defer cache.Close()

		entry := &CacheEntry{
			Data:      "expiring data",
			Timestamp: time.Now(),
			TTL:       50 * time.Millisecond, // Very short TTL
			Layer:     L1Layer,
		}

		cache.Set("expiring_key", entry)

		// Should find immediately
		_, found := cache.Get("expiring_key")
		assert.True(t, found, "Should find entry immediately")

		// Wait for expiration
		time.Sleep(60 * time.Millisecond)

		_, found = cache.Get("expiring_key")
		assert.False(t, found, "Should not find expired entry")
	})

	t.Run("TestMemoryCacheLRUEviction", func(t *testing.T) {
		cache := NewMemoryCache(2, 5*time.Minute) // Small cache for testing eviction
		defer cache.Close()

		// Add entries to fill cache
		entry1 := &CacheEntry{Data: "data1", Timestamp: time.Now(), TTL: 5 * time.Minute, Layer: L1Layer}
		entry2 := &CacheEntry{Data: "data2", Timestamp: time.Now(), TTL: 5 * time.Minute, Layer: L1Layer}
		entry3 := &CacheEntry{Data: "data3", Timestamp: time.Now(), TTL: 5 * time.Minute, Layer: L1Layer}

		cache.Set("key1", entry1)
		cache.Set("key2", entry2)
		cache.Set("key3", entry3) // Should evict key1 (LRU)

		_, found1 := cache.Get("key1")
		_, found2 := cache.Get("key2")
		_, found3 := cache.Get("key3")

		assert.False(t, found1, "key1 should be evicted")
		assert.True(t, found2, "key2 should exist")
		assert.True(t, found3, "key3 should exist")
	})

	t.Run("TestMemoryCachePatternDelete", func(t *testing.T) {
		cache := NewMemoryCache(100, 5*time.Minute)
		defer cache.Close()

		// Add test entries
		entry := &CacheEntry{Data: "test", Timestamp: time.Now(), TTL: 5 * time.Minute, Layer: L1Layer}
		cache.Set("gm:default:gpt-3.5", entry)
		cache.Set("gm:default:gpt-4", entry)
		cache.Set("gm:premium:gpt-4", entry)
		cache.Set("ch:123", entry)

		// Delete pattern
		cache.DeletePattern("gm:default:*")

		// Check results
		_, found1 := cache.Get("gm:default:gpt-3.5")
		_, found2 := cache.Get("gm:default:gpt-4")
		_, found3 := cache.Get("gm:premium:gpt-4")
		_, found4 := cache.Get("ch:123")

		assert.False(t, found1, "gm:default:gpt-3.5 should be deleted")
		assert.False(t, found2, "gm:default:gpt-4 should be deleted")
		assert.True(t, found3, "gm:premium:gpt-4 should remain")
		assert.True(t, found4, "ch:123 should remain")
	})

	t.Run("TestMemoryCacheStats", func(t *testing.T) {
		cache := NewMemoryCache(100, 5*time.Minute)
		defer cache.Close()

		// Add some entries
		entry := &CacheEntry{Data: "test", Timestamp: time.Now(), TTL: 5 * time.Minute, Layer: L1Layer}
		cache.Set("key1", entry)
		cache.Set("key2", entry)

		assert.Equal(t, 2, cache.Size(), "Cache should contain 2 entries")
		assert.Greater(t, cache.MemoryUsage(), int64(0), "Memory usage should be > 0")

		stats := cache.GetStats()
		assert.Equal(t, 2, stats["size"], "Stats should show 2 entries")
		assert.Equal(t, 100, stats["max_items"], "Stats should show max_items")
	})
}

func TestLayeredCacheManager(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestCacheManagerCreation", func(t *testing.T) {
		config := DefaultCacheConfig()
		config.RedisCacheEnabled = false // Disable Redis for unit tests

		manager, err := NewLayeredCacheManager(config)
		require.NoError(t, err, "Should create cache manager successfully")
		require.NotNil(t, manager, "Cache manager should not be nil")

		// Cleanup
		manager.Shutdown(context.Background())
	})

	t.Run("TestCacheManagerChannelOperations", func(t *testing.T) {
		// Create test channel
		testChannel := &Channel{
			Id:       9001,
			Name:     "Cache Test Channel",
			Models:   "gpt-3.5-turbo",
			Group:    "default",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}

		// Insert test channel
		err := DB.Create(testChannel).Error
		require.NoError(t, err, "Should create test channel")

		defer func() {
			DB.Unscoped().Delete(testChannel)
		}()

		config := DefaultCacheConfig()
		config.RedisCacheEnabled = false // Disable Redis for unit tests
		config.WarmupEnabled = false    // Disable warmup for cleaner tests

		manager, err := NewLayeredCacheManager(config)
		require.NoError(t, err, "Should create cache manager")
		defer manager.Shutdown(context.Background())

		// Test GetChannel
		channel, err := manager.GetChannel(9001)
		require.NoError(t, err, "Should get channel successfully")
		require.NotNil(t, channel, "Channel should not be nil")
		assert.Equal(t, "Cache Test Channel", channel.Name, "Channel name should match")

		// Test cache hit (should be faster on second call)
		start := time.Now()
		channel2, err := manager.GetChannel(9001)
		duration := time.Since(start)
		require.NoError(t, err, "Should get cached channel")
		assert.Equal(t, channel.Name, channel2.Name, "Cached channel should match")
		assert.Less(t, duration.Milliseconds(), int64(10), "Cached lookup should be fast")

		// Test metrics
		metrics := manager.GetMetrics()
		assert.Greater(t, metrics.L1Hits, int64(0), "Should have L1 cache hits")
	})

	t.Run("TestCacheInvalidation", func(t *testing.T) {
		// Create test channel
		testChannel := &Channel{
			Id:       9002,
			Name:     "Invalidation Test Channel",
			Models:   "gpt-3.5-turbo",
			Group:    "default",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}

		err := DB.Create(testChannel).Error
		require.NoError(t, err)
		defer DB.Unscoped().Delete(testChannel)

		config := DefaultCacheConfig()
		config.RedisCacheEnabled = false
		config.WarmupEnabled = false

		manager, err := NewLayeredCacheManager(config)
		require.NoError(t, err)
		defer manager.Shutdown(context.Background())

		// Cache the channel
		_, err = manager.GetChannel(9002)
		require.NoError(t, err)

		// Invalidate the channel
		err = manager.InvalidateChannel(9002)
		require.NoError(t, err, "Should invalidate channel successfully")

		// Verify invalidation
		metrics := manager.GetMetrics()
		assert.Greater(t, metrics.InvalidationCount, int64(0), "Should have invalidation count")
	})

	t.Run("TestHealthCheck", func(t *testing.T) {
		config := DefaultCacheConfig()
		config.RedisCacheEnabled = false
		config.MetricsEnabled = true

		manager, err := NewLayeredCacheManager(config)
		require.NoError(t, err)
		defer manager.Shutdown(context.Background())

		// Test health check
		err = manager.HealthCheck()
		assert.NoError(t, err, "Health check should pass")

		metrics := manager.GetMetrics()
		assert.True(t, metrics.IsHealthy, "Cache should be healthy")
		assert.NotZero(t, metrics.LastHealthCheck, "Last health check should be set")
	})
}

func TestCacheWarmer(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestCacheWarmerCreation", func(t *testing.T) {
		config := DefaultCacheWarmerConfig()
		warmer := NewCacheWarmer(config)

		assert.NotNil(t, warmer, "Cache warmer should not be nil")
		assert.Equal(t, 4, warmer.config.Workers, "Should have correct number of workers")
		assert.Equal(t, 50, warmer.config.BatchSize, "Should have correct batch size")
	})

	t.Run("TestWarmupTaskGeneration", func(t *testing.T) {
		// Create test data
		testChannel := &Channel{
			Id:       9003,
			Name:     "Warmup Test Channel",
			Models:   "gpt-3.5-turbo",
			Group:    "default",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}

		err := DB.Create(testChannel).Error
		require.NoError(t, err)
		defer DB.Unscoped().Delete(testChannel)

		// Add abilities
		err = testChannel.UpdateAbilities(nil)
		require.NoError(t, err)
		defer DB.Where("channel_id = ?", 9003).Delete(&Ability{})

		config := DefaultCacheWarmerConfig()
		warmer := NewCacheWarmer(config)

		// Generate tasks
		tasks, err := warmer.generateWarmupTasks()
		require.NoError(t, err, "Should generate warmup tasks")
		assert.Greater(t, len(tasks), 0, "Should generate at least one task")

		// Verify task types
		taskTypes := make(map[string]int)
		for _, task := range tasks {
			taskTypes[task.Type]++
		}

		if len(taskTypes) > 0 {
			t.Logf("Generated task types: %v", taskTypes)
		}
	})

	t.Run("TestWarmupExecution", func(t *testing.T) {
		// Create minimal test setup
		testChannel := &Channel{
			Id:       9004,
			Name:     "Warmup Execution Test",
			Models:   "gpt-3.5-turbo",
			Group:    "default",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}

		err := DB.Create(testChannel).Error
		require.NoError(t, err)
		defer DB.Unscoped().Delete(testChannel)

		err = testChannel.UpdateAbilities(nil)
		require.NoError(t, err)
		defer DB.Where("channel_id = ?", 9004).Delete(&Ability{})

		// Create cache manager
		config := DefaultCacheConfig()
		config.RedisCacheEnabled = false
		config.WarmupEnabled = true
		config.WarmupTimeout = 10 * time.Second

		manager, err := NewLayeredCacheManager(config)
		require.NoError(t, err)
		defer manager.Shutdown(context.Background())

		// Perform warmup
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		err = manager.WarmupCache(ctx)
		// Note: Warmup might fail in test environment, but it shouldn't panic
		if err != nil {
			t.Logf("Warmup completed with error (expected in test env): %v", err)
		} else {
			t.Log("Warmup completed successfully")
		}

		// Check if warmup completed
		assert.True(t, manager.IsWarmupComplete(), "Warmup should be marked as complete")

		// Verify metrics
		metrics := manager.GetMetrics()
		assert.Greater(t, metrics.WarmupCount, int64(0), "Should have warmup count")
		assert.NotZero(t, metrics.LastWarmupTime, "Last warmup time should be set")
	})
}

func TestCacheIntegration(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestFullCacheWorkflow", func(t *testing.T) {
		// Create test data
		testChannel := &Channel{
			Id:       9005,
			Name:     "Integration Test Channel",
			Models:   "gpt-3.5-turbo,gpt-4",
			Group:    "default,premium",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}

		err := DB.Create(testChannel).Error
		require.NoError(t, err)
		defer DB.Unscoped().Delete(testChannel)

		err = testChannel.UpdateAbilities(nil)
		require.NoError(t, err)
		defer DB.Where("channel_id = ?", 9005).Delete(&Ability{})

		// Create cache manager with minimal configuration
		config := DefaultCacheConfig()
		config.RedisCacheEnabled = false
		config.WarmupEnabled = true
		config.WarmupTimeout = 5 * time.Second
		config.MemoryCacheEnabled = true
		config.MaxMemoryItems = 1000

		manager, err := NewLayeredCacheManager(config)
		require.NoError(t, err)
		defer manager.Shutdown(context.Background())

		// Test complete workflow
		// 1. Initial cache miss
		start := time.Now()
		channel, err := manager.GetChannel(9005)
		firstCallDuration := time.Since(start)
		require.NoError(t, err)
		assert.Equal(t, "Integration Test Channel", channel.Name)

		// 2. Cache hit (should be faster)
		start = time.Now()
		cachedChannel, err := manager.GetChannel(9005)
		secondCallDuration := time.Since(start)
		require.NoError(t, err)
		assert.Equal(t, channel.Name, cachedChannel.Name)

		// Cache hit should be significantly faster
		if firstCallDuration > time.Millisecond {
			assert.Less(t, secondCallDuration, firstCallDuration/2,
				"Cached call should be significantly faster than DB call")
		}

		// 3. Test channel selection caching
		ctx := &gin.Context{}
		selectedChannel, group, err := manager.GetRandomSatisfiedChannel(ctx, "default", "gpt-3.5-turbo", 0)
		if err == nil && selectedChannel != nil {
			assert.NotNil(t, selectedChannel, "Should get a channel")
			assert.NotEmpty(t, group, "Should get a group")
		}

		// 4. Test invalidation
		err = manager.InvalidateChannel(9005)
		require.NoError(t, err)

		// 5. Test metrics
		metrics := manager.GetMetrics()
		assert.Greater(t, metrics.L1Hits+metrics.L2Hits+metrics.Misses, int64(0),
			"Should have recorded cache operations")
		assert.GreaterOrEqual(t, metrics.HitRate, float64(0), "Hit rate should be >= 0")
		assert.LessOrEqual(t, metrics.HitRate, float64(1), "Hit rate should be <= 1")

		// 6. Test health check
		err = manager.HealthCheck()
		assert.NoError(t, err, "Health check should pass")

		t.Logf("Cache metrics: L1Hits=%d, L2Hits=%d, Misses=%d, HitRate=%.2f",
			metrics.L1Hits, metrics.L2Hits, metrics.Misses, metrics.HitRate)
	})
}

// Benchmark tests for performance validation
func BenchmarkMemoryCache(b *testing.B) {
	cache := NewMemoryCache(10000, 5*time.Minute)
	defer cache.Close()

	entry := &CacheEntry{
		Data:      "benchmark data",
		Timestamp: time.Now(),
		TTL:       5 * time.Minute,
		Layer:     L1Layer,
	}

	b.Run("Set", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Set(fmt.Sprintf("key_%d", i), entry)
		}
	})

	// Pre-populate cache for get benchmark
	for i := 0; i < 1000; i++ {
		cache.Set(fmt.Sprintf("get_key_%d", i), entry)
	}

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cache.Get(fmt.Sprintf("get_key_%d", i%1000))
		}
	})
}

func BenchmarkCacheManager(b *testing.B) {
	if DB == nil {
		b.Skip("Database not available for benchmarking")
	}

	// Create test channel
	testChannel := &Channel{
		Id:       9999,
		Name:     "Benchmark Channel",
		Models:   "gpt-3.5-turbo",
		Group:    "default",
		Status:   common.ChannelStatusEnabled,
		Priority: common.GetPointer[int64](100),
	}

	DB.Create(testChannel)
	defer DB.Unscoped().Delete(testChannel)

	config := DefaultCacheConfig()
	config.RedisCacheEnabled = false
	config.WarmupEnabled = false

	manager, err := NewLayeredCacheManager(config)
	if err != nil {
		b.Fatalf("Failed to create cache manager: %v", err)
	}
	defer manager.Shutdown(context.Background())

	b.Run("GetChannel", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			manager.GetChannel(9999)
		}
	})
}