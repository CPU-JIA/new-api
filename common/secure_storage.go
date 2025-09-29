package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// SecureStorage defines the interface for secure encryption/decryption operations
type SecureStorage interface {
	// Core encryption operations
	EncryptSensitiveData(data []byte) ([]byte, error)
	DecryptSensitiveData(encrypted []byte) ([]byte, error)

	// String convenience methods
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)

	// API key specific operations
	EncryptAPIKey(key string) (string, error)
	DecryptAPIKey(encrypted string) (string, error)

	// Token specific operations
	EncryptToken(token string) (string, error)
	DecryptToken(encrypted string) (string, error)

	// Secure memory operations
	SecureWipeBytes(data []byte)
	SecureWipeString(data *string)

	// Key management
	RotateEncryptionKey() error
	ValidateIntegrity() error
}

// AESSecureStorage implements SecureStorage using AES-256-GCM
type AESSecureStorage struct {
	masterKey    []byte
	keyVersion   int
}

// SecureStorageConfig holds configuration for the secure storage system
type SecureStorageConfig struct {
	MasterPassword     string // Master password for key derivation
	KeyVersion         int    // Current key version for rotation
	PBKDF2Iterations   int    // Number of PBKDF2 iterations
	EnableMemoryWipe   bool   // Enable secure memory wiping
}

// DefaultSecureStorageConfig returns secure default configuration
func DefaultSecureStorageConfig() *SecureStorageConfig {
	return &SecureStorageConfig{
		MasterPassword:   "", // Must be set by user
		KeyVersion:       1,
		PBKDF2Iterations: 100000, // OWASP recommended minimum
		EnableMemoryWipe: true,
	}
}

// NewAESSecureStorage creates a new AES-based secure storage instance
func NewAESSecureStorage(config *SecureStorageConfig) (*AESSecureStorage, error) {
	if config == nil {
		return nil, errors.New("secure storage config cannot be nil")
	}

	if config.MasterPassword == "" {
		// Try to get master key from environment
		config.MasterPassword = os.Getenv("ONEAPI_MASTER_KEY")
		if config.MasterPassword == "" {
			return nil, errors.New("master password not provided and ONEAPI_MASTER_KEY environment variable not set")
		}
	}

	// Derive master key using PBKDF2
	salt := []byte("oneapi_salt_v1") // Fixed salt for consistency (in production, should be random and stored)
	masterKey := pbkdf2.Key([]byte(config.MasterPassword), salt, config.PBKDF2Iterations, 32, sha256.New)

	storage := &AESSecureStorage{
		masterKey:  masterKey,
		keyVersion: config.KeyVersion,
	}

	// Validate the setup
	if err := storage.ValidateIntegrity(); err != nil {
		return nil, fmt.Errorf("secure storage integrity validation failed: %w", err)
	}

	return storage, nil
}

// EncryptSensitiveData encrypts data using AES-256-GCM
func (s *AESSecureStorage) EncryptSensitiveData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot encrypt empty data")
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	return ciphertext, nil
}

// DecryptSensitiveData decrypts data using AES-256-GCM
func (s *AESSecureStorage) DecryptSensitiveData(encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, errors.New("cannot decrypt empty data")
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimum length
	if len(encrypted) < gcm.NonceSize() {
		return nil, errors.New("encrypted data too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := encrypted[:gcm.NonceSize()], encrypted[gcm.NonceSize():]

	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64 encoded result
func (s *AESSecureStorage) EncryptString(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("cannot encrypt empty string")
	}

	// Convert to bytes
	plaintextBytes := []byte(plaintext)

	// Encrypt
	encrypted, err := s.EncryptSensitiveData(plaintextBytes)
	if err != nil {
		return "", err
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(encrypted)

	// Add version prefix for future compatibility
	versioned := fmt.Sprintf("v%d:%s", s.keyVersion, encoded)

	// Secure wipe plaintext bytes
	s.SecureWipeBytes(plaintextBytes)

	return versioned, nil
}

// DecryptString decrypts a base64 encoded string
func (s *AESSecureStorage) DecryptString(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", errors.New("cannot decrypt empty string")
	}

	// Parse version prefix
	parts := strings.SplitN(ciphertext, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid encrypted string format")
	}

	var version int
	var encoded string
	if _, err := fmt.Sscanf(parts[0], "v%d", &version); err != nil {
		return "", fmt.Errorf("failed to parse version: %w", err)
	}
	encoded = parts[1]

	// For now, we only support version 1
	if version != 1 {
		return "", fmt.Errorf("unsupported encryption version: %d", version)
	}

	// Decode from base64
	encrypted, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// Decrypt
	decrypted, err := s.DecryptSensitiveData(encrypted)
	if err != nil {
		return "", err
	}

	// Convert to string
	result := string(decrypted)

	// Secure wipe decrypted bytes
	s.SecureWipeBytes(decrypted)

	return result, nil
}

// EncryptAPIKey encrypts an API key with additional context
func (s *AESSecureStorage) EncryptAPIKey(key string) (string, error) {
	if key == "" {
		return "", errors.New("API key cannot be empty")
	}

	// Add context prefix to distinguish from other encrypted data
	contextualKey := "APIKEY:" + key

	encrypted, err := s.EncryptString(contextualKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt API key: %w", err)
	}

	return encrypted, nil
}

// DecryptAPIKey decrypts an API key and validates context
func (s *AESSecureStorage) DecryptAPIKey(encrypted string) (string, error) {
	if encrypted == "" {
		return "", errors.New("encrypted API key cannot be empty")
	}

	decrypted, err := s.DecryptString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt API key: %w", err)
	}

	// Validate context prefix
	const prefix = "APIKEY:"
	if !strings.HasPrefix(decrypted, prefix) {
		// Secure wipe before returning error
		s.SecureWipeString(&decrypted)
		return "", errors.New("invalid API key format")
	}

	// Extract actual key
	key := decrypted[len(prefix):]

	// Secure wipe decrypted string
	s.SecureWipeString(&decrypted)

	return key, nil
}

// EncryptToken encrypts a token with additional context
func (s *AESSecureStorage) EncryptToken(token string) (string, error) {
	if token == "" {
		return "", errors.New("token cannot be empty")
	}

	// Add context prefix to distinguish from other encrypted data
	contextualToken := "TOKEN:" + token

	encrypted, err := s.EncryptString(contextualToken)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt token: %w", err)
	}

	return encrypted, nil
}

// DecryptToken decrypts a token and validates context
func (s *AESSecureStorage) DecryptToken(encrypted string) (string, error) {
	if encrypted == "" {
		return "", errors.New("encrypted token cannot be empty")
	}

	decrypted, err := s.DecryptString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	// Validate context prefix
	const prefix = "TOKEN:"
	if !strings.HasPrefix(decrypted, prefix) {
		// Secure wipe before returning error
		s.SecureWipeString(&decrypted)
		return "", errors.New("invalid token format")
	}

	// Extract actual token
	token := decrypted[len(prefix):]

	// Secure wipe decrypted string
	s.SecureWipeString(&decrypted)

	return token, nil
}

// SecureWipeBytes securely wipes a byte slice from memory
func (s *AESSecureStorage) SecureWipeBytes(data []byte) {
	if len(data) == 0 {
		return
	}

	// Zero out the memory
	for i := range data {
		data[i] = 0
	}

	// Force garbage collection to reduce time sensitive data spends in memory
	runtime.GC()
}

// SecureWipeString securely wipes a string from memory (best effort)
func (s *AESSecureStorage) SecureWipeString(data *string) {
	if data == nil || *data == "" {
		return
	}

	// Convert to bytes for wiping
	bytes := []byte(*data)
	s.SecureWipeBytes(bytes)

	// Clear the string reference
	*data = ""

	runtime.GC()
}

// RotateEncryptionKey rotates the encryption key (placeholder for future implementation)
func (s *AESSecureStorage) RotateEncryptionKey() error {
	// This would implement key rotation logic
	// For now, return not implemented
	return errors.New("key rotation not yet implemented")
}

// ValidateIntegrity validates the integrity of the secure storage system
func (s *AESSecureStorage) ValidateIntegrity() error {
	// Test encryption/decryption roundtrip
	testData := "integrity_check_" + fmt.Sprintf("%d", s.keyVersion)

	encrypted, err := s.EncryptString(testData)
	if err != nil {
		return fmt.Errorf("integrity check encryption failed: %w", err)
	}

	decrypted, err := s.DecryptString(encrypted)
	if err != nil {
		return fmt.Errorf("integrity check decryption failed: %w", err)
	}

	if decrypted != testData {
		return errors.New("integrity check failed: data mismatch")
	}

	// Secure wipe test data
	s.SecureWipeString(&decrypted)

	return nil
}

// Global secure storage instance
var globalSecureStorage SecureStorage

// InitializeSecureStorage initializes the global secure storage instance
func InitializeSecureStorage(config *SecureStorageConfig) error {
	storage, err := NewAESSecureStorage(config)
	if err != nil {
		return fmt.Errorf("failed to initialize secure storage: %w", err)
	}

	globalSecureStorage = storage
	SysLog("Secure storage system initialized successfully")

	return nil
}

// GetSecureStorage returns the global secure storage instance
func GetSecureStorage() SecureStorage {
	return globalSecureStorage
}

// IsSecureStorageEnabled returns whether secure storage is available
func IsSecureStorageEnabled() bool {
	return globalSecureStorage != nil
}

// Convenience functions for global secure storage

// EncryptAPIKey encrypts an API key using the global secure storage
func EncryptAPIKey(key string) (string, error) {
	if globalSecureStorage == nil {
		return "", errors.New("secure storage not initialized")
	}
	return globalSecureStorage.EncryptAPIKey(key)
}

// DecryptAPIKey decrypts an API key using the global secure storage
func DecryptAPIKey(encrypted string) (string, error) {
	if globalSecureStorage == nil {
		return "", errors.New("secure storage not initialized")
	}
	return globalSecureStorage.DecryptAPIKey(encrypted)
}

// EncryptToken encrypts a token using the global secure storage
func EncryptToken(token string) (string, error) {
	if globalSecureStorage == nil {
		return "", errors.New("secure storage not initialized")
	}
	return globalSecureStorage.EncryptToken(token)
}

// DecryptToken decrypts a token using the global secure storage
func DecryptToken(encrypted string) (string, error) {
	if globalSecureStorage == nil {
		return "", errors.New("secure storage not initialized")
	}
	return globalSecureStorage.DecryptToken(encrypted)
}

// SecureWipeBytes securely wipes bytes using the global secure storage
func SecureWipeBytes(data []byte) {
	if globalSecureStorage != nil {
		globalSecureStorage.SecureWipeBytes(data)
	} else {
		// Fallback implementation
		for i := range data {
			data[i] = 0
		}
		runtime.GC()
	}
}

// IsDataEncrypted checks if a string appears to be encrypted data
func IsDataEncrypted(data string) bool {
	// Check for version prefix pattern
	return strings.HasPrefix(data, "v1:") && len(data) > 10
}