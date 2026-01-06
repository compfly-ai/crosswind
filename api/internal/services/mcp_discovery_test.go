package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindMessageField(t *testing.T) {
	tests := []struct {
		name     string
		schema   map[string]interface{}
		expected string
	}{
		{
			name: "finds message field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": map[string]interface{}{
						"type":        "string",
						"description": "The message to send",
					},
				},
				"required": []interface{}{"message"},
			},
			expected: "message",
		},
		{
			name: "finds text field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type": "string",
					},
					"option": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expected: "text",
		},
		{
			name: "finds prompt field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{
						"type": "string",
					},
					"temperature": map[string]interface{}{
						"type": "number",
					},
				},
			},
			expected: "prompt",
		},
		{
			name: "finds query field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type": "string",
					},
					"limit": map[string]interface{}{
						"type": "integer",
					},
				},
			},
			expected: "query",
		},
		{
			name: "finds input field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expected: "input",
		},
		{
			name: "finds content field",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"content": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expected: "content",
		},
		{
			name: "priority order: message over text",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"text": map[string]interface{}{
						"type": "string",
					},
					"message": map[string]interface{}{
						"type": "string",
					},
				},
			},
			expected: "message",
		},
		{
			name: "fallback to first string property",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"custom_field": map[string]interface{}{
						"type": "string",
					},
					"count": map[string]interface{}{
						"type": "integer",
					},
				},
			},
			expected: "custom_field",
		},
		{
			name:     "empty schema defaults to message",
			schema:   map[string]interface{}{},
			expected: "message",
		},
		{
			name: "no properties defaults to message",
			schema: map[string]interface{}{
				"type": "object",
			},
			expected: "message",
		},
		{
			name:     "nil schema defaults to message",
			schema:   nil,
			expected: "message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindMessageField(tt.schema)
			if result != tt.expected {
				t.Errorf("FindMessageField() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSendMCPRequest_SSEParsing(t *testing.T) {
	// Test SSE response format parsing
	tests := []struct {
		name        string
		response    string
		contentType string
		expectError bool
	}{
		{
			name: "SSE format with event and data",
			response: `event: message
data: {"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"Test","version":"1.0"}}}

`,
			contentType: "text/event-stream",
			expectError: false,
		},
		{
			name:        "plain JSON format",
			response:    `{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"Test","version":"1.0"}}}`,
			contentType: "application/json",
			expectError: false,
		},
		{
			name: "SSE format without data line",
			response: `event: message

`,
			contentType: "text/event-stream",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Create agent service with nil dependencies (we're testing the HTTP layer)
			svc := &AgentService{}

			// Send request
			req := jsonRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "2024-11-05",
				},
			}

			requestBuilder := NewSafeHTTPRequestBuilder()
			result, err := svc.sendMCPRequest(
				t.Context(),
				&http.Client{},
				requestBuilder,
				server.URL,
				req,
				nil,
				"",
			)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected result, got nil")
				}
			}
		})
	}
}

func TestSendMCPRequest_SessionID(t *testing.T) {
	// Test session ID handling
	capturedSessionID := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture incoming session ID header
		capturedSessionID = r.Header.Get("mcp-session-id")

		// Return response with session ID
		w.Header().Set("mcp-session-id", "server-session-123")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer server.Close()

	svc := &AgentService{}
	requestBuilder := NewSafeHTTPRequestBuilder()

	// Test 1: No session ID on first request
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	result, err := svc.sendMCPRequest(
		t.Context(),
		&http.Client{},
		requestBuilder,
		server.URL,
		req,
		nil,
		"", // No session ID
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSessionID != "" {
		t.Errorf("expected no session ID header, got %q", capturedSessionID)
	}
	if result.SessionID != "server-session-123" {
		t.Errorf("expected session ID 'server-session-123', got %q", result.SessionID)
	}

	// Test 2: Session ID passed on subsequent request
	capturedSessionID = ""
	_, err = svc.sendMCPRequest(
		t.Context(),
		&http.Client{},
		requestBuilder,
		server.URL,
		req,
		nil,
		"client-session-456", // Passing session ID
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSessionID != "client-session-456" {
		t.Errorf("expected session ID header 'client-session-456', got %q", capturedSessionID)
	}
}

func TestSendMCPRequest_AuthHeaders(t *testing.T) {
	capturedHeaders := make(map[string]string)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture auth headers
		capturedHeaders["Authorization"] = r.Header.Get("Authorization")
		capturedHeaders["X-API-Key"] = r.Header.Get("X-API-Key")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer server.Close()

	svc := &AgentService{}

	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	authHeaders := map[string]string{
		"Authorization": "Bearer test-token",
		"X-API-Key":     "api-key-123",
	}

	requestBuilder := NewSafeHTTPRequestBuilder()
	_, err := svc.sendMCPRequest(
		t.Context(),
		&http.Client{},
		requestBuilder,
		server.URL,
		req,
		authHeaders,
		"",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedHeaders["Authorization"] != "Bearer test-token" {
		t.Errorf("expected Authorization header 'Bearer test-token', got %q", capturedHeaders["Authorization"])
	}
	if capturedHeaders["X-API-Key"] != "api-key-123" {
		t.Errorf("expected X-API-Key header 'api-key-123', got %q", capturedHeaders["X-API-Key"])
	}
}

func TestSendMCPRequest_ErrorHandling(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		response     string
		expectError  bool
		errorMessage string
	}{
		{
			name:        "non-200 status",
			statusCode:  404,
			response:    "Not Found",
			expectError: true,
		},
		{
			name:       "JSON-RPC error response",
			statusCode: 200,
			response:   `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`,
			expectError: true,
		},
		{
			name:        "invalid JSON response",
			statusCode:  200,
			response:    "not json",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			svc := &AgentService{}
			requestBuilder := NewSafeHTTPRequestBuilder()

			req := jsonRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "test",
			}

			_, err := svc.sendMCPRequest(
				t.Context(),
				&http.Client{},
				requestBuilder,
				server.URL,
				req,
				nil,
				"",
			)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestMCPDiscoveryFlow tests the full discovery flow with a mock MCP server
func TestMCPDiscoveryFlow(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("mcp-session-id", "test-session-id")

		var response interface{}

		switch req.Method {
		case "initialize":
			requestCount++
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"serverInfo": map[string]interface{}{
						"name":    "Test MCP Server",
						"version": "1.0.0",
					},
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
					"protocolVersion": "2024-11-05",
				},
			}
		case "notifications/initialized":
			// No response for notification
			requestCount++
			w.WriteHeader(http.StatusOK)
			return
		case "tools/list":
			requestCount++
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "chat",
							"description": "Send a chat message for customer support",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"message": map[string]interface{}{
										"type":        "string",
										"description": "The user message",
									},
								},
								"required": []interface{}{"message"},
							},
						},
						{
							"name":        "search",
							"description": "Search for documents",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"query": map[string]interface{}{
										"type": "string",
									},
								},
							},
						},
					},
				},
			}
		default:
			t.Errorf("unexpected method: %s", req.Method)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	svc := &AgentService{}

	result, err := svc.DiscoverMCPTool(
		t.Context(),
		server.URL,
		"chat",
		"streamable_http",
		nil,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tool info
	if result.Tool.Name != "chat" {
		t.Errorf("expected tool name 'chat', got %q", result.Tool.Name)
	}
	if result.Tool.Description != "Send a chat message for customer support" {
		t.Errorf("unexpected description: %s", result.Tool.Description)
	}
	if result.Tool.InputSchema == nil {
		t.Error("expected inputSchema to be populated")
	}

	// Verify server info
	if result.Server.Name != "Test MCP Server" {
		t.Errorf("expected server name 'Test MCP Server', got %q", result.Server.Name)
	}
	if result.Server.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", result.Server.Version)
	}

	// Verify available tools
	if len(result.AvailableTools) != 2 {
		t.Errorf("expected 2 available tools, got %d", len(result.AvailableTools))
	}
}

func TestMCPDiscoveryFlow_ToolNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req jsonRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("mcp-session-id", "test-session")

		var response interface{}

		switch req.Method {
		case "initialize":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"serverInfo": map[string]interface{}{
						"name":    "Test",
						"version": "1.0",
					},
				},
			}
		case "notifications/initialized":
			w.WriteHeader(http.StatusOK)
			return
		case "tools/list":
			response = map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result": map[string]interface{}{
					"tools": []map[string]interface{}{
						{
							"name":        "other_tool",
							"description": "Not the tool you're looking for",
							"inputSchema": map[string]interface{}{},
						},
					},
				},
			}
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	svc := &AgentService{}

	// Use streamable_http transport since SSE requires real event stream
	_, err := svc.DiscoverMCPTool(
		t.Context(),
		server.URL,
		"nonexistent_tool",
		"streamable_http",
		nil,
	)

	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
	if err != nil && !contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
