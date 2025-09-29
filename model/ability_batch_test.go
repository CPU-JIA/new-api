package model

import (
	"one-api/common"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbilityBatchOperations(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	// Clean up test data
	defer func() {
		DB.Unscoped().Where("id > 0").Delete(&Channel{})
		DB.Unscoped().Where("channel_id > 0").Delete(&Ability{})
	}()

	t.Run("TestDefaultTxOptions", func(t *testing.T) {
		options := DefaultTxOptions()
		assert.NotNil(t, options)
		assert.Equal(t, 100, options.BatchSize)
		assert.Equal(t, 3, options.MaxRetries)
		assert.Equal(t, 100*time.Millisecond, options.RetryDelay)
		assert.True(t, options.EnableMetrics)
		assert.NotNil(t, options.MetricsLogger)
	})

	t.Run("TestUpdateAbilitiesBatch", func(t *testing.T) {
		// Create test channels
		testChannels := []*Channel{
			{
				Id:       1001,
				Name:     "Test Channel 1",
				Models:   "gpt-3.5-turbo,gpt-4",
				Group:    "default,premium",
				Status:   common.ChannelStatusEnabled,
				Priority: common.GetPointer[int64](100),
				Tag:      common.GetPointer("test-tag"),
			},
			{
				Id:       1002,
				Name:     "Test Channel 2",
				Models:   "claude-3-haiku",
				Group:    "default",
				Status:   common.ChannelStatusEnabled,
				Priority: common.GetPointer[int64](50),
				Tag:      common.GetPointer("test-tag-2"),
			},
		}

		// Insert test channels
		for _, channel := range testChannels {
			err := DB.Create(channel).Error
			require.NoError(t, err)
		}

		// Test batch update
		options := &TxOptions{
			BatchSize:     10,
			EnableMetrics: false, // Disable metrics for cleaner test output
		}

		err := UpdateAbilitiesBatch(testChannels, nil, options)
		require.NoError(t, err, "Batch update should succeed")

		// Verify abilities were created
		var abilities []Ability
		err = DB.Where("channel_id IN (?, ?)", 1001, 1002).Find(&abilities).Error
		require.NoError(t, err)

		// Channel 1: 2 models × 2 groups = 4 abilities
		// Channel 2: 1 model × 1 group = 1 ability
		// Total expected: 5 abilities
		assert.Len(t, abilities, 5, "Should create expected number of abilities")

		// Verify ability properties
		for _, ability := range abilities {
			assert.True(t, ability.Enabled, "Abilities should be enabled for enabled channels")
			assert.NotNil(t, ability.Priority, "Priority should be set")
			if ability.ChannelId == 1001 {
				assert.Equal(t, int64(100), *ability.Priority)
				assert.Equal(t, "test-tag", *ability.Tag)
			} else if ability.ChannelId == 1002 {
				assert.Equal(t, int64(50), *ability.Priority)
				assert.Equal(t, "test-tag-2", *ability.Tag)
			}
		}
	})

	t.Run("TestUpdateAbilitiesBatchWithTransaction", func(t *testing.T) {
		// Test with provided transaction
		tx := DB.Begin()
		defer tx.Rollback()

		testChannel := &Channel{
			Id:       1003,
			Name:     "Test Channel 3",
			Models:   "gpt-3.5-turbo",
			Group:    "default",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](75),
		}

		err := tx.Create(testChannel).Error
		require.NoError(t, err)

		channels := []*Channel{testChannel}
		err = UpdateAbilitiesBatch(channels, tx, nil)
		require.NoError(t, err)

		// Verify ability exists in transaction
		var count int64
		err = tx.Model(&Ability{}).Where("channel_id = ?", 1003).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Should create ability within transaction")

		// Verify ability doesn't exist outside transaction
		err = DB.Model(&Ability{}).Where("channel_id = ?", 1003).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Should not create ability outside transaction")
	})

	t.Run("TestBulkInsertAbilitiesMySQL", func(t *testing.T) {
		if !common.UsingMySQL {
			t.Skip("MySQL-specific test")
		}

		abilities := []Ability{
			{
				Group:     "test",
				Model:     "test-model-1",
				ChannelId: 2001,
				Enabled:   true,
				Priority:  common.GetPointer[int64](100),
				Weight:    50,
			},
			{
				Group:     "test",
				Model:     "test-model-2",
				ChannelId: 2001,
				Enabled:   true,
				Priority:  common.GetPointer[int64](100),
				Weight:    30,
			},
		}

		tx := DB.Begin()
		defer tx.Rollback()

		err := bulkInsertAbilitiesMySQL(abilities, tx, DefaultTxOptions())
		assert.NoError(t, err, "MySQL bulk insert should succeed")
	})

	t.Run("TestBulkInsertAbilitiesPostgreSQL", func(t *testing.T) {
		if !common.UsingPostgreSQL {
			t.Skip("PostgreSQL-specific test")
		}

		abilities := []Ability{
			{
				Group:     "test",
				Model:     "test-model-1",
				ChannelId: 2002,
				Enabled:   true,
				Priority:  common.GetPointer[int64](100),
				Weight:    50,
			},
		}

		tx := DB.Begin()
		defer tx.Rollback()

		err := bulkInsertAbilitiesPostgreSQL(abilities, tx, DefaultTxOptions())
		assert.NoError(t, err, "PostgreSQL bulk insert should succeed")
	})

	t.Run("TestBulkInsertAbilitiesSQLite", func(t *testing.T) {
		if !common.UsingSQLite {
			t.Skip("SQLite-specific test")
		}

		abilities := []Ability{
			{
				Group:     "test",
				Model:     "test-model-1",
				ChannelId: 2003,
				Enabled:   true,
				Priority:  common.GetPointer[int64](100),
				Weight:    50,
			},
		}

		tx := DB.Begin()
		defer tx.Rollback()

		err := bulkInsertAbilitiesSQLite(abilities, tx, DefaultTxOptions())
		assert.NoError(t, err, "SQLite bulk insert should succeed")
	})
}

func TestFixAbilityBatch(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	// Clean up test data
	defer func() {
		DB.Unscoped().Where("id > 0").Delete(&Channel{})
		DB.Unscoped().Where("channel_id > 0").Delete(&Ability{})
	}()

	t.Run("TestFixAbilityBatchWithNoChannels", func(t *testing.T) {
		// Ensure no channels exist
		DB.Unscoped().Where("id > 0").Delete(&Channel{})
		DB.Unscoped().Where("channel_id > 0").Delete(&Ability{})

		successCount, failCount, err := FixAbilityBatch(nil)
		assert.NoError(t, err)
		assert.Equal(t, 0, successCount)
		assert.Equal(t, 0, failCount)
	})

	t.Run("TestFixAbilityBatchWithChannels", func(t *testing.T) {
		// Create test channels
		testChannels := []*Channel{
			{
				Id:       3001,
				Name:     "Fix Test Channel 1",
				Models:   "gpt-3.5-turbo",
				Group:    "default",
				Status:   common.ChannelStatusEnabled,
				Priority: common.GetPointer[int64](100),
			},
			{
				Id:       3002,
				Name:     "Fix Test Channel 2",
				Models:   "claude-3-haiku",
				Group:    "premium",
				Status:   common.ChannelStatusManuallyDisabled,
				Priority: common.GetPointer[int64](50),
			},
		}

		// Insert test channels
		for _, channel := range testChannels {
			err := DB.Create(channel).Error
			require.NoError(t, err)
		}

		// Add some old abilities that should be cleared
		oldAbilities := []Ability{
			{
				Group:     "old",
				Model:     "old-model",
				ChannelId: 3001,
				Enabled:   false,
			},
		}
		for _, ability := range oldAbilities {
			err := DB.Create(&ability).Error
			require.NoError(t, err)
		}

		// Run fix
		options := &TxOptions{
			EnableMetrics: false, // Disable metrics for cleaner test output
		}

		successCount, failCount, err := FixAbilityBatch(options)
		require.NoError(t, err, "Fix ability batch should succeed")
		assert.Equal(t, 2, successCount, "Should process 2 channels successfully")
		assert.Equal(t, 0, failCount, "Should have no failures")

		// Verify old abilities were cleared
		var oldAbilityCount int64
		err = DB.Model(&Ability{}).Where("model = ?", "old-model").Count(&oldAbilityCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), oldAbilityCount, "Old abilities should be cleared")

		// Verify new abilities were created correctly
		var newAbilities []Ability
		err = DB.Where("channel_id IN (?, ?)", 3001, 3002).Find(&newAbilities).Error
		require.NoError(t, err)

		enabledCount := 0
		disabledCount := 0
		for _, ability := range newAbilities {
			if ability.Enabled {
				enabledCount++
				assert.Equal(t, 3001, ability.ChannelId, "Enabled ability should be for enabled channel")
			} else {
				disabledCount++
				assert.Equal(t, 3002, ability.ChannelId, "Disabled ability should be for disabled channel")
			}
		}

		assert.Equal(t, 1, enabledCount, "Should have 1 enabled ability")
		assert.Equal(t, 1, disabledCount, "Should have 1 disabled ability")
	})

	t.Run("TestFixAbilityBatchConcurrency", func(t *testing.T) {
		// Test that concurrent fix attempts are blocked
		go func() {
			FixAbilityBatch(nil)
		}()

		time.Sleep(10 * time.Millisecond) // Let first goroutine acquire lock

		_, _, err := FixAbilityBatch(nil)
		assert.Error(t, err, "Concurrent fix should be blocked")
		assert.Contains(t, err.Error(), "already running", "Error should indicate concurrent execution")
	})
}

func TestBatchSetChannelTagOptimized(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	// Clean up test data
	defer func() {
		DB.Unscoped().Where("id > 0").Delete(&Channel{})
		DB.Unscoped().Where("channel_id > 0").Delete(&Ability{})
	}()

	t.Run("TestBatchSetChannelTag", func(t *testing.T) {
		// Create test channels
		testChannels := []*Channel{
			{
				Id:       4001,
				Name:     "Tag Test Channel 1",
				Models:   "gpt-3.5-turbo",
				Group:    "default",
				Status:   common.ChannelStatusEnabled,
				Priority: common.GetPointer[int64](100),
				Tag:      common.GetPointer("old-tag"),
			},
			{
				Id:       4002,
				Name:     "Tag Test Channel 2",
				Models:   "claude-3-haiku",
				Group:    "default",
				Status:   common.ChannelStatusEnabled,
				Priority: common.GetPointer[int64](50),
				Tag:      common.GetPointer("old-tag"),
			},
		}

		// Insert test channels
		for _, channel := range testChannels {
			err := DB.Create(channel).Error
			require.NoError(t, err)
		}

		// Create initial abilities
		err := UpdateAbilitiesBatch(testChannels, nil, nil)
		require.NoError(t, err)

		// Update tags
		newTag := "new-batch-tag"
		ids := []int{4001, 4002}
		options := &TxOptions{
			EnableMetrics: false,
		}

		err = BatchSetChannelTagOptimized(ids, &newTag, options)
		require.NoError(t, err, "Batch tag update should succeed")

		// Verify channel tags were updated
		var channels []Channel
		err = DB.Where("id IN (?, ?)", 4001, 4002).Find(&channels).Error
		require.NoError(t, err)

		for _, channel := range channels {
			require.NotNil(t, channel.Tag, "Tag should not be nil")
			assert.Equal(t, newTag, *channel.Tag, "Channel tag should be updated")
		}

		// Verify ability tags were updated
		var abilities []Ability
		err = DB.Where("channel_id IN (?, ?)", 4001, 4002).Find(&abilities).Error
		require.NoError(t, err)

		for _, ability := range abilities {
			require.NotNil(t, ability.Tag, "Ability tag should not be nil")
			assert.Equal(t, newTag, *ability.Tag, "Ability tag should be updated")
		}
	})

	t.Run("TestBatchSetChannelTagEmpty", func(t *testing.T) {
		// Test with empty ID list
		err := BatchSetChannelTagOptimized([]int{}, common.GetPointer("test"), nil)
		assert.NoError(t, err, "Empty ID list should not cause error")
	})
}

func TestAbilityBatchMetrics(t *testing.T) {
	t.Run("TestRecordBatchOperation", func(t *testing.T) {
		// Reset metrics for clean test
		ResetBatchMetrics()

		metrics := GetBatchMetrics()
		assert.Equal(t, int64(0), metrics.TotalOperations)
		assert.Equal(t, int64(0), metrics.SuccessfulBatches)
		assert.Equal(t, int64(0), metrics.FailedBatches)

		// Record successful operation
		globalBatchMetrics.RecordBatchOperation(100*time.Millisecond, 50, true)

		metrics = GetBatchMetrics()
		assert.Equal(t, int64(1), metrics.TotalOperations)
		assert.Equal(t, int64(1), metrics.SuccessfulBatches)
		assert.Equal(t, int64(0), metrics.FailedBatches)
		assert.Equal(t, int64(50), metrics.TotalProcessedItems)
		assert.Equal(t, 100*time.Millisecond, metrics.AverageLatency)

		// Record failed operation
		globalBatchMetrics.RecordBatchOperation(200*time.Millisecond, 25, false)

		metrics = GetBatchMetrics()
		assert.Equal(t, int64(2), metrics.TotalOperations)
		assert.Equal(t, int64(1), metrics.SuccessfulBatches)
		assert.Equal(t, int64(1), metrics.FailedBatches)
		assert.Equal(t, int64(75), metrics.TotalProcessedItems)
		assert.Equal(t, 150*time.Millisecond, metrics.AverageLatency) // (100+200)/2

		// Reset and verify
		ResetBatchMetrics()
		metrics = GetBatchMetrics()
		assert.Equal(t, int64(0), metrics.TotalOperations)
	})
}

func TestTruncateAbilitiesTable(t *testing.T) {
	if DB == nil {
		t.Skip("Database not available for testing")
	}

	t.Run("TestTruncateAbilities", func(t *testing.T) {
		// Create test ability
		testAbility := Ability{
			Group:     "truncate-test",
			Model:     "test-model",
			ChannelId: 5001,
			Enabled:   true,
		}

		err := DB.Create(&testAbility).Error
		require.NoError(t, err)

		// Verify ability exists
		var count int64
		err = DB.Model(&Ability{}).Where("group = ?", "truncate-test").Count(&count).Error
		require.NoError(t, err)
		assert.Greater(t, count, int64(0), "Test ability should exist")

		// Truncate table
		err = truncateAbilitiesTable()
		require.NoError(t, err, "Truncate should succeed")

		// Verify table is empty
		err = DB.Model(&Ability{}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "Abilities table should be empty after truncate")
	})
}

func BenchmarkUpdateAbilitiesBatch(b *testing.B) {
	if DB == nil {
		b.Skip("Database not available for benchmarking")
	}

	// Create test channels
	testChannels := make([]*Channel, 10)
	for i := 0; i < 10; i++ {
		testChannels[i] = &Channel{
			Id:       6000 + i,
			Name:     "Benchmark Channel",
			Models:   "gpt-3.5-turbo,gpt-4",
			Group:    "default,premium",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}
		DB.Create(testChannels[i])
	}

	defer func() {
		for _, channel := range testChannels {
			DB.Unscoped().Delete(channel)
		}
		DB.Where("channel_id >= ?", 6000).Delete(&Ability{})
	}()

	options := &TxOptions{
		BatchSize:     100,
		EnableMetrics: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		UpdateAbilitiesBatch(testChannels, nil, options)
	}
}

func BenchmarkFixAbilityBatch(b *testing.B) {
	if DB == nil {
		b.Skip("Database not available for benchmarking")
	}

	// Create test channels
	testChannels := make([]*Channel, 5)
	for i := 0; i < 5; i++ {
		testChannels[i] = &Channel{
			Id:       7000 + i,
			Name:     "Fix Benchmark Channel",
			Models:   "gpt-3.5-turbo",
			Group:    "default",
			Status:   common.ChannelStatusEnabled,
			Priority: common.GetPointer[int64](100),
		}
		DB.Create(testChannels[i])
	}

	defer func() {
		for _, channel := range testChannels {
			DB.Unscoped().Delete(channel)
		}
		DB.Where("channel_id >= ?", 7000).Delete(&Ability{})
	}()

	options := &TxOptions{
		EnableMetrics: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FixAbilityBatch(options)
	}
}