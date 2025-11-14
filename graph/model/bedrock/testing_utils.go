package bedrock

import "strings"

// isModelAccessError checks if the error indicates model access is not enabled.
//
// AWS Bedrock requires accounts to submit a use case form before accessing
// certain models (particularly Claude). This function detects these errors
// so tests can skip gracefully rather than fail.
//
// Common error patterns:
// - "Model use case details have not been submitted for this account"
// - "ResourceNotFoundException" (model not available in region)
// - "ModelNotReadyException" (model still being provisioned)
func isModelAccessError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Model use case details have not been submitted") ||
		strings.Contains(errStr, "ResourceNotFoundException") ||
		strings.Contains(errStr, "ModelNotReadyException")
}
