package restutils

import "github.com/remiges-tech/alya/wscutils"

// Problem is a problem-details style error response.
type Problem struct {
	Type     string         `json:"type,omitempty"`
	Title    string         `json:"title,omitempty"`
	Status   int            `json:"status,omitempty"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	TraceID  string         `json:"trace_id,omitempty"`
	Errors   []FieldError   `json:"errors,omitempty"`
	Meta     map[string]any `json:"meta,omitempty"`
}

// FieldError describes one request field error.
type FieldError struct {
	wscutils.ErrorMessage
	Message string            `json:"message,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

// BindErrorKind identifies request binding failures.
type BindErrorKind string

const (
	BindErrorEmptyBody          BindErrorKind = "empty_body"
	BindErrorInvalidContentType BindErrorKind = "invalid_content_type"
	BindErrorMalformedJSON      BindErrorKind = "malformed_json"
	BindErrorUnknownField       BindErrorKind = "unknown_field"
	BindErrorInvalidValue       BindErrorKind = "invalid_value"
)

// BindError describes one request binding failure.
type BindError struct {
	Kind   BindErrorKind
	Field  string
	Detail string
}

func (e *BindError) Error() string {
	if e == nil {
		return ""
	}
	if e.Detail != "" {
		return e.Detail
	}
	return string(e.Kind)
}
