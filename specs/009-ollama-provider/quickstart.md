# Quickstart: Ollama Model Provider

**Date**: 2025-11-14
**Branch**: `009-ollama-provider`

## Prerequisites

1. **Go 1.21+** installed
2. **Ollama** installed and running:
   ```bash
   # Install Ollama (macOS/Linux)
   curl -fsSL https://ollama.com/install.sh | sh

   # Pull a model
   ollama pull llama3.2

   # Start Ollama server
   ollama serve
   ```

3. **LangGraph-Go** project cloned:
   ```bash
   git clone https://github.com/dshills/langgraph-go.git
   cd langgraph-go
   git checkout 009-ollama-provider
   ```

## 5-Minute Quickstart

### Step 1: Import the Package

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/dshills/langgraph-go/graph/model"
    "github.com/dshills/langgraph-go/graph/model/ollama"
)
```

### Step 2: Create the Adapter

```go
func main() {
    // Configure the adapter
    config := ollama.Config{
        Model: "llama3.2", // Model you pulled with 'ollama pull'
    }

    // Create the adapter
    adapter, err := ollama.NewChatModel(config)
    if err != nil {
        log.Fatalf("Failed to create adapter: %v", err)
    }

    fmt.Println("✓ Ollama adapter created")
```

### Step 3: Send a Message

```go
    // Create a message
    messages := []model.Message{
        {Role: model.RoleUser, Content: "What is the capital of France?"},
    }

    // Send to Ollama
    ctx := context.Background()
    out, err := adapter.Chat(ctx, messages, nil)
    if err != nil {
        log.Fatalf("Chat failed: %v", err)
    }

    // Print response
    fmt.Printf("Response: %s\n", out.Text)
    fmt.Printf("Model: %s\n", out.Meta["model"])
}
```

### Step 4: Run

```bash
go run main.go
```

**Expected Output**:
```
✓ Ollama adapter created
Response: The capital of France is Paris.
Model: llama3.2
```

---

## Common Use Cases

### 1. Local Development (Default)

```go
// Uses local Ollama at http://localhost:11434
config := ollama.Config{
    Model: "llama3.2",
}
adapter, _ := ollama.NewChatModel(config)
```

**When to use**:
- Development and testing
- Privacy-sensitive workflows
- Offline execution
- Zero API costs

---

### 2. Remote Ollama Instance

```go
// Connect to remote Ollama server
config := ollama.Config{
    Endpoint: "http://ollama-server.example.com:11434",
    Model:    "mistral",
}
adapter, _ := ollama.NewChatModel(config)
```

**When to use**:
- Team shared Ollama instance
- Containerized deployments (Docker, Kubernetes)
- Resource-constrained local machines
- GPU-accelerated remote server

---

### 3. Conversation with History

```go
messages := []model.Message{
    {Role: model.RoleSystem, Content: "You are a helpful coding assistant."},
    {Role: model.RoleUser, Content: "Write a function to reverse a string in Go"},
    {Role: model.RoleAssistant, Content: "Here's a function:\n```go\nfunc reverse(s string) string {...}```"},
    {Role: model.RoleUser, Content: "Now add error handling"},
}

out, _ := adapter.Chat(ctx, messages, nil)
fmt.Println(out.Text) // Improved function with error handling
```

**When to use**:
- Multi-turn conversations
- Context-aware responses
- Follow-up questions

---

### 4. Tool Calling (Agentic Workflows)

```go
// Define tools
tools := []model.ToolSpec{
    {
        Name:        "get_weather",
        Description: "Get current weather for a location",
        Schema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "location": map[string]interface{}{
                    "type":        "string",
                    "description": "City name",
                },
            },
            "required": []string{"location"},
        },
    },
}

// Send message with tools
messages := []model.Message{
    {Role: model.RoleUser, Content: "What's the weather in Paris?"},
}

out, _ := adapter.Chat(ctx, messages, tools)

// Check for tool calls
for _, call := range out.ToolCalls {
    fmt.Printf("Tool: %s, Input: %v\n", call.Name, call.Input)
    // Execute tool and send result back to LLM
}
```

**When to use**:
- Agentic workflows (ReAct, agents)
- Function calling patterns
- External API integration

---

### 5. Deterministic Generation (Testing)

```go
seed := 42
config := ollama.Config{
    Model:       "llama3.2",
    Seed:        &seed,
    Temperature: 0.0, // Minimum randomness
}
adapter, _ := ollama.NewChatModel(config)

// Same inputs will always produce identical outputs
```

**When to use**:
- Unit testing
- Regression testing
- Reproducible experiments

---

### 6. Custom Timeouts

```go
// Set timeout on context
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

out, err := adapter.Chat(ctx, messages, nil)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("Request timed out after 30 seconds")
}
```

**When to use**:
- Rate limiting
- Latency requirements
- Resource-constrained environments

---

### 7. Custom HTTP Client (Proxies, TLS)

```go
httpClient := &http.Client{
    Timeout: 60 * time.Second,
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // For self-signed certs
    },
}

config := ollama.Config{
    Model:      "llama3.2",
    HTTPClient: httpClient,
}
adapter, _ := ollama.NewChatModel(config)
```

**When to use**:
- Corporate proxies
- Self-signed SSL certificates
- Custom transport requirements

---

## LangGraph Integration

### Use Ollama in a Graph Workflow

```go
package main

import (
    "context"
    "fmt"

    "github.com/dshills/langgraph-go/graph"
    "github.com/dshills/langgraph-go/graph/model"
    "github.com/dshills/langgraph-go/graph/model/ollama"
    "github.com/dshills/langgraph-go/graph/store"
    "github.com/dshills/langgraph-go/graph/emit"
)

type State struct {
    Messages []model.Message
    Result   string
}

func main() {
    // Create Ollama adapter
    adapter, _ := ollama.NewChatModel(ollama.Config{Model: "llama3.2"})

    // Create LLM node
    llmNode := graph.NodeFunc[State](func(ctx context.Context, s State) graph.NodeResult[State] {
        out, err := adapter.Chat(ctx, s.Messages, nil)
        if err != nil {
            return graph.NodeResult[State]{Err: err}
        }

        return graph.NodeResult[State]{
            Delta: State{Result: out.Text},
            Route: graph.Stop(),
        }
    })

    // Create engine
    reducer := func(prev, delta State) State {
        if delta.Result != "" {
            prev.Result = delta.Result
        }
        return prev
    }

    store := store.NewMemStore[State]()
    emitter := emit.NewStdoutEmitter[State]()
    engine := graph.New(reducer, store, emitter)

    // Wire graph
    engine.AddNode("llm", llmNode)
    engine.AddEdge("__start__", "llm")

    // Execute workflow
    initialState := State{
        Messages: []model.Message{
            {Role: model.RoleUser, Content: "Explain LangGraph in one sentence"},
        },
    }

    finalState, _ := engine.Run(context.Background(), "run-1", initialState)
    fmt.Printf("Result: %s\n", finalState.Result)
}
```

---

## Troubleshooting

### Error: "Failed to connect to Ollama"

**Solution**:
```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# If not running, start it:
ollama serve
```

---

### Error: "Model 'llama3.2' not available"

**Solution**:
```bash
# Pull the model
ollama pull llama3.2

# List available models
ollama list
```

---

### Error: "context deadline exceeded"

**Cause**: Request took longer than context timeout.

**Solution**:
```go
// Increase timeout
ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
defer cancel()

// Or use larger/faster model
config := ollama.Config{Model: "llama3.2:1b"} // Smaller, faster model
```

---

### Error: "tool calling not supported"

**Cause**: Model doesn't support tool/function calling.

**Solution**:
```bash
# Use a tool-capable model
ollama pull llama3.1  # Supports tools
ollama pull mistral   # Supports tools
```

---

## Next Steps

1. **Read the API Contract**: See [contracts/api-contract.md](./contracts/api-contract.md)
2. **Review Examples**: Check `examples/ollama/main.go`
3. **Run Tests**: `go test ./graph/model/ollama/...`
4. **Explore Models**: Visit [Ollama Model Library](https://ollama.com/library)

---

## Model Recommendations

| Use Case | Model | Why |
|----------|-------|-----|
| General chat | `llama3.2` | Balanced performance/quality |
| Code generation | `codellama` | Specialized for code |
| Fast responses | `llama3.2:1b` | Lightweight, fast |
| Reasoning | `llama3.1:70b` | High quality (needs GPU) |
| Tool calling | `llama3.1` | Native tool support |
| French/multilingual | `mistral` | Strong multilingual |

---

## Performance Tips

1. **Use smaller models for speed**: `llama3.2:1b` vs `llama3.2:8b`
2. **Set `NumPredict` to limit generation**: `NumPredict: 100` for short responses
3. **Use `Temperature: 0.0` for deterministic outputs**: Reduces variance
4. **Run Ollama on GPU**: Dramatically faster inference (requires GPU support)
5. **Batch requests**: Reuse adapter instance across multiple calls (thread-safe)

---

## Summary

You've learned:
- ✓ How to create and configure the Ollama adapter
- ✓ Basic chat with local Ollama
- ✓ Remote instance configuration
- ✓ Tool calling for agentic workflows
- ✓ LangGraph integration patterns
- ✓ Common troubleshooting steps

**Time to first response**: ~5 minutes (including Ollama setup)
