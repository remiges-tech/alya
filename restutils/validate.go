package restutils

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/remiges-tech/alya/wscutils"
)

const (
	msgIDMissing    = 45
	msgIDDataFormat = 101
	msgIDTooSmall   = 102
	msgIDTooLarge   = 103
	msgIDInvalid    = 104
)

// ValidationRule maps one validator tag to Alya-style error fields.
type ValidationRule struct {
	MsgID      int
	ErrCode    string
	GetVals    func(err validator.FieldError) []string
	GetMessage func(err validator.FieldError) string
	GetParams  func(err validator.FieldError) map[string]string
}

// ValidatorConfig adds service-specific validation mappings on top of the built-in tag rules.
type ValidatorConfig struct {
	TagRules    map[string]ValidationRule
	FieldRules  map[string]map[string]ValidationRule
	DefaultRule *ValidationRule
}

var fallbackValidationRule = ValidationRule{
	MsgID:   msgIDInvalid,
	ErrCode: "invalid",
	GetMessage: func(err validator.FieldError) string {
		return err.Error()
	},
}

var defaultTagRules = map[string]ValidationRule{
	"required": {
		MsgID:   msgIDMissing,
		ErrCode: "missing",
		GetMessage: func(err validator.FieldError) string {
			return "is required"
		},
	},
	"email": {
		MsgID:   msgIDDataFormat,
		ErrCode: "datafmt",
		GetMessage: func(err validator.FieldError) string {
			return "must be a valid email address"
		},
	},
	"uuid": {
		MsgID:   msgIDDataFormat,
		ErrCode: "datafmt",
		GetMessage: func(err validator.FieldError) string {
			return "has an invalid format"
		},
	},
	"e164": {
		MsgID:   msgIDDataFormat,
		ErrCode: "datafmt",
		GetMessage: func(err validator.FieldError) string {
			return "has an invalid format"
		},
	},
	"alphanum": {
		MsgID:   msgIDDataFormat,
		ErrCode: "datafmt",
		GetMessage: func(err validator.FieldError) string {
			return "has an invalid format"
		},
	},
	"oneof": {
		MsgID:   msgIDInvalid,
		ErrCode: "invalid",
		GetVals: func(err validator.FieldError) []string {
			return []string{err.Param()}
		},
		GetMessage: func(err validator.FieldError) string {
			return "has an invalid value"
		},
		GetParams: func(err validator.FieldError) map[string]string {
			return map[string]string{"allowed": err.Param()}
		},
	},
	"min": {
		MsgID:   msgIDTooSmall,
		ErrCode: "toosmall",
		GetVals: func(err validator.FieldError) []string {
			return []string{err.Param()}
		},
		GetMessage: func(err validator.FieldError) string {
			if isLengthRule(err.Kind()) {
				return fmt.Sprintf("must be at least %s characters", err.Param())
			}
			return fmt.Sprintf("must be at least %s", err.Param())
		},
		GetParams: func(err validator.FieldError) map[string]string {
			return map[string]string{"min": err.Param()}
		},
	},
	"max": {
		MsgID:   msgIDTooLarge,
		ErrCode: "toobig",
		GetVals: func(err validator.FieldError) []string {
			return []string{err.Param()}
		},
		GetMessage: func(err validator.FieldError) string {
			if isLengthRule(err.Kind()) {
				return fmt.Sprintf("must be at most %s characters", err.Param())
			}
			return fmt.Sprintf("must be at most %s", err.Param())
		},
		GetParams: func(err validator.FieldError) map[string]string {
			return map[string]string{"max": err.Param()}
		},
	},
	"gte": {
		MsgID:   msgIDTooSmall,
		ErrCode: "toosmall",
		GetVals: func(err validator.FieldError) []string {
			return []string{err.Param()}
		},
		GetMessage: func(err validator.FieldError) string {
			return fmt.Sprintf("must be at least %s", err.Param())
		},
		GetParams: func(err validator.FieldError) map[string]string {
			return map[string]string{"min": err.Param()}
		},
	},
	"lte": {
		MsgID:   msgIDTooLarge,
		ErrCode: "toobig",
		GetVals: func(err validator.FieldError) []string {
			return []string{err.Param()}
		},
		GetMessage: func(err validator.FieldError) string {
			return fmt.Sprintf("must be at most %s", err.Param())
		},
		GetParams: func(err validator.FieldError) map[string]string {
			return map[string]string{"max": err.Param()}
		},
	},
}

// Validator wraps go-playground/validator and returns REST field errors.
type Validator struct {
	validate    *validator.Validate
	tagRules    map[string]ValidationRule
	fieldRules  map[string]map[string]ValidationRule
	defaultRule *ValidationRule
}

// NewValidator creates a Validator with the built-in tag mappings.
func NewValidator() *Validator {
	return NewValidatorWithConfig(ValidatorConfig{})
}

// NewValidatorWithConfig creates a Validator with the built-in tag mappings and optional service overrides.
func NewValidatorWithConfig(cfg ValidatorConfig) *Validator {
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
	return &Validator{
		validate:    v,
		tagRules:    copyTagRules(cfg.TagRules),
		fieldRules:  copyFieldRules(cfg.FieldRules),
		defaultRule: copyDefaultRule(cfg.DefaultRule),
	}
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
		return []FieldError{newFieldError(msgIDInvalid, "invalid", "", nil, err.Error())}
	}

	result := make([]FieldError, 0, len(validationErrors))
	for _, validationErr := range validationErrors {
		result = append(result, v.toFieldError(validationErr))
	}
	return result
}

func (v *Validator) toFieldError(err validator.FieldError) FieldError {
	field := namespaceWithoutRoot(err.Namespace())
	if field == "" {
		field = err.Field()
	}

	rule := v.resolveRule(field, err.Tag())
	vals := getVals(rule, err)
	message := getMessage(rule, err)
	params := getParams(rule, err)
	return newFieldError(rule.MsgID, rule.ErrCode, field, params, message, vals...)
}

func (v *Validator) resolveRule(field, tag string) ValidationRule {
	rule := fallbackValidationRule
	matched := false

	if defaultRule, ok := defaultTagRules[tag]; ok {
		rule = mergeValidationRule(rule, defaultRule)
		matched = true
	}
	if tagRule, ok := v.tagRules[tag]; ok {
		rule = mergeValidationRule(rule, tagRule)
		matched = true
	}
	if fieldRule, ok := fieldRuleFor(v.fieldRules, field, tag); ok {
		rule = mergeValidationRule(rule, fieldRule)
		matched = true
	}
	if !matched && v.defaultRule != nil {
		rule = mergeValidationRule(rule, *v.defaultRule)
	}
	return rule
}

func mergeValidationRule(base, override ValidationRule) ValidationRule {
	merged := base
	if override.MsgID != 0 {
		merged.MsgID = override.MsgID
	}
	if override.ErrCode != "" {
		merged.ErrCode = override.ErrCode
	}
	if override.GetVals != nil {
		merged.GetVals = override.GetVals
	}
	if override.GetMessage != nil {
		merged.GetMessage = override.GetMessage
	}
	if override.GetParams != nil {
		merged.GetParams = override.GetParams
	}
	return merged
}

func fieldRuleFor(fieldRules map[string]map[string]ValidationRule, field, tag string) (ValidationRule, bool) {
	tagMap, ok := fieldRules[field]
	if !ok {
		return ValidationRule{}, false
	}
	rule, ok := tagMap[tag]
	return rule, ok
}

func getVals(rule ValidationRule, err validator.FieldError) []string {
	if rule.GetVals == nil {
		return nil
	}
	return rule.GetVals(err)
}

func getMessage(rule ValidationRule, err validator.FieldError) string {
	if rule.GetMessage == nil {
		return ""
	}
	return rule.GetMessage(err)
}

func getParams(rule ValidationRule, err validator.FieldError) map[string]string {
	if rule.GetParams == nil {
		return nil
	}
	return rule.GetParams(err)
}

func copyTagRules(src map[string]ValidationRule) map[string]ValidationRule {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]ValidationRule, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func copyFieldRules(src map[string]map[string]ValidationRule) map[string]map[string]ValidationRule {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]map[string]ValidationRule, len(src))
	for field, rules := range src {
		copiedRules := make(map[string]ValidationRule, len(rules))
		for tag, rule := range rules {
			copiedRules[tag] = rule
		}
		dst[field] = copiedRules
	}
	return dst
}

func copyDefaultRule(rule *ValidationRule) *ValidationRule {
	if rule == nil {
		return nil
	}
	copied := *rule
	return &copied
}

func newFieldError(msgID int, errCode, field string, params map[string]string, message string, vals ...string) FieldError {
	return FieldError{
		ErrorMessage: wscutils.BuildErrorMessage(msgID, errCode, field, vals...),
		Message:      message,
		Params:       params,
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
