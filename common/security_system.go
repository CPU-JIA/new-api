package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// SecuritySystemConfig holds configuration for the entire security system
type SecuritySystemConfig struct {
	// Core security settings
	MasterKey            string        // Master encryption key (from env or config)
	SecurityEnabled      bool          // Enable security features globally
	ForceEncryption      bool          // Force encryption for all new keys
	ValidationInterval   time.Duration // Interval for security validation checks

	// Component configurations
	StorageConfig      *SecureStorageConfig
	MaskerConfig       *DataMaskerConfig
	LoggerConfig       *SecureLoggerConfig

	// Migration settings
	AutoMigrate        bool          // Automatically migrate existing keys
	MigrationTimeout   time.Duration // Timeout for migration operations
	MigrationBatchSize int           // Batch size for migration

	// Monitoring
	HealthCheckInterval time.Duration // Health check interval
	MetricsEnabled      bool          // Enable security metrics collection
}

// DefaultSecuritySystemConfig returns secure default configuration
func DefaultSecuritySystemConfig() *SecuritySystemConfig {
	return &SecuritySystemConfig{
		MasterKey:           os.Getenv("ONEAPI_MASTER_KEY"),
		SecurityEnabled:     true,
		ForceEncryption:     true,
		ValidationInterval:  1 * time.Hour,
		StorageConfig:       DefaultSecureStorageConfig(),
		MaskerConfig:        DefaultDataMaskerConfig(),
		LoggerConfig:        DefaultSecureLoggerConfig(),
		AutoMigrate:         false, // Requires explicit enabling for safety
		MigrationTimeout:    30 * time.Minute,
		MigrationBatchSize:  100,
		HealthCheckInterval: 5 * time.Minute,
		MetricsEnabled:      true,
	}
}

// SecuritySystem manages all security components
type SecuritySystem struct {
	config           *SecuritySystemConfig
	initialized      bool
	healthStatus     map[string]bool
	lastHealthCheck  time.Time
	healthMutex      sync.RWMutex
	shutdownCh       chan struct{}
	wg               sync.WaitGroup
}

// Global security system instance
var (
	globalSecuritySystem *SecuritySystem
	securityInitMutex    sync.Mutex
)

// InitializeSecuritySystem initializes the complete security system
func InitializeSecuritySystem(config *SecuritySystemConfig) error {
	securityInitMutex.Lock()
	defer securityInitMutex.Unlock()

	if globalSecuritySystem != nil {
		return errors.New("security system already initialized")
	}

	if config == nil {
		config = DefaultSecuritySystemConfig()
	}

	// Validate configuration
	if err := validateSecurityConfig(config); err != nil {
		return fmt.Errorf("invalid security configuration: %w", err)
	}

	system := &SecuritySystem{
		config:       config,
		healthStatus: make(map[string]bool),
		shutdownCh:   make(chan struct{}),
	}

	// Initialize components in correct order
	if err := system.initializeComponents(); err != nil {
		return fmt.Errorf("failed to initialize security components: %w", err)
	}

	// Start background services
	if err := system.startBackgroundServices(); err != nil {
		return fmt.Errorf("failed to start background services: %w", err)
	}

	system.initialized = true
	globalSecuritySystem = system

	SysLog("Security system initialized successfully")
	return nil
}

// GetSecuritySystem returns the global security system instance
func GetSecuritySystem() *SecuritySystem {
	return globalSecuritySystem
}

// IsSecuritySystemEnabled returns whether the security system is enabled and initialized
func IsSecuritySystemEnabled() bool {
	return globalSecuritySystem != nil &&
		   globalSecuritySystem.initialized &&
		   globalSecuritySystem.config.SecurityEnabled
}

// initializeComponents initializes all security components in the correct order
func (ss *SecuritySystem) initializeComponents() error {
	// Step 1: Initialize secure storage
	if err := InitializeSecureStorage(ss.config.StorageConfig); err != nil {
		return fmt.Errorf("failed to initialize secure storage: %w", err)
	}
	ss.healthStatus["secure_storage"] = true

	// Step 2: Initialize data masker
	InitializeDataMasker(ss.config.MaskerConfig)
	ss.healthStatus["data_masker"] = true

	// Step 3: Initialize secure logger
	if err := InitializeSecureLogger(ss.config.LoggerConfig); err != nil {
		return fmt.Errorf("failed to initialize secure logger: %w", err)
	}
	ss.healthStatus["secure_logger"] = true

	// Step 4: Log security system startup
	if IsSecureLoggingEnabled() {
		logger := GetSecureLogger()
		logger.LogSecurityEvent("security_system_initialized", map[string]interface{}{
			"encryption_enabled": ss.config.ForceEncryption,
			"auto_migrate":      ss.config.AutoMigrate,
			"validation_interval": ss.config.ValidationInterval.String(),
		})
	}

	return nil
}

// startBackgroundServices starts health checking and monitoring services
func (ss *SecuritySystem) startBackgroundServices() error {
	// Start health check service
	if ss.config.HealthCheckInterval > 0 {
		ss.wg.Add(1)
		go ss.healthCheckService()
	}

	// Start validation service
	if ss.config.ValidationInterval > 0 {
		ss.wg.Add(1)
		go ss.validationService()
	}

	return nil
}

// healthCheckService performs periodic health checks
func (ss *SecuritySystem) healthCheckService() {
	defer ss.wg.Done()

	ticker := time.NewTicker(ss.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ss.performHealthCheck()
		case <-ss.shutdownCh:
			return
		}
	}
}

// validationService performs periodic security validation
func (ss *SecuritySystem) validationService() {
	defer ss.wg.Done()

	ticker := time.NewTicker(ss.config.ValidationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ss.performSecurityValidation()
		case <-ss.shutdownCh:
			return
		}
	}
}

// performHealthCheck checks health of all security components
func (ss *SecuritySystem) performHealthCheck() {
	ss.healthMutex.Lock()
	defer ss.healthMutex.Unlock()

	ss.lastHealthCheck = time.Now()

	// Check secure storage
	if storage := GetSecureStorage(); storage != nil {
		err := storage.ValidateIntegrity()
		ss.healthStatus["secure_storage"] = err == nil
		if err != nil && IsSecureLoggingEnabled() {
			GetSecureLogger().LogError("secure storage health check failed", err, nil)
		}
	} else {
		ss.healthStatus["secure_storage"] = false
	}

	// Check data masker
	ss.healthStatus["data_masker"] = IsDataMaskingEnabled()

	// Check secure logger
	ss.healthStatus["secure_logger"] = IsSecureLoggingEnabled()

	// Log health status if any issues found
	unhealthyComponents := make([]string, 0)
	for component, healthy := range ss.healthStatus {
		if !healthy {
			unhealthyComponents = append(unhealthyComponents, component)
		}
	}

	if len(unhealthyComponents) > 0 && IsSecureLoggingEnabled() {
		GetSecureLogger().LogSecurityEvent("security_health_issues", map[string]interface{}{
			"unhealthy_components": unhealthyComponents,
			"last_check":          ss.lastHealthCheck.Unix(),
		})
	}
}

// performSecurityValidation performs comprehensive security validation
func (ss *SecuritySystem) performSecurityValidation() {
	if !IsSecureLoggingEnabled() {
		return
	}

	logger := GetSecureLogger()

	// Validate encryption keys if channel manager is available
	validationErrors := make([]string, 0)

	// Test basic encryption/decryption
	if storage := GetSecureStorage(); storage != nil {
		testKey := "test_validation_key_" + fmt.Sprint(time.Now().Unix())
		encrypted, err := storage.EncryptAPIKey(testKey)
		if err != nil {
			validationErrors = append(validationErrors, "encryption_test_failed: "+err.Error())
		} else {
			decrypted, err := storage.DecryptAPIKey(encrypted)
			if err != nil || decrypted != testKey {
				validationErrors = append(validationErrors, "decryption_test_failed")
			}
		}
	}

	// Test data masking
	if masker := GetDataMasker(); masker != nil {
		testData := "sk-test1234567890abcdef"
		masked := masker.MaskAPIKey(testData)
		if masked == testData {
			validationErrors = append(validationErrors, "masking_test_failed")
		}
	}

	// Report validation results
	if len(validationErrors) > 0 {
		logger.LogSecurityEvent("security_validation_failed", map[string]interface{}{
			"errors": validationErrors,
			"timestamp": time.Now().Unix(),
		})
	} else {
		logger.LogInfo("security validation passed", map[string]interface{}{
			"timestamp": time.Now().Unix(),
		})
	}
}

// GetHealthStatus returns the current health status of all components
func (ss *SecuritySystem) GetHealthStatus() map[string]interface{} {
	ss.healthMutex.RLock()
	defer ss.healthMutex.RUnlock()

	status := make(map[string]interface{})
	status["initialized"] = ss.initialized
	status["enabled"] = ss.config.SecurityEnabled
	status["last_health_check"] = ss.lastHealthCheck.Unix()

	componentStatus := make(map[string]bool)
	overallHealthy := true

	for component, healthy := range ss.healthStatus {
		componentStatus[component] = healthy
		if !healthy {
			overallHealthy = false
		}
	}

	status["components"] = componentStatus
	status["overall_healthy"] = overallHealthy

	return status
}

// MigrateToEncryption migrates the system to use encryption
func (ss *SecuritySystem) MigrateToEncryption(ctx context.Context) error {
	if !ss.config.ForceEncryption {
		return errors.New("encryption is not enforced in configuration")
	}

	if !IsSecureLoggingEnabled() {
		SysLog("Starting encryption migration (secure logging not available)")
	} else {
		GetSecureLogger().LogSecurityEvent("encryption_migration_started", map[string]interface{}{
			"batch_size": ss.config.MigrationBatchSize,
			"timeout":   ss.config.MigrationTimeout.String(),
		})
	}

	// This would integrate with the secure channel manager when available
	// For now, log the intent
	if IsSecureLoggingEnabled() {
		GetSecureLogger().LogSecurityEvent("encryption_migration_placeholder", map[string]interface{}{
			"message": "Migration logic will be implemented with channel integration",
		})
	}

	// Use context to ensure operation can be cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Shutdown gracefully shuts down the security system
func (ss *SecuritySystem) Shutdown(ctx context.Context) error {
	if !ss.initialized {
		return nil
	}

	// Signal shutdown to background services
	close(ss.shutdownCh)

	// Wait for background services with timeout
	done := make(chan struct{})
	go func() {
		ss.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All services shut down gracefully
	case <-ctx.Done():
		// Timeout exceeded
		return ctx.Err()
	}

	// Close logger if available
	if logger := GetSecureLogger(); logger != nil {
		if closer, ok := logger.(*StandardSecureLogger); ok {
			closer.Close()
		}
	}

	ss.initialized = false
	if IsSecureLoggingEnabled() {
		SysLogSecure("Security system shutdown completed")
	}

	return nil
}

// validateSecurityConfig validates the security system configuration
func validateSecurityConfig(config *SecuritySystemConfig) error {
	if !config.SecurityEnabled {
		return nil // No validation needed if disabled
	}

	if config.MasterKey == "" {
		return errors.New("master key is required when security is enabled")
	}

	if len(config.MasterKey) < 16 {
		return errors.New("master key must be at least 16 characters long")
	}

	if config.StorageConfig == nil {
		return errors.New("storage configuration is required")
	}

	if config.MaskerConfig == nil {
		return errors.New("masker configuration is required")
	}

	if config.LoggerConfig == nil {
		return errors.New("logger configuration is required")
	}

	return nil
}

// Global convenience functions

// ShutdownSecuritySystem gracefully shuts down the global security system
func ShutdownSecuritySystem(ctx context.Context) error {
	securityInitMutex.Lock()
	defer securityInitMutex.Unlock()

	if globalSecuritySystem == nil {
		return nil
	}

	err := globalSecuritySystem.Shutdown(ctx)
	globalSecuritySystem = nil
	return err
}

// GetSecurityHealthStatus returns the global security system health status
func GetSecurityHealthStatus() map[string]interface{} {
	if globalSecuritySystem == nil {
		return map[string]interface{}{
			"initialized": false,
			"error": "security system not initialized",
		}
	}
	return globalSecuritySystem.GetHealthStatus()
}