package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/r3labs/sse/v2"
	"go.uber.org/zap"
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

// mcpInitResponse is the parsed response from MCP initialize
type mcpInitResponse struct {
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

// Standard MCP requests - reused across transports
func newInitializeRequest() jsonRPCRequest {
	return jsonRPCRequest{
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
}

func newInitializedNotification() jsonRPCRequest {
	return jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
}

func newToolsListRequest() jsonRPCRequest {
	return jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
}

// setAuthHeaders copies auth headers to an HTTP request
func setAuthHeaders(req *http.Request, authHeaders map[string]string) {
	for k, v := range authHeaders {
		req.Header.Set(k, v)
	}
}

// DiscoverMCPTool discovers tool details from an MCP server using JSON-RPC 2.0.
// It performs the MCP initialization handshake and retrieves tool information.
// Supports both SSE and streamable-http transports.
func (s *AgentService) DiscoverMCPTool(
	ctx context.Context,
	endpoint string,
	toolName string,
	transport string,
	authHeaders map[string]string,
) (*MCPDiscoveryResult, error) {
	// Validate endpoint URL before making any requests
	if _, err := ValidateEndpointURL(endpoint); err != nil {
		return nil, err
	}

	// Normalize transport name
	transport = strings.ToLower(strings.ReplaceAll(transport, "-", "_"))

	if transport == "sse" {
		return s.discoverMCPToolSSE(ctx, endpoint, toolName, authHeaders)
	}
	return s.discoverMCPToolStreamableHTTP(ctx, endpoint, toolName, authHeaders)
}

// discoverMCPToolStreamableHTTP handles discovery for streamable-http transport.
// Uses direct POST requests with JSON-RPC payloads.
// The endpoint URL must be pre-validated by the caller.
func (s *AgentService) discoverMCPToolStreamableHTTP(
	ctx context.Context,
	endpoint string,
	toolName string,
	authHeaders map[string]string,
) (*MCPDiscoveryResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	requestBuilder := NewSafeHTTPRequestBuilder()

	// Step 1: Initialize connection
	initResult, err := s.sendMCPRequest(ctx, client, requestBuilder, endpoint, newInitializeRequest(), authHeaders, "")
	if err != nil {
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}

	// Capture session ID for subsequent requests
	sessionID := initResult.SessionID

	// Parse server info from init response
	var initData mcpInitResponse
	if err := json.Unmarshal(initResult.Response.Result, &initData); err != nil {
		return nil, fmt.Errorf("failed to parse init response: %w", err)
	}

	// Step 2: Send initialized notification (fire and forget)
	_, _ = s.sendMCPRequest(ctx, client, requestBuilder, endpoint, newInitializedNotification(), authHeaders, sessionID)

	// Step 3: List tools
	toolsResp, err := s.sendMCPRequest(ctx, client, requestBuilder, endpoint, newToolsListRequest(), authHeaders, sessionID)
	if err != nil {
		return nil, fmt.Errorf("MCP tools/list failed: %w", err)
	}

	return s.parseToolsResponse(toolsResp.Response.Result, toolName, initData.ServerInfo.Name, initData.ServerInfo.Version, initData.Capabilities)
}

// discoverMCPToolSSE handles discovery for SSE transport using native Go HTTP client.
// SSE transport uses GET to establish event stream, then POST to a message endpoint.
// Responses come through the SSE stream, not the POST response.
// The endpoint URL must be pre-validated by the caller.
func (s *AgentService) discoverMCPToolSSE(
	ctx context.Context,
	endpoint string,
	toolName string,
	authHeaders map[string]string,
) (*MCPDiscoveryResult, error) {
	requestBuilder := NewSafeHTTPRequestBuilder()

	// Create SSE client using r3labs/sse for spec-compliant event parsing
	sseClient := sse.NewClient(endpoint)
	sseClient.ReconnectStrategy = nil // Disable auto-reconnect for discovery

	// Set auth headers
	for key, value := range authHeaders {
		sseClient.Headers[key] = value
	}

	s.logger.Debug("Connecting to SSE endpoint", zap.String("endpoint", endpoint))

	// Channel to receive SSE events
	eventChan := make(chan *sse.Event, 10)

	// Create cancellable context for SSE subscription
	sseCtx, cancelSSE := context.WithCancel(ctx)
	defer cancelSSE()

	// Subscribe to SSE events in background
	errChan := make(chan error, 1)
	go func() {
		err := sseClient.SubscribeChanRawWithContext(sseCtx, eventChan)
		if err != nil && err != context.Canceled {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Wait for endpoint event - returns a validated endpoint string
	messageEndpoint, err := s.waitForSSEEndpoint(ctx, requestBuilder, endpoint, eventChan, errChan)
	if err != nil {
		return nil, err
	}

	// HTTP client for POST requests
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Helper to send JSON-RPC and wait for SSE response
	sendAndReceive := func(reqData jsonRPCRequest) (*jsonRPCResponse, error) {
		return s.sendSSERequest(ctx, httpClient, requestBuilder, messageEndpoint, reqData, authHeaders, eventChan, errChan)
	}

	// Step 1: Initialize
	initResp, err := sendAndReceive(newInitializeRequest())
	if err != nil {
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}

	var initData mcpInitResponse
	if err := json.Unmarshal(initResp.Result, &initData); err != nil {
		return nil, fmt.Errorf("failed to parse init response: %w", err)
	}

	// Step 2: Send initialized notification (fire and forget)
	s.sendSSENotification(ctx, httpClient, requestBuilder, messageEndpoint, newInitializedNotification(), authHeaders)

	// Step 3: List tools
	toolsResp, err := sendAndReceive(newToolsListRequest())
	if err != nil {
		return nil, fmt.Errorf("MCP tools/list failed: %w", err)
	}

	return s.parseToolsResponse(toolsResp.Result, toolName, initData.ServerInfo.Name, initData.ServerInfo.Version, initData.Capabilities)
}

// waitForSSEEndpoint waits for the endpoint event from the SSE stream.
// Returns a validated endpoint string for the message endpoint.
func (s *AgentService) waitForSSEEndpoint(ctx context.Context, requestBuilder *SafeHTTPRequestBuilder, baseEndpoint string, eventChan <-chan *sse.Event, errChan <-chan error) (string, error) {
	select {
	case event := <-eventChan:
		eventType := string(event.Event)
		eventData := string(event.Data)
		if eventType == "endpoint" && eventData != "" {
			resolved, err := requestBuilder.ResolveURL(baseEndpoint, eventData)
			if err != nil {
				return "", fmt.Errorf("failed to resolve SSE endpoint: %w", err)
			}
			s.logger.Info("SSE message endpoint discovered", zap.String("endpoint", resolved))
			return resolved, nil
		}
		return "", fmt.Errorf("unexpected first event: %s", eventType)
	case err := <-errChan:
		return "", fmt.Errorf("SSE read error: %w", err)
	case <-time.After(5 * time.Second):
		return "", fmt.Errorf("timeout waiting for endpoint event")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// sendSSERequest sends a JSON-RPC request and waits for response via SSE stream.
// Uses SafeHTTPRequestBuilder for secure request creation.
func (s *AgentService) sendSSERequest(
	ctx context.Context,
	client *http.Client,
	requestBuilder *SafeHTTPRequestBuilder,
	endpoint string,
	reqData jsonRPCRequest,
	authHeaders map[string]string,
	eventChan <-chan *sse.Event,
	errChan <-chan error,
) (*jsonRPCResponse, error) {
	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	httpReq, err := requestBuilder.NewPOSTRequest(ctx, endpoint, body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthHeaders(httpReq, authHeaders)

	postResp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("message endpoint returned status %d", postResp.StatusCode)
	}

	// Wait for matching response from SSE stream
	requestID := fmt.Sprintf("%v", reqData.ID)
	timeout := time.After(15 * time.Second)
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return nil, fmt.Errorf("SSE channel closed")
			}
			eventType := string(event.Event)
			eventData := string(event.Data)
			if eventType == "message" {
				var rpcResp jsonRPCResponse
				if err := json.Unmarshal([]byte(eventData), &rpcResp); err != nil {
					continue
				}
				if fmt.Sprintf("%v", rpcResp.ID) == requestID {
					if rpcResp.Error != nil {
						return nil, fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
					}
					return &rpcResp, nil
				}
			}
		case err := <-errChan:
			return nil, fmt.Errorf("SSE error: %w", err)
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for response")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// sendSSENotification sends a JSON-RPC notification (no response expected).
// Uses SafeHTTPRequestBuilder for secure request creation.
func (s *AgentService) sendSSENotification(
	ctx context.Context,
	client *http.Client,
	requestBuilder *SafeHTTPRequestBuilder,
	endpoint string,
	reqData jsonRPCRequest,
	authHeaders map[string]string,
) {
	body, _ := json.Marshal(reqData)
	httpReq, err := requestBuilder.NewPOSTRequest(ctx, endpoint, body)
	if err != nil {
		return // Silently fail for notification
	}
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthHeaders(httpReq, authHeaders)
	_, _ = client.Do(httpReq) // Fire and forget - ignore response
}

// parseToolsResponse extracts tool information from the tools/list response.
func (s *AgentService) parseToolsResponse(
	result json.RawMessage,
	toolName string,
	serverName string,
	serverVersion string,
	capabilities map[string]interface{},
) (*MCPDiscoveryResult, error) {
	var toolsData struct {
		Tools []struct {
			Name        string                 `json:"name"`
			Description string                 `json:"description"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &toolsData); err != nil {
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
			Name:         serverName,
			Version:      serverVersion,
			Capabilities: capabilities,
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
// Uses SafeHTTPRequestBuilder for secure request creation.
func (s *AgentService) sendMCPRequest(
	ctx context.Context,
	client *http.Client,
	requestBuilder *SafeHTTPRequestBuilder,
	endpoint string,
	req jsonRPCRequest,
	authHeaders map[string]string,
	sessionID string,
) (*mcpRequestResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := requestBuilder.NewPOSTRequest(ctx, endpoint, body)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		httpReq.Header.Set("mcp-session-id", sessionID)
	}
	setAuthHeaders(httpReq, authHeaders)

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
