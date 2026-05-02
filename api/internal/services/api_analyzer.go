package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// MinConfidenceThreshold is the minimum confidence required to activate an agent
const MinConfidenceThreshold = 0.7

// APIAnalyzer uses GPT-5.2 to analyze and infer agent API structure
type APIAnalyzer struct {
	client     openai.Client
	model      string
	httpClient *http.Client
}

// NewAPIAnalyzer creates a new API analyzer
func NewAPIAnalyzer(apiKey string) *APIAnalyzer {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &APIAnalyzer{
		client: client,
		model:  "gpt-5.2",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AnalyzeResult contains the analysis output
type AnalyzeResult struct {
	Schema     *models.InferredAPISchema `json:"schema"`
	ProbeLog   []ProbeAttempt            `json:"probeLog,omitempty"`
	Successful bool                      `json:"successful"`
	Error      string                    `json:"error,omitempty"`
	// FailureReason provides specific context when Successful is false
	// Possible values: "unreachable", "auth_failed", "low_confidence", "analysis_failed"
	FailureReason string `json:"failureReason,omitempty"`
}

// ProbeAttempt records a single probe attempt
type ProbeAttempt struct {
	Request    string `json:"request"`
	Response   string `json:"response"`
	StatusCode int    `json:"statusCode"`
	Error      string `json:"error,omitempty"`
}

// AnalyzeAgent performs comprehensive API analysis
func (a *APIAnalyzer) AnalyzeAgent(ctx context.Context, agent *models.Agent) (*AnalyzeResult, error) {
	result := &AnalyzeResult{
		ProbeLog: []ProbeAttempt{},
	}

	// Strategy 1: Parse OpenAPI spec if available (via specUrl)
	if agent.EndpointConfig.SpecURL != "" {
		schema, err := a.analyzeFromSpec(ctx, agent)
		if err == nil && schema != nil {
			// Spec-based analysis is high confidence, check threshold
			if schema.Confidence >= MinConfidenceThreshold {
				result.Schema = schema
				result.Successful = true
				return result, nil
			}
		}
		// Fall through to probing if spec parsing fails or low confidence
	}

	// Strategy 2: Probe the endpoint
	probeResult, probeLog := a.probeEndpoint(ctx, agent)
	result.ProbeLog = probeLog

	// Check probe results for connectivity and auth issues
	probeStatus := analyzeProbeResults(probeLog)

	// If endpoint is unreachable (all probes failed with connection errors), fail fast
	if probeStatus.allUnreachable {
		result.Successful = false
		result.FailureReason = "unreachable"
		result.Error = "Unable to reach endpoint. Check that the URL is correct and the service is running."
		return result, nil
	}

	// If all probes returned 401/403, authentication is failing
	if probeStatus.allAuthFailed {
		result.Successful = false
		result.FailureReason = "auth_failed"
		result.Error = "Authentication failed (401/403). Check your credentials."
		return result, nil
	}

	// Require at least one successful probe (2xx) to proceed with GPT analysis
	if !probeStatus.hasSuccessfulProbe {
		result.Successful = false
		result.FailureReason = "unreachable"
		result.Error = fmt.Sprintf("No successful response from endpoint. Last status: %d. Check endpoint URL and credentials.", probeStatus.lastStatusCode)
		return result, nil
	}

	// Strategy 3: Use GPT-5.2 to analyze successful probe results
	schema, err := a.analyzeWithGPT(ctx, agent, probeResult, probeLog)
	if err != nil {
		result.Error = err.Error()
		result.FailureReason = "analysis_failed"
		result.Successful = false
		return result, nil
	}

	result.Schema = schema

	// Check confidence threshold
	if schema.Confidence < MinConfidenceThreshold {
		result.Successful = false
		result.FailureReason = "low_confidence"
		result.Error = fmt.Sprintf("Analysis confidence (%.0f%%) is below threshold (%.0f%%). Consider providing an OpenAPI spec or checking the endpoint configuration.", schema.Confidence*100, MinConfidenceThreshold*100)
		return result, nil
	}

	result.Successful = true
	return result, nil
}

// probeAnalysis contains aggregated probe result analysis
type probeAnalysis struct {
	hasSuccessfulProbe bool // At least one 2xx response
	allUnreachable     bool // All probes failed with connection errors
	allAuthFailed      bool // All probes returned 401 or 403
	lastStatusCode     int  // Most recent status code for error messages
}

// analyzeProbeResults examines probe attempts to determine connectivity and auth status
func analyzeProbeResults(probeLog []ProbeAttempt) probeAnalysis {
	if len(probeLog) == 0 {
		return probeAnalysis{allUnreachable: true}
	}

	var (
		connectionErrors int
		authFailures     int
		successCount     int
		lastStatusCode   int
	)

	for _, attempt := range probeLog {
		if attempt.StatusCode > 0 {
			lastStatusCode = attempt.StatusCode
		}

		// Check for connection errors (no status code means connection failed)
		if attempt.Error != "" && attempt.StatusCode == 0 {
			connectionErrors++
			continue
		}

		// Check for auth failures
		if attempt.StatusCode == 401 || attempt.StatusCode == 403 {
			authFailures++
			continue
		}

		// Check for success
		if attempt.StatusCode >= 200 && attempt.StatusCode < 300 {
			successCount++
		}
	}

	return probeAnalysis{
		hasSuccessfulProbe: successCount > 0,
		allUnreachable:     connectionErrors == len(probeLog),
		allAuthFailed:      authFailures == len(probeLog) && connectionErrors == 0,
		lastStatusCode:     lastStatusCode,
	}
}

// analyzeFromSpec parses OpenAPI/Swagger spec from specUrl
func (a *APIAnalyzer) analyzeFromSpec(ctx context.Context, agent *models.Agent) (*models.InferredAPISchema, error) {
	// Fetch spec from URL
	if agent.EndpointConfig.SpecURL == "" {
		return nil, fmt.Errorf("no spec URL available")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", agent.EndpointConfig.SpecURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, err
	}

	// Use GPT to parse the spec since OpenAPI can be complex
	specJSON, _ := json.MarshalIndent(spec, "", "  ")

	// Derive conversation endpoint path from the full endpoint URL
	var conversationEndpoint string
	if agent.EndpointConfig.Endpoint != "" {
		if parsed, err := url.Parse(agent.EndpointConfig.Endpoint); err == nil {
			conversationEndpoint = parsed.Path
		}
	}

	return a.parseSpecWithGPT(ctx, string(specJSON), conversationEndpoint)
}

// parseSpecWithGPT uses GPT-5.2 to extract API schema from OpenAPI spec
func (a *APIAnalyzer) parseSpecWithGPT(ctx context.Context, specJSON, conversationEndpoint string) (*models.InferredAPISchema, error) {
	systemPrompt := `You are an expert API analyst. Extract the request/response schema from an OpenAPI spec.

## API STYLES - Identify which pattern:

Core styles:
- chat_stateless: "messages" array with [{role, content}...] (OpenAI/Claude)
- single_message: single string field like "message", "prompt", "query"

Framework styles:
- langserve: {input: {...}} with /invoke endpoint (LangChain)
- flowise: {question: "..."} with /prediction/ endpoint
- dify: {inputs: {...}} or {query: "..."} (Dify workflow/chat)
- haystack: {query: "...", params: {...}} (Haystack pipeline)
- botpress: {conversationId, payload: {text: "..."}} (Botpress)
- thread_based: /threads/{id}/messages path (OpenAI Assistants)
- task_based: POST creates run → GET SSE stream (response has runId/streamUrl, no content)

## OUTPUT FORMAT

For most styles:
{
  "apiStyle": "chat_stateless",
  "requestMethod": "POST",
  "requestContentType": "application/json",
  "messageField": "messages",
  "responseContentField": "choices[0].message.content",
  "sessionIdField": "",
  "sessionIdInResponse": "session_id",
  "sessionCreateMethod": "auto",
  "streamingSupported": false,
  "confidence": 0.95
}

For task_based (include additional fields):
{
  "apiStyle": "task_based",
  "messageField": "message",
  "sessionIdField": "sessionId",
  "sessionIdInResponse": "sessionId",
  "sessionCreateMethod": "auto",
  "streamingSupported": true,
  "runIdField": "runId",
  "streamEndpoint": "/v1/runs/{runId}/stream",
  "streamMethod": "GET",
  "sseContentType": "text.delta",
  "sseContentField": "text",
  "sseDoneType": "run.completed",
  "confidence": 0.85
}

## FIELD RULES

| Style | messageField | responseContentField |
|-------|-------------|---------------------|
| chat_stateless | "messages" | "choices[0].message.content" |
| single_message | "message"/"prompt" | "response" |
| langserve | "input" | "output" |
| flowise | "question" | "text" |
| dify | "inputs"/"query" | "answer" |
| task_based | "message" | (N/A — content from SSE stream) |

Use dot notation: "choices[0].message.content"

Return ONLY valid JSON, no markdown.`

	userPrompt := fmt.Sprintf(`Analyze this OpenAPI spec and extract the schema for the conversation endpoint: %s

Spec:
%s`, conversationEndpoint, truncateSpec(specJSON, 12000))

	maxTokens := int64(2000)

	resp, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:                a.model,
		MaxCompletionTokens:  openai.Int(maxTokens),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from GPT")
	}

	content := resp.Choices[0].Message.Content
	content = cleanJSONResponse(content)

	var schema models.InferredAPISchema
	if err := json.Unmarshal([]byte(content), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse GPT response: %w", err)
	}

	schema.InferredAt = time.Now()
	schema.InferenceMethod = "openapi_spec"

	return &schema, nil
}

// probeEndpoint sends test requests to understand the API
func (a *APIAnalyzer) probeEndpoint(ctx context.Context, agent *models.Agent) (map[string]interface{}, []ProbeAttempt) {
	var probeLog []ProbeAttempt
	var successfulResponse map[string]interface{}

	// Use the full endpoint URL directly
	probeURL := agent.EndpointConfig.Endpoint
	if probeURL == "" {
		return nil, nil // Can't probe without endpoint
	}

	// Common request formats to try - ordered by likelihood
	probeFormats := []map[string]interface{}{
		// OpenAI/Claude chat style (most common)
		{"messages": []map[string]string{{"role": "user", "content": "Hello"}}},

		// Simple message styles
		{"message": "Hello"},
		{"prompt": "Hello"},
		{"query": "Hello"},

		// Async run pattern (task_based) — POST returns runId/streamUrl instead of content
		{"message": "Hello", "sessionId": "probe-session"},

		// LangServe style
		{"input": map[string]string{"message": "Hello"}},
		{"input": "Hello"},

		// Flowise style
		{"question": "Hello"},
		{"question": "Hello", "history": []interface{}{}},

		// Dify workflow style
		{"inputs": map[string]string{"query": "Hello"}, "response_mode": "blocking", "user": "test-user"},
		// Dify chat style
		{"query": "Hello", "user": "test-user"},

		// Haystack style
		{"query": "Hello", "params": map[string]interface{}{}},

		// Botpress style
		{"type": "text", "payload": map[string]string{"text": "Hello"}},

		// Generic fallbacks
		{"text": "Hello"},
		{"content": "Hello"},
	}

	// A2A JSON-RPC message/send format
	a2aProbe := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "probe-1",
		"method":  "message/send",
		"params": map[string]interface{}{
			"message": map[string]interface{}{
				"role":      "user",
				"parts":     []map[string]string{{"kind": "text", "text": "Hello"}},
				"messageId": "probe-msg-1",
			},
			"configuration": map[string]interface{}{
				"contextId": "probe-session",
			},
		},
	}

	// A2A agents: try JSON-RPC first since the protocol is already known
	if agent.EndpointConfig.Protocol == "a2a" {
		attempt := a.tryProbe(ctx, probeURL, a2aProbe, agent.AuthConfig)
		probeLog = append(probeLog, attempt)

		if attempt.StatusCode >= 200 && attempt.StatusCode < 300 && attempt.Error == "" {
			var respData map[string]interface{}
			if err := json.Unmarshal([]byte(attempt.Response), &respData); err == nil {
				return respData, probeLog
			}
		}
	}

	for _, payload := range probeFormats {
		attempt := a.tryProbe(ctx, probeURL, payload, agent.AuthConfig)
		probeLog = append(probeLog, attempt)

		if attempt.StatusCode >= 200 && attempt.StatusCode < 300 && attempt.Error == "" {
			var respData map[string]interface{}
			if err := json.Unmarshal([]byte(attempt.Response), &respData); err == nil {
				// Detected JSON-RPC endpoint — retry with A2A envelope
				if _, hasJsonrpc := respData["jsonrpc"]; hasJsonrpc {
					if _, hasErr := respData["error"]; hasErr {
						a2aAttempt := a.tryProbe(ctx, probeURL, a2aProbe, agent.AuthConfig)
						probeLog = append(probeLog, a2aAttempt)
						if a2aAttempt.StatusCode >= 200 && a2aAttempt.StatusCode < 300 && a2aAttempt.Error == "" {
							var a2aResp map[string]interface{}
							if err := json.Unmarshal([]byte(a2aAttempt.Response), &a2aResp); err == nil {
								successfulResponse = a2aResp
								break
							}
						}
					}
				}
				successfulResponse = respData
				break
			}
		}
	}

	return successfulResponse, probeLog
}

// tryProbe sends a single probe request
func (a *APIAnalyzer) tryProbe(ctx context.Context, url string, payload map[string]interface{}, auth models.AuthConfig) ProbeAttempt {
	attempt := ProbeAttempt{}

	body, _ := json.Marshal(payload)
	attempt.Request = string(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		attempt.Error = err.Error()
		return attempt
	}

	req.Header.Set("Content-Type", "application/json")
	a.applyAuth(req, auth)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		attempt.Error = err.Error()
		return attempt
	}
	defer resp.Body.Close()

	attempt.StatusCode = resp.StatusCode

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		attempt.Error = err.Error()
		return attempt
	}

	attempt.Response = string(respBody)
	return attempt
}

// applyAuth applies authentication to the request
func (a *APIAnalyzer) applyAuth(req *http.Request, auth models.AuthConfig) {
	switch auth.Type {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+auth.Credentials)
	case "api_key":
		headerName := auth.HeaderName
		if headerName == "" {
			headerName = "X-API-Key"
		}
		prefix := auth.HeaderPrefix
		if prefix != "" {
			req.Header.Set(headerName, prefix+" "+auth.Credentials)
		} else {
			req.Header.Set(headerName, auth.Credentials)
		}
	case "basic":
		req.SetBasicAuth(strings.Split(auth.Credentials, ":")[0], strings.Split(auth.Credentials, ":")[1])
	}
}

// analyzeWithGPT uses GPT-5.2 for comprehensive analysis
func (a *APIAnalyzer) analyzeWithGPT(ctx context.Context, agent *models.Agent, probeResult map[string]interface{}, probeLog []ProbeAttempt) (*models.InferredAPISchema, error) {
	systemPrompt := `You are an expert AI agent API analyst. Analyze probe results to determine the correct API schema.

## API STYLES - Identify which pattern the API uses:

### CORE STYLES

#### 1. chat_stateless (OpenAI/Claude/Groq style) - MOST COMMON
- Client sends FULL conversation history with every request
- Messages field contains array of {role, content} objects
Detection: Request has "messages" array with role/content objects
Example request:
  {"messages": [{"role": "user", "content": "Hello"}]}
Example response:
  {"choices": [{"message": {"content": "Hi!"}}]} OR {"response": "Hi!", "session_id": "..."}

#### 2. single_message (Simple agent APIs)
- Client sends just the current message as a string
- Server may track context via session_id
Detection: Request has single string field like "message", "prompt", "query"
Example request:
  {"message": "Hello", "session_id": "abc123"}
Example response:
  {"response": "Hi!", "session_id": "abc123"}

### FRAMEWORK-SPECIFIC STYLES

#### 3. langserve (LangChain LangServe)
- Wraps LangChain runnables with /invoke, /stream, /batch endpoints
- Input wrapped in "input" object, optional "config" for session
Detection: Endpoint contains "/invoke" or "/stream", request has "input" field
Example request:
  {"input": {"topic": "cats"}, "config": {"configurable": {"session_id": "123"}}}
Example response:
  {"output": "Cats are...", "metadata": {...}}

#### 4. flowise (Flowise Prediction API)
- RAG and chatflow platform
- Uses "question" field with optional history array
Detection: Endpoint contains "/prediction/", request has "question" field
Example request:
  {"question": "Hello", "history": [], "sessionId": "abc", "streaming": false}
Example response:
  {"text": "Hi there!", "sessionId": "abc"}

#### 5. dify (Dify Workflow/Chat)
- Low-code AI workflow platform
- Uses "inputs" object for workflow, "query" for chat
Detection: Endpoint contains "/workflows/run" or "/chat-messages"
Example request (workflow):
  {"inputs": {"query": "Hello"}, "user": "user-123", "response_mode": "blocking"}
Example request (chat):
  {"query": "Hello", "user": "user-123", "conversation_id": "conv-abc"}
Example response:
  {"answer": "Hi!", "conversation_id": "conv-abc"}

#### 6. haystack (Haystack Hayhooks)
- Pipeline-based, dynamic inputs based on pipeline structure
Detection: Often has "query" or pipeline-specific input fields
Example request:
  {"query": "What is AI?", "params": {...}}
Example response:
  {"answers": [...], "documents": [...]}

#### 7. botpress (Botpress Webhook)
- Conversational AI platform with webhook-based API
Detection: Request has "conversationId", "userId", "payload" structure
Example request:
  {"conversationId": "conv-123", "userId": "user-456", "type": "text", "payload": {"text": "Hello"}}
Example response:
  {"responses": [{"type": "text", "payload": {"text": "Hi!"}}]}

### ASYNC STYLES

#### 8. thread_based (OpenAI Assistants style)
- Messages added to server-managed thread via separate endpoint
Detection: URL contains /threads/ path

#### 9. task_based (Async Run / SSE stream pattern)
- POST to create a run/task → response contains runId + streamUrl (NO direct content)
- GET the stream URL → SSE event stream delivers response as typed events
- Used by: OpenAI Assistants, LangGraph Platform, custom agents with long-running operations
Detection: POST response contains "runId"/"taskId"/"id" AND "streamUrl"/"stream_url"/"streamPath" but NO assistant content
Example request:
  {"message": "Hello", "sessionId": "abc123"}
Example POST response (201):
  {"runId": "run_abc123", "sessionId": "abc123", "streamUrl": "/v1/runs/run_abc123/stream"}
Example SSE stream:
  data: {"type": "text.delta", "text": "Hello"}
  data: {"type": "text.delta", "text": " there!"}
  data: {"type": "run.completed", "summary": "Done", "durationMs": 1200}
IMPORTANT: If the POST response has a run/task ID and a stream URL but no actual text content, this is task_based. Do NOT classify it as single_message.

## OUTPUT FORMAT

For most styles:
{
  "apiStyle": "chat_stateless",
  "requestMethod": "POST",
  "requestContentType": "application/json",
  "messageField": "messages",
  "responseContentField": "choices[0].message.content",
  "sessionIdField": "",
  "sessionIdInResponse": "session_id",
  "sessionCreateMethod": "auto",
  "streamingSupported": false,
  "confidence": 0.9,
  "reasoning": "Brief explanation of detection"
}

For task_based style (additional fields required):
{
  "apiStyle": "task_based",
  "requestMethod": "POST",
  "requestContentType": "application/json",
  "messageField": "message",
  "sessionIdField": "sessionId",
  "sessionIdInResponse": "sessionId",
  "sessionCreateMethod": "auto",
  "streamingSupported": true,
  "runIdField": "runId",
  "streamEndpoint": "/v1/runs/{runId}/stream",
  "streamMethod": "GET",
  "sseContentType": "text.delta",
  "sseContentField": "text",
  "sseDoneType": "run.completed",
  "confidence": 0.85,
  "reasoning": "POST returns runId + streamUrl, no content — async run pattern"
}

## FIELD RULES BY API STYLE

| Style | messageField | sessionIdField | responseContentField |
|-------|-------------|----------------|---------------------|
| chat_stateless | "messages" | "" or "session_id" | "choices[0].message.content" or "response" |
| single_message | "message"/"prompt"/"query" | "session_id" | "response" |
| langserve | "input" | "config.configurable.session_id" | "output" |
| flowise | "question" | "sessionId" | "text" |
| dify | "inputs" or "query" | "conversation_id" | "answer" |
| haystack | "query" | "" | "answers[0]" |
| botpress | "payload.text" | "conversationId" | "responses[0].payload.text" |
| task_based | "message" | "sessionId" | (N/A — content from SSE stream) |

## TASK_BASED FIELD RULES

When apiStyle is "task_based", you MUST include these additional fields:
- runIdField: JSON path to extract run/task ID from POST response (e.g., "runId", "id", "taskId")
- streamEndpoint: endpoint pattern with {runId} placeholder (e.g., "/v1/runs/{runId}/stream")
- streamMethod: "GET" (most common) or "POST" (e.g., A2A sendSubscribe, JSON-RPC streaming)
- sseContentType: the "type" field in SSE JSON data that carries text chunks (e.g., "text.delta", "content.delta")
- sseContentField: JSON field within that event holding the text (e.g., "text", "content", "delta")
- sseDoneType: the "type" field in SSE JSON data signaling completion (e.g., "run.completed", "done")

Determining streamMethod: If the stream URL pattern looks like a resource path (e.g., /runs/{runId}/stream), use GET.
If it looks like an RPC call (e.g., /tasks/sendSubscribe) or requires a request body, use POST.

## RESPONSE FIELD PATHS

Use dot notation and array access:
- "response" - simple field
- "choices[0].message.content" - OpenAI format
- "content[0].text" - Anthropic format
- "output" - LangServe
- "text" - Flowise
- "answer" - Dify
- "data.response" - nested field

Return ONLY valid JSON, no markdown.`

	// Build probe summary
	var probeSummary strings.Builder
	probeSummary.WriteString("Probe attempts:\n\n")
	for i, attempt := range probeLog {
		probeSummary.WriteString(fmt.Sprintf("Attempt %d:\n", i+1))
		probeSummary.WriteString(fmt.Sprintf("Request: %s\n", attempt.Request))
		probeSummary.WriteString(fmt.Sprintf("Status: %d\n", attempt.StatusCode))
		if attempt.Error != "" {
			probeSummary.WriteString(fmt.Sprintf("Error: %s\n", attempt.Error))
		} else {
			probeSummary.WriteString(fmt.Sprintf("Response: %s\n", truncate(attempt.Response, 500)))
		}
		probeSummary.WriteString("\n")
	}

	// Add session endpoint info if available
	sessionInfo := ""
	if agent.EndpointConfig.SessionEndpoint != "" {
		sessionInfo = fmt.Sprintf("\nSession Endpoint: %s%s (suggests explicit session creation)",
			agent.EndpointConfig.BaseURL, agent.EndpointConfig.SessionEndpoint)
	}

	userPrompt := fmt.Sprintf(`Analyze this AI agent's API:

Agent: %s
Description: %s
Conversation Endpoint: %s%s

%s

Based on the probe results, determine:
1. The correct request/response format
2. How sessions are managed for multi-turn conversations
3. Whether conversation history should be sent in requests
4. Whether the response contains a run/task ID + stream URL instead of direct content (task_based pattern)`,
		agent.Name,
		agent.Description,
		agent.EndpointConfig.Endpoint,
		sessionInfo,
		probeSummary.String())

	maxTokens := int64(2000)

	resp, err := a.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:                a.model,
		MaxCompletionTokens:  openai.Int(maxTokens),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from GPT")
	}

	content := resp.Choices[0].Message.Content
	content = cleanJSONResponse(content)

	// Parse response
	var result struct {
		models.InferredAPISchema
		Reasoning string `json:"reasoning"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse GPT response: %w", err)
	}

	schema := result.InferredAPISchema
	schema.InferredAt = time.Now()
	schema.InferenceMethod = "gpt_analysis"
	schema.RawAnalysis = result.Reasoning

	// Set defaults if not provided
	if schema.RequestMethod == "" {
		schema.RequestMethod = "POST"
	}
	if schema.RequestContentType == "" {
		schema.RequestContentType = "application/json"
	}
	if schema.MessageField == "" {
		schema.MessageField = "message"
	}
	if schema.ResponseContentField == "" {
		schema.ResponseContentField = "response"
	}

	return &schema, nil
}

// cleanJSONResponse removes markdown code blocks from JSON response
func cleanJSONResponse(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, `"true"`, `true`)
	content = strings.ReplaceAll(content, `"false"`, `false`)
	return content
}

// truncateSpec truncates OpenAPI spec for prompt
func truncateSpec(spec string, maxLen int) string {
	if len(spec) <= maxLen {
		return spec
	}
	return spec[:maxLen] + "\n... (truncated)"
}
