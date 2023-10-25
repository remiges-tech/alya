package rigel

import (
	"database/sql"
	"encoding/json"
	"go-framework/internal/pg/sqlc-gen"
	"go-framework/internal/wscutils"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/sqlc-dev/pqtype"
)

type Config struct {
	Name        string  `'json:"name" binding:"required" validate:"required,min=2,max=150"`
	Description *string `validate:"omitempty,min=2,max=150"`
	Active      *bool   `validate:"omitempty"`
	SchemaID    int32   `json:"schema_id" validate:"required"`
	Values      []Value `json:"values" validate:"required,dive"` // Add this line
	Tags        *[]Tag  `validate:"omitempty,dive"`
}

type Value struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type Tag struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type ConfigResponse struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	SchemaID    int32     `json:"schema_id"`
	Active      bool      `json:"active"`
	Description string    `json:"description,omitempty"`
	Tags        []Tag     `json:"tags,omitempty"`
	Values      []Value   `json:"values"`
	CreatedBy   string    `json:"created_by"`
	UpdatedBy   string    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *RigelHandler) createConfig(c *gin.Context) {
	h.lh.Log("info", "createConfig called")
	var config Config
	var createConfigParams sqlc.CreateConfigParams

	// Get the RequestUser from the gin context
	// TODO: remove the following line. The value will be set by the auth middleware.
	c.Set("RequestUser", "test")
	requestUserStr, err := wscutils.GetRequestUser(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse(err.Error(), err.Error()))
		return
	}

	// step 1: bind request body to struct
	err = wscutils.BindJson(c, &config)

	if err != nil {
		c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse("invalid_req_body", "invalid_req_body"))
		return
	}

	// step 2: validate request body
	validationErrors := validateConfigCreate(config)

	// step 3: if there are validation errors, add them to response and send it
	if len(validationErrors) > 0 {
		h.lh.Log("error", "validation error", validationErrors)
		c.JSON(http.StatusBadRequest, wscutils.NewResponse(wscutils.ErrorStatus, nil, validationErrors))
		return
	}

	// step 4: process the request

	tagsJson, err := json.Marshal(config.Tags)
	if err != nil {
		log.Printf("Error marshalling tags: %v", err)
	}

	valuesJson, err := json.Marshal(config.Values)
	if err != nil {
		log.Printf("Error marshalling values: %v", err)
	}

	// Create and initialize createConfigParams with data from the config struct
	createConfigParams = sqlc.CreateConfigParams{
		Name: config.Name,
		Description: sql.NullString{
			String: func() string {
				if config.Description != nil {
					return *config.Description
				}
				return ""
			}(),
			Valid: config.Description != nil,
		},
		Tags: pqtype.NullRawMessage{
			RawMessage: tagsJson,
			Valid:      len(tagsJson) > 0,
		},
		Values: valuesJson,
		Active: sql.NullBool{
			Bool: func() bool {
				if config.Active != nil {
					return *config.Active
				}
				return false
			}(),
			Valid: config.Active != nil,
		},
		SchemaID:  config.SchemaID,
		CreatedBy: requestUserStr,
		UpdatedBy: requestUserStr,
	}

	// Call the SQLC generated function to insert the config
	newConfig, err := h.sqlq.CreateConfig(c, createConfigParams)
	if err != nil {
		// log the error
		h.lh.Log("error", "error creating config", err.Error())
		// buildvalidationerror for something went wrong
		c.JSON(http.StatusInternalServerError, wscutils.NewErrorResponse("database_error", "error_creating_config"))

		return
	}

	configResponse := ConvertToConfigResponse(newConfig)

	// step 5: if there are no errors, send success response
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, configResponse, []wscutils.ErrorMessage{}))
}

func validateConfigCreate(config Config) []wscutils.ErrorMessage {
	// step 2.1: validate request body using standard validator
	validationErrors := wscutils.WscValidate(config, config.getValsForConfigError)

	// if there are standard validation errors, return
	// do not execute custom validations
	if len(validationErrors) > 0 {
		return validationErrors
	}

	// step 2.3: check request specific custom validations and add errors
	// we do not need any custom validation for createConfig

	return validationErrors
}

// getValsForUserError returns a slice of strings to be used as vals for a validation error.
// The vals are determined based on the field and the validation rule that failed.
func (c *Config) getValsForConfigError(err validator.FieldError) []string {
	var vals []string

	// check fields (err.Field()) and its validation rule (err.Tag()) that failed
	switch err.Field() {
	case "Name":
		switch err.Tag() {
		case "min":
			vals = append(vals, "2")
			vals = append(vals, strconv.Itoa(len(c.Name))) // provided value that failed validation
		case "max":
			vals = append(vals, "150")
			vals = append(vals, strconv.Itoa(len(c.Name))) // provided value that failed validation
		}
		// Add more cases as needed
	}
	return vals
}

func ConvertToConfigResponse(config sqlc.Config) ConfigResponse {
	var tags []Tag
	if err := json.Unmarshal(config.Tags.RawMessage, &tags); err != nil {
		log.Printf("Error unmarshalling tags: %v", err)
	}

	var values []Value
	if err := json.Unmarshal(config.Values, &values); err != nil {
		log.Printf("Error unmarshalling values: %v", err)
	}

	return ConfigResponse{
		ID:          int64(config.ID),
		Name:        config.Name,
		Active:      config.Active.Bool,
		Description: config.Description.String,
		Tags:        tags,
		Values:      values,
		CreatedBy:   config.CreatedBy,
		UpdatedBy:   config.UpdatedBy,
		CreatedAt:   config.CreatedAt.Time,
		UpdatedAt:   config.UpdatedAt.Time,
	}
}

func (h *RigelHandler) getConfig(c *gin.Context) {
	configID := c.Query("config_id")
	configName := c.Query("config_name")
	schemaName := c.Query("schema_name")

	var config sqlc.Config
	var qerr error
	if configID != "" {
		id, err := strconv.Atoi(configID)
		if err != nil {
			c.JSON(http.StatusBadRequest, wscutils.NewErrorResponse("invalid_config_id", "invalid_config_id"))
			return
		}
		config, qerr = h.sqlq.GetConfig(c, int32(id))
	}
	if configName != "" && schemaName != "" {
		getConfigByNameAndSchemaParams := sqlc.GetConfigByNameAndSchemaParams{
			ConfigName: configName,
			SchemaName: schemaName,
		}
		config, qerr = h.sqlq.GetConfigByNameAndSchema(c, getConfigByNameAndSchemaParams)
		configResponse := ConvertToConfigResponse(config)
		c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, configResponse, []wscutils.ErrorMessage{}))
		return
	}

	// Check the error and respond accordingly
	if qerr != nil {
		if qerr == sql.ErrNoRows {
			// If there is no such config, we should return an empty JSON
			c.JSON(http.StatusNotFound, wscutils.NewErrorResponse("config_not_found", "The requested config could not be found."))
		} else {
			c.JSON(http.StatusInternalServerError, wscutils.NewErrorResponse("database_error", "error_getting_config"))
		}
		return
	}

	configResponse := ConvertToConfigResponse(config)
	c.JSON(http.StatusOK, wscutils.NewResponse(wscutils.SuccessStatus, configResponse, []wscutils.ErrorMessage{}))
	return
}
