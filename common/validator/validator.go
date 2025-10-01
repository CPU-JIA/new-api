package validator

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// ValidationRule defines the validation rule interface
type ValidationRule interface {
	Validate(value interface{}, fieldName string) error
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Rule    string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}

	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// Validator provides configuration validation functionality
type Validator struct {
	envValidations map[string][]ValidationRule
	errors         ValidationErrors
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		envValidations: make(map[string][]ValidationRule),
		errors:        make(ValidationErrors, 0),
	}
}

// AddEnvValidation adds validation rules for environment variables
func (v *Validator) AddEnvValidation(envName string, rules ...ValidationRule) *Validator {
	v.envValidations[envName] = rules
	return v
}

// ValidateEnvVars validates all registered environment variables
func (v *Validator) ValidateEnvVars() error {
	v.errors = make(ValidationErrors, 0)

	for envName, rules := range v.envValidations {
		value := os.Getenv(envName)
		for _, rule := range rules {
			if err := rule.Validate(value, envName); err != nil {
				if validationErr, ok := err.(*ValidationError); ok {
					v.errors = append(v.errors, *validationErr)
				} else {
					v.errors = append(v.errors, ValidationError{
						Field:   envName,
						Value:   value,
						Message: err.Error(),
					})
				}
			}
		}
	}

	if v.errors.HasErrors() {
		return v.errors
	}
	return nil
}

// ValidateStruct validates a struct using struct tags
func (v *Validator) ValidateStruct(s interface{}) error {
	v.errors = make(ValidationErrors, 0)

	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", val.Kind())
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get validation tags
		validationTag := fieldType.Tag.Get("validate")
		if validationTag == "" {
			continue
		}

		fieldName := fieldType.Name
		if jsonTag := fieldType.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			fieldName = strings.Split(jsonTag, ",")[0]
		}

		// Parse and apply validation rules
		if err := v.validateField(field.Interface(), fieldName, validationTag); err != nil {
			if validationErr, ok := err.(*ValidationError); ok {
				v.errors = append(v.errors, *validationErr)
			}
		}
	}

	if v.errors.HasErrors() {
		return v.errors
	}
	return nil
}

// validateField validates a single field based on validation tags
func (v *Validator) validateField(value interface{}, fieldName, tag string) error {
	rules := strings.Split(tag, ",")

	for _, ruleStr := range rules {
		ruleStr = strings.TrimSpace(ruleStr)
		if ruleStr == "" {
			continue
		}

		// Parse rule and parameters
		parts := strings.SplitN(ruleStr, "=", 2)
		ruleName := parts[0]
		ruleParam := ""
		if len(parts) > 1 {
			ruleParam = parts[1]
		}

		var rule ValidationRule
		switch ruleName {
		case "required":
			rule = &RequiredRule{}
		case "min":
			if param, err := strconv.Atoi(ruleParam); err == nil {
				rule = &MinRule{Min: param}
			}
		case "max":
			if param, err := strconv.Atoi(ruleParam); err == nil {
				rule = &MaxRule{Max: param}
			}
		case "range":
			if params := strings.Split(ruleParam, "-"); len(params) == 2 {
				if min, err1 := strconv.Atoi(params[0]); err1 == nil {
					if max, err2 := strconv.Atoi(params[1]); err2 == nil {
						rule = &RangeRule{Min: min, Max: max}
					}
				}
			}
		case "regex":
			rule = &RegexRule{Pattern: ruleParam}
		case "url":
			rule = &URLRule{}
		case "email":
			rule = &EmailRule{}
		case "password_complexity":
			rule = &PasswordComplexityRule{}
		case "oneof":
			values := strings.Split(ruleParam, " ")
			rule = &OneOfRule{Values: values}
		default:
			continue // Skip unknown rules
		}

		if rule != nil {
			if err := rule.Validate(value, fieldName); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetErrors returns all validation errors
func (v *Validator) GetErrors() ValidationErrors {
	return v.errors
}

// ClearErrors clears all validation errors
func (v *Validator) ClearErrors() {
	v.errors = make(ValidationErrors, 0)
}