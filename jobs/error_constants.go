package jobs

// Error message IDs for configuration-related errors
const (
// Message IDs for configuration errors
MsgIDProcessorNotFound         = 2 // Processor function not found
MsgIDInvalidProcessorType      = 3 // Invalid processor type
MsgIDInitializerNotFound       = 4 // Initializer not found
MsgIDInitializerFailed         = 5 // Initializer function failed
MsgIDUnknownConfigurationError = 6 // Unknown configuration error

// Message IDs for other error types
MsgIDProcessingError           = 10 // General processing error
)

// Error codes for machine-to-machine communication
const (
// All configuration errors use the same error code for consistent handling
ErrCodeConfiguration = "configerror"

// Standard error code for processing errors
ErrCodeProcessing = "processing_error"
)
