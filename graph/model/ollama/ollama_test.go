package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dshills/langgraph-go/graph/model"
	"github.com/ollama/ollama/api"
)

// T013: Mock HTTP server setup for testing Ollama API responses
func setupMockOllamaServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

// mockOllamaHandler returns a handler that responds with a valid Ollama ChatResponse
func mockOllamaHandler(response api.ChatResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if !strings.HasSuffix(r.URL.Path, "/api/chat") {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

// mockOllamaErrorHandler returns a handler that returns an error response
func mockOllamaErrorHandler(statusCode int, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": message,
		})
	}
}

// T014: Test for NewChatModel() constructor with valid config
func TestNewChatModel_ValidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "minimal config with required fields",
			config: Config{
				Model: "gpt-oss",
			},
		},
		{
			name: "full config with all fields",
			config: Config{
				Endpoint:    "http://localhost:11434",
				Model:       "llama3.2",
				Temperature: Float64Ptr(0.7),
				TopP:        Float64Ptr(0.9),
				NumPredict:  IntPtr(2048),
				HTTPClient:  &http.Client{Timeout: 30 * time.Second},
			},
		},
		{
			name: "config with seed",
			config: Config{
				Model: "mistral",
				Seed:  IntPtr(42),
			},
		},
		{
			name: "config with custom endpoint",
			config: Config{
				Endpoint: "https://ollama.example.com",
				Model:    "codellama",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatModel, err := NewChatModel(tt.config)
			if err != nil {
				t.Fatalf("NewChatModel() unexpected error = %v", err)
			}

			if chatModel == nil {
				t.Fatal("NewChatModel() returned nil model")
			}

			// Verify model implements ChatModel interface
			var _ model.ChatModel = chatModel
		})
	}
}

// T015: Test for NewChatModel() constructor with invalid config (empty Model)
func TestNewChatModel_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "empty model name",
			config: Config{
				Model: "",
			},
			wantErr: "model is required",
		},
		{
			name: "temperature out of range - too low",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(-0.1),
			},
			wantErr: "temperature must be between",
		},
		{
			name: "temperature out of range - too high",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(2.5),
			},
			wantErr: "temperature must be between",
		},
		{
			name: "top_p out of range - too low",
			config: Config{
				Model: "gpt-oss",
				TopP:  Float64Ptr(-0.1),
			},
			wantErr: "top_p must be between",
		},
		{
			name: "top_p out of range - too high",
			config: Config{
				Model: "gpt-oss",
				TopP:  Float64Ptr(1.5),
			},
			wantErr: "top_p must be between",
		},
		{
			name: "num_predict invalid",
			config: Config{
				Model:      "gpt-oss",
				NumPredict: IntPtr(-2),
			},
			wantErr: "num_predict must be >= -1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewChatModel(tt.config)
			if err == nil {
				t.Fatal("NewChatModel() expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NewChatModel() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

// T016: Test for successful Chat() call with mock Ollama response
func TestChatModel_Chat_Success(t *testing.T) {
	tests := []struct {
		name         string
		messages     []model.Message
		tools        []model.ToolSpec
		mockResponse api.ChatResponse
		wantText     string
		wantMetaKeys []string
	}{
		{
			name: "simple text response",
			messages: []model.Message{
				{Role: model.RoleUser, Content: "What is the capital of France?"},
			},
			tools: nil,
			mockResponse: api.ChatResponse{
				Model:     "gpt-oss",
				CreatedAt: time.Now(),
				Message: api.Message{
					Role:    "assistant",
					Content: "The capital of France is Paris.",
				},
				Done: true,
				Metrics: api.Metrics{
					TotalDuration:      time.Second,
					PromptEvalCount:    10,
					PromptEvalDuration: 100 * time.Millisecond,
					EvalCount:          8,
					EvalDuration:       900 * time.Millisecond,
				},
			},
			wantText:     "The capital of France is Paris.",
			wantMetaKeys: []string{"model", "created_at", "done", "total_duration", "prompt_eval_count", "eval_count"},
		},
		{
			name: "multi-turn conversation",
			messages: []model.Message{
				{Role: model.RoleSystem, Content: "You are a helpful assistant."},
				{Role: model.RoleUser, Content: "Hello"},
				{Role: model.RoleAssistant, Content: "Hi there!"},
				{Role: model.RoleUser, Content: "How are you?"},
			},
			tools: nil,
			mockResponse: api.ChatResponse{
				Model:     "llama3.2",
				CreatedAt: time.Now(),
				Message: api.Message{
					Role:    "assistant",
					Content: "I'm doing well, thank you for asking!",
				},
				Done: true,
				Metrics: api.Metrics{
					TotalDuration: 2 * time.Second,
				},
			},
			wantText:     "I'm doing well, thank you for asking!",
			wantMetaKeys: []string{"model", "done"},
		},
		{
			name: "empty response content",
			messages: []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			},
			tools: nil,
			mockResponse: api.ChatResponse{
				Model: "gpt-oss",
				Message: api.Message{
					Role:    "assistant",
					Content: "",
				},
				Done: true,
			},
			wantText:     "",
			wantMetaKeys: []string{"model", "done"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock server
			server := setupMockOllamaServer(t, mockOllamaHandler(tt.mockResponse))

			// Create ChatModel with mock endpoint
			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			// Call Chat
			ctx := context.Background()
			out, err := chatModel.Chat(ctx, tt.messages, tt.tools)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}

			// Verify text content
			if out.Text != tt.wantText {
				t.Errorf("Chat() Text = %q, want %q", out.Text, tt.wantText)
			}

			// Verify metadata keys exist
			for _, key := range tt.wantMetaKeys {
				if _, exists := out.Meta[key]; !exists {
					t.Errorf("Chat() Meta missing key %q", key)
				}
			}
		})
	}
}

// T017: Test for message translation (model.Message → api.Message)
func TestChatModel_MessageTranslation(t *testing.T) {
	tests := []struct {
		name         string
		messages     []model.Message
		validateFunc func(t *testing.T, r *http.Request)
	}{
		{
			name: "translate single user message",
			messages: []model.Message{
				{Role: model.RoleUser, Content: "Hello"},
			},
			validateFunc: func(t *testing.T, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				if len(req.Messages) != 1 {
					t.Fatalf("expected 1 message, got %d", len(req.Messages))
				}

				if req.Messages[0].Role != "user" {
					t.Errorf("expected role 'user', got %q", req.Messages[0].Role)
				}

				if req.Messages[0].Content != "Hello" {
					t.Errorf("expected content 'Hello', got %q", req.Messages[0].Content)
				}
			},
		},
		{
			name: "translate system, user, assistant messages",
			messages: []model.Message{
				{Role: model.RoleSystem, Content: "You are helpful."},
				{Role: model.RoleUser, Content: "Question"},
				{Role: model.RoleAssistant, Content: "Answer"},
			},
			validateFunc: func(t *testing.T, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				if len(req.Messages) != 3 {
					t.Fatalf("expected 3 messages, got %d", len(req.Messages))
				}

				roles := []string{"system", "user", "assistant"}
				contents := []string{"You are helpful.", "Question", "Answer"}

				for i, expectedRole := range roles {
					if req.Messages[i].Role != expectedRole {
						t.Errorf("message[%d] role = %q, want %q", i, req.Messages[i].Role, expectedRole)
					}
					if req.Messages[i].Content != contents[i] {
						t.Errorf("message[%d] content = %q, want %q", i, req.Messages[i].Content, contents[i])
					}
				}
			},
		},
		{
			name:     "translate empty message array",
			messages: []model.Message{},
			validateFunc: func(t *testing.T, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				if len(req.Messages) != 0 {
					t.Errorf("expected 0 messages, got %d", len(req.Messages))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock server with request validation
			handler := func(w http.ResponseWriter, r *http.Request) {
				tt.validateFunc(t, r)

				// Return valid response
				response := api.ChatResponse{
					Model: "gpt-oss",
					Message: api.Message{
						Role:    "assistant",
						Content: "Response",
					},
					Done: true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			server := setupMockOllamaServer(t, handler)

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			// Call Chat to trigger message translation
			ctx := context.Background()
			_, err = chatModel.Chat(ctx, tt.messages, nil)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}
		})
	}
}

// T018: Test for response parsing (api.ChatResponse → model.ChatOut)
func TestChatModel_ResponseParsing(t *testing.T) {
	tests := []struct {
		name          string
		mockResponse  api.ChatResponse
		wantText      string
		wantToolCalls int
	}{
		{
			name: "parse text response",
			mockResponse: api.ChatResponse{
				Model: "gpt-oss",
				Message: api.Message{
					Role:    "assistant",
					Content: "Hello, world!",
				},
				Done: true,
			},
			wantText:      "Hello, world!",
			wantToolCalls: 0,
		},
		{
			name: "parse response with tool calls",
			mockResponse: api.ChatResponse{
				Model: "gpt-oss",
				Message: api.Message{
					Role:    "assistant",
					Content: "Let me check that for you.",
					ToolCalls: []api.ToolCall{
						{
							Function: api.ToolCallFunction{
								Name: "get_weather",
								Arguments: api.ToolCallFunctionArguments{
									"location": "Paris",
								},
							},
						},
					},
				},
				Done: true,
			},
			wantText:      "Let me check that for you.",
			wantToolCalls: 1,
		},
		{
			name: "parse response with multiple tool calls",
			mockResponse: api.ChatResponse{
				Model: "gpt-oss",
				Message: api.Message{
					Role:    "assistant",
					Content: "",
					ToolCalls: []api.ToolCall{
						{
							Function: api.ToolCallFunction{
								Name: "search",
								Arguments: api.ToolCallFunctionArguments{
									"query": "Go programming",
								},
							},
						},
						{
							Function: api.ToolCallFunction{
								Name: "calculate",
								Arguments: api.ToolCallFunctionArguments{
									"expression": "2+2",
								},
							},
						},
					},
				},
				Done: true,
			},
			wantText:      "",
			wantToolCalls: 2,
		},
		{
			name: "parse empty response",
			mockResponse: api.ChatResponse{
				Model: "gpt-oss",
				Message: api.Message{
					Role:    "assistant",
					Content: "",
				},
				Done: true,
			},
			wantText:      "",
			wantToolCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupMockOllamaServer(t, mockOllamaHandler(tt.mockResponse))

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			out, err := chatModel.Chat(ctx, messages, nil)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}

			// Verify text parsing
			if out.Text != tt.wantText {
				t.Errorf("Chat() Text = %q, want %q", out.Text, tt.wantText)
			}

			// Verify tool calls parsing
			if len(out.ToolCalls) != tt.wantToolCalls {
				t.Errorf("Chat() ToolCalls count = %d, want %d", len(out.ToolCalls), tt.wantToolCalls)
			}

			// Verify tool call details if present
			if tt.wantToolCalls > 0 {
				for i, tc := range out.ToolCalls {
					ollamaTC := tt.mockResponse.Message.ToolCalls[i]

					if tc.Name != ollamaTC.Function.Name {
						t.Errorf("ToolCall[%d].Name = %q, want %q", i, tc.Name, ollamaTC.Function.Name)
					}

					if tc.Input == nil {
						t.Errorf("ToolCall[%d].Input is nil", i)
					}
				}
			}
		})
	}
}

// T019: Test for metadata extraction in ChatOut.Meta
func TestChatModel_MetadataExtraction(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse api.ChatResponse
		checkMeta    func(t *testing.T, meta map[string]interface{})
	}{
		{
			name: "extract model metadata",
			mockResponse: api.ChatResponse{
				Model: "gpt-oss",
				Message: api.Message{
					Role:    "assistant",
					Content: "Response",
				},
				Done: true,
			},
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				if model, ok := meta["model"].(string); !ok || model != "gpt-oss" {
					t.Errorf("Meta['model'] = %v, want 'gpt-oss'", meta["model"])
				}

				if done, ok := meta["done"].(bool); !ok || !done {
					t.Errorf("Meta['done'] = %v, want true", meta["done"])
				}
			},
		},
		{
			name: "extract timing metadata",
			mockResponse: api.ChatResponse{
				Model: "llama3.2",
				Message: api.Message{
					Role:    "assistant",
					Content: "Response",
				},
				Done: true,
				Metrics: api.Metrics{
					TotalDuration:      1234567890 * time.Nanosecond,
					LoadDuration:       123456789 * time.Nanosecond,
					PromptEvalDuration: 234567890 * time.Nanosecond,
					EvalDuration:       876543210 * time.Nanosecond,
				},
			},
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				if td, ok := meta["total_duration"].(time.Duration); !ok || td != 1234567890*time.Nanosecond {
					t.Errorf("Meta['total_duration'] = %v, want %v", meta["total_duration"], 1234567890*time.Nanosecond)
				}

				if ld, ok := meta["load_duration"].(time.Duration); !ok || ld != 123456789*time.Nanosecond {
					t.Errorf("Meta['load_duration'] = %v, want %v", meta["load_duration"], 123456789*time.Nanosecond)
				}
			},
		},
		{
			name: "extract token count metadata",
			mockResponse: api.ChatResponse{
				Model: "mistral",
				Message: api.Message{
					Role:    "assistant",
					Content: "Response",
				},
				Done: true,
				Metrics: api.Metrics{
					PromptEvalCount: 25,
					EvalCount:       50,
				},
			},
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				if pec, ok := meta["prompt_eval_count"].(int); !ok || pec != 25 {
					t.Errorf("Meta['prompt_eval_count'] = %v, want 25", meta["prompt_eval_count"])
				}

				if ec, ok := meta["eval_count"].(int); !ok || ec != 50 {
					t.Errorf("Meta['eval_count'] = %v, want 50", meta["eval_count"])
				}
			},
		},
		{
			name: "extract stop reason metadata",
			mockResponse: api.ChatResponse{
				Model: "codellama",
				Message: api.Message{
					Role:    "assistant",
					Content: "Response",
				},
				Done:       true,
				DoneReason: "stop",
			},
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				if dr, ok := meta["done_reason"].(string); !ok || dr != "stop" {
					t.Errorf("Meta['done_reason'] = %v, want 'stop'", meta["done_reason"])
				}
			},
		},
		{
			name: "extract created_at metadata",
			mockResponse: api.ChatResponse{
				Model:     "gpt-oss",
				CreatedAt: time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC),
				Message: api.Message{
					Role:    "assistant",
					Content: "Response",
				},
				Done: true,
			},
			checkMeta: func(t *testing.T, meta map[string]interface{}) {
				if ca, ok := meta["created_at"].(time.Time); !ok {
					t.Errorf("Meta['created_at'] type = %T, want time.Time", meta["created_at"])
				} else if ca.Year() != 2025 {
					t.Errorf("Meta['created_at'].Year() = %d, want 2025", ca.Year())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupMockOllamaServer(t, mockOllamaHandler(tt.mockResponse))

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			out, err := chatModel.Chat(ctx, messages, nil)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}

			// Verify metadata
			if out.Meta == nil {
				t.Fatal("Chat() Meta is nil, want non-nil map")
			}

			tt.checkMeta(t, out.Meta)
		})
	}
}

// Additional test: Context cancellation
func TestChatModel_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	handler := func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		response := api.ChatResponse{
			Model: "gpt-oss",
			Message: api.Message{
				Role:    "assistant",
				Content: "Too late!",
			},
			Done: true,
		}
		json.NewEncoder(w).Encode(response)
	}

	server := setupMockOllamaServer(t, handler)

	config := Config{
		Endpoint: server.URL,
		Model:    "gpt-oss",
	}
	chatModel, err := NewChatModel(config)
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	messages := []model.Message{
		{Role: model.RoleUser, Content: "Test"},
	}

	_, err = chatModel.Chat(ctx, messages, nil)
	if err == nil {
		t.Fatal("Chat() expected context.Canceled error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Chat() error = %v, want context.Canceled", err)
	}
}

// Additional test: HTTP error handling
func TestChatModel_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
	}{
		{
			name:       "500 internal server error",
			statusCode: http.StatusInternalServerError,
			message:    "internal server error",
		},
		{
			name:       "404 not found",
			statusCode: http.StatusNotFound,
			message:    "endpoint not found",
		},
		{
			name:       "503 service unavailable",
			statusCode: http.StatusServiceUnavailable,
			message:    "service unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := setupMockOllamaServer(t, mockOllamaErrorHandler(tt.statusCode, tt.message))

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			_, err = chatModel.Chat(ctx, messages, nil)
			if err == nil {
				t.Fatal("Chat() expected error, got nil")
			}
		})
	}
}

// ============================================================================
// Phase 5 (US3): Model Configuration Tests
// ============================================================================

// T041: Test for Seed parameter (deterministic generation)
func TestChatModel_SeedParameter(t *testing.T) {
	tests := []struct {
		name     string
		seed     *int
		wantSeed bool
	}{
		{
			name:     "with seed for deterministic generation",
			seed:     IntPtr(42),
			wantSeed: true,
		},
		{
			name:     "without seed for non-deterministic generation",
			seed:     nil,
			wantSeed: false,
		},
		{
			name:     "with zero seed",
			seed:     IntPtr(0),
			wantSeed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handler validates that seed is passed correctly
			handler := func(w http.ResponseWriter, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				// Verify seed in options
				if tt.wantSeed {
					seedVal, hasSeed := req.Options["seed"]
					if !hasSeed {
						t.Errorf("expected seed in options, but not found")
					} else {
						// Convert to int for comparison
						seedInt, ok := seedVal.(float64)
						if !ok {
							t.Errorf("seed type = %T, want float64 (JSON number)", seedVal)
						} else if int(seedInt) != *tt.seed {
							t.Errorf("seed = %v, want %v", int(seedInt), *tt.seed)
						}
					}
				} else {
					if _, hasSeed := req.Options["seed"]; hasSeed {
						t.Errorf("expected no seed in options, but found one")
					}
				}

				// Return valid response
				response := api.ChatResponse{
					Model: "gpt-oss",
					Message: api.Message{
						Role:    "assistant",
						Content: "Response",
					},
					Done: true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			server := setupMockOllamaServer(t, handler)

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
				Seed:     tt.seed,
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			_, err = chatModel.Chat(ctx, messages, nil)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}
		})
	}
}

// T042: Test for parameter transmission to Ollama API
func TestChatModel_ParameterTransmission(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantOptions map[string]interface{}
	}{
		{
			name: "all parameters",
			config: Config{
				Model:       "gpt-oss",
				Temperature: Float64Ptr(0.7),
				TopP:        Float64Ptr(0.95),
				NumPredict:  IntPtr(100),
				Seed:        IntPtr(42),
			},
			wantOptions: map[string]interface{}{
				"temperature": 0.7,
				"top_p":       0.95,
				"num_predict": 100.0, // JSON numbers are float64
				"seed":        42.0,
			},
		},
		{
			name: "only temperature and topP",
			config: Config{
				Model:       "llama3.2",
				Temperature: Float64Ptr(0.5),
				TopP:        Float64Ptr(0.8),
			},
			wantOptions: map[string]interface{}{
				"temperature": 0.5,
				"top_p":       0.8,
				"num_predict": -1.0, // Default
			},
		},
		{
			name: "zero temperature for deterministic output",
			config: Config{
				Model:       "mistral",
				Temperature: Float64Ptr(0.0),
				Seed:        IntPtr(123),
			},
			wantOptions: map[string]interface{}{
				"temperature": 0.0,
				"seed":        123.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handler validates that all parameters are transmitted correctly
			handler := func(w http.ResponseWriter, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				// Verify all expected options are present
				for key, expectedVal := range tt.wantOptions {
					actualVal, exists := req.Options[key]
					if !exists {
						t.Errorf("expected option %q not found in request", key)
						continue
					}

					// Compare as float64 (JSON numbers)
					expectedFloat, _ := expectedVal.(float64)
					actualFloat, ok := actualVal.(float64)
					if !ok {
						t.Errorf("option %q type = %T, want float64", key, actualVal)
					} else if actualFloat != expectedFloat {
						t.Errorf("option %q = %v, want %v", key, actualFloat, expectedFloat)
					}
				}

				// Return valid response
				response := api.ChatResponse{
					Model: "gpt-oss",
					Message: api.Message{
						Role:    "assistant",
						Content: "Response",
					},
					Done: true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			server := setupMockOllamaServer(t, handler)
			tt.config.Endpoint = server.URL

			chatModel, err := NewChatModel(tt.config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			_, err = chatModel.Chat(ctx, messages, nil)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}
		})
	}
}

// T043: Test for model name in request
func TestChatModel_ModelNameInRequest(t *testing.T) {
	tests := []struct {
		name      string
		modelName string
	}{
		{
			name:      "gpt-oss model",
			modelName: "gpt-oss",
		},
		{
			name:      "llama3.2 model",
			modelName: "llama3.2",
		},
		{
			name:      "mistral model",
			modelName: "mistral",
		},
		{
			name:      "codellama model",
			modelName: "codellama",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handler validates that model name is transmitted correctly
			handler := func(w http.ResponseWriter, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				// Verify model name
				if req.Model != tt.modelName {
					t.Errorf("request Model = %q, want %q", req.Model, tt.modelName)
				}

				// Return valid response
				response := api.ChatResponse{
					Model: tt.modelName,
					Message: api.Message{
						Role:    "assistant",
						Content: "Response",
					},
					Done: true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			server := setupMockOllamaServer(t, handler)

			config := Config{
				Endpoint: server.URL,
				Model:    tt.modelName,
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			_, err = chatModel.Chat(ctx, messages, nil)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}
		})
	}
}

// ============================================================================
// Phase 6 (US4): Tool Calling Tests
// ============================================================================

// T049: Test for tool spec translation (model.ToolSpec → api.Tool)
func TestChatModel_ToolSpecTranslation(t *testing.T) {
	tests := []struct {
		name      string
		tools     []model.ToolSpec
		wantTools int
		validate  func(t *testing.T, reqTools []api.Tool)
	}{
		{
			name: "single tool with simple schema",
			tools: []model.ToolSpec{
				{
					Name:        "get_weather",
					Description: "Get current weather",
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
			},
			wantTools: 1,
			validate: func(t *testing.T, reqTools []api.Tool) {
				if len(reqTools) != 1 {
					t.Fatalf("expected 1 tool, got %d", len(reqTools))
				}

				tool := reqTools[0]
				if tool.Type != "function" {
					t.Errorf("tool.Type = %q, want 'function'", tool.Type)
				}
				if tool.Function.Name != "get_weather" {
					t.Errorf("tool.Function.Name = %q, want 'get_weather'", tool.Function.Name)
				}
				if tool.Function.Description != "Get current weather" {
					t.Errorf("tool.Function.Description = %q, want 'Get current weather'", tool.Function.Description)
				}

				// Verify parameters structure
				params := tool.Function.Parameters
				if params.Type != "object" {
					t.Errorf("params.Type = %q, want 'object'", params.Type)
				}
				if len(params.Properties) != 1 {
					t.Errorf("params.Properties length = %d, want 1", len(params.Properties))
				}
				if len(params.Required) != 1 || params.Required[0] != "location" {
					t.Errorf("params.Required = %v, want ['location']", params.Required)
				}
			},
		},
		{
			name: "multiple tools",
			tools: []model.ToolSpec{
				{
					Name:        "search",
					Description: "Search the web",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
				{
					Name:        "calculate",
					Description: "Calculate expression",
					Schema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"expression": map[string]interface{}{
								"type": "string",
							},
						},
					},
				},
			},
			wantTools: 2,
			validate: func(t *testing.T, reqTools []api.Tool) {
				if len(reqTools) != 2 {
					t.Fatalf("expected 2 tools, got %d", len(reqTools))
				}

				toolNames := []string{reqTools[0].Function.Name, reqTools[1].Function.Name}
				expectedNames := []string{"search", "calculate"}

				for i, expectedName := range expectedNames {
					if toolNames[i] != expectedName {
						t.Errorf("tool[%d].Function.Name = %q, want %q", i, toolNames[i], expectedName)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handler validates tool translation
			handler := func(w http.ResponseWriter, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				// Validate tools
				if len(req.Tools) != tt.wantTools {
					t.Errorf("request has %d tools, want %d", len(req.Tools), tt.wantTools)
				}

				if tt.validate != nil {
					tt.validate(t, req.Tools)
				}

				// Return valid response
				response := api.ChatResponse{
					Model: "gpt-oss",
					Message: api.Message{
						Role:    "assistant",
						Content: "Response",
					},
					Done: true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			server := setupMockOllamaServer(t, handler)

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			_, err = chatModel.Chat(ctx, messages, tt.tools)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}
		})
	}
}

// T051: Test for JSON schema pass-through in tool definitions
func TestChatModel_ToolSchemaPassThrough(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]interface{}
	}{
		{
			name: "schema with enum",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"unit": map[string]interface{}{
						"type":        "string",
						"description": "Temperature unit",
						"enum":        []interface{}{"celsius", "fahrenheit"},
					},
				},
			},
		},
		{
			name: "schema with array type",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"tags": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
			},
		},
		{
			name: "schema with $defs",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"address": map[string]interface{}{
						"type": "object",
					},
				},
				"$defs": map[string]interface{}{
					"Address": map[string]interface{}{
						"type": "object",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := []model.ToolSpec{
				{
					Name:        "test_tool",
					Description: "Test tool",
					Schema:      tt.schema,
				},
			}

			// Handler validates schema pass-through
			handler := func(w http.ResponseWriter, r *http.Request) {
				var req api.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				if len(req.Tools) != 1 {
					t.Fatalf("expected 1 tool, got %d", len(req.Tools))
				}

				tool := req.Tools[0]
				params := tool.Function.Parameters

				// Verify schema type preserved
				if schemaType, ok := tt.schema["type"].(string); ok {
					if params.Type != schemaType {
						t.Errorf("params.Type = %q, want %q", params.Type, schemaType)
					}
				}

				// Verify $defs passed through
				if _, hasDefs := tt.schema["$defs"]; hasDefs {
					if params.Defs == nil {
						t.Error("expected $defs in params, but not found")
					}
				}

				// Return valid response
				response := api.ChatResponse{
					Model: "gpt-oss",
					Message: api.Message{
						Role:    "assistant",
						Content: "Response",
					},
					Done: true,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}

			server := setupMockOllamaServer(t, handler)

			config := Config{
				Endpoint: server.URL,
				Model:    "gpt-oss",
			}
			chatModel, err := NewChatModel(config)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			messages := []model.Message{
				{Role: model.RoleUser, Content: "Test"},
			}

			ctx := context.Background()
			_, err = chatModel.Chat(ctx, messages, tools)
			if err != nil {
				t.Fatalf("Chat() unexpected error = %v", err)
			}
		})
	}
}

// T053: Test for tool call with invalid JSON arguments (error handling)
func TestChatModel_ToolCallInvalidJSON(t *testing.T) {
	// Note: The Ollama API returns structured data, so we test graceful handling
	// of missing or nil arguments rather than malformed JSON
	mockResponse := api.ChatResponse{
		Model: "gpt-oss",
		Message: api.Message{
			Role:    "assistant",
			Content: "Let me help with that.",
			ToolCalls: []api.ToolCall{
				{
					Function: api.ToolCallFunction{
						Name:      "get_weather",
						Arguments: nil, // No arguments provided
					},
				},
			},
		},
		Done: true,
	}

	server := setupMockOllamaServer(t, mockOllamaHandler(mockResponse))

	config := Config{
		Endpoint: server.URL,
		Model:    "gpt-oss",
	}
	chatModel, err := NewChatModel(config)
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "What's the weather?"},
	}

	ctx := context.Background()
	out, err := chatModel.Chat(ctx, messages, nil)
	if err != nil {
		t.Fatalf("Chat() unexpected error = %v", err)
	}

	// Should still parse tool call even with nil/empty arguments
	if len(out.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(out.ToolCalls))
	}

	if out.ToolCalls[0].Name != "get_weather" {
		t.Errorf("ToolCall.Name = %q, want 'get_weather'", out.ToolCalls[0].Name)
	}

	// Input should be empty map, not nil
	if out.ToolCalls[0].Input == nil {
		t.Error("ToolCall.Input is nil, want empty map")
	}
}

// T054: Test for model without tool support (graceful degradation)
func TestChatModel_NoToolSupport(t *testing.T) {
	// Simulates a model that doesn't support tools by returning regular text response
	mockResponse := api.ChatResponse{
		Model: "gpt-oss",
		Message: api.Message{
			Role:    "assistant",
			Content: "I can help with the weather, but I need more information.",
			// No ToolCalls in response
		},
		Done: true,
	}

	server := setupMockOllamaServer(t, mockOllamaHandler(mockResponse))

	config := Config{
		Endpoint: server.URL,
		Model:    "gpt-oss",
	}
	chatModel, err := NewChatModel(config)
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}

	// Provide tools even though model may not support them
	tools := []model.ToolSpec{
		{
			Name:        "get_weather",
			Description: "Get weather",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	messages := []model.Message{
		{Role: model.RoleUser, Content: "What's the weather in Paris?"},
	}

	ctx := context.Background()
	out, err := chatModel.Chat(ctx, messages, tools)
	if err != nil {
		t.Fatalf("Chat() unexpected error = %v", err)
	}

	// Should gracefully degrade to text response
	if out.Text == "" {
		t.Error("expected text response, got empty")
	}

	if len(out.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls (graceful degradation), got %d", len(out.ToolCalls))
	}
}
