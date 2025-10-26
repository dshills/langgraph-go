# LLM Integration

This guide covers integrating Large Language Models (LLMs) into your workflows, provider switching patterns, error handling, and tool usage.

## Overview

LangGraph-Go provides a unified `ChatModel` interface that abstracts differences between LLM providers (OpenAI, Anthropic, Google). This enables:

- **Provider Swapping**: Change providers without modifying workflow logic
- **Multi-Provider Workflows**: Use different providers for different tasks
- **Fallback Strategies**: Handle provider errors gracefully
- **Tool Calling**: Enable LLMs to invoke external functions

## ChatModel Interface

All LLM providers implement the same interface:

```go
type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}
```

### Message Format

```go
type Message struct {
    Role    string  // "system", "user", or "assistant"
    Content string  // Message text
}

// Standard roles
const (
    RoleSystem    = "system"     // System instructions/context
    RoleUser      = "user"       // User input
    RoleAssistant = "assistant"  // LLM response
)
```

### Basic Usage

```go
import (
    "context"
    "github.com/dshills/langgraph-go/graph/model"
    "github.com/dshills/langgraph-go/graph/model/openai"
)

// Create model
model := openai.NewChatModel(apiKey, "gpt-4o")

// Send messages
messages := []model.Message{
    {Role: model.RoleUser, Content: "What is the capital of France?"},
}

out, err := model.Chat(ctx, messages, nil)
if err != nil {
    log.Fatal(err)
}

fmt.Println(out.Text) // "The capital of France is Paris."
```

## Supported Providers

### OpenAI (GPT-4, GPT-3.5)

```go
import "github.com/dshills/langgraph-go/graph/model/openai"

// Latest GPT-4o
model := openai.NewChatModel(apiKey, "gpt-4o")

// Or GPT-3.5 Turbo for faster/cheaper
model := openai.NewChatModel(apiKey, "gpt-3.5-turbo")
```

**Strengths**:
- Excellent tool calling
- JSON mode support
- Automatic retry logic
- Fast response times

**Best For**: Tool-heavy workflows, structured outputs, general tasks

### Anthropic (Claude)

```go
import "github.com/dshills/langgraph-go/graph/model/anthropic"

// Claude Sonnet 4.5 (latest)
model := anthropic.NewChatModel(apiKey, "claude-sonnet-4-5-20250929")

// Or Claude Opus for maximum quality
model := anthropic.NewChatModel(apiKey, "claude-3-opus-20240229")
```

**Strengths**:
- Long context windows (200K+ tokens)
- Detailed reasoning
- Strong system prompt adherence
- High-quality outputs

**Best For**: Long-form reasoning, detailed analysis, complex instructions

### Google (Gemini)

```go
import "github.com/dshills/langgraph-go/graph/model/google"

// Gemini 2.5 Flash (fast and efficient)
model := google.NewChatModel(apiKey, "gemini-2.5-flash")

// Or Gemini Pro Vision for multimodal
model := google.NewChatModel(apiKey, "gemini-pro-vision")
```

**Strengths**:
- Very fast responses
- Multimodal capabilities (vision)
- Competitive pricing
- Good for simple tasks

**Best For**: Quick responses, multimodal tasks, cost optimization

## Integration Patterns

### Pattern 1: Single Provider

Use one provider consistently:

```go
type State struct {
    Query    string
    Response string
}

func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Use OpenAI for all queries
    model := openai.NewChatModel(apiKey, "gpt-4o")

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    out, err := model.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}
```

### Pattern 2: Task-Based Provider Selection

Choose provider based on task requirements:

```go
type State struct {
    TaskType string
    Query    string
    Response string
}

func selectModel(taskType string, apiKeys map[string]string) model.ChatModel {
    switch taskType {
    case "reasoning":
        // Claude for deep reasoning
        return anthropic.NewChatModel(apiKeys["anthropic"], "claude-sonnet-4-5-20250929")

    case "quick":
        // Gemini for fast responses
        return google.NewChatModel(apiKeys["google"], "gemini-2.5-flash")

    case "tools":
        // GPT-4 for tool calling
        return openai.NewChatModel(apiKeys["openai"], "gpt-4o")

    default:
        // GPT-3.5 for general tasks
        return openai.NewChatModel(apiKeys["openai"], "gpt-3.5-turbo")
    }
}

func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    llm := selectModel(s.TaskType, apiKeys)

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}
```

### Pattern 3: Fallback on Error

Handle provider failures with fallbacks:

```go
type State struct {
    Query         string
    Response      string
    ProviderUsed  string
}

func llmNodeWithFallback(ctx context.Context, s State) graph.NodeResult[State] {
    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    // Try providers in order
    providers := []struct {
        name  string
        model model.ChatModel
    }{
        {"openai", openai.NewChatModel(openaiKey, "gpt-4o")},
        {"anthropic", anthropic.NewChatModel(anthropicKey, "claude-sonnet-4-5-20250929")},
        {"google", google.NewChatModel(googleKey, "gemini-2.5-flash")},
    }

    var lastErr error
    for _, p := range providers {
        out, err := p.model.Chat(ctx, messages, nil)
        if err == nil {
            return graph.NodeResult[State]{
                Delta: State{
                    Response:     out.Text,
                    ProviderUsed: p.name,
                },
                Route: graph.Goto("next"),
            }
        }

        log.Printf("Provider %s failed: %v, trying next...", p.name, err)
        lastErr = err
    }

    return graph.NodeResult[State]{
        Err: fmt.Errorf("all providers failed: %w", lastErr),
    }
}
```

### Pattern 4: Multi-Provider Consensus

Get responses from multiple providers and compare:

```go
type State struct {
    Query            string
    Responses        map[string]string
    ConsensusAnswer  string
}

func multiProviderNode(ctx context.Context, s State) graph.NodeResult[State] {
    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    responses := make(map[string]string)

    // Query all providers in parallel (using fan-out)
    // For simplicity, shown sequentially here
    providers := map[string]model.ChatModel{
        "gpt4":   openai.NewChatModel(openaiKey, "gpt-4o"),
        "claude": anthropic.NewChatModel(anthropicKey, "claude-sonnet-4-5-20250929"),
        "gemini": google.NewChatModel(googleKey, "gemini-2.5-flash"),
    }

    for name, llm := range providers {
        out, err := llm.Chat(ctx, messages, nil)
        if err != nil {
            log.Printf("%s failed: %v", name, err)
            continue
        }
        responses[name] = out.Text
    }

    // Compute consensus (simplified - just pick most common answer)
    consensus := findConsensus(responses)

    return graph.NodeResult[State]{
        Delta: State{
            Responses:       responses,
            ConsensusAnswer: consensus,
        },
        Route: graph.Goto("next"),
    }
}

func findConsensus(responses map[string]string) string {
    // Simple implementation: return first non-empty response
    // In production: implement actual consensus logic
    for _, resp := range responses {
        if resp != "" {
            return resp
        }
    }
    return ""
}
```

### Pattern 5: Cost Optimization

Use cheaper models for simple tasks:

```go
type State struct {
    Complexity int // 1-10 scale
    Query      string
    Response   string
}

func selectByComplexity(complexity int) model.ChatModel {
    if complexity >= 8 {
        // High complexity: use best model
        return openai.NewChatModel(apiKey, "gpt-4o")
    } else if complexity >= 5 {
        // Medium complexity: balanced model
        return anthropic.NewChatModel(apiKey, "claude-3-sonnet-20240229")
    } else {
        // Low complexity: fast & cheap
        return openai.NewChatModel(apiKey, "gpt-3.5-turbo")
    }
}

func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    llm := selectByComplexity(s.Complexity)

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}
```

## Tool Calling

LLMs can invoke external tools/functions:

### Defining Tools

```go
// Define a weather tool
weatherTool := model.ToolSpec{
    Name:        "get_weather",
    Description: "Get current weather for a location",
    Schema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "location": map[string]interface{}{
                "type":        "string",
                "description": "City name or coordinates",
            },
            "units": map[string]interface{}{
                "type":        "string",
                "enum":        []string{"celsius", "fahrenheit"},
                "description": "Temperature units",
            },
        },
        "required": []string{"location"},
    },
}

// Define a calculator tool
calcTool := model.ToolSpec{
    Name:        "calculator",
    Description: "Performs basic arithmetic",
    Schema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "operation": map[string]interface{}{
                "type": "string",
                "enum": []string{"add", "subtract", "multiply", "divide"},
            },
            "a": map[string]interface{}{"type": "number"},
            "b": map[string]interface{}{"type": "number"},
        },
        "required": []string{"operation", "a", "b"},
    },
}

tools := []model.ToolSpec{weatherTool, calcTool}
```

### Using Tools in Workflows

```go
type State struct {
    Query      string
    Messages   []model.Message
    ToolResult string
    Response   string
}

func llmWithToolsNode(ctx context.Context, s State) graph.NodeResult[State] {
    llm := openai.NewChatModel(apiKey, "gpt-4o")

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    // Call LLM with tools
    out, err := llm.Chat(ctx, messages, tools)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Check if LLM wants to call tools
    if len(out.ToolCalls) > 0 {
        // Route to tool execution node
        return graph.NodeResult[State]{
            Delta: State{
                Messages: append(messages, model.Message{
                    Role:    model.RoleAssistant,
                    Content: "", // Tool call message
                }),
            },
            Route: graph.Goto("execute-tools"),
        }
    }

    // Direct text response
    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("finalize"),
    }
}

func executeToolsNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Execute tool calls (implementation depends on your tools)
    // For example:
    result := executeWeatherTool(toolCall.Input)

    // Send result back to LLM
    messages := append(s.Messages, model.Message{
        Role:    model.RoleUser,
        Content: fmt.Sprintf("Tool result: %s", result),
    })

    llm := openai.NewChatModel(apiKey, "gpt-4o")
    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("finalize"),
    }
}
```

## Error Handling

### Provider-Specific Errors

Handle provider-specific error types:

```go
import (
    "errors"
    "github.com/dshills/langgraph-go/graph/model/google"
)

func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    llm := google.NewChatModel(googleKey, "gemini-2.5-flash")

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        // Check for Google safety filter errors
        var safetyErr *google.SafetyFilterError
        if errors.As(err, &safetyErr) {
            log.Printf("Content blocked: %s (category: %s)",
                safetyErr.Reason(), safetyErr.Category())

            // Try with different provider
            fallbackLLM := openai.NewChatModel(openaiKey, "gpt-4o")
            out, err = fallbackLLM.Chat(ctx, messages, nil)
            if err != nil {
                return graph.NodeResult[State]{Err: err}
            }
        } else {
            return graph.NodeResult[State]{Err: err}
        }
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}
```

### Context Timeouts

All providers respect context cancellation:

```go
func llmNodeWithTimeout(ctx context.Context, s State) graph.NodeResult[State] {
    // Set 30-second timeout for LLM call
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    llm := openai.NewChatModel(apiKey, "gpt-4o")

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            log.Println("LLM request timed out")
            // Could retry with simpler model or different provider
        }
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}
```

### Retry Logic

OpenAI adapter includes automatic retry for transient errors. For other providers, implement manually:

```go
func llmNodeWithRetry(ctx context.Context, s State) graph.NodeResult[State] {
    llm := anthropic.NewChatModel(anthropicKey, "claude-sonnet-4-5-20250929")

    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    var out model.ChatOut
    var err error

    // Retry up to 3 times
    for attempt := 0; attempt < 3; attempt++ {
        out, err = llm.Chat(ctx, messages, nil)
        if err == nil {
            break // Success
        }

        if !isTransientError(err) {
            break // Don't retry non-transient errors
        }

        // Exponential backoff
        time.Sleep(time.Second * time.Duration(1<<uint(attempt)))
        log.Printf("Retry attempt %d after error: %v", attempt+1, err)
    }

    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}

func isTransientError(err error) bool {
    // Check for rate limits, timeouts, temporary failures
    // Implementation depends on error types
    return strings.Contains(err.Error(), "rate limit") ||
           strings.Contains(err.Error(), "timeout") ||
           strings.Contains(err.Error(), "503")
}
```

## Best Practices

### 1. Use System Prompts Effectively

```go
messages := []model.Message{
    {
        Role: model.RoleSystem,
        Content: "You are a helpful assistant that provides concise, factual answers.",
    },
    {Role: model.RoleUser, Content: query},
}
```

### 2. Maintain Conversation Context

```go
type State struct {
    ConversationHistory []model.Message
    UserQuery           string
    Response            string
}

func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    // Append new user message
    messages := append(s.ConversationHistory, model.Message{
        Role:    model.RoleUser,
        Content: s.UserQuery,
    })

    llm := openai.NewChatModel(apiKey, "gpt-4o")
    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Save conversation including assistant response
    updatedHistory := append(messages, model.Message{
        Role:    model.RoleAssistant,
        Content: out.Text,
    })

    return graph.NodeResult[State]{
        Delta: State{
            ConversationHistory: updatedHistory,
            Response:            out.Text,
        },
        Route: graph.Goto("next"),
    }
}
```

### 3. Monitor Costs

```go
type State struct {
    TokensUsed  int
    CostUSD     float64
    Response    string
}

// Track usage (would need provider-specific implementation)
func llmNode(ctx context.Context, s State) graph.NodeResult[State] {
    llm := openai.NewChatModel(apiKey, "gpt-4o")

    messages := []model.Message{
        {Role: model.RoleUser, Content: query},
    }

    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Estimate tokens and cost (rough approximation)
    tokens := estimateTokens(query, out.Text)
    cost := calculateCost("gpt-4o", tokens)

    return graph.NodeResult[State]{
        Delta: State{
            TokensUsed: tokens,
            CostUSD:    cost,
            Response:   out.Text,
        },
        Route: graph.Goto("next"),
    }
}
```

### 4. Cache Responses

```go
var responseCache = make(map[string]string)
var cacheMutex sync.RWMutex

func llmNodeWithCache(ctx context.Context, s State) graph.NodeResult[State] {
    // Check cache
    cacheMutex.RLock()
    if cached, ok := responseCache[s.Query]; ok {
        cacheMutex.RUnlock()
        return graph.NodeResult[State]{
            Delta: State{Response: cached},
            Route: graph.Goto("next"),
        }
    }
    cacheMutex.RUnlock()

    // Call LLM
    llm := openai.NewChatModel(apiKey, "gpt-4o")
    messages := []model.Message{
        {Role: model.RoleUser, Content: s.Query},
    }

    out, err := llm.Chat(ctx, messages, nil)
    if err != nil {
        return graph.NodeResult[State]{Err: err}
    }

    // Store in cache
    cacheMutex.Lock()
    responseCache[s.Query] = out.Text
    cacheMutex.Unlock()

    return graph.NodeResult[State]{
        Delta: State{Response: out.Text},
        Route: graph.Goto("next"),
    }
}
```

## Testing LLM Workflows

### Mock Provider for Testing

```go
type MockChatModel struct {
    responses []string
    callCount int
}

func (m *MockChatModel) Chat(ctx context.Context, messages []model.Message, tools []model.ToolSpec) (model.ChatOut, error) {
    if m.callCount >= len(m.responses) {
        return model.ChatOut{}, errors.New("no more mock responses")
    }

    response := m.responses[m.callCount]
    m.callCount++

    return model.ChatOut{Text: response}, nil
}

func TestLLMNode(t *testing.T) {
    mockLLM := &MockChatModel{
        responses: []string{"Mocked response"},
    }

    // Use mock instead of real provider
    state := State{Query: "Test query"}
    result := llmNode(context.Background(), state, mockLLM)

    if result.Err != nil {
        t.Fatal(result.Err)
    }

    if result.Delta.Response != "Mocked response" {
        t.Errorf("wrong response: %s", result.Delta.Response)
    }
}
```

---

**Next:** Learn about observability with [Event Tracing](./08-event-tracing.md) →
