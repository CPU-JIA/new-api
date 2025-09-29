package model

import (
	"context"
	"fmt"
	"one-api/common"
	"one-api/setting"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// CacheWarmerConfig holds configuration for cache warming
type CacheWarmerConfig struct {
	Workers     int           // Number of concurrent workers
	BatchSize   int           // Items to process per batch
	Timeout     time.Duration // Total warming timeout
	RetryCount  int           // Number of retries on failure
	RetryDelay  time.Duration // Delay between retries
}

// DefaultCacheWarmerConfig returns sensible defaults
func DefaultCacheWarmerConfig() *CacheWarmerConfig {
	return &CacheWarmerConfig{
		Workers:    4,
		BatchSize:  50,
		Timeout:    60 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
	}
}

// CacheWarmer implements intelligent cache preheating strategies
type CacheWarmer struct {
	config     *CacheWarmerConfig
	workersWG  sync.WaitGroup
}

// WarmupTask represents a single cache warming task
type WarmupTask struct {
	Type        string      `json:"type"`          // "channel", "group_model", "abilities"
	Key         string      `json:"key"`           // Identifier for the item
	Data        interface{} `json:"data"`          // The actual data to cache
	Priority    int         `json:"priority"`      // Higher number = higher priority
	Retries     int         `json:"retries"`       // Number of retry attempts
}

// WarmupProgress tracks the progress of cache warming
type WarmupProgress struct {
	Total         int       `json:"total"`
	Completed     int       `json:"completed"`
	Failed        int       `json:"failed"`
	StartTime     time.Time `json:"start_time"`
	EstimatedTime time.Duration `json:"estimated_time_remaining"`
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(config *CacheWarmerConfig) *CacheWarmer {
	if config == nil {
		config = DefaultCacheWarmerConfig()
	}

	return &CacheWarmer{
		config: config,
	}
}

// WarmupAll performs comprehensive cache warming
func (cw *CacheWarmer) WarmupAll(ctx context.Context, manager CacheManager) error {
	start := time.Now()
	common.SysLog("Starting intelligent cache warmup...")

	// Create context with timeout
	warmupCtx, cancel := context.WithTimeout(ctx, cw.config.Timeout)
	defer cancel()

	// Generate warmup tasks with priorities
	tasks, err := cw.generateWarmupTasks()
	if err != nil {
		return fmt.Errorf("failed to generate warmup tasks: %w", err)
	}

	if len(tasks) == 0 {
		common.SysLog("No warmup tasks generated - cache is already optimal or no data available")
		return nil
	}

	// Execute tasks with worker pool
	progress := &WarmupProgress{
		Total:     len(tasks),
		StartTime: start,
	}

	err = cw.executeTasks(warmupCtx, tasks, manager, progress)

	duration := time.Since(start)
	if err != nil {
		common.SysLog(fmt.Sprintf("Cache warmup completed with errors in %.2fs: %d/%d successful",
			duration.Seconds(), progress.Completed, progress.Total))
		return err
	}

	common.SysLog(fmt.Sprintf("Cache warmup completed successfully in %.2fs: %d tasks executed",
		duration.Seconds(), progress.Completed))

	return nil
}

// WarmupChannels performs targeted channel warming
func (cw *CacheWarmer) WarmupChannels(ctx context.Context, manager CacheManager, channelIDs []int) error {
	if len(channelIDs) == 0 {
		return nil
	}

	common.SysLog(fmt.Sprintf("Warming up %d specific channels", len(channelIDs)))

	// Create tasks for specific channels
	tasks := make([]*WarmupTask, 0, len(channelIDs))

	for _, id := range channelIDs {
		tasks = append(tasks, &WarmupTask{
			Type:     "channel",
			Key:      fmt.Sprintf("ch:%d", id),
			Data:     id,
			Priority: 100, // High priority for targeted warming
		})
	}

	progress := &WarmupProgress{
		Total:     len(tasks),
		StartTime: time.Now(),
	}

	return cw.executeTasks(ctx, tasks, manager, progress)
}

// WarmupGroupModels performs targeted group-model mapping warming
func (cw *CacheWarmer) WarmupGroupModels(ctx context.Context, manager CacheManager, groups []string, models []string) error {
	if len(groups) == 0 || len(models) == 0 {
		return nil
	}

	common.SysLog(fmt.Sprintf("Warming up %d groups × %d models", len(groups), len(models)))

	// Create tasks for group-model combinations
	tasks := make([]*WarmupTask, 0, len(groups)*len(models))

	for _, group := range groups {
		for _, model := range models {
			tasks = append(tasks, &WarmupTask{
				Type:     "group_model",
				Key:      fmt.Sprintf("gm:%s:%s", group, model),
				Data:     map[string]string{"group": group, "model": model},
				Priority: 80, // Medium-high priority
			})
		}
	}

	progress := &WarmupProgress{
		Total:     len(tasks),
		StartTime: time.Now(),
	}

	return cw.executeTasks(ctx, tasks, manager, progress)
}

// generateWarmupTasks creates prioritized warmup tasks based on system state
func (cw *CacheWarmer) generateWarmupTasks() ([]*WarmupTask, error) {
	var tasks []*WarmupTask

	// 1. Generate channel warming tasks (highest priority)
	channelTasks, err := cw.generateChannelTasks()
	if err != nil {
		return nil, fmt.Errorf("failed to generate channel tasks: %w", err)
	}
	tasks = append(tasks, channelTasks...)

	// 2. Generate group-model mapping tasks (high priority)
	groupModelTasks, err := cw.generateGroupModelTasks()
	if err != nil {
		return nil, fmt.Errorf("failed to generate group-model tasks: %w", err)
	}
	tasks = append(tasks, groupModelTasks...)

	// 3. Generate ability warming tasks (medium priority)
	abilityTasks, err := cw.generateAbilityTasks()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ability tasks: %w", err)
	}
	tasks = append(tasks, abilityTasks...)

	// Sort tasks by priority (higher first)
	cw.sortTasksByPriority(tasks)

	return tasks, nil
}

// generateChannelTasks creates tasks for warming individual channels
func (cw *CacheWarmer) generateChannelTasks() ([]*WarmupTask, error) {
	var tasks []*WarmupTask

	// Get enabled channels (these are most likely to be accessed)
	var channels []*Channel
	err := DB.Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error
	if err != nil {
		return nil, err
	}

	for _, channel := range channels {
		priority := cw.calculateChannelPriority(channel)

		tasks = append(tasks, &WarmupTask{
			Type:     "channel",
			Key:      fmt.Sprintf("ch:%d", channel.Id),
			Data:     channel.Id,
			Priority: priority,
		})
	}

	return tasks, nil
}

// generateGroupModelTasks creates tasks for warming group-model combinations
func (cw *CacheWarmer) generateGroupModelTasks() ([]*WarmupTask, error) {
	var tasks []*WarmupTask

	// Get distinct group-model combinations from abilities
	var combinations []struct {
		Group string
		Model string
		Count int
	}

	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	err := DB.Table("abilities").
		Select(groupCol + ", model, COUNT(*) as count").
		Where("enabled = ?", true).
		Group(groupCol + ", model").
		Scan(&combinations).Error

	if err != nil {
		return nil, err
	}

	for _, combo := range combinations {
		priority := cw.calculateGroupModelPriority(combo.Group, combo.Model, combo.Count)

		tasks = append(tasks, &WarmupTask{
			Type:     "group_model",
			Key:      fmt.Sprintf("gm:%s:%s", combo.Group, combo.Model),
			Data:     map[string]string{"group": combo.Group, "model": combo.Model},
			Priority: priority,
		})
	}

	return tasks, nil
}

// generateAbilityTasks creates tasks for warming channel abilities
func (cw *CacheWarmer) generateAbilityTasks() ([]*WarmupTask, error) {
	var tasks []*WarmupTask

	// Get channels with their ability counts
	var channelAbilities []struct {
		ChannelID    int
		AbilityCount int
	}

	err := DB.Table("abilities").
		Select("channel_id, COUNT(*) as ability_count").
		Where("enabled = ?", true).
		Group("channel_id").
		Having("COUNT(*) > 0").
		Scan(&channelAbilities).Error

	if err != nil {
		return nil, err
	}

	for _, ca := range channelAbilities {
		priority := cw.calculateAbilityPriority(ca.AbilityCount)

		tasks = append(tasks, &WarmupTask{
			Type:     "abilities",
			Key:      fmt.Sprintf("ab:%d", ca.ChannelID),
			Data:     ca.ChannelID,
			Priority: priority,
		})
	}

	return tasks, nil
}

// executeTasks executes warmup tasks using a worker pool
func (cw *CacheWarmer) executeTasks(ctx context.Context, tasks []*WarmupTask, manager CacheManager, progress *WarmupProgress) error {
	if len(tasks) == 0 {
		return nil
	}

	// Create task channel
	taskChan := make(chan *WarmupTask, len(tasks))

	// Fill task channel
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// Result channel for tracking progress
	resultChan := make(chan error, len(tasks))

	// Start workers
	for i := 0; i < cw.config.Workers; i++ {
		cw.workersWG.Add(1)
		go cw.worker(ctx, taskChan, resultChan, manager)
	}

	// Progress tracking goroutine
	go cw.trackProgress(ctx, progress, resultChan)

	// Wait for all workers to complete
	cw.workersWG.Wait()
	close(resultChan)

	// Check if context was cancelled
	if ctx.Err() != nil {
		return fmt.Errorf("warmup cancelled: %w", ctx.Err())
	}

	return nil
}

// worker processes warmup tasks
func (cw *CacheWarmer) worker(ctx context.Context, taskChan <-chan *WarmupTask, resultChan chan<- error, manager CacheManager) {
	defer cw.workersWG.Done()

	for task := range taskChan {
		select {
		case <-ctx.Done():
			resultChan <- ctx.Err()
			return
		default:
		}

		err := cw.executeTask(ctx, task, manager)
		resultChan <- err

		if err != nil && common.DebugEnabled {
			common.SysLog(fmt.Sprintf("Warmup task failed: type=%s, key=%s, error=%v",
				task.Type, task.Key, err))
		}
	}
}

// executeTask executes a single warmup task
func (cw *CacheWarmer) executeTask(ctx context.Context, task *WarmupTask, manager CacheManager) error {
	var err error

	// Execute task based on type
	switch task.Type {
	case "channel":
		if channelID, ok := task.Data.(int); ok {
			_, err = manager.GetChannel(channelID)
		} else {
			err = fmt.Errorf("invalid channel ID data type")
		}

	case "group_model":
		if data, ok := task.Data.(map[string]string); ok {
			group := data["group"]
			model := data["model"]
			if group != "" && model != "" {
				// Simulate channel selection to warm the cache
				ginCtx := &gin.Context{}
				_, _, err = manager.GetRandomSatisfiedChannel(ginCtx, group, model, 0)
			} else {
				err = fmt.Errorf("invalid group-model data")
			}
		} else {
			err = fmt.Errorf("invalid group-model data type")
		}

	case "abilities":
		if channelID, ok := task.Data.(int); ok {
			// Pre-warm abilities for this channel by getting the channel
			_, err = manager.GetChannel(channelID)
		} else {
			err = fmt.Errorf("invalid channel ID data type for abilities")
		}

	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	// Retry logic
	if err != nil && task.Retries < cw.config.RetryCount {
		task.Retries++
		time.Sleep(cw.config.RetryDelay)
		return cw.executeTask(ctx, task, manager)
	}

	return err
}

// trackProgress monitors warmup progress
func (cw *CacheWarmer) trackProgress(ctx context.Context, progress *WarmupProgress, resultChan <-chan error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case err := <-resultChan:
			if err != nil {
				progress.Failed++
			} else {
				progress.Completed++
			}

			// Update estimated time
			if progress.Completed > 0 {
				elapsed := time.Since(progress.StartTime)
				avgTimePerTask := elapsed / time.Duration(progress.Completed)
				remaining := progress.Total - progress.Completed - progress.Failed
				progress.EstimatedTime = avgTimePerTask * time.Duration(remaining)
			}

			// Check if done
			if progress.Completed+progress.Failed >= progress.Total {
				return
			}

		case <-ticker.C:
			// Periodic progress logging
			if common.DebugEnabled {
				common.SysLog(fmt.Sprintf("Warmup progress: %d/%d completed, %d failed, ~%.1fs remaining",
					progress.Completed, progress.Total, progress.Failed, progress.EstimatedTime.Seconds()))
			}

		case <-ctx.Done():
			return
		}
	}
}

// Priority calculation methods

func (cw *CacheWarmer) calculateChannelPriority(channel *Channel) int {
	priority := 50 // Base priority

	// Higher priority for enabled channels
	if channel.Status == common.ChannelStatusEnabled {
		priority += 30
	}

	// Higher priority based on channel priority setting
	if channel.Priority != nil {
		priority += int(*channel.Priority / 10) // Scale down priority value
	}

	// Higher priority for channels with more models/groups
	modelCount := len(strings.Split(channel.Models, ","))
	groupCount := len(strings.Split(channel.Group, ","))
	priority += (modelCount + groupCount) * 2

	return priority
}

func (cw *CacheWarmer) calculateGroupModelPriority(group, model string, count int) int {
	priority := 60 // Base priority for group-model combinations

	// Higher priority for default group
	if group == "default" {
		priority += 20
	}

	// Higher priority for common models
	commonModels := map[string]int{
		"gpt-3.5-turbo": 15,
		"gpt-4": 10,
		"claude-3-haiku": 8,
		"claude-3-sonnet": 8,
	}
	if bonus, exists := commonModels[model]; exists {
		priority += bonus
	}

	// Higher priority based on channel count
	priority += count * 2

	// Boost priority for auto groups
	if contains(setting.AutoGroups, group) {
		priority += 10
	}

	return priority
}

func (cw *CacheWarmer) calculateAbilityPriority(abilityCount int) int {
	priority := 40 // Base priority for abilities

	// Higher priority for channels with more abilities
	priority += abilityCount

	return priority
}

// Helper methods

func (cw *CacheWarmer) sortTasksByPriority(tasks []*WarmupTask) {
	// Simple bubble sort by priority (descending)
	n := len(tasks)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if tasks[j].Priority < tasks[j+1].Priority {
				tasks[j], tasks[j+1] = tasks[j+1], tasks[j]
			}
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}