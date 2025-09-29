package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"one-api/common"
	"one-api/types"
	"strings"
	"sync"
	"time"
)

// SecureChannelConfig holds configuration for secure channel operations
type SecureChannelConfig struct {
	// Encryption settings
	EnableEncryption    bool   // Enable API key encryption
	EncryptionVersion   int    // Current encryption version
	BatchSize          int    // Batch size for migration operations
	MigrationTimeout   time.Duration // Timeout for migration operations

	// Logging settings
	LogKeyAccess       bool   // Log all key access operations
	LogDecryption      bool   // Log decryption operations
	MaskKeysInLogs     bool   // Mask keys in all logs
}

// DefaultSecureChannelConfig returns secure default configuration
func DefaultSecureChannelConfig() *SecureChannelConfig {
	return &SecureChannelConfig{
		EnableEncryption:   true,
		EncryptionVersion:  1,
		BatchSize:          100,
		MigrationTimeout:   30 * time.Minute,
		LogKeyAccess:      true,
		LogDecryption:     false, // Avoid excessive logging
		MaskKeysInLogs:    true,
	}
}

// SecureChannelManager manages secure channel operations
type SecureChannelManager struct {
	config       *SecureChannelConfig
	storage      common.SecureStorage
	masker       common.DataMasker
	logger       common.SecureLogger
	migrationMux sync.RWMutex
}

// Global secure channel manager instance
var globalSecureChannelManager *SecureChannelManager

// InitializeSecureChannelManager initializes the global secure channel manager
func InitializeSecureChannelManager(config *SecureChannelConfig) error {
	if config == nil {
		config = DefaultSecureChannelConfig()
	}

	// Initialize security dependencies
	if !common.IsSecureStorageEnabled() {
		return errors.New("secure storage not initialized")
	}
	if !common.IsDataMaskingEnabled() {
		return errors.New("data masking not initialized")
	}

	manager := &SecureChannelManager{
		config:  config,
		storage: common.GetSecureStorage(),
		masker:  common.GetDataMasker(),
		logger:  common.GetSecureLogger(),
	}

	globalSecureChannelManager = manager

	if manager.logger != nil {
		manager.logger.LogSecurityEvent("secure_channel_manager_initialized", map[string]interface{}{
			"encryption_enabled": config.EnableEncryption,
			"logging_enabled":   config.LogKeyAccess,
		})
	}

	return nil
}

// GetSecureChannelManager returns the global secure channel manager
func GetSecureChannelManager() *SecureChannelManager {
	return globalSecureChannelManager
}

// IsSecureChannelEnabled returns whether secure channel management is available
func IsSecureChannelEnabled() bool {
	return globalSecureChannelManager != nil &&
		   globalSecureChannelManager.config.EnableEncryption &&
		   common.IsSecureStorageEnabled()
}

// SecureChannel extends Channel with security methods
type SecureChannel struct {
	*Channel
	manager *SecureChannelManager
}

// NewSecureChannel creates a new SecureChannel instance
func NewSecureChannel(channel *Channel) *SecureChannel {
	return &SecureChannel{
		Channel: channel,
		manager: globalSecureChannelManager,
	}
}

// EncryptKey encrypts and stores the API key
func (sc *SecureChannel) EncryptKey(ctx context.Context) error {
	if sc.manager == nil {
		return errors.New("secure channel manager not initialized")
	}

	if sc.Key == "" {
		return errors.New("cannot encrypt empty key")
	}

	// Check if already encrypted
	if common.IsDataEncrypted(sc.Key) {
		sc.logKeyAccess("key_already_encrypted", nil)
		return nil
	}

	// Encrypt the key
	encryptedKey, err := sc.manager.storage.EncryptAPIKey(sc.Key)
	if err != nil {
		sc.logKeyAccess("key_encryption_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("failed to encrypt API key: %w", err)
	}

	// Store encrypted key
	originalKey := sc.Key
	sc.Key = encryptedKey

	// Log the operation
	sc.logKeyAccess("key_encrypted", map[string]interface{}{
		"channel_id": sc.Id,
		"key_length": len(originalKey),
	})

	// Secure wipe original key
	common.SecureWipeBytes([]byte(originalKey))

	return nil
}

// DecryptKey decrypts and returns the API key
func (sc *SecureChannel) DecryptKey() (string, error) {
	if sc.manager == nil {
		return sc.Key, nil // Fallback to plaintext if not initialized
	}

	// If not encrypted, return as-is
	if !common.IsDataEncrypted(sc.Key) {
		sc.logKeyAccess("key_plaintext_access", nil)
		return sc.Key, nil
	}

	// Decrypt the key
	decryptedKey, err := sc.manager.storage.DecryptAPIKey(sc.Key)
	if err != nil {
		sc.logKeyAccess("key_decryption_failed", map[string]interface{}{
			"error": err.Error(),
		})
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	if sc.manager.config.LogDecryption {
		sc.logKeyAccess("key_decrypted", nil)
	}

	return decryptedKey, nil
}

// GetSecureKeys returns decrypted keys array
func (sc *SecureChannel) GetSecureKeys() ([]string, error) {
	if sc.Key == "" {
		return []string{}, nil
	}

	// Get decrypted key
	decryptedKey, err := sc.DecryptKey()
	if err != nil {
		return nil, err
	}

	// Parse keys using existing logic
	trimmed := strings.TrimSpace(decryptedKey)
	if strings.HasPrefix(trimmed, "[") {
		var arr []json.RawMessage
		if err := common.Unmarshal([]byte(trimmed), &arr); err == nil {
			res := make([]string, len(arr))
			for i, v := range arr {
				res[i] = string(v)
			}
			return res, nil
		}
	}

	// Split by newline
	keys := strings.Split(strings.Trim(decryptedKey, "\n"), "\n")

	sc.logKeyAccess("keys_accessed", map[string]interface{}{
		"key_count": len(keys),
	})

	return keys, nil
}

// GetNextEnabledSecureKey returns next enabled key with security
func (sc *SecureChannel) GetNextEnabledSecureKey() (string, int, *types.NewAPIError) {
	// If not in multi-key mode, return decrypted key
	if !sc.ChannelInfo.IsMultiKey {
		decryptedKey, err := sc.DecryptKey()
		if err != nil {
			return "", 0, types.NewError(err, types.ErrorCodeChannelKeyDecryptionFailed)
		}
		return decryptedKey, 0, nil
	}

	// Get all decrypted keys
	keys, err := sc.GetSecureKeys()
	if err != nil {
		return "", 0, types.NewError(err, types.ErrorCodeChannelKeyDecryptionFailed)
	}

	if len(keys) == 0 {
		return "", 0, types.NewError(errors.New("no keys available"), types.ErrorCodeChannelNoAvailableKey)
	}

	// Use existing multi-key logic (simplified version)
	lock := GetChannelPollingLock(sc.Id)
	lock.Lock()
	defer lock.Unlock()

	statusList := sc.ChannelInfo.MultiKeyStatusList
	getStatus := func(idx int) int {
		if statusList == nil {
			return common.ChannelStatusEnabled
		}
		if status, ok := statusList[idx]; ok {
			return status
		}
		return common.ChannelStatusEnabled
	}

	// Find enabled keys
	enabledIdx := make([]int, 0, len(keys))
	for i := range keys {
		if getStatus(i) == common.ChannelStatusEnabled {
			enabledIdx = append(enabledIdx, i)
		}
	}

	if len(enabledIdx) == 0 {
		return keys[0], 0, nil
	}

	selectedIdx := enabledIdx[0] // Simplified: return first enabled
	selectedKey := keys[selectedIdx]

	sc.logKeyAccess("secure_key_selected", map[string]interface{}{
		"key_index":     selectedIdx,
		"enabled_count": len(enabledIdx),
	})

	return selectedKey, selectedIdx, nil
}

// logKeyAccess logs key access operations with masking
func (sc *SecureChannel) logKeyAccess(operation string, details map[string]interface{}) {
	if sc.manager == nil || sc.manager.logger == nil || !sc.manager.config.LogKeyAccess {
		return
	}

	if details == nil {
		details = make(map[string]interface{})
	}

	details["channel_id"] = sc.Id
	details["channel_name"] = sc.manager.masker.MaskString(sc.Name)

	sc.manager.logger.LogSecurityEvent(fmt.Sprintf("channel_%s", operation), details)
}

// MigrateChannelKeysToEncrypted migrates plaintext keys to encrypted format
func (scm *SecureChannelManager) MigrateChannelKeysToEncrypted(ctx context.Context) error {
	if !scm.config.EnableEncryption {
		return errors.New("encryption is not enabled")
	}

	scm.migrationMux.Lock()
	defer scm.migrationMux.Unlock()

	if scm.logger != nil {
		scm.logger.LogSecurityEvent("channel_key_migration_started", map[string]interface{}{
			"batch_size": scm.config.BatchSize,
		})
	}

	// Create timeout context
	migrationCtx, cancel := context.WithTimeout(ctx, scm.config.MigrationTimeout)
	defer cancel()

	var totalMigrated, totalErrors int
	offset := 0

	for {
		// Check context cancellation
		select {
		case <-migrationCtx.Done():
			return fmt.Errorf("migration timeout: %w", migrationCtx.Err())
		default:
		}

		// Get batch of channels with plaintext keys
		var channels []Channel
		err := DB.Where("key != '' AND key NOT LIKE 'v%:%'").
			Offset(offset).
			Limit(scm.config.BatchSize).
			Find(&channels).Error

		if err != nil {
			return fmt.Errorf("failed to fetch channels: %w", err)
		}

		if len(channels) == 0 {
			break // No more channels to migrate
		}

		// Process batch
		for _, channel := range channels {
			sc := NewSecureChannel(&channel)

			err := sc.EncryptKey(migrationCtx)
			if err != nil {
				totalErrors++
				if scm.logger != nil {
					scm.logger.LogError("channel key migration failed", err, map[string]interface{}{
						"channel_id": channel.Id,
					})
				}
				continue
			}

			// Save encrypted key
			err = DB.Model(&channel).Update("key", sc.Key).Error
			if err != nil {
				totalErrors++
				if scm.logger != nil {
					scm.logger.LogError("failed to save encrypted key", err, map[string]interface{}{
						"channel_id": channel.Id,
					})
				}
				continue
			}

			totalMigrated++
		}

		offset += scm.config.BatchSize

		// Progress logging
		if scm.logger != nil && totalMigrated%100 == 0 {
			scm.logger.LogInfo("migration progress", map[string]interface{}{
				"migrated": totalMigrated,
				"errors":   totalErrors,
			})
		}
	}

	if scm.logger != nil {
		scm.logger.LogSecurityEvent("channel_key_migration_completed", map[string]interface{}{
			"total_migrated": totalMigrated,
			"total_errors":   totalErrors,
		})
	}

	return nil
}

// ValidateChannelKeyIntegrity validates encrypted channel keys
func (scm *SecureChannelManager) ValidateChannelKeyIntegrity(ctx context.Context) error {
	var channels []Channel
	err := DB.Where("key != '' AND key LIKE 'v%:%'").Find(&channels).Error
	if err != nil {
		return fmt.Errorf("failed to fetch encrypted channels: %w", err)
	}

	var validationErrors []string
	validCount := 0

	for _, channel := range channels {
		sc := NewSecureChannel(&channel)

		_, err := sc.DecryptKey()
		if err != nil {
			validationErrors = append(validationErrors,
				fmt.Sprintf("Channel %d: %v", channel.Id, err))
		} else {
			validCount++
		}
	}

	if scm.logger != nil {
		scm.logger.LogSecurityEvent("channel_key_validation_completed", map[string]interface{}{
			"total_channels":    len(channels),
			"valid_channels":    validCount,
			"validation_errors": len(validationErrors),
		})
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("validation failed for %d channels: %v",
			len(validationErrors), validationErrors[:min(5, len(validationErrors))])
	}

	return nil
}

// GetChannelSecurely retrieves a channel with secure key handling
func GetChannelSecurely(id int) (*SecureChannel, error) {
	var channel Channel
	err := DB.First(&channel, id).Error
	if err != nil {
		return nil, err
	}

	sc := NewSecureChannel(&channel)

	// Log secure access
	if globalSecureChannelManager != nil && globalSecureChannelManager.logger != nil {
		sc.logKeyAccess("channel_accessed_securely", nil)
	}

	return sc, nil
}

// ChannelKeyStatus represents the encryption status of a channel key
type ChannelKeyStatus struct {
	ChannelID    int    `json:"channel_id"`
	ChannelName  string `json:"channel_name"`
	IsEncrypted  bool   `json:"is_encrypted"`
	CanDecrypt   bool   `json:"can_decrypt"`
	LastChecked  int64  `json:"last_checked"`
	Error        string `json:"error,omitempty"`
}

// GetChannelKeySecurityStatus returns security status for all channels
func (scm *SecureChannelManager) GetChannelKeySecurityStatus(ctx context.Context) ([]ChannelKeyStatus, error) {
	var channels []Channel
	err := DB.Select("id, name, key").Find(&channels).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channels: %w", err)
	}

	results := make([]ChannelKeyStatus, len(channels))

	for i, channel := range channels {
		status := ChannelKeyStatus{
			ChannelID:   channel.Id,
			ChannelName: scm.masker.MaskString(channel.Name),
			IsEncrypted: common.IsDataEncrypted(channel.Key),
			LastChecked: time.Now().Unix(),
		}

		// Test decryption if encrypted
		if status.IsEncrypted {
			sc := NewSecureChannel(&channel)
			_, err := sc.DecryptKey()
			status.CanDecrypt = err == nil
			if err != nil {
				status.Error = scm.masker.MaskString(err.Error())
			}
		} else {
			status.CanDecrypt = true // Plaintext is always "decryptable"
		}

		results[i] = status
	}

	return results, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}