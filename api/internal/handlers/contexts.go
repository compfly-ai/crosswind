package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/compfly-ai/crosswind/internal/services"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	// MaxFileSize is the maximum allowed size for individual uploaded files (20MB)
	MaxFileSize = 20 << 20 // 20MB

	// MaxTotalUploadSize is the maximum total size for all files in a request (100MB)
	MaxTotalUploadSize = 100 << 20 // 100MB
)

// ContextHandlers handles context-related HTTP requests
type ContextHandlers struct {
	services *services.Services
	logger   *zap.Logger
}

// NewContextHandlers creates a new context handlers instance
func NewContextHandlers(svc *services.Services, logger *zap.Logger) *ContextHandlers {
	return &ContextHandlers{
		services: svc,
		logger:   logger,
	}
}

// Create handles POST /v1/contexts
func (h *ContextHandlers) Create(c *gin.Context) {
	// Check if context service is available
	if h.services.Context == nil {
		respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Context service not configured", nil)
		return
	}

	// Parse multipart form (max 100MB total)
	if err := c.Request.ParseMultipartForm(100 << 20); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form", gin.H{"error": err.Error()})
		return
	}

	// Get form fields
	name := c.PostForm("name")
	if name == "" {
		respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "Name is required", gin.H{"field": "name"})
		return
	}
	description := c.PostForm("description")

	// Get uploaded files
	form, err := c.MultipartForm()
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to get multipart form", nil)
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "At least one file is required", gin.H{"field": "files"})
		return
	}

	// Validate individual file sizes (max 20MB per file)
	for _, file := range files {
		if file.Size > MaxFileSize {
			respondWithError(c, http.StatusBadRequest, "FILE_TOO_LARGE",
				fmt.Sprintf("File '%s' exceeds maximum size of %dMB", file.Filename, MaxFileSize>>20),
				gin.H{
					"file":      file.Filename,
					"size":      file.Size,
					"maxSize":   MaxFileSize,
					"maxSizeMB": MaxFileSize >> 20,
				})
			return
		}
	}

	ctxDoc, err := h.services.Context.Create(c.Request.Context(), name, description, files)
	if err != nil {
		if errors.Is(err, services.ErrNoFilesProvided) {
			respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "At least one file is required", nil)
		} else if errors.Is(err, services.ErrUnsupportedFileType) {
			// Provide helpful error message with supported types
			respondWithError(c, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE", err.Error(), gin.H{
				"supportedTypes": []string{"PDF (.pdf)", "Markdown (.md)", "Text (.txt)", "CSV (.csv)", "Excel (.xlsx, .xls)", "JSON (.json)"},
			})
		} else if errors.Is(err, services.ErrFileTooLarge) {
			respondWithError(c, http.StatusBadRequest, "FILE_TOO_LARGE", err.Error(), nil)
		} else if strings.Contains(err.Error(), "unsupported file type") {
			// Wrapped error - extract and provide helpful message
			respondWithError(c, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE", err.Error(), gin.H{
				"supportedTypes": []string{"PDF (.pdf)", "Markdown (.md)", "Text (.txt)", "CSV (.csv)", "Excel (.xlsx, .xls)", "JSON (.json)"},
			})
		} else {
			h.logger.Error("failed to create context", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create context", nil)
		}
		return
	}

	c.JSON(http.StatusCreated, ctxDoc)
}

// Get handles GET /v1/contexts/:contextId
func (h *ContextHandlers) Get(c *gin.Context) {
	contextID := c.Param("contextId")

	if h.services.Context == nil {
		respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Context service not configured", nil)
		return
	}

	ctxDoc, err := h.services.Context.Get(c.Request.Context(), contextID)
	if err != nil {
		if err == services.ErrContextNotFound {
			respondWithError(c, http.StatusNotFound, "CONTEXT_NOT_FOUND", "Context not found", gin.H{"contextId": contextID})
			return
		}
		h.logger.Error("failed to get context", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get context", nil)
		return
	}

	c.JSON(http.StatusOK, ctxDoc)
}

// List handles GET /v1/contexts
func (h *ContextHandlers) List(c *gin.Context) {
	if h.services.Context == nil {
		respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Context service not configured", nil)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	response, err := h.services.Context.List(c.Request.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to list contexts", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list contexts", nil)
		return
	}

	c.JSON(http.StatusOK, response)
}

// Delete handles DELETE /v1/contexts/:contextId
func (h *ContextHandlers) Delete(c *gin.Context) {
	contextID := c.Param("contextId")

	if h.services.Context == nil {
		respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Context service not configured", nil)
		return
	}

	err := h.services.Context.Delete(c.Request.Context(), contextID)
	if err != nil {
		if err == services.ErrContextNotFound {
			respondWithError(c, http.StatusNotFound, "CONTEXT_NOT_FOUND", "Context not found", gin.H{"contextId": contextID})
			return
		}
		h.logger.Error("failed to delete context", zap.Error(err))
		respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to delete context", nil)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetFile handles GET /v1/contexts/:contextId/files/:fileName
func (h *ContextHandlers) GetFile(c *gin.Context) {
	contextID := c.Param("contextId")
	fileName := c.Param("fileName")

	if h.services.Context == nil {
		respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Context service not configured", nil)
		return
	}

	reader, contentType, err := h.services.Context.GetFile(c.Request.Context(), contextID, fileName)
	if err != nil {
		if err == services.ErrContextNotFound {
			respondWithError(c, http.StatusNotFound, "CONTEXT_NOT_FOUND", "Context not found", gin.H{"contextId": contextID})
			return
		}
		h.logger.Error("failed to get file", zap.Error(err))
		respondWithError(c, http.StatusNotFound, "FILE_NOT_FOUND", err.Error(), nil)
		return
	}
	defer reader.Close()

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

// AddFiles handles POST /v1/contexts/:contextId/files
func (h *ContextHandlers) AddFiles(c *gin.Context) {
	contextID := c.Param("contextId")

	if h.services.Context == nil {
		respondWithError(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Context service not configured", nil)
		return
	}

	// Parse multipart form (max 100MB total)
	if err := c.Request.ParseMultipartForm(100 << 20); err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to parse multipart form", gin.H{"error": err.Error()})
		return
	}

	// Get uploaded files
	form, err := c.MultipartForm()
	if err != nil {
		respondWithError(c, http.StatusBadRequest, "INVALID_REQUEST", "Failed to get multipart form", nil)
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "At least one file is required", gin.H{"field": "files"})
		return
	}

	// Validate individual file sizes (max 20MB per file)
	for _, file := range files {
		if file.Size > MaxFileSize {
			respondWithError(c, http.StatusBadRequest, "FILE_TOO_LARGE",
				fmt.Sprintf("File '%s' exceeds maximum size of %dMB", file.Filename, MaxFileSize>>20),
				gin.H{
					"file":      file.Filename,
					"size":      file.Size,
					"maxSize":   MaxFileSize,
					"maxSizeMB": MaxFileSize >> 20,
				})
			return
		}
	}

	ctxDoc, err := h.services.Context.AddFiles(c.Request.Context(), contextID, files)
	if err != nil {
		if errors.Is(err, services.ErrContextNotFound) {
			respondWithError(c, http.StatusNotFound, "CONTEXT_NOT_FOUND", "Context not found", gin.H{"contextId": contextID})
		} else if errors.Is(err, services.ErrNoFilesProvided) {
			respondWithError(c, http.StatusBadRequest, "VALIDATION_ERROR", "At least one file is required", nil)
		} else if errors.Is(err, services.ErrUnsupportedFileType) || strings.Contains(err.Error(), "unsupported file type") {
			respondWithError(c, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE", err.Error(), gin.H{
				"supportedTypes": []string{"PDF (.pdf)", "Markdown (.md)", "Text (.txt)", "CSV (.csv)", "Excel (.xlsx, .xls)", "JSON (.json)"},
			})
		} else if errors.Is(err, services.ErrFileTooLarge) {
			respondWithError(c, http.StatusBadRequest, "FILE_TOO_LARGE", err.Error(), nil)
		} else if strings.HasPrefix(err.Error(), "file already exists:") {
			respondWithError(c, http.StatusConflict, "FILE_EXISTS", err.Error(), nil)
		} else {
			h.logger.Error("failed to add files", zap.Error(err))
			respondWithError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to add files", nil)
		}
		return
	}

	c.JSON(http.StatusOK, ctxDoc)
}
