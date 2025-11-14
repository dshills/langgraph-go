package bedrock

import "fmt"

// BedrockError wraps AWS Bedrock-specific errors with actionable context.
//
// Provides retry classification and regional context for error handling.
// Used to determine whether to retry requests and which fallback strategies to use.
type BedrockError struct {
	// Code is the AWS error code (e.g., "ThrottlingException", "AccessDeniedException").
	Code string

	// Message is a human-readable error description.
	Message string

	// RequestID is the AWS request ID for debugging with AWS support.
	RequestID string

	// Region is the AWS region where the error occurred.
	Region string

	// Retryable indicates if this error is transient and safe to retry.
	// True for: ThrottlingException, ModelTimeoutException, InternalServerException
	// False for: AccessDeniedException, ValidationException, ModelNotReadyException
	Retryable bool

	// OriginalError is the underlying AWS SDK error.
	OriginalError error
}

// Error implements the error interface.
func (e *BedrockError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("bedrock error [%s] in region %s (request: %s): %s",
			e.Code, e.Region, e.RequestID, e.Message)
	}
	return fmt.Sprintf("bedrock error [%s] in region %s: %s",
		e.Code, e.Region, e.Message)
}

// Unwrap returns the original error for errors.Is/As compatibility.
func (e *BedrockError) Unwrap() error {
	return e.OriginalError
}

// Common Bedrock error codes for reference:
//
// Retryable errors (transient):
// - ThrottlingException: Rate limit exceeded, retry with backoff
// - ModelTimeoutException: Model inference timeout, retry may succeed
// - InternalServerException: AWS service error, retry may succeed
//
// Non-retryable errors (permanent):
// - AccessDeniedException: IAM permissions issue, fix credentials
// - ValidationException: Invalid request parameters, fix request
// - ModelNotReadyException: Model not available in region, use different region/model
// - ServiceQuotaExceededException: Account quota exceeded, request quota increase
// - ResourceNotFoundException: Model or endpoint not found, check model ID

// wrapAWSError converts an AWS SDK error to BedrockError with retry classification.
//
// Examines the error type and code to determine:
// - Error code and message
// - Request ID for debugging
// - Whether the error is retryable
//
// Returns nil if err is nil.
func wrapAWSError(err error, region string) *BedrockError {
	if err == nil {
		return nil
	}

	// Extract error code from error message
	// AWS SDK errors typically include error code in message like:
	// "ThrottlingException: Rate exceeded" or "ValidationException: Invalid input"
	errMsg := err.Error()
	code := extractErrorCode(errMsg)

	return &BedrockError{
		Code:          code,
		Message:       errMsg,
		Region:        region,
		Retryable:     isRetryableError(code),
		OriginalError: err,
	}
}

// extractErrorCode extracts the AWS error code from an error message.
// AWS SDK errors typically format as "ErrorCode: message"
func extractErrorCode(errMsg string) string {
	// Look for pattern "ErrorCode: " at start
	for i := 0; i < len(errMsg); i++ {
		if errMsg[i] == ':' {
			return errMsg[:i]
		}
	}
	return "UnknownException"
}

// isRetryableError determines if an AWS error code represents a retryable error.
//
// Retryable errors are transient and may succeed on retry:
// - ThrottlingException: Rate limiting, retry with backoff
// - ServiceUnavailableException: AWS service temporarily unavailable
// - ModelTimeoutException: Model inference timeout
// - ModelNotReadyException: Model still loading
// - InternalServerException: AWS internal error
//
// Non-retryable errors are permanent and require user action:
// - ValidationException: Invalid request parameters
// - AccessDeniedException: IAM permissions issue
// - ResourceNotFoundException: Model/endpoint not found
// - ModelErrorException: Model failed to generate response
func isRetryableError(errorCode string) bool {
	switch errorCode {
	case "ThrottlingException",
		"ServiceUnavailableException",
		"ModelTimeoutException",
		"ModelNotReadyException",
		"InternalServerException":
		return true
	default:
		return false
	}
}
