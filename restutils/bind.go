package restutils

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// BindBody decodes a JSON request body into dst.
func BindBody(c *gin.Context, dst any) error {
	if c.Request.Body == nil {
		return &BindError{Kind: BindErrorEmptyBody, Detail: "request body is empty"}
	}

	if err := requireJSONContentType(c.Request); err != nil {
		return err
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return classifyDecodeError(err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return &BindError{Kind: BindErrorMalformedJSON, Detail: "request body must contain a single JSON object"}
	}

	return nil
}

func requireJSONContentType(r *http.Request) error {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return &BindError{Kind: BindErrorInvalidContentType, Detail: "Content-Type must be application/json"}
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return &BindError{Kind: BindErrorInvalidContentType, Detail: "Content-Type must be application/json"}
	}
	return nil
}

func classifyDecodeError(err error) error {
	switch e := err.(type) {
	case *json.SyntaxError:
		return &BindError{Kind: BindErrorMalformedJSON, Detail: fmt.Sprintf("request body contains malformed JSON at position %d", e.Offset)}
	case *json.UnmarshalTypeError:
		field := e.Field
		if field == "" {
			field = e.Struct
		}
		detail := "request body contains a value with the wrong type"
		if field != "" {
			detail = fmt.Sprintf("request body contains a value with the wrong type for field %q", field)
		}
		return &BindError{Kind: BindErrorInvalidValue, Field: field, Detail: detail}
	case nil:
		return nil
	}

	if err == io.EOF {
		return &BindError{Kind: BindErrorEmptyBody, Detail: "request body is empty"}
	}

	message := err.Error()
	if strings.HasPrefix(message, "json: unknown field ") {
		field := strings.TrimPrefix(message, "json: unknown field ")
		field = strings.Trim(field, `"`)
		return &BindError{Kind: BindErrorUnknownField, Field: field, Detail: fmt.Sprintf("request body contains unknown field %q", field)}
	}

	return &BindError{Kind: BindErrorMalformedJSON, Detail: "request body contains malformed JSON"}
}
