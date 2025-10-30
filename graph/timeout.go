package graph

import (
	"context"
	"fmt"
	"time"
)

// getNodeTimeout determines the timeout duration for a node based on precedence:
// 1. NodePolicy.Timeout (per-node override)
// 2. defaultTimeout (engine-wide default)
// 3. 0 (no timeout, unlimited execution)
//
// US2: T019 - Timeout precedence logic
func getNodeTimeout(policy *NodePolicy, defaultTimeout time.Duration) time.Duration {
	// Check for per-node timeout override
	if policy != nil && policy.Timeout > 0 {
		return policy.Timeout
	}

	// Fall back to engine default
	if defaultTimeout > 0 {
		return defaultTimeout
	}

	// No timeout configured (0 = unlimited)
	return 0
}

// executeNodeWithTimeout wraps node execution with timeout enforcement.
//
// It determines the timeout based on precedence (NodePolicy > DefaultNodeTimeout),
// creates a timeout context if needed, executes the node, and handles timeout errors.
//
// Parameters:
//   - ctx: Parent context
//   - node: Node implementation to execute
//   - nodeID: Node identifier for error messages
//   - state: Current workflow state
//   - policy: Optional node policy (may be nil)
//   - defaultTimeout: Engine-wide default timeout
//
// Returns:
//   - result: Node execution result
//   - timeoutErr: Timeout error if node exceeded time limit, nil otherwise
//
// US2: T017, T018 - Per-node timeout enforcement
func executeNodeWithTimeout[S any](
	ctx context.Context,
	node Node[S],
	nodeID string,
	state S,
	policy *NodePolicy,
	defaultTimeout time.Duration,
) (NodeResult[S], error) {
	// Determine timeout duration (T019)
	timeout := getNodeTimeout(policy, defaultTimeout)

	// If no timeout configured, execute directly
	if timeout == 0 {
		result := node.Run(ctx, state)
		return result, nil
	}

	// Create timeout context (T017, T018)
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel() // Always cleanup to prevent context leaks

	// Execute node with timeout context
	result := node.Run(timeoutCtx, state)

	// Check if context deadline was exceeded (T020)
	if timeoutCtx.Err() == context.DeadlineExceeded {
		// Node exceeded configured timeout
		timeoutErr := &EngineError{
			Message: fmt.Sprintf("node %s exceeded timeout of %v", nodeID, timeout),
			Code:    "NODE_TIMEOUT",
		}
		return result, timeoutErr
	}

	// No timeout error
	return result, nil
}
