package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Context represents uploaded documents for scenario generation
type Context struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	ContextID   string             `bson:"contextId" json:"contextId"`
	Environment string             `bson:"environment,omitempty" json:"environment,omitempty"` // For processor isolation (production, staging, dev)
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Status      string             `bson:"status" json:"status"` // processing, ready, failed
	Files       []ContextFile      `bson:"files" json:"files"`
	GCSPath     string             `bson:"gcsPath" json:"gcsPath"` // contexts/{contextId}
	Summary     *ContextSummary    `bson:"summary,omitempty" json:"summary,omitempty"`
	Error       string             `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
	ExpiresAt   *time.Time         `bson:"expiresAt,omitempty" json:"expiresAt,omitempty"`
}

// ContextFile represents a single uploaded file
type ContextFile struct {
	Name            string `bson:"name" json:"name"`
	Size            int64  `bson:"size" json:"size"`
	ContentType     string `bson:"contentType" json:"contentType"`
	Status          string `bson:"status" json:"status"` // uploading, processing, ready, failed
	GCSObjectName   string `bson:"gcsObjectName" json:"gcsObjectName"`
	ExtractedText   string `bson:"extractedText,omitempty" json:"-"`                 // Extracted text (stored in GCS, loaded on demand)
	ExtractedChars  int    `bson:"extractedChars,omitempty" json:"extractedChars,omitempty"`
	PageCount       int    `bson:"pageCount,omitempty" json:"pageCount,omitempty"`   // For PDFs
	RowCount        int    `bson:"rowCount,omitempty" json:"rowCount,omitempty"`     // For CSV/Excel
	Error           string `bson:"error,omitempty" json:"error,omitempty"`
}

// ContextSummary provides aggregate stats for a context
type ContextSummary struct {
	TotalFiles      int   `bson:"totalFiles" json:"totalFiles"`
	TotalSize       int64 `bson:"totalSize" json:"totalSize"`
	ExtractedTokens int   `bson:"extractedTokens" json:"extractedTokens"`
	ReadyFiles      int   `bson:"readyFiles" json:"readyFiles"`
	FailedFiles     int   `bson:"failedFiles" json:"failedFiles"`
}

// ContextStatus constants
const (
	ContextStatusProcessing = "processing"
	ContextStatusReady      = "ready"
	ContextStatusFailed     = "failed"
)

// FileStatus constants
const (
	FileStatusUploading  = "uploading"
	FileStatusProcessing = "processing"
	FileStatusReady      = "ready"
	FileStatusFailed     = "failed"
)

// Supported file types
var SupportedContextFileTypes = map[string]bool{
	"application/pdf":                                                        true,
	"text/markdown":                                                          true,
	"text/x-markdown":                                                        true,
	"text/plain":                                                             true,
	"text/csv":                                                               true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":      true, // xlsx
	"application/vnd.ms-excel":                                               true, // xls
	"application/json":                                                       true,
}

// File extensions mapping
var FileExtensionToContentType = map[string]string{
	".pdf":  "application/pdf",
	".md":   "text/markdown",
	".txt":  "text/plain",
	".csv":  "text/csv",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".xls":  "application/vnd.ms-excel",
	".json": "application/json",
}

// CreateContextRequest represents the request body for creating a context
// Note: Files come via multipart/form-data, not JSON
type CreateContextRequest struct {
	Name        string `form:"name" binding:"required"`
	Description string `form:"description"`
}

// ContextListResponse represents a paginated list of contexts
type ContextListResponse struct {
	Contexts []ContextSummaryResponse `json:"contexts"`
	Total    int64                    `json:"total"`
	Limit    int                      `json:"limit"`
	Offset   int                      `json:"offset"`
}

// ContextSummaryResponse represents a summarized view for listings
type ContextSummaryResponse struct {
	ContextID   string          `json:"contextId"`
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	FileCount   int             `json:"fileCount"`
	TotalSize   int64           `json:"totalSize"`
	Summary     *ContextSummary `json:"summary,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
}
