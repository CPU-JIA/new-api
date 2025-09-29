package validator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfigValidator(t *testing.T) {
	cv := NewConfigValidator()
	assert.NotNil(t, cv)
	assert.NotNil(t, cv.Validator)

	// Check that common validations are registered
	assert.Contains(t, cv.envValidations, "DATABASE_TYPE")
	assert.Contains(t, cv.envValidations, "PORT")
	assert.Contains(t, cv.envValidations, "GIN_MODE")
}

func TestConfigValidator_ValidateCommonConfigs(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
	}{
		{
			name: "Valid configuration",
			envVars: map[string]string{
				"DATABASE_TYPE":             "mysql",
				"PORT":                      "8080",
				"GIN_MODE":                  "release",
				"CHANNEL_UPDATE_FREQUENCY":  "300",
				"BATCH_UPDATE_ENABLED":      "true",
				"ENABLE_PPROF":              "false",
				"UMAMI_WEBSITE_ID":          "12345678-1234-1234-1234-123456789abc",
				"UMAMI_SCRIPT_URL":          "https://analytics.example.com/script.js",
				"RATE_LIMIT_ENABLED":        "true",
				"GLOBAL_RATE_LIMIT":         "100",
				"LOG_LEVEL":                 "info",
				"REDIS_CONN_STRING":         "redis://localhost:6379", // Add missing required var
			},
		},
		{
			name: "Invalid configuration",
			envVars: map[string]string{
				"DATABASE_TYPE":            "invalid_db",
				"PORT":                     "99999",
				"GIN_MODE":                 "invalid_mode",
				"CHANNEL_UPDATE_FREQUENCY": "0",
				"BATCH_UPDATE_ENABLED":     "maybe",
				"ENABLE_PPROF":             "invalid",
				"UMAMI_SCRIPT_URL":         "not-a-url",
				"GLOBAL_RATE_LIMIT":        "-10",
				"LOG_LEVEL":                "unknown",
				"REDIS_CONN_STRING":        "", // Empty required field
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			cv := NewConfigValidator()
			err := cv.ValidateCommonConfigs()

			if tt.expectError {
				require.Error(t, err)
				validationErrors, ok := err.(ValidationErrors)
				require.True(t, ok)
				assert.NotEmpty(t, validationErrors)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidator_ValidateDatabaseConfig(t *testing.T) {
	cv := NewConfigValidator()

	tests := []struct {
		name        string
		config      DatabaseConfig
		expectError bool
	}{
		{
			name: "Valid MySQL config",
			config: DatabaseConfig{
				Type:     "mysql",
				Host:     "localhost",
				Port:     3306,
				Username: "user",
				Password: "pass",
				Database: "testdb",
				URL:      "mysql://user:pass@localhost:3306/testdb",
			},
		},
		{
			name: "Invalid config - missing required fields",
			config: DatabaseConfig{
				Type: "invalid",
				Port: 99999,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateDatabaseConfig(tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidator_ValidateRedisConfig(t *testing.T) {
	cv := NewConfigValidator()

	tests := []struct {
		name        string
		config      RedisConfig
		expectError bool
	}{
		{
			name: "Valid Redis config",
			config: RedisConfig{
				Host:        "localhost",
				Port:        6379,
				Password:    "password",
				Database:    0,
				MaxRetries:  3,
				PoolSize:    10,
				MaxConnAge:  0,
				IdleTimeout: 0,
				ConnString:  "redis://localhost:6379",
			},
		},
		{
			name: "Invalid Redis config - invalid port and database",
			config: RedisConfig{
				Host:       "localhost",
				Port:       99999, // Invalid port
				Database:   20,    // Redis databases are 0-15
				MaxRetries: 15,    // Too many retries
				PoolSize:   200,   // Too large pool
				ConnString: "",    // Missing required connection string
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateRedisConfig(tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidator_ValidateServerConfig(t *testing.T) {
	cv := NewConfigValidator()

	tests := []struct {
		name        string
		config      ServerConfig
		expectError bool
	}{
		{
			name: "Valid server config",
			config: ServerConfig{
				Port:           8080,
				Host:           "0.0.0.0",
				Mode:           "release",
				ReadTimeout:    30,
				WriteTimeout:   30,
				MaxHeaderBytes: 1048576,
				EnablePprof:    false,
			},
		},
		{
			name: "Invalid server config",
			config: ServerConfig{
				Port:           99999, // Invalid port
				Host:           "",    // Missing required host
				Mode:           "invalid", // Invalid mode
				ReadTimeout:    0,     // Too low
				WriteTimeout:   500,   // Too high
				MaxHeaderBytes: 100,   // Too small
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateServerConfig(tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidator_ValidateAllConfigs(t *testing.T) {
	cv := NewConfigValidator()

	validDB := DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Username: "user",
		Password: "pass",
		Database: "testdb",
		URL:      "mysql://user:pass@localhost:3306/testdb",
	}

	validRedis := RedisConfig{
		Host:       "localhost",
		Port:       6379,
		Database:   0,
		MaxRetries: 3,
		PoolSize:   10,
		ConnString: "redis://localhost:6379",
	}

	invalidDB := DatabaseConfig{
		Type: "invalid",
		Port: 99999,
	}

	tests := []struct {
		name        string
		envVars     map[string]string
		configs     []interface{}
		expectError bool
	}{
		{
			name: "All valid configs",
			envVars: map[string]string{
				"DATABASE_TYPE":             "mysql",
				"PORT":                      "8080",
				"GIN_MODE":                  "release",
				"REDIS_CONN_STRING":         "redis://localhost:6379",
				"CHANNEL_UPDATE_FREQUENCY":  "300", // Add missing env var
				"GLOBAL_RATE_LIMIT":         "100", // Add missing env var
			},
			configs: []interface{}{validDB, validRedis},
		},
		{
			name: "Invalid env vars and configs",
			envVars: map[string]string{
				"DATABASE_TYPE": "invalid",
				"PORT":          "99999",
			},
			configs:     []interface{}{invalidDB},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			err := cv.ValidateAllConfigs(tt.configs...)

			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRateLimitConfig_Validation(t *testing.T) {
	cv := NewConfigValidator()

	tests := []struct {
		name        string
		config      RateLimitConfig
		expectError bool
	}{
		{
			name: "Valid rate limit config",
			config: RateLimitConfig{
				Enabled:       true,
				GlobalLimit:   100,
				UserLimit:     10,
				WindowSeconds: 60,
			},
		},
		{
			name: "Invalid rate limit config",
			config: RateLimitConfig{
				Enabled:       true,
				GlobalLimit:   -10,  // Negative not allowed
				UserLimit:     -5,   // Negative not allowed
				WindowSeconds: 0,    // Must be at least 1
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateStruct(tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLogConfig_Validation(t *testing.T) {
	cv := NewConfigValidator()

	tests := []struct {
		name        string
		config      LogConfig
		expectError bool
	}{
		{
			name: "Valid log config",
			config: LogConfig{
				Level:      "info",
				Format:     "json",
				Output:     "file",
				File:       "/var/log/app.log",
				MaxSize:    100,
				MaxBackups: 5,
				MaxAge:     30,
				Compress:   true,
			},
		},
		{
			name: "Invalid log config",
			config: LogConfig{
				Level:      "invalid", // Invalid level
				Format:     "xml",     // Invalid format
				Output:     "network", // Invalid output
				MaxSize:    2000,      // Too large
				MaxBackups: 200,       // Too many
				MaxAge:     500,       // Too long
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := cv.ValidateStruct(tt.config)

			if tt.expectError {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckRequiredEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		required    []string
		envVars     map[string]string
		expectError bool
	}{
		{
			name:     "All required vars present",
			required: []string{"VAR1", "VAR2"},
			envVars:  map[string]string{"VAR1": "value1", "VAR2": "value2"},
		},
		{
			name:        "Some required vars missing",
			required:    []string{"VAR1", "VAR2", "VAR3"},
			envVars:     map[string]string{"VAR1": "value1"},
			expectError: true,
		},
		{
			name:     "No required vars",
			required: []string{},
			envVars:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all environment variables first
			for _, envVar := range tt.required {
				os.Unsetenv(envVar)
			}

			// Set up environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			err := CheckRequiredEnvVars(tt.required)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "missing required environment variables")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}