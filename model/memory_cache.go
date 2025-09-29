package model

import (
	"fmt"
	"one-api/common"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// MemoryCache implements a thread-safe in-memory cache with LRU eviction
type MemoryCache struct {
	data      map[string]*memoryCacheNode
	lruHead   *memoryCacheNode
	lruTail   *memoryCacheNode
	maxItems  int
	defaultTTL time.Duration
	mutex     sync.RWMutex
	size      int
}

// memoryCacheNode represents a node in the LRU linked list
type memoryCacheNode struct {
	key       string
	entry     *CacheEntry
	expiresAt time.Time
	prev      *memoryCacheNode
	next      *memoryCacheNode
}

// NewMemoryCache creates a new memory cache with the specified configuration
func NewMemoryCache(maxItems int, defaultTTL time.Duration) *MemoryCache {
	// Create dummy head and tail nodes for the LRU list
	head := &memoryCacheNode{}
	tail := &memoryCacheNode{}
	head.next = tail
	tail.prev = head

	return &MemoryCache{
		data:       make(map[string]*memoryCacheNode),
		lruHead:    head,
		lruTail:    tail,
		maxItems:   maxItems,
		defaultTTL: defaultTTL,
		size:       0,
	}
}

// Get retrieves an item from the cache
func (mc *MemoryCache) Get(key string) (*CacheEntry, bool) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	node, exists := mc.data[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Now().After(node.expiresAt) {
		mc.removeNode(node)
		delete(mc.data, key)
		mc.size--
		return nil, false
	}

	// Move to front (most recently used)
	mc.moveToFront(node)

	return node.entry, true
}

// Set adds or updates an item in the cache
func (mc *MemoryCache) Set(key string, entry *CacheEntry) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// Determine TTL
	ttl := mc.defaultTTL
	if entry.TTL > 0 {
		ttl = entry.TTL
	}

	expiresAt := time.Now().Add(ttl)

	if node, exists := mc.data[key]; exists {
		// Update existing entry
		node.entry = entry
		node.expiresAt = expiresAt
		mc.moveToFront(node)
		return
	}

	// Create new node
	newNode := &memoryCacheNode{
		key:       key,
		entry:     entry,
		expiresAt: expiresAt,
	}

	// Add to front of LRU list
	mc.addToFront(newNode)
	mc.data[key] = newNode
	mc.size++

	// Evict if necessary
	if mc.size > mc.maxItems {
		mc.evictLRU()
	}
}

// Delete removes an item from the cache
func (mc *MemoryCache) Delete(key string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	if node, exists := mc.data[key]; exists {
		mc.removeNode(node)
		delete(mc.data, key)
		mc.size--
	}
}

// DeletePattern removes all keys matching the given pattern
func (mc *MemoryCache) DeletePattern(pattern string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// Convert glob pattern to matching logic
	keysToDelete := make([]string, 0)

	for key := range mc.data {
		if mc.matchesPattern(key, pattern) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	// Delete matched keys
	for _, key := range keysToDelete {
		if node, exists := mc.data[key]; exists {
			mc.removeNode(node)
			delete(mc.data, key)
			mc.size--
		}
	}

	if len(keysToDelete) > 0 && common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Deleted %d cache entries matching pattern: %s", len(keysToDelete), pattern))
	}
}

// Clear removes all items from the cache
func (mc *MemoryCache) Clear() {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	mc.data = make(map[string]*memoryCacheNode)
	mc.lruHead.next = mc.lruTail
	mc.lruTail.prev = mc.lruHead
	mc.size = 0
}

// Size returns the current number of items in the cache
func (mc *MemoryCache) Size() int {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	return mc.size
}

// MemoryUsage estimates the memory usage of the cache in bytes
func (mc *MemoryCache) MemoryUsage() int64 {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	var totalSize int64

	// Estimate memory usage for each cache entry
	for key, node := range mc.data {
		// Key size
		totalSize += int64(len(key))

		// Node structure size
		totalSize += int64(unsafe.Sizeof(*node))

		// CacheEntry size estimation
		if node.entry != nil {
			totalSize += int64(unsafe.Sizeof(*node.entry))
			// Add estimated data size (this is a rough approximation)
			totalSize += mc.estimateDataSize(node.entry.Data)
		}
	}

	// Add overhead for map structure
	totalSize += int64(len(mc.data)) * 8 // rough estimate for map overhead

	return totalSize
}

// CleanupExpired removes all expired entries
func (mc *MemoryCache) CleanupExpired() int {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, node := range mc.data {
		if now.After(node.expiresAt) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		if node, exists := mc.data[key]; exists {
			mc.removeNode(node)
			delete(mc.data, key)
			mc.size--
		}
	}

	return len(expiredKeys)
}

// HealthCheck performs a health check on the cache
func (mc *MemoryCache) HealthCheck() error {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	// Check if cache is operational
	if mc.data == nil {
		return fmt.Errorf("cache data map is nil")
	}

	if mc.lruHead == nil || mc.lruTail == nil {
		return fmt.Errorf("LRU list is corrupted")
	}

	// Verify cache size consistency
	if len(mc.data) != mc.size {
		return fmt.Errorf("cache size inconsistency: map size=%d, recorded size=%d",
			len(mc.data), mc.size)
	}

	return nil
}

// Close gracefully shuts down the cache
func (mc *MemoryCache) Close() {
	mc.Clear()
}

// GetStats returns cache statistics
func (mc *MemoryCache) GetStats() map[string]interface{} {
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()

	stats := map[string]interface{}{
		"size":            mc.size,
		"max_items":       mc.maxItems,
		"memory_usage":    mc.MemoryUsage(),
		"default_ttl_ms":  mc.defaultTTL.Milliseconds(),
	}

	return stats
}

// StartCleanupWorker starts a background goroutine to clean up expired entries
func (mc *MemoryCache) StartCleanupWorker(interval time.Duration) chan<- struct{} {
	stopChan := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				expiredCount := mc.CleanupExpired()
				if expiredCount > 0 && common.DebugEnabled {
					common.SysLog(fmt.Sprintf("Memory cache cleanup: removed %d expired entries", expiredCount))
				}

			case <-stopChan:
				return
			}
		}
	}()

	return stopChan
}

// Helper methods for LRU list management

func (mc *MemoryCache) addToFront(node *memoryCacheNode) {
	node.prev = mc.lruHead
	node.next = mc.lruHead.next
	mc.lruHead.next.prev = node
	mc.lruHead.next = node
}

func (mc *MemoryCache) removeNode(node *memoryCacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (mc *MemoryCache) moveToFront(node *memoryCacheNode) {
	mc.removeNode(node)
	mc.addToFront(node)
}

func (mc *MemoryCache) evictLRU() {
	lru := mc.lruTail.prev
	if lru != mc.lruHead {
		mc.removeNode(lru)
		delete(mc.data, lru.key)
		mc.size--
	}
}

// matchesPattern checks if a key matches a glob-like pattern
func (mc *MemoryCache) matchesPattern(key, pattern string) bool {
	// Simple glob pattern matching supporting only '*' wildcard
	if !strings.Contains(pattern, "*") {
		return key == pattern
	}

	// Split pattern by '*'
	parts := strings.Split(pattern, "*")

	// Check if key starts with the first part
	if len(parts) > 0 && parts[0] != "" {
		if !strings.HasPrefix(key, parts[0]) {
			return false
		}
		key = key[len(parts[0]):]
	}

	// Check if key ends with the last part
	if len(parts) > 1 && parts[len(parts)-1] != "" {
		lastPart := parts[len(parts)-1]
		if !strings.HasSuffix(key, lastPart) {
			return false
		}
		key = key[:len(key)-len(lastPart)]
	}

	// Check middle parts
	for i := 1; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}

		idx := strings.Index(key, part)
		if idx == -1 {
			return false
		}
		key = key[idx+len(part):]
	}

	return true
}

// estimateDataSize provides a rough estimate of data size
func (mc *MemoryCache) estimateDataSize(data interface{}) int64 {
	if data == nil {
		return 0
	}

	switch v := data.(type) {
	case *Channel:
		// Rough estimate for Channel struct
		return int64(unsafe.Sizeof(*v)) + int64(len(v.Name)) + int64(len(v.Models)) + int64(len(v.Group))
	case *ChannelSelectionResult:
		// Rough estimate for ChannelSelectionResult
		channelSize := int64(0)
		if v.Channel != nil {
			channelSize = int64(unsafe.Sizeof(*v.Channel)) + int64(len(v.Channel.Name))
		}
		return int64(unsafe.Sizeof(*v)) + channelSize + int64(len(v.SelectedGroup))
	case string:
		return int64(len(v))
	case []byte:
		return int64(len(v))
	default:
		// Fallback estimate
		return int64(unsafe.Sizeof(data))
	}
}