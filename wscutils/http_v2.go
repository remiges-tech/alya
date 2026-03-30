package wscutils

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type requestEnvelope[T any] struct {
	Data T `json:"data"`
}

// BindData binds a request body with Alya's {"data": ...} envelope.
//
// This API is additive. It does not change BindJSON behavior used by existing code.
func BindData[T any](c *gin.Context, dst *T) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	if err := requireJSONContentType(c.Request); err != nil {
		return err
	}

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	var req requestEnvelope[T]
	if err := decoder.Decode(&req); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain a single JSON object")
	}

	*dst = req.Data
	return nil
}

func requireJSONContentType(r *http.Request) error {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return fmt.Errorf("Content-Type must be application/json")
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("Content-Type must be application/json")
	}
	return nil
}

// ParseInt64PathParam parses one path parameter as int64.
func ParseInt64PathParam(c *gin.Context, name string) (int64, error) {
	value := c.Param(name)
	if value == "" {
		return 0, fmt.Errorf("missing path parameter %q", name)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid path parameter %q", name)
	}
	return parsed, nil
}

// SendOK sends a 200 success response in Alya envelope format.
func SendOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, NewSuccessResponse(data))
}

// SendCreated sends a 201 success response in Alya envelope format.
func SendCreated(c *gin.Context, location string, data any) {
	if location != "" {
		c.Header("Location", location)
	}
	c.JSON(http.StatusCreated, NewSuccessResponse(data))
}

// SendAccepted sends a 202 success response in Alya envelope format.
func SendAccepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, NewSuccessResponse(data))
}

// SendDeleted sends a 200 success response with an empty data object.
func SendDeleted(c *gin.Context) {
	c.JSON(http.StatusOK, NewSuccessResponse(map[string]any{}))
}

// SendError sends an error response in Alya envelope format with the given HTTP status.
func SendError(c *gin.Context, status int, messages []ErrorMessage) {
	c.JSON(status, NewResponse(ErrorStatus, nil, messages))
}
