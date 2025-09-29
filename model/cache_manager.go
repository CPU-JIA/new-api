package model

import (
	"context"
	"fmt"
	"one-api/common"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// CacheManager defines the interface for the distributed cache system
type CacheManager interface {
	// Core cache operations
	GetChannel(id int) (*Channel, error)
	GetRandomSatisfiedChannel(ctx *gin.Context, group, model string, retry int) (*Channel, string, error)

	// Cache invalidation
	InvalidateChannel(id int) error
	InvalidateGroup(group string) error
	InvalidateAll() error

	// Cache warming and lifecycle
	WarmupCache(ctx context.Context) error
	IsWarmupComplete() bool

	// Metrics and health
	GetMetrics() *CacheMetrics
	HealthCheck() error

	// Event-driven updates
	OnChannelUpdate(channel *Channel) error
	OnChannelStatusChange(id int, status int) error

	// Lifecycle management
	Shutdown(ctx context.Context) error
}

// CacheLayer represents different cache layers
type CacheLayer string

const (
	L1Layer CacheLayer = "L1" // Memory cache
	L2Layer CacheLayer = "L2" // Redis cache
)

// CacheConfig holds configuration for the cache system
type CacheConfig struct {
	// Memory cache settings
	MemoryCacheEnabled bool
	MaxMemoryItems     int
	L1TTL              time.Duration

	// Redis cache settings
	RedisCacheEnabled  bool
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	L2TTL              time.Duration

	// Warming settings
	WarmupEnabled      bool
	WarmupWorkers      int
	WarmupBatchSize    int
	WarmupTimeout      time.Duration

	// Monitoring settings
	MetricsEnabled     bool
	HealthCheckInterval time.Duration
}

// DefaultCacheConfig returns sensible default configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MemoryCacheEnabled:  true,
		MaxMemoryItems:      10000,
		L1TTL:              5 * time.Minute,
		RedisCacheEnabled:   false, // Disabled by default
		L2TTL:              30 * time.Minute,
		WarmupEnabled:       true,
		WarmupWorkers:       4,
		WarmupBatchSize:     100,
		WarmupTimeout:       30 * time.Second,
		MetricsEnabled:      true,
		HealthCheckInterval: 30 * time.Second,
	}
}

// CacheMetrics provides comprehensive cache performance metrics
type CacheMetrics struct {
	// Hit/Miss statistics
	L1Hits        int64   `json:"l1_hits"`
	L2Hits        int64   `json:"l2_hits"`
	Misses        int64   `json:"misses"`
	HitRate       float64 `json:"hit_rate"`
	L1HitRate     float64 `json:"l1_hit_rate"`
	L2HitRate     float64 `json:"l2_hit_rate"`

	// Performance metrics
	AvgResponseTime time.Duration `json:"avg_response_time_ms"`
	WarmupTime      time.Duration `json:"warmup_time_ms"`
	LastWarmupTime  time.Time     `json:"last_warmup_time"`

	// Resource usage
	MemoryUsage     int64     `json:"memory_usage_bytes"`
	L1ItemCount     int       `json:"l1_item_count"`
	L2ItemCount     int       `json:"l2_item_count"`

	// Health status
	IsHealthy       bool      `json:"is_healthy"`
	LastHealthCheck time.Time `json:"last_health_check"`

	// Operation counters
	InvalidationCount int64 `json:"invalidation_count"`
	WarmupCount       int64 `json:"warmup_count"`
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	TTL       time.Duration `json:"ttl"`
	Layer     CacheLayer  `json:"layer"`
	Version   int64       `json:"version"`
}

// LayeredCacheManager implements a multi-layer cache system
type LayeredCacheManager struct {
	config   *CacheConfig

	// Cache layers
	l1Cache  *MemoryCache
	l2Cache  *RedisCache

	// Cache warming
	warmer   *CacheWarmer

	// Metrics and monitoring
	metrics  *CacheMetrics

	// Synchronization
	mutex    sync.RWMutex

	// State management
	isWarmupComplete int32
	shutdownChan     chan struct{}

	// Event channels
	invalidationChan chan CacheInvalidationEvent
}

// CacheInvalidationEvent represents a cache invalidation event
type CacheInvalidationEvent struct {
	Type      string    `json:"type"` // "channel", "group", "all"
	Key       string    `json:"key"`
	Timestamp time.Time `json:"timestamp"`
}

// NewLayeredCacheManager creates a new cache manager with the given configuration
func NewLayeredCacheManager(config *CacheConfig) (*LayeredCacheManager, error) {
	if config == nil {
		config = DefaultCacheConfig()
	}

	manager := &LayeredCacheManager{
		config:           config,
		metrics:          &CacheMetrics{LastHealthCheck: time.Now()},
		shutdownChan:     make(chan struct{}),
		invalidationChan: make(chan CacheInvalidationEvent, 1000),
	}

	// Initialize L1 memory cache
	if config.MemoryCacheEnabled {
		manager.l1Cache = NewMemoryCache(config.MaxMemoryItems, config.L1TTL)
	}

	// Initialize L2 redis cache
	if config.RedisCacheEnabled {
		redisCache, err := NewRedisCache(&RedisCacheConfig{
			Addr:     config.RedisAddr,
			Password: config.RedisPassword,
			DB:       config.RedisDB,
			TTL:      config.L2TTL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Redis cache: %w", err)
		}
		manager.l2Cache = redisCache
	}

	// Initialize cache warmer
	if config.WarmupEnabled {
		manager.warmer = NewCacheWarmer(&CacheWarmerConfig{
			Workers:     config.WarmupWorkers,
			BatchSize:   config.WarmupBatchSize,
			Timeout:     config.WarmupTimeout,
		})
	}

	// Start background processes
	go manager.runInvalidationProcessor()
	if config.MetricsEnabled {
		go manager.runMetricsUpdater()
	}

	return manager, nil
}

// GetChannel retrieves a channel from the cache hierarchy
func (cm *LayeredCacheManager) GetChannel(id int) (*Channel, error) {
	start := time.Now()
	defer func() {
		cm.metrics.AvgResponseTime = time.Since(start)
	}()

	key := fmt.Sprintf("ch:%d", id)

	// Try L1 cache first
	if cm.l1Cache != nil {
		if entry, found := cm.l1Cache.Get(key); found {
			atomic.AddInt64(&cm.metrics.L1Hits, 1)
			if channel, ok := entry.Data.(*Channel); ok {
				return channel, nil
			}
		}
	}

	// Try L2 cache
	if cm.l2Cache != nil {
		if entry, err := cm.l2Cache.Get(context.Background(), key); err == nil && entry != nil {
			atomic.AddInt64(&cm.metrics.L2Hits, 1)
			if channel, ok := entry.Data.(*Channel); ok {
				// Populate L1 cache
				if cm.l1Cache != nil {
					cm.l1Cache.Set(key, entry)
				}
				return channel, nil
			}
		}
	}

	// Cache miss - fetch from database
	atomic.AddInt64(&cm.metrics.Misses, 1)
	channel, err := GetChannelById(id, true)
	if err != nil {
		return nil, err
	}

	// Cache the result in both layers
	entry := &CacheEntry{
		Data:      channel,
		Timestamp: time.Now(),
		TTL:       cm.config.L1TTL,
		Layer:     L1Layer,
		Version:   1,
	}

	if cm.l1Cache != nil {
		cm.l1Cache.Set(key, entry)
	}
	if cm.l2Cache != nil {
		entry.TTL = cm.config.L2TTL
		entry.Layer = L2Layer
		cm.l2Cache.Set(context.Background(), key, entry)
	}

	return channel, nil
}

// GetRandomSatisfiedChannel provides cached channel selection with fallback
func (cm *LayeredCacheManager) GetRandomSatisfiedChannel(ctx *gin.Context, group, model string, retry int) (*Channel, string, error) {
	start := time.Now()
	defer func() {
		cm.metrics.AvgResponseTime = time.Since(start)
	}()

	// If cache is not warmed up or disabled, fall back to original method
	if !cm.IsWarmupComplete() || !common.MemoryCacheEnabled {
		return CacheGetRandomSatisfiedChannel(ctx, group, model, retry)
	}

	// Use enhanced caching logic for channel selection
	key := fmt.Sprintf("gm:%s:%s:%d", group, model, retry)

	// Try L1 cache first
	if cm.l1Cache != nil {
		if entry, found := cm.l1Cache.Get(key); found {
			atomic.AddInt64(&cm.metrics.L1Hits, 1)
			if result, ok := entry.Data.(*ChannelSelectionResult); ok {
				return result.Channel, result.SelectedGroup, nil
			}
		}
	}

	// Cache miss - perform selection and cache result
	atomic.AddInt64(&cm.metrics.Misses, 1)
	channel, selectedGroup, err := CacheGetRandomSatisfiedChannel(ctx, group, model, retry)
	if err != nil || channel == nil {
		return channel, selectedGroup, err
	}

	// Cache the selection result with shorter TTL (since it's randomized)
	result := &ChannelSelectionResult{
		Channel:       channel,
		SelectedGroup: selectedGroup,
		Timestamp:     time.Now(),
	}

	entry := &CacheEntry{
		Data:      result,
		Timestamp: time.Now(),
		TTL:       30 * time.Second, // Short TTL for randomized results
		Layer:     L1Layer,
		Version:   1,
	}

	if cm.l1Cache != nil {
		cm.l1Cache.Set(key, entry)
	}

	return channel, selectedGroup, nil
}

// ChannelSelectionResult caches the result of channel selection
type ChannelSelectionResult struct {
	Channel       *Channel  `json:"channel"`
	SelectedGroup string    `json:"selected_group"`
	Timestamp     time.Time `json:"timestamp"`
}

// InvalidateChannel removes a channel from all cache layers
func (cm *LayeredCacheManager) InvalidateChannel(id int) error {
	key := fmt.Sprintf("ch:%d", id)

	// Remove from L1 cache
	if cm.l1Cache != nil {
		cm.l1Cache.Delete(key)
	}

	// Remove from L2 cache
	if cm.l2Cache != nil {
		if err := cm.l2Cache.Delete(context.Background(), key); err != nil {
			common.SysLog(fmt.Sprintf("Failed to invalidate L2 cache for channel %d: %v", id, err))
		}
	}

	// Send invalidation event
	select {
	case cm.invalidationChan <- CacheInvalidationEvent{
		Type:      "channel",
		Key:       strconv.Itoa(id),
		Timestamp: time.Now(),
	}:
	default:
		// Channel full, log warning
		common.SysLog("Warning: invalidation channel is full")
	}

	atomic.AddInt64(&cm.metrics.InvalidationCount, 1)
	return nil
}

// InvalidateGroup removes all group-related cache entries
func (cm *LayeredCacheManager) InvalidateGroup(group string) error {
	pattern := fmt.Sprintf("gm:%s:*", group)

	// Remove from L1 cache
	if cm.l1Cache != nil {
		cm.l1Cache.DeletePattern(pattern)
	}

	// Remove from L2 cache
	if cm.l2Cache != nil {
		if err := cm.l2Cache.DeletePattern(context.Background(), pattern); err != nil {
			common.SysLog(fmt.Sprintf("Failed to invalidate L2 cache for group %s: %v", group, err))
		}
	}

	// Send invalidation event
	select {
	case cm.invalidationChan <- CacheInvalidationEvent{
		Type:      "group",
		Key:       group,
		Timestamp: time.Now(),
	}:
	default:
		common.SysLog("Warning: invalidation channel is full")
	}

	atomic.AddInt64(&cm.metrics.InvalidationCount, 1)
	return nil
}

// InvalidateAll clears all cache layers
func (cm *LayeredCacheManager) InvalidateAll() error {
	// Clear L1 cache
	if cm.l1Cache != nil {
		cm.l1Cache.Clear()
	}

	// Clear L2 cache
	if cm.l2Cache != nil {
		if err := cm.l2Cache.Clear(context.Background()); err != nil {
			common.SysLog(fmt.Sprintf("Failed to clear L2 cache: %v", err))
		}
	}

	// Send invalidation event
	select {
	case cm.invalidationChan <- CacheInvalidationEvent{
		Type:      "all",
		Key:       "",
		Timestamp: time.Now(),
	}:
	default:
		common.SysLog("Warning: invalidation channel is full")
	}

	atomic.AddInt64(&cm.metrics.InvalidationCount, 1)
	return nil
}

// WarmupCache performs intelligent cache warming
func (cm *LayeredCacheManager) WarmupCache(ctx context.Context) error {
	if cm.warmer == nil {
		return fmt.Errorf("cache warmer not initialized")
	}

	start := time.Now()
	atomic.StoreInt32(&cm.isWarmupComplete, 0)

	err := cm.warmer.WarmupAll(ctx, cm)

	duration := time.Since(start)
	cm.metrics.WarmupTime = duration
	cm.metrics.LastWarmupTime = time.Now()
	atomic.AddInt64(&cm.metrics.WarmupCount, 1)

	if err != nil {
		common.SysLog(fmt.Sprintf("Cache warmup failed after %.2fs: %v", duration.Seconds(), err))
		return err
	}

	atomic.StoreInt32(&cm.isWarmupComplete, 1)
	common.SysLog(fmt.Sprintf("Cache warmup completed successfully in %.2fs", duration.Seconds()))

	return nil
}

// IsWarmupComplete returns whether cache warmup is complete
func (cm *LayeredCacheManager) IsWarmupComplete() bool {
	return atomic.LoadInt32(&cm.isWarmupComplete) == 1
}

// GetMetrics returns current cache performance metrics
func (cm *LayeredCacheManager) GetMetrics() *CacheMetrics {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// Calculate hit rates
	totalRequests := cm.metrics.L1Hits + cm.metrics.L2Hits + cm.metrics.Misses
	if totalRequests > 0 {
		cm.metrics.HitRate = float64(cm.metrics.L1Hits+cm.metrics.L2Hits) / float64(totalRequests)
		cm.metrics.L1HitRate = float64(cm.metrics.L1Hits) / float64(totalRequests)
		cm.metrics.L2HitRate = float64(cm.metrics.L2Hits) / float64(totalRequests)
	}

	// Update cache item counts
	if cm.l1Cache != nil {
		cm.metrics.L1ItemCount = cm.l1Cache.Size()
		cm.metrics.MemoryUsage = cm.l1Cache.MemoryUsage()
	}
	if cm.l2Cache != nil {
		cm.metrics.L2ItemCount = cm.l2Cache.Size()
	}

	// Create a copy to avoid data races
	metricsCopy := *cm.metrics
	return &metricsCopy
}

// HealthCheck performs a comprehensive health check
func (cm *LayeredCacheManager) HealthCheck() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.metrics.LastHealthCheck = time.Now()
	cm.metrics.IsHealthy = true

	// Check L1 cache health
	if cm.l1Cache != nil {
		if err := cm.l1Cache.HealthCheck(); err != nil {
			cm.metrics.IsHealthy = false
			return fmt.Errorf("L1 cache health check failed: %w", err)
		}
	}

	// Check L2 cache health
	if cm.l2Cache != nil {
		if err := cm.l2Cache.HealthCheck(); err != nil {
			cm.metrics.IsHealthy = false
			common.SysLog(fmt.Sprintf("L2 cache health check failed: %v", err))
			// Don't fail the entire health check if only L2 is down
		}
	}

	return nil
}

// OnChannelUpdate handles channel update events
func (cm *LayeredCacheManager) OnChannelUpdate(channel *Channel) error {
	if channel == nil {
		return fmt.Errorf("channel cannot be nil")
	}

	// Invalidate the specific channel
	if err := cm.InvalidateChannel(channel.Id); err != nil {
		return err
	}

	// Invalidate group-model mappings that might be affected
	groups := parseCommaSeparated(channel.Group)
	for _, group := range groups {
		if err := cm.InvalidateGroup(group); err != nil {
			common.SysLog(fmt.Sprintf("Failed to invalidate group %s: %v", group, err))
		}
	}

	return nil
}

// OnChannelStatusChange handles channel status change events
func (cm *LayeredCacheManager) OnChannelStatusChange(id int, status int) error {
	return cm.InvalidateChannel(id)
}

// runInvalidationProcessor processes invalidation events in the background
func (cm *LayeredCacheManager) runInvalidationProcessor() {
	for {
		select {
		case event := <-cm.invalidationChan:
			if common.DebugEnabled {
				common.SysLog(fmt.Sprintf("Processing cache invalidation: type=%s, key=%s",
					event.Type, event.Key))
			}
			// Additional event processing logic can be added here

		case <-cm.shutdownChan:
			return
		}
	}
}

// runMetricsUpdater updates metrics periodically
func (cm *LayeredCacheManager) runMetricsUpdater() {
	ticker := time.NewTicker(cm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cm.HealthCheck(); err != nil {
				common.SysLog(fmt.Sprintf("Cache health check failed: %v", err))
			}

		case <-cm.shutdownChan:
			return
		}
	}
}

// Shutdown gracefully shuts down the cache manager
func (cm *LayeredCacheManager) Shutdown(ctx context.Context) error {
	close(cm.shutdownChan)

	// Close cache layers
	if cm.l1Cache != nil {
		cm.l1Cache.Close()
	}
	if cm.l2Cache != nil {
		cm.l2Cache.Close()
	}

	common.SysLog("Cache manager shutdown completed")
	return nil
}

// parseCommaSeparated splits a comma-separated string and trims whitespace
func parseCommaSeparated(input string) []string {
	if input == "" {
		return []string{}
	}

	parts := make([]string, 0)
	for _, part := range strings.Split(input, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}