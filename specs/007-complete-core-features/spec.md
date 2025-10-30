# Feature Specification: Complete Core Features for Production Readiness

**Feature Branch**: `007-complete-core-features`
**Created**: 2025-10-30
**Status**: Draft
**Input**: User description: "Complete missing core functionality identified in comprehensive codebase review to achieve production readiness"

## User Scenarios & Testing

### User Story 1 - Sequential Execution with Retries (Priority: P1)

Developers building workflows with sequential execution need automatic retry capability when operations fail transiently. Currently, retry logic only works in concurrent mode (MaxConcurrentNodes > 0), forcing developers to choose between retries or sequential execution.

**Why this priority**: Blocking feature for production workflows that require deterministic ordering (e.g., financial transactions, audit trails) but also need resilience against transient failures.

**Independent Test**: Can be fully tested by creating a workflow with MaxConcurrentNodes: 0, configuring a retry policy, triggering a transient failure, and verifying the operation retries and succeeds. Delivers immediate value for sequential workflow reliability.

**Acceptance Scenarios**:

1. **Given** a sequential workflow (MaxConcurrentNodes: 0) with a node configured for 3 retries, **When** the node fails with a transient error on first attempt, **Then** the workflow automatically retries up to 3 times with exponential backoff before failing
2. **Given** a sequential workflow with retry policy, **When** multiple runs execute with the same runID, **Then** all runs produce identical retry delay sequences (deterministic backoff)
3. **Given** a sequential workflow with retries, **When** a node succeeds on the second attempt, **Then** the workflow continues to next node without further retries

---

### User Story 2 - Per-Node Timeout Control (Priority: P2)

Developers need fine-grained timeout control for individual workflow nodes. Currently, only global workflow timeouts are enforced, making it impossible to set different timeout limits for fast operations (e.g., cache lookups) versus slow operations (e.g., model training).

**Why this priority**: Important for production where different operations have vastly different expected durations. Workaround exists (use global timeout), but forces conservative settings that slow down fast-path operations.

**Independent Test**: Can be fully tested by creating a workflow with nodes having different timeout policies, verifying fast nodes timeout quickly while slow nodes have longer limits, and ensuring timeout errors are properly reported. Delivers value for production efficiency.

**Acceptance Scenarios**:

1. **Given** a workflow with Node A (10s timeout) and Node B (60s timeout), **When** Node A runs for 11 seconds, **Then** Node A times out with clear error while Node B continues normally
2. **Given** a node with explicit timeout policy, **When** no global timeout is configured, **Then** the node-specific timeout is enforced independently
3. **Given** a workflow with default node timeout (30s) and Node C with override (5s), **When** Node C executes, **Then** the override timeout (5s) takes precedence over the default

---

### User Story 3 - Backpressure Visibility (Priority: P3)

Operations teams running production workflows need visibility into queue saturation to detect performance bottlenecks and capacity issues. While backpressure handling works correctly (blocking when queue fills), there's no way to monitor when it occurs or measure its frequency.

**Why this priority**: Quality-of-life improvement for production monitoring. Functionality works correctly; this adds observability for capacity planning and alerting.

**Independent Test**: Can be fully tested by saturating the work queue, verifying backpressure metrics increment, checking events are emitted, and confirming alerts can be configured. Delivers value for production operations.

**Acceptance Scenarios**:

1. **Given** a workflow with queue depth 10 and 20 pending items, **When** the queue fills completely, **Then** backpressure metrics increment and monitoring systems can detect saturation
2. **Given** backpressure monitoring enabled, **When** workflow execution blocks due to queue pressure, **Then** events are emitted with queue depth, wait time, and affected node information
3. **Given** production monitoring dashboards, **When** viewing workflow health, **Then** backpressure frequency and duration are visible for capacity planning

---

### Edge Cases

- What happens when a node times out during retry backoff delay?
- How does system handle timeout during checkpoint save after node execution?
- What if backpressure timeout expires while waiting for queue space?
- How are per-node timeouts coordinated with global workflow timeout budget?
- How does system handle clock skew between retry delay timestamps?

## Requirements

### Functional Requirements

#### Sequential Retry Logic
- **FR-001**: System MUST support retry policies in sequential execution mode (MaxConcurrentNodes: 0)
- **FR-002**: Retry delays MUST be deterministic and reproducible across runs with the same runID
- **FR-003**: System MUST apply exponential backoff with deterministic jitter for retry delays
- **FR-004**: System MUST respect maximum retry attempt limits configured in node policy
- **FR-005**: System MUST distinguish between transient errors (retryable) and permanent failures (non-retryable)

#### Per-Node Timeout Enforcement
- **FR-006**: System MUST enforce timeout limits configured in NodePolicy.Timeout field
- **FR-007**: System MUST use DefaultNodeTimeout as fallback when NodePolicy.Timeout is zero
- **FR-008**: System MUST cancel node execution when timeout expires and return timeout error
- **FR-009**: Per-node timeout MUST NOT exceed global RunWallClockBudget if configured
- **FR-010**: System MUST include timeout duration and node identifier in timeout error messages

#### Backpressure Monitoring
- **FR-011**: System MUST emit metrics when workflow queue reaches capacity (backpressure event)
- **FR-012**: Backpressure metrics MUST include queue depth, wait duration, and affected node identifier
- **FR-013**: System MUST emit backpressure events through configured emitter for monitoring integration
- **FR-014**: System MUST track backpressure frequency and duration for capacity analysis
- **FR-015**: Backpressure monitoring MUST NOT impact workflow execution performance or correctness

### Key Entities

- **RetryAttempt**: Represents a single retry execution with attempt number, delay duration, error encountered, and timestamp
- **TimeoutPolicy**: Configuration for node-specific timeout with duration limit and error context
- **BackpressureEvent**: Monitoring data capturing queue saturation including depth, wait time, node ID, and timestamp

## Success Criteria

### Measurable Outcomes

- **SC-001**: Sequential workflows with retry policies achieve 99.9% success rate for transient failures (measured over 1000 test runs)
- **SC-002**: Retry delay sequences are 100% deterministic across repeated executions with identical runID
- **SC-003**: Per-node timeouts terminate long-running operations within 1% tolerance of configured limit
- **SC-004**: Workflows with mixed timeout requirements execute efficiently (fast paths not delayed by slow path timeouts)
- **SC-005**: Backpressure events are emitted within 100ms of queue saturation occurring
- **SC-006**: Operations teams can detect queue capacity issues within 5 minutes through monitoring dashboards
- **SC-007**: All 11 currently skipped tests pass after implementation completion
- **SC-008**: Zero performance regression in existing workflows (benchmark variance < 5%)

## Assumptions

- Retry logic will reuse existing RNG infrastructure for deterministic jitter calculation
- Timeout enforcement leverages existing context cancellation mechanisms
- Backpressure metrics integrate with existing Metrics interface implementation
- Sequential execution path exists and needs retry enhancement
- Context timeout propagation doesn't require engine architecture changes
- Monitoring infrastructure (Prometheus/OTEL) is already configured in production deployments

## Dependencies

- Existing Options.Retries configuration (already defined)
- Existing Options.DefaultNodeTimeout configuration (already defined)  
- Existing Options.BackpressureTimeout configuration (already defined)
- Current Metrics interface and Prometheus integration
- Existing Emitter interface for event emission
- NodePolicy interface with Timeout field

## Scope

### In Scope

- Sequential retry logic implementation matching concurrent behavior
- Per-node timeout context wrapping and enforcement
- Backpressure metric emission and event generation
- Test completion for all 11 skipped pending-implementation tests
- Documentation updates for new capabilities
- Example updates demonstrating retry and timeout patterns

### Out of Scope

- Replay execution (ReplayRun method) - deferred to future feature
- Phase 8 API enhancements (PauseRun, ResumeRun) - quality-of-life improvements
- Advanced conflict resolution policies (LastWriterWins, CRDT) - future enhancement
- Store batch operations - optimization, not core functionality
- New workflow examples beyond retry/timeout demonstrations
- Changes to concurrent execution path (already working)

## Non-Functional Requirements

- **NFR-001**: Retry implementation MUST maintain backward compatibility with existing workflows
- **NFR-002**: Timeout enforcement MUST NOT introduce race conditions in concurrent execution
- **NFR-003**: Backpressure monitoring MUST have negligible performance overhead (< 1% latency increase)
- **NFR-004**: All implementations MUST maintain existing determinism guarantees
- **NFR-005**: Error messages MUST be clear and actionable for debugging
- **NFR-006**: Monitoring metrics MUST follow existing Prometheus naming conventions

## Risks

- **Risk**: Sequential retry implementation may require refactoring runSequential method
  **Mitigation**: Reuse existing retry logic patterns from concurrent execution, adapt for sequential context

- **Risk**: Per-node timeouts may conflict with global timeout budget in edge cases
  **Mitigation**: Document precedence rules clearly - global timeout is hard limit, per-node are soft limits within budget

- **Risk**: Backpressure metrics may impact performance in high-throughput scenarios
  **Mitigation**: Use atomic counters and buffered event emission, benchmark before/after

- **Risk**: Test completion may reveal additional bugs in retry or timeout logic
  **Mitigation**: Address bugs as discovered, prioritize based on severity

## Version History

| Version | Date       | Changes          | Author |
|---------|------------|------------------|--------|
| 1.0     | 2025-10-30 | Initial draft based on codebase review | Claude |
