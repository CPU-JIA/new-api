package model

import (
	"context"
	"fmt"
	"one-api/common"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Global cache manager instance
var (
	globalCacheManager CacheManager
	cacheManagerMutex  sync.RWMutex
	cacheManagerOnce   sync.Once
)

// CacheIntegrationConfig holds configuration for cache system integration
type CacheIntegrationConfig struct {
	// Migration settings
	EnableNewCache     bool          // Enable new layered cache system
	MigrationDelay     time.Duration // Delay before switching to new system
	FallbackEnabled    bool          // Enable fallback to old system on errors

	// Performance settings
	WarmupOnStartup    bool          // Perform warmup during application startup
	WarmupTimeout      time.Duration // Maximum time to wait for warmup

	// Cache layers configuration
	MemoryCache        *CacheConfig  // Memory cache configuration
	RedisCache         *RedisCacheConfig // Redis cache configuration (optional)
}

// DefaultCacheIntegrationConfig returns sensible defaults
func DefaultCacheIntegrationConfig() *CacheIntegrationConfig {
	return &CacheIntegrationConfig{
		EnableNewCache:  true,
		MigrationDelay:  0 * time.Second,
		FallbackEnabled: true,
		WarmupOnStartup: true,
		WarmupTimeout:   30 * time.Second,
		MemoryCache:     DefaultCacheConfig(),
		RedisCache:      nil, // Disabled by default
	}
}

// InitializeAdvancedCacheSystem initializes the new layered cache system
func InitializeAdvancedCacheSystem(config *CacheIntegrationConfig) error {
	cacheManagerMutex.Lock()
	defer cacheManagerMutex.Unlock()

	if config == nil {
		config = DefaultCacheIntegrationConfig()
	}

	if !config.EnableNewCache {
		common.SysLog("Advanced cache system disabled, using legacy cache")
		return nil
	}

	// Create cache manager configuration
	cacheConfig := config.MemoryCache
	if cacheConfig == nil {
		cacheConfig = DefaultCacheConfig()
	}

	// Configure Redis if provided
	if config.RedisCache != nil {
		cacheConfig.RedisCacheEnabled = true
		cacheConfig.RedisAddr = config.RedisCache.Addr
		cacheConfig.RedisPassword = config.RedisCache.Password
		cacheConfig.RedisDB = config.RedisCache.DB
		cacheConfig.L2TTL = config.RedisCache.TTL
	}

	// Configure warmup
	cacheConfig.WarmupEnabled = config.WarmupOnStartup
	cacheConfig.WarmupTimeout = config.WarmupTimeout

	// Create the cache manager
	manager, err := NewLayeredCacheManager(cacheConfig)
	if err != nil {
		if config.FallbackEnabled {
			common.SysLog(fmt.Sprintf("Failed to initialize advanced cache, falling back to legacy: %v", err))
			return nil
		}
		return err
	}

	globalCacheManager = manager
	common.SysLog("Advanced layered cache system initialized successfully")

	// Perform warmup if enabled
	if config.WarmupOnStartup {
		go func() {
			if config.MigrationDelay > 0 {
				time.Sleep(config.MigrationDelay)
			}

			ctx, cancel := context.WithTimeout(context.Background(), config.WarmupTimeout)
			defer cancel()

			common.SysLog("Starting cache warmup process...")
			if err := manager.WarmupCache(ctx); err != nil {
				common.SysLog(fmt.Sprintf("Cache warmup completed with errors: %v", err))
			} else {
				common.SysLog("Cache warmup completed successfully")
			}
		}()
	}

	return nil
}

// GetCacheManager returns the global cache manager instance
func GetCacheManager() CacheManager {
	cacheManagerMutex.RLock()
	defer cacheManagerMutex.RUnlock()
	return globalCacheManager
}

// IsAdvancedCacheEnabled returns whether the advanced cache system is active
func IsAdvancedCacheEnabled() bool {
	return GetCacheManager() != nil
}

// Enhanced cache functions that integrate with existing API

// GetChannelByIdCached retrieves a channel with advanced caching
func GetChannelByIdCached(id int) (*Channel, error) {
	if manager := GetCacheManager(); manager != nil {
		return manager.GetChannel(id)
	}

	// Fallback to original implementation
	return GetChannelById(id, true)
}

// GetRandomSatisfiedChannelCached provides enhanced channel selection with caching
func GetRandomSatisfiedChannelCached(c *gin.Context, group string, model string, retry int) (*Channel, string, error) {
	if manager := GetCacheManager(); manager != nil {
		return manager.GetRandomSatisfiedChannel(c, group, model, retry)
	}

	// Fallback to original implementation
	return CacheGetRandomSatisfiedChannel(c, group, model, retry)
}

// InvalidateChannelCache invalidates cache entries for a specific channel
func InvalidateChannelCache(id int) error {
	if manager := GetCacheManager(); manager != nil {
		return manager.InvalidateChannel(id)
	}

	// For legacy cache, we can trigger a cache rebuild
	if common.MemoryCacheEnabled {
		go InitChannelCache()
	}

	return nil
}

// InvalidateGroupCache invalidates cache entries for a specific group
func InvalidateGroupCache(group string) error {
	if manager := GetCacheManager(); manager != nil {
		return manager.InvalidateGroup(group)
	}

	// For legacy cache, trigger full rebuild
	if common.MemoryCacheEnabled {
		go InitChannelCache()
	}

	return nil
}

// GetCacheMetrics returns comprehensive cache performance metrics
func GetCacheMetrics() *CacheMetrics {
	if manager := GetCacheManager(); manager != nil {
		return manager.GetMetrics()
	}

	// Return basic metrics for legacy cache
	return &CacheMetrics{
		IsHealthy: common.MemoryCacheEnabled,
		LastHealthCheck: time.Now(),
	}
}

// PerformCacheHealthCheck performs a health check on the cache system
func PerformCacheHealthCheck() error {
	if manager := GetCacheManager(); manager != nil {
		return manager.HealthCheck()
	}

	// Legacy cache health check is basic
	if !common.MemoryCacheEnabled {
		return fmt.Errorf("legacy cache is disabled")
	}

	return nil
}

// Enhanced channel update handlers that trigger cache invalidation

// OnChannelUpdatedCached handles channel update events with cache invalidation
func OnChannelUpdatedCached(channel *Channel) error {
	// Update database first
	err := DB.Save(channel).Error
	if err != nil {
		return err
	}

	// Invalidate cache
	if manager := GetCacheManager(); manager != nil {
		if err := manager.OnChannelUpdate(channel); err != nil {
			common.SysLog(fmt.Sprintf("Failed to invalidate cache after channel update: %v", err))
		}
	} else if common.MemoryCacheEnabled {
		// Fallback: update legacy cache
		CacheUpdateChannel(channel)
	}

	return nil
}

// OnChannelStatusChangedCached handles channel status changes with cache invalidation
func OnChannelStatusChangedCached(id int, status int) error {
	// Update database first
	if !UpdateChannelStatus(id, "", status, "") {
		return fmt.Errorf("failed to update channel status in database")
	}

	// Invalidate cache
	if manager := GetCacheManager(); manager != nil {
		if err := manager.OnChannelStatusChange(id, status); err != nil {
			common.SysLog(fmt.Sprintf("Failed to invalidate cache after status change: %v", err))
		}
	} else if common.MemoryCacheEnabled {
		// Fallback: update legacy cache
		CacheUpdateChannelStatus(id, status)
	}

	return nil
}

// Cache migration utilities

// MigrateLegacyCacheData migrates data from legacy cache to new cache system
func MigrateLegacyCacheData() error {
	manager := GetCacheManager()
	if manager == nil {
		return fmt.Errorf("advanced cache system not initialized")
	}

	if !common.MemoryCacheEnabled {
		return fmt.Errorf("legacy cache is not enabled")
	}

	common.SysLog("Starting legacy cache data migration...")

	// Trigger warmup to populate new cache
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := manager.WarmupCache(ctx)
	if err != nil {
		return fmt.Errorf("cache migration failed during warmup: %w", err)
	}

	common.SysLog("Legacy cache data migration completed")
	return nil
}

// ShutdownCacheSystem gracefully shuts down the cache system
func ShutdownCacheSystem() error {
	cacheManagerMutex.Lock()
	defer cacheManagerMutex.Unlock()

	if globalCacheManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := globalCacheManager.Shutdown(ctx)
		globalCacheManager = nil

		if err != nil {
			return err
		}

		common.SysLog("Advanced cache system shutdown completed")
	}

	return nil
}

// Cache monitoring and metrics endpoints

// GetCacheStatus returns detailed cache system status
func GetCacheStatus() map[string]interface{} {
	status := map[string]interface{}{
		"advanced_cache_enabled": IsAdvancedCacheEnabled(),
		"legacy_cache_enabled":   common.MemoryCacheEnabled,
		"timestamp":              time.Now().Unix(),
	}

	if manager := GetCacheManager(); manager != nil {
		metrics := manager.GetMetrics()
		status["metrics"] = metrics
		status["warmup_complete"] = manager.IsWarmupComplete()
	}

	return status
}

// GetCacheConfig returns current cache configuration
func GetCacheConfig() map[string]interface{} {
	config := map[string]interface{}{
		"memory_cache_enabled": common.MemoryCacheEnabled,
		"advanced_cache":       IsAdvancedCacheEnabled(),
	}

	if manager := GetCacheManager(); manager != nil {
		// Add advanced cache configuration details
		metrics := manager.GetMetrics()
		config["l1_items"] = metrics.L1ItemCount
		config["l2_items"] = metrics.L2ItemCount
		config["memory_usage"] = metrics.MemoryUsage
		config["hit_rate"] = metrics.HitRate
	}

	return config
}

// Cache debugging utilities for development

// DebugCacheContent returns cache content for debugging (development only)
func DebugCacheContent() map[string]interface{} {
	if !common.DebugEnabled {
		return map[string]interface{}{"error": "debug mode not enabled"}
	}

	debug := map[string]interface{}{
		"advanced_cache": IsAdvancedCacheEnabled(),
		"timestamp":      time.Now().Unix(),
	}

	if manager := GetCacheManager(); manager != nil {
		metrics := manager.GetMetrics()
		debug["advanced_metrics"] = metrics
	}

	// Note: Detailed cache content inspection would be added here in development builds
	debug["content_inspection"] = "Available in development builds only"

	return debug
}

// Cleanup resources when cache system is no longer needed
func init() {
	// Register cleanup handler for graceful shutdown
	// This could be integrated with application shutdown hooks
}

// Background maintenance for cache system
func StartCacheMaintenanceWorkers() {
	if !IsAdvancedCacheEnabled() {
		return
	}

	// Start periodic health checks
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if err := PerformCacheHealthCheck(); err != nil {
				common.SysLog(fmt.Sprintf("Cache health check failed: %v", err))
			}
		}
	}()

	// Start periodic metrics logging
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if common.DebugEnabled {
				metrics := GetCacheMetrics()
				common.SysLog(fmt.Sprintf("Cache metrics: HitRate=%.2f%%, L1Hits=%d, L2Hits=%d, Misses=%d",
					metrics.HitRate*100, metrics.L1Hits, metrics.L2Hits, metrics.Misses))
			}
		}
	}()

	common.SysLog("Cache maintenance workers started")
}