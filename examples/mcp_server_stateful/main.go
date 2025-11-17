// Package main implements an MCP server that demonstrates resource exposure
// capabilities for stateful workflows.
//
// This example shows how to:
// - Register dynamic resources that expose workflow state
// - Provide checkpoint access via resources
// - Register static resources for configuration
// - Integrate resources with LangGraph Store for state persistence
//
// Usage:
//
//	go build -o stateful-server
//	./stateful-server
//
// The server communicates over stdio and can be connected to Claude Desktop
// or other MCP clients by adding it to the MCP configuration.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dshills/langgraph-go/graph/emit"
	"github.com/dshills/langgraph-go/graph/mcp"
	"github.com/dshills/langgraph-go/graph/store"
)

// WorkflowState represents the state tracked by the stateful workflow.
// This state is exposed via MCP resources to allow LLM clients to
// inspect and understand the current workflow execution.
type WorkflowState struct {
	CurrentStep  string            // Current step name in the workflow
	StepCount    int               // Number of steps executed
	Data         map[string]string // Arbitrary workflow data
	LastUpdate   time.Time         // When state was last updated
	ErrorMessage string            // Last error encountered (if any)
}

// reducer merges state updates deterministically.
// This enables the workflow state to be tracked, checkpointed, and exposed via resources.
func reducer(prev, delta WorkflowState) WorkflowState {
	if delta.CurrentStep != "" {
		prev.CurrentStep = delta.CurrentStep
	}
	if delta.StepCount != 0 {
		prev.StepCount = delta.StepCount
	}
	if delta.Data != nil {
		if prev.Data == nil {
			prev.Data = make(map[string]string)
		}
		for k, v := range delta.Data {
			prev.Data[k] = v
		}
	}
	if !delta.LastUpdate.IsZero() {
		prev.LastUpdate = delta.LastUpdate
	}
	if delta.ErrorMessage != "" {
		prev.ErrorMessage = delta.ErrorMessage
	}
	return prev
}

// ProcessTool implements a simple workflow processing tool that updates state.
// This demonstrates how tools and resources work together in a stateful workflow.
type ProcessTool struct {
	store store.Store[WorkflowState]
}

// Name returns the unique identifier for this tool.
func (p *ProcessTool) Name() string {
	return "process_step"
}

// Call executes a workflow step and updates the persistent state.
//
// Input schema:
//
//	{
//	  "step_name": string (required) - Name of the step to process
//	  "data": object (optional) - Data to merge into workflow state
//	}
//
// Output schema:
//
//	{
//	  "success": bool - Whether the step completed successfully
//	  "step_count": int - Total number of steps processed
//	  "message": string - Status message
//	}
func (p *ProcessTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate required parameter
	stepName, ok := input["step_name"].(string)
	if !ok || stepName == "" {
		return nil, fmt.Errorf("step_name parameter required (must be non-empty string)")
	}

	// Check context cancellation before processing
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Load current state from store
	currentState, stepNum, err := p.store.LoadLatest(ctx, "workflow-run-001")
	if err != nil && err != store.ErrNotFound {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// If no state exists, initialize it
	if err == store.ErrNotFound {
		currentState = WorkflowState{
			Data: make(map[string]string),
		}
		stepNum = 0
	}

	// Process optional data parameter
	stepData := make(map[string]string)
	if data, ok := input["data"].(map[string]interface{}); ok {
		for k, v := range data {
			if strVal, ok := v.(string); ok {
				stepData[k] = strVal
			}
		}
	}

	// Update state with new step
	newState := reducer(currentState, WorkflowState{
		CurrentStep: stepName,
		StepCount:   currentState.StepCount + 1,
		Data:        stepData,
		LastUpdate:  time.Now(),
	})

	// Save updated state to store
	err = p.store.SaveStep(ctx, "workflow-run-001", stepNum+1, "process_step", newState)
	if err != nil {
		// Update state with error and save it
		errorState := reducer(newState, WorkflowState{
			ErrorMessage: err.Error(),
			LastUpdate:   time.Now(),
		})
		_ = p.store.SaveStep(ctx, "workflow-run-001", stepNum+1, "process_step", errorState)
		return nil, fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("Processed step '%s' - total steps: %d", stepName, newState.StepCount)

	return map[string]interface{}{
		"success":    true,
		"step_count": newState.StepCount,
		"message":    fmt.Sprintf("Step '%s' processed successfully", stepName),
	}, nil
}

// CheckpointTool creates a named checkpoint of the current workflow state.
type CheckpointTool struct {
	store store.Store[WorkflowState]
}

// Name returns the unique identifier for this tool.
func (c *CheckpointTool) Name() string {
	return "create_checkpoint"
}

// Call creates a named checkpoint of the current workflow state.
//
// Input schema:
//
//	{
//	  "label": string (required) - Name for the checkpoint
//	}
//
// Output schema:
//
//	{
//	  "success": bool - Whether checkpoint was created
//	  "label": string - Checkpoint label
//	  "message": string - Status message
//	}
func (c *CheckpointTool) Call(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate required parameter
	label, ok := input["label"].(string)
	if !ok || label == "" {
		return nil, fmt.Errorf("label parameter required (must be non-empty string)")
	}

	// Check context cancellation before processing
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Load current state
	currentState, stepNum, err := c.store.LoadLatest(ctx, "workflow-run-001")
	if err != nil {
		return nil, fmt.Errorf("failed to load current state: %w", err)
	}

	// Save checkpoint with unique ID combining run and label
	checkpointID := fmt.Sprintf("workflow-run-001:%s", label)
	err = c.store.SaveCheckpoint(ctx, checkpointID, currentState, stepNum)
	if err != nil {
		return nil, fmt.Errorf("failed to save checkpoint: %w", err)
	}

	log.Printf("Created checkpoint '%s' at step %d", label, stepNum)

	return map[string]interface{}{
		"success": true,
		"label":   label,
		"message": fmt.Sprintf("Checkpoint '%s' created successfully", label),
	}, nil
}

func main() {
	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping server...")
		cancel()
	}()

	// Create LangGraph components
	// In production, use a persistent store like MySQL
	st := store.NewMemStore[WorkflowState]()
	emitter := emit.NewLogEmitter(os.Stdout, false)

	// Initialize workflow state
	initialState := WorkflowState{
		CurrentStep: "initialized",
		StepCount:   0,
		Data:        make(map[string]string),
		LastUpdate:  time.Now(),
	}
	err := st.SaveStep(ctx, "workflow-run-001", 0, "init", initialState)
	if err != nil {
		log.Fatalf("Failed to initialize state: %v", err)
	}

	// Create MCP server with configuration
	mcpServer := mcp.NewServer(mcp.ServerConfig{
		Name:    "langgraph-stateful",
		Version: "1.0.0",
		Emitter: emitter,
	})

	// Register workflow tools
	processTool := &ProcessTool{store: st}
	err = mcpServer.RegisterTool("process_step", processTool, mcp.ToolMetadata{
		Description: "Process a workflow step and update state",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"step_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the step to process",
				},
				"data": map[string]interface{}{
					"type":        "object",
					"description": "Optional data to merge into workflow state",
				},
			},
			"required": []string{"step_name"},
		},
	})
	if err != nil {
		log.Fatalf("Failed to register process_step tool: %v", err)
	}

	checkpointTool := &CheckpointTool{store: st}
	err = mcpServer.RegisterTool("create_checkpoint", checkpointTool, mcp.ToolMetadata{
		Description: "Create a named checkpoint of current workflow state",
		Schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"label": map[string]interface{}{
					"type":        "string",
					"description": "Name for the checkpoint",
				},
			},
			"required": []string{"label"},
		},
	})
	if err != nil {
		log.Fatalf("Failed to register create_checkpoint tool: %v", err)
	}

	log.Println("Tools registered successfully")

	// Register dynamic resource: Current workflow state (T058)
	err = mcpServer.RegisterDynamicResource(
		"workflow_state/current",
		"Current Workflow State",
		"The current state of the workflow execution including step count, current step, and data",
		"application/json",
		func(ctx context.Context) ([]byte, error) {
			state, stepNum, err := st.LoadLatest(ctx, "workflow-run-001")
			if err != nil {
				return nil, fmt.Errorf("failed to load workflow state: %w", err)
			}

			// Create enriched response with metadata
			response := map[string]interface{}{
				"state":    state,
				"step":     stepNum,
				"run_id":   "workflow-run-001",
				"accessed": time.Now().Format(time.RFC3339),
			}

			return json.Marshal(response)
		},
	)
	if err != nil {
		log.Fatalf("Failed to register workflow_state/current resource: %v", err)
	}

	// Register dynamic resource: Checkpoint retrieval (T059)
	err = mcpServer.RegisterDynamicResource(
		"workflow_checkpoints/get",
		"Workflow Checkpoint",
		"Retrieve a named checkpoint from workflow execution history",
		"application/json",
		func(ctx context.Context) ([]byte, error) {
			// Note: In a real implementation, you would pass the checkpoint label
			// via a parameter. For this example, we'll try to load a default checkpoint.
			// MCP resources don't support parameters directly, but you could:
			// 1. Use different resource URIs for different checkpoints
			// 2. Use a tool to set which checkpoint to expose
			// 3. Return a list of all checkpoints

			// For demonstration, we'll return a helpful message about checkpoint usage
			response := map[string]interface{}{
				"message": "To retrieve checkpoints, use the create_checkpoint tool to create named checkpoints, then query them by label",
				"example": "Use create_checkpoint with label 'before_validation' to create a checkpoint",
				"note":    "In a production system, you would register separate resources per checkpoint or use a parameter mechanism",
			}

			return json.Marshal(response)
		},
	)
	if err != nil {
		log.Fatalf("Failed to register workflow_checkpoints/get resource: %v", err)
	}

	// Register static resource: Server configuration
	configJSON := []byte(`{
		"server": "langgraph-stateful",
		"version": "1.0.0",
		"capabilities": {
			"tools": ["process_step", "create_checkpoint"],
			"resources": ["workflow_state/current", "workflow_checkpoints/get", "config/server"],
			"features": ["stateful_workflows", "checkpointing", "persistent_state"]
		},
		"store": {
			"type": "memory",
			"production_recommendation": "Use MySQL/Aurora for production workloads"
		}
	}`)

	err = mcpServer.RegisterStaticResource(
		"config/server",
		"Server Configuration",
		"Static configuration information about the MCP server and its capabilities",
		"application/json",
		configJSON,
	)
	if err != nil {
		log.Fatalf("Failed to register config/server resource: %v", err)
	}

	log.Println("Resources registered successfully")
	log.Println("")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("MCP Server: langgraph-stateful")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("")
	log.Println("Available Tools:")
	log.Println("  • process_step: Process a workflow step and update state")
	log.Println("  • create_checkpoint: Create a named checkpoint of current state")
	log.Println("")
	log.Println("Available Resources:")
	log.Println("  • workflow_state/current: Current workflow execution state")
	log.Println("  • workflow_checkpoints/get: Retrieve workflow checkpoints")
	log.Println("  • config/server: Server configuration and capabilities")
	log.Println("")
	log.Println("Example Workflow:")
	log.Println("  1. Call process_step with step_name='validate_input'")
	log.Println("  2. Read workflow_state/current to see updated state")
	log.Println("  3. Call create_checkpoint with label='after_validation'")
	log.Println("  4. Call process_step with step_name='process_data'")
	log.Println("  5. Read workflow_state/current to see final state")
	log.Println("")
	log.Println("Configure Claude Desktop with:")
	log.Println(`  "langgraph-stateful": {`)
	log.Printf(`    "command": "%s"`, getExecutablePath())
	log.Println(`  }`)
	log.Println("")

	// Start MCP server (blocks until context is cancelled)
	if err := mcpServer.Start(ctx); err != nil {
		// Check if this is the expected "not yet implemented" error
		errMsg := err.Error()
		if errMsg == "failed to create transport: transport creation not yet implemented - will be completed in transport integration phase" {
			log.Println("")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("Note: MCP server transport layer is in development.")
			log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			log.Println("")
			log.Println("This example demonstrates the resource registration API:")
			log.Println("")
			log.Println("Dynamic Resources (computed on-demand):")
			log.Println("  ✓ workflow_state/current: Exposes Store.LoadLatest()")
			log.Println("  ✓ workflow_checkpoints/get: Demonstrates checkpoint access")
			log.Println("")
			log.Println("Static Resources (fixed content):")
			log.Println("  ✓ config/server: Server configuration JSON")
			log.Println("")
			log.Println("Integration with LangGraph:")
			log.Println("  • State persisted via store.Store[WorkflowState]")
			log.Println("  • Tools update state through reducer function")
			log.Println("  • Resources expose state to LLM clients")
			log.Println("  • Checkpoints enable workflow resumption")
			log.Println("")
			log.Println("Once transport is implemented, LLM clients can:")
			log.Println("  • Call tools to modify workflow state")
			log.Println("  • Read resources to inspect current state")
			log.Println("  • Create and retrieve checkpoints")
			log.Println("  • Resume workflows from checkpoints")
			return
		}
		log.Fatalf("MCP server error: %v", err)
	}

	log.Println("MCP server stopped cleanly")
}

// getExecutablePath returns the absolute path to the current executable.
// This is used to generate the Claude Desktop configuration example.
func getExecutablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "/path/to/stateful-server"
	}
	return exe
}
