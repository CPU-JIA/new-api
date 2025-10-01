package claude

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"one-api/common"
	"one-api/constant"
	"one-api/dto"
	"one-api/logger"
	"one-api/relay/channel/openrouter"
	relaycommon "one-api/relay/common"
	"one-api/relay/helper"
	"one-api/service"
	"one-api/setting/model_setting"
	"one-api/types"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	WebSearchMaxUsesLow    = 1
	WebSearchMaxUsesMedium = 5
	WebSearchMaxUsesHigh   = 10
)

func stopReasonClaude2OpenAI(reason string) string {
	switch reason {
	case "stop_sequence":
		return "stop"
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return reason
	}
}

func RequestOpenAI2ClaudeComplete(textRequest dto.GeneralOpenAIRequest) *dto.ClaudeRequest {

	claudeRequest := dto.ClaudeRequest{
		Model:         textRequest.Model,
		Prompt:        "",
		StopSequences: nil,
		Temperature:   textRequest.Temperature,
		TopP:          textRequest.TopP,
		TopK:          textRequest.TopK,
		Stream:        textRequest.Stream,
	}
	if claudeRequest.MaxTokensToSample == 0 {
		claudeRequest.MaxTokensToSample = 4096
	}
	prompt := ""
	for _, message := range textRequest.Messages {
		if message.Role == "user" {
			prompt += fmt.Sprintf("\n\nHuman: %s", message.StringContent())
		} else if message.Role == "assistant" {
			prompt += fmt.Sprintf("\n\nAssistant: %s", message.StringContent())
		} else if message.Role == "system" {
			if prompt == "" {
				prompt = message.StringContent()
			}
		}
	}
	prompt += "\n\nAssistant:"
	claudeRequest.Prompt = prompt
	return &claudeRequest
}

func RequestOpenAI2ClaudeMessage(c *gin.Context, textRequest dto.GeneralOpenAIRequest) (*dto.ClaudeRequest, error) {
	claudeTools := make([]any, 0, len(textRequest.Tools))

	for _, tool := range textRequest.Tools {
		if params, ok := tool.Function.Parameters.(map[string]any); ok {
			claudeTool := dto.Tool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
			}
			claudeTool.InputSchema = make(map[string]interface{})
			if params["type"] != nil {
				claudeTool.InputSchema["type"] = params["type"].(string)
			}
			claudeTool.InputSchema["properties"] = params["properties"]
			claudeTool.InputSchema["required"] = params["required"]
			for s, a := range params {
				if s == "type" || s == "properties" || s == "required" {
					continue
				}
				claudeTool.InputSchema[s] = a
			}
			claudeTools = append(claudeTools, &claudeTool)
		}
	}

	// Web search tool
	// https://docs.anthropic.com/en/docs/agents-and-tools/tool-use/web-search-tool
	if textRequest.WebSearchOptions != nil {
		webSearchTool := dto.ClaudeWebSearchTool{
			Type: "web_search_20250305",
			Name: "web_search",
		}

		// 处理 user_location
		if textRequest.WebSearchOptions.UserLocation != nil {
			anthropicUserLocation := &dto.ClaudeWebSearchUserLocation{
				Type: "approximate", // 固定为 "approximate"
			}

			// 解析 UserLocation JSON
			var userLocationMap map[string]interface{}
			if err := json.Unmarshal(textRequest.WebSearchOptions.UserLocation, &userLocationMap); err == nil {
				// 检查是否有 approximate 字段
				if approximateData, ok := userLocationMap["approximate"].(map[string]interface{}); ok {
					if timezone, ok := approximateData["timezone"].(string); ok && timezone != "" {
						anthropicUserLocation.Timezone = timezone
					}
					if country, ok := approximateData["country"].(string); ok && country != "" {
						anthropicUserLocation.Country = country
					}
					if region, ok := approximateData["region"].(string); ok && region != "" {
						anthropicUserLocation.Region = region
					}
					if city, ok := approximateData["city"].(string); ok && city != "" {
						anthropicUserLocation.City = city
					}
				}
			}

			webSearchTool.UserLocation = anthropicUserLocation
		}

		// 处理 search_context_size 转换为 max_uses
		if textRequest.WebSearchOptions.SearchContextSize != "" {
			switch textRequest.WebSearchOptions.SearchContextSize {
			case "low":
				webSearchTool.MaxUses = WebSearchMaxUsesLow
			case "medium":
				webSearchTool.MaxUses = WebSearchMaxUsesMedium
			case "high":
				webSearchTool.MaxUses = WebSearchMaxUsesHigh
			}
		}

		claudeTools = append(claudeTools, &webSearchTool)
	}

	claudeRequest := dto.ClaudeRequest{
		Model:         textRequest.Model,
		MaxTokens:     textRequest.GetMaxTokens(),
		StopSequences: nil,
		Temperature:   textRequest.Temperature,
		TopP:          textRequest.TopP,
		TopK:          textRequest.TopK,
		Stream:        textRequest.Stream,
		Tools:         claudeTools,
	}

	// 处理 tool_choice 和 parallel_tool_calls
	if textRequest.ToolChoice != nil || textRequest.ParallelTooCalls != nil {
		claudeToolChoice := mapToolChoice(textRequest.ToolChoice, textRequest.ParallelTooCalls)
		if claudeToolChoice != nil {
			claudeRequest.ToolChoice = claudeToolChoice
		}
	}

	if claudeRequest.MaxTokens == 0 {
		claudeRequest.MaxTokens = uint(model_setting.GetClaudeSettings().GetDefaultMaxTokens(textRequest.Model))
	}

	if model_setting.GetClaudeSettings().ThinkingAdapterEnabled &&
		strings.HasSuffix(textRequest.Model, "-thinking") {

		// 因为BudgetTokens 必须大于1024
		if claudeRequest.MaxTokens < 1280 {
			claudeRequest.MaxTokens = 1280
		}

		// BudgetTokens 为 max_tokens 的 80%
		claudeRequest.Thinking = &dto.Thinking{
			Type:         "enabled",
			BudgetTokens: common.GetPointer[int](int(float64(claudeRequest.MaxTokens) * model_setting.GetClaudeSettings().ThinkingAdapterBudgetTokensPercentage)),
		}
		// TODO: 临时处理
		// https://docs.anthropic.com/en/docs/build-with-claude/extended-thinking#important-considerations-when-using-extended-thinking
		claudeRequest.TopP = 0
		claudeRequest.Temperature = common.GetPointer[float64](1.0)
		claudeRequest.Model = strings.TrimSuffix(textRequest.Model, "-thinking")
	}

	if textRequest.ReasoningEffort != "" {
		switch textRequest.ReasoningEffort {
		case "low":
			claudeRequest.Thinking = &dto.Thinking{
				Type:         "enabled",
				BudgetTokens: common.GetPointer[int](1280),
			}
		case "medium":
			claudeRequest.Thinking = &dto.Thinking{
				Type:         "enabled",
				BudgetTokens: common.GetPointer[int](2048),
			}
		case "high":
			claudeRequest.Thinking = &dto.Thinking{
				Type:         "enabled",
				BudgetTokens: common.GetPointer[int](4096),
			}
		}
	}

	// 指定了 reasoning 参数,覆盖 budgetTokens
	if textRequest.Reasoning != nil {
		var reasoning openrouter.RequestReasoning
		if err := common.Unmarshal(textRequest.Reasoning, &reasoning); err != nil {
			return nil, err
		}

		budgetTokens := reasoning.MaxTokens
		if budgetTokens > 0 {
			claudeRequest.Thinking = &dto.Thinking{
				Type:         "enabled",
				BudgetTokens: &budgetTokens,
			}
		}
	}

	if textRequest.Stop != nil {
		// stop maybe string/array string, convert to array string
		switch textRequest.Stop.(type) {
		case string:
			claudeRequest.StopSequences = []string{textRequest.Stop.(string)}
		case []interface{}:
			stopSequences := make([]string, 0)
			for _, stop := range textRequest.Stop.([]interface{}) {
				stopSequences = append(stopSequences, stop.(string))
			}
			claudeRequest.StopSequences = stopSequences
		}
	}
	formatMessages := make([]dto.Message, 0)
	lastMessage := dto.Message{
		Role: "tool",
	}
	for i, message := range textRequest.Messages {
		if message.Role == "" {
			textRequest.Messages[i].Role = "user"
		}
		fmtMessage := dto.Message{
			Role:    message.Role,
			Content: message.Content,
		}
		if message.Role == "tool" {
			fmtMessage.ToolCallId = message.ToolCallId
		}
		if message.Role == "assistant" && message.ToolCalls != nil {
			fmtMessage.ToolCalls = message.ToolCalls
		}
		if lastMessage.Role == message.Role && lastMessage.Role != "tool" {
			if lastMessage.IsStringContent() && message.IsStringContent() {
				fmtMessage.SetStringContent(strings.Trim(fmt.Sprintf("%s %s", lastMessage.StringContent(), message.StringContent()), "\""))
				// delete last message
				formatMessages = formatMessages[:len(formatMessages)-1]
			}
		}
		if fmtMessage.Content == nil {
			fmtMessage.SetStringContent("...")
		}
		formatMessages = append(formatMessages, fmtMessage)
		lastMessage = fmtMessage
	}

	claudeMessages := make([]dto.ClaudeMessage, 0)
	isFirstMessage := true
	// 初始化system消息数组，用于累积多个system消息
	var systemMessages []dto.ClaudeMediaMessage

	for _, message := range formatMessages {
		if message.Role == "system" {
			// 根据Claude API规范，system字段使用数组格式更有通用性
			if message.IsStringContent() {
				systemMessages = append(systemMessages, dto.ClaudeMediaMessage{
					Type: "text",
					Text: common.GetPointer[string](message.StringContent()),
				})
			} else {
				// 支持复合内容的system消息（虽然不常见，但需要考虑完整性）
				for _, ctx := range message.ParseContent() {
					if ctx.Type == "text" {
						systemMessages = append(systemMessages, dto.ClaudeMediaMessage{
							Type: "text",
							Text: common.GetPointer[string](ctx.Text),
						})
					}
					// 未来可以在这里扩展对图片等其他类型的支持
				}
			}
		} else {
			if isFirstMessage {
				isFirstMessage = false
				if message.Role != "user" {
					// fix: first message is assistant, add user message
					claudeMessage := dto.ClaudeMessage{
						Role: "user",
						Content: []dto.ClaudeMediaMessage{
							{
								Type: "text",
								Text: common.GetPointer[string]("..."),
							},
						},
					}
					claudeMessages = append(claudeMessages, claudeMessage)
				}
			}
			claudeMessage := dto.ClaudeMessage{
				Role: message.Role,
			}
			if message.Role == "tool" {
				if len(claudeMessages) > 0 && claudeMessages[len(claudeMessages)-1].Role == "user" {
					lastMessage := claudeMessages[len(claudeMessages)-1]
					if content, ok := lastMessage.Content.(string); ok {
						lastMessage.Content = []dto.ClaudeMediaMessage{
							{
								Type: "text",
								Text: common.GetPointer[string](content),
							},
						}
					}
					lastMessage.Content = append(lastMessage.Content.([]dto.ClaudeMediaMessage), dto.ClaudeMediaMessage{
						Type:      "tool_result",
						ToolUseId: message.ToolCallId,
						Content:   message.Content,
					})
					claudeMessages[len(claudeMessages)-1] = lastMessage
					continue
				} else {
					claudeMessage.Role = "user"
					claudeMessage.Content = []dto.ClaudeMediaMessage{
						{
							Type:      "tool_result",
							ToolUseId: message.ToolCallId,
							Content:   message.Content,
						},
					}
				}
			} else if message.IsStringContent() && message.ToolCalls == nil {
				claudeMessage.Content = message.StringContent()
			} else {
				claudeMediaMessages := make([]dto.ClaudeMediaMessage, 0)
				for _, mediaMessage := range message.ParseContent() {
					claudeMediaMessage := dto.ClaudeMediaMessage{
						Type: mediaMessage.Type,
					}
					if mediaMessage.Type == "text" {
						claudeMediaMessage.Text = common.GetPointer[string](mediaMessage.Text)
					} else {
						imageUrl := mediaMessage.GetImageMedia()
						claudeMediaMessage.Type = "image"
						claudeMediaMessage.Source = &dto.ClaudeMessageSource{
							Type: "base64",
						}
						// 判断是否是url
						if strings.HasPrefix(imageUrl.Url, "http") {
							// 是url，获取图片的类型和base64编码的数据
							fileData, err := service.GetFileBase64FromUrl(c, imageUrl.Url, "formatting image for Claude")
							if err != nil {
								return nil, fmt.Errorf("get file base64 from url failed: %s", err.Error())
							}
							claudeMediaMessage.Source.MediaType = fileData.MimeType
							claudeMediaMessage.Source.Data = fileData.Base64Data
						} else {
							_, format, base64String, err := service.DecodeBase64ImageData(imageUrl.Url)
							if err != nil {
								return nil, err
							}
							claudeMediaMessage.Source.MediaType = "image/" + format
							claudeMediaMessage.Source.Data = base64String
						}
					}
					claudeMediaMessages = append(claudeMediaMessages, claudeMediaMessage)
				}
				if message.ToolCalls != nil {
					for _, toolCall := range message.ParseToolCalls() {
						inputObj := make(map[string]any)
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &inputObj); err != nil {
							common.SysLog("tool call function arguments is not a map[string]any: " + fmt.Sprintf("%v", toolCall.Function.Arguments))
							continue
						}
						claudeMediaMessages = append(claudeMediaMessages, dto.ClaudeMediaMessage{
							Type:  "tool_use",
							Id:    toolCall.ID,
							Name:  toolCall.Function.Name,
							Input: inputObj,
						})
					}
				}
				claudeMessage.Content = claudeMediaMessages
			}
			claudeMessages = append(claudeMessages, claudeMessage)
		}
	}

	// 设置累积的system消息
	if len(systemMessages) > 0 {
		claudeRequest.System = systemMessages
	}

	claudeRequest.Prompt = ""
	claudeRequest.Messages = claudeMessages

	// Apply pool cache optimization if enabled
	applyPoolCacheToClaudeRequest(c, &claudeRequest)

	if common.DebugEnabled {
		if channelSetting, ok := common.GetContextKeyType[dto.ChannelSettings](c, "channel_setting"); ok {
			if channelSetting.EnablePoolCacheOptimization {
				systemBlockCount := 0
				if claudeRequest.System != nil {
					systemBlocks := claudeRequest.ParseSystem()
					systemBlockCount = len(systemBlocks)
				}
				common.SysLog(fmt.Sprintf("PoolCache: Relay layer applied, system_blocks=%d, messages=%d",
					systemBlockCount, len(claudeRequest.Messages)))
			}
		}
	}

	return &claudeRequest, nil
}

func StreamResponseClaude2OpenAI(reqMode int, claudeResponse *dto.ClaudeResponse) *dto.ChatCompletionsStreamResponse {
	var response dto.ChatCompletionsStreamResponse
	response.Object = "chat.completion.chunk"
	response.Model = claudeResponse.Model
	response.Choices = make([]dto.ChatCompletionsStreamResponseChoice, 0)
	tools := make([]dto.ToolCallResponse, 0)
	fcIdx := 0
	if claudeResponse.Index != nil {
		fcIdx = *claudeResponse.Index - 1
		if fcIdx < 0 {
			fcIdx = 0
		}
	}
	var choice dto.ChatCompletionsStreamResponseChoice
	if reqMode == RequestModeCompletion {
		choice.Delta.SetContentString(claudeResponse.Completion)
		finishReason := stopReasonClaude2OpenAI(claudeResponse.StopReason)
		if finishReason != "null" {
			choice.FinishReason = &finishReason
		}
	} else {
		if claudeResponse.Type == "message_start" {
			response.Id = claudeResponse.Message.Id
			response.Model = claudeResponse.Message.Model
			//claudeUsage = &claudeResponse.Message.Usage
			choice.Delta.SetContentString("")
			choice.Delta.Role = "assistant"
		} else if claudeResponse.Type == "content_block_start" {
			if claudeResponse.ContentBlock != nil {
				// 如果是文本块，尽可能发送首段文本（若存在）
				if claudeResponse.ContentBlock.Type == "text" && claudeResponse.ContentBlock.Text != nil {
					choice.Delta.SetContentString(*claudeResponse.ContentBlock.Text)
				}
				if claudeResponse.ContentBlock.Type == "tool_use" {
					tools = append(tools, dto.ToolCallResponse{
						Index: common.GetPointer(fcIdx),
						ID:    claudeResponse.ContentBlock.Id,
						Type:  "function",
						Function: dto.FunctionResponse{
							Name:      claudeResponse.ContentBlock.Name,
							Arguments: "",
						},
					})
				}
			} else {
				return nil
			}
		} else if claudeResponse.Type == "content_block_delta" {
			if claudeResponse.Delta != nil {
				choice.Delta.Content = claudeResponse.Delta.Text
				switch claudeResponse.Delta.Type {
				case "input_json_delta":
					tools = append(tools, dto.ToolCallResponse{
						Type:  "function",
						Index: common.GetPointer(fcIdx),
						Function: dto.FunctionResponse{
							Arguments: *claudeResponse.Delta.PartialJson,
						},
					})
				case "signature_delta":
					// 加密的不处理
					signatureContent := "\n"
					choice.Delta.ReasoningContent = &signatureContent
				case "thinking_delta":
					thinkingContent := claudeResponse.Delta.Thinking
					choice.Delta.ReasoningContent = &thinkingContent
				}
			}
		} else if claudeResponse.Type == "message_delta" {
			finishReason := stopReasonClaude2OpenAI(*claudeResponse.Delta.StopReason)
			if finishReason != "null" {
				choice.FinishReason = &finishReason
			}
			//claudeUsage = &claudeResponse.Usage
		} else if claudeResponse.Type == "message_stop" {
			return nil
		} else {
			return nil
		}
	}
	if len(tools) > 0 {
		choice.Delta.Content = nil // compatible with other OpenAI derivative applications, like LobeOpenAICompatibleFactory ...
		choice.Delta.ToolCalls = tools
	}
	response.Choices = append(response.Choices, choice)

	return &response
}

func ResponseClaude2OpenAI(reqMode int, claudeResponse *dto.ClaudeResponse) *dto.OpenAITextResponse {
	choices := make([]dto.OpenAITextResponseChoice, 0)
	fullTextResponse := dto.OpenAITextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", common.GetUUID()),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
	}
	var responseText string
	var responseThinking string
	if len(claudeResponse.Content) > 0 {
		responseText = claudeResponse.Content[0].GetText()
		responseThinking = claudeResponse.Content[0].Thinking
	}
	tools := make([]dto.ToolCallResponse, 0)
	thinkingContent := ""

	if reqMode == RequestModeCompletion {
		choice := dto.OpenAITextResponseChoice{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: strings.TrimPrefix(claudeResponse.Completion, " "),
				Name:    nil,
			},
			FinishReason: stopReasonClaude2OpenAI(claudeResponse.StopReason),
		}
		choices = append(choices, choice)
	} else {
		fullTextResponse.Id = claudeResponse.Id
		for _, message := range claudeResponse.Content {
			switch message.Type {
			case "tool_use":
				args, _ := json.Marshal(message.Input)
				tools = append(tools, dto.ToolCallResponse{
					ID:   message.Id,
					Type: "function", // compatible with other OpenAI derivative applications
					Function: dto.FunctionResponse{
						Name:      message.Name,
						Arguments: string(args),
					},
				})
			case "thinking":
				// 加密的不管， 只输出明文的推理过程
				thinkingContent = message.Thinking
			case "text":
				responseText = message.GetText()
			}
		}
	}
	choice := dto.OpenAITextResponseChoice{
		Index: 0,
		Message: dto.Message{
			Role: "assistant",
		},
		FinishReason: stopReasonClaude2OpenAI(claudeResponse.StopReason),
	}
	choice.SetStringContent(responseText)
	if len(responseThinking) > 0 {
		choice.ReasoningContent = responseThinking
	}
	if len(tools) > 0 {
		choice.Message.SetToolCalls(tools)
	}
	choice.Message.ReasoningContent = thinkingContent
	fullTextResponse.Model = claudeResponse.Model
	choices = append(choices, choice)
	fullTextResponse.Choices = choices
	return &fullTextResponse
}

type ClaudeResponseInfo struct {
	ResponseId   string
	Created      int64
	Model        string
	ResponseText strings.Builder
	Usage        *dto.Usage
	Done         bool
}

func FormatClaudeResponseInfo(requestMode int, claudeResponse *dto.ClaudeResponse, oaiResponse *dto.ChatCompletionsStreamResponse, claudeInfo *ClaudeResponseInfo) bool {
	if requestMode == RequestModeCompletion {
		claudeInfo.ResponseText.WriteString(claudeResponse.Completion)
	} else {
		if claudeResponse.Type == "message_start" {
			claudeInfo.ResponseId = claudeResponse.Message.Id
			claudeInfo.Model = claudeResponse.Message.Model

			// message_start, 获取usage
			claudeInfo.Usage.PromptTokens = claudeResponse.Message.Usage.InputTokens
			claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Message.Usage.CacheReadInputTokens
			claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Message.Usage.CacheCreationInputTokens
			claudeInfo.Usage.CompletionTokens = claudeResponse.Message.Usage.OutputTokens
		} else if claudeResponse.Type == "content_block_delta" {
			if claudeResponse.Delta.Text != nil {
				claudeInfo.ResponseText.WriteString(*claudeResponse.Delta.Text)
			}
			if claudeResponse.Delta.Thinking != "" {
				claudeInfo.ResponseText.WriteString(claudeResponse.Delta.Thinking)
			}
		} else if claudeResponse.Type == "message_delta" {
			// 最终的usage获取
			if claudeResponse.Usage.InputTokens > 0 {
				// 不叠加，只取最新的
				claudeInfo.Usage.PromptTokens = claudeResponse.Usage.InputTokens
			}
			claudeInfo.Usage.CompletionTokens = claudeResponse.Usage.OutputTokens
			claudeInfo.Usage.TotalTokens = claudeInfo.Usage.PromptTokens + claudeInfo.Usage.CompletionTokens

			// 判断是否完整
			claudeInfo.Done = true
		} else if claudeResponse.Type == "content_block_start" {
		} else {
			return false
		}
	}
	if oaiResponse != nil {
		oaiResponse.Id = claudeInfo.ResponseId
		oaiResponse.Created = claudeInfo.Created
		oaiResponse.Model = claudeInfo.Model
	}
	return true
}

func HandleStreamResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, data string, requestMode int) *types.NewAPIError {
	var claudeResponse dto.ClaudeResponse
	err := common.UnmarshalJsonStr(data, &claudeResponse)
	if err != nil {
		common.SysLog("error unmarshalling stream response: " + err.Error())
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	if info.RelayFormat == types.RelayFormatClaude {
		FormatClaudeResponseInfo(requestMode, &claudeResponse, nil, claudeInfo)

		if requestMode == RequestModeCompletion {
		} else {
			if claudeResponse.Type == "message_start" {
				// message_start, 获取usage
				info.UpstreamModelName = claudeResponse.Message.Model
			} else if claudeResponse.Type == "content_block_delta" {
			} else if claudeResponse.Type == "message_delta" {
			}
		}
		helper.ClaudeChunkData(c, claudeResponse, data)
	} else if info.RelayFormat == types.RelayFormatOpenAI {
		response := StreamResponseClaude2OpenAI(requestMode, &claudeResponse)

		if !FormatClaudeResponseInfo(requestMode, &claudeResponse, response, claudeInfo) {
			return nil
		}

		err = helper.ObjectData(c, response)
		if err != nil {
			logger.LogError(c, "send_stream_response_failed: "+err.Error())
		}
	}
	return nil
}

func HandleStreamFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, requestMode int) {

	if requestMode == RequestModeCompletion {
		claudeInfo.Usage = service.ResponseText2Usage(claudeInfo.ResponseText.String(), info.UpstreamModelName, info.PromptTokens)
	} else {
		if claudeInfo.Usage.PromptTokens == 0 {
			//上游出错
		}
		if claudeInfo.Usage.CompletionTokens == 0 || !claudeInfo.Done {
			if common.DebugEnabled {
				common.SysLog("claude response usage is not complete, maybe upstream error")
			}
			claudeInfo.Usage = service.ResponseText2Usage(claudeInfo.ResponseText.String(), info.UpstreamModelName, claudeInfo.Usage.PromptTokens)
		}
	}

	if info.RelayFormat == types.RelayFormatClaude {
		//
	} else if info.RelayFormat == types.RelayFormatOpenAI {
		if info.ShouldIncludeUsage {
			response := helper.GenerateFinalUsageResponse(claudeInfo.ResponseId, claudeInfo.Created, info.UpstreamModelName, *claudeInfo.Usage)
			err := helper.ObjectData(c, response)
			if err != nil {
				common.SysLog("send final response failed: " + err.Error())
			}
		}
		helper.Done(c)
	}
}

func ClaudeStreamHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, requestMode int) (*dto.Usage, *types.NewAPIError) {
	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	var err *types.NewAPIError
	helper.StreamScannerHandler(c, resp, info, func(data string) bool {
		err = HandleStreamResponseData(c, info, claudeInfo, data, requestMode)
		if err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}

	HandleStreamFinalResponse(c, info, claudeInfo, requestMode)
	return claudeInfo.Usage, nil
}

func HandleClaudeResponseData(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo, httpResp *http.Response, data []byte, requestMode int) *types.NewAPIError {
	var claudeResponse dto.ClaudeResponse
	err := common.Unmarshal(data, &claudeResponse)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if claudeError := claudeResponse.GetClaudeError(); claudeError != nil && claudeError.Type != "" {
		return types.WithClaudeError(*claudeError, http.StatusInternalServerError)
	}
	if requestMode == RequestModeCompletion {
		completionTokens := service.CountTextToken(claudeResponse.Completion, info.OriginModelName)
		claudeInfo.Usage.PromptTokens = info.PromptTokens
		claudeInfo.Usage.CompletionTokens = completionTokens
		claudeInfo.Usage.TotalTokens = info.PromptTokens + completionTokens
	} else {
		claudeInfo.Usage.PromptTokens = claudeResponse.Usage.InputTokens
		claudeInfo.Usage.CompletionTokens = claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.TotalTokens = claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens
		claudeInfo.Usage.PromptTokensDetails.CachedTokens = claudeResponse.Usage.CacheReadInputTokens
		claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens = claudeResponse.Usage.CacheCreationInputTokens
	}
	var responseData []byte
	switch info.RelayFormat {
	case types.RelayFormatOpenAI:
		openaiResponse := ResponseClaude2OpenAI(requestMode, &claudeResponse)
		openaiResponse.Usage = *claudeInfo.Usage
		responseData, err = json.Marshal(openaiResponse)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
	case types.RelayFormatClaude:
		responseData = data
	}

	if claudeResponse.Usage.ServerToolUse != nil && claudeResponse.Usage.ServerToolUse.WebSearchRequests > 0 {
		c.Set("claude_web_search_requests", claudeResponse.Usage.ServerToolUse.WebSearchRequests)
	}

	service.IOCopyBytesGracefully(c, httpResp, responseData)
	return nil
}

func ClaudeHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, requestMode int) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	claudeInfo := &ClaudeResponseInfo{
		ResponseId:   helper.GetResponseID(c),
		Created:      common.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if common.DebugEnabled {
		println("responseBody: ", string(responseBody))
	}
	handleErr := HandleClaudeResponseData(c, info, claudeInfo, resp, responseBody, requestMode)
	if handleErr != nil {
		return nil, handleErr
	}
	return claudeInfo.Usage, nil
}

func mapToolChoice(toolChoice any, parallelToolCalls *bool) *dto.ClaudeToolChoice {
	var claudeToolChoice *dto.ClaudeToolChoice

	// 处理 tool_choice 字符串值
	if toolChoiceStr, ok := toolChoice.(string); ok {
		switch toolChoiceStr {
		case "auto":
			claudeToolChoice = &dto.ClaudeToolChoice{
				Type: "auto",
			}
		case "required":
			claudeToolChoice = &dto.ClaudeToolChoice{
				Type: "any",
			}
		case "none":
			claudeToolChoice = &dto.ClaudeToolChoice{
				Type: "none",
			}
		}
	} else if toolChoiceMap, ok := toolChoice.(map[string]interface{}); ok {
		// 处理 tool_choice 对象值
		if function, ok := toolChoiceMap["function"].(map[string]interface{}); ok {
			if toolName, ok := function["name"].(string); ok {
				claudeToolChoice = &dto.ClaudeToolChoice{
					Type: "tool",
					Name: toolName,
				}
			}
		}
	}

	// 处理 parallel_tool_calls
	if parallelToolCalls != nil {
		if claudeToolChoice == nil {
			// 如果没有 tool_choice，但有 parallel_tool_calls，创建默认的 auto 类型
			claudeToolChoice = &dto.ClaudeToolChoice{
				Type: "auto",
			}
		}

		// 设置 disable_parallel_tool_use
		// 如果 parallel_tool_calls 为 true，则 disable_parallel_tool_use 为 false
		claudeToolChoice.DisableParallelToolUse = !*parallelToolCalls
	}

	return claudeToolChoice
}

// applyPoolCacheToClaudeRequest applies pool cache optimization to ClaudeRequest
func applyPoolCacheToClaudeRequest(c *gin.Context, request *dto.ClaudeRequest) {
	// CRITICAL: Verify model supports Prompt Caching
	// Reference: https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching
	if !constant.IsClaudeModelSupportCache(request.Model) {
		if common.DebugEnabled {
			common.SysLog(fmt.Sprintf("CacheOptimization: Model %s does not support prompt caching, skipping cache padding", request.Model))
		}
		return
	}

	// Get channel settings from context
	channelSetting, ok := common.GetContextKeyType[dto.ChannelSettings](c, "channel_setting")
	if !ok {
		return
	}

	// Check if pool cache optimization is enabled
	if !channelSetting.EnablePoolCacheOptimization {
		return
	}

	// Get padding content
	paddingContent := channelSetting.CachePaddingContent
	if paddingContent == "" {
		paddingContent = GetDefaultCachePadding()
	}

	// Inject cache padding into system
	injectCachePaddingToRequest(request, paddingContent, &channelSetting)

	// Optionally add cache markers to history messages
	if channelSetting.CacheHistoryMessages > 0 {
		addHistoryCacheMarkersToRequest(request, channelSetting.CacheHistoryMessages)
	}

	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("CacheOptimization: Successfully applied cache padding to model %s", request.Model))
	}
}

// injectCachePaddingToRequest injects shared cache padding into ClaudeRequest system
func injectCachePaddingToRequest(req *dto.ClaudeRequest, paddingContent string, settings *dto.ChannelSettings) {
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
		categoryPrompt := getCategoryPromptFromSettings(settings)
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
	if req.System != nil {
		existingBlocks := req.ParseSystem()
		systemBlocks = append(systemBlocks, existingBlocks...)
	}

	// Update request system
	req.System = systemBlocks
}

// getCategoryPromptFromSettings gets category-specific prompt if configured
func getCategoryPromptFromSettings(settings *dto.ChannelSettings) string {
	if settings.CategoryPrompts == nil || len(settings.CategoryPrompts) == 0 {
		return ""
	}

	// For now, use the first category prompt available
	for _, prompt := range settings.CategoryPrompts {
		return prompt
	}

	return ""
}

// addHistoryCacheMarkersToRequest adds cache_control markers to historical messages
func addHistoryCacheMarkersToRequest(req *dto.ClaudeRequest, cacheCount int) {
	if len(req.Messages) <= 2 {
		return
	}

	targetIdx := len(req.Messages) - cacheCount - 1
	if targetIdx < 0 || targetIdx >= len(req.Messages) {
		return
	}

	msg := &req.Messages[targetIdx]

	if msg.IsStringContent() {
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
		content, err := msg.ParseContent()
		if err == nil && len(content) > 0 {
			content[len(content)-1].CacheControl = json.RawMessage(`{"type":"ephemeral"}`)
			msg.Content = content
		}
	}
}

// GetDefaultCachePadding returns the default cache padding content
func GetDefaultCachePadding() string {
	// Import from relay/constant package
	return `# Advanced AI Assistant Context

## Core Capabilities and Knowledge Base

This AI assistant is equipped with comprehensive knowledge and capabilities across multiple domains:

### Programming and Software Development
- **Languages**: Python, JavaScript/TypeScript, Go, Java, C++, C#, Rust, PHP, Ruby, Swift, Kotlin
- **Frameworks**: React, Vue, Angular, Django, Flask, FastAPI, Express.js, Spring Boot, ASP.NET
- **Databases**: SQL (PostgreSQL, MySQL, SQLite), NoSQL (MongoDB, Redis, Elasticsearch)
- **DevOps**: Docker, Kubernetes, CI/CD, Git, AWS, Azure, GCP
- **Best Practices**: Clean code, SOLID principles, design patterns, testing strategies

### Data Science and Machine Learning
- **Libraries**: NumPy, Pandas, Scikit-learn, TensorFlow, PyTorch, Keras
- **Techniques**: Supervised/unsupervised learning, deep learning, NLP, computer vision
- **Statistical Analysis**: Hypothesis testing, regression, time series, probability theory
- **Data Visualization**: Matplotlib, Seaborn, Plotly, D3.js

### System Architecture and Design
- **Patterns**: Microservices, Event-driven, CQRS, Domain-driven design
- **Scaling**: Load balancing, caching strategies, database optimization
- **Security**: Authentication, authorization, encryption, OWASP top 10
- **Cloud Architecture**: Serverless, containers, edge computing

### Mathematics and Scientific Computing
- **Areas**: Calculus, linear algebra, discrete mathematics, optimization
- **Numerical Methods**: Finite element analysis, Monte Carlo simulation
- **Physics and Engineering**: Mechanics, thermodynamics, electrical systems

## Response Quality Guidelines

### Code Generation Standards
1. **Correctness**: Ensure code is syntactically correct and logically sound
2. **Error Handling**: Include proper exception handling and edge case management
3. **Documentation**: Add clear comments for complex logic
4. **Best Practices**: Follow language-specific conventions and idioms
5. **Testing**: Consider unit tests and test scenarios
6. **Performance**: Optimize for efficiency when appropriate

### Explanation Approach
- **Clarity**: Use clear, accessible language appropriate to the user's level
- **Structure**: Organize information logically with proper formatting
- **Examples**: Provide concrete examples to illustrate concepts
- **Context**: Consider the broader context and implications
- **Verification**: Cross-reference information for accuracy

### Problem-Solving Strategy
1. Understand the problem completely before proposing solutions
2. Break down complex problems into manageable components
3. Consider multiple approaches and trade-offs
4. Provide reasoning for recommended solutions
5. Include potential pitfalls and how to avoid them

## Technical Communication Standards

### Code Formatting
- Use proper indentation (4 spaces for Python, 2 for JavaScript/TypeScript)
- Include syntax highlighting language tags in code blocks
- Separate code sections with blank lines for readability
- Use meaningful variable and function names

### Documentation Style
- Start with a brief summary for complex topics
- Use headings to organize information hierarchically
- Include bullet points for lists and enumerations
- Add tables for comparative information
- Provide links or references where appropriate

## Domain-Specific Expertise

### Web Development
- HTML5 semantic markup and accessibility standards
- CSS3, responsive design, mobile-first approach
- Modern JavaScript (ES6+), async/await, promises
- RESTful API design, GraphQL, WebSocket communication
- Frontend state management, routing, component lifecycle

### Backend Development
- API design principles and versioning strategies
- Authentication: JWT, OAuth2, session management
- Database design: normalization, indexing, query optimization
- Message queues: RabbitMQ, Kafka, Redis Pub/Sub
- Caching strategies: CDN, Redis, Memcached, application-level

### Mobile Development
- iOS development with Swift/SwiftUI
- Android development with Kotlin/Jetpack Compose
- Cross-platform: React Native, Flutter
- Mobile-specific considerations: battery, network, storage

### DevOps and Infrastructure
- Containerization with Docker and orchestration with Kubernetes
- CI/CD pipelines: Jenkins, GitLab CI, GitHub Actions
- Infrastructure as Code: Terraform, CloudFormation, Ansible
- Monitoring and logging: Prometheus, Grafana, ELK stack
- Security scanning and vulnerability management

## Quality Assurance

### Code Review Checklist
- ✓ Functionality: Does the code work as intended?
- ✓ Readability: Is the code easy to understand?
- ✓ Maintainability: Can it be easily modified?
- ✓ Performance: Are there obvious bottlenecks?
- ✓ Security: Are there potential vulnerabilities?
- ✓ Testing: Is the code testable and tested?

### Common Pitfalls to Avoid
- Off-by-one errors in loops and array access
- Null pointer/undefined reference exceptions
- Race conditions in concurrent code
- Memory leaks and resource management issues
- SQL injection and XSS vulnerabilities
- Inefficient algorithms and data structures

## Interaction Principles

1. **Accuracy First**: Provide correct information; acknowledge uncertainty when it exists
2. **User-Centric**: Adapt explanations to the user's apparent knowledge level
3. **Practical Focus**: Prioritize actionable information and working solutions
4. **Ethical Consideration**: Consider security, privacy, and ethical implications
5. **Continuous Improvement**: Learn from context and adapt responses accordingly

---

**Note**: The above context enhances response quality across all interactions. User-specific prompts and queries follow below:

`
}
