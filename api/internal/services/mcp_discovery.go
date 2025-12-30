package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MCPToolInfo represents discovered tool information
type MCPToolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPServerInfo represents discovered server information
type MCPServerInfo struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
}

// MCPDiscoveryResult contains the discovery response
type MCPDiscoveryResult struct {
	Tool           MCPToolInfo   `json:"tool"`
	Server         MCPServerInfo `json:"server"`
	AvailableTools []string      `json:"availableTools"`
}

// jsonRPCRequest represents a JSON-RPC 2.0 request
type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// jsonRPCResponse represents a JSON-RPC 2.0 response
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// DiscoverMCPTool discovers tool details from an MCP server using JSON-RPC 2.0.
// It performs the MCP initialization handshake and retrieves tool information.
func (s *AgentService) DiscoverMCPTool(
	ctx context.Context,
	endpoint string,
	toolName string,
	transport string,
	authHeaders map[string]string,
) (*MCPDiscoveryResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Initialize connection
	initReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "crosswind",
				"version": "1.0.0",
			},
		},
	}

	initResult, err := s.sendMCPRequest(ctx, client, endpoint, initReq, authHeaders, "")
	if err != nil {
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}

	// Capture session ID for subsequent requests
	sessionID := initResult.SessionID

	// Parse server info from init response
	var initData struct {
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
		Capabilities map[string]interface{} `json:"capabilities"`
	}
	if err := json.Unmarshal(initResult.Response.Result, &initData); err != nil {
		return nil, fmt.Errorf("failed to parse init response: %w", err)
	}

	// Step 2: Send initialized notification (no response expected)
	notifyReq := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	// Fire and forget - notifications don't expect responses
	_, _ = s.sendMCPRequest(ctx, client, endpoint, notifyReq, authHeaders, sessionID)

	// Step 3: List tools
	toolsReq := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	toolsResp, err := s.sendMCPRequest(ctx, client, endpoint, toolsReq, authHeaders, sessionID)
	if err != nil {
		return nil, fmt.Errorf("MCP tools/list failed: %w", err)
	}

	// Parse tools list
	var toolsData struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(toolsResp.Response.Result, &toolsData); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	// Find the requested tool
	var foundTool *MCPToolInfo
	availableTools := make([]string, 0, len(toolsData.Tools))
	for _, t := range toolsData.Tools {
		availableTools = append(availableTools, t.Name)
		if t.Name == toolName {
			foundTool = &MCPToolInfo{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.InputSchema,
			}
		}
	}

	if foundTool == nil {
		return nil, fmt.Errorf("tool '%s' not found. Available tools: %v", toolName, availableTools)
	}

	return &MCPDiscoveryResult{
		Tool: *foundTool,
		Server: MCPServerInfo{
			Name:         initData.ServerInfo.Name,
			Version:      initData.ServerInfo.Version,
			Capabilities: initData.Capabilities,
		},
		AvailableTools: availableTools,
	}, nil
}

// mcpRequestResult contains the response and session ID
type mcpRequestResult struct {
	Response  *jsonRPCResponse
	SessionID string
}

// sendMCPRequest sends a JSON-RPC request to the MCP server.
// Handles SSE response format used by streamable-http transport.
// Returns the response and any session ID from the response headers.
func (s *AgentService) sendMCPRequest(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	req jsonRPCRequest,
	authHeaders map[string]string,
	sessionID string,
) (*mcpRequestResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpReq.Header.Set("mcp-session-id", sessionID)
	}
	for k, v := range authHeaders {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP server returned status %d", resp.StatusCode)
	}

	// Capture session ID from response headers
	respSessionID := resp.Header.Get("mcp-session-id")

	// Read the full response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse SSE format if present (event: message\ndata: {...})
	var jsonData []byte
	respStr := string(respBody)
	if strings.HasPrefix(respStr, "event:") {
		// Parse SSE - look for "data: " line
		lines := strings.Split(respStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				jsonData = []byte(strings.TrimPrefix(line, "data: "))
				break
			}
		}
		if jsonData == nil {
			return nil, fmt.Errorf("SSE response missing data field")
		}
	} else {
		// Plain JSON response
		jsonData = respBody
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(jsonData, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return &mcpRequestResult{
		Response:  &rpcResp,
		SessionID: respSessionID,
	}, nil
}

// FindMessageField identifies the primary text input field from an MCP tool's input schema.
// It searches for common field names used for message/prompt input.
func FindMessageField(schema map[string]interface{}) string {
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return "message" // default
	}

	// Priority order for common field names
	candidates := []string{"message", "text", "prompt", "query", "input", "content"}
	for _, name := range candidates {
		if _, exists := props[name]; exists {
			return name
		}
	}

	// Fallback to first string property
	for name, prop := range props {
		if p, ok := prop.(map[string]interface{}); ok {
			if p["type"] == "string" {
				return name
			}
		}
	}

	return "message"
}
