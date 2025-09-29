package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequiredRule(t *testing.T) {
	rule := &RequiredRule{}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"String value", "hello", false},
		{"Empty string", "", true},
		{"Zero int", 0, false}, // 0 is not considered empty for numbers
		{"Nil value", nil, true},
		{"Empty slice", []string{}, true},
		{"Non-empty slice", []string{"item"}, false},
		{"Empty map", map[string]string{}, true},
		{"Non-empty map", map[string]string{"key": "value"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMinRule(t *testing.T) {
	rule := &MinRule{Min: 10}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid int", 15, false},
		{"Valid string number", "20", false},
		{"Below minimum int", 5, true},
		{"Below minimum string", "3", true},
		{"Equal to minimum", 10, false},
		{"Invalid string", "not a number", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMaxRule(t *testing.T) {
	rule := &MaxRule{Max: 100}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid int", 50, false},
		{"Valid string number", "80", false},
		{"Above maximum int", 150, true},
		{"Above maximum string", "200", true},
		{"Equal to maximum", 100, false},
		{"Invalid string", "not a number", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRangeRule(t *testing.T) {
	rule := &RangeRule{Min: 10, Max: 100}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid int in range", 50, false},
		{"Valid string in range", "75", false},
		{"Below range", 5, true},
		{"Above range", 150, true},
		{"Equal to min", 10, false},
		{"Equal to max", 100, false},
		{"Invalid string", "not a number", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegexRule(t *testing.T) {
	rule := &RegexRule{Pattern: "^[A-Z]+$"}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Matching string", "HELLO", false},
		{"Non-matching string", "hello", true},
		{"Non-matching mixed case", "Hello", true},
		{"Empty string", "", false}, // Empty values allowed unless required
		{"Nil value", nil, false},   // Nil values allowed unless required
		{"Non-string value", 123, true}, // Numbers converted to string, "123" doesn't match pattern
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegexRule_InvalidPattern(t *testing.T) {
	rule := &RegexRule{Pattern: "[invalid"} // Invalid regex pattern

	err := rule.Validate("test", "testField")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex pattern")
}

func TestURLRule(t *testing.T) {
	rule := &URLRule{}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid HTTP URL", "http://example.com", false},
		{"Valid HTTPS URL", "https://example.com", false},
		{"Valid URL with path", "https://example.com/path", false},
		{"Valid URL with query", "https://example.com?query=value", false},
		{"Invalid URL - no scheme", "example.com", true}, // Now should fail due to missing scheme
		{"Invalid URL - malformed", "://invalid-url", true}, // This should fail
		{"Empty string", "", false}, // Empty values allowed unless required
		{"Nil value", nil, false},   // Nil values allowed unless required
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEmailRule(t *testing.T) {
	rule := &EmailRule{}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid email", "test@example.com", false},
		{"Valid email with subdomain", "user@mail.example.com", false},
		{"Valid email with numbers", "user123@example.com", false},
		{"Invalid email - no @", "testexample.com", true},
		{"Invalid email - no domain", "test@", true},
		{"Invalid email - no TLD", "test@example", true},
		{"Empty string", "", false}, // Empty values allowed unless required
		{"Nil value", nil, false},   // Nil values allowed unless required
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOneOfRule(t *testing.T) {
	rule := &OneOfRule{Values: []string{"red", "green", "blue"}}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid value - red", "red", false},
		{"Valid value - green", "green", false},
		{"Valid value - blue", "blue", false},
		{"Invalid value", "yellow", true},
		{"Empty string", "", false}, // Empty values allowed unless required
		{"Nil value", nil, false},   // Nil values allowed unless required
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPortRule(t *testing.T) {
	rule := &PortRule{}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid port", 8080, false},
		{"Valid port string", "3000", false},
		{"Valid min port", 1, false},
		{"Valid max port", 65535, false},
		{"Invalid port - too low", 0, true},
		{"Invalid port - too high", 65536, true},
		{"Invalid string", "not a number", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBoolRule(t *testing.T) {
	rule := &BoolRule{}

	tests := []struct {
		name        string
		value       interface{}
		expectError bool
	}{
		{"Valid bool true", true, false},
		{"Valid bool false", false, false},
		{"Valid string true", "true", false},
		{"Valid string false", "false", false},
		{"Valid string 1", "1", false},
		{"Valid string 0", "0", false},
		{"Invalid string", "maybe", true},
		{"Empty string", "", false}, // Empty values allowed unless required
		{"Nil value", nil, false},   // Nil values allowed unless required
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Validate(tt.value, "testField")
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_isEmpty(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"Nil", nil, true},
		{"Empty string", "", true},
		{"Non-empty string", "hello", false},
		{"Empty slice", []string{}, true},
		{"Non-empty slice", []string{"item"}, false},
		{"Empty map", map[string]string{}, true},
		{"Non-empty map", map[string]string{"key": "value"}, false},
		{"Zero int", 0, false}, // Numbers are never considered empty
		{"Non-zero int", 42, false},
		{"False bool", false, false}, // Bools are never considered empty
		{"True bool", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isEmpty(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_toString(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"Nil", nil, ""},
		{"String", "hello", "hello"},
		{"Int", 42, "42"},
		{"Float", 3.14, "3.14"},
		{"Bool true", true, "true"},
		{"Bool false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toString(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_toInt(t *testing.T) {
	tests := []struct {
		name        string
		value       interface{}
		expected    int
		expectError bool
	}{
		{"Int", 42, 42, false},
		{"Int8", int8(42), 42, false},
		{"Int64", int64(42), 42, false},
		{"Uint", uint(42), 42, false},
		{"Float", 42.0, 42, false},
		{"String number", "42", 42, false},
		{"String non-number", "hello", 0, true},
		{"Unsupported type", []string{}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toInt(tt.value)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}