package wscutils

const (
	//HTTP responses
	ErrorStatus   = "error"
	SuccessStatus = "success"

	// Validation error messages
	RequiredError  = "required"
	InvalidEmail   = "email"
	EmailDomainErr = "emaildomain"

	// Validation Error Types
	InvalidJSON = "invalid_json"
	Unknown     = "unknown"

	// Ports
	//DefaultPort = ":8080" // this should come from config
)
