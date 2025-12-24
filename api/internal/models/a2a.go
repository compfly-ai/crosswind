package models

// A2AAgentCard represents the structure of an A2A agent card
// Based on A2A Protocol Specification v0.3
type A2AAgentCard struct {
	// Required fields
	ID              string              `json:"id"`                        // REQUIRED: Unique agent identifier
	Name            string              `json:"name"`                      // REQUIRED: Human-readable agent name
	ProtocolVersion string              `json:"protocolVersion"`           // REQUIRED: Latest supported A2A version (e.g., "0.3")
	Provider        A2AProvider         `json:"provider"`                  // REQUIRED: Publisher/maintainer information
	Capabilities    A2ACapabilities     `json:"capabilities"`              // REQUIRED: Feature support declaration
	Interfaces      []A2AInterface      `json:"interfaces"`                // REQUIRED: Supported protocol bindings
	SecuritySchemes []A2ASecurityScheme `json:"securitySchemes,omitempty"` // REQUIRED: Authentication methods
	Security        map[string][]string `json:"security,omitempty"`        // REQUIRED: Security requirements mapping

	// Optional fields
	Description               string                 `json:"description,omitempty"`               // OPTIONAL: Agent purpose/capabilities summary
	Version                   string                 `json:"version,omitempty"`                   // OPTIONAL: Agent version
	Skills                    []A2ASkill             `json:"skills,omitempty"`                    // OPTIONAL: Available agent skills/actions
	Extensions                []A2AExtension         `json:"extensions,omitempty"`                // OPTIONAL: Additional functionality
	SupportsExtendedAgentCard bool                   `json:"supportsExtendedAgentCard,omitempty"` // OPTIONAL: Extended card availability
	Metadata                  map[string]interface{} `json:"metadata,omitempty"`                  // OPTIONAL: Custom key-value attributes
}

// A2AProvider describes the entity publishing/maintaining the agent
type A2AProvider struct {
	ID   string `json:"id,omitempty"`  // OPTIONAL: Provider identifier
	Name string `json:"name"`          // REQUIRED: Provider display name
	URL  string `json:"url,omitempty"` // OPTIONAL: Provider website/contact URL
}

// A2ACapabilities declares which optional features the agent implements
type A2ACapabilities struct {
	Streaming         bool `json:"streaming,omitempty"`         // Real-time event delivery support
	PushNotifications bool `json:"pushNotifications,omitempty"` // Webhook-based async updates
}

// A2AInterface specifies a supported protocol binding and endpoint
type A2AInterface struct {
	Type string `json:"type"` // REQUIRED: Protocol identifier ("json-rpc", "http", "grpc")
	URL  string `json:"url"`  // REQUIRED: Service endpoint URI
}

// A2ASecurityScheme defines an authentication mechanism
type A2ASecurityScheme struct {
	Type        string `json:"type"`                  // REQUIRED: "apiKey", "http", "oauth2", "openIdConnect", "mutualTLS"
	Description string `json:"description,omitempty"` // OPTIONAL: Authentication approach overview
	// APIKey specific
	Name string `json:"name,omitempty"` // Header/query param name for apiKey
	In   string `json:"in,omitempty"`   // Location: "header", "query", "cookie"
	// HTTP specific
	Scheme string `json:"scheme,omitempty"` // e.g., "bearer", "basic"
}

// A2ASkill represents a specific capability or task the agent can perform
type A2ASkill struct {
	ID           string                 `json:"id"`                     // REQUIRED: Skill identifier
	Name         string                 `json:"name"`                   // REQUIRED: Human-readable skill name
	Description  string                 `json:"description,omitempty"` // OPTIONAL: Skill purpose/usage details
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty"` // OPTIONAL: Expected message content structure
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty"` // OPTIONAL: Artifact/response structure
}

// A2AExtension represents additional functionality beyond core specification
type A2AExtension struct {
	URI         string `json:"uri"`                   // REQUIRED: Extension identifier URI
	Description string `json:"description,omitempty"` // OPTIONAL: Extension purpose
}

// A2AAgentSpecResponse contains the agent card and pre-populated agent object
type A2AAgentSpecResponse struct {
	// Raw agent card data (for display purposes)
	AgentCard *A2AAgentCard `json:"agentCard"`

	// Pre-populated Agent object - frontend can use to display/edit
	// Fields like Goal, Industry, Credentials need user input
	Agent *Agent `json:"agent"`
}
