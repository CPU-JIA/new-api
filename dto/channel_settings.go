package dto

import "fmt"

type ChannelSettings struct {
	ForceFormat            bool   `json:"force_format,omitempty"`
	ThinkingToContent      bool   `json:"thinking_to_content,omitempty"`
	Proxy                  string `json:"proxy"`
	PassThroughBodyEnabled bool   `json:"pass_through_body_enabled,omitempty"`
	SystemPrompt           string `json:"system_prompt,omitempty"`
	SystemPromptOverride   bool   `json:"system_prompt_override,omitempty"`

	// Pool Cache Optimization - for API pool scenarios
	EnablePoolCacheOptimization bool   `json:"enable_pool_cache_optimization,omitempty"` // Enable automatic cache padding injection
	CachePaddingContent         string `json:"cache_padding_content,omitempty"`          // Custom padding content (if empty, use default)
	CacheTTL                    string `json:"cache_ttl,omitempty"`                      // Cache TTL: "5m" (default) or "1h"
	EnableSmartWarmup           bool   `json:"enable_smart_warmup,omitempty"`            // Enable intelligent cache keep-alive
	WarmupThreshold             int    `json:"warmup_threshold,omitempty"`               // Min requests per 5min to trigger warmup (default: 10)
	WarmupInterval              int    `json:"warmup_interval,omitempty"`                // Warmup interval in seconds (default: 240)

	// Multi-level cache (optional advanced feature)
	EnableCategoryCache bool              `json:"enable_category_cache,omitempty"`    // Enable category-based second-level cache
	CategoryPrompts     map[string]string `json:"category_prompts,omitempty"`         // Category name -> prompt content
	CacheHistoryMessages int              `json:"cache_history_messages,omitempty"`   // Number of history messages to cache (0=disabled, default: 3)
}

// NormalizeCacheConfig sets default values for cache-related configuration
// ECP-B1: DRY - centralize default value logic in one place
// ECP-C1: Defensive Programming - always ensure valid configuration
func (cs *ChannelSettings) NormalizeCacheConfig() {
	// Set CacheTTL default
	if cs.CacheTTL == "" {
		cs.CacheTTL = "5m" // Default to 5-minute TTL
	}

	// Set WarmupThreshold default
	if cs.WarmupThreshold == 0 {
		cs.WarmupThreshold = 10 // Default: 10 requests per 5 minutes
	}

	// WarmupInterval is deprecated (now calculated dynamically)
	// Keep it for backward compatibility but mark as unused
	if cs.WarmupInterval == 0 {
		cs.WarmupInterval = 240 // Keep default for compatibility
	}

	// Enforce logical dependencies
	if !cs.EnablePoolCacheOptimization {
		// If pool cache is disabled, disable dependent features
		cs.EnableSmartWarmup = false
		cs.EnableCategoryCache = false
		cs.CacheHistoryMessages = 0
	}

	if !cs.EnableSmartWarmup {
		// Warmup-specific settings are irrelevant
		// Don't reset them (allow pre-configuration), just ignore at runtime
	}

	if cs.EnableCategoryCache && (cs.CategoryPrompts == nil || len(cs.CategoryPrompts) == 0) {
		// Category cache enabled but no prompts defined - auto-disable
		cs.EnableCategoryCache = false
	}
}

// ValidateCacheConfig validates cache-related configuration
// Returns error for invalid configuration that should be rejected
// ECP-C2: Systematic Error Handling - provide clear validation errors
func (cs *ChannelSettings) ValidateCacheConfig() error {
	// Validate CacheTTL (must be "5m" or "1h")
	if cs.CacheTTL != "" && cs.CacheTTL != "5m" && cs.CacheTTL != "1h" {
		return fmt.Errorf("invalid cache_ttl: must be '5m' or '1h', got '%s'", cs.CacheTTL)
	}

	// Validate WarmupThreshold (reasonable range)
	if cs.WarmupThreshold < 0 || cs.WarmupThreshold > 100 {
		return fmt.Errorf("invalid warmup_threshold: must be 0-100, got %d", cs.WarmupThreshold)
	}

	// Validate WarmupInterval (deprecated but still validate if set)
	if cs.WarmupInterval < 0 || cs.WarmupInterval > 3600 {
		return fmt.Errorf("invalid warmup_interval: must be 0-3600 seconds, got %d", cs.WarmupInterval)
	}

	// Validate CacheHistoryMessages (reasonable range)
	if cs.CacheHistoryMessages < 0 || cs.CacheHistoryMessages > 10 {
		return fmt.Errorf("invalid cache_history_messages: must be 0-10, got %d", cs.CacheHistoryMessages)
	}

	return nil
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type ChannelOtherSettings struct {
	AzureResponsesVersion string        `json:"azure_responses_version,omitempty"`
	VertexKeyType         VertexKeyType `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise  *bool         `json:"openrouter_enterprise,omitempty"`
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}
