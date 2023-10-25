package rigel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/wscutils"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type CreateSchemaVersionRequest struct {
	SchemaID int     `json:"schema_id" validate:"required"`
	Version  string  `json:"version"  validate:"required,min=2,max=50"`
	Fields   []Field `json:"fields"  validate:"required,dive"`
}

type Field struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	Type        string `json:"type" validate:"required,oneof=string int float bool"`
	Description string `json:"description" validate:"required,min=2,max=150"`
}

type SchemaVersionResponse struct {
	ID        int64     `json:"id"`
	SchemaID  int32     `json:"schema_id"`
	Version   string    `json:"version"`
	Fields    []Field   `json:"fields"`
	CreatedBy string    `json:"created_by"`
	UpdatedBy string    `json:"updated_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *RigelHandler) createSchemaVersion(c *gin.Context) {
	h.lh.Log("info", "createSchemaVersion called")
	// TODO: remove the following line. The value will be set by the auth middleware.
	c.Set("RequestUser", "test")
	requestUser, err := wscutils.GetRequestUser(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse(err.Error(), err.Error()))
		return
	}

	var schemaVersion CreateSchemaVersionRequest

	// step 1: bind request body to struct
	err = wscutils.BindJson(c, &schemaVersion)
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

	// step 3: process request
	// convert schemaVersion to sqlc.CreateSchemaVersionParams
	// Convert the fields to JSON
	fieldsJson, err := json.Marshal(schemaVersion.Fields)
	if err != nil {
		log.Printf("error marshalling fields: %v", err)
	}

	// Create the parameters for the CreateSchemaVersion function
	createSchemaVersionParams := sqlc.CreateSchemaVersionParams{
		SchemaID:  sql.NullInt32{Int32: int32(schemaVersion.SchemaID), Valid: true},
		Version:   schemaVersion.Version,
		Fields:    fieldsJson,
		CreatedBy: requestUser,
		UpdatedBy: requestUser,
	}

	// Call the CreateSchemaVersion function
	ctx := context.TODO() // perhaps we should create one from the gin context
	newSchemaVersion, err := h.sqlq.CreateSchemaVersion(ctx, createSchemaVersionParams)
	if err != nil {
		fmt.Println("error creating schema version", err)
		c.JSON(http.StatusInternalServerError, wscutils.NewErrorResponse("internal_error", "internal_error"))
		return
	}

	// Convert newSchemaVersion to SchemaVersionResponse
	schemaVersionResponse := ConvertToSchemaVersionResponse(newSchemaVersion)

	// step 4: send response
	c.JSON(http.StatusOK, wscutils.NewSuccessResponse(schemaVersionResponse))
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

func ConvertToSchemaVersionResponse(schemaVersion sqlc.SchemaVersion) SchemaVersionResponse {
	var fields []Field
	if err := json.Unmarshal(schemaVersion.Fields, &fields); err != nil {
		log.Printf("Error unmarshalling fields: %v", err)
	}
	return SchemaVersionResponse{
		ID:        int64(schemaVersion.ID),
		SchemaID:  schemaVersion.SchemaID.Int32,
		Version:   schemaVersion.Version,
		Fields:    fields,
		CreatedBy: schemaVersion.CreatedBy,
		UpdatedBy: schemaVersion.UpdatedBy,
		CreatedAt: schemaVersion.CreatedAt.Time,
		UpdatedAt: schemaVersion.UpdatedAt.Time,
	}
}
