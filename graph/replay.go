package graph

import (
	"encoding/json"
	"time"
)

// Replay provides deterministic replay of graph executions

// RecordedIO captures an external interaction (API call, database query, etc.)
// for deterministic replay without re-invoking the external service.
//
// During initial execution, RecordedIO instances are created for nodes with
// SideEffectPolicy.Recordable=true. During replay, these recorded I/Os are
// matched by (NodeID, Attempt) and their responses are returned directly
// without executing the node.
//
// The Hash field enables mismatch detection: if a live execution produces
// a different response hash than recorded, ErrReplayMismatch is raised,
// indicating non-deterministic behavior.
type RecordedIO struct {
	// NodeID identifies the node that performed this I/O operation
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

	// Hash is a SHA-256 hash of the response content, used for mismatch detection
	// during replay. Format: "sha256:hex_encoded_hash"
	Hash string `json:"hash"`

	// Timestamp records when this I/O operation was captured
	Timestamp time.Time `json:"timestamp"`

	// Duration is how long the I/O operation took to complete.
	// This can be used for performance analysis and replay simulation.
	Duration time.Duration `json:"duration"`
}
