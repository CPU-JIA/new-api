package model

import (
	"context"
	"encoding/json"
	"fmt"
	"one-api/common"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCacheConfig holds Redis cache configuration
type RedisCacheConfig struct {
	Addr         string
	Password     string
	DB           int
	TTL          time.Duration
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
	IdleTimeout  time.Duration
}

// DefaultRedisCacheConfig returns default Redis cache configuration
func DefaultRedisCacheConfig() *RedisCacheConfig {
	return &RedisCacheConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		TTL:          30 * time.Minute,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		IdleTimeout:  5 * time.Minute,
	}
}

// RedisCache implements a Redis-based distributed cache
type RedisCache struct {
	client    *redis.Client
	config    *RedisCacheConfig
	keyPrefix string
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(config *RedisCacheConfig) (*RedisCache, error) {
	if config == nil {
		config = DefaultRedisCacheConfig()
	}

	// Create Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		MaxRetries:   config.MaxRetries,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
		PoolSize:     config.PoolSize,
		IdleTimeout:  config.IdleTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client:    rdb,
		config:    config,
		keyPrefix: "oneapi:cache:",
	}, nil
}

// Get retrieves an item from Redis cache
func (rc *RedisCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	fullKey := rc.keyPrefix + key

	data, err := rc.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get cache entry: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("failed to deserialize cache entry: %w", err)
	}

	// Check if entry is expired (additional safety check)
	if time.Now().After(entry.Timestamp.Add(entry.TTL)) {
		// Entry is expired, delete it
		rc.client.Del(ctx, fullKey)
		return nil, nil
	}

	return &entry, nil
}

// Set stores an item in Redis cache
func (rc *RedisCache) Set(ctx context.Context, key string, entry *CacheEntry) error {
	fullKey := rc.keyPrefix + key

	// Use configured TTL if entry doesn't have one
	ttl := entry.TTL
	if ttl <= 0 {
		ttl = rc.config.TTL
	}

	// Update entry metadata
	entry.Layer = L2Layer
	entry.Timestamp = time.Now()

	// Serialize entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to serialize cache entry: %w", err)
	}

	// Store with TTL
	if err := rc.client.Set(ctx, fullKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	return nil
}

// Delete removes an item from Redis cache
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	fullKey := rc.keyPrefix + key

	if err := rc.client.Del(ctx, fullKey).Err(); err != nil {
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	return nil
}

// DeletePattern removes all keys matching the given pattern
func (rc *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	fullPattern := rc.keyPrefix + pattern

	// Use SCAN to find matching keys
	keys, err := rc.scanKeys(ctx, fullPattern)
	if err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete keys in batches
	batchSize := 100
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		if err := rc.client.Del(ctx, batch...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys batch: %w", err)
		}
	}

	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Deleted %d Redis cache entries matching pattern: %s", len(keys), pattern))
	}

	return nil
}

// Clear removes all cache entries
func (rc *RedisCache) Clear(ctx context.Context) error {
	pattern := rc.keyPrefix + "*"

	keys, err := rc.scanKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to scan all keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete all keys
	if err := rc.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	common.SysLog(fmt.Sprintf("Cleared %d Redis cache entries", len(keys)))
	return nil
}

// Size returns the approximate number of cache entries
func (rc *RedisCache) Size() int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pattern := rc.keyPrefix + "*"
	keys, err := rc.scanKeys(ctx, pattern)
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to get Redis cache size: %v", err))
		return 0
	}

	return len(keys)
}

// MemoryUsage returns Redis memory usage (approximate)
func (rc *RedisCache) MemoryUsage() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := rc.client.Info(ctx, "memory").Result()
	if err != nil {
		common.SysLog(fmt.Sprintf("Failed to get Redis memory info: %v", err))
		return 0
	}

	// Parse used_memory from Redis INFO
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "used_memory:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				var memory int64
				if _, err := fmt.Sscanf(parts[1], "%d", &memory); err == nil {
					return memory
				}
			}
		}
	}

	return 0
}

// HealthCheck performs a health check on the Redis connection
func (rc *RedisCache) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rc.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// Close gracefully closes the Redis connection
func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

// GetStats returns Redis cache statistics
func (rc *RedisCache) GetStats() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stats := map[string]interface{}{
		"addr":        rc.config.Addr,
		"db":          rc.config.DB,
		"ttl_ms":      rc.config.TTL.Milliseconds(),
		"key_prefix":  rc.keyPrefix,
	}

	// Add Redis info if available
	if info, err := rc.client.Info(ctx).Result(); err == nil {
		stats["redis_info"] = rc.parseRedisInfo(info)
	}

	return stats
}

// Expire sets TTL for a specific key
func (rc *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := rc.keyPrefix + key
	return rc.client.Expire(ctx, fullKey, ttl).Err()
}

// Exists checks if a key exists
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	fullKey := rc.keyPrefix + key
	count, err := rc.client.Exists(ctx, fullKey).Result()
	return count > 0, err
}

// GetTTL returns the remaining TTL for a key
func (rc *RedisCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	fullKey := rc.keyPrefix + key
	return rc.client.TTL(ctx, fullKey).Result()
}

// SetNX sets a key only if it doesn't exist (atomic operation)
func (rc *RedisCache) SetNX(ctx context.Context, key string, entry *CacheEntry, ttl time.Duration) (bool, error) {
	fullKey := rc.keyPrefix + key

	// Update entry metadata
	entry.Layer = L2Layer
	entry.Timestamp = time.Now()

	// Serialize entry
	data, err := json.Marshal(entry)
	if err != nil {
		return false, fmt.Errorf("failed to serialize cache entry: %w", err)
	}

	// Use configured TTL if not provided
	if ttl <= 0 {
		ttl = rc.config.TTL
	}

	success, err := rc.client.SetNX(ctx, fullKey, data, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to set cache entry with NX: %w", err)
	}

	return success, nil
}

// Publish publishes a message to a Redis channel (for cache invalidation events)
func (rc *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	if err := rc.client.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Subscribe subscribes to a Redis channel (for cache invalidation events)
func (rc *RedisCache) Subscribe(ctx context.Context, channels ...string) *redis.PubSub {
	return rc.client.Subscribe(ctx, channels...)
}

// Helper methods

// scanKeys scans for keys matching a pattern
func (rc *RedisCache) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	cursor := uint64(0)

	for {
		result, newCursor, err := rc.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, result...)
		cursor = newCursor

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// parseRedisInfo parses Redis INFO output into a map
func (rc *RedisCache) parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(info, "\r\n")

	for _, line := range lines {
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "#") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

// GetMulti retrieves multiple cache entries in a single operation
func (rc *RedisCache) GetMulti(ctx context.Context, keys []string) (map[string]*CacheEntry, error) {
	if len(keys) == 0 {
		return make(map[string]*CacheEntry), nil
	}

	// Prepare full keys
	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = rc.keyPrefix + key
	}

	// Get all values
	values, err := rc.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get multiple cache entries: %w", err)
	}

	// Parse results
	result := make(map[string]*CacheEntry)
	for i, value := range values {
		if value == nil {
			continue // Cache miss
		}

		if data, ok := value.(string); ok {
			var entry CacheEntry
			if err := json.Unmarshal([]byte(data), &entry); err != nil {
				common.SysLog(fmt.Sprintf("Failed to deserialize cache entry for key %s: %v", keys[i], err))
				continue
			}

			// Check expiration
			if time.Now().After(entry.Timestamp.Add(entry.TTL)) {
				// Entry expired, clean it up asynchronously
				go rc.client.Del(context.Background(), fullKeys[i])
				continue
			}

			result[keys[i]] = &entry
		}
	}

	return result, nil
}

// SetMulti stores multiple cache entries in a single operation
func (rc *RedisCache) SetMulti(ctx context.Context, entries map[string]*CacheEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Prepare pipeline
	pipe := rc.client.Pipeline()

	for key, entry := range entries {
		fullKey := rc.keyPrefix + key

		// Use configured TTL if entry doesn't have one
		ttl := entry.TTL
		if ttl <= 0 {
			ttl = rc.config.TTL
		}

		// Update entry metadata
		entry.Layer = L2Layer
		entry.Timestamp = time.Now()

		// Serialize entry
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to serialize cache entry for key %s: %w", key, err)
		}

		pipe.Set(ctx, fullKey, data, ttl)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to set multiple cache entries: %w", err)
	}

	return nil
}