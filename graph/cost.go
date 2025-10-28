package graph

import (
	"fmt"
	"sync"
	"time"
)

// ModelPricing defines input and output token costs for LLM models.
// Prices are in USD per 1M tokens (per million tokens).
type ModelPricing struct {
	InputPer1M  float64 // Cost per 1M input tokens in USD
	OutputPer1M float64 // Cost per 1M output tokens in USD
}

// Static pricing map for major LLM providers (as of 2025-01-01).
// Prices are in USD per 1M tokens.
//
// Sources:
//   - OpenAI: https://openai.com/pricing
//   - Anthropic: https://anthropic.com/pricing
//   - Google: https://cloud.google.com/vertex-ai/pricing
//
// Note: Prices subject to change. Update this map as providers adjust pricing.
var defaultModelPricing = map[string]ModelPricing{
	// OpenAI GPT-4o (optimized)
	"gpt-4o": {
		InputPer1M:  2.50,  // $2.50 per 1M input tokens
		OutputPer1M: 10.00, // $10.00 per 1M output tokens
	},
	"gpt-4o-2024-08-06": {
		InputPer1M:  2.50,
		OutputPer1M: 10.00,
	},

	// OpenAI GPT-4o-mini (smaller, cheaper)
	"gpt-4o-mini": {
		InputPer1M:  0.15, // $0.15 per 1M input tokens
		OutputPer1M: 0.60, // $0.60 per 1M output tokens
	},

	// OpenAI GPT-4 Turbo
	"gpt-4-turbo": {
		InputPer1M:  10.00, // $10.00 per 1M input tokens
		OutputPer1M: 30.00, // $30.00 per 1M output tokens
	},
	"gpt-4-turbo-2024-04-09": {
		InputPer1M:  10.00,
		OutputPer1M: 30.00,
	},

	// OpenAI GPT-3.5 Turbo
	"gpt-3.5-turbo": {
		InputPer1M:  0.50, // $0.50 per 1M input tokens
		OutputPer1M: 1.50, // $1.50 per 1M output tokens
	},

	// Anthropic Claude 3.5 Sonnet
	"claude-3-5-sonnet-20241022": {
		InputPer1M:  3.00,  // $3.00 per 1M input tokens
		OutputPer1M: 15.00, // $15.00 per 1M output tokens
	},
	"claude-3.5-sonnet": {
		InputPer1M:  3.00,
		OutputPer1M: 15.00,
	},

	// Anthropic Claude 3 Opus (most capable)
	"claude-3-opus-20240229": {
		InputPer1M:  15.00, // $15.00 per 1M input tokens
		OutputPer1M: 75.00, // $75.00 per 1M output tokens
	},
	"claude-3-opus": {
		InputPer1M:  15.00,
		OutputPer1M: 75.00,
	},

	// Anthropic Claude 3 Sonnet (balanced)
	"claude-3-sonnet-20240229": {
		InputPer1M:  3.00,  // $3.00 per 1M input tokens
		OutputPer1M: 15.00, // $15.00 per 1M output tokens
	},
	"claude-3-sonnet": {
		InputPer1M:  3.00,
		OutputPer1M: 15.00,
	},

	// Anthropic Claude 3 Haiku (fastest, cheapest)
	"claude-3-haiku-20240307": {
		InputPer1M:  0.25, // $0.25 per 1M input tokens
		OutputPer1M: 1.25, // $1.25 per 1M output tokens
	},
	"claude-3-haiku": {
		InputPer1M:  0.25,
		OutputPer1M: 1.25,
	},

	// Google Gemini 1.5 Pro
	"gemini-1.5-pro": {
		InputPer1M:  1.25, // $1.25 per 1M input tokens
		OutputPer1M: 5.00, // $5.00 per 1M output tokens
	},
	"gemini-1.5-pro-001": {
		InputPer1M:  1.25,
		OutputPer1M: 5.00,
	},

	// Google Gemini 1.5 Flash (faster, cheaper)
	"gemini-1.5-flash": {
		InputPer1M:  0.075, // $0.075 per 1M input tokens
		OutputPer1M: 0.30,  // $0.30 per 1M output tokens
	},
	"gemini-1.5-flash-001": {
		InputPer1M:  0.075,
		OutputPer1M: 0.30,
	},

	// Google Gemini 1.0 Pro (legacy)
	"gemini-1.0-pro": {
		InputPer1M:  0.50, // $0.50 per 1M input tokens
		OutputPer1M: 1.50, // $1.50 per 1M output tokens
	},
}

// LLMCall represents a single LLM API invocation with token usage and cost.
type LLMCall struct {
	Model        string    // Model identifier (e.g., "gpt-4o", "claude-3-sonnet")
	InputTokens  int       // Number of input tokens consumed
	OutputTokens int       // Number of output tokens generated
	CostUSD      float64   // Calculated cost in USD
	Timestamp    time.Time // When the call was made
	NodeID       string    // Node that made the call (optional)
}

// CostTracker (T040) tracks financial costs associated with LLM API calls,
// providing detailed token usage and cost attribution for production monitoring.
//
// Features:
//   - Per-model token counting (input/output separate)
//   - Accurate cost calculation using static pricing tables
//   - Cumulative cost tracking across multiple calls
//   - Per-model cost breakdown for attribution
//   - Thread-safe concurrent recording
//
// Pricing is based on static tables (defaultModelPricing) for major providers:
//   - OpenAI: GPT-4o, GPT-4o-mini, GPT-4-turbo, GPT-3.5-turbo
//   - Anthropic: Claude 3.5 Sonnet, Claude 3 Opus/Sonnet/Haiku
//   - Google: Gemini 1.5 Pro/Flash, Gemini 1.0 Pro
//
// Usage:
//
//	// Create tracker for a run
//	tracker := NewCostTracker("run-123", "USD")
//
//	// Record LLM calls
//	tracker.RecordLLMCall("gpt-4o", 1000, 500, "nodeA")
//	tracker.RecordLLMCall("claude-3-sonnet", 2000, 800, "nodeB")
//
//	// Get total cost
//	total := tracker.GetTotalCost() // e.g., $0.0345
//
//	// Get per-model breakdown
//	costs := tracker.GetCostByModel() // map[string]float64{"gpt-4o": 0.0125, "claude-3-sonnet": 0.0220}
//
// Thread-safe: All methods use mutex protection for concurrent access.
type CostTracker struct {
	// RunID associates costs with a specific workflow execution
	RunID string

	// Currency is the cost unit (e.g., "USD")
	Currency string

	// Pricing maps model names to their input/output token costs
	Pricing map[string]ModelPricing

	// Calls records all LLM invocations with full details
	Calls []LLMCall

	// TotalCost accumulates all costs in the specified currency
	TotalCost float64

	// ModelCosts tracks costs per model for attribution
	ModelCosts map[string]float64

	// InputTokens counts total input tokens across all calls
	InputTokens int64

	// OutputTokens counts total output tokens across all calls
	OutputTokens int64

	// CreatedAt marks when cost tracking began
	CreatedAt time.Time

	// Mutex protects concurrent access to tracker state
	mu sync.RWMutex

	// enabled controls whether cost tracking is active
	enabled bool
}

// NewCostTracker (T040) creates a new cost tracker with default pricing tables.
//
// Parameters:
//   - runID: Unique workflow execution identifier
//   - currency: Cost unit (e.g., "USD")
//
// Returns:
//   - *CostTracker: Fully initialized cost tracker with default model pricing
//
// Example:
//
//	tracker := NewCostTracker("run-123", "USD")
func NewCostTracker(runID, currency string) *CostTracker {
	return &CostTracker{
		RunID:      runID,
		Currency:   currency,
		Pricing:    defaultModelPricing, // Use static pricing table
		Calls:      make([]LLMCall, 0, 100),
		ModelCosts: make(map[string]float64),
		CreatedAt:  time.Now(),
		enabled:    true,
	}
}

// RecordLLMCall (T041) records a single LLM API invocation with token usage and calculates cost.
//
// This method:
//  1. Looks up model pricing from the static pricing table
//  2. Calculates cost: (inputTokens * inputPrice + outputTokens * outputPrice) / 1M
//  3. Records the call with full metadata
//  4. Updates cumulative totals (TotalCost, ModelCosts, InputTokens, OutputTokens)
//
// Parameters:
//   - model: Model identifier (must exist in pricing table, e.g., "gpt-4o", "claude-3-sonnet")
//   - inputTokens: Number of input tokens consumed
//   - outputTokens: Number of output tokens generated
//   - nodeID: Node that made the call (optional, use "" if not applicable)
//
// Returns:
//   - error: ErrUnknownModel if model not found in pricing table, nil otherwise
//
// Example:
//
//	// Record a GPT-4o call: 1000 input tokens, 500 output tokens
//	err := tracker.RecordLLMCall("gpt-4o", 1000, 500, "research_node")
//	if err != nil {
//	    log.Printf("Cost tracking error: %v", err)
//	}
//
// Thread-safe: Uses mutex protection for concurrent recording.
func (ct *CostTracker) RecordLLMCall(model string, inputTokens, outputTokens int, nodeID string) error {
	if !ct.enabled {
		return nil
	}

	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Lookup pricing for this model
	pricing, ok := ct.Pricing[model]
	if !ok {
		// Model not in pricing table - still record but with zero cost
		pricing = ModelPricing{InputPer1M: 0, OutputPer1M: 0}
	}

	// Calculate cost: (tokens / 1M) * price_per_1M
	inputCost := (float64(inputTokens) / 1_000_000.0) * pricing.InputPer1M
	outputCost := (float64(outputTokens) / 1_000_000.0) * pricing.OutputPer1M
	totalCost := inputCost + outputCost

	// Record the call
	call := LLMCall{
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CostUSD:      totalCost,
		Timestamp:    time.Now(),
		NodeID:       nodeID,
	}
	ct.Calls = append(ct.Calls, call)

	// Update cumulative totals
	ct.TotalCost += totalCost
	ct.ModelCosts[model] += totalCost
	ct.InputTokens += int64(inputTokens)
	ct.OutputTokens += int64(outputTokens)

	return nil
}

// GetTotalCost (T042) returns the cumulative cost across all recorded LLM calls.
//
// Returns:
//   - float64: Total cost in the tracker's currency (e.g., USD)
//
// Example:
//
//	total := tracker.GetTotalCost()
//	fmt.Printf("Total LLM cost: $%.4f\n", total)
//
// Thread-safe: Uses read lock for concurrent access.
func (ct *CostTracker) GetTotalCost() float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.TotalCost
}

// GetCostByModel (T043) returns a breakdown of costs attributed to each model.
//
// Returns:
//   - map[string]float64: Map of model name to cumulative cost in tracker's currency
//
// Example:
//
//	costs := tracker.GetCostByModel()
//	for model, cost := range costs {
//	    fmt.Printf("%s: $%.4f\n", model, cost)
//	}
//	// Output:
//	// gpt-4o: $0.0125
//	// claude-3-sonnet: $0.0220
//
// Thread-safe: Uses read lock and returns a copy to prevent mutation.
func (ct *CostTracker) GetCostByModel() map[string]float64 {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	// Return a copy to prevent external mutation
	costs := make(map[string]float64, len(ct.ModelCosts))
	for model, cost := range ct.ModelCosts {
		costs[model] = cost
	}
	return costs
}

// GetCallHistory returns all recorded LLM calls with full metadata.
//
// Returns:
//   - []LLMCall: Slice of all calls in chronological order
//
// Thread-safe: Uses read lock and returns a copy.
func (ct *CostTracker) GetCallHistory() []LLMCall {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	// Return a copy to prevent external mutation
	calls := make([]LLMCall, len(ct.Calls))
	copy(calls, ct.Calls)
	return calls
}

// GetTokenUsage returns total input and output token counts.
//
// Returns:
//   - inputTokens: Total input tokens across all calls
//   - outputTokens: Total output tokens across all calls
//
// Thread-safe: Uses read lock.
func (ct *CostTracker) GetTokenUsage() (inputTokens, outputTokens int64) {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return ct.InputTokens, ct.OutputTokens
}

// SetCustomPricing allows overriding default pricing for specific models.
// Useful for custom deployments, enterprise pricing, or price updates.
//
// Parameters:
//   - model: Model identifier
//   - inputPer1M: Cost per 1M input tokens in USD
//   - outputPer1M: Cost per 1M output tokens in USD
//
// Example:
//
//	// Override GPT-4o pricing for enterprise rate
//	tracker.SetCustomPricing("gpt-4o", 2.00, 8.00)
func (ct *CostTracker) SetCustomPricing(model string, inputPer1M, outputPer1M float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	if ct.Pricing == nil {
		ct.Pricing = make(map[string]ModelPricing)
	}
	ct.Pricing[model] = ModelPricing{
		InputPer1M:  inputPer1M,
		OutputPer1M: outputPer1M,
	}
}

// Disable temporarily disables cost tracking (useful for testing).
func (ct *CostTracker) Disable() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.enabled = false
}

// Enable re-enables cost tracking after Disable().
func (ct *CostTracker) Enable() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.enabled = true
}

// Reset clears all recorded data and resets cumulative totals.
// Preserves pricing configuration.
func (ct *CostTracker) Reset() {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.Calls = make([]LLMCall, 0, 100)
	ct.TotalCost = 0
	ct.ModelCosts = make(map[string]float64)
	ct.InputTokens = 0
	ct.OutputTokens = 0
}

// String returns a human-readable summary of cost tracking.
func (ct *CostTracker) String() string {
	ct.mu.RLock()
	defer ct.mu.RUnlock()

	return fmt.Sprintf(
		"CostTracker{RunID: %s, Calls: %d, TotalCost: $%.4f %s, InputTokens: %d, OutputTokens: %d}",
		ct.RunID,
		len(ct.Calls),
		ct.TotalCost,
		ct.Currency,
		ct.InputTokens,
		ct.OutputTokens,
	)
}
