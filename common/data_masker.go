package common

import (
	"fmt"
	"regexp"
	"strings"
)

// DataMasker defines the interface for sensitive data masking operations
type DataMasker interface {
	// Core masking operations
	MaskAPIKey(key string) string
	MaskToken(token string) string
	MaskEmail(email string) string
	MaskPhoneNumber(phone string) string
	MaskURL(url string) string
	MaskIPAddress(ip string) string

	// JSON and structured data masking
	MaskJSON(data interface{}) interface{}
	MaskMap(data map[string]interface{}) map[string]interface{}
	MaskSlice(data []interface{}) []interface{}

	// String processing
	MaskString(text string) string
	MaskLogMessage(message string) string

	// Custom field masking
	AddSensitiveField(fieldName string)
	RemoveSensitiveField(fieldName string)
	IsSensitiveField(fieldName string) bool
}

// StandardDataMasker implements DataMasker with configurable masking rules
type StandardDataMasker struct {
	// Sensitive field patterns (case-insensitive)
	sensitiveFields map[string]bool

	// Compiled regex patterns for performance
	apiKeyPattern    *regexp.Regexp
	tokenPattern     *regexp.Regexp
	emailPattern     *regexp.Regexp
	phonePattern     *regexp.Regexp
	urlPattern       *regexp.Regexp
	ipPattern        *regexp.Regexp
	creditCardPattern *regexp.Regexp
}

// DataMaskerConfig holds configuration for the data masker
type DataMaskerConfig struct {
	// Masking behavior
	MaskingCharacter     string   // Character to use for masking (default: "*")
	PreserveLength       bool     // Whether to preserve original length
	ShowPrefixLength     int      // Number of prefix characters to show
	ShowSuffixLength     int      // Number of suffix characters to show

	// Additional sensitive fields
	CustomSensitiveFields []string // Custom field names to mask

	// Masking levels
	AggressiveMasking    bool     // Enable aggressive pattern matching
	MaskInternalIPs      bool     // Mask internal IP addresses
}

// DefaultDataMaskerConfig returns sensible default configuration
func DefaultDataMaskerConfig() *DataMaskerConfig {
	return &DataMaskerConfig{
		MaskingCharacter:     "*",
		PreserveLength:       false, // For security, don't preserve length
		ShowPrefixLength:     2,     // Show first 2 characters
		ShowSuffixLength:     4,     // Show last 4 characters
		CustomSensitiveFields: []string{},
		AggressiveMasking:    true,
		MaskInternalIPs:      false,
	}
}

// NewStandardDataMasker creates a new data masker with the given configuration
func NewStandardDataMasker(config *DataMaskerConfig) *StandardDataMasker {
	if config == nil {
		config = DefaultDataMaskerConfig()
	}

	masker := &StandardDataMasker{
		sensitiveFields: make(map[string]bool),
	}

	// Initialize default sensitive fields
	defaultFields := []string{
		"key", "token", "password", "secret", "auth", "authorization",
		"api_key", "apikey", "access_token", "refresh_token", "bearer",
		"credential", "credentials", "private_key", "private", "cert",
		"certificate", "signature", "hash", "session", "cookie",
		"x-api-key", "x-auth-token", "x-secret", "authorization",
	}

	for _, field := range defaultFields {
		masker.sensitiveFields[strings.ToLower(field)] = true
	}

	// Add custom sensitive fields
	for _, field := range config.CustomSensitiveFields {
		masker.sensitiveFields[strings.ToLower(field)] = true
	}

	// Compile regex patterns
	masker.compilePatterns()

	return masker
}

// compilePatterns compiles all regex patterns used for masking
func (m *StandardDataMasker) compilePatterns() {
	// API Key patterns (OpenAI, Claude, etc.)
	m.apiKeyPattern = regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9]{20,}|api[_-]?key[_-]?[a-zA-Z0-9]{10,}|bearer\s+[a-zA-Z0-9]{20,})`)

	// Token patterns
	m.tokenPattern = regexp.MustCompile(`(?i)(token[_-]?[a-zA-Z0-9]{10,}|[a-zA-Z0-9]{20,}\.[a-zA-Z0-9]{10,}\.[a-zA-Z0-9-_]{10,})`)

	// Email pattern
	m.emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

	// Phone pattern (international format)
	m.phonePattern = regexp.MustCompile(`(\+?[1-9]\d{1,14}|\(\d{3}\)\s?\d{3}-?\d{4})`)

	// URL pattern with credentials
	m.urlPattern = regexp.MustCompile(`(https?://[^:\s]+:[^@\s]+@[^\s]+)`)

	// IP address pattern
	m.ipPattern = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)

	// Credit card pattern
	m.creditCardPattern = regexp.MustCompile(`\b(?:\d{4}[\s-]?){3}\d{4}\b`)
}

// MaskAPIKey masks API keys while preserving recognizable format
func (m *StandardDataMasker) MaskAPIKey(key string) string {
	if key == "" {
		return ""
	}

	// Handle different API key formats
	if strings.HasPrefix(key, "sk-") {
		// OpenAI format: sk-1234567890abcdef -> sk-****cdef
		if len(key) > 8 {
			return key[:3] + strings.Repeat("*", 4) + key[len(key)-4:]
		}
		return key[:3] + strings.Repeat("*", len(key)-3)
	}

	// Generic format: show first 2 and last 4 characters
	if len(key) > 10 {
		return key[:2] + strings.Repeat("*", 6) + key[len(key)-4:]
	} else if len(key) > 6 {
		return key[:2] + strings.Repeat("*", len(key)-4) + key[len(key)-2:]
	}

	return strings.Repeat("*", len(key))
}

// MaskToken masks tokens while preserving format
func (m *StandardDataMasker) MaskToken(token string) string {
	if token == "" {
		return ""
	}

	// JWT format: eyJ...header.eyJ...payload.signature -> eyJ*****.eyJ*****.****
	if strings.Count(token, ".") == 2 {
		parts := strings.Split(token, ".")
		if len(parts) == 3 {
			header := m.maskPart(parts[0], 3, 0)
			payload := m.maskPart(parts[1], 3, 0)
			signature := strings.Repeat("*", 4)
			return fmt.Sprintf("%s.%s.%s", header, payload, signature)
		}
	}

	// Generic token masking
	if len(token) > 16 {
		return token[:4] + strings.Repeat("*", 8) + token[len(token)-4:]
	} else if len(token) > 8 {
		return token[:2] + strings.Repeat("*", len(token)-4) + token[len(token)-2:]
	}

	return strings.Repeat("*", len(token))
}

// MaskEmail masks email addresses
func (m *StandardDataMasker) MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email // Not a valid email format
	}

	username := parts[0]
	domain := parts[1]

	// Mask username
	maskedUsername := m.maskPart(username, 1, 0)

	// Mask domain but preserve structure
	domainParts := strings.Split(domain, ".")
	if len(domainParts) >= 2 {
		// Mask domain name but preserve TLD
		maskedDomain := m.maskPart(domainParts[0], 1, 0)
		for i := 1; i < len(domainParts); i++ {
			maskedDomain += "." + domainParts[i]
		}
		domain = maskedDomain
	}

	return maskedUsername + "@" + domain
}

// MaskPhoneNumber masks phone numbers
func (m *StandardDataMasker) MaskPhoneNumber(phone string) string {
	if phone == "" {
		return ""
	}

	// Remove non-digits for processing
	digits := regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")

	if len(digits) >= 10 {
		// Preserve country code and last 4 digits
		if strings.HasPrefix(digits, "+") {
			return digits[:3] + strings.Repeat("*", len(digits)-7) + digits[len(digits)-4:]
		} else {
			return digits[:3] + strings.Repeat("*", len(digits)-7) + digits[len(digits)-4:]
		}
	}

	return strings.Repeat("*", len(phone))
}

// MaskURL masks URLs containing credentials
func (m *StandardDataMasker) MaskURL(url string) string {
	if url == "" {
		return ""
	}

	// Pattern: https://username:password@domain.com/path -> https://****:****@domain.com/path
	if strings.Contains(url, "@") && (strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		parts := strings.SplitN(url, "://", 2)
		if len(parts) == 2 {
			scheme := parts[0]
			rest := parts[1]

			if strings.Contains(rest, "@") {
				authAndHost := strings.SplitN(rest, "@", 2)
				if len(authAndHost) == 2 {
					// Mask credentials
					return scheme + "://****:****@" + authAndHost[1]
				}
			}
		}
	}

	return url
}

// MaskIPAddress masks IP addresses
func (m *StandardDataMasker) MaskIPAddress(ip string) string {
	if ip == "" {
		return ""
	}

	// IPv4: 192.168.1.100 -> 192.168.*.*
	if match := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)\.(\d+)$`).FindStringSubmatch(ip); len(match) == 5 {
		return match[1] + "." + match[2] + ".*.*"
	}

	// IPv6: abbreviated masking
	if strings.Contains(ip, ":") {
		parts := strings.Split(ip, ":")
		if len(parts) > 4 {
			return strings.Join(parts[:2], ":") + ":****:****"
		}
	}

	return ip
}

// MaskJSON recursively masks sensitive fields in JSON data
func (m *StandardDataMasker) MaskJSON(data interface{}) interface{} {
	if data == nil {
		return nil
	}

	switch v := data.(type) {
	case map[string]interface{}:
		return m.MaskMap(v)
	case []interface{}:
		return m.MaskSlice(v)
	case string:
		return m.MaskString(v)
	default:
		return data
	}
}

// MaskMap masks sensitive fields in a map
func (m *StandardDataMasker) MaskMap(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := make(map[string]interface{})

	for key, value := range data {
		if m.IsSensitiveField(key) {
			// Mask sensitive field
			if str, ok := value.(string); ok {
				result[key] = m.MaskString(str)
			} else {
				result[key] = "****"
			}
		} else {
			// Recursively process nested structures
			result[key] = m.MaskJSON(value)
		}
	}

	return result
}

// MaskSlice masks sensitive data in a slice
func (m *StandardDataMasker) MaskSlice(data []interface{}) []interface{} {
	if data == nil {
		return nil
	}

	result := make([]interface{}, len(data))
	for i, item := range data {
		result[i] = m.MaskJSON(item)
	}

	return result
}

// MaskString applies pattern-based masking to a string
func (m *StandardDataMasker) MaskString(text string) string {
	if text == "" {
		return ""
	}

	// Apply API key masking
	text = m.apiKeyPattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskAPIKey(match)
	})

	// Apply token masking
	text = m.tokenPattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskToken(match)
	})

	// Apply email masking
	text = m.emailPattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskEmail(match)
	})

	// Apply phone masking
	text = m.phonePattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskPhoneNumber(match)
	})

	// Apply URL masking
	text = m.urlPattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskURL(match)
	})

	// Apply IP masking
	text = m.ipPattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.MaskIPAddress(match)
	})

	// Apply credit card masking
	text = m.creditCardPattern.ReplaceAllStringFunc(text, func(match string) string {
		return m.maskPart(match, 4, 4)
	})

	return text
}

// MaskLogMessage masks sensitive data in log messages
func (m *StandardDataMasker) MaskLogMessage(message string) string {
	return m.MaskString(message)
}

// AddSensitiveField adds a field name to the sensitive fields list
func (m *StandardDataMasker) AddSensitiveField(fieldName string) {
	m.sensitiveFields[strings.ToLower(fieldName)] = true
}

// RemoveSensitiveField removes a field name from the sensitive fields list
func (m *StandardDataMasker) RemoveSensitiveField(fieldName string) {
	delete(m.sensitiveFields, strings.ToLower(fieldName))
}

// IsSensitiveField checks if a field name is considered sensitive
func (m *StandardDataMasker) IsSensitiveField(fieldName string) bool {
	return m.sensitiveFields[strings.ToLower(fieldName)]
}

// Helper method to mask part of a string
func (m *StandardDataMasker) maskPart(text string, prefixLen, suffixLen int) string {
	if len(text) <= prefixLen+suffixLen {
		return strings.Repeat("*", len(text))
	}

	prefix := text[:prefixLen]
	suffix := text[len(text)-suffixLen:]
	maskLen := len(text) - prefixLen - suffixLen

	if maskLen < 4 {
		maskLen = 4 // Minimum mask length for security
	}

	return prefix + strings.Repeat("*", maskLen) + suffix
}

// Global data masker instance
var globalDataMasker DataMasker

// InitializeDataMasker initializes the global data masker instance
func InitializeDataMasker(config *DataMaskerConfig) {
	globalDataMasker = NewStandardDataMasker(config)
	SysLog("Data masking system initialized successfully")
}

// GetDataMasker returns the global data masker instance
func GetDataMasker() DataMasker {
	return globalDataMasker
}

// IsDataMaskingEnabled returns whether data masking is available
func IsDataMaskingEnabled() bool {
	return globalDataMasker != nil
}

// Convenience functions for global data masker

// MaskAPIKeyGlobal masks an API key using the global masker
func MaskAPIKeyGlobal(key string) string {
	if globalDataMasker == nil {
		// Fallback masking if masker not initialized
		if len(key) > 8 {
			return key[:4] + strings.Repeat("*", 4) + key[len(key)-4:]
		}
		return strings.Repeat("*", len(key))
	}
	return globalDataMasker.MaskAPIKey(key)
}

// MaskTokenGlobal masks a token using the global masker
func MaskTokenGlobal(token string) string {
	if globalDataMasker == nil {
		// Fallback masking
		if len(token) > 8 {
			return token[:4] + strings.Repeat("*", 4) + token[len(token)-4:]
		}
		return strings.Repeat("*", len(token))
	}
	return globalDataMasker.MaskToken(token)
}

// MaskEmailGlobal masks an email using the global masker
func MaskEmailGlobal(email string) string {
	if globalDataMasker == nil {
		// Fallback masking
		parts := strings.Split(email, "@")
		if len(parts) == 2 {
			return "***@" + parts[1]
		}
		return email
	}
	return globalDataMasker.MaskEmail(email)
}

// MaskJSONGlobal masks sensitive fields in JSON data
func MaskJSONGlobal(data interface{}) interface{} {
	if globalDataMasker == nil {
		return data
	}
	return globalDataMasker.MaskJSON(data)
}

// MaskLogMessageGlobal masks sensitive data in log messages
func MaskLogMessageGlobal(message string) string {
	if globalDataMasker == nil {
		return message
	}
	return globalDataMasker.MaskLogMessage(message)
}

// MaskForLogging masks data specifically for logging purposes
func MaskForLogging(data interface{}) interface{} {
	if globalDataMasker == nil {
		// Basic fallback: mask anything that looks like sensitive data
		if str, ok := data.(string); ok {
			// Simple pattern matching fallback
			if strings.HasPrefix(str, "sk-") || strings.HasPrefix(str, "Bearer ") {
				return MaskAPIKeyGlobal(str)
			}
		}
		return data
	}

	return globalDataMasker.MaskJSON(data)
}

// DetectSensitiveData detects if a string contains sensitive information
func DetectSensitiveData(text string) bool {
	if text == "" {
		return false
	}

	// Check for common sensitive patterns
	sensitivePatterns := []string{
		`sk-[a-zA-Z0-9]{10,}`,           // OpenAI API keys (at least 10 chars after sk-)
		`Bearer\s+[a-zA-Z0-9]{10,}`,     // Bearer tokens
		`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, // Email addresses
		`password`,                       // Password field
		`secret`,                         // Secret field
		`token`,                          // Token field
	}

	for _, pattern := range sensitivePatterns {
		if matched, _ := regexp.MatchString(`(?i)`+pattern, text); matched {
			return true
		}
	}

	return false
}