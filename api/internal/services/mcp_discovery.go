package services

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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
	// Validate endpoint URL before making any requests - this sanitizes the URL
	validatedURL, err := ValidateEndpointURL(endpoint)
	if err != nil {
		return nil, err
	}

	// Normalize transport name
	transport = strings.ToLower(strings.ReplaceAll(transport, "-", "_"))

	if transport == "sse" {
		return s.discoverMCPToolSSE(ctx, validatedURL, toolName, authHeaders)
	}
	return s.discoverMCPToolStreamableHTTP(ctx, validatedURL, toolName, authHeaders)
}

// discoverMCPToolStreamableHTTP handles discovery for streamable-http transport.
// Uses direct POST requests with JSON-RPC payloads.
// The endpoint URL must be pre-validated by the caller.
func (s *AgentService) discoverMCPToolStreamableHTTP(
	ctx context.Context,
	validatedURL *url.URL,
	toolName string,
	authHeaders map[string]string,
) (*MCPDiscoveryResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Step 1: Initialize connection
	initResult, err := s.sendMCPRequest(ctx, client, validatedURL, newInitializeRequest(), authHeaders, "")
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
	_, _ = s.sendMCPRequest(ctx, client, validatedURL, newInitializedNotification(), authHeaders, sessionID)

	// Step 3: List tools
	toolsResp, err := s.sendMCPRequest(ctx, client, validatedURL, newToolsListRequest(), authHeaders, sessionID)
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
	validatedURL *url.URL,
	toolName string,
	authHeaders map[string]string,
) (*MCPDiscoveryResult, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: false,
			MaxIdleConns:      10,
			IdleConnTimeout:   90 * time.Second,
		},
	}

	// Establish SSE connection using pre-validated URL
	req, err := http.NewRequestWithContext(ctx, "GET", validatedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	setAuthHeaders(req, authHeaders)

	s.logger.Debug("Connecting to SSE endpoint", zap.String("endpoint", validatedURL.String()))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SSE connection failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("SSE endpoint returned status %d", resp.StatusCode)
	}

	// Channels for events
	eventChan := make(chan sseEvent, 10)
	errChan := make(chan error, 1)
	done := make(chan struct{})

	// Start reading SSE events in background
	go func() {
		defer close(eventChan)
		s.readSSEEvents(resp.Body, eventChan, errChan, done)
	}()
	defer func() {
		close(done)
		resp.Body.Close()
	}()

	// Wait for endpoint event - returns a validated URL
	messageEndpointURL, err := s.waitForSSEEndpoint(ctx, validatedURL, eventChan, errChan)
	if err != nil {
		return nil, err
	}

	// Helper to send JSON-RPC and wait for SSE response
	sendAndReceive := func(reqData jsonRPCRequest) (*jsonRPCResponse, error) {
		return s.sendSSERequest(ctx, client, messageEndpointURL, reqData, authHeaders, eventChan, errChan)
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
	s.sendSSENotification(ctx, client, messageEndpointURL, newInitializedNotification(), authHeaders)

	// Step 3: List tools
	toolsResp, err := sendAndReceive(newToolsListRequest())
	if err != nil {
		return nil, fmt.Errorf("MCP tools/list failed: %w", err)
	}

	return s.parseToolsResponse(toolsResp.Result, toolName, initData.ServerInfo.Name, initData.ServerInfo.Version, initData.Capabilities)
}

// waitForSSEEndpoint waits for the endpoint event from the SSE stream.
// Returns a validated URL for the message endpoint.
func (s *AgentService) waitForSSEEndpoint(ctx context.Context, baseURL *url.URL, eventChan <-chan sseEvent, errChan <-chan error) (*url.URL, error) {
	select {
	case event := <-eventChan:
		if event.EventType == "endpoint" && event.Data != "" {
			resolved, err := s.resolveAndValidateEndpointURL(baseURL, event.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve SSE endpoint: %w", err)
			}
			s.logger.Info("SSE message endpoint discovered", zap.String("endpoint", resolved.String()))
			return resolved, nil
		}
		return nil, fmt.Errorf("unexpected first event: %s", event.EventType)
	case err := <-errChan:
		return nil, fmt.Errorf("SSE read error: %w", err)
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for endpoint event")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// sendSSERequest sends a JSON-RPC request and waits for response via SSE stream.
// The endpoint URL must be pre-validated by the caller.
func (s *AgentService) sendSSERequest(
	ctx context.Context,
	client *http.Client,
	validatedURL *url.URL,
	reqData jsonRPCRequest,
	authHeaders map[string]string,
	eventChan <-chan sseEvent,
	errChan <-chan error,
) (*jsonRPCResponse, error) {
	body, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}

	// URL is pre-validated by caller
	httpReq, err := http.NewRequestWithContext(ctx, "POST", validatedURL.String(), bytes.NewReader(body))
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
			if event.EventType == "message" {
				var rpcResp jsonRPCResponse
				if err := json.Unmarshal([]byte(event.Data), &rpcResp); err != nil {
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
// The endpoint URL must be pre-validated by the caller.
func (s *AgentService) sendSSENotification(
	ctx context.Context,
	client *http.Client,
	validatedURL *url.URL,
	reqData jsonRPCRequest,
	authHeaders map[string]string,
) {
	body, _ := json.Marshal(reqData)
	// URL is pre-validated by caller
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", validatedURL.String(), bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	setAuthHeaders(httpReq, authHeaders)
	_, _ = client.Do(httpReq) // Fire and forget - ignore response
}

// sseEvent represents a parsed SSE event
type sseEvent struct {
	EventType string
	Data      string
}

// readSSEEvents reads events from an SSE stream
func (s *AgentService) readSSEEvents(body io.ReadCloser, eventChan chan<- sseEvent, errChan chan<- error, done <-chan struct{}) {
	reader := bufio.NewReader(body)
	var eventType, eventData string

	for {
		select {
		case <-done:
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				select {
				case errChan <- err:
				default:
				}
			}
			return
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			eventData = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		} else if line == "" && eventType != "" {
			// Complete event
			select {
			case eventChan <- sseEvent{EventType: eventType, Data: eventData}:
			case <-done:
				return
			}
			eventType = ""
			eventData = ""
		}
	}
}

// resolveAndValidateEndpointURL resolves a potentially relative endpoint path against the base URL,
// then validates and sanitizes the result. Returns a validated *url.URL.
func (s *AgentService) resolveAndValidateEndpointURL(baseURL *url.URL, endpointPath string) (*url.URL, error) {
	// If it's already absolute, validate it directly
	if strings.HasPrefix(endpointPath, "http://") || strings.HasPrefix(endpointPath, "https://") {
		return ValidateEndpointURL(endpointPath)
	}

	// Parse endpoint path (may include query params like ?sessionId=xxx)
	ref, err := url.Parse(endpointPath)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint path: %w", err)
	}

	// Resolve against base
	resolved := baseURL.ResolveReference(ref)

	// Validate the resolved URL
	return ValidateEndpointURL(resolved.String())
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
// The endpoint URL must be pre-validated by the caller.
func (s *AgentService) sendMCPRequest(
	ctx context.Context,
	client *http.Client,
	validatedURL *url.URL,
	req jsonRPCRequest,
	authHeaders map[string]string,
	sessionID string,
) (*mcpRequestResult, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// URL is pre-validated by caller
	httpReq, err := http.NewRequestWithContext(ctx, "POST", validatedURL.String(), bytes.NewReader(body))
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
