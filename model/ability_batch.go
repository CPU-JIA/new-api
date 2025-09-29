package model

import (
	"fmt"
	"one-api/common"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AbilityBatchOperation represents a batch operation for abilities
type AbilityBatchOperation struct {
	ChannelIDs []int
	Operation  string // "INSERT", "UPDATE", "DELETE", "RECREATE"
	Abilities  []Ability
	TxOptions  *TxOptions
}

// TxOptions provides transaction configuration for batch operations
type TxOptions struct {
	BatchSize      int
	MaxRetries     int
	RetryDelay     time.Duration
	EnableMetrics  bool
	MetricsLogger  func(string, time.Duration, int, error)
}

// DefaultTxOptions provides sensible defaults for batch operations
func DefaultTxOptions() *TxOptions {
	return &TxOptions{
		BatchSize:      100,
		MaxRetries:     3,
		RetryDelay:     100 * time.Millisecond,
		EnableMetrics:  true,
		MetricsLogger:  defaultMetricsLogger,
	}
}

func defaultMetricsLogger(operation string, duration time.Duration, count int, err error) {
	if err != nil {
		common.SysLog(fmt.Sprintf("Batch operation %s failed: %v (duration: %.2fms, count: %d)",
			operation, err, float64(duration.Nanoseconds())/1000000.0, count))
	} else if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Batch operation %s completed: %.2fms (count: %d)",
			operation, float64(duration.Nanoseconds())/1000000.0, count))
	}
}

// UpdateAbilitiesBatch optimizes ability updates for multiple channels
func UpdateAbilitiesBatch(channels []*Channel, tx *gorm.DB, options *TxOptions) error {
	if len(channels) == 0 {
		return nil
	}

	if options == nil {
		options = DefaultTxOptions()
	}

	start := time.Now()
	defer func() {
		if options.EnableMetrics && options.MetricsLogger != nil {
			options.MetricsLogger("UpdateAbilitiesBatch", time.Since(start), len(channels), nil)
		}
	}()

	// Determine if we need to create a new transaction
	isNewTx := (tx == nil)
	if isNewTx {
		tx = DB.Begin()
		if tx.Error != nil {
			return tx.Error
		}
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
				panic(r)
			}
		}()
	}

	// Process channels in batches
	for _, chunk := range lo.Chunk(channels, options.BatchSize) {
		err := updateAbilitiesBatchChunk(chunk, tx, options)
		if err != nil {
			if isNewTx {
				tx.Rollback()
			}
			return fmt.Errorf("batch update failed: %w", err)
		}
	}

	// Commit transaction if we created it
	if isNewTx {
		return tx.Commit().Error
	}

	return nil
}

// updateAbilitiesBatchChunk processes a single chunk of channels
func updateAbilitiesBatchChunk(channels []*Channel, tx *gorm.DB, options *TxOptions) error {
	if len(channels) == 0 {
		return nil
	}

	// Collect all channel IDs for bulk deletion
	channelIDs := make([]int, len(channels))
	for i, channel := range channels {
		channelIDs[i] = channel.Id
	}

	// Step 1: Bulk delete existing abilities for all channels in chunk
	err := tx.Where("channel_id IN ?", channelIDs).Delete(&Ability{}).Error
	if err != nil {
		return fmt.Errorf("bulk delete abilities failed: %w", err)
	}

	// Step 2: Prepare new abilities for bulk insert
	var allAbilities []Ability
	abilitySet := make(map[string]struct{}) // Deduplicate across all channels

	for _, channel := range channels {
		models := strings.Split(channel.Models, ",")
		groups := strings.Split(channel.Group, ",")

		for _, model := range models {
			for _, group := range groups {
				model = strings.TrimSpace(model)
				group = strings.TrimSpace(group)

				if model == "" || group == "" {
					continue
				}

				key := fmt.Sprintf("%d|%s|%s", channel.Id, group, model)
				if _, exists := abilitySet[key]; exists {
					continue
				}
				abilitySet[key] = struct{}{}

				ability := Ability{
					Group:     group,
					Model:     model,
					ChannelId: channel.Id,
					Enabled:   channel.Status == common.ChannelStatusEnabled,
					Priority:  channel.Priority,
					Weight:    uint(channel.GetWeight()),
					Tag:       channel.Tag,
				}
				allAbilities = append(allAbilities, ability)
			}
		}
	}

	// Step 3: Bulk insert new abilities if any exist
	if len(allAbilities) > 0 {
		return bulkInsertAbilities(allAbilities, tx, options)
	}

	return nil
}

// bulkInsertAbilities performs optimized bulk insertion of abilities
func bulkInsertAbilities(abilities []Ability, tx *gorm.DB, options *TxOptions) error {
	if len(abilities) == 0 {
		return nil
	}

	// Use database-specific optimizations
	if common.UsingMySQL {
		return bulkInsertAbilitiesMySQL(abilities, tx, options)
	} else if common.UsingPostgreSQL {
		return bulkInsertAbilitiesPostgreSQL(abilities, tx, options)
	} else {
		return bulkInsertAbilitiesSQLite(abilities, tx, options)
	}
}

// bulkInsertAbilitiesMySQL uses MySQL-specific optimizations
func bulkInsertAbilitiesMySQL(abilities []Ability, tx *gorm.DB, options *TxOptions) error {
	// Use INSERT ... ON DUPLICATE KEY UPDATE for MySQL
	const mysqlBatchSize = 200 // MySQL can handle larger batches efficiently

	for _, chunk := range lo.Chunk(abilities, mysqlBatchSize) {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "group"}, {Name: "model"}, {Name: "channel_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "priority", "weight", "tag"}),
		}).Create(&chunk).Error

		if err != nil {
			return fmt.Errorf("MySQL bulk insert failed: %w", err)
		}
	}

	return nil
}

// bulkInsertAbilitiesPostgreSQL uses PostgreSQL-specific optimizations
func bulkInsertAbilitiesPostgreSQL(abilities []Ability, tx *gorm.DB, options *TxOptions) error {
	// Use PostgreSQL UPSERT (INSERT ... ON CONFLICT)
	const postgresBatchSize = 150

	for _, chunk := range lo.Chunk(abilities, postgresBatchSize) {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "group"}, {Name: "model"}, {Name: "channel_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"enabled", "priority", "weight", "tag"}),
		}).Create(&chunk).Error

		if err != nil {
			return fmt.Errorf("PostgreSQL bulk insert failed: %w", err)
		}
	}

	return nil
}

// bulkInsertAbilitiesSQLite uses SQLite-specific optimizations
func bulkInsertAbilitiesSQLite(abilities []Ability, tx *gorm.DB, options *TxOptions) error {
	// SQLite has smaller batch size limits
	const sqliteBatchSize = 50

	for _, chunk := range lo.Chunk(abilities, sqliteBatchSize) {
		err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&chunk).Error
		if err != nil {
			return fmt.Errorf("SQLite bulk insert failed: %w", err)
		}
	}

	return nil
}

// FixAbilityBatch is the optimized version of FixAbility with batch processing
func FixAbilityBatch(options *TxOptions) (int, int, error) {
	// Use a global lock to prevent concurrent fix operations
	if !fixLock.TryLock() {
		return 0, 0, fmt.Errorf("another fix operation is already running")
	}
	defer fixLock.Unlock()

	if options == nil {
		options = DefaultTxOptions()
	}

	start := time.Now()
	defer func() {
		if options.EnableMetrics && options.MetricsLogger != nil {
			options.MetricsLogger("FixAbilityBatch", time.Since(start), 0, nil)
		}
	}()

	common.SysLog("Starting optimized ability batch fix...")

	// Step 1: Truncate abilities table (more efficient than DELETE)
	err := truncateAbilitiesTable()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to truncate abilities table: %w", err)
	}

	// Step 2: Get all channels in batches
	var totalChannels int64
	err = DB.Model(&Channel{}).Count(&totalChannels).Error
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count channels: %w", err)
	}

	if totalChannels == 0 {
		common.SysLog("No channels found, ability fix completed")
		return 0, 0, nil
	}

	// Step 3: Process channels in optimized batches
	successCount := 0
	failCount := 0
	const channelBatchSize = 100

	offset := 0
	for offset < int(totalChannels) {
		var channels []*Channel
		err = DB.Limit(channelBatchSize).Offset(offset).Find(&channels).Error
		if err != nil {
			common.SysLog(fmt.Sprintf("Failed to fetch channels at offset %d: %v", offset, err))
			failCount += channelBatchSize
			offset += channelBatchSize
			continue
		}

		if len(channels) == 0 {
			break
		}

		// Use batch processing for this chunk
		batchErr := UpdateAbilitiesBatch(channels, nil, options)
		if batchErr != nil {
			common.SysLog(fmt.Sprintf("Batch update failed for channels %d-%d: %v",
				offset, offset+len(channels), batchErr))
			failCount += len(channels)
		} else {
			successCount += len(channels)
		}

		offset += len(channels)

		// Progress logging
		if offset%500 == 0 || offset >= int(totalChannels) {
			common.SysLog(fmt.Sprintf("Processed %d/%d channels (%.1f%% complete)",
				offset, totalChannels, float64(offset)/float64(totalChannels)*100))
		}
	}

	// Step 4: Rebuild cache after batch operations
	if common.MemoryCacheEnabled {
		common.SysLog("Rebuilding channel cache after batch fix...")
		InitChannelCache()
	}

	common.SysLog(fmt.Sprintf("Optimized ability batch fix completed: %d success, %d failed, %.2fs total",
		successCount, failCount, time.Since(start).Seconds()))

	return successCount, failCount, nil
}

// truncateAbilitiesTable efficiently clears the abilities table
func truncateAbilitiesTable() error {
	if common.UsingSQLite {
		// SQLite doesn't support TRUNCATE, use DELETE
		return DB.Exec("DELETE FROM abilities").Error
	} else {
		// MySQL and PostgreSQL support TRUNCATE (faster than DELETE)
		return DB.Exec("TRUNCATE TABLE abilities").Error
	}
}

// BatchSetChannelTagOptimized optimizes the tag update operation
func BatchSetChannelTagOptimized(ids []int, tag *string, options *TxOptions) error {
	if len(ids) == 0 {
		return nil
	}

	if options == nil {
		options = DefaultTxOptions()
	}

	start := time.Now()
	defer func() {
		if options.EnableMetrics && options.MetricsLogger != nil {
			options.MetricsLogger("BatchSetChannelTagOptimized", time.Since(start), len(ids), nil)
		}
	}()

	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Step 1: Update channel tags in bulk
	err := tx.Model(&Channel{}).Where("id IN ?", ids).Update("tag", tag).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update channel tags: %w", err)
	}

	// Step 2: Get updated channels for ability update
	var channels []*Channel
	err = tx.Where("id IN ?", ids).Find(&channels).Error
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to fetch updated channels: %w", err)
	}

	// Step 3: Update abilities in batch
	err = UpdateAbilitiesBatch(channels, tx, options)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update abilities in batch: %w", err)
	}

	// Commit transaction
	return tx.Commit().Error
}

// AbilityBatchMetrics provides monitoring for batch operations
type AbilityBatchMetrics struct {
	TotalOperations     int64         `json:"total_operations"`
	SuccessfulBatches   int64         `json:"successful_batches"`
	FailedBatches       int64         `json:"failed_batches"`
	AverageLatency      time.Duration `json:"average_latency_ms"`
	TotalProcessedItems int64         `json:"total_processed_items"`
	LastOperationTime   time.Time     `json:"last_operation_time"`
	mutex               sync.RWMutex
}

var globalBatchMetrics = &AbilityBatchMetrics{}

// RecordBatchOperation records metrics for a batch operation
func (m *AbilityBatchMetrics) RecordBatchOperation(duration time.Duration, itemCount int, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.TotalOperations++
	m.TotalProcessedItems += int64(itemCount)
	m.LastOperationTime = time.Now()

	if success {
		m.SuccessfulBatches++
	} else {
		m.FailedBatches++
	}

	// Update rolling average (simple approach)
	if m.TotalOperations == 1 {
		m.AverageLatency = duration
	} else {
		m.AverageLatency = time.Duration(
			(int64(m.AverageLatency)*int64(m.TotalOperations-1) + int64(duration)) / int64(m.TotalOperations),
		)
	}
}

// GetBatchMetrics returns current batch operation metrics
func GetBatchMetrics() AbilityBatchMetrics {
	globalBatchMetrics.mutex.RLock()
	defer globalBatchMetrics.mutex.RUnlock()

	// Return a copy without the mutex to avoid copying lock values
	return AbilityBatchMetrics{
		TotalOperations:     globalBatchMetrics.TotalOperations,
		SuccessfulBatches:   globalBatchMetrics.SuccessfulBatches,
		FailedBatches:       globalBatchMetrics.FailedBatches,
		AverageLatency:      globalBatchMetrics.AverageLatency,
		TotalProcessedItems: globalBatchMetrics.TotalProcessedItems,
		LastOperationTime:   globalBatchMetrics.LastOperationTime,
	}
}

// ResetBatchMetrics clears all batch operation metrics
func ResetBatchMetrics() {
	globalBatchMetrics.mutex.Lock()
	defer globalBatchMetrics.mutex.Unlock()

	globalBatchMetrics.TotalOperations = 0
	globalBatchMetrics.SuccessfulBatches = 0
	globalBatchMetrics.FailedBatches = 0
	globalBatchMetrics.AverageLatency = 0
	globalBatchMetrics.TotalProcessedItems = 0
	globalBatchMetrics.LastOperationTime = time.Time{}
}