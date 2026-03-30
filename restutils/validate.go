package restutils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground/validator and returns REST field errors.
type Validator struct {
	validate *validator.Validate
}

// NewValidator creates a Validator with JSON tag field names.
func NewValidator() *Validator {
	v := validator.New()
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			return field.Name
		}
		if name == "-" {
			return ""
		}
		return name
	})
	return &Validator{validate: v}
}

// Engine returns the underlying validator engine.
func (v *Validator) Engine() *validator.Validate {
	return v.validate
}

// Validate validates data and returns REST field errors.
func (v *Validator) Validate(data any) []FieldError {
	err := v.validate.Struct(data)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []FieldError{{Code: "invalid", Message: err.Error()}}
	}

	result := make([]FieldError, 0, len(validationErrors))
	for _, validationErr := range validationErrors {
		result = append(result, toFieldError(validationErr))
	}
	return result
}

func toFieldError(err validator.FieldError) FieldError {
	field := namespaceWithoutRoot(err.Namespace())
	if field == "" {
		field = err.Field()
	}

	switch err.Tag() {
	case "required":
		return FieldError{Field: field, Code: "required", Message: "is required"}
	case "email":
		return FieldError{Field: field, Code: "invalid_format", Message: "must be a valid email address"}
	case "uuid", "e164", "alphanum":
		return FieldError{Field: field, Code: "invalid_format", Message: "has an invalid format"}
	case "oneof":
		return FieldError{Field: field, Code: "invalid_value", Message: "has an invalid value", Params: map[string]string{"allowed": err.Param()}}
	case "min":
		if isLengthRule(err.Kind()) {
			return FieldError{Field: field, Code: "min_length", Message: fmt.Sprintf("must be at least %s characters", err.Param()), Params: map[string]string{"min": err.Param()}}
		}
		return FieldError{Field: field, Code: "too_small", Message: fmt.Sprintf("must be at least %s", err.Param()), Params: map[string]string{"min": err.Param()}}
	case "max":
		if isLengthRule(err.Kind()) {
			return FieldError{Field: field, Code: "max_length", Message: fmt.Sprintf("must be at most %s characters", err.Param()), Params: map[string]string{"max": err.Param()}}
		}
		return FieldError{Field: field, Code: "too_large", Message: fmt.Sprintf("must be at most %s", err.Param()), Params: map[string]string{"max": err.Param()}}
	case "gte":
		return FieldError{Field: field, Code: "too_small", Message: fmt.Sprintf("must be at least %s", err.Param()), Params: map[string]string{"min": err.Param()}}
	case "lte":
		return FieldError{Field: field, Code: "too_large", Message: fmt.Sprintf("must be at most %s", err.Param()), Params: map[string]string{"max": err.Param()}}
	default:
		return FieldError{Field: field, Code: err.Tag(), Message: err.Error()}
	}
}

func namespaceWithoutRoot(namespace string) string {
	if namespace == "" {
		return ""
	}
	parts := strings.Split(namespace, ".")
	if len(parts) <= 1 {
		return namespace
	}
	return strings.Join(parts[1:], ".")
}

func isLengthRule(kind reflect.Kind) bool {
	switch kind {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return true
	default:
		return false
	}
}
