package restutils

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	problemTypeBadRequest           = "https://alya.dev/problems/bad-request"
	problemTypeUnauthorized         = "https://alya.dev/problems/unauthorized"
	problemTypeForbidden            = "https://alya.dev/problems/forbidden"
	problemTypeNotFound             = "https://alya.dev/problems/not-found"
	problemTypeConflict             = "https://alya.dev/problems/conflict"
	problemTypeUnsupportedMediaType = "https://alya.dev/problems/unsupported-media-type"
	problemTypeValidation           = "https://alya.dev/problems/validation"
	problemTypeInternal             = "https://alya.dev/problems/internal"
)

// NewProblem creates a problem response with the given values.
func NewProblem(status int, typeURI, title, detail string) Problem {
	return Problem{
		Type:   typeURI,
		Title:  title,
		Status: status,
		Detail: detail,
	}
}

// ValidationProblem returns a 422 problem for field validation errors.
func ValidationProblem(errors []FieldError) Problem {
	return Problem{
		Type:   problemTypeValidation,
		Title:  "Validation failed",
		Status: http.StatusUnprocessableEntity,
		Errors: errors,
	}
}

// InternalServerError returns a generic 500 problem.
func InternalServerError() Problem {
	return NewProblem(
		http.StatusInternalServerError,
		problemTypeInternal,
		"Internal server error",
		"the server could not process the request",
	)
}

// ProblemFromBindError converts a binding error to a problem response.
func ProblemFromBindError(err error) Problem {
	var bindErr *BindError
	if !errors.As(err, &bindErr) {
		return NewProblem(
			http.StatusBadRequest,
			problemTypeBadRequest,
			"Bad request",
			err.Error(),
		)
	}

	switch bindErr.Kind {
	case BindErrorInvalidContentType:
		return NewProblem(
			http.StatusUnsupportedMediaType,
			problemTypeUnsupportedMediaType,
			"Unsupported media type",
			bindErr.Detail,
		)
	case BindErrorUnknownField:
		return Problem{
			Type:   problemTypeBadRequest,
			Title:  "Bad request",
			Status: http.StatusBadRequest,
			Detail: bindErr.Detail,
			Errors: []FieldError{{Field: bindErr.Field, Code: string(BindErrorUnknownField)}},
		}
	case BindErrorEmptyBody, BindErrorMalformedJSON, BindErrorInvalidValue:
		return NewProblem(
			http.StatusBadRequest,
			problemTypeBadRequest,
			"Bad request",
			bindErr.Detail,
		)
	default:
		return NewProblem(
			http.StatusBadRequest,
			problemTypeBadRequest,
			"Bad request",
			bindErr.Detail,
		)
	}
}

// WriteProblem writes a problem response and aborts the request.
func WriteProblem(c *gin.Context, p Problem) {
	if p.Status == 0 {
		p.Status = http.StatusInternalServerError
	}
	if p.Instance == "" && c != nil && c.Request != nil && c.Request.URL != nil {
		p.Instance = c.Request.URL.Path
	}
	if p.TraceID == "" {
		if traceID, ok := c.Get("trace_id"); ok {
			if value, ok := traceID.(string); ok {
				p.TraceID = value
			}
		}
	}
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(p.Status, p)
}
