package restutils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type validateRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func TestValidatorUsesJSONFieldNames(t *testing.T) {
	validator := NewValidator()
	errs := validator.Validate(validateRequest{Email: "not-an-email"})
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Field != "email" {
		t.Fatalf("expected field email, got %q", errs[0].Field)
	}
	if errs[0].Code != "invalid_format" {
		t.Fatalf("expected code invalid_format, got %q", errs[0].Code)
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
}

func ginTestContext(recorder *httptest.ResponseRecorder, method, target, body string) (*gin.Context, *http.Request) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = req
	return ctx, req
}
