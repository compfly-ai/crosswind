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

	"github.com/compfly-ai/crosswind/internal/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// APIAnalyzer uses GPT-5.1 to analyze and infer agent API structure
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
		model:  "gpt-5.1",
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
			result.Schema = schema
			result.Successful = true
			return result, nil
		}
		// Fall through to probing if spec parsing fails
	}

	// Strategy 2: Probe the endpoint
	probeResult, probeLog := a.probeEndpoint(ctx, agent)
	result.ProbeLog = probeLog

	// Strategy 3: Use GPT-5.1 to analyze everything
	schema, err := a.analyzeWithGPT(ctx, agent, probeResult, probeLog)
	if err != nil {
		result.Error = err.Error()
		result.Successful = false
		return result, nil
	}

	result.Schema = schema
	result.Successful = true
	return result, nil
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

// parseSpecWithGPT uses GPT-5.1 to extract API schema from OpenAPI spec
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
- task_based: JSON-RPC with contextId (Google A2A)

## OUTPUT FORMAT

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

## FIELD RULES

| Style | messageField | responseContentField |
|-------|-------------|---------------------|
| chat_stateless | "messages" | "choices[0].message.content" |
| single_message | "message"/"prompt" | "response" |
| langserve | "input" | "output" |
| flowise | "question" | "text" |
| dify | "inputs"/"query" | "answer" |

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

	for _, payload := range probeFormats {
		attempt := a.tryProbe(ctx, probeURL, payload, agent.AuthConfig)
		probeLog = append(probeLog, attempt)

		if attempt.StatusCode >= 200 && attempt.StatusCode < 300 && attempt.Error == "" {
			// Parse response
			var respData map[string]interface{}
			if err := json.Unmarshal([]byte(attempt.Response), &respData); err == nil {
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

// analyzeWithGPT uses GPT-5.1 for comprehensive analysis
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

### RARE STYLES (flag but may need manual config)

#### 8. thread_based (OpenAI Assistants style)
- Messages added to server-managed thread via separate endpoint
Detection: URL contains /threads/ path

#### 9. task_based (Google A2A style)
- Task-oriented with contextId, uses JSON-RPC
Detection: Request has "jsonrpc" field or contextId/taskId structure

## OUTPUT FORMAT

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
3. Whether conversation history should be sent in requests`,
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
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
	}
	return strings.TrimSpace(content)
}

// truncateSpec truncates OpenAPI spec for prompt
func truncateSpec(spec string, maxLen int) string {
	if len(spec) <= maxLen {
		return spec
	}
	return spec[:maxLen] + "\n... (truncated)"
}
