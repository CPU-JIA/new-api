package common

import (
	"net/http"
	"net/http/httptest"
	"one-api/constant"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFullRequestURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		requestURL  string
		channelType int
		expected    string
	}{
		{
			name:        "Standard OpenAI URL",
			baseURL:     "https://api.openai.com",
			requestURL:  "/v1/chat/completions",
			channelType: constant.ChannelTypeOpenAI,
			expected:    "https://api.openai.com/v1/chat/completions",
		},
		{
			name:        "Cloudflare OpenAI Gateway",
			baseURL:     "https://gateway.ai.cloudflare.com",
			requestURL:  "/v1/chat/completions",
			channelType: constant.ChannelTypeOpenAI,
			expected:    "https://gateway.ai.cloudflare.com/chat/completions",
		},
		{
			name:        "Cloudflare Azure Gateway",
			baseURL:     "https://gateway.ai.cloudflare.com",
			requestURL:  "/openai/deployments/gpt-4/chat/completions",
			channelType: constant.ChannelTypeAzure,
			expected:    "https://gateway.ai.cloudflare.com/gpt-4/chat/completions",
		},
		{
			name:        "Azure Direct URL",
			baseURL:     "https://myresource.openai.azure.com",
			requestURL:  "/openai/deployments/gpt-4/chat/completions",
			channelType: constant.ChannelTypeAzure,
			expected:    "https://myresource.openai.azure.com/openai/deployments/gpt-4/chat/completions",
		},
		{
			name:        "Empty base URL",
			baseURL:     "",
			requestURL:  "/v1/chat/completions",
			channelType: constant.ChannelTypeOpenAI,
			expected:    "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFullRequestURL(tt.baseURL, tt.requestURL, tt.channelType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAPIVersion(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		context  string
		expected string
	}{
		{
			name:     "From query parameter",
			query:    "2023-12-01-preview",
			context:  "",
			expected: "2023-12-01-preview",
		},
		{
			name:     "From context when query empty",
			query:    "",
			context:  "2024-02-15-preview",
			expected: "2024-02-15-preview",
		},
		{
			name:     "Query parameter takes precedence",
			query:    "2023-12-01-preview",
			context:  "2024-02-15-preview",
			expected: "2023-12-01-preview",
		},
		{
			name:     "Both empty",
			query:    "",
			context:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test HTTP request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.query != "" {
				q := req.URL.Query()
				q.Set("api-version", tt.query)
				req.URL.RawQuery = q.Encode()
			}

			// Create Gin context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Set context value if provided
			if tt.context != "" {
				c.Set("api_version", tt.context)
			}

			result := GetAPIVersion(c)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidatePrompt(t *testing.T) {
	tests := []struct {
		name        string
		prompt      string
		expectError bool
		expectCode  string
	}{
		{
			name:        "Valid prompt",
			prompt:      "Hello, world!",
			expectError: false,
		},
		{
			name:        "Empty prompt",
			prompt:      "",
			expectError: true,
			expectCode:  "invalid_request",
		},
		{
			name:        "Whitespace only prompt",
			prompt:      "   \n\t  ",
			expectError: true,
			expectCode:  "invalid_request",
		},
		{
			name:        "Valid prompt with leading/trailing whitespace",
			prompt:      "  Hello, world!  ",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validatePrompt(tt.prompt)

			if tt.expectError {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectCode, result.Code)
				assert.Equal(t, http.StatusBadRequest, result.StatusCode)
				assert.True(t, result.LocalError)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}

func TestTaskSubmitReq_GetPrompt(t *testing.T) {
	req := TaskSubmitReq{
		Prompt: "Test prompt",
	}
	assert.Equal(t, "Test prompt", req.GetPrompt())
}

func TestTaskSubmitReq_HasImage(t *testing.T) {
	tests := []struct {
		name     string
		req      TaskSubmitReq
		expected bool
	}{
		{
			name: "No images",
			req: TaskSubmitReq{
				Images: []string{},
			},
			expected: false,
		},
		{
			name: "Has images",
			req: TaskSubmitReq{
				Images: []string{"image1.jpg", "image2.png"},
			},
			expected: true,
		},
		{
			name: "Nil images slice",
			req: TaskSubmitReq{
				Images: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.req.HasImage()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateTaskError(t *testing.T) {
	err := assert.AnError
	code := "test_error"
	statusCode := http.StatusBadRequest
	localError := true

	result := createTaskError(err, code, statusCode, localError)

	require.NotNil(t, result)
	assert.Equal(t, code, result.Code)
	assert.Equal(t, err.Error(), result.Message)
	assert.Equal(t, statusCode, result.StatusCode)
	assert.Equal(t, localError, result.LocalError)
	assert.Equal(t, err, result.Error)
}

func TestValidateBasicTaskRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		requestBody string
		info        *RelayInfo
		expectError bool
		expectCode  string
	}{
		{
			name:        "Valid request with prompt",
			requestBody: `{"prompt":"Hello world","model":"test-model"}`,
			info:        &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}, TaskRelayInfo: &TaskRelayInfo{}},
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			requestBody: `{"prompt":"Hello world"`,
			info:        &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}},
			expectError: true,
			expectCode:  "invalid_request",
		},
		{
			name:        "Empty prompt",
			requestBody: `{"prompt":"","model":"test-model"}`,
			info:        &RelayInfo{TaskRelayInfo: &TaskRelayInfo{}},
			expectError: true,
			expectCode:  "invalid_request",
		},
		{
			name:        "Request with single image",
			requestBody: `{"prompt":"Generate image","image":"base64imagedata","model":"test-model"}`,
			info:        &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}, TaskRelayInfo: &TaskRelayInfo{}},
			expectError: false,
		},
		{
			name:        "Request with multiple images",
			requestBody: `{"prompt":"Generate from images","images":["img1","img2"],"model":"test-model"}`,
			info:        &RelayInfo{ChannelMeta: &ChannelMeta{ChannelType: constant.ChannelTypeOpenAI}, TaskRelayInfo: &TaskRelayInfo{}},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("POST", "/test", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Create Gin context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			result := ValidateBasicTaskRequest(c, tt.info, "generate")

			if tt.expectError {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectCode, result.Code)
			} else {
				assert.Nil(t, result)
			}
		})
	}
}