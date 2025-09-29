package validator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	assert.NotNil(t, validator)
	assert.Empty(t, validator.envValidations)
	assert.Empty(t, validator.errors)
}

func TestValidator_AddEnvValidation(t *testing.T) {
	validator := NewValidator()
	rule := &RequiredRule{}

	result := validator.AddEnvValidation("TEST_ENV", rule)

	assert.Same(t, validator, result) // Should return self for chaining
	assert.Contains(t, validator.envValidations, "TEST_ENV")
	assert.Equal(t, []ValidationRule{rule}, validator.envValidations["TEST_ENV"])
}

func TestValidator_ValidateEnvVars(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		rules       map[string][]ValidationRule
		expectError bool
		errorCount  int
	}{
		{
			name:    "No validations - should pass",
			envVars: map[string]string{},
			rules:   map[string][]ValidationRule{},
		},
		{
			name:    "Required env var present - should pass",
			envVars: map[string]string{"TEST_VAR": "value"},
			rules:   map[string][]ValidationRule{"TEST_VAR": {&RequiredRule{}}},
		},
		{
			name:        "Required env var missing - should fail",
			envVars:     map[string]string{},
			rules:       map[string][]ValidationRule{"TEST_VAR": {&RequiredRule{}}},
			expectError: true,
			errorCount:  1,
		},
		{
			name:    "Multiple valid env vars - should pass",
			envVars: map[string]string{"VAR1": "value1", "VAR2": "123"},
			rules: map[string][]ValidationRule{
				"VAR1": {&RequiredRule{}},
				"VAR2": {&RequiredRule{}, &MinRule{Min: 100}},
			},
		},
		{
			name:        "Multiple env vars with errors - should fail",
			envVars:     map[string]string{"VAR1": "", "VAR2": "50"},
			rules: map[string][]ValidationRule{
				"VAR1": {&RequiredRule{}},
				"VAR2": {&RequiredRule{}, &MinRule{Min: 100}},
			},
			expectError: true,
			errorCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
				defer os.Unsetenv(key)
			}

			validator := NewValidator()
			for envName, rules := range tt.rules {
				validator.AddEnvValidation(envName, rules...)
			}

			err := validator.ValidateEnvVars()

			if tt.expectError {
				require.Error(t, err)
				validationErrors, ok := err.(ValidationErrors)
				require.True(t, ok)
				assert.Len(t, validationErrors, tt.errorCount)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateStruct(t *testing.T) {
	type TestStruct struct {
		RequiredField string `validate:"required"`
		MinValue      int    `validate:"min=10"`
		MaxValue      int    `validate:"max=100"`
		RangeValue    int    `validate:"range=1-50"`
		EmailField    string `validate:"email"`
		URLField      string `validate:"url"`
		OneOfField    string `validate:"oneof=red green blue"`
		RegexField    string `validate:"regex=^[A-Z]+$"`
	}

	tests := []struct {
		name        string
		input       TestStruct
		expectError bool
		errorCount  int
	}{
		{
			name: "Valid struct - should pass",
			input: TestStruct{
				RequiredField: "present",
				MinValue:      15,
				MaxValue:      50,
				RangeValue:    25,
				EmailField:    "test@example.com",
				URLField:      "https://example.com",
				OneOfField:    "red",
				RegexField:    "HELLO",
			},
		},
		{
			name: "Multiple validation errors - should fail",
			input: TestStruct{
				RequiredField: "",        // Missing required
				MinValue:      5,         // Below minimum
				MaxValue:      150,       // Above maximum
				RangeValue:    100,       // Outside range
				EmailField:    "invalid", // Invalid email
				URLField:      "://invalid-url", // Invalid URL - make it clearly invalid
				OneOfField:    "yellow",  // Not in allowed values
				RegexField:    "hello",   // Doesn't match regex
			},
			expectError: true,
			errorCount:  8,
		},
		{
			name: "Empty optional fields - should pass",
			input: TestStruct{
				RequiredField: "present",
				MinValue:      15,
				MaxValue:      50,
				RangeValue:    25,
				// Other fields left empty - should be OK since not required
			},
		},
	}

	validator := NewValidator()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateStruct(tt.input)

			if tt.expectError {
				require.Error(t, err)
				validationErrors, ok := err.(ValidationErrors)
				require.True(t, ok)
				assert.Len(t, validationErrors, tt.errorCount)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_ValidateStruct_NonStruct(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateStruct("not a struct")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected struct, got string")
}

func TestValidator_ValidateStruct_Pointer(t *testing.T) {
	type TestStruct struct {
		Field string `validate:"required"`
	}

	validator := NewValidator()
	s := &TestStruct{Field: "value"}

	err := validator.ValidateStruct(s)
	assert.NoError(t, err)
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{
		Field:   "testField",
		Value:   "testValue",
		Rule:    "required",
		Message: "field is required",
	}

	expected := "validation failed for field 'testField': field is required (value: testValue)"
	assert.Equal(t, expected, err.Error())
}

func TestValidationErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   ValidationErrors
		expected string
	}{
		{
			name:     "Empty errors",
			errors:   ValidationErrors{},
			expected: "",
		},
		{
			name: "Single error",
			errors: ValidationErrors{
				{Field: "field1", Value: "value1", Message: "error1"},
			},
			expected: "validation failed for field 'field1': error1 (value: value1)",
		},
		{
			name: "Multiple errors",
			errors: ValidationErrors{
				{Field: "field1", Value: "value1", Message: "error1"},
				{Field: "field2", Value: "value2", Message: "error2"},
			},
			expected: "validation failed for field 'field1': error1 (value: value1); validation failed for field 'field2': error2 (value: value2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.errors.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidationErrors_HasErrors(t *testing.T) {
	empty := ValidationErrors{}
	assert.False(t, empty.HasErrors())

	withErrors := ValidationErrors{{Field: "test", Message: "error"}}
	assert.True(t, withErrors.HasErrors())
}

func TestValidator_GetErrors(t *testing.T) {
	validator := NewValidator()
	validator.errors = ValidationErrors{{Field: "test", Message: "error"}}

	errors := validator.GetErrors()
	assert.Len(t, errors, 1)
	assert.Equal(t, "test", errors[0].Field)
}

func TestValidator_ClearErrors(t *testing.T) {
	validator := NewValidator()
	validator.errors = ValidationErrors{{Field: "test", Message: "error"}}

	validator.ClearErrors()
	assert.Empty(t, validator.errors)
}