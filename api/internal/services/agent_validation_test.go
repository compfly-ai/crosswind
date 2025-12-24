package services

import (
	"testing"

	"github.com/compfly-ai/crosswind/api/internal/models"
)

func TestIsValidProtocol(t *testing.T) {
	validProtocols := []string{
		models.ProtocolOpenAI,
		models.ProtocolAzureOpenAI,
		models.ProtocolLangGraph,
		models.ProtocolBedrock,
		models.ProtocolVertex,
		models.ProtocolCustom,
		models.ProtocolCustomWS,
		models.ProtocolA2A,
		models.ProtocolMCP,
	}

	for _, p := range validProtocols {
		t.Run("valid_"+p, func(t *testing.T) {
			if !isValidProtocol(p) {
				t.Errorf("expected %q to be valid", p)
			}
		})
	}

	invalidProtocols := []string{
		"",
		"invalid",
		"http",
		"grpc",
		"OpenAI", // case sensitive
	}

	for _, p := range invalidProtocols {
		t.Run("invalid_"+p, func(t *testing.T) {
			if isValidProtocol(p) {
				t.Errorf("expected %q to be invalid", p)
			}
		})
	}
}

func TestValidateProtocolRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		config  models.EndpointConfig
		wantErr error
	}{
		// OpenAI protocol
		{
			name: "openai with promptId",
			config: models.EndpointConfig{
				Protocol: models.ProtocolOpenAI,
				PromptID: "prompt_123",
			},
			wantErr: nil,
		},
		{
			name: "openai with assistantId",
			config: models.EndpointConfig{
				Protocol:    models.ProtocolOpenAI,
				AssistantID: "asst_123",
			},
			wantErr: nil,
		},
		{
			name: "openai with workflowId",
			config: models.EndpointConfig{
				Protocol:   models.ProtocolOpenAI,
				WorkflowID: "wf_123",
			},
			wantErr: nil,
		},
		{
			name: "openai missing identifier",
			config: models.EndpointConfig{
				Protocol: models.ProtocolOpenAI,
			},
			wantErr: ErrMissingAgentIdentifier,
		},

		// Azure OpenAI protocol
		{
			name: "azure openai valid",
			config: models.EndpointConfig{
				Protocol: models.ProtocolAzureOpenAI,
				BaseURL:  "https://myresource.openai.azure.com",
				PromptID: "prompt_123",
			},
			wantErr: nil,
		},
		{
			name: "azure openai missing baseUrl",
			config: models.EndpointConfig{
				Protocol: models.ProtocolAzureOpenAI,
				PromptID: "prompt_123",
			},
			wantErr: ErrMissingBaseURL,
		},
		{
			name: "azure openai missing identifier",
			config: models.EndpointConfig{
				Protocol: models.ProtocolAzureOpenAI,
				BaseURL:  "https://myresource.openai.azure.com",
			},
			wantErr: ErrMissingAgentIdentifier,
		},

		// LangGraph protocol
		{
			name: "langgraph valid",
			config: models.EndpointConfig{
				Protocol: models.ProtocolLangGraph,
				BaseURL:  "https://my-deployment.langchain.app",
			},
			wantErr: nil,
		},
		{
			name: "langgraph missing baseUrl",
			config: models.EndpointConfig{
				Protocol: models.ProtocolLangGraph,
			},
			wantErr: ErrMissingBaseURL,
		},

		// Bedrock protocol
		{
			name: "bedrock valid",
			config: models.EndpointConfig{
				Protocol: models.ProtocolBedrock,
				AgentID:  "agent_123",
			},
			wantErr: nil,
		},
		{
			name: "bedrock missing agentId",
			config: models.EndpointConfig{
				Protocol: models.ProtocolBedrock,
			},
			wantErr: ErrMissingAgentID,
		},

		// Vertex protocol
		{
			name: "vertex valid",
			config: models.EndpointConfig{
				Protocol:          models.ProtocolVertex,
				ProjectID:         "my-project",
				ReasoningEngineID: "engine_123",
			},
			wantErr: nil,
		},
		{
			name: "vertex missing projectId",
			config: models.EndpointConfig{
				Protocol:          models.ProtocolVertex,
				ReasoningEngineID: "engine_123",
			},
			wantErr: ErrMissingProjectID,
		},
		{
			name: "vertex missing reasoningEngineId",
			config: models.EndpointConfig{
				Protocol:  models.ProtocolVertex,
				ProjectID: "my-project",
			},
			wantErr: ErrMissingReasoningEngineID,
		},

		// Custom HTTP protocol
		{
			name: "custom valid",
			config: models.EndpointConfig{
				Protocol: models.ProtocolCustom,
				Endpoint: "https://my-agent.example.com/chat",
			},
			wantErr: nil,
		},
		{
			name: "custom missing endpoint",
			config: models.EndpointConfig{
				Protocol: models.ProtocolCustom,
			},
			wantErr: ErrMissingEndpoint,
		},

		// Custom WebSocket protocol
		{
			name: "custom_ws valid",
			config: models.EndpointConfig{
				Protocol: models.ProtocolCustomWS,
				Endpoint: "wss://my-agent.example.com/ws",
			},
			wantErr: nil,
		},
		{
			name: "custom_ws missing endpoint",
			config: models.EndpointConfig{
				Protocol: models.ProtocolCustomWS,
			},
			wantErr: ErrMissingEndpoint,
		},

		// A2A protocol
		{
			name: "a2a valid",
			config: models.EndpointConfig{
				Protocol:     models.ProtocolA2A,
				AgentCardURL: "https://my-agent.example.com/.well-known/agent.json",
			},
			wantErr: nil,
		},
		{
			name: "a2a missing agentCardUrl",
			config: models.EndpointConfig{
				Protocol: models.ProtocolA2A,
			},
			wantErr: ErrMissingAgentCardURL,
		},

		// MCP protocol
		{
			name: "mcp valid",
			config: models.EndpointConfig{
				Protocol:     models.ProtocolMCP,
				Endpoint:     "https://my-mcp-server.example.com",
				MCPTransport: "sse",
			},
			wantErr: nil,
		},
		{
			name: "mcp missing endpoint",
			config: models.EndpointConfig{
				Protocol:     models.ProtocolMCP,
				MCPTransport: "sse",
			},
			wantErr: ErrMissingEndpoint,
		},
		{
			name: "mcp missing mcpTransport",
			config: models.EndpointConfig{
				Protocol: models.ProtocolMCP,
				Endpoint: "https://my-mcp-server.example.com",
			},
			wantErr: ErrMissingMCPTransport,
		},

		// Unknown protocol (should not error on required fields)
		{
			name: "unknown protocol passes",
			config: models.EndpointConfig{
				Protocol: "unknown",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProtocolRequiredFields(tt.config)
			if err != tt.wantErr {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProtocolRequiredFields_EdgeCases(t *testing.T) {
	// Test that empty strings are treated as missing
	tests := []struct {
		name    string
		config  models.EndpointConfig
		wantErr error
	}{
		{
			name: "openai with empty promptId",
			config: models.EndpointConfig{
				Protocol: models.ProtocolOpenAI,
				PromptID: "",
			},
			wantErr: ErrMissingAgentIdentifier,
		},
		{
			name: "custom with whitespace-only endpoint still passes",
			config: models.EndpointConfig{
				Protocol: models.ProtocolCustom,
				Endpoint: "   ", // whitespace - current impl doesn't trim
			},
			wantErr: nil, // Note: current impl doesn't trim whitespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProtocolRequiredFields(tt.config)
			if err != tt.wantErr {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}
