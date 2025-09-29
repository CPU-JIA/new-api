package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SecureLogger defines the interface for secure logging operations
type SecureLogger interface {
	// Core logging with automatic masking
	LogWithMasking(level string, message string, fields map[string]interface{})
	LogInfo(message string, fields map[string]interface{})
	LogWarn(message string, fields map[string]interface{})
	LogError(message string, err error, fields map[string]interface{})
	LogDebug(message string, fields map[string]interface{})

	// API operation logging
	LogAPICall(request, response interface{}, sensitiveFields []string)
	LogChannelOperation(operation string, channelID int, details map[string]interface{})
	LogTokenOperation(operation string, userID int, details map[string]interface{})

	// Security event logging
	LogSecurityEvent(event string, details map[string]interface{})
	LogAuthEvent(event string, userID int, details map[string]interface{})
	LogDataAccess(resource string, userID int, details map[string]interface{})

	// Structured logging
	LogStructured(entry LogEntry)

	// Configuration and lifecycle
	SetMaskingEnabled(enabled bool)
	IsMaskingEnabled() bool
	Flush() error
}

// LogLevel represents different log levels
type LogLevel string

const (
	LogLevelDebug    LogLevel = "DEBUG"
	LogLevelInfo     LogLevel = "INFO"
	LogLevelWarn     LogLevel = "WARN"
	LogLevelError    LogLevel = "ERROR"
	LogLevelSecurity LogLevel = "SECURITY"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp    time.Time              `json:"timestamp"`
	Level        LogLevel               `json:"level"`
	Message      string                 `json:"message"`
	Fields       map[string]interface{} `json:"fields,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Component    string                 `json:"component,omitempty"`
	Operation    string                 `json:"operation,omitempty"`
	UserID       int                    `json:"user_id,omitempty"`
	SessionID    string                 `json:"session_id,omitempty"`
	RequestID    string                 `json:"request_id,omitempty"`
	Duration     time.Duration          `json:"duration,omitempty"`
	Masked       bool                   `json:"masked,omitempty"`
}

// SecureLoggerConfig holds configuration for the secure logger
type SecureLoggerConfig struct {
	// Masking configuration
	EnableMasking        bool     // Enable automatic sensitive data masking
	MaskingLevel         int      // 0=minimal, 1=standard, 2=aggressive

	// Output configuration
	EnableFileOutput     bool     // Enable file-based logging
	LogDirectory         string   // Directory for log files
	LogFilePrefix        string   // Prefix for log files
	MaxLogFileSize       int64    // Maximum log file size in bytes
	MaxLogFiles          int      // Maximum number of log files to keep

	// Rotation configuration
	RotateDaily          bool     // Rotate logs daily
	CompressOldLogs      bool     // Compress rotated log files

	// Security configuration
	LogSecurityEvents    bool     // Log security-related events
	LogDataAccess        bool     // Log sensitive data access
	EnableAuditMode      bool     // Enable audit mode (more detailed logging)

	// Performance configuration
	AsyncLogging         bool     // Enable asynchronous logging
	BufferSize           int      // Buffer size for async logging
	FlushInterval        time.Duration // Interval to flush buffers
}

// DefaultSecureLoggerConfig returns secure default configuration
func DefaultSecureLoggerConfig() *SecureLoggerConfig {
	return &SecureLoggerConfig{
		EnableMasking:        true,
		MaskingLevel:         1, // Standard masking
		EnableFileOutput:     true,
		LogDirectory:         "./logs",
		LogFilePrefix:        "oneapi",
		MaxLogFileSize:       100 * 1024 * 1024, // 100MB
		MaxLogFiles:          10,
		RotateDaily:          true,
		CompressOldLogs:      true,
		LogSecurityEvents:    true,
		LogDataAccess:        true,
		EnableAuditMode:      false,
		AsyncLogging:         true,
		BufferSize:           1000,
		FlushInterval:        5 * time.Second,
	}
}

// StandardSecureLogger implements SecureLogger with masking and structured output
type StandardSecureLogger struct {
	config       *SecureLoggerConfig
	masker       DataMasker
	mutex        sync.RWMutex

	// File output
	currentLogFile *os.File
	currentLogPath string
	logFileSize    int64

	// Async logging
	logChannel     chan LogEntry
	stopChannel    chan struct{}
	flushChannel   chan struct{}
	wg             sync.WaitGroup
}

// NewStandardSecureLogger creates a new secure logger with the given configuration
func NewStandardSecureLogger(config *SecureLoggerConfig) (*StandardSecureLogger, error) {
	if config == nil {
		config = DefaultSecureLoggerConfig()
	}

	logger := &StandardSecureLogger{
		config:       config,
		masker:       GetDataMasker(),
		stopChannel:  make(chan struct{}),
		flushChannel: make(chan struct{}),
	}

	// Initialize file output if enabled
	if config.EnableFileOutput {
		if err := logger.initializeFileOutput(); err != nil {
			return nil, fmt.Errorf("failed to initialize file output: %w", err)
		}
	}

	// Start async logging if enabled
	if config.AsyncLogging {
		logger.logChannel = make(chan LogEntry, config.BufferSize)
		logger.startAsyncLogging()
	}

	return logger, nil
}

// initializeFileOutput sets up file-based logging
func (l *StandardSecureLogger) initializeFileOutput() error {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(l.config.LogDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open current log file
	return l.rotateLogFile()
}

// rotateLogFile creates a new log file or rotates existing one
func (l *StandardSecureLogger) rotateLogFile() error {
	// Close existing file if open
	if l.currentLogFile != nil {
		l.currentLogFile.Close()
	}

	// Generate new log file name
	timestamp := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.log", l.config.LogFilePrefix, timestamp)
	l.currentLogPath = filepath.Join(l.config.LogDirectory, filename)

	// Open new log file
	file, err := os.OpenFile(l.currentLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.currentLogFile = file
	l.logFileSize = 0

	// Get current file size if file already exists
	if stat, err := file.Stat(); err == nil {
		l.logFileSize = stat.Size()
	}

	return nil
}

// startAsyncLogging starts the async logging goroutine
func (l *StandardSecureLogger) startAsyncLogging() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		ticker := time.NewTicker(l.config.FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case entry := <-l.logChannel:
				l.writeLogEntry(entry)

			case <-ticker.C:
				l.flushLogs()

			case <-l.flushChannel:
				l.flushLogs()

			case <-l.stopChannel:
				// Drain remaining entries
				for len(l.logChannel) > 0 {
					entry := <-l.logChannel
					l.writeLogEntry(entry)
				}
				l.flushLogs()
				return
			}
		}
	}()
}

// LogWithMasking logs a message with automatic sensitive data masking
func (l *StandardSecureLogger) LogWithMasking(level string, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevel(level),
		Message:   message,
		Fields:    fields,
		Component: "system",
		Masked:    l.config.EnableMasking,
	}

	if l.config.EnableMasking && l.masker != nil {
		// Mask message
		entry.Message = l.masker.MaskLogMessage(entry.Message)

		// Mask fields
		if entry.Fields != nil {
			entry.Fields = l.masker.MaskMap(entry.Fields)
		}
	}

	l.logEntry(entry)
}

// LogInfo logs an info message
func (l *StandardSecureLogger) LogInfo(message string, fields map[string]interface{}) {
	l.LogWithMasking(string(LogLevelInfo), message, fields)
}

// LogWarn logs a warning message
func (l *StandardSecureLogger) LogWarn(message string, fields map[string]interface{}) {
	l.LogWithMasking(string(LogLevelWarn), message, fields)
}

// LogError logs an error message
func (l *StandardSecureLogger) LogError(message string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}

	if err != nil {
		fields["error"] = err.Error()
	}

	l.LogWithMasking(string(LogLevelError), message, fields)
}

// LogDebug logs a debug message
func (l *StandardSecureLogger) LogDebug(message string, fields map[string]interface{}) {
	if DebugEnabled {
		l.LogWithMasking(string(LogLevelDebug), message, fields)
	}
}

// LogAPICall logs API call details with automatic masking
func (l *StandardSecureLogger) LogAPICall(request, response interface{}, sensitiveFields []string) {
	if !l.config.EnableAuditMode {
		return
	}

	fields := map[string]interface{}{
		"request":  request,
		"response": response,
	}

	// Apply extra masking for specified sensitive fields
	if l.masker != nil && len(sensitiveFields) > 0 {
		for _, field := range sensitiveFields {
			l.masker.AddSensitiveField(field)
		}
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Message:   "API call executed",
		Fields:    fields,
		Component: "api",
		Operation: "api_call",
		Masked:    l.config.EnableMasking,
	}

	l.logEntry(entry)
}

// LogChannelOperation logs channel-related operations
func (l *StandardSecureLogger) LogChannelOperation(operation string, channelID int, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["channel_id"] = channelID

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Message:   fmt.Sprintf("Channel operation: %s", operation),
		Fields:    details,
		Component: "channel",
		Operation: operation,
		Masked:    l.config.EnableMasking,
	}

	l.logEntry(entry)
}

// LogTokenOperation logs token-related operations
func (l *StandardSecureLogger) LogTokenOperation(operation string, userID int, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Message:   fmt.Sprintf("Token operation: %s", operation),
		Fields:    details,
		Component: "token",
		Operation: operation,
		UserID:    userID,
		Masked:    l.config.EnableMasking,
	}

	l.logEntry(entry)
}

// LogSecurityEvent logs security-related events
func (l *StandardSecureLogger) LogSecurityEvent(event string, details map[string]interface{}) {
	if !l.config.LogSecurityEvents {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelSecurity,
		Message:   fmt.Sprintf("Security event: %s", event),
		Fields:    details,
		Component: "security",
		Operation: event,
		Masked:    l.config.EnableMasking,
	}

	l.logEntry(entry)

	// Also log to standard system log for immediate visibility
	SysLog(fmt.Sprintf("[SECURITY] %s: %v", event, details))
}

// LogAuthEvent logs authentication-related events
func (l *StandardSecureLogger) LogAuthEvent(event string, userID int, details map[string]interface{}) {
	if details == nil {
		details = make(map[string]interface{})
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelSecurity,
		Message:   fmt.Sprintf("Auth event: %s", event),
		Fields:    details,
		Component: "auth",
		Operation: event,
		UserID:    userID,
		Masked:    l.config.EnableMasking,
	}

	l.logEntry(entry)
}

// LogDataAccess logs sensitive data access
func (l *StandardSecureLogger) LogDataAccess(resource string, userID int, details map[string]interface{}) {
	if !l.config.LogDataAccess {
		return
	}

	if details == nil {
		details = make(map[string]interface{})
	}
	details["resource"] = resource

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     LogLevelInfo,
		Message:   fmt.Sprintf("Data access: %s", resource),
		Fields:    details,
		Component: "data",
		Operation: "access",
		UserID:    userID,
		Masked:    l.config.EnableMasking,
	}

	l.logEntry(entry)
}

// LogStructured logs a structured log entry
func (l *StandardSecureLogger) LogStructured(entry LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	entry.Masked = l.config.EnableMasking

	l.logEntry(entry)
}

// logEntry processes and outputs a log entry
func (l *StandardSecureLogger) logEntry(entry LogEntry) {
	if l.config.AsyncLogging && l.logChannel != nil {
		// Send to async channel (non-blocking)
		select {
		case l.logChannel <- entry:
		default:
			// Channel full, log synchronously as fallback
			l.writeLogEntry(entry)
		}
	} else {
		// Log synchronously
		l.writeLogEntry(entry)
	}
}

// writeLogEntry writes a log entry to all configured outputs
func (l *StandardSecureLogger) writeLogEntry(entry LogEntry) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Apply masking if enabled
	if l.config.EnableMasking && l.masker != nil && entry.Fields != nil {
		entry.Fields = l.masker.MaskMap(entry.Fields)
		entry.Message = l.masker.MaskLogMessage(entry.Message)
	}

	// Convert to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple text format
		fmt.Fprintf(gin.DefaultErrorWriter, "[LOG ERROR] Failed to marshal log entry: %v\n", err)
		return
	}

	// Write to console (always)
	fmt.Fprintf(gin.DefaultWriter, "%s\n", string(jsonData))

	// Write to file if enabled
	if l.config.EnableFileOutput && l.currentLogFile != nil {
		if _, err := l.currentLogFile.WriteString(string(jsonData) + "\n"); err == nil {
			l.logFileSize += int64(len(jsonData) + 1)

			// Check if rotation is needed
			if l.logFileSize > l.config.MaxLogFileSize || (l.config.RotateDaily && l.shouldRotateDaily()) {
				l.rotateLogFile()
			}
		}
	}
}

// shouldRotateDaily checks if daily rotation is needed
func (l *StandardSecureLogger) shouldRotateDaily() bool {
	if !l.config.RotateDaily {
		return false
	}

	// Check if current log file is from today
	today := time.Now().Format("2006-01-02")
	expectedFilename := fmt.Sprintf("%s_%s.log", l.config.LogFilePrefix, today)
	currentFilename := filepath.Base(l.currentLogPath)

	return currentFilename != expectedFilename
}

// flushLogs flushes any buffered log data
func (l *StandardSecureLogger) flushLogs() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.currentLogFile != nil {
		l.currentLogFile.Sync()
	}
}

// SetMaskingEnabled enables or disables masking
func (l *StandardSecureLogger) SetMaskingEnabled(enabled bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	l.config.EnableMasking = enabled
}

// IsMaskingEnabled returns whether masking is enabled
func (l *StandardSecureLogger) IsMaskingEnabled() bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	return l.config.EnableMasking
}

// Flush flushes all pending log entries
func (l *StandardSecureLogger) Flush() error {
	if l.config.AsyncLogging {
		select {
		case l.flushChannel <- struct{}{}:
		default:
		}
	}

	l.flushLogs()
	return nil
}

// Close gracefully shuts down the logger
func (l *StandardSecureLogger) Close() error {
	if l.config.AsyncLogging {
		close(l.stopChannel)
		l.wg.Wait()
	}

	if l.currentLogFile != nil {
		l.currentLogFile.Close()
	}

	return nil
}

// Global secure logger instance
var globalSecureLogger SecureLogger

// InitializeSecureLogger initializes the global secure logger instance
func InitializeSecureLogger(config *SecureLoggerConfig) error {
	logger, err := NewStandardSecureLogger(config)
	if err != nil {
		return fmt.Errorf("failed to initialize secure logger: %w", err)
	}

	globalSecureLogger = logger
	SysLog("Secure logging system initialized successfully")

	return nil
}

// GetSecureLogger returns the global secure logger instance
func GetSecureLogger() SecureLogger {
	return globalSecureLogger
}

// IsSecureLoggingEnabled returns whether secure logging is available
func IsSecureLoggingEnabled() bool {
	return globalSecureLogger != nil
}

// Convenience functions for global secure logger

// LogSecurityEventGlobal logs a security event using the global logger
func LogSecurityEventGlobal(event string, details map[string]interface{}) {
	if globalSecureLogger != nil {
		globalSecureLogger.LogSecurityEvent(event, details)
	} else {
		// Fallback to system log
		SysLog(fmt.Sprintf("[SECURITY] %s: %v", event, details))
	}
}

// LogChannelOperationGlobal logs a channel operation using the global logger
func LogChannelOperationGlobal(operation string, channelID int, details map[string]interface{}) {
	if globalSecureLogger != nil {
		globalSecureLogger.LogChannelOperation(operation, channelID, details)
	}
}

// LogTokenOperationGlobal logs a token operation using the global logger
func LogTokenOperationGlobal(operation string, userID int, details map[string]interface{}) {
	if globalSecureLogger != nil {
		globalSecureLogger.LogTokenOperation(operation, userID, details)
	}
}

// LogAPICallGlobal logs an API call using the global logger
func LogAPICallGlobal(request, response interface{}, sensitiveFields []string) {
	if globalSecureLogger != nil {
		globalSecureLogger.LogAPICall(request, response, sensitiveFields)
	}
}

// SysLogSecure logs to system log with automatic masking
func SysLogSecure(message string) {
	if globalDataMasker != nil {
		message = globalDataMasker.MaskLogMessage(message)
	}
	SysLog(message)
}

// Enhanced SysLog with masking - replacement for existing SysLog when dealing with sensitive data
func SysLogMasked(message string) {
	if globalDataMasker != nil {
		message = globalDataMasker.MaskLogMessage(message)
	}
	SysLog(message)
}