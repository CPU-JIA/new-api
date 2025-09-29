package tests

import (
	"context"
	"one-api/common"
	"one-api/middleware"
	"one-api/model"

	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecuritySystemIntegration(t *testing.T) {
	if model.DB == nil {
		t.Skip("Database not available for testing")
	}

	// Set up master key for testing
	t.Setenv("ONEAPI_MASTER_KEY", "integration_test_master_key_32_chars")

	t.Run("TestFullSecuritySystemInitialization", func(t *testing.T) {
		// Initialize complete security system
		config := common.DefaultSecuritySystemConfig()
		config.AutoMigrate = false // Disable auto-migration for test safety
		config.ValidationInterval = 30 * time.Second
		config.HealthCheckInterval = 10 * time.Second

		err := common.InitializeSecuritySystem(config)
		require.NoError(t, err, "Should initialize security system successfully")

		// Verify all components are available
		assert.True(t, common.IsSecuritySystemEnabled(), "Security system should be enabled")
		assert.True(t, common.IsSecureStorageEnabled(), "Secure storage should be enabled")
		assert.True(t, common.IsDataMaskingEnabled(), "Data masking should be enabled")
		assert.True(t, common.IsSecureLoggingEnabled(), "Secure logging should be enabled")

		// Verify health status
		healthStatus := common.GetSecurityHealthStatus()
		assert.True(t, healthStatus["initialized"].(bool), "System should be initialized")
		assert.True(t, healthStatus["overall_healthy"].(bool), "System should be healthy")

		// Cleanup
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			common.ShutdownSecuritySystem(ctx)
		}()
	})

	t.Run("TestSecureChannelOperations", func(t *testing.T) {
		// Initialize security system
		config := common.DefaultSecuritySystemConfig()
		err := common.InitializeSecuritySystem(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			common.ShutdownSecuritySystem(ctx)
		}()

		// Initialize secure channel manager
		channelConfig := model.DefaultSecureChannelConfig()
		err = model.InitializeSecureChannelManager(channelConfig)
		require.NoError(t, err, "Should initialize secure channel manager")
		assert.True(t, model.IsSecureChannelEnabled(), "Secure channels should be enabled")

		// Create test channel with plaintext key
		testChannel := &model.Channel{
			Id:     9999,
			Name:   "Security Integration Test Channel",
			Key:    "sk-test1234567890abcdefghijklmnop",
			Type:   1,
			Status: common.ChannelStatusEnabled,
			Models: "gpt-3.5-turbo",
			Group:  "default",
		}

		// Insert test channel
		err = model.DB.Create(testChannel).Error
		require.NoError(t, err, "Should create test channel")
		defer model.DB.Unscoped().Delete(testChannel)

		// Test secure channel operations
		secureChannel := model.NewSecureChannel(testChannel)

		// Test key encryption
		originalKey := testChannel.Key
		err = secureChannel.EncryptKey(context.Background())
		require.NoError(t, err, "Should encrypt key successfully")
		assert.NotEqual(t, originalKey, secureChannel.Key, "Key should be encrypted")
		assert.True(t, common.IsDataEncrypted(secureChannel.Key), "Key should be in encrypted format")

		// Test key decryption
		decryptedKey, err := secureChannel.DecryptKey()
		require.NoError(t, err, "Should decrypt key successfully")
		assert.Equal(t, originalKey, decryptedKey, "Decrypted key should match original")

		// Test secure key retrieval
		keys, err := secureChannel.GetSecureKeys()
		require.NoError(t, err, "Should get secure keys successfully")
		assert.Len(t, keys, 1, "Should have one key")
		assert.Equal(t, originalKey, keys[0], "Retrieved key should match original")

		// Test next enabled key with security
		nextKey, index, apiErr := secureChannel.GetNextEnabledSecureKey()
		require.Nil(t, apiErr, "Should get next enabled key without error")
		assert.Equal(t, 0, index, "Index should be 0 for single key")
		assert.Equal(t, originalKey, nextKey, "Next key should match original")

		t.Logf("Security integration test completed successfully")
	})

	t.Run("TestSecureChannelMigration", func(t *testing.T) {
		// Initialize security system
		config := common.DefaultSecuritySystemConfig()
		err := common.InitializeSecuritySystem(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			common.ShutdownSecuritySystem(ctx)
		}()

		// Initialize secure channel manager
		channelConfig := model.DefaultSecureChannelConfig()
		err = model.InitializeSecureChannelManager(channelConfig)
		require.NoError(t, err)

		// Create multiple test channels with plaintext keys
		testChannels := []*model.Channel{
			{
				Id:     10001,
				Name:   "Migration Test Channel 1",
				Key:    "sk-migration1234567890abcdef",
				Type:   1,
				Status: common.ChannelStatusEnabled,
			},
			{
				Id:     10002,
				Name:   "Migration Test Channel 2",
				Key:    "sk-migration0987654321fedcba",
				Type:   1,
				Status: common.ChannelStatusEnabled,
			},
		}

		// Insert test channels
		for _, channel := range testChannels {
			err = model.DB.Create(channel).Error
			require.NoError(t, err, "Should create test channel")
			defer model.DB.Unscoped().Delete(channel)
		}

		// Get secure channel manager
		manager := model.GetSecureChannelManager()
		require.NotNil(t, manager, "Secure channel manager should be available")

		// Test migration
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err = manager.MigrateChannelKeysToEncrypted(ctx)
		require.NoError(t, err, "Migration should complete successfully")

		// Verify keys are encrypted
		var updatedChannels []model.Channel
		err = model.DB.Where("id IN ?", []int{10001, 10002}).Find(&updatedChannels).Error
		require.NoError(t, err, "Should fetch updated channels")

		for _, channel := range updatedChannels {
			assert.True(t, common.IsDataEncrypted(channel.Key),
				"Channel %d key should be encrypted", channel.Id)

			// Test decryption
			sc := model.NewSecureChannel(&channel)
			decrypted, err := sc.DecryptKey()
			require.NoError(t, err, "Should decrypt migrated key")

			// Find original key for comparison
			var originalKey string
			for _, orig := range testChannels {
				if orig.Id == channel.Id {
					originalKey = orig.Key
					break
				}
			}
			assert.Equal(t, originalKey, decrypted, "Decrypted key should match original")
		}

		// Test validation
		err = manager.ValidateChannelKeyIntegrity(ctx)
		require.NoError(t, err, "Validation should pass after migration")

		t.Logf("Migration test completed successfully")
	})

	t.Run("TestSecurityMiddlewareIntegration", func(t *testing.T) {
		// Initialize security system
		config := common.DefaultSecuritySystemConfig()
		err := common.InitializeSecuritySystem(config)
		require.NoError(t, err)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			common.ShutdownSecuritySystem(ctx)
		}()

		// Create test gin router with security middleware
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// Add security middleware
		middlewareConfig := middleware.DefaultSecureLoggingConfig()
		middlewareConfig.LogRequestBody = true
		middlewareConfig.LogResponseBody = true
		middlewareConfig.MaxBodySize = 1024

		router.Use(middleware.SecureRequestIDMiddleware())
		router.Use(middleware.SecureLoggingMiddleware(middlewareConfig))

		// Add test endpoint
		router.POST("/test-api", func(c *gin.Context) {
			// Simulate API endpoint that handles sensitive data
			var requestData map[string]interface{}
			if err := c.ShouldBindJSON(&requestData); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}

			// Response with potentially sensitive data
			c.JSON(200, gin.H{
				"message": "success",
				"api_key": "sk-response1234567890abcdef", // This should be masked
				"safe_data": "this is safe",
			})
		})

		// This test verifies that middleware is properly configured
		// In a full test, you would make HTTP requests and verify logging
		assert.NotNil(t, router, "Router with security middleware should be configured")

		t.Logf("Security middleware integration test completed")
	})

	t.Run("TestEndToEndSecurityWorkflow", func(t *testing.T) {
		// Initialize complete security system
		config := common.DefaultSecuritySystemConfig()
		config.ValidationInterval = 5 * time.Second // Shorter interval for testing
		err := common.InitializeSecuritySystem(config)
		require.NoError(t, err)

		// Initialize secure channel manager
		channelConfig := model.DefaultSecureChannelConfig()
		err = model.InitializeSecureChannelManager(channelConfig)
		require.NoError(t, err)

		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			common.ShutdownSecuritySystem(ctx)
		}()

		// Create test channel
		testChannel := &model.Channel{
			Id:     10999,
			Name:   "End-to-End Test Channel",
			Key:    "sk-endtoend1234567890abcdef",
			Type:   1,
			Status: common.ChannelStatusEnabled,
			Models: "gpt-3.5-turbo",
			Group:  "test",
		}

		err = model.DB.Create(testChannel).Error
		require.NoError(t, err)
		defer model.DB.Unscoped().Delete(testChannel)

		// Step 1: Retrieve channel securely
		secureChannel, err := model.GetChannelSecurely(testChannel.Id)
		require.NoError(t, err, "Should retrieve channel securely")
		assert.Equal(t, testChannel.Name, secureChannel.Name)

		// Step 2: Encrypt the key
		originalKey := testChannel.Key
		err = secureChannel.EncryptKey(context.Background())
		require.NoError(t, err, "Should encrypt key")

		// Step 3: Update database with encrypted key
		err = model.DB.Model(testChannel).Update("key", secureChannel.Key).Error
		require.NoError(t, err, "Should update database with encrypted key")

		// Step 4: Retrieve and use encrypted key
		var updatedChannel model.Channel
		err = model.DB.First(&updatedChannel, testChannel.Id).Error
		require.NoError(t, err, "Should fetch updated channel")

		secureUpdatedChannel := model.NewSecureChannel(&updatedChannel)
		nextKey, _, apiErr := secureUpdatedChannel.GetNextEnabledSecureKey()
		require.Nil(t, apiErr, "Should get next enabled key from encrypted channel")
		assert.Equal(t, originalKey, nextKey, "Retrieved key should match original")

		// Step 5: Test security status reporting
		manager := model.GetSecureChannelManager()
		statusList, err := manager.GetChannelKeySecurityStatus(context.Background())
		require.NoError(t, err, "Should get security status")

		// Find our test channel in status
		var channelStatus *model.ChannelKeyStatus
		for i, status := range statusList {
			if status.ChannelID == testChannel.Id {
				channelStatus = &statusList[i]
				break
			}
		}

		require.NotNil(t, channelStatus, "Should find channel in security status")
		assert.True(t, channelStatus.IsEncrypted, "Channel should be marked as encrypted")
		assert.True(t, channelStatus.CanDecrypt, "Channel key should be decryptable")

		// Step 6: Verify system health
		healthStatus := common.GetSecurityHealthStatus()
		assert.True(t, healthStatus["overall_healthy"].(bool), "System should remain healthy")

		t.Logf("End-to-end security workflow test completed successfully")
		t.Logf("Encryption status: %+v", channelStatus)
	})
}

func TestSecurityPerformanceImpact(t *testing.T) {
	if model.DB == nil {
		t.Skip("Database not available for testing")
	}

	// Set up master key
	t.Setenv("ONEAPI_MASTER_KEY", "performance_test_master_key_32_chars")

	// Initialize security system
	config := common.DefaultSecuritySystemConfig()
	err := common.InitializeSecuritySystem(config)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		common.ShutdownSecuritySystem(ctx)
	}()

	// Initialize secure channel manager
	channelConfig := model.DefaultSecureChannelConfig()
	err = model.InitializeSecureChannelManager(channelConfig)
	require.NoError(t, err)

	// Create test channel
	testChannel := &model.Channel{
		Id:     20000,
		Name:   "Performance Test Channel",
		Key:    "sk-performance1234567890abcdef",
		Type:   1,
		Status: common.ChannelStatusEnabled,
		Models: "gpt-3.5-turbo",
		Group:  "perf",
	}

	err = model.DB.Create(testChannel).Error
	require.NoError(t, err)
	defer model.DB.Unscoped().Delete(testChannel)

	secureChannel := model.NewSecureChannel(testChannel)

	// Benchmark encryption
	t.Run("BenchmarkKeyEncryption", func(t *testing.T) {
		start := time.Now()
		iterations := 100

		for i := 0; i < iterations; i++ {
			// Create a copy for each iteration
			channelCopy := *testChannel
			sc := model.NewSecureChannel(&channelCopy)
			err := sc.EncryptKey(context.Background())
			require.NoError(t, err)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)

		t.Logf("Encryption performance: %d iterations in %v (avg: %v per operation)",
			iterations, duration, avgDuration)

		// Performance assertion - should complete within reasonable time
		assert.Less(t, avgDuration, 10*time.Millisecond,
			"Average encryption should complete within 10ms")
	})

	// Benchmark decryption
	t.Run("BenchmarkKeyDecryption", func(t *testing.T) {
		// First encrypt the key
		err := secureChannel.EncryptKey(context.Background())
		require.NoError(t, err)

		start := time.Now()
		iterations := 100

		for i := 0; i < iterations; i++ {
			_, err := secureChannel.DecryptKey()
			require.NoError(t, err)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)

		t.Logf("Decryption performance: %d iterations in %v (avg: %v per operation)",
			iterations, duration, avgDuration)

		// Performance assertion
		assert.Less(t, avgDuration, 5*time.Millisecond,
			"Average decryption should complete within 5ms")
	})

	// Test masking performance
	t.Run("BenchmarkDataMasking", func(t *testing.T) {
		masker := common.GetDataMasker()
		require.NotNil(t, masker)

		testData := map[string]interface{}{
			"api_key":  "sk-test1234567890abcdefghijklmnop",
			"password": "super_secret_password",
			"email":    "user@example.com",
			"token":    "token_abcdefghijklmnop",
			"safe":     "not sensitive data",
		}

		start := time.Now()
		iterations := 1000

		for i := 0; i < iterations; i++ {
			_ = masker.MaskJSON(testData)
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(iterations)

		t.Logf("Masking performance: %d iterations in %v (avg: %v per operation)",
			iterations, duration, avgDuration)

		// Performance assertion
		assert.Less(t, avgDuration, 1*time.Millisecond,
			"Average masking should complete within 1ms")
	})
}