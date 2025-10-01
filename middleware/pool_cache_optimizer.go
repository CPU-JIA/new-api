package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"one-api/common"
	"one-api/constant"
	"one-api/dto"
	relay_constant "one-api/relay/constant"
	"one-api/service"
	"strings"

	"github.com/gin-gonic/gin"
)

// PoolCacheOptimizer optimizes prompt caching for API pool scenarios
// by automatically injecting shared cache padding content into requests.
// This allows multiple users to share the same cache, dramatically reducing costs.
func PoolCacheOptimizer() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				common.SysError("PoolCacheOptimizer panic recovered: " + common.Interface2String(r))
				// Continue processing even if optimizer fails
				c.Next()
			}
		}()

		// Get channel settings from context
		channelSetting, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting)
		if !ok {
			c.Next()
			return
		}

		// Check if pool cache optimization is enabled
		if !channelSetting.EnablePoolCacheOptimization {
			c.Next()
			return
		}

		// Record request for cache warmer metrics
		if channelSetting.EnableSmartWarmup {
			channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
			channelName := common.GetContextKeyString(c, constant.ContextKeyChannelName)
			service.GetCacheWarmerService().RecordRequest(channelID, channelName, &channelSetting)
		}

		// Only apply to Claude API endpoints
		if !strings.HasPrefix(c.Request.URL.Path, "/v1/messages") {
			c.Next()
			return
		}

		// Apply cache optimization
		err := applyPoolCacheOptimization(c, &channelSetting)
		if err != nil {
			common.SysError("Failed to apply pool cache optimization: " + err.Error())
			// Continue even if optimization fails - don't break user requests
		} else {
			if common.DebugEnabled {
				common.SysLog("PoolCache: Middleware successfully applied to " + c.Request.URL.Path)
			}
		}

		c.Next()
	}
}

// applyPoolCacheOptimization applies cache padding and cache_control markers
func applyPoolCacheOptimization(c *gin.Context, settings *dto.ChannelSettings) error {
	// Parse request body as ClaudeRequest
	var request dto.ClaudeRequest
	err := common.UnmarshalBodyReusable(c, &request)
	if err != nil {
		return err
	}

	// Get padding content
	paddingContent := getPaddingContent(settings)

	// Inject cache padding into system
	err = injectCachePadding(&request, paddingContent, settings)
	if err != nil {
		return err
	}

	// Optionally add cache markers to history messages
	if settings.CacheHistoryMessages > 0 {
		addHistoryCacheMarkers(&request, settings.CacheHistoryMessages)
	}

	// Marshal modified request back to body
	modifiedBody, err := json.Marshal(request)
	if err != nil {
		return err
	}

	// Replace request body with modified version
	c.Request.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
	c.Request.ContentLength = int64(len(modifiedBody))

	if common.DebugEnabled {
		systemBlockCount := 0
		if req, ok := interface{}(&request).(*dto.ClaudeRequest); ok {
			if req.System != nil {
				systemBlocks := req.ParseSystem()
				systemBlockCount = len(systemBlocks)
			}
		}
		common.SysLog(fmt.Sprintf("PoolCache: Injected padding=%d bytes, system_blocks=%d, history_markers=%d",
			len(paddingContent), systemBlockCount, settings.CacheHistoryMessages))
	}

	return nil
}

// getPaddingContent returns the padding content to use
func getPaddingContent(settings *dto.ChannelSettings) string {
	if settings != nil && settings.CachePaddingContent != "" {
		return settings.CachePaddingContent
	}
	return relay_constant.DefaultCachePadding
}

// injectCachePadding injects the shared cache padding into system prompt
func injectCachePadding(req *dto.ClaudeRequest, paddingContent string, settings *dto.ChannelSettings) error {
	// Build multi-level system blocks
	systemBlocks := []dto.ClaudeMediaMessage{}

	// Level 1: Global cache padding (shared across all users)
	paddingBlock := dto.ClaudeMediaMessage{
		Type: "text",
	}
	paddingBlock.SetText(paddingContent)
	paddingBlock.CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
	systemBlocks = append(systemBlocks, paddingBlock)

	// Level 2: Category cache (if enabled)
	if settings != nil && settings.EnableCategoryCache {
		categoryPrompt := getCategoryPrompt(settings)
		if categoryPrompt != "" {
			categoryBlock := dto.ClaudeMediaMessage{
				Type: "text",
			}
			categoryBlock.SetText(categoryPrompt)
			categoryBlock.CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
			systemBlocks = append(systemBlocks, categoryBlock)
		}
	}

	// Level 3: User's original system prompt (no cache marker to preserve flexibility)
	userSystem := req.GetStringSystem()
	if userSystem != "" {
		userBlock := dto.ClaudeMediaMessage{
			Type: "text",
		}
		userBlock.SetText(userSystem)
		systemBlocks = append(systemBlocks, userBlock)
	} else if !req.IsStringSystem() && req.System != nil {
		// If user already has structured system blocks, append them
		existingBlocks := req.ParseSystem()
		systemBlocks = append(systemBlocks, existingBlocks...)
	}

	// Update request system
	req.System = systemBlocks

	return nil
}

// getCategoryPrompt gets category-specific prompt if configured
func getCategoryPrompt(settings *dto.ChannelSettings) string {
	if settings.CategoryPrompts == nil || len(settings.CategoryPrompts) == 0 {
		return ""
	}

	// For now, use the first category prompt available
	// In future, this could be user-specific based on token metadata
	for _, prompt := range settings.CategoryPrompts {
		return prompt // Return first available category
	}

	return ""
}

// addHistoryCacheMarkers adds cache_control markers to historical messages
func addHistoryCacheMarkers(req *dto.ClaudeRequest, cacheCount int) {
	if len(req.Messages) <= 2 {
		return // Need at least 3 messages to cache history
	}

	// Calculate the index to add cache marker
	// Cache the message at position (len - cacheCount - 1)
	// This leaves the last N messages without cache
	targetIdx := len(req.Messages) - cacheCount - 1

	if targetIdx < 0 || targetIdx >= len(req.Messages) {
		return
	}

	// Add cache_control to the target message
	msg := &req.Messages[targetIdx]

	if msg.IsStringContent() {
		// Convert string content to structured content with cache marker
		content := msg.GetStringContent()
		structuredContent := []dto.ClaudeMediaMessage{
			{
				Type: "text",
			},
		}
		structuredContent[0].SetText(content)
		structuredContent[0].CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
		msg.Content = structuredContent
	} else {
		// Already structured content, add cache marker to last block
		content, err := msg.ParseContent()
		if err == nil && len(content) > 0 {
			content[len(content)-1].CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
			msg.Content = content
		}
	}
}

// estimateTokens provides a rough estimation of token count
// Used to validate padding content meets minimum threshold
func estimateTokens(text string) int {
	// Rough estimation: 1 token ≈ 4 characters for English
	// For mixed content, we use a conservative estimate
	return len(strings.ReplaceAll(text, " ", "")) / 3
}