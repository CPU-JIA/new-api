package validator

import (
	"fmt"
	"one-api/common"
	"os"
)

// ConfigValidator provides validation for new-api specific configurations
type ConfigValidator struct {
	*Validator
}

// NewConfigValidator creates a new config validator with predefined rules for new-api
func NewConfigValidator() *ConfigValidator {
	validator := NewValidator()
	cv := &ConfigValidator{Validator: validator}

	// Register common environment variable validations
	cv.registerCommonEnvValidations()

	return cv
}

// registerCommonEnvValidations registers validation rules for common environment variables
func (cv *ConfigValidator) registerCommonEnvValidations() {
	// Database configuration
	cv.AddEnvValidation("DATABASE_TYPE", &OneOfRule{Values: []string{"sqlite", "mysql", "postgres", "postgresql"}})
	cv.AddEnvValidation("DATABASE_URL", &URLRule{})

	// Redis configuration
	cv.AddEnvValidation("REDIS_CONN_STRING", &RequiredRule{})

	// Server configuration
	cv.AddEnvValidation("PORT", &PortRule{})
	cv.AddEnvValidation("GIN_MODE", &OneOfRule{Values: []string{"debug", "release", "test"}})

	// Channel update frequency
	cv.AddEnvValidation("CHANNEL_UPDATE_FREQUENCY", &RangeRule{Min: 1, Max: 3600})

	// Feature flags
	cv.AddEnvValidation("BATCH_UPDATE_ENABLED", &BoolRule{})
	cv.AddEnvValidation("ENABLE_PPROF", &BoolRule{})

	// Analytics configuration
	cv.AddEnvValidation("UMAMI_WEBSITE_ID", &RegexRule{Pattern: `^[a-f0-9-]+$`})
	cv.AddEnvValidation("UMAMI_SCRIPT_URL", &URLRule{})

	// Rate limiting
	cv.AddEnvValidation("RATE_LIMIT_ENABLED", &BoolRule{})
	cv.AddEnvValidation("GLOBAL_RATE_LIMIT", &MinRule{Min: 0})

	// Log level
	cv.AddEnvValidation("LOG_LEVEL", &OneOfRule{Values: []string{"debug", "info", "warn", "error", "fatal"}})
}

// ValidateCommonConfigs validates commonly used configuration structures
func (cv *ConfigValidator) ValidateCommonConfigs() error {
	errors := make(ValidationErrors, 0)

	// Validate environment variables
	if err := cv.ValidateEnvVars(); err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrors...)
		}
	}

	// Validate runtime configuration
	if err := cv.validateRuntimeConfig(); err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrors...)
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// validateRuntimeConfig validates runtime configuration values
func (cv *ConfigValidator) validateRuntimeConfig() error {
	errors := make(ValidationErrors, 0)

	// Validate common runtime settings only if they are initialized
	if common.Port != nil && (*common.Port < 1 || *common.Port > 65535) {
		errors = append(errors, ValidationError{
			Field:   "Port",
			Value:   *common.Port,
			Rule:    "port",
			Message: "port must be between 1 and 65535",
		})
	}

	// Only validate batch update interval if it's been set to a non-zero value
	if common.BatchUpdateInterval > 0 && (common.BatchUpdateInterval < 1 || common.BatchUpdateInterval > 3600) {
		errors = append(errors, ValidationError{
			Field:   "BatchUpdateInterval",
			Value:   common.BatchUpdateInterval,
			Rule:    "range",
			Message: "batch update interval must be between 1 and 3600 seconds",
		})
	}

	if len(errors) > 0 {
		return ValidationErrors(errors)
	}

	return nil
}

// ValidateDatabaseConfig validates database configuration
func (cv *ConfigValidator) ValidateDatabaseConfig(config DatabaseConfig) error {
	return cv.ValidateStruct(config)
}

// ValidateRedisConfig validates Redis configuration
func (cv *ConfigValidator) ValidateRedisConfig(config RedisConfig) error {
	return cv.ValidateStruct(config)
}

// ValidateServerConfig validates server configuration
func (cv *ConfigValidator) ValidateServerConfig(config ServerConfig) error {
	return cv.ValidateStruct(config)
}

// Common configuration structures with validation tags

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Type     string `json:"type" validate:"required,oneof=sqlite mysql postgres postgresql"`
	Host     string `json:"host" validate:"required"`
	Port     int    `json:"port" validate:"required,port"`
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
	Database string `json:"database" validate:"required"`
	URL      string `json:"url" validate:"url"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Host        string `json:"host" validate:"required"`
	Port        int    `json:"port" validate:"required,port"`
	Password    string `json:"password"`
	Database    int    `json:"database" validate:"min=0,max=15"`
	MaxRetries  int    `json:"max_retries" validate:"min=0,max=10"`
	PoolSize    int    `json:"pool_size" validate:"min=1,max=100"`
	MaxConnAge  int    `json:"max_conn_age" validate:"min=0"`
	IdleTimeout int    `json:"idle_timeout" validate:"min=0"`
	ConnString  string `json:"conn_string" validate:"required"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Port           int    `json:"port" validate:"required,port"`
	Host           string `json:"host" validate:"required"`
	Mode           string `json:"mode" validate:"required,oneof=debug release test"`
	ReadTimeout    int    `json:"read_timeout" validate:"min=1,max=300"`
	WriteTimeout   int    `json:"write_timeout" validate:"min=1,max=300"`
	MaxHeaderBytes int    `json:"max_header_bytes" validate:"min=1024"`
	EnablePprof    bool   `json:"enable_pprof"`
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	Enabled       bool `json:"enabled"`
	GlobalLimit   int  `json:"global_limit" validate:"min=0"`
	UserLimit     int  `json:"user_limit" validate:"min=0"`
	WindowSeconds int  `json:"window_seconds" validate:"min=1,max=3600"`
}

// LogConfig represents logging configuration
type LogConfig struct {
	Level      string `json:"level" validate:"required,oneof=debug info warn error fatal"`
	Format     string `json:"format" validate:"oneof=json text"`
	Output     string `json:"output" validate:"oneof=stdout stderr file"`
	File       string `json:"file"`
	MaxSize    int    `json:"max_size" validate:"min=1,max=1000"`
	MaxBackups int    `json:"max_backups" validate:"min=0,max=100"`
	MaxAge     int    `json:"max_age" validate:"min=0,max=365"`
	Compress   bool   `json:"compress"`
}

// ValidateAllConfigs validates all configuration types at once
func (cv *ConfigValidator) ValidateAllConfigs(configs ...interface{}) error {
	errors := make(ValidationErrors, 0)

	// First validate environment variables
	if err := cv.ValidateEnvVars(); err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrors...)
		}
	}

	// Then validate each provided config struct
	for _, config := range configs {
		if err := cv.ValidateStruct(config); err != nil {
			if validationErrors, ok := err.(ValidationErrors); ok {
				errors = append(errors, validationErrors...)
			}
		}
	}

	// Finally validate runtime config
	if err := cv.validateRuntimeConfig(); err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			errors = append(errors, validationErrors...)
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// PrintValidationSummary prints a summary of validation results
func (cv *ConfigValidator) PrintValidationSummary() {
	if err := cv.ValidateCommonConfigs(); err != nil {
		if validationErrors, ok := err.(ValidationErrors); ok {
			common.SysError("Configuration validation failed:")
			for _, vErr := range validationErrors {
				common.SysError(fmt.Sprintf("  - %s", vErr.Error()))
			}
		}
	} else {
		common.SysLog("All configurations validated successfully")
	}
}

// CheckRequired verifies that all required environment variables are set
func CheckRequiredEnvVars(required []string) error {
	var missing []string

	for _, envVar := range required {
		if os.Getenv(envVar) == "" {
			missing = append(missing, envVar)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	return nil
}