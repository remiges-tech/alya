package restutils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type validateRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func TestValidatorUsesJSONFieldNames(t *testing.T) {
	v := NewValidator()
	errs := v.Validate(validateRequest{Email: "not-an-email"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Field != "email" {
		t.Fatalf("expected field email, got %q", errs[0].Field)
	}
	if errs[0].ErrCode != "datafmt" {
		t.Fatalf("expected errcode datafmt, got %q", errs[0].ErrCode)
	}
	if errs[0].MsgID != msgIDDataFormat {
		t.Fatalf("expected msgid %d, got %d", msgIDDataFormat, errs[0].MsgID)
	}
}

func TestValidatorTagRuleOverrideUsesBuiltInMessageAndParams(t *testing.T) {
	v := NewValidatorWithConfig(ValidatorConfig{
		TagRules: map[string]ValidationRule{
			"max": {MsgID: 7, ErrCode: "toobig_custom"},
		},
	})

	req := struct {
		AccountNumber string `json:"account_number" validate:"max=10"`
	}{AccountNumber: "12345678901"}

	errs := v.Validate(req)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Field != "account_number" {
		t.Fatalf("expected field account_number, got %q", errs[0].Field)
	}
	if errs[0].MsgID != 7 {
		t.Fatalf("expected msgid 7, got %d", errs[0].MsgID)
	}
	if errs[0].ErrCode != "toobig_custom" {
		t.Fatalf("expected errcode toobig_custom, got %q", errs[0].ErrCode)
	}
	if errs[0].Message != "must be at most 10 characters" {
		t.Fatalf("expected built-in message, got %q", errs[0].Message)
	}
	if len(errs[0].Vals) != 1 || errs[0].Vals[0] != "10" {
		t.Fatalf("expected vals [10], got %#v", errs[0].Vals)
	}
	if errs[0].Params["max"] != "10" {
		t.Fatalf("expected params max=10, got %#v", errs[0].Params)
	}
}

func TestValidatorFieldRuleOverrideBeatsTagRule(t *testing.T) {
	v := NewValidatorWithConfig(ValidatorConfig{
		TagRules: map[string]ValidationRule{
			"max": {MsgID: 7, ErrCode: "toobig_tag"},
		},
		FieldRules: map[string]map[string]ValidationRule{
			"account_number": {
				"max": {
					MsgID:   8,
					ErrCode: "toobig_field",
					GetMessage: func(err validator.FieldError) string {
						return "Account number must be 10 characters long"
					},
				},
			},
		},
	})

	req := struct {
		AccountNumber string `json:"account_number" validate:"max=10"`
		Username      string `json:"username" validate:"max=5"`
	}{
		AccountNumber: "12345678901",
		Username:      "longer-than-5",
	}

	errs := v.Validate(req)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}

	accountErr := errs[0]
	usernameErr := errs[1]

	if accountErr.Field != "account_number" {
		t.Fatalf("expected first field account_number, got %q", accountErr.Field)
	}
	if accountErr.MsgID != 8 || accountErr.ErrCode != "toobig_field" {
		t.Fatalf("expected field override, got msgid=%d errcode=%q", accountErr.MsgID, accountErr.ErrCode)
	}
	if accountErr.Message != "Account number must be 10 characters long" {
		t.Fatalf("expected custom field message, got %q", accountErr.Message)
	}
	if len(accountErr.Vals) != 1 || accountErr.Vals[0] != "10" {
		t.Fatalf("expected account vals [10], got %#v", accountErr.Vals)
	}

	if usernameErr.Field != "username" {
		t.Fatalf("expected second field username, got %q", usernameErr.Field)
	}
	if usernameErr.MsgID != 7 || usernameErr.ErrCode != "toobig_tag" {
		t.Fatalf("expected tag override for username, got msgid=%d errcode=%q", usernameErr.MsgID, usernameErr.ErrCode)
	}
	if usernameErr.Message != "must be at most 5 characters" {
		t.Fatalf("expected built-in tag message, got %q", usernameErr.Message)
	}
}

func TestValidatorDefaultRuleForUnknownTag(t *testing.T) {
	v := NewValidatorWithConfig(ValidatorConfig{
		DefaultRule: &ValidationRule{
			MsgID:   900,
			ErrCode: "custom_invalid",
			GetMessage: func(err validator.FieldError) string {
				return "custom default"
			},
		},
	})
	v.Engine().RegisterValidation("mustfoo", func(fl validator.FieldLevel) bool {
		return fl.Field().String() == "foo"
	})

	req := struct {
		Name string `json:"name" validate:"mustfoo"`
	}{Name: "bar"}

	errs := v.Validate(req)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].MsgID != 900 || errs[0].ErrCode != "custom_invalid" {
		t.Fatalf("expected default override, got msgid=%d errcode=%q", errs[0].MsgID, errs[0].ErrCode)
	}
	if errs[0].Message != "custom default" {
		t.Fatalf("expected custom default message, got %q", errs[0].Message)
	}
}

func TestBindBodyRejectsUnknownFields(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := ginTestContext(recorder, http.MethodPost, "/users", `{"name":"alice","extra":"x"}`)

	var req struct {
		Name string `json:"name"`
	}

	err := BindBody(ctx, &req)
	if err == nil {
		t.Fatal("expected bind error")
	}
	problem := ProblemFromBindError(err)
	if problem.Status != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", problem.Status)
	}
	if len(problem.Errors) != 1 || problem.Errors[0].Field != "extra" {
		t.Fatalf("expected unknown field error for extra, got %#v", problem.Errors)
	}
	if problem.Errors[0].ErrCode != "invalid" {
		t.Fatalf("expected errcode invalid, got %q", problem.Errors[0].ErrCode)
	}
}

func ginTestContext(recorder *httptest.ResponseRecorder, method, target, body string) (*gin.Context, *http.Request) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	return ctx, req
}
