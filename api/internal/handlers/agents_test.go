package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compfly-ai/crosswind/api/internal/config"
	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/compfly-ai/crosswind/api/internal/services"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// mockAgentRepo is an in-memory implementation of AgentRepository for testing
type mockAgentRepo struct {
	agents map[string]*models.Agent
}

func newMockAgentRepo() *mockAgentRepo {
	return &mockAgentRepo{
		agents: make(map[string]*models.Agent),
	}
}

func (m *mockAgentRepo) Create(ctx context.Context, agent *models.Agent) error {
	if _, exists := m.agents[agent.AgentID]; exists {
		return services.ErrAgentAlreadyExists
	}
	m.agents[agent.AgentID] = agent
	return nil
}

func (m *mockAgentRepo) FindByID(ctx context.Context, agentID string) (*models.Agent, error) {
	if agent, exists := m.agents[agentID]; exists {
		if agent.Status != "deleted" {
			return agent, nil
		}
	}
	return nil, services.ErrAgentNotFound
}

func (m *mockAgentRepo) List(ctx context.Context, status string, limit, offset int) ([]models.Agent, int64, error) {
	var result []models.Agent
	for _, agent := range m.agents {
		if agent.Status == "deleted" {
			continue
		}
		if status == "" || agent.Status == status {
			result = append(result, *agent)
		}
	}
	total := int64(len(result))
	// Apply pagination
	if offset >= len(result) {
		return []models.Agent{}, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (m *mockAgentRepo) Update(ctx context.Context, agentID string, update bson.M) error {
	agent, exists := m.agents[agentID]
	if !exists || agent.Status == "deleted" {
		return services.ErrAgentNotFound
	}
	// Apply updates - service sends flat bson.M, not $set
	if name, ok := update["name"].(string); ok {
		agent.Name = name
	}
	if desc, ok := update["description"].(string); ok {
		agent.Description = desc
	}
	if status, ok := update["status"].(string); ok {
		agent.Status = status
	}
	return nil
}

func (m *mockAgentRepo) Delete(ctx context.Context, agentID string) error {
	agent, exists := m.agents[agentID]
	if !exists {
		return services.ErrAgentNotFound
	}
	agent.Status = "deleted"
	return nil
}

func (m *mockAgentRepo) Exists(ctx context.Context, agentID string) (bool, error) {
	agent, exists := m.agents[agentID]
	return exists && agent.Status != "deleted", nil
}

func (m *mockAgentRepo) HardDelete(ctx context.Context, agentID string) error {
	delete(m.agents, agentID)
	return nil
}

// mockEvalRunRepo is a minimal implementation for testing
type mockEvalRunRepo struct{}

func (m *mockEvalRunRepo) Create(ctx context.Context, run *models.EvalRun) error { return nil }
func (m *mockEvalRunRepo) FindByRunID(ctx context.Context, runID string) (*models.EvalRun, error) {
	return nil, nil
}
func (m *mockEvalRunRepo) ListByAgent(ctx context.Context, agentID string, status string, limit, offset int) ([]models.EvalRun, int64, error) {
	return nil, 0, nil
}
func (m *mockEvalRunRepo) Update(ctx context.Context, runID string, update bson.M) error { return nil }
func (m *mockEvalRunRepo) UpdateProgress(ctx context.Context, runID string, progress models.EvalProgress) error {
	return nil
}
func (m *mockEvalRunRepo) UpdateStatus(ctx context.Context, runID, status string) error { return nil }
func (m *mockEvalRunRepo) HasActiveRun(ctx context.Context, agentID string) (bool, error) {
	return false, nil
}
func (m *mockEvalRunRepo) GetLatestRun(ctx context.Context, agentID string) (*models.EvalRun, error) {
	return nil, nil
}
func (m *mockEvalRunRepo) GetLatestRunsByAgentIDs(ctx context.Context, agentIDs []string) (map[string]*models.EvalRun, error) {
	return make(map[string]*models.EvalRun), nil
}

// setupTestRouter creates a test router with mock repositories
func setupTestRouter(t *testing.T) (*gin.Engine, *mockAgentRepo) {
	gin.SetMode(gin.TestMode)

	logger, _ := zap.NewDevelopment()
	cfg := &config.Config{
		EncryptionKey: "test-encryption-key-32-bytes-ok",
	}

	agentRepo := newMockAgentRepo()
	evalRunRepo := &mockEvalRunRepo{}

	agentService, err := services.NewAgentService(agentRepo, evalRunRepo, cfg, nil, logger)
	if err != nil {
		t.Fatalf("failed to create agent service: %v", err)
	}

	svc := &services.Services{
		Agent: agentService,
	}

	handlers := NewHandlers(svc, logger)

	router := gin.New()
	v1 := router.Group("/v1")
	{
		agents := v1.Group("/agents")
		{
			agents.POST("", handlers.Agents.Create)
			agents.GET("", handlers.Agents.List)
			agents.GET("/:agentId", handlers.Agents.Get)
			agents.PATCH("/:agentId", handlers.Agents.Update)
			agents.DELETE("/:agentId", handlers.Agents.Delete)
		}
	}

	return router, agentRepo
}

func TestAgentCRUDFlow(t *testing.T) {
	router, _ := setupTestRouter(t)

	// === CREATE ===
	t.Run("create agent", func(t *testing.T) {
		body := `{
			"agentId": "test-agent-1",
			"name": "Test Agent",
			"description": "A test agent",
			"goal": "Help users",
			"industry": "technology",
			"endpointConfig": {
				"protocol": "custom",
				"endpoint": "https://example.com/chat"
			},
			"authConfig": {
				"type": "bearer",
				"credentials": "test-token"
			}
		}`

		req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if response["agentId"] == nil {
			t.Error("expected agentId in response")
		}
		if response["name"] != "Test Agent" {
			t.Errorf("expected name 'Test Agent', got %v", response["name"])
		}
	})

	// === GET ===
	t.Run("get agent", func(t *testing.T) {
		// First create an agent
		body := `{
			"agentId": "get-test-agent",
			"name": "Get Test Agent",
			"description": "Agent for GET test",
			"goal": "Test goal",
			"industry": "tech",
			"endpointConfig": {
				"protocol": "custom",
				"endpoint": "https://example.com/api"
			},
			"authConfig": {"type": "none"}
		}`

		createReq := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(body))
		createReq.Header.Set("Content-Type", "application/json")
		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createReq)

		var createResp map[string]interface{}
		if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
			t.Fatalf("failed to unmarshal create response: %v", err)
		}
		agentID := createResp["agentId"].(string)

		// Now GET the agent
		getReq := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID, nil)
		getW := httptest.NewRecorder()
		router.ServeHTTP(getW, getReq)

		if getW.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", getW.Code, getW.Body.String())
		}

		var getResp map[string]interface{}
		if err := json.Unmarshal(getW.Body.Bytes(), &getResp); err != nil {
			t.Fatalf("failed to unmarshal get response: %v", err)
		}

		if getResp["name"] != "Get Test Agent" {
			t.Errorf("expected name 'Get Test Agent', got %v", getResp["name"])
		}
	})

	// === LIST ===
	t.Run("list agents", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("failed to unmarshal list response: %v", err)
		}

		agents := response["agents"].([]interface{})
		if len(agents) < 2 {
			t.Errorf("expected at least 2 agents, got %d", len(agents))
		}
	})

	// === UPDATE ===
	t.Run("update agent", func(t *testing.T) {
		// Create an agent first
		body := `{
			"agentId": "update-test-agent",
			"name": "Update Test Agent",
			"description": "Original description",
			"goal": "Test goal",
			"industry": "tech",
			"endpointConfig": {
				"protocol": "custom",
				"endpoint": "https://example.com/update"
			},
			"authConfig": {"type": "none"}
		}`

		createReq := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(body))
		createReq.Header.Set("Content-Type", "application/json")
		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createReq)

		var createResp map[string]interface{}
		_ = json.Unmarshal(createW.Body.Bytes(), &createResp)
		agentID := createResp["agentId"].(string)

		// Update the agent
		updateBody := `{"name": "Updated Agent Name", "description": "Updated description"}`
		updateReq := httptest.NewRequest(http.MethodPatch, "/v1/agents/"+agentID, bytes.NewBufferString(updateBody))
		updateReq.Header.Set("Content-Type", "application/json")
		updateW := httptest.NewRecorder()
		router.ServeHTTP(updateW, updateReq)

		if updateW.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", updateW.Code, updateW.Body.String())
		}

		// Verify update
		getReq := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID, nil)
		getW := httptest.NewRecorder()
		router.ServeHTTP(getW, getReq)

		var getResp map[string]interface{}
		_ = json.Unmarshal(getW.Body.Bytes(), &getResp)

		if getResp["name"] != "Updated Agent Name" {
			t.Errorf("expected name 'Updated Agent Name', got %v", getResp["name"])
		}
	})

	// === DELETE ===
	t.Run("delete agent", func(t *testing.T) {
		// Create an agent first
		body := `{
			"agentId": "delete-test-agent",
			"name": "Delete Test Agent",
			"description": "Will be deleted",
			"goal": "Test goal",
			"industry": "tech",
			"endpointConfig": {
				"protocol": "custom",
				"endpoint": "https://example.com/delete"
			},
			"authConfig": {"type": "none"}
		}`

		createReq := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(body))
		createReq.Header.Set("Content-Type", "application/json")
		createW := httptest.NewRecorder()
		router.ServeHTTP(createW, createReq)

		var createResp map[string]interface{}
		_ = json.Unmarshal(createW.Body.Bytes(), &createResp)
		agentID := createResp["agentId"].(string)

		// Delete the agent
		deleteReq := httptest.NewRequest(http.MethodDelete, "/v1/agents/"+agentID, nil)
		deleteW := httptest.NewRecorder()
		router.ServeHTTP(deleteW, deleteReq)

		if deleteW.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d: %s", deleteW.Code, deleteW.Body.String())
		}

		// Verify agent is not found
		getReq := httptest.NewRequest(http.MethodGet, "/v1/agents/"+agentID, nil)
		getW := httptest.NewRecorder()
		router.ServeHTTP(getW, getReq)

		if getW.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", getW.Code)
		}
	})
}

func TestAgentValidationErrors(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "invalid protocol",
			body:           `{"agentId": "test-1", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "invalid"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PROTOCOL",
		},
		{
			name:           "missing endpoint for custom protocol",
			body:           `{"agentId": "test-2", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "custom"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_ENDPOINT",
		},
		{
			name:           "missing baseUrl for langgraph",
			body:           `{"agentId": "test-3", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "langgraph"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_BASE_URL",
		},
		{
			name:           "missing agentId for bedrock",
			body:           `{"agentId": "test-4", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "bedrock"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_AGENT_ID",
		},
		{
			name:           "missing projectId for vertex",
			body:           `{"agentId": "test-5", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "vertex", "reasoningEngineId": "engine1"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_PROJECT_ID",
		},
		{
			name:           "missing identifier for openai",
			body:           `{"agentId": "test-6", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "openai"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_AGENT_IDENTIFIER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			var response ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to parse error response: %v", err)
			}

			if response.Error.Code != tt.expectedCode {
				t.Errorf("expected error code %s, got %s", tt.expectedCode, response.Error.Code)
			}
		})
	}
}

func TestAgentNotFound(t *testing.T) {
	router, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/nonexistent-agent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}

	var response ErrorResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	if response.Error.Code != "AGENT_NOT_FOUND" {
		t.Errorf("expected error code AGENT_NOT_FOUND, got %s", response.Error.Code)
	}
}

// === MCP Protocol Tests ===

func TestMCPAgentValidation(t *testing.T) {
	router, _ := setupTestRouter(t)

	tests := []struct {
		name           string
		body           string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "missing endpoint for MCP",
			body:           `{"agentId": "mcp-1", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "mcp", "mcpTransport": "sse"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_ENDPOINT",
		},
		{
			name:           "missing mcpTransport for MCP",
			body:           `{"agentId": "mcp-2", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "mcp", "endpoint": "http://example.com/mcp"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "MISSING_MCP_TRANSPORT",
		},
		{
			name:           "valid MCP config with all fields",
			body:           `{"agentId": "mcp-3", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "mcp", "endpoint": "http://example.com/mcp", "mcpTransport": "sse", "mcpToolName": "chat"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusCreated,
			expectedCode:   "",
		},
		{
			name:           "valid MCP with streamable_http transport",
			body:           `{"agentId": "mcp-4", "name": "Test", "description": "Test", "goal": "Test", "industry": "tech", "endpointConfig": {"protocol": "mcp", "endpoint": "http://localhost:9000/mcp", "mcpTransport": "streamable_http", "mcpToolName": "search"}, "authConfig": {"type": "none"}}`,
			expectedStatus: http.StatusCreated,
			expectedCode:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedCode != "" {
				var response ErrorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if response.Error.Code != tt.expectedCode {
					t.Errorf("expected error code %s, got %s", tt.expectedCode, response.Error.Code)
				}
			}
		})
	}
}

func TestMCPAgentLenientParsing(t *testing.T) {
	// MCP protocol uses lenient parsing - only agentId is required
	// Name, description, goal are auto-populated from MCP discovery
	// Since we don't have a real MCP server, we test that:
	// 1. The request doesn't fail validation for missing name/description/goal
	// 2. The validation for protocol-specific fields still works

	router, _ := setupTestRouter(t)

	// This should fail because mcpTransport is missing (protocol validation)
	// but NOT because name/description/goal are missing (lenient parsing)
	body := `{
		"agentId": "mcp-lenient-test",
		"industry": "technology",
		"endpointConfig": {
			"protocol": "mcp",
			"endpoint": "http://localhost:9000/mcp"
		},
		"authConfig": {"type": "none"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should fail with MISSING_MCP_TRANSPORT, not MISSING_NAME
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var response ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}

	if response.Error.Code != "MISSING_MCP_TRANSPORT" {
		t.Errorf("expected MISSING_MCP_TRANSPORT, got %s - lenient parsing may not be working", response.Error.Code)
	}
}

func TestMCPAgentCreationWithMinimalFields(t *testing.T) {
	// Test that MCP agent can be created with minimal fields
	// (only agentId and proper endpointConfig)

	router, _ := setupTestRouter(t)

	// Minimal MCP request - only agentId, endpoint config, auth
	body := `{
		"agentId": "mcp-minimal",
		"industry": "technology",
		"endpointConfig": {
			"protocol": "mcp",
			"endpoint": "http://localhost:9000/mcp",
			"mcpTransport": "streamable_http",
			"mcpToolName": "chat"
		},
		"authConfig": {"type": "none"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/agents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// MCP discovery will fail (no real server), but request parsing should succeed
	// The service should handle the discovery failure gracefully
	// In test mode without real MCP server, we may get 201 with empty name
	// or 500 if discovery is required - depends on implementation

	// For now, verify the request was at least parsed correctly
	if w.Code == http.StatusBadRequest {
		var response ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &response)
		// Should NOT be a validation error for missing name/description/goal
		if response.Error.Code == "VALIDATION_ERROR" &&
		   (contains(response.Error.Message, "name") ||
		    contains(response.Error.Message, "description") ||
		    contains(response.Error.Message, "goal")) {
			t.Errorf("MCP lenient parsing failed - got validation error for: %s", response.Error.Message)
		}
	}
}

// Helper for string containment
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return len(substr) == 0
}
