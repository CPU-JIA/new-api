package dto

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
	EnableSmartWarmup           bool   `json:"enable_smart_warmup,omitempty"`            // Enable intelligent cache keep-alive
	WarmupThreshold             int    `json:"warmup_threshold,omitempty"`               // Min requests per 5min to trigger warmup (default: 10)
	WarmupInterval              int    `json:"warmup_interval,omitempty"`                // Warmup interval in seconds (default: 240)

	// Multi-level cache (optional advanced feature)
	EnableCategoryCache bool              `json:"enable_category_cache,omitempty"`    // Enable category-based second-level cache
	CategoryPrompts     map[string]string `json:"category_prompts,omitempty"`         // Category name -> prompt content
	CacheHistoryMessages int              `json:"cache_history_messages,omitempty"`   // Number of history messages to cache (0=disabled, default: 3)
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
