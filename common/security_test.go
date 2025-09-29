package common

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecureStorage(t *testing.T) {
	// Set up test environment
	os.Setenv("ONEAPI_MASTER_KEY", "test_master_key_for_testing_12345")
	defer os.Unsetenv("ONEAPI_MASTER_KEY")

	t.Run("TestSecureStorageCreation", func(t *testing.T) {
		config := DefaultSecureStorageConfig()
		storage, err := NewAESSecureStorage(config)
		require.NoError(t, err, "Should create secure storage successfully")
		assert.NotNil(t, storage, "Storage should not be nil")

		// Test integrity validation
		err = storage.ValidateIntegrity()
		assert.NoError(t, err, "Integrity validation should pass")
	})

	t.Run("TestBasicEncryptionDecryption", func(t *testing.T) {
		config := DefaultSecureStorageConfig()
		storage, err := NewAESSecureStorage(config)
		require.NoError(t, err)

		testData := "This is sensitive test data"

		// Test string encryption/decryption
		encrypted, err := storage.EncryptString(testData)
		require.NoError(t, err, "Should encrypt successfully")
		assert.NotEmpty(t, encrypted, "Encrypted data should not be empty")
		assert.NotEqual(t, testData, encrypted, "Encrypted data should differ from original")

		decrypted, err := storage.DecryptString(encrypted)
		require.NoError(t, err, "Should decrypt successfully")
		assert.Equal(t, testData, decrypted, "Decrypted data should match original")
	})

	t.Run("TestAPIKeyEncryption", func(t *testing.T) {
		config := DefaultSecureStorageConfig()
		storage, err := NewAESSecureStorage(config)
		require.NoError(t, err)

		testAPIKey := "sk-1234567890abcdefghijklmnopqrstuvwxyz"

		// Encrypt API key
		encrypted, err := storage.EncryptAPIKey(testAPIKey)
		require.NoError(t, err, "Should encrypt API key successfully")
		assert.NotEmpty(t, encrypted, "Encrypted API key should not be empty")
		assert.NotEqual(t, testAPIKey, encrypted, "Encrypted API key should differ from original")

		// Decrypt API key
		decrypted, err := storage.DecryptAPIKey(encrypted)
		require.NoError(t, err, "Should decrypt API key successfully")
		assert.Equal(t, testAPIKey, decrypted, "Decrypted API key should match original")
	})

	t.Run("TestTokenEncryption", func(t *testing.T) {
		config := DefaultSecureStorageConfig()
		storage, err := NewAESSecureStorage(config)
		require.NoError(t, err)

		testToken := "token_abcdefghijklmnopqrstuvwxyz123456"

		// Encrypt token
		encrypted, err := storage.EncryptToken(testToken)
		require.NoError(t, err, "Should encrypt token successfully")
		assert.NotEmpty(t, encrypted, "Encrypted token should not be empty")

		// Decrypt token
		decrypted, err := storage.DecryptToken(encrypted)
		require.NoError(t, err, "Should decrypt token successfully")
		assert.Equal(t, testToken, decrypted, "Decrypted token should match original")
	})

	t.Run("TestInvalidDecryption", func(t *testing.T) {
		config := DefaultSecureStorageConfig()
		storage, err := NewAESSecureStorage(config)
		require.NoError(t, err)

		// Test decryption of invalid data
		_, err = storage.DecryptString("invalid_encrypted_data")
		assert.Error(t, err, "Should fail to decrypt invalid data")

		_, err = storage.DecryptAPIKey("invalid_api_key")
		assert.Error(t, err, "Should fail to decrypt invalid API key")

		_, err = storage.DecryptToken("invalid_token")
		assert.Error(t, err, "Should fail to decrypt invalid token")
	})

	t.Run("TestSecureWipe", func(t *testing.T) {
		config := DefaultSecureStorageConfig()
		storage, err := NewAESSecureStorage(config)
		require.NoError(t, err)

		// Test byte slice wiping
		sensitiveData := []byte("sensitive_data_to_wipe")
		storage.SecureWipeBytes(sensitiveData)

		// Verify data is zeroed
		for _, b := range sensitiveData {
			assert.Equal(t, byte(0), b, "All bytes should be zeroed")
		}

		// Test string wiping
		sensitiveString := "sensitive_string_to_wipe"
		storage.SecureWipeString(&sensitiveString)
		assert.Equal(t, "", sensitiveString, "String should be empty after wipe")
	})
}

func TestDataMasker(t *testing.T) {
	t.Run("TestDataMaskerCreation", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)
		assert.NotNil(t, masker, "Masker should not be nil")
	})

	t.Run("TestAPIKeyMasking", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)

		testCases := []struct {
			input    string
			expected string
		}{
			{"sk-1234567890abcdefghij", "sk-****ghij"},
			{"sk-abc123def456", "sk-****f456"},
			{"api_key_abcdefghijklmnop", "ap******mnop"},
			{"short", "*****"},
		}

		for _, tc := range testCases {
			result := masker.MaskAPIKey(tc.input)
			assert.Equal(t, tc.expected, result, "API key masking should work correctly")
			assert.NotEqual(t, tc.input, result, "Masked result should differ from input")
		}
	})

	t.Run("TestTokenMasking", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)

		testCases := []struct {
			input    string
			expected string
		}{
			{"token_1234567890abcdef", "toke********cdef"},
			{"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", "eyJ*****.eyJ*****.****"},
		}

		for _, tc := range testCases {
			result := masker.MaskToken(tc.input)
			assert.NotEqual(t, tc.input, result, "Masked result should differ from input")
			t.Logf("Token masking: %s -> %s", tc.input, result)
		}
	})

	t.Run("TestEmailMasking", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)

		testCases := []struct {
			input    string
			expected string
		}{
			{"user@example.com", "u***@e***.com"},
			{"john.doe@company.co.uk", "j***@c***.co.uk"},
			{"test@domain.org", "t***@d***.org"},
		}

		for _, tc := range testCases {
			result := masker.MaskEmail(tc.input)
			assert.NotEqual(t, tc.input, result, "Masked result should differ from input")
			assert.Contains(t, result, "@", "Masked email should retain @ symbol")
			assert.Contains(t, result, "***", "Masked email should contain masking characters")
			t.Logf("Email masking: %s -> %s", tc.input, result)
		}
	})

	t.Run("TestJSONMasking", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)

		testData := map[string]interface{}{
			"username": "testuser",
			"api_key":  "sk-1234567890abcdefghij",
			"password": "secret123",
			"email":    "user@example.com",
			"public_info": "this is not sensitive",
			"nested": map[string]interface{}{
				"token": "token_abcdefghijk",
				"value": 12345,
			},
		}

		masked := masker.MaskJSON(testData)
		maskedMap, ok := masked.(map[string]interface{})
		require.True(t, ok, "Masked data should be a map")

		// Verify sensitive fields are masked
		assert.NotEqual(t, testData["api_key"], maskedMap["api_key"], "API key should be masked")
		assert.NotEqual(t, testData["password"], maskedMap["password"], "Password should be masked")
		assert.NotEqual(t, testData["email"], maskedMap["email"], "Email should be masked")

		// Verify non-sensitive fields are preserved
		assert.Equal(t, testData["username"], maskedMap["username"], "Username should not be masked")
		assert.Equal(t, testData["public_info"], maskedMap["public_info"], "Public info should not be masked")

		// Verify nested masking
		nestedMasked, ok := maskedMap["nested"].(map[string]interface{})
		require.True(t, ok, "Nested data should be a map")
		nestedOriginal := testData["nested"].(map[string]interface{})
		assert.NotEqual(t, nestedOriginal["token"], nestedMasked["token"], "Nested token should be masked")
		assert.Equal(t, nestedOriginal["value"], nestedMasked["value"], "Nested value should not be masked")
	})

	t.Run("TestSensitiveFieldDetection", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)

		// Test default sensitive fields
		sensitiveFields := []string{"key", "password", "secret", "token", "api_key", "authorization"}
		for _, field := range sensitiveFields {
			assert.True(t, masker.IsSensitiveField(field), "Field %s should be sensitive", field)
			assert.True(t, masker.IsSensitiveField(strings.ToUpper(field)), "Field %s (uppercase) should be sensitive", field)
		}

		// Test non-sensitive fields
		nonSensitiveFields := []string{"username", "email", "name", "id", "value"}
		for _, field := range nonSensitiveFields {
			assert.False(t, masker.IsSensitiveField(field), "Field %s should not be sensitive by default", field)
		}

		// Test custom sensitive field
		masker.AddSensitiveField("custom_secret")
		assert.True(t, masker.IsSensitiveField("custom_secret"), "Custom sensitive field should be detected")

		masker.RemoveSensitiveField("custom_secret")
		assert.False(t, masker.IsSensitiveField("custom_secret"), "Removed field should not be sensitive")
	})

	t.Run("TestStringPatternMasking", func(t *testing.T) {
		config := DefaultDataMaskerConfig()
		masker := NewStandardDataMasker(config)

		testString := "User sk-1234567890abcdef has email user@example.com and phone +1-555-123-4567"
		masked := masker.MaskString(testString)

		assert.NotEqual(t, testString, masked, "String should be masked")
		assert.NotContains(t, masked, "sk-1234567890abcdef", "API key should be masked in string")
		assert.NotContains(t, masked, "user@example.com", "Email should be masked in string")
		assert.Contains(t, masked, "sk-123***", "API key should be partially visible")
		t.Logf("String masking: %s -> %s", testString, masked)
	})
}

func TestSecureLogger(t *testing.T) {
	// Initialize data masker for logger testing
	InitializeDataMasker(DefaultDataMaskerConfig())

	t.Run("TestSecureLoggerCreation", func(t *testing.T) {
		config := DefaultSecureLoggerConfig()
		config.EnableFileOutput = false // Disable file output for testing
		config.AsyncLogging = false     // Disable async for simpler testing

		logger, err := NewStandardSecureLogger(config)
		require.NoError(t, err, "Should create secure logger successfully")
		assert.NotNil(t, logger, "Logger should not be nil")

		defer logger.Close()
	})

	t.Run("TestBasicLogging", func(t *testing.T) {
		config := DefaultSecureLoggerConfig()
		config.EnableFileOutput = false
		config.AsyncLogging = false

		logger, err := NewStandardSecureLogger(config)
		require.NoError(t, err)
		defer logger.Close()

		// Test different log levels
		logger.LogInfo("Test info message", map[string]interface{}{"key": "value"})
		logger.LogWarn("Test warning message", map[string]interface{}{"warning": "details"})
		logger.LogError("Test error message", nil, map[string]interface{}{"error_code": 500})
		logger.LogDebug("Test debug message", map[string]interface{}{"debug": "info"})

		// No assertions here since output goes to console, but should not panic
	})

	t.Run("TestSensitiveDataMasking", func(t *testing.T) {
		config := DefaultSecureLoggerConfig()
		config.EnableFileOutput = false
		config.AsyncLogging = false
		config.EnableMasking = true

		logger, err := NewStandardSecureLogger(config)
		require.NoError(t, err)
		defer logger.Close()

		// Test logging with sensitive data
		sensitiveData := map[string]interface{}{
			"api_key":  "sk-1234567890abcdefghij",
			"password": "secret123",
			"email":    "user@example.com",
			"token":    "token_abcdefghijk",
			"safe_data": "this is not sensitive",
		}

		logger.LogInfo("Testing sensitive data masking", sensitiveData)

		// Verify masking is enabled
		assert.True(t, logger.IsMaskingEnabled(), "Masking should be enabled")
	})

	t.Run("TestSecurityEventLogging", func(t *testing.T) {
		config := DefaultSecureLoggerConfig()
		config.EnableFileOutput = false
		config.AsyncLogging = false
		config.LogSecurityEvents = true

		logger, err := NewStandardSecureLogger(config)
		require.NoError(t, err)
		defer logger.Close()

		// Test security event logging
		logger.LogSecurityEvent("unauthorized_access", map[string]interface{}{
			"ip":        "192.168.1.100",
			"user_id":   123,
			"attempted": "admin_access",
		})

		logger.LogAuthEvent("login_success", 456, map[string]interface{}{
			"ip":        "10.0.0.1",
			"timestamp": time.Now().Unix(),
		})
	})

	t.Run("TestAPICallLogging", func(t *testing.T) {
		config := DefaultSecureLoggerConfig()
		config.EnableFileOutput = false
		config.AsyncLogging = false
		config.EnableAuditMode = true

		logger, err := NewStandardSecureLogger(config)
		require.NoError(t, err)
		defer logger.Close()

		request := map[string]interface{}{
			"method": "POST",
			"path":   "/api/v1/chat",
			"headers": map[string]interface{}{
				"authorization": "Bearer sk-1234567890abcdef",
				"content-type":  "application/json",
			},
		}

		response := map[string]interface{}{
			"status": 200,
			"body":   "API response data",
		}

		logger.LogAPICall(request, response, []string{"authorization", "api_key"})
	})

	t.Run("TestChannelAndTokenLogging", func(t *testing.T) {
		config := DefaultSecureLoggerConfig()
		config.EnableFileOutput = false
		config.AsyncLogging = false

		logger, err := NewStandardSecureLogger(config)
		require.NoError(t, err)
		defer logger.Close()

		// Test channel operation logging
		logger.LogChannelOperation("create", 123, map[string]interface{}{
			"name":    "Test Channel",
			"api_key": "sk-masked_for_logging",
		})

		// Test token operation logging
		logger.LogTokenOperation("generate", 456, map[string]interface{}{
			"token_type": "access",
			"expires":    time.Now().Add(24 * time.Hour).Unix(),
		})
	})
}

func TestSecuritySystemIntegration(t *testing.T) {
	// Set up test environment
	os.Setenv("ONEAPI_MASTER_KEY", "integration_test_master_key_12345")
	defer os.Unsetenv("ONEAPI_MASTER_KEY")

	t.Run("TestFullSecurityStackInitialization", func(t *testing.T) {
		// Initialize all security components
		err := InitializeSecureStorage(DefaultSecureStorageConfig())
		require.NoError(t, err, "Should initialize secure storage")

		InitializeDataMasker(DefaultDataMaskerConfig())

		loggerConfig := DefaultSecureLoggerConfig()
		loggerConfig.EnableFileOutput = false
		loggerConfig.AsyncLogging = false
		err = InitializeSecureLogger(loggerConfig)
		require.NoError(t, err, "Should initialize secure logger")

		// Verify all components are available
		assert.True(t, IsSecureStorageEnabled(), "Secure storage should be enabled")
		assert.True(t, IsDataMaskingEnabled(), "Data masking should be enabled")
		assert.True(t, IsSecureLoggingEnabled(), "Secure logging should be enabled")
	})

	t.Run("TestEndToEndSensitiveDataHandling", func(t *testing.T) {
		// Initialize components
		err := InitializeSecureStorage(DefaultSecureStorageConfig())
		require.NoError(t, err)
		InitializeDataMasker(DefaultDataMaskerConfig())

		// Test API key encryption -> masking -> logging workflow
		originalAPIKey := "sk-1234567890abcdefghijklmnopqrstuvwxyz"

		// Step 1: Encrypt API key for storage
		encryptedAPIKey, err := EncryptAPIKey(originalAPIKey)
		require.NoError(t, err, "Should encrypt API key")
		assert.NotEqual(t, originalAPIKey, encryptedAPIKey, "Encrypted key should differ")

		// Step 2: Decrypt API key for use
		decryptedAPIKey, err := DecryptAPIKey(encryptedAPIKey)
		require.NoError(t, err, "Should decrypt API key")
		assert.Equal(t, originalAPIKey, decryptedAPIKey, "Decrypted key should match original")

		// Step 3: Mask API key for logging
		maskedAPIKey := MaskAPIKeyGlobal(originalAPIKey)
		assert.NotEqual(t, originalAPIKey, maskedAPIKey, "Masked key should differ")
		assert.Contains(t, maskedAPIKey, "sk-****", "Masked key should preserve format")

		// Step 4: Log with automatic masking
		LogSecurityEventGlobal("api_key_test", map[string]interface{}{
			"original_key": originalAPIKey, // This should be automatically masked
			"masked_key":   maskedAPIKey,
		})

		t.Logf("Original: %s", originalAPIKey)
		t.Logf("Encrypted: %s", encryptedAPIKey)
		t.Logf("Masked: %s", maskedAPIKey)
	})

	t.Run("TestSecurityEventDetection", func(t *testing.T) {
		InitializeDataMasker(DefaultDataMaskerConfig())

		// Test sensitive data detection
		testCases := []struct {
			data      string
			sensitive bool
		}{
			{"sk-1234567890abcdef", true},
			{"Bearer token123456789", true},
			{"user@example.com", true},
			{"password: secret123", true},
			{"normal text content", false},
			{"user name and id", false},
		}

		for _, tc := range testCases {
			result := DetectSensitiveData(tc.data)
			assert.Equal(t, tc.sensitive, result, "Sensitive data detection for: %s", tc.data)
		}
	})

	t.Run("TestMaskingConsistency", func(t *testing.T) {
		InitializeDataMasker(DefaultDataMaskerConfig())

		testAPIKey := "sk-1234567890abcdefghijklmnop"

		// Mask the same key multiple times
		masked1 := MaskAPIKeyGlobal(testAPIKey)
		masked2 := MaskAPIKeyGlobal(testAPIKey)
		masked3 := MaskAPIKeyGlobal(testAPIKey)

		// Results should be consistent
		assert.Equal(t, masked1, masked2, "Masking should be consistent")
		assert.Equal(t, masked2, masked3, "Masking should be consistent")

		// All should be different from original
		assert.NotEqual(t, testAPIKey, masked1, "Masked result should differ from original")
	})
}

func TestGlobalSecurityFunctions(t *testing.T) {
	// Set up test environment
	os.Setenv("ONEAPI_MASTER_KEY", "global_test_master_key_12345")
	defer os.Unsetenv("ONEAPI_MASTER_KEY")

	t.Run("TestGlobalAPIKeyOperations", func(t *testing.T) {
		err := InitializeSecureStorage(DefaultSecureStorageConfig())
		require.NoError(t, err)

		testAPIKey := "sk-globaltest1234567890abcdef"

		// Test global encryption/decryption functions
		encrypted, err := EncryptAPIKey(testAPIKey)
		require.NoError(t, err, "Global EncryptAPIKey should work")

		decrypted, err := DecryptAPIKey(encrypted)
		require.NoError(t, err, "Global DecryptAPIKey should work")
		assert.Equal(t, testAPIKey, decrypted, "Global functions should work correctly")
	})

	t.Run("TestGlobalMaskingFunctions", func(t *testing.T) {
		InitializeDataMasker(DefaultDataMaskerConfig())

		testData := map[string]interface{}{
			"api_key": "sk-globalmasktest123456789",
			"email":   "test@globalmask.com",
			"safe":    "not sensitive",
		}

		masked := MaskJSONGlobal(testData)
		maskedMap, ok := masked.(map[string]interface{})
		require.True(t, ok)

		assert.NotEqual(t, testData["api_key"], maskedMap["api_key"])
		assert.NotEqual(t, testData["email"], maskedMap["email"])
		assert.Equal(t, testData["safe"], maskedMap["safe"])
	})

	t.Run("TestIsDataEncrypted", func(t *testing.T) {
		// Test encrypted data detection
		assert.True(t, IsDataEncrypted("v1:base64encodedencrypteddata=="), "Should detect encrypted data")
		assert.False(t, IsDataEncrypted("sk-plaintext123456789"), "Should not detect plaintext as encrypted")
		assert.False(t, IsDataEncrypted("normal text"), "Should not detect normal text as encrypted")
		assert.False(t, IsDataEncrypted(""), "Should not detect empty string as encrypted")
	})
}

// Benchmark tests for performance validation
func BenchmarkSecureStorage(b *testing.B) {
	os.Setenv("ONEAPI_MASTER_KEY", "benchmark_test_key_123456789")
	defer os.Unsetenv("ONEAPI_MASTER_KEY")

	config := DefaultSecureStorageConfig()
	storage, err := NewAESSecureStorage(config)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}

	testData := "sk-benchmarktest1234567890abcdefghijklmnop"

	b.Run("EncryptAPIKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := storage.EncryptAPIKey(testData)
			if err != nil {
				b.Fatalf("Encryption failed: %v", err)
			}
		}
	})

	// Pre-encrypt for decryption benchmark
	encrypted, _ := storage.EncryptAPIKey(testData)

	b.Run("DecryptAPIKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := storage.DecryptAPIKey(encrypted)
			if err != nil {
				b.Fatalf("Decryption failed: %v", err)
			}
		}
	})
}

func BenchmarkDataMasker(b *testing.B) {
	config := DefaultDataMaskerConfig()
	masker := NewStandardDataMasker(config)

	testAPIKey := "sk-benchmark1234567890abcdefghijklmnop"
	testEmail := "benchmark@example.com"
	testJSON := map[string]interface{}{
		"api_key":  testAPIKey,
		"email":    testEmail,
		"password": "secret123",
		"token":    "token_abcdefghijklmnop",
		"safe":     "not sensitive data",
	}

	b.Run("MaskAPIKey", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			masker.MaskAPIKey(testAPIKey)
		}
	})

	b.Run("MaskEmail", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			masker.MaskEmail(testEmail)
		}
	})

	b.Run("MaskJSON", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			masker.MaskJSON(testJSON)
		}
	})
}