package common

import (
	"net/http/httptest"
	"one-api/constant"
	"one-api/dto"
	"one-api/types"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayInfo_SetPromptTokens(t *testing.T) {
	info := &RelayInfo{}
	tokens := 150

	info.SetPromptTokens(tokens)

	assert.Equal(t, tokens, info.PromptTokens)
}

func TestRelayInfo_SetFirstResponseTime(t *testing.T) {
	info := &RelayInfo{
		isFirstResponse: true,
		StartTime:       time.Now(),
	}

	// First call should set the time
	before := time.Now()
	info.SetFirstResponseTime()
	after := time.Now()

	assert.False(t, info.isFirstResponse)
	assert.True(t, info.FirstResponseTime.After(before) || info.FirstResponseTime.Equal(before))
	assert.True(t, info.FirstResponseTime.Before(after) || info.FirstResponseTime.Equal(after))

	// Second call should not change the time
	previousTime := info.FirstResponseTime
	time.Sleep(time.Millisecond) // Ensure some time passes
	info.SetFirstResponseTime()

	assert.Equal(t, previousTime, info.FirstResponseTime)
}

func TestRelayInfo_HasSendResponse(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		startTime         time.Time
		firstResponseTime time.Time
		expected          bool
	}{
		{
			name:              "Response sent",
			startTime:         now,
			firstResponseTime: now.Add(time.Second),
			expected:          true,
		},
		{
			name:              "No response sent",
			startTime:         now,
			firstResponseTime: now.Add(-time.Second),
			expected:          false,
		},
		{
			name:              "Response at start time",
			startTime:         now,
			firstResponseTime: now,
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &RelayInfo{
				StartTime:         tt.startTime,
				FirstResponseTime: tt.firstResponseTime,
			}

			result := info.HasSendResponse()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRelayInfo_ToString(t *testing.T) {
	info := &RelayInfo{
		RelayFormat:           types.RelayFormatOpenAI,
		RelayMode:             1,
		IsStream:              true,
		IsPlayground:          false,
		RequestURLPath:        "/v1/chat/completions",
		OriginModelName:       "gpt-4",
		PromptTokens:          100,
		ShouldIncludeUsage:    true,
		DisablePing:           false,
		SendResponseCount:     1,
		FinalPreConsumedQuota: 50,
		UserId:                123,
		UserEmail:             "test@example.com",
		UserGroup:             "default",
		UsingGroup:            "premium",
		UserQuota:             1000,
		TokenId:               456,
		TokenUnlimited:        false,
		StartTime:             time.Now(),
		FirstResponseTime:     time.Now().Add(time.Millisecond * 100),
	}

	result := info.ToString()

	// Test that key information is present
	assert.Contains(t, result, "RelayInfo{")
	assert.Contains(t, result, "RelayFormat: openai")
	assert.Contains(t, result, "IsStream: true")
	assert.Contains(t, result, "OriginModelName: \"gpt-4\"")
	assert.Contains(t, result, "PromptTokens: 100")
	assert.Contains(t, result, "User{ Id: 123")
	assert.Contains(t, result, "***@example.com") // Email should be masked (actual format)
	assert.Contains(t, result, "Token{ Id: 456")
	assert.Contains(t, result, "Key: ***masked***") // Token key should be masked
}

func TestRelayInfo_ToString_Nil(t *testing.T) {
	var info *RelayInfo
	result := info.ToString()
	assert.Equal(t, "RelayInfo<nil>", result)
}

func TestRelayInfo_InitChannelMeta(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test context
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Set context values
	c.Set(string(constant.ContextKeyChannelType), constant.ChannelTypeOpenAI)
	c.Set(string(constant.ContextKeyChannelId), 123)
	c.Set(string(constant.ContextKeyChannelIsMultiKey), false)
	c.Set(string(constant.ContextKeyChannelMultiKeyIndex), 0)
	c.Set(string(constant.ContextKeyChannelBaseUrl), "https://api.openai.com")
	c.Set(string(constant.ContextKeyChannelKey), "sk-test-key")
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-4")

	info := &RelayInfo{}
	info.InitChannelMeta(c)

	require.NotNil(t, info.ChannelMeta)
	assert.Equal(t, constant.ChannelTypeOpenAI, info.ChannelMeta.ChannelType)
	assert.Equal(t, 123, info.ChannelMeta.ChannelId)
	assert.Equal(t, false, info.ChannelMeta.ChannelIsMultiKey)
	assert.Equal(t, "https://api.openai.com", info.ChannelMeta.ChannelBaseUrl)
	assert.Equal(t, "gpt-4", info.ChannelMeta.UpstreamModelName)
	assert.True(t, info.ChannelMeta.SupportStreamOptions) // OpenAI supports stream options
}

func TestTaskSubmitReq_Interfaces(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "Test prompt",
		Images: []string{"img1.jpg"},
	}

	// Test HasPrompt interface
	var hasPrompt HasPrompt = req
	assert.Equal(t, "Test prompt", hasPrompt.GetPrompt())

	// Test HasImage interface
	var hasImage HasImage = req
	assert.True(t, hasImage.HasImage())
}

func TestGenRelayInfoOpenAI(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Set required context values
	c.Set(string(constant.ContextKeyRequestStartTime), time.Now())
	c.Set(string(constant.ContextKeyUserId), 123)
	c.Set(string(constant.ContextKeyOriginalModel), "gpt-4")

	// Mock request
	mockRequest := &dto.GeneralOpenAIRequest{
		Model: "gpt-4",
	}

	info := GenRelayInfoOpenAI(c, mockRequest)

	require.NotNil(t, info)
	assert.Equal(t, types.RelayFormatOpenAI, info.RelayFormat)
	assert.Equal(t, 123, info.UserId)
	assert.Equal(t, "gpt-4", info.OriginModelName)
	assert.NotNil(t, info.Request)
}

func TestGenRelayInfoClaude(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/v1/messages", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Set required context values
	c.Set(string(constant.ContextKeyRequestStartTime), time.Now())
	c.Set(string(constant.ContextKeyUserId), 123)

	// Mock request
	mockRequest := &dto.ClaudeRequest{
		Model: "claude-3-sonnet",
	}

	info := GenRelayInfoClaude(c, mockRequest)

	require.NotNil(t, info)
	assert.Equal(t, types.RelayFormat("claude"), info.RelayFormat)
	assert.False(t, info.ShouldIncludeUsage)
	assert.NotNil(t, info.ClaudeConvertInfo)
	assert.Equal(t, LastMessageTypeNone, info.ClaudeConvertInfo.LastMessagesType)
}

func TestGenRelayInfoEmbedding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/v1/embeddings", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Set required context values
	c.Set(string(constant.ContextKeyRequestStartTime), time.Now())
	c.Set(string(constant.ContextKeyUserId), 123)

	// Mock request
	mockRequest := &dto.EmbeddingRequest{
		Model: "text-embedding-ada-002",
	}

	info := GenRelayInfoEmbedding(c, mockRequest)

	require.NotNil(t, info)
	assert.Equal(t, types.RelayFormat("embedding"), info.RelayFormat)
	assert.Equal(t, 123, info.UserId)
}

func TestGenRelayInfo_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	info, err := GenRelayInfo(c, types.RelayFormat("invalid"), nil, nil)

	assert.Nil(t, info)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid relay format")
}

func TestStreamSupportedChannels(t *testing.T) {
	// Test some known supported channels
	supportedChannels := []int{
		constant.ChannelTypeOpenAI,
		constant.ChannelTypeAnthropic,
		constant.ChannelTypeAzure,
		constant.ChannelTypeGemini,
	}

	for _, channelType := range supportedChannels {
		assert.True(t, streamSupportedChannels[channelType],
			"Channel type %d should support stream options", channelType)
	}

	// Test a channel that shouldn't exist
	assert.False(t, streamSupportedChannels[99999],
		"Non-existent channel type should not support stream options")
}