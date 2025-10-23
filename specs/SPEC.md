---
title: LangGraph-Go Specification
version: 0.1
status: draft
authors:
  - Davin Hills
license: MIT
---

# LangGraph-Go Specification

## Table of Contents
1. [Overview](#overview)
2. [Core Concepts](#1-core-concepts)
3. [State Model](#2-state-model)
4. [Node Abstraction](#3-node-abstraction)
5. [Routing and Edges](#4-routing-and-edges)
6. [Engine](#5-engine)
7. [Persistence (Store)](#6-persistence-store)
8. [Observability and Events](#7-observability-and-events)
9. [LLM and Tool Interfaces](#8-llm-and-tool-interfaces)
10. [Example Workflow](#9-example-workflow)
11. [Concurrency and Fan-Out](#10-concurrency-and-fan-out)
12. [Error Handling](#11-error-handling)
13. [Tracing and Visualization](#12-tracing-and-visualization)
14. [Example Unit Test](#13-example-unit-test)
15. [Package Layout](#14-package-layout)
16. [Design Goals](#15-design-goals)
17. [Future Extensions](#16-future-extensions)
18. [Summary](#17-summary)

---

## Overview

**LangGraph-Go** is a Go-native orchestration framework for building **stateful, graph-based LLM and tool workflows**.  
It models reasoning pipelines as **graphs of nodes**, where each node represents a computation step (LLM call, tool, or logic block), and data/state flows through edges between them.

LangGraph-Go provides:
- Type-safe graph orchestration using Go generics.
- Persistent, resumable execution with checkpoints.
- Deterministic replay for debugging and analysis.
- Integration with LLMs, tools, and traditional functions.

---

## 1. Core Concepts

LangGraph-Go represents a program as a directed graph:

- **Nodes**: processing units (LLM calls, tools, or functions).  
- **Edges**: define control flow between nodes.  
- **State**: shared context that evolves through node outputs.  
- **Reducers**: merge partial state deltas deterministically.

Graphs support **loops**, **branches**, **fan-out**, and **stateful recovery**.

---

## 2. State Model

### 2.1 State Definition

```go
type Session struct {
    Query     string
    Plan      []string
    Docs      []Doc
    Tool      string
    Answer    string
    Turns     int
    LastError string
}
```

### 2.2 Reducer Function

```go
type Reducer[S any] func(prev S, delta S) S
```

Example:

```go
func ReduceSession(prev, delta Session) Session {
    if delta.Query != ""     { prev.Query = delta.Query }
    if delta.Tool != ""      { prev.Tool = delta.Tool }
    if delta.Answer != ""    { prev.Answer = delta.Answer }
    if delta.LastError != "" { prev.LastError = delta.LastError }
    if delta.Plan != nil     { prev.Plan = append([]string{}, delta.Plan...) }
    if delta.Docs != nil     { prev.Docs = append([]Doc{}, delta.Docs...) }
    if delta.Turns != 0      { prev.Turns = delta.Turns }
    return prev
}
```

---

## 3. Node Abstraction

### 3.1 Node Interface

```go
type Node[S any] interface {
    Run(ctx context.Context, state S) NodeResult[S]
}
```

### 3.2 Functional Node

```go
type NodeFunc[S any] func(ctx context.Context, state S) NodeResult[S]
func (f NodeFunc[S]) Run(ctx context.Context, s S) NodeResult[S] { return f(ctx, s) }
```

### 3.3 Node Result

```go
type NodeResult[S any] struct {
    Delta   S
    Route   Next
    Events  []Event
    Err     error
}
```

---

## 4. Routing and Edges

### 4.1 Next Hop

```go
type Next struct {
    To       string
    Terminal bool
}

func Stop() Next          { return Next{Terminal: true} }
func Goto(id string) Next { return Next{To: id} }
```

### 4.2 Conditional Edges

```go
type Predicate[S any] func(S) bool

type Edge[S any] struct {
    From, To string
    When     Predicate[S]
}
```

---

## 5. Engine

### 5.1 Definition

```go
type Engine[S any] struct {
    reducer Reducer[S]
    nodes   map[string]Node[S]
    edges   map[string][]Edge[S]
    store   Store[S]
    emitter Emitter
    opts    Options
    start   string
}
```

### 5.2 Options

```go
type Options struct {
    MaxSteps int
    Retries  int
}
```

### 5.3 Execution Flow

1. Begin at the start node.
2. Execute the node.
3. Merge delta into state.
4. Determine next hop via node result or edge predicates.
5. Persist state.
6. Repeat until terminal, error, or max step reached.

### 5.4 Execution

```go
func (e *Engine[S]) Run(ctx context.Context, runID string, initial S) (S, error)
```

---

## 6. Persistence (Store)

### 6.1 Store Interface

```go
type Store[S any] interface {
    SaveStep(ctx context.Context, runID string, step int, nodeID string, state S) error
    LoadLatest(ctx context.Context, runID string) (state S, step int, nodeID string, _ error)
    SaveCheckpoint(ctx context.Context, runID, label string, state S, step int, nodeID string) error
    LoadCheckpoint(ctx context.Context, runID, label string) (state S, step int, nodeID string, _ error)
}
```

### 6.2 Example SQL Schema

```sql
CREATE TABLE runs (
  run_id VARCHAR(64) PRIMARY KEY,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE steps (
  run_id VARCHAR(64) NOT NULL,
  step_no INT NOT NULL,
  node_id VARCHAR(64) NOT NULL,
  state_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (run_id, step_no)
);

CREATE TABLE checkpoints (
  run_id VARCHAR(64) NOT NULL,
  label VARCHAR(64) NOT NULL,
  step_no INT NOT NULL,
  node_id VARCHAR(64) NOT NULL,
  state_json JSON NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (run_id, label)
);
```

---

## 7. Observability and Events

### 7.1 Event Definition

```go
type Event struct {
    RunID  string
    Step   int
    NodeID string
    Msg    string
    Meta   map[string]any
}
```

### 7.2 Emitter Interface

```go
type Emitter interface {
    Emit(Event)
}
```

Emitters can log to console, file, OpenTelemetry, or JSON output.

---

## 8. LLM and Tool Interfaces

### 8.1 Tool Interface

```go
type Tool interface {
    Name() string
    Call(ctx context.Context, input any) (any, error)
}
```

### 8.2 Chat Model Interface

```go
type Message struct{ Role, Content string }

type ChatModel interface {
    Chat(ctx context.Context, messages []Message, tools []ToolSpec) (ChatOut, error)
}

type ToolSpec struct{ Name, Description string; Schema any }

type ChatOut struct {
    Text      string
    ToolCalls []struct{ Name string; Args any }
}
```

---

## 9. Example Workflow

### 9.1 Node Definitions

```go
plan := graph.NodeFunc[Session](func(ctx context.Context, s Session) graph.NodeResult[Session] {
    return graph.NodeResult[Session]{Delta: Session{Plan: []string{"search", "summarize"}}, Route: graph.Goto("retrieve")}
})

retrieve := graph.NodeFunc[Session](func(ctx context.Context, s Session) graph.NodeResult[Session] {
    docs := searchIndex(s.Query)
    return graph.NodeResult[Session]{Delta: Session{Docs: docs}, Route: graph.Goto("answer")}
})

answer := graph.NodeFunc[Session](func(ctx context.Context, s Session) graph.NodeResult[Session] {
    ans := summarize(s.Docs)
    return graph.NodeResult[Session]{Delta: Session{Answer: ans, Turns: s.Turns + 1}}
})

judge := graph.NodeFunc[Session](func(ctx context.Context, s Session) graph.NodeResult[Session] {
    if len(s.Answer) < 200 && s.Turns < 3 {
        return graph.NodeResult[Session]{Route: graph.Goto("retrieve")}
    }
    return graph.NodeResult[Session]{Route: graph.Stop()}
})
```

### 9.2 Graph Wiring

```go
g := graph.New[Session](ReduceSession, NewMemStore[Session](), NewLogEmitter(), graph.Options{MaxSteps: 20, Retries: 1})
g.Add("plan", plan)
g.Add("retrieve", retrieve)
g.Add("answer", answer)
g.Add("judge", judge)
g.StartAt("plan")
g.Connect("answer", "judge", nil)

final, err := g.Run(ctx, "run-123", Session{Query: "How does n8n compare to LangGraph?"})
```

---

## 10. Concurrency and Fan-Out

Nodes can return multiple next hops to enable parallel execution.

```go
type Next struct {
    To       string
    Many     []string
    Terminal bool
}
```

Branches execute concurrently with state isolation and merge at a Join node using the reducer.

---

## 11. Error Handling

- Node errors (`NodeResult.Err`) trigger retry or routing to an error node.  
- Retries configurable via `Options.Retries`.  
- Persistent `LastError` in state for downstream logic.  

---

## 12. Tracing and Visualization

Execution events can be visualized:
- Render DOT/PNG graph.
- Export run history.
- Stream to OpenTelemetry or JSON for observability.

---

## 13. Example Unit Test

```go
func TestSmallLoop(t *testing.T) {
    type S struct{ N int }
    reduce := func(p, d S) S { if d.N != 0 { p.N = d.N }; return p }

    inc := NodeFunc[S](func(ctx context.Context, s S) NodeResult[S] {
        return NodeResult[S]{Delta: S{N: s.N + 1}}
    })
    stopIf3 := NodeFunc[S](func(ctx context.Context, s S) NodeResult[S] {
        if s.N >= 3 { return NodeResult[S]{Route: Stop()} }
        return NodeResult[S]{Route: Goto("inc")}
    })

    g := New[S](reduce, NewMemStore[S](), nil, Options{MaxSteps: 10})
    g.Add("inc", inc)
    g.Add("check", stopIf3)
    g.StartAt("inc")
    g.Connect("inc", "check", nil)

    out, err := g.Run(context.Background(), "t1", S{})
    require.NoError(t, err)
    require.Equal(t, 3, out.N)
}
```

---

## 14. Package Layout

```
/graph
  engine.go        // Engine, wiring, runner
  node.go          // Node, NodeFunc, Next, Edge
  state.go         // Reducer, helpers
  store/
    memory.go      // In-memory store for tests
    mysql.go       // Aurora/MySQL implementation
  emit/
    log.go         // Stdout logger
    otel.go        // OpenTelemetry emitter
  model/
    chat.go        // ChatModel interface + adapters
    openai.go      // OpenAI adapter
    ollama.go      // Local adapter
  tool/
    tool.go        // Tool interface
    http.go        // Example HTTP tool
```

---

## 15. Design Goals

| Goal | Description |
|------|--------------|
| **Deterministic replay** | Every run can be resumed or re-simulated. |
| **Type safety** | Strongly typed state with Go generics. |
| **Low dependencies** | Pure Go core, external adapters optional. |
| **Composable** | Loops, branches, and fan-out supported. |
| **Production-ready** | Checkpointing, persistence, observability. |

---

## 16. Future Extensions

- HTTP/GraphQL runtime API for graph control.  
- Visual editor for workflow construction.  
- CRDT-based distributed reducers.  
- Async node scheduling for long jobs.  
- Policy framework for access control.  

---

## 17. Summary

LangGraph-Go provides a **strongly-typed**, **replayable**, and **composable** framework for orchestrating reasoning workflows in Go.

It is:
- Deterministic and testable.  
- Extensible for LLMs, tools, and external systems.  
- Suitable for both AI reasoning and non-AI task orchestration.

LangGraph-Go is designed for **clarity, composability, and control**—a Go-native evolution of the LangGraph architecture.
