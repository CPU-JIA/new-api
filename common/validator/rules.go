package validator

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// RequiredRule validates that a field is not empty
type RequiredRule struct{}

func (r *RequiredRule) Validate(value interface{}, fieldName string) error {
	if isEmpty(value) {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "required",
			Message: "field is required and cannot be empty",
		}
	}
	return nil
}

// MinRule validates that a numeric value is at least a minimum value
type MinRule struct {
	Min int
}

func (r *MinRule) Validate(value interface{}, fieldName string) error {
	num, err := toInt(value)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "min",
			Message: fmt.Sprintf("value must be a number for min validation"),
		}
	}

	if num < r.Min {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "min",
			Message: fmt.Sprintf("value must be at least %d", r.Min),
		}
	}
	return nil
}

// MaxRule validates that a numeric value is at most a maximum value
type MaxRule struct {
	Max int
}

func (r *MaxRule) Validate(value interface{}, fieldName string) error {
	num, err := toInt(value)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "max",
			Message: fmt.Sprintf("value must be a number for max validation"),
		}
	}

	if num > r.Max {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "max",
			Message: fmt.Sprintf("value must be at most %d", r.Max),
		}
	}
	return nil
}

// RangeRule validates that a numeric value is within a range
type RangeRule struct {
	Min int
	Max int
}

func (r *RangeRule) Validate(value interface{}, fieldName string) error {
	num, err := toInt(value)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "range",
			Message: fmt.Sprintf("value must be a number for range validation"),
		}
	}

	if num < r.Min || num > r.Max {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "range",
			Message: fmt.Sprintf("value must be between %d and %d", r.Min, r.Max),
		}
	}
	return nil
}

// RegexRule validates that a string matches a regular expression
type RegexRule struct {
	Pattern string
}

func (r *RegexRule) Validate(value interface{}, fieldName string) error {
	str := toString(value)
	if str == "" && isEmpty(value) {
		return nil // Empty values are allowed unless required rule is also specified
	}

	matched, err := regexp.MatchString(r.Pattern, str)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "regex",
			Message: fmt.Sprintf("invalid regex pattern: %s", err.Error()),
		}
	}

	if !matched {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "regex",
			Message: fmt.Sprintf("value does not match pattern: %s", r.Pattern),
		}
	}
	return nil
}

// URLRule validates that a string is a valid URL
type URLRule struct{}

func (r *URLRule) Validate(value interface{}, fieldName string) error {
	str := toString(value)
	if str == "" && isEmpty(value) {
		return nil // Empty values are allowed unless required rule is also specified
	}

	u, err := url.Parse(str)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "url",
			Message: "value must be a valid URL",
		}
	}

	// Additional validation for proper URL format
	if u.Scheme == "" {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "url",
			Message: "URL must have a scheme (http, https, etc.)",
		}
	}

	return nil
}

// EmailRule validates that a string is a valid email address
type EmailRule struct{}

func (r *EmailRule) Validate(value interface{}, fieldName string) error {
	str := toString(value)
	if str == "" && isEmpty(value) {
		return nil // Empty values are allowed unless required rule is also specified
	}

	emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, err := regexp.MatchString(emailRegex, str)
	if err != nil || !matched {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "email",
			Message: "value must be a valid email address",
		}
	}
	return nil
}

// OneOfRule validates that a value is one of the specified values
type OneOfRule struct {
	Values []string
}

func (r *OneOfRule) Validate(value interface{}, fieldName string) error {
	str := toString(value)
	if str == "" && isEmpty(value) {
		return nil // Empty values are allowed unless required rule is also specified
	}

	for _, validValue := range r.Values {
		if str == validValue {
			return nil
		}
	}

	return &ValidationError{
		Field:   fieldName,
		Value:   value,
		Rule:    "oneof",
		Message: fmt.Sprintf("value must be one of: %s", strings.Join(r.Values, ", ")),
	}
}

// PortRule validates that a value is a valid port number
type PortRule struct{}

func (r *PortRule) Validate(value interface{}, fieldName string) error {
	num, err := toInt(value)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "port",
			Message: "value must be a number for port validation",
		}
	}

	if num < 1 || num > 65535 {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "port",
			Message: "value must be a valid port number (1-65535)",
		}
	}
	return nil
}

// BoolRule validates that a value is a valid boolean
type BoolRule struct{}

func (r *BoolRule) Validate(value interface{}, fieldName string) error {
	str := toString(value)
	if str == "" && isEmpty(value) {
		return nil // Empty values are allowed unless required rule is also specified
	}

	_, err := strconv.ParseBool(str)
	if err != nil {
		return &ValidationError{
			Field:   fieldName,
			Value:   value,
			Rule:    "bool",
			Message: "value must be a valid boolean (true/false, 1/0)",
		}
	}
	return nil
}

// Helper functions

// isEmpty checks if a value is considered empty
func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// toString converts a value to string
func toString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toInt converts a value to integer
func toInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}