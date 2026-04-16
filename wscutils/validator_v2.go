package wscutils

import (
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// ValidationRule maps a validator tag to Alya error fields.
type ValidationRule struct {
	MsgID   int
	ErrCode string
	GetVals func(err validator.FieldError) []string
}

// Validator validates request structs and returns Alya ErrorMessage values.
type Validator struct {
	validate    *validator.Validate
	rules       map[string]ValidationRule
	defaultRule ValidationRule
}

// NewValidator creates an instance-based validator.
//
// This API is additive. It does not change the behavior of WscValidate or the
// package-level validation maps used by existing code.
func NewValidator(rules map[string]ValidationRule, defaultRule ValidationRule) *Validator {
	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			return field.Name
		}
		if name == "-" {
			return ""
		}
		return name
	})

	copiedRules := make(map[string]ValidationRule, len(rules))
	for key, value := range rules {
		copiedRules[key] = value
	}

	return &Validator{
		validate:    validate,
		rules:       copiedRules,
		defaultRule: defaultRule,
	}
}

// Engine returns the underlying validator engine.
func (v *Validator) Engine() *validator.Validate {
	return v.validate
}

// Validate validates data and returns Alya ErrorMessage values.
func (v *Validator) Validate(data any) []ErrorMessage {
	err := v.validate.Struct(data)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return []ErrorMessage{BuildErrorMessage(v.defaultRule.MsgID, v.defaultRule.ErrCode, "")}
	}

	result := make([]ErrorMessage, 0, len(validationErrors))
	for _, validationErr := range validationErrors {
		rule, ok := v.rules[validationErr.Tag()]
		if !ok {
			rule = v.defaultRule
		}

		var vals []string
		if rule.GetVals != nil {
			vals = rule.GetVals(validationErr)
		}

		field := validationErr.Field()
		result = append(result, BuildErrorMessage(rule.MsgID, rule.ErrCode, field, vals...))
	}

	return result
}
