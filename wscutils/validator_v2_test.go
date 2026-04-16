package wscutils

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type validatorV2Request struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2"`
}

func TestValidatorV2UsesJSONFieldNamesAndRules(t *testing.T) {
	validator := NewValidator(
		map[string]ValidationRule{
			"required": {
				MsgID:   45,
				ErrCode: "missing",
			},
			"email": {
				MsgID:   101,
				ErrCode: "datafmt",
			},
			"min": {
				MsgID:   102,
				ErrCode: "toosmall",
				GetVals: func(err validator.FieldError) []string {
					return []string{err.Param()}
				},
			},
		},
		ValidationRule{MsgID: 9999, ErrCode: "invalid"},
	)

	errs := validator.Validate(validatorV2Request{Email: "bad-email", Name: "A"})
	want := []ErrorMessage{
		{MsgID: 101, ErrCode: "datafmt", Field: "email"},
		{MsgID: 102, ErrCode: "toosmall", Field: "name", Vals: []string{"2"}},
	}

	if !reflect.DeepEqual(errs, want) {
		t.Fatalf("Validate() got %#v, want %#v", errs, want)
	}
}

func TestBindDataSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type request struct {
		Name string `json:"name"`
	}

	body := bytes.NewBufferString(`{"data":{"name":"John"}}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var data request
	if err := BindData(c, &data); err != nil {
		t.Fatalf("BindData() error = %v", err)
	}
	if data.Name != "John" {
		t.Fatalf("BindData() name = %q, want %q", data.Name, "John")
	}
}

func TestBindDataRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	type request struct {
		Name string `json:"name"`
	}

	body := bytes.NewBufferString(`{"data":{"name":"John","extra":"x"}}`)
	req, _ := http.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var data request
	if err := BindData(c, &data); err == nil {
		t.Fatal("BindData() expected error")
	}
}
