package validation

import (
	"github.com/go-playground/validator/v10"
	"github.com/remiges-tech/alya/wscutils"
)

const (
	MsgIDInvalidJSON   = 1001
	MsgIDMissing       = 45
	MsgIDDataFormat    = 101
	MsgIDTooSmall      = 102
	MsgIDTooLarge      = 103
	MsgIDInvalid       = 104
	MsgIDConflict      = 105
	MsgIDNotFound      = 106
	ErrCodeInvalidJSON = "invalid_json"
)

func NewUserValidator() *wscutils.Validator {
	return wscutils.NewValidator(
		map[string]wscutils.ValidationRule{
			"required": {MsgID: MsgIDMissing, ErrCode: "missing"},
			"email":    {MsgID: MsgIDDataFormat, ErrCode: "datafmt"},
			"alphanum": {MsgID: MsgIDDataFormat, ErrCode: "datafmt"},
			"min": {
				MsgID:   MsgIDTooSmall,
				ErrCode: "toosmall",
				GetVals: func(err validator.FieldError) []string { return []string{err.Param()} },
			},
			"max": {
				MsgID:   MsgIDTooLarge,
				ErrCode: "toobig",
				GetVals: func(err validator.FieldError) []string { return []string{err.Param()} },
			},
		},
		wscutils.ValidationRule{MsgID: MsgIDInvalid, ErrCode: "invalid"},
	)
}

func NewOrderValidator() *wscutils.Validator {
	return wscutils.NewValidator(
		map[string]wscutils.ValidationRule{
			"required": {MsgID: MsgIDMissing, ErrCode: "missing"},
			"alphanum": {MsgID: MsgIDDataFormat, ErrCode: "datafmt"},
			"oneof": {
				MsgID:   MsgIDInvalid,
				ErrCode: "invalid",
				GetVals: func(err validator.FieldError) []string { return []string{err.Param()} },
			},
			"min": {
				MsgID:   MsgIDTooSmall,
				ErrCode: "toosmall",
				GetVals: func(err validator.FieldError) []string { return []string{err.Param()} },
			},
			"max": {
				MsgID:   MsgIDTooLarge,
				ErrCode: "toobig",
				GetVals: func(err validator.FieldError) []string { return []string{err.Param()} },
			},
			"gte": {
				MsgID:   MsgIDTooSmall,
				ErrCode: "toosmall",
				GetVals: func(err validator.FieldError) []string { return []string{err.Param()} },
			},
		},
		wscutils.ValidationRule{MsgID: MsgIDInvalid, ErrCode: "invalid"},
	)
}
