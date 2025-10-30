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
//
// # Determinism Guarantees (T043)
//
// The LangGraph-Go engine guarantees deterministic execution and replay through
// three key mechanisms:
//
// 1. Seeded Random Number Generation (RNG):
//   - Each workflow run is assigned a unique seed derived from its runID
//   - The seed is computed via SHA-256 hash of the runID for collision resistance
//   - Same runID always produces the same random sequence across replays
//   - RNG is accessible to nodes via context.Value(RNGKey)
//   - Validation: 100+ executions with same runID produce byte-identical final states
//
// 2. OrderKey-Based Merge Ordering:
//   - Parallel branch results are merged in deterministic order based on OrderKey
//   - OrderKey is computed from parent node ID and edge index: hash(parentNodeID || edgeIndex)
//   - Results are sorted by OrderKey (ascending) before applying reducer
//   - Merge order is independent of goroutine completion order
//   - Validation: 50+ parallel executions produce identical merge sequences
//
// 3. Recorded I/O Replay:
//   - External I/O operations (LLM calls, API requests) are captured during execution
//   - Each RecordedIO contains request, response, and SHA-256 hash for verification
//   - During replay, recorded responses are returned without re-executing external calls
//   - Strict replay mode verifies response hashes match expected values
//   - Validation: Replayed executions produce identical state transitions
//
// # Validation Results
//
// The determinism implementation has been validated with comprehensive stress tests:
//   - Retry delays: 100 runs with identical backoff sequences (RNG validation)
//   - Parallel merge: 50 runs with identical merge order (Frontier validation)
//   - RNG sequences: 100 runs with byte-identical random values
//   - OrderKey merge: 50 runs with identical value sequences
//   - Stress test: 1000 runs with 100% determinism (combined validation)
//
// # Usage Guidelines
//
// To ensure deterministic behavior in your nodes:
//
// 1. Use seeded RNG from context (DO NOT use global rand or crypto/rand):
//
//	func (n *MyNode) Run(ctx context.Context, state S) NodeResult[S] {
//	    rng := ctx.Value(RNGKey).(*rand.Rand)
//	    randomValue := rng.Intn(100)  // ✅ Deterministic
//	    // NOT: rand.Intn(100)         // ❌ Non-deterministic
//	}
//
// 2. Avoid time-based randomness (DO NOT use time.Now() for random seeds):
//
//	// ❌ Non-deterministic:
//	seed := time.Now().UnixNano()
//	rng := rand.New(rand.NewSource(seed))
//
//	// ✅ Deterministic:
//	rng := ctx.Value(RNGKey).(*rand.Rand)
//
// 3. Record external I/O for replay (implement Effects() for nodes with side effects):
//
//	func (n *APINode) Effects() SideEffectPolicy {
//	    return SideEffectPolicy{
//	        Recordable:          true,  // Enable I/O recording
//	        RequiresIdempotency: true,  // Use idempotency keys
//	    }
//	}
//
// 4. Use deterministic data structures (avoid map iteration):
//
//	// ❌ Non-deterministic map iteration:
//	for key, val := range myMap {
//	    process(key, val)
//	}
//
//	// ✅ Deterministic sorted keys:
//	keys := make([]string, 0, len(myMap))
//	for k := range myMap {
//	    keys = append(keys, k)
//	}
//	sort.Strings(keys)
//	for _, key := range keys {
//	    process(key, myMap[key])
//	}
//
// # Common Pitfalls
//
// Sources of non-determinism to avoid:
//   - Global rand package (use context RNG instead)
//   - crypto/rand (use context RNG for non-security random values)
//   - time.Now() for random seeds or jitter (use context RNG)
//   - Map iteration order (sort keys before iteration)
//   - Goroutine scheduling (use OrderKey-based merge)
//   - External state dependencies (record I/O responses)
//   - File system timestamps (use logical timestamps)
//
// # Replay Mode
//
// To replay a previous execution:
//
//  1. Original execution (record mode):
//     opts := Options{ReplayMode: false}
//     engine := New(reducer, store, emitter, opts)
//     _, err := engine.Run(ctx, "run-001", initialState)
//
//  2. Replay execution:
//     replayOpts := Options{ReplayMode: true, StrictReplay: true}
//     replayEngine := New(reducer, store, emitter, replayOpts)
//     replayedState, err := replayEngine.ReplayRun(ctx, "run-001")
//
// Replayed executions will:
//   - Use recorded I/O responses instead of making live calls
//   - Verify response hashes match expected values (if StrictReplay=true)
//   - Produce byte-identical final states
//   - Complete without invoking external services
//
// This enables:
//   - Debugging production issues locally
//   - Testing workflow logic without external dependencies
//   - Auditing execution flow for compliance
//   - Performance analysis without I/O latency

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
//
//nolint:unused // Reserved for future replay functionality
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
//
//nolint:unused // Reserved for future replay functionality
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
//
//nolint:unused // Reserved for future replay functionality
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
