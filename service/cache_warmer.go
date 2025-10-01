package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"one-api/common"
	"one-api/constant"
	"one-api/dto"
	"one-api/model"
	"sync"
	"time"
)

// CacheWarmerService manages intelligent cache keep-alive for pool scenarios
type CacheWarmerService struct {
	mu                sync.RWMutex
	channelMetrics    map[int]*ChannelCacheMetrics // channelId -> metrics
	ticker            *time.Ticker
	stopCh            chan struct{}
	warmupThreshold   int           // Min requests per 5min to trigger warmup
	warmupInterval    time.Duration // How often to send warmup requests
	checkInterval     time.Duration // How often to check if warmup is needed
	isRunning         bool
}

// ChannelCacheMetrics tracks request metrics for a channel
type ChannelCacheMetrics struct {
	ChannelID          int
	ChannelName        string
	RequestCount5Min   int       // Requests in last 5 minutes
	LastRequest        time.Time // Last user request time
	LastWarmup         time.Time // Last warmup request time
	WindowStart        time.Time // Start of current 5-min window
	WarmupEnabled      bool      // Whether warmup is currently active
	PaddingContent     string    // Channel-specific padding content
	EnablePoolCache    bool      // Whether pool cache is enabled for this channel
	OptimalInterval    time.Duration // Dynamically calculated warmup interval
	RequestRate        float64       // Requests per 5 minutes (rolling average)
	TTL                string        // Cache TTL: "5m" or "1h"

	// ECP-C3: Performance Awareness - ROI monitoring for auto-disable
	WarmupCount       int       // Total number of warmup requests sent
	LastROICheck      time.Time // Last time ROI was evaluated
	ConsecutiveLowROI int       // Counter for consecutive low ROI detections
}

var (
	globalWarmer *CacheWarmerService
	warmerOnce   sync.Once
)

// GetCacheWarmerService returns the global cache warmer instance
func GetCacheWarmerService() *CacheWarmerService {
	warmerOnce.Do(func() {
		globalWarmer = &CacheWarmerService{
			channelMetrics:  make(map[int]*ChannelCacheMetrics),
			warmupThreshold: 10,              // Default: 10 requests per 5min
			warmupInterval:  4 * time.Minute, // Default: every 4 minutes (before 5min TTL)
			checkInterval:   1 * time.Minute, // Check every minute
			stopCh:          make(chan struct{}),
		}
	})
	return globalWarmer
}

// Start starts the cache warmer background service
func (cw *CacheWarmerService) Start() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.isRunning {
		common.SysLog("CacheWarmer: Already running")
		return
	}

	cw.ticker = time.NewTicker(cw.checkInterval)
	cw.isRunning = true

	go cw.run()
	common.SysLog("CacheWarmer: Service started")
}

// Stop stops the cache warmer service
func (cw *CacheWarmerService) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.isRunning {
		return
	}

	close(cw.stopCh)
	cw.ticker.Stop()
	cw.isRunning = false
	common.SysLog("CacheWarmer: Service stopped")
}

// RecordRequest records a user request for metrics tracking
func (cw *CacheWarmerService) RecordRequest(channelID int, channelName string, settings *dto.ChannelSettings) {
	if settings == nil || !settings.EnablePoolCacheOptimization || !settings.EnableSmartWarmup {
		return
	}

	// ECP-B1: DRY - normalize configuration in one place
	settings.NormalizeCacheConfig()

	cw.mu.Lock()
	defer cw.mu.Unlock()

	now := time.Now()
	metrics, exists := cw.channelMetrics[channelID]

	// Get TTL configuration (now guaranteed to have default value)
	cacheTTL := settings.CacheTTL

	if !exists {
		metrics = &ChannelCacheMetrics{
			ChannelID:       channelID,
			ChannelName:     channelName,
			WindowStart:     now,
			LastRequest:     now,
			EnablePoolCache: true,
			PaddingContent:  settings.CachePaddingContent,
			TTL:             cacheTTL,
		}
		cw.channelMetrics[channelID] = metrics
	}

	// Reset window if more than 5 minutes passed
	if now.Sub(metrics.WindowStart) > 5*time.Minute {
		metrics.WindowStart = now
		metrics.RequestCount5Min = 0
	}

	metrics.RequestCount5Min++
	metrics.LastRequest = now
	metrics.TTL = cacheTTL // Update TTL in case it changed

	// Update rolling average request rate
	metrics.RequestRate = float64(metrics.RequestCount5Min)

	// Check if warmup should be enabled (use normalized threshold)
	threshold := settings.WarmupThreshold

	if metrics.RequestCount5Min >= threshold {
		if !metrics.WarmupEnabled {
			metrics.WarmupEnabled = true
			// Calculate initial optimal interval based on TTL and request rate
			metrics.OptimalInterval = cw.calculateWarmupInterval(metrics.RequestRate, metrics.TTL)
			if common.DebugEnabled {
				common.SysLog(fmt.Sprintf("CacheWarmer: Enabled for channel %s (id=%d), requests=%d, interval=%v, TTL=%s",
					channelName, channelID, metrics.RequestCount5Min, metrics.OptimalInterval, metrics.TTL))
			}
		}
	}
}

// calculateWarmupInterval calculates the optimal warmup interval based on request rate and TTL
// ECP-C3: Performance Awareness - optimize warmup frequency based on actual usage and TTL
func (cw *CacheWarmerService) calculateWarmupInterval(requestRate float64, ttl string) time.Duration {
	// For 1-hour TTL, use longer intervals to reduce warmup cost
	if ttl == "1h" {
		// Base interval: 45 minutes (safe margin before 1-hour expiry)
		// Adjust based on request rate for active channels
		switch {
		case requestRate >= 50:
			return 40 * time.Minute // Ultra-high frequency: warmup more often
		case requestRate >= 20:
			return 45 * time.Minute // High frequency: standard interval
		case requestRate >= 10:
			return 50 * time.Minute // Medium frequency: safe margin
		default:
			return 50 * time.Minute // Low frequency: maximum interval
		}
	}

	// For 5-minute TTL, use existing dynamic logic
	// requestRate is requests per 5 minutes
	// We want to warmup before the 5-minute cache expires
	switch {
	case requestRate >= 50:
		return 2 * time.Minute // Ultra-high frequency
	case requestRate >= 20:
		return 3 * time.Minute // High frequency
	case requestRate >= 10:
		return 4 * time.Minute // Medium frequency (default)
	case requestRate >= 5:
		return 270 * time.Second // Low frequency (4.5 min)
	default:
		// Very low frequency: use default but might be disabled anyway
		return cw.warmupInterval
	}
}

// run is the background loop that checks and sends warmup requests
func (cw *CacheWarmerService) run() {
	for {
		select {
		case <-cw.stopCh:
			return
		case <-cw.ticker.C:
			cw.checkAndWarmup()
		}
	}
}

// checkAndWarmup checks all channels and sends warmup requests if needed
// ECP-C1: Defensive Programming - use Lock instead of RLock to prevent race conditions
// This function modifies metrics fields (LastROICheck, ConsecutiveLowROI, WarmupEnabled)
func (cw *CacheWarmerService) checkAndWarmup() {
	cw.mu.Lock()
	channelsToWarmup := make([]*ChannelCacheMetrics, 0)

	now := time.Now()
	for _, metrics := range cw.channelMetrics {
		if metrics.WarmupEnabled {
			// ECP-C3: Performance Awareness - evaluate ROI every 10 minutes
			timeSinceLastROICheck := now.Sub(metrics.LastROICheck)
			if metrics.LastROICheck.IsZero() || timeSinceLastROICheck >= 10*time.Minute {
				metrics.LastROICheck = now
				cw.evaluateChannelPerformance(metrics)
				// Check if evaluation disabled the channel
				if !metrics.WarmupEnabled {
					continue
				}
			}

			// Recalculate optimal interval based on current request rate and TTL
			// ECP-C3: Performance Awareness - dynamic interval adjustment
			if metrics.OptimalInterval == 0 {
				metrics.OptimalInterval = cw.calculateWarmupInterval(metrics.RequestRate, metrics.TTL)
			}

			// Check if it's time to send warmup
			timeSinceLastWarmup := now.Sub(metrics.LastWarmup)
			timeSinceLastRequest := now.Sub(metrics.LastRequest)

			// Use dynamic interval instead of fixed warmupInterval
			warmupInterval := metrics.OptimalInterval
			if warmupInterval == 0 {
				warmupInterval = cw.warmupInterval // fallback
			}

			// Send warmup if:
			// 1. Never sent before OR
			// 2. More than optimal interval since last warmup AND less than TTL expiry since last user request
			maxIdleTime := 5 * time.Minute // Default for 5m TTL
			if metrics.TTL == "1h" {
				maxIdleTime = 65 * time.Minute // Allow slightly more than 1 hour for 1h TTL
			}

			shouldWarmup := metrics.LastWarmup.IsZero() ||
				(timeSinceLastWarmup >= warmupInterval && timeSinceLastRequest < maxIdleTime)

			if shouldWarmup {
				channelsToWarmup = append(channelsToWarmup, metrics)
			}

			// Disable warmup if no requests for extended period
			// For 1h TTL, allow longer idle time before disabling
			maxInactiveTime := 10 * time.Minute
			if metrics.TTL == "1h" {
				maxInactiveTime = 70 * time.Minute
			}

			if timeSinceLastRequest > maxInactiveTime {
				metrics.WarmupEnabled = false
				if common.DebugEnabled {
					common.SysLog(fmt.Sprintf("CacheWarmer: Disabled for channel %s (id=%d), idle=%v",
						metrics.ChannelName, metrics.ChannelID, timeSinceLastRequest))
				}
			}
		}
	}
	cw.mu.Unlock()

	// Send warmup requests outside the lock
	for _, metrics := range channelsToWarmup {
		cw.sendWarmupRequest(metrics)
	}
}

// sendWarmupRequest sends a minimal warmup request to keep cache alive
func (cw *CacheWarmerService) sendWarmupRequest(metrics *ChannelCacheMetrics) {
	cw.mu.Lock()
	metrics.LastWarmup = time.Now()
	cw.mu.Unlock()

	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("CacheWarmer: Sending warmup for channel %s (id=%d)",
			metrics.ChannelName, metrics.ChannelID))
		common.SysLog(fmt.Sprintf("CacheWarmer: Using channel's own API key, cost ~$0.001 per warmup"))
	}

	// Send warmup request asynchronously to avoid blocking
	go func() {
		err := cw.doSendWarmup(metrics)
		if err != nil {
			common.SysError(fmt.Sprintf("CacheWarmer: Warmup failed for channel %s (id=%d): %v",
				metrics.ChannelName, metrics.ChannelID, err))
		} else {
			// ECP-C3: Performance Awareness - track successful warmups for ROI calculation
			cw.mu.Lock()
			metrics.WarmupCount++
			cw.mu.Unlock()

			if common.DebugEnabled {
				common.SysLog(fmt.Sprintf("CacheWarmer: Warmup succeeded for channel %s (id=%d), total_warmups=%d",
					metrics.ChannelName, metrics.ChannelID, metrics.WarmupCount))
			}
		}
	}()
}

// evaluateChannelPerformance evaluates channel ROI and auto-disables if inefficient
// ECP-C3: Performance Awareness - prevent wasteful warmup spending
func (cw *CacheWarmerService) evaluateChannelPerformance(metrics *ChannelCacheMetrics) {
	// Skip evaluation if no warmups sent yet
	if metrics.WarmupCount == 0 {
		return
	}

	// Calculate estimated hourly requests based on current 5-min window
	// RequestCount5Min * 12 = estimated requests per hour
	estimatedHourlyRequests := metrics.RequestCount5Min * 12
	if estimatedHourlyRequests == 0 {
		// No requests in current window, skip evaluation
		return
	}

	// ROI Heuristic: WarmupCount / EstimatedHourlyRequests
	// If ratio > 1.5, we're sending more warmups than requests (negative ROI)
	roiRatio := float64(metrics.WarmupCount) / float64(estimatedHourlyRequests)
	isLowROI := roiRatio > 1.5

	if isLowROI {
		metrics.ConsecutiveLowROI++
		common.SysLog(fmt.Sprintf("CacheWarmer: Low ROI detected for channel %s (id=%d): "+
			"warmups=%d, est_hourly_requests=%d, roi_ratio=%.2f, consecutive_low=%d",
			metrics.ChannelName, metrics.ChannelID, metrics.WarmupCount,
			estimatedHourlyRequests, roiRatio, metrics.ConsecutiveLowROI))

		// Auto-disable after 3 consecutive low ROI detections
		if metrics.ConsecutiveLowROI >= 3 {
			metrics.WarmupEnabled = false
			common.SysLog(fmt.Sprintf("CacheWarmer: AUTO-DISABLED warmup for channel %s (id=%d) "+
				"due to sustained negative ROI (warmups=%d > 1.5x est_requests=%d). "+
				"Cost savings: ~$%.3f/hour. Re-enable manually if traffic increases.",
				metrics.ChannelName, metrics.ChannelID, metrics.WarmupCount,
				estimatedHourlyRequests, float64(metrics.WarmupCount)*0.001))
			// Reset counters for potential future re-evaluation
			metrics.ConsecutiveLowROI = 0
		}
	} else {
		// Good ROI, reset counter
		if metrics.ConsecutiveLowROI > 0 {
			common.SysLog(fmt.Sprintf("CacheWarmer: Good ROI for channel %s (id=%d): "+
				"warmups=%d, est_hourly_requests=%d, roi_ratio=%.2f (reset low_roi counter)",
				metrics.ChannelName, metrics.ChannelID, metrics.WarmupCount,
				estimatedHourlyRequests, roiRatio))
		}
		metrics.ConsecutiveLowROI = 0
	}
}

// doSendWarmup performs the actual warmup HTTP request
// IMPORTANT: Warmup requests are quota-exempt by design:
// - Bypasses all Gin middleware (TokenAuth, Distribute, billing)
// - Uses channel's API key directly, not user tokens
// - Only consumes channel's API quota, not user quota
// - Cost is absorbed by the system to maintain cache benefits
func (cw *CacheWarmerService) doSendWarmup(metrics *ChannelCacheMetrics) error {
	// Get channel details
	channel, err := model.GetChannelById(metrics.ChannelID, true)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	if channel.Status != common.ChannelStatusEnabled {
		return fmt.Errorf("channel is disabled")
	}

	// Get channel settings
	settings := channel.GetSetting()
	if !settings.EnablePoolCacheOptimization {
		return fmt.Errorf("pool cache not enabled")
	}

	// Construct minimal warmup request with only padding content
	paddingContent := metrics.PaddingContent
	if paddingContent == "" {
		paddingContent = GetDefaultWarmupPadding()
	}

	// Use cheapest Claude model that supports caching for warmup
	warmupModel := "claude-3-5-haiku-20241022"

	// CRITICAL: Verify model supports caching before adding cache_control
	if !constant.IsClaudeModelSupportCache(warmupModel) {
		return fmt.Errorf("warmup model %s does not support prompt caching", warmupModel)
	}

	claudeRequest := dto.ClaudeRequest{
		Model:     warmupModel,
		MaxTokens: 1,        // Minimal tokens
		Messages: []dto.ClaudeMessage{
			{
				Role:    "user",
				Content: "warmup", // Minimal message
			},
		},
	}

	// Build system with cache control based on TTL (only if model supports it)
	// TTL is guaranteed to be set by Normalize in RecordRequest
	// ECP-B1: DRY - no need to check for empty string
	cacheTTL := metrics.TTL

	// Generate cache_control JSON based on TTL
	var cacheControlJSON json.RawMessage
	if cacheTTL == "1h" {
		cacheControlJSON = json.RawMessage(`{"type":"ephemeral","ttl":"1h"}`)
	} else {
		cacheControlJSON = json.RawMessage(`{"type":"ephemeral"}`) // Default 5m
	}

	systemBlocks := []dto.ClaudeMediaMessage{
		{
			Type:         "text",
			Text:         common.GetPointer(paddingContent),
			CacheControl: cacheControlJSON,
		},
	}
	claudeRequest.System = systemBlocks

	// Send HTTP request
	err = cw.sendClaudeRequest(channel, &claudeRequest)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	return nil
}

// sendClaudeRequest sends the warmup request to Claude API
func (cw *CacheWarmerService) sendClaudeRequest(channel *model.Channel, request *dto.ClaudeRequest) error {
	// Marshal request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// Get channel API key
	key, _, err := channel.GetNextEnabledKey()
	if err != nil {
		return fmt.Errorf("get key failed: %w", err)
	}

	// Construct HTTP request
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("User-Agent", "New-API-CacheWarmer/1.0")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bad status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetDefaultWarmupPadding returns default padding for warmup requests
func GetDefaultWarmupPadding() string {
	// Use the same padding as cache optimization
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

// GetMetrics returns current metrics for monitoring/debugging
func (cw *CacheWarmerService) GetMetrics() map[int]*ChannelCacheMetrics {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	// Return a copy to avoid race conditions
	metrics := make(map[int]*ChannelCacheMetrics)
	for k, v := range cw.channelMetrics {
		metricsCopy := *v
		metrics[k] = &metricsCopy
	}
	return metrics
}