package wscutils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func setup() {
	// Set default message ID and error code for validation errors
	SetDefaultMsgID(9999)
	SetDefaultErrCode("default_error")

	// Set a custom message ID for invalid JSON errors
	SetMsgIDInvalidJSON(1001)
	SetErrCodeInvalidJSON("invalid_json")

	// Register any other necessary mappings for validation tags to message IDs and error codes
	customValidationMap := map[string]int{
		"required": 1001,
		"email":    1002,
		"min":      1003,
		"max":      1004,
	}
	SetValidationTagToMsgIDMap(customValidationMap)

	customErrCodeMap := map[string]string{
		"required": "required",
		"email":    "email",
		"min":      "min",
		"max":      "max",
	}
	SetValidationTagToErrCodeMap(customErrCodeMap)
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

type TestUser struct {
	Name  string `validate:"required"`
	Email string `validate:"required,email"`
	Age   int    `validate:"min=18,max=150"`
}

func TestSendSuccessResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	test := struct {
		name     string
		response *Response
		expected string
	}{
		name:     "Success response",
		response: NewSuccessResponse("test data"),
		expected: `{"status":"success","data":"test data","messages":null}`,
	}

	t.Run(test.name, func(t *testing.T) {
		// Create a response recorder
		w := httptest.NewRecorder()

		// Create a gin context with the response recorder as the writer
		c, _ := gin.CreateTestContext(w)

		// Call the function with a test response
		SendSuccessResponse(c, test.response)

		// Assert that the response body was correctly set
		assert.Equal(t, test.expected, w.Body.String())
	})
}

func TestSendErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Assuming ErrMsgIDInvalidJson is defined and represents the message ID for invalid JSON errors.
	msgID := msgIDInvalidJSON

	test := struct {
		name     string
		response *Response
		expected string
	}{
		name: "Error response",
		// Updated to include the msgID parameter.
		response: NewErrorResponse(msgID, ErrcodeInvalidJson),
		expected: `{"status":"error","data":null,"messages":[{"msgid":` + strconv.Itoa(msgID) + `,"errcode":"invalid_json"}]}`,
	}

	t.Run(test.name, func(t *testing.T) {
		// Create a response recorder
		w := httptest.NewRecorder()

		// Create a gin context with the response recorder as the writer
		c, _ := gin.CreateTestContext(w)

		// Call the function with a test response
		SendErrorResponse(c, test.response)

		// Assert that the response body was correctly set
		assert.Equal(t, test.expected, w.Body.String())
	})
}

// Adjusted getVals to return multiple values for a hypothetical "MultiValField".
func getVals(err validator.FieldError) []string {
	if err.Field() == "Age" {
		return []string{"10", "18-65"}
	}
	return []string{err.Field()}
}

const DefaultMsgID = 9999

func TestWscValidate(t *testing.T) {
	// Define test cases
	tests := []struct {
		name    string
		input   TestUser
		wantErr bool
		errMsgs []ErrorMessage // Expected error messages
	}{
		{
			name:    "Valid input",
			input:   TestUser{Name: "John Doe", Email: "john@example.com", Age: 18},
			wantErr: false,
			errMsgs: nil,
		},
		{
			name:    "Missing name",
			input:   TestUser{Email: "john@example.com", Age: 20},
			wantErr: true,
			errMsgs: []ErrorMessage{{MsgID: 1001, ErrCode: "required", Field: "Name", Vals: []string{"Name"}}},
		},
		{
			name:    "Invalid email",
			input:   TestUser{Name: "John Doe", Email: "not-an-email", Age: 20},
			wantErr: true,
			errMsgs: []ErrorMessage{{MsgID: 1002, ErrCode: "email", Field: "Email", Vals: []string{"Email"}}},
		},
		{
			name:    "Field with multiple values",
			input:   TestUser{Name: "MultiValField", Email: "john@example.com", Age: 10},
			wantErr: true,
			errMsgs: []ErrorMessage{
				{
					MsgID:   1003,
					ErrCode: "min",
					Field:   "Age",
					Vals:    []string{"10", "18-65"},
				},
			},
		},
	}

	// Iterate over test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsgs := WscValidate(tt.input, getVals)

			if (len(errMsgs) > 0) != tt.wantErr {
				t.Errorf("WscValidate() error = %v, wantErr %v", len(errMsgs) > 0, tt.wantErr)
			}

			if !reflect.DeepEqual(errMsgs, tt.errMsgs) {
				t.Errorf("WscValidate() got %v, want %v", errMsgs, tt.errMsgs)
			}
		})
	}
}

// Helper function to get a pointer to a string (for Field in ErrorMessage).
func pointerToString(s string) *string {
	return &s
}

func TestBindJSON_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Define a struct that matches the expected data structure.
	type TestData struct {
		Name string `json:"name"`
	}

	// Define test cases
	tests := []struct {
		name    string
		jsonStr string
		want    TestData
	}{
		{
			name:    "Single field",
			jsonStr: `{"data": {"name": "John Doe"}}`,
			want:    TestData{Name: "John Doe"},
		},
		{
			name:    "Empty name",
			jsonStr: `{"data": {"name": ""}}`,
			want:    TestData{Name: ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate a JSON request body
			body := bytes.NewBufferString(tc.jsonStr)
			req, _ := http.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// The data variable where the JSON will be bound.
			var data TestData

			// Call BindJSON with the context and the data variable.
			err := BindJSON(c, &data)

			// Assert that there is no error and the data is correctly bound.
			assert.Nil(t, err)
			assert.Equal(t, tc.want, data)
		})
	}
}

func TestBindJSON_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Define test cases
	tests := []struct {
		name         string
		jsonStr      string
		expectedCode int
		expectedBody string
	}{
		{
			name:         "Incorrect Structure",
			jsonStr:      `{"data": "invalid JSON"}`,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"status":"error","data":null,"messages":[{"msgid":1001,"errcode":"invalid_json"}]}`,
		},
		{
			name:         "Malformed JSON",
			jsonStr:      `{"data": }`,
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"status":"error","data":null,"messages":[{"msgid":1001,"errcode":"invalid_json"}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate a JSON request body
			body := bytes.NewBufferString(tc.jsonStr)
			req, _ := http.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Attempt to bind JSON to a Request struct
			_ = BindJSON(c, &Request{})

			// Assert that the response code and body are as expected
			assert.Equal(t, tc.expectedCode, w.Code)
			assert.JSONEq(t, tc.expectedBody, w.Body.String())
		})
	}
}

// The following four test functions thoroughly test the Optional[T] generic type's functionality.
// Each test has a specific purpose:
// 1. TestOptionalUnmarshalJSON tests the basic unmarshaling mechanism with string values.
// 2. TestOptionalWithDifferentTypes verifies the type works with various Go data types (int, bool, struct).
// Above tests test Unmarshal function directly -- not through json.Unmarshal
// Below tests test Unmarshal function through json.Unmarshal
// 3. TestOptionalInStruct checks real-world usage when Optional fields are embedded in structs.
// 4. TestOptionalWithComplexTypes validates handling of advanced data structures (slices, maps, nested objects).

// TestOptionalUnmarshalJSON tests the basic behavior of the Optional.UnmarshalJSON method
// Tests Unmarshal function directly -- not through json.Unmarshal
func TestOptionalUnmarshalJSON(t *testing.T) {
	// Define test cases in a table-driven test format
	tests := []struct {
		name        string
		jsonData    string
		wantPresent bool
		wantNull    bool
		wantValue   string
		wantErr     bool
	}{
		{
			name:        "Field with value",
			jsonData:    `"test value"`,
			wantPresent: true,
			wantNull:    false,
			wantValue:   "test value",
			wantErr:     false,
			// Tests that Optional correctly handles JSON strings by verifying the unmarshaled value matches the input
			// and flags are set correctly (Present=true, Null=false)
		},
		{
			name:        "Field with null",
			jsonData:    `null`,
			wantPresent: true,
			wantNull:    true,
			wantValue:   "", // Zero value for string
			wantErr:     false,
			// Tests that Optional correctly handles null JSON values by verifying the IsNull flag is set
			// and the Value field contains the zero value for the type
		},
		{
			name:        "Invalid JSON",
			jsonData:    `{invalid json}`,
			wantPresent: false,
			wantNull:    false,
			wantValue:   "",
			wantErr:     true,
			// Tests that Optional correctly handles invalid JSON input by returning an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an Optional of type string for testing
			var opt Optional[string]

			// Convert the JSON string to bytes
			data := []byte(tt.jsonData)

			// Call UnmarshalJSON
			err := opt.UnmarshalJSON(data)

			// Check if error was expected
			if (err != nil) != tt.wantErr {
				t.Errorf("Optional.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we're expecting an error, don't check the other fields
			if tt.wantErr {
				return
			}

			// Check if Present field is as expected
			if opt.Present != tt.wantPresent {
				t.Errorf("Optional.Present = %v, want %v", opt.Present, tt.wantPresent)
			}

			// Check if Null field is as expected
			if opt.Null != tt.wantNull {
				t.Errorf("Optional.Null = %v, want %v", opt.Null, tt.wantNull)
			}

			// Check if Value is as expected (only if not null)
			if !tt.wantNull && opt.Value != tt.wantValue {
				t.Errorf("Optional.Value = %v, want %v", opt.Value, tt.wantValue)
			}
		})
	}
}

// Test different data types with Optional
// Tests Unmarshal function directly -- not through json.Unmarshal
func TestOptionalWithDifferentTypes(t *testing.T) {
	// Define Person struct for use in multiple test cases
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	// Test with integer type
	t.Run("Optional with int", func(t *testing.T) {
		// Tests that Optional correctly handles primitive integer values
		// by verifying the unmarshaled value matches the input and flags are set correctly
		var intOpt Optional[int]
		err := intOpt.UnmarshalJSON([]byte(`42`))
		assert.NoError(t, err)
		assert.True(t, intOpt.Present)
		assert.False(t, intOpt.Null)
		assert.Equal(t, 42, intOpt.Value)
	})

	// Test with boolean type
	t.Run("Optional with bool", func(t *testing.T) {
		// Tests that Optional correctly handles boolean values
		// by verifying the unmarshaled boolean value and flags are set correctly
		var boolOpt Optional[bool]
		err := boolOpt.UnmarshalJSON([]byte(`true`))
		assert.NoError(t, err)
		assert.True(t, boolOpt.Present)
		assert.False(t, boolOpt.Null)
		assert.Equal(t, true, boolOpt.Value)
	})

	// Test with struct type
	t.Run("Optional with struct", func(t *testing.T) {
		// Tests that Optional correctly handles complex struct types
		// by verifying the struct is properly unmarshaled with all its fields
		var structOpt Optional[Person]
		err := structOpt.UnmarshalJSON([]byte(`{"name":"John","age":30}`))
		assert.NoError(t, err)
		assert.True(t, structOpt.Present)
		assert.False(t, structOpt.Null)
		assert.Equal(t, Person{Name: "John", Age: 30}, structOpt.Value)
	})

	// Test with null for all types
	t.Run("Optional with null", func(t *testing.T) {
		// Tests that Optional correctly handles null values for different types
		// by verifying Null flag is set and the Value field contains the zero value

		// For int
		var intOpt Optional[int]
		errInt := intOpt.UnmarshalJSON([]byte(`null`))
		assert.NoError(t, errInt)
		assert.True(t, intOpt.Present)
		assert.True(t, intOpt.Null)
		assert.Equal(t, 0, intOpt.Value)

		// For struct
		var structOpt Optional[Person]
		errStruct := structOpt.UnmarshalJSON([]byte(`null`))
		assert.NoError(t, errStruct)
		assert.True(t, structOpt.Present)
		assert.True(t, structOpt.Null)
		assert.Equal(t, Person{}, structOpt.Value)
	})
}

// Test practical usage in a struct with JSON unmarshaling
func TestOptionalInStruct(t *testing.T) {
	type User struct {
		ID    int              `json:"id"`
		Name  string           `json:"name"`
		Email Optional[string] `json:"email"`
		Age   Optional[int]    `json:"age"`
	}

	jsonTests := []struct {
		name      string
		jsonData  string
		wantUser  User
		wantEmail bool
		wantAge   bool
		emailNull bool
		ageNull   bool
	}{
		{
			name:      "All fields present",
			jsonData:  `{"id":1,"name":"John","email":"john@example.com","age":30}`,
			wantUser:  User{ID: 1, Name: "John"},
			wantEmail: true,
			wantAge:   true,
			emailNull: false,
			ageNull:   false,
			// Tests that all fields are correctly unmarshaled when present in the JSON
			// and Optional fields have Present=true, Null=false with proper values
		},
		{
			name:      "Email missing",
			jsonData:  `{"id":2,"name":"Jane","age":25}`,
			wantUser:  User{ID: 2, Name: "Jane"},
			wantEmail: false,
			wantAge:   true,
			emailNull: false,
			ageNull:   false,
			// Tests that missing fields are correctly detected with Present=false
			// while other fields are properly unmarshaled
		},
		{
			name:      "Age null",
			jsonData:  `{"id":3,"name":"Bob","email":"bob@example.com","age":null}`,
			wantUser:  User{ID: 3, Name: "Bob"},
			wantEmail: true,
			wantAge:   true,
			emailNull: false,
			ageNull:   true,
			// Tests that explicit null values are correctly detected with
			// Present=true, Null=true while other fields are properly unmarshaled
		},
		{
			name:      "Both email and age null",
			jsonData:  `{"id":4,"name":"Alice","email":null,"age":null}`,
			wantUser:  User{ID: 4, Name: "Alice"},
			wantEmail: true,
			wantAge:   true,
			emailNull: true,
			ageNull:   true,
			// Tests that multiple explicit null values are correctly detected
			// and all flags are set appropriately
		},
	}

	for _, tt := range jsonTests {
		t.Run(tt.name, func(t *testing.T) {
			var user User
			err := json.Unmarshal([]byte(tt.jsonData), &user)
			assert.NoError(t, err)

			// Check ID and Name
			assert.Equal(t, tt.wantUser.ID, user.ID)
			assert.Equal(t, tt.wantUser.Name, user.Name)

			// Check Email present/null status
			assert.Equal(t, tt.wantEmail, user.Email.Present)
			assert.Equal(t, tt.emailNull, user.Email.Null)

			// Check Age present/null status
			assert.Equal(t, tt.wantAge, user.Age.Present)
			assert.Equal(t, tt.ageNull, user.Age.Null)

			// Check specific values if present and not null
			if tt.wantEmail && !tt.emailNull {
				assert.NotEmpty(t, user.Email.Value)
			}
			if tt.wantAge && !tt.ageNull {
				assert.NotZero(t, user.Age.Value)
			}
		})
	}
}

// TestOptionalWithComplexTypes tests the Optional type with more complex types like slices, maps, and structs
func TestOptionalWithComplexTypes(t *testing.T) {
	type Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
		Zip    string `json:"zip"`
	}

	type User struct {
		ID         int                      `json:"id"`
		Name       string                   `json:"name"`
		Tags       Optional[[]string]       `json:"tags"`
		Address    Optional[Address]        `json:"address"`
		Properties Optional[map[string]any] `json:"properties"`
	}

	tests := []struct {
		name          string
		jsonData      string
		wantTags      bool
		wantAddress   bool
		wantProps     bool
		tagsNull      bool
		addressNull   bool
		propsNull     bool
		expectedTags  []string
		expectedAddr  Address
		expectedProps map[string]any
	}{
		{
			name:          "All complex fields present",
			jsonData:      `{"id":1,"name":"John","tags":["developer","golang"],"address":{"street":"123 Main St","city":"San Francisco","zip":"94105"},"properties":{"active":true,"level":5,"score":98.6}}`,
			wantTags:      true,
			wantAddress:   true,
			wantProps:     true,
			tagsNull:      false,
			addressNull:   false,
			propsNull:     false,
			expectedTags:  []string{"developer", "golang"},
			expectedAddr:  Address{Street: "123 Main St", City: "San Francisco", Zip: "94105"},
			expectedProps: map[string]any{"active": true, "level": float64(5), "score": 98.6},
			// Tests that complex nested data structures (slice, struct, map) are correctly unmarshaled
			// with all values properly set and Optional flags indicating Present=true, Null=false
		},
		{
			name:          "Empty slice",
			jsonData:      `{"id":2,"name":"Alice","tags":[],"address":{"street":"456 Oak St","city":"New York","zip":"10001"},"properties":{"active":false}}`,
			wantTags:      true,
			wantAddress:   true,
			wantProps:     true,
			tagsNull:      false,
			addressNull:   false,
			propsNull:     false,
			expectedTags:  []string{},
			expectedAddr:  Address{Street: "456 Oak St", City: "New York", Zip: "10001"},
			expectedProps: map[string]any{"active": false},
			// Tests that empty collections like [] are correctly detected as Present=true, Null=false
			// but with an empty slice/array as the value
		},
		{
			name:        "Null fields",
			jsonData:    `{"id":3,"name":"Bob","tags":null,"address":null,"properties":null}`,
			wantTags:    true,
			wantAddress: true,
			wantProps:   true,
			tagsNull:    true,
			addressNull: true,
			propsNull:   true,
			// Tests that explicit null values for complex types are correctly detected
			// with Present=true, Null=true for slices, structs, and maps
		},
		{
			name:        "Missing fields",
			jsonData:    `{"id":4,"name":"Carol"}`,
			wantTags:    false,
			wantAddress: false,
			wantProps:   false,
			tagsNull:    false,
			addressNull: false,
			propsNull:   false,
			// Tests that missing fields are correctly detected with Present=false
			// for all complex data types
		},
		{
			name:         "Mix of present, null, and missing",
			jsonData:     `{"id":5,"name":"Dave","tags":["admin"],"properties":null}`,
			wantTags:     true,
			wantAddress:  false,
			wantProps:    true,
			tagsNull:     false,
			addressNull:  false,
			propsNull:    true,
			expectedTags: []string{"admin"},
			// Tests a combination of states: present value (tags), explicit null (properties),
			// and missing field (address) to verify all three cases work together
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var user User
			err := json.Unmarshal([]byte(tt.jsonData), &user)
			assert.NoError(t, err)

			// Check Tags present/null status
			assert.Equal(t, tt.wantTags, user.Tags.Present, "Tags.Present")
			assert.Equal(t, tt.tagsNull, user.Tags.Null, "Tags.Null")

			// Check Address present/null status
			assert.Equal(t, tt.wantAddress, user.Address.Present, "Address.Present")
			assert.Equal(t, tt.addressNull, user.Address.Null, "Address.Null")

			// Check Properties present/null status
			assert.Equal(t, tt.wantProps, user.Properties.Present, "Properties.Present")
			assert.Equal(t, tt.propsNull, user.Properties.Null, "Properties.Null")

			// Check values if present and not null
			if tt.wantTags && !tt.tagsNull {
				assert.Equal(t, tt.expectedTags, user.Tags.Value)
			}

			if tt.wantAddress && !tt.addressNull {
				assert.Equal(t, tt.expectedAddr, user.Address.Value)
			}

			if tt.wantProps && !tt.propsNull {
				assert.Equal(t, tt.expectedProps, user.Properties.Value)
			}
		})
	}
}

// TestOptionalGet tests the Get method of the Optional type
func TestOptionalGet(t *testing.T) {
	tests := []struct {
		name    string
		opt     Optional[string]
		wantVal string
		wantOk  bool
	}{
		{
			name: "Present value",
			opt: Optional[string]{
				Value:   "test value",
				Present: true,
				Null:    false,
			},
			wantVal: "test value",
			wantOk:  true,
		},
		{
			name: "Null value",
			opt: Optional[string]{
				Value:   "", // zero value
				Present: true,
				Null:    true,
			},
			wantVal: "",
			wantOk:  false,
		},
		{
			name: "Not present",
			opt: Optional[string]{
				Value:   "", // zero value
				Present: false,
				Null:    false,
			},
			wantVal: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVal, gotOk := tt.opt.Get()

			if gotOk != tt.wantOk {
				t.Errorf("Optional.Get() ok = %v, want %v", gotOk, tt.wantOk)
			}

			if gotVal != tt.wantVal {
				t.Errorf("Optional.Get() value = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}
