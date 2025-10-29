// Package graph provides the core graph execution engine for LangGraph-Go.
package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Replay provides deterministic replay of graph executions.

// RecordedIO captures an external interaction (API call, database query, etc.).
// for deterministic replay without re-invoking the external service.
//
// During initial execution, RecordedIO instances are created for nodes with.
// SideEffectPolicy.Recordable=true. During replay, these recorded I/Os are.
// matched by (NodeID, Attempt) and their responses are returned directly.
// without executing the node.
//
// The Hash field enables mismatch detection: if a live execution produces.
// a different response hash than recorded, ErrReplayMismatch is raised,
// indicating non-deterministic behavior.
type RecordedIO struct {
	// NodeID identifies the node that performed this I/O operation.
	NodeID string `json:"node_id"`

	// Attempt is the retry attempt number this I/O corresponds to.
	// This allows matching I/O recordings to specific retry attempts.
	Attempt int `json:"attempt"`

	// Request is the serialized request data sent to the external service.
	// Stored as JSON for cross-language compatibility and human readability.
	Request json.RawMessage `json:"request"`

	// Response is the serialized response data received from the external service.
	// Stored as JSON for cross-language compatibility and human readability.
	Response json.RawMessage `json:"response"`

	// Hash is a SHA-256 hash of the response content, used for mismatch detection.
	// during replay. Format: "sha256:hex_encoded_hash".
	Hash string `json:"hash"`

	// Timestamp records when this I/O operation was captured.
	Timestamp time.Time `json:"timestamp"`

	// Duration is how long the I/O operation took to complete.
	// This can be used for performance analysis and replay simulation.
	Duration time.Duration `json:"duration"`
}

// recordIO captures an external I/O operation for deterministic replay.
//
// This function serializes the request and response to JSON, computes a SHA-256.
// hash of the response for mismatch detection, and creates a RecordedIO structure.
// that can be stored in a checkpoint.
//
// During initial execution, nodes with SideEffectPolicy.Recordable=true should.
// call this function to record their external interactions. During replay, these.
// recordings are looked up by (nodeID, attempt) tuple and replayed without.
// re-executing the external call.
//
// Parameters:
// - nodeID: The identifier of the node performing the I/O.
// - attempt: The retry attempt number (0 for first execution).
// - request: The request data to be serialized.
// - response: The response data to be serialized and hashed.
//
// Returns:
// - RecordedIO: The complete I/O recording with hash.
// - error: Serialization error if request or response cannot be marshaled to JSON.
//
// Example usage in a node:
//
// recording, err := recordIO("api_call_node", 0, apiRequest, apiResponse).
// if err != nil {.
// return NodeResult[S]{Err: fmt.Errorf("failed to record I/O: %w", err)}.
// }.
// // Store recording in checkpoint via engine.
func recordIO(nodeID string, attempt int, request, response interface{}) (RecordedIO, error) {
	start := time.Now()

	// Serialize request to JSON.
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return RecordedIO{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Serialize response to JSON.
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return RecordedIO{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	// Compute SHA-256 hash of response for mismatch detection.
	hasher := sha256.New()
	hasher.Write(responseJSON)
	hashBytes := hasher.Sum(nil)
	hashStr := "sha256:" + hex.EncodeToString(hashBytes)

	duration := time.Since(start)

	return RecordedIO{
		NodeID:    nodeID,
		Attempt:   attempt,
		Request:   json.RawMessage(requestJSON),
		Response:  json.RawMessage(responseJSON),
		Hash:      hashStr,
		Timestamp: time.Now(),
		Duration:  duration,
	}, nil
}

// lookupRecordedIO retrieves a previously recorded I/O operation by (nodeID, attempt) tuple.
//
// During replay mode, the engine uses this function to find recorded responses.
// instead of re-executing nodes. The recordings are indexed by the combination.
// of node ID and retry attempt number, allowing the same node to have different.
// recordings for different retry attempts.
//
// Parameters:
// - recordings: Slice of recorded I/O operations from a checkpoint.
// - nodeID: The identifier of the node to look up.
// - attempt: The retry attempt number to look up.
//
// Returns:
// - RecordedIO: The matched recording if found.
// - bool: true if a matching recording was found, false otherwise.
//
// Example usage in replay mode:
//
// recording, found := lookupRecordedIO(checkpoint.RecordedIOs, "api_call_node", 0).
// if found {.
// // Use recording.Response instead of executing node.
// var response MyResponse.
// json.Unmarshal(recording.Response, &response).
// }.
func lookupRecordedIO(recordings []RecordedIO, nodeID string, attempt int) (RecordedIO, bool) {
	// Linear search through recordings to find matching (nodeID, attempt).
	for _, rec := range recordings {
		if rec.NodeID == nodeID && rec.Attempt == attempt {
			return rec, true
		}
	}
	return RecordedIO{}, false
}

// verifyReplayHash validates that a live execution response matches a recorded response.
//
// During replay verification mode (when StrictReplay=true), this function compares.
// the SHA-256 hash of the actual response with the hash from the recorded I/O.
// If they differ, it indicates non-deterministic behavior in the node.
//
// Common causes of replay mismatch:
// - Node using random number generation without seeded RNG.
// - Node reading system time (use context-provided values instead).
// - Node depending on external state that has changed.
// - Node using map iteration order (non-deterministic in Go).
//
// Parameters:
// - recorded: The recorded I/O with the expected response hash.
// - actualResponse: The actual response from live execution.
//
// Returns:
// - error: ErrReplayMismatch if hashes don't match, nil if they match.
//
// Example usage:
//
// if err := verifyReplayHash(recorded, actualResponse); err != nil {.
// return NodeResult[S]{Err: fmt.Errorf("replay verification failed: %w", err)}.
// }.
func verifyReplayHash(recorded RecordedIO, actualResponse interface{}) error {
	// Serialize actual response to JSON.
	actualJSON, err := json.Marshal(actualResponse)
	if err != nil {
		return fmt.Errorf("failed to marshal actual response: %w", err)
	}

	// Compute SHA-256 hash of actual response.
	hasher := sha256.New()
	hasher.Write(actualJSON)
	hashBytes := hasher.Sum(nil)
	actualHash := "sha256:" + hex.EncodeToString(hashBytes)

	// Compare with recorded hash.
	if actualHash != recorded.Hash {
		return fmt.Errorf("%w: expected %s, got %s", ErrReplayMismatch, recorded.Hash, actualHash)
	}

	return nil
}
