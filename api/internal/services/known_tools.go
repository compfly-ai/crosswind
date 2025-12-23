package services

import "github.com/compfly-ai/crosswind/internal/models"

// KnownTools is a registry of common enterprise tools with default permissions
// Used to expand simple tool names (e.g., "salesforce") to full ToolDefinitions
var KnownTools = map[string]models.ToolDefinition{
	// CRM Systems
	"salesforce": {
		Name:         "salesforce",
		Type:         "crm",
		Permissions:  []string{"read:contacts", "read:accounts", "read:opportunities", "write:opportunities", "read:reports"},
		CanAccessPII: true,
		Description:  "Salesforce CRM - customer relationship management",
	},
	"hubspot": {
		Name:         "hubspot",
		Type:         "crm",
		Permissions:  []string{"read:contacts", "read:companies", "read:deals", "write:deals", "read:tickets"},
		CanAccessPII: true,
		Description:  "HubSpot CRM - marketing, sales, and service platform",
	},
	"dynamics365": {
		Name:         "dynamics365",
		Type:         "crm",
		Permissions:  []string{"read:contacts", "read:accounts", "read:opportunities", "write:opportunities"},
		CanAccessPII: true,
		Description:  "Microsoft Dynamics 365 CRM",
	},

	// Messaging & Communication
	"slack": {
		Name:         "slack",
		Type:         "messaging",
		Permissions:  []string{"read:channels", "read:messages", "send:messages", "read:users"},
		CanAccessPII: false,
		Description:  "Slack - team messaging and collaboration",
	},
	"teams": {
		Name:         "teams",
		Type:         "messaging",
		Permissions:  []string{"read:channels", "read:messages", "send:messages", "read:users"},
		CanAccessPII: false,
		Description:  "Microsoft Teams - collaboration platform",
	},
	"email": {
		Name:         "email",
		Type:         "messaging",
		Permissions:  []string{"read:inbox", "send:email", "read:contacts"},
		CanAccessPII: true,
		Description:  "Email system (Gmail, Outlook, etc.)",
	},
	"gmail": {
		Name:         "gmail",
		Type:         "messaging",
		Permissions:  []string{"read:inbox", "send:email", "read:contacts", "read:calendar"},
		CanAccessPII: true,
		Description:  "Google Gmail and Calendar",
	},
	"outlook": {
		Name:         "outlook",
		Type:         "messaging",
		Permissions:  []string{"read:inbox", "send:email", "read:contacts", "read:calendar"},
		CanAccessPII: true,
		Description:  "Microsoft Outlook email and calendar",
	},

	// Project Management
	"jira": {
		Name:         "jira",
		Type:         "project_management",
		Permissions:  []string{"read:issues", "write:issues", "read:projects", "write:comments"},
		CanAccessPII: false,
		Description:  "Atlassian Jira - issue and project tracking",
	},
	"asana": {
		Name:         "asana",
		Type:         "project_management",
		Permissions:  []string{"read:tasks", "write:tasks", "read:projects", "read:workspaces"},
		CanAccessPII: false,
		Description:  "Asana - project and task management",
	},
	"monday": {
		Name:         "monday",
		Type:         "project_management",
		Permissions:  []string{"read:boards", "write:items", "read:users"},
		CanAccessPII: false,
		Description:  "Monday.com - work management platform",
	},
	"linear": {
		Name:         "linear",
		Type:         "project_management",
		Permissions:  []string{"read:issues", "write:issues", "read:projects", "read:teams"},
		CanAccessPII: false,
		Description:  "Linear - issue tracking for software teams",
	},

	// Code & Development
	"github": {
		Name:         "github",
		Type:         "code_repository",
		Permissions:  []string{"read:repos", "write:repos", "read:issues", "write:issues", "read:pull_requests"},
		CanAccessPII: false,
		Description:  "GitHub - code hosting and collaboration",
	},
	"gitlab": {
		Name:         "gitlab",
		Type:         "code_repository",
		Permissions:  []string{"read:repos", "write:repos", "read:issues", "write:issues", "read:merge_requests"},
		CanAccessPII: false,
		Description:  "GitLab - DevOps platform",
	},
	"bitbucket": {
		Name:         "bitbucket",
		Type:         "code_repository",
		Permissions:  []string{"read:repos", "write:repos", "read:pull_requests"},
		CanAccessPII: false,
		Description:  "Atlassian Bitbucket - Git repository management",
	},

	// Documentation & Knowledge
	"confluence": {
		Name:         "confluence",
		Type:         "documentation",
		Permissions:  []string{"read:pages", "write:pages", "read:spaces"},
		CanAccessPII: false,
		Description:  "Atlassian Confluence - team documentation",
	},
	"notion": {
		Name:         "notion",
		Type:         "documentation",
		Permissions:  []string{"read:pages", "write:pages", "read:databases", "write:databases"},
		CanAccessPII: false,
		Description:  "Notion - workspace and documentation",
	},
	"google_docs": {
		Name:         "google_docs",
		Type:         "documentation",
		Permissions:  []string{"read:documents", "write:documents", "read:drive"},
		CanAccessPII: true,
		Description:  "Google Docs and Drive",
	},
	"sharepoint": {
		Name:         "sharepoint",
		Type:         "documentation",
		Permissions:  []string{"read:documents", "write:documents", "read:sites"},
		CanAccessPII: true,
		Description:  "Microsoft SharePoint",
	},

	// Database & Data
	"database": {
		Name:         "database",
		Type:         "database",
		Permissions:  []string{"read:tables", "write:tables", "execute:queries"},
		CanAccessPII: true,
		Description:  "Generic database access (SQL, NoSQL)",
	},
	"snowflake": {
		Name:         "snowflake",
		Type:         "data_warehouse",
		Permissions:  []string{"read:tables", "execute:queries", "read:warehouses"},
		CanAccessPII: true,
		Description:  "Snowflake data warehouse",
	},
	"bigquery": {
		Name:         "bigquery",
		Type:         "data_warehouse",
		Permissions:  []string{"read:tables", "execute:queries", "read:datasets"},
		CanAccessPII: true,
		Description:  "Google BigQuery data warehouse",
	},

	// Customer Support
	"zendesk": {
		Name:         "zendesk",
		Type:         "customer_support",
		Permissions:  []string{"read:tickets", "write:tickets", "read:users", "read:organizations"},
		CanAccessPII: true,
		Description:  "Zendesk - customer support platform",
	},
	"intercom": {
		Name:         "intercom",
		Type:         "customer_support",
		Permissions:  []string{"read:conversations", "send:messages", "read:users", "read:companies"},
		CanAccessPII: true,
		Description:  "Intercom - customer messaging platform",
	},
	"freshdesk": {
		Name:         "freshdesk",
		Type:         "customer_support",
		Permissions:  []string{"read:tickets", "write:tickets", "read:contacts"},
		CanAccessPII: true,
		Description:  "Freshdesk - customer support software",
	},

	// Finance & Payments
	"stripe": {
		Name:         "stripe",
		Type:         "payments",
		Permissions:  []string{"read:payments", "read:customers", "read:subscriptions"},
		CanAccessPII: true,
		Description:  "Stripe - payment processing",
	},
	"quickbooks": {
		Name:         "quickbooks",
		Type:         "accounting",
		Permissions:  []string{"read:invoices", "read:customers", "read:accounts", "read:reports"},
		CanAccessPII: true,
		Description:  "QuickBooks - accounting software",
	},
	"netsuite": {
		Name:         "netsuite",
		Type:         "erp",
		Permissions:  []string{"read:transactions", "read:customers", "read:inventory", "read:reports"},
		CanAccessPII: true,
		Description:  "NetSuite - ERP and financial management",
	},

	// HR & People
	"workday": {
		Name:         "workday",
		Type:         "hr",
		Permissions:  []string{"read:employees", "read:org_chart", "read:time_off", "read:compensation"},
		CanAccessPII: true,
		Description:  "Workday - HR management system",
	},
	"bamboohr": {
		Name:         "bamboohr",
		Type:         "hr",
		Permissions:  []string{"read:employees", "read:time_off", "read:directory"},
		CanAccessPII: true,
		Description:  "BambooHR - HR software",
	},
	"greenhouse": {
		Name:         "greenhouse",
		Type:         "recruiting",
		Permissions:  []string{"read:candidates", "read:jobs", "read:applications", "write:notes"},
		CanAccessPII: true,
		Description:  "Greenhouse - recruiting and hiring",
	},

	// Cloud & Infrastructure
	"aws": {
		Name:         "aws",
		Type:         "cloud",
		Permissions:  []string{"read:resources", "read:logs", "read:metrics"},
		CanAccessPII: false,
		Description:  "Amazon Web Services",
	},
	"gcp": {
		Name:         "gcp",
		Type:         "cloud",
		Permissions:  []string{"read:resources", "read:logs", "read:metrics"},
		CanAccessPII: false,
		Description:  "Google Cloud Platform",
	},
	"azure": {
		Name:         "azure",
		Type:         "cloud",
		Permissions:  []string{"read:resources", "read:logs", "read:metrics"},
		CanAccessPII: false,
		Description:  "Microsoft Azure",
	},

	// Generic Tools
	"calendar": {
		Name:         "calendar",
		Type:         "productivity",
		Permissions:  []string{"read:events", "write:events", "read:availability"},
		CanAccessPII: false,
		Description:  "Calendar management (Google, Outlook, etc.)",
	},
	"web_browser": {
		Name:         "web_browser",
		Type:         "browsing",
		Permissions:  []string{"browse:web", "read:pages"},
		CanAccessPII: false,
		Description:  "Web browsing capability",
	},
	"file_system": {
		Name:         "file_system",
		Type:         "storage",
		Permissions:  []string{"read:files", "write:files", "list:directories"},
		CanAccessPII: true,
		Description:  "Local or cloud file system access",
	},
}

// ExpandToolNames converts a list of tool names to full ToolDefinitions
// Unknown tools are included with minimal information
func ExpandToolNames(names []string) []models.ToolDefinition {
	result := make([]models.ToolDefinition, 0, len(names))
	for _, name := range names {
		if tool, ok := KnownTools[name]; ok {
			result = append(result, tool)
		} else {
			// Unknown tool - create minimal definition
			result = append(result, models.ToolDefinition{
				Name:        name,
				Type:        "custom",
				Permissions: []string{},
			})
		}
	}
	return result
}

// GetToolDefinition returns the definition for a known tool, or nil if not found
func GetToolDefinition(name string) *models.ToolDefinition {
	if tool, ok := KnownTools[name]; ok {
		return &tool
	}
	return nil
}

// IsKnownTool returns true if the tool name is in the registry
func IsKnownTool(name string) bool {
	_, ok := KnownTools[name]
	return ok
}

// GetToolsByType returns all tools of a specific type
func GetToolsByType(toolType string) []models.ToolDefinition {
	result := make([]models.ToolDefinition, 0)
	for _, tool := range KnownTools {
		if tool.Type == toolType {
			result = append(result, tool)
		}
	}
	return result
}

// GetPIICapableTools returns all tools that can access PII
func GetPIICapableTools() []models.ToolDefinition {
	result := make([]models.ToolDefinition, 0)
	for _, tool := range KnownTools {
		if tool.CanAccessPII {
			result = append(result, tool)
		}
	}
	return result
}
