package validator

import (
	"fmt"
	"log"
	"os"
)

// ExampleUsage demonstrates how to use the configuration validator
func ExampleUsage() {
	// Create a new config validator with predefined rules
	validator := NewConfigValidator()

	// Example 1: Validate environment variables
	fmt.Println("=== Example 1: Environment Variable Validation ===")

	// Set some test environment variables
	os.Setenv("DATABASE_TYPE", "mysql")
	os.Setenv("PORT", "8080")
	os.Setenv("GIN_MODE", "release")
	os.Setenv("REDIS_CONN_STRING", "redis://localhost:6379")
	defer func() {
		os.Unsetenv("DATABASE_TYPE")
		os.Unsetenv("PORT")
		os.Unsetenv("GIN_MODE")
		os.Unsetenv("REDIS_CONN_STRING")
	}()

	if err := validator.ValidateEnvVars(); err != nil {
		fmt.Printf("Environment validation failed: %v\n", err)
	} else {
		fmt.Println("All environment variables validated successfully!")
	}

	// Example 2: Validate configuration structures
	fmt.Println("\n=== Example 2: Struct Validation ===")

	// Valid database configuration
	dbConfig := DatabaseConfig{
		Type:     "mysql",
		Host:     "localhost",
		Port:     3306,
		Username: "root",
		Password: "password",
		Database: "newapi",
		URL:      "mysql://root:password@localhost:3306/newapi",
	}

	if err := validator.ValidateDatabaseConfig(dbConfig); err != nil {
		fmt.Printf("Database config validation failed: %v\n", err)
	} else {
		fmt.Println("Database configuration is valid!")
	}

	// Invalid database configuration (for demonstration)
	invalidDbConfig := DatabaseConfig{
		Type: "invalid_db", // Invalid database type
		Port: 99999,        // Invalid port
		// Missing required fields
	}

	if err := validator.ValidateDatabaseConfig(invalidDbConfig); err != nil {
		fmt.Printf("Invalid database config (as expected): %v\n", err)
	}

	// Example 3: Custom validation rules
	fmt.Println("\n=== Example 3: Custom Validation Rules ===")

	customValidator := NewValidator()
	customValidator.AddEnvValidation("CUSTOM_PORT", &PortRule{})
	customValidator.AddEnvValidation("CUSTOM_EMAIL", &RequiredRule{}, &EmailRule{})

	os.Setenv("CUSTOM_PORT", "3000")
	os.Setenv("CUSTOM_EMAIL", "admin@example.com")
	defer func() {
		os.Unsetenv("CUSTOM_PORT")
		os.Unsetenv("CUSTOM_EMAIL")
	}()

	if err := customValidator.ValidateEnvVars(); err != nil {
		fmt.Printf("Custom validation failed: %v\n", err)
	} else {
		fmt.Println("Custom validation passed!")
	}

	// Example 4: Validate all configurations at once
	fmt.Println("\n=== Example 4: Comprehensive Validation ===")

	redisConfig := RedisConfig{
		Host:       "localhost",
		Port:       6379,
		Database:   0,
		MaxRetries: 3,
		PoolSize:   10,
		ConnString: "redis://localhost:6379",
	}

	logConfig := LogConfig{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	}

	// Validate all configurations together
	if err := validator.ValidateAllConfigs(dbConfig, redisConfig, logConfig); err != nil {
		fmt.Printf("Comprehensive validation failed: %v\n", err)
	} else {
		fmt.Println("All configurations are valid!")
	}

	// Example 5: Using the PrintValidationSummary method
	fmt.Println("\n=== Example 5: Validation Summary ===")
	validator.PrintValidationSummary()
}

// ExampleIntegrationUsage shows how to integrate validator in main application
func ExampleIntegrationUsage() {
	// This would typically be called in your main.go or initialization code

	// 1. Check required environment variables first
	requiredEnvVars := []string{
		"DATABASE_URL",
		"REDIS_CONN_STRING",
		"JWT_SECRET",
	}

	if err := CheckRequiredEnvVars(requiredEnvVars); err != nil {
		log.Fatalf("Missing required environment variables: %v", err)
	}

	// 2. Create and configure validator
	validator := NewConfigValidator()

	// 3. Add custom validations for your specific needs
	validator.AddEnvValidation("JWT_SECRET", &RequiredRule{}, &MinRule{Min: 32})
	validator.AddEnvValidation("API_RATE_LIMIT", &RangeRule{Min: 1, Max: 10000})

	// 4. Validate all configurations
	if err := validator.ValidateCommonConfigs(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// 5. Continue with application initialization...
	log.Println("Configuration validation passed. Starting application...")
}

// ExampleCustomValidator shows how to create validators for custom config structs
func ExampleCustomValidator() {
	// Define your custom configuration struct
	type APIConfig struct {
		Timeout        int    `json:"timeout" validate:"required,range=1-300"`
		MaxConnections int    `json:"max_connections" validate:"required,min=1,max=1000"`
		EnableCORS     bool   `json:"enable_cors"`
		AllowedOrigins string `json:"allowed_origins" validate:"required"`
		LogLevel       string `json:"log_level" validate:"required,oneof=debug info warn error"`
	}

	// Create validator
	validator := NewValidator()

	// Valid configuration
	validConfig := APIConfig{
		Timeout:        30,
		MaxConnections: 100,
		EnableCORS:     true,
		AllowedOrigins: "*",
		LogLevel:       "info",
	}

	if err := validator.ValidateStruct(validConfig); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
	} else {
		fmt.Println("API configuration is valid!")
	}

	// Invalid configuration
	invalidConfig := APIConfig{
		Timeout:        500, // Too high (max 300)
		MaxConnections: 0,   // Too low (min 1)
		LogLevel:       "verbose", // Not in allowed values
		// Missing required AllowedOrigins
	}

	if err := validator.ValidateStruct(invalidConfig); err != nil {
		fmt.Printf("Invalid configuration (as expected): %v\n", err)
	}
}