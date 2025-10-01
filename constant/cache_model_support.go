package constant

import "strings"

// CacheSupportedClaudeModels lists all Claude models that support Prompt Caching
// Reference: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching
// Last updated: 2025-01-09
//
// Supported models include:
// - Claude Opus 4.1, 4
// - Claude Sonnet 4.5, 4, 3.7, 3.5, 3
// - Claude Haiku 3.5, 3
//
// Minimum requirements:
// - 1024 tokens minimum for caching
// - Cache TTL: 5 minutes (default) or 1 hour
// - Up to 4 cache breakpoints per request
//
// Cost structure:
// - Cache write: base input price + 25%
// - Cache read: base input price × 10% (90% savings)
var CacheSupportedClaudeModels = map[string]bool{
	// Claude 4 Series (Latest generation - Released 2025)
	"claude-sonnet-4.5-20250929":        true, // Sonnet 4.5 (Latest flagship model)
	"claude-sonnet-4.5-20250929-thinking": true,
	"claude-sonnet-4-20250514":          true,
	"claude-sonnet-4-20250514-thinking": true,
	"claude-opus-4-20250514":            true,
	"claude-opus-4-20250514-thinking":   true,
	"claude-opus-4-1-20250805":          true,
	"claude-opus-4-1-20250805-thinking": true,

	// Claude 3.7 Series
	"claude-3-7-sonnet-20250219":          true,
	"claude-3-7-sonnet-20250219-thinking": true,

	// Claude 3.5 Series
	"claude-3-5-sonnet-20241022": true,
	"claude-3-5-sonnet-20240620": true, // Deprecated but still supported
	"claude-3-5-haiku-20241022":  true,

	// Claude 3 Series (Original generation)
	"claude-3-sonnet-20240229": true,
	"claude-3-opus-20240229":   true,
	"claude-3-haiku-20240307":  true,
}

// CacheUnsupportedClaudeModels lists Claude models that DO NOT support Prompt Caching
var CacheUnsupportedClaudeModels = map[string]bool{
	"claude-instant-1.2": true,
	"claude-instant-1.1": true,
	"claude-instant-1.0": true,
	"claude-instant-1":   true,
	"claude-2.1":         true,
	"claude-2.0":         true,
	"claude-2":           true,
	"claude-1.3":         true,
	"claude-1.2":         true,
	"claude-1.0":         true,
	"claude-1":           true,
}

// IsClaudeModelSupportCache checks if a Claude model supports Prompt Caching
// Uses flexible matching to handle various model naming formats
func IsClaudeModelSupportCache(modelName string) bool {
	if modelName == "" {
		return false
	}

	// Exact match (fastest path)
	if CacheSupportedClaudeModels[modelName] {
		return true
	}

	// Explicit unsupported check (for Claude 2.x and Instant)
	if CacheUnsupportedClaudeModels[modelName] {
		return false
	}

	// Fuzzy matching for versioned models
	// This handles cases like:
	// - "claude-sonnet-4-20250514" matching "claude-sonnet-4"
	// - "claude-3-5-sonnet-20241022" matching "claude-3-5-sonnet"
	modelLower := strings.ToLower(modelName)

	// Check for Claude 4 series patterns (all support caching)
	if strings.Contains(modelLower, "claude-4") ||
		strings.Contains(modelLower, "claude-sonnet-4") ||
		strings.Contains(modelLower, "claude-opus-4") {
		return true
	}

	// Check for Claude 3.5/3.7 series patterns (all support caching)
	if strings.Contains(modelLower, "claude-3-5") ||
		strings.Contains(modelLower, "claude-3-7") {
		return true
	}

	// Check for Claude 3 series patterns (all support caching)
	if strings.Contains(modelLower, "claude-3-sonnet") ||
		strings.Contains(modelLower, "claude-3-opus") ||
		strings.Contains(modelLower, "claude-3-haiku") {
		return true
	}

	// Check for Claude 2.x or Instant (do NOT support caching)
	if strings.Contains(modelLower, "claude-2") ||
		strings.Contains(modelLower, "claude-instant") ||
		strings.Contains(modelLower, "claude-1") {
		return false
	}

	// Conservative default: if unknown Claude model, assume it supports caching
	// This ensures forward compatibility with new models
	// (Anthropic is adding caching support to all new models)
	if strings.HasPrefix(modelLower, "claude-") {
		return true
	}

	// Not a Claude model
	return false
}

// GetCacheMinimumTokens returns the minimum number of tokens required for caching
// For Claude models, this is 1024 tokens
func GetCacheMinimumTokens() int {
	return 1024
}

// GetCacheMaxBreakpoints returns the maximum number of cache breakpoints allowed
// For Claude models, this is 4 breakpoints
func GetCacheMaxBreakpoints() int {
	return 4
}

// GetCacheDefaultTTL returns the default cache TTL in seconds
// For Claude models, this is 5 minutes (300 seconds)
func GetCacheDefaultTTL() int {
	return 300
}

// GetCacheLongTTL returns the extended cache TTL in seconds
// For Claude models, this is 1 hour (3600 seconds)
func GetCacheLongTTL() int {
	return 3600
}