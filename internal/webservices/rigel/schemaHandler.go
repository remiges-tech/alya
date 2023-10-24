package rigel

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go-framework/internal/wscutils"
	"net/http"
	"strconv"
)

type CreateSchemaVersionRequest struct {
	SchemaID int         `json:"schema_id" validate:"required"`
	Version  string      `json:"version"  validate:"required,min=2,max=50"`
	Fields   []FieldType `json:"fields"  validate:"required,dive"`
}

type FieldType struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Type        string `json:"type" validate:"required,oneof=string int float bool"`
	Description string `json:"description" validate:"required,min=2,max=150"`
}

func (h *RigelHandler) createSchemaVersion(c *gin.Context) {
	h.lh.Log("info", "createSchemaVersion called")
	var schemaVersion CreateSchemaVersionRequest

	// step 1: bind request body to struct
	err := wscutils.BindJson(c, &schemaVersion)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse("invalid_req_body", "invalid_req_body"))
		return
	}

	// step 2: validate request body
	validationErrors := h.validateCreateSchemaVersion(schemaVersion, c)
	if len(validationErrors) > 0 {
		h.lh.Log("error", "validation error", validationErrors)
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}
}

func (h *RigelHandler) validateCreateSchemaVersion(schemaVersion CreateSchemaVersionRequest, c *gin.Context) []wscutils.ErrorMessage {
	var validationErrors []wscutils.ErrorMessage

	// step 2.1: validate request body
	validationErrors = wscutils.WscValidate(schemaVersion, schemaVersion.getValsForCreateSchemaVersionError)

	// if there are standard validation errors, return
	// do not execute custom validations
	if len(validationErrors) > 0 {
		return validationErrors
	}

	// step 2.2: custom validations
	// check if schema_id exists
	schemExists, err := h.sqlq.CheckSchemaExists(c, int32(schemaVersion.SchemaID))
	if err != nil {
		validationErrors = append(validationErrors, wscutils.BuildErrorMessage("internal_error", "internal_error"))
		return validationErrors
	}

	if !schemExists {
		validationErrors = append(validationErrors, wscutils.BuildErrorMessage("schema_id", "schema_id does not exist"))
		return validationErrors
	}

	return validationErrors

}

func (sv *CreateSchemaVersionRequest) getValsForCreateSchemaVersionError(err validator.FieldError) []string {
	var vals []string
	errField := err.Field()
	fmt.Println("errField", errField)
	switch err.Field() {
	case "Version":
		switch err.Tag() {
		case "min":
			vals = append(vals, "2")
			vals = append(vals, strconv.Itoa(len(sv.Version))) // provided value that failed validation
		case "max":
			vals = append(vals, "50")
			vals = append(vals, strconv.Itoa(len(sv.Version))) // provided value that failed validation
		}
	case "Type":
		switch err.Tag() {
		case "oneof":
			vals = append(vals, "string, int, float, bool")
			vals = append(vals, err.Value().(string))
		}

	}
	return vals

}
