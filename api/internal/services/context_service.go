package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-eval/agent-eval/internal/models"
	"github.com/agent-eval/agent-eval/internal/repository/mongo"
	"github.com/agent-eval/agent-eval/pkg/storage"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Context service errors
var (
	ErrContextNotFound     = errors.New("context not found")
	ErrUnsupportedFileType = errors.New("unsupported file type")
	ErrFileTooLarge        = errors.New("file too large")
	ErrNoFilesProvided     = errors.New("no files provided")
)

// MaxFileSize is the maximum allowed file size (50MB)
const MaxFileSize = 50 * 1024 * 1024

// ContextService handles context document operations
type ContextService struct {
	repos       *mongo.Repositories
	gcs         *storage.GCSClient
	environment string
	logger      *zap.Logger
}

// NewContextService creates a new context service
func NewContextService(repos *mongo.Repositories, gcs *storage.GCSClient, environment string, logger *zap.Logger) *ContextService {
	return &ContextService{
		repos:       repos,
		gcs:         gcs,
		environment: environment,
		logger:      logger,
	}
}

// Create creates a new context with uploaded files
func (s *ContextService) Create(ctx context.Context, name, description string, files []*multipart.FileHeader) (*models.Context, error) {
	if len(files) == 0 {
		return nil, ErrNoFilesProvided
	}

	// Generate context ID
	contextID := fmt.Sprintf("ctx_%s_%s", time.Now().Format("20060102"), uuid.New().String()[:8])
	gcsPath := storage.BuildContextPath(contextID)

	// Prepare file metadata
	contextFiles := make([]models.ContextFile, 0, len(files))
	var totalSize int64

	for _, fh := range files {
		// Validate file size
		if fh.Size > MaxFileSize {
			return nil, fmt.Errorf("%w: %s exceeds %dMB limit", ErrFileTooLarge, fh.Filename, MaxFileSize/(1024*1024))
		}

		// Get content type
		contentType := getContentType(fh)
		if !models.SupportedContextFileTypes[contentType] {
			return nil, fmt.Errorf("%w: %s (%s)", ErrUnsupportedFileType, fh.Filename, contentType)
		}

		contextFiles = append(contextFiles, models.ContextFile{
			Name:          fh.Filename,
			Size:          fh.Size,
			ContentType:   contentType,
			Status:        models.FileStatusUploading,
			GCSObjectName: storage.BuildFilePath(contextID, fh.Filename),
		})
		totalSize += fh.Size
	}

	// Create context record
	ctxDoc := &models.Context{
		ContextID:   contextID,
		Environment: s.environment,
		Name:        name,
		Description: description,
		Status:      models.ContextStatusProcessing,
		Files:       contextFiles,
		GCSPath:     gcsPath,
	}

	if err := s.repos.Contexts.Create(ctx, ctxDoc); err != nil {
		return nil, fmt.Errorf("failed to create context: %w", err)
	}

	// Upload files to GCS in background
	go s.uploadFilesInBackground(contextID, files)

	return ctxDoc, nil
}

// Text-based content types that can be processed directly without Python worker
var textBasedContentTypes = map[string]bool{
	"text/plain":      true,
	"text/markdown":   true,
	"text/x-markdown": true,
	"text/csv":        true,
	"application/json": true,
}

// uploadFilesInBackground uploads files to GCS and updates status
func (s *ContextService) uploadFilesInBackground(contextID string, files []*multipart.FileHeader) {
	ctx := context.Background()

	var successCount, failCount, readyCount int

	for _, fh := range files {
		contentType := getContentType(fh)
		objectName := storage.BuildFilePath(contextID, fh.Filename)

		// Open file
		file, err := fh.Open()
		if err != nil {
			s.logger.Error("failed to open file",
				zap.String("contextId", contextID),
				zap.String("file", fh.Filename),
				zap.Error(err),
			)
			s.repos.Contexts.UpdateFileStatus(ctx, contextID, fh.Filename, models.FileStatusFailed, map[string]interface{}{
				"error": err.Error(),
			})
			failCount++
			continue
		}

		// For text-based files, read content before uploading
		var textContent string
		if textBasedContentTypes[contentType] {
			content, readErr := io.ReadAll(file)
			if readErr != nil {
				file.Close()
				s.logger.Error("failed to read text file",
					zap.String("contextId", contextID),
					zap.String("file", fh.Filename),
					zap.Error(readErr),
				)
				s.repos.Contexts.UpdateFileStatus(ctx, contextID, fh.Filename, models.FileStatusFailed, map[string]interface{}{
					"error": readErr.Error(),
				})
				failCount++
				continue
			}
			textContent = string(content)
			// Reset file reader for GCS upload
			file.Close()
			file, _ = fh.Open()
		}

		// Upload to GCS
		if err := s.gcs.UploadFile(ctx, objectName, file, contentType); err != nil {
			file.Close()
			s.logger.Error("failed to upload file to GCS",
				zap.String("contextId", contextID),
				zap.String("file", fh.Filename),
				zap.Error(err),
			)
			s.repos.Contexts.UpdateFileStatus(ctx, contextID, fh.Filename, models.FileStatusFailed, map[string]interface{}{
				"error": err.Error(),
			})
			failCount++
			continue
		}
		file.Close()

		// For text-based files, mark as ready immediately with extracted text
		if textContent != "" {
			s.repos.Contexts.UpdateFileStatus(ctx, contextID, fh.Filename, models.FileStatusReady, map[string]interface{}{
				"extractedText":  textContent,
				"extractedChars": len(textContent),
			})
			readyCount++
			s.logger.Info("processed text file inline",
				zap.String("contextId", contextID),
				zap.String("file", fh.Filename),
				zap.Int("chars", len(textContent)),
			)
		} else {
			// Binary files (PDF, Excel, etc.) need Python worker processing
			s.repos.Contexts.UpdateFileStatus(ctx, contextID, fh.Filename, models.FileStatusProcessing, nil)
			s.logger.Info("uploaded binary file to storage, awaiting worker processing",
				zap.String("contextId", contextID),
				zap.String("file", fh.Filename),
				zap.String("contentType", contentType),
			)
		}
		successCount++
	}

	// Update context status based on results
	if failCount == len(files) {
		s.repos.Contexts.UpdateStatus(ctx, contextID, models.ContextStatusFailed, "all files failed to upload")
	} else if readyCount == successCount {
		// All files are text-based and processed - context is ready
		s.repos.Contexts.UpdateStatus(ctx, contextID, models.ContextStatusReady, "")
		s.logger.Info("context ready (all text files processed inline)",
			zap.String("contextId", contextID),
			zap.Int("files", readyCount),
		)
	} else {
		// Some files need Python worker processing
		s.logger.Info("context files uploaded, some awaiting worker processing",
			zap.String("contextId", contextID),
			zap.Int("ready", readyCount),
			zap.Int("processing", successCount-readyCount),
			zap.Int("failed", failCount),
		)
	}
}

// Get retrieves a context by ID
func (s *ContextService) Get(ctx context.Context, contextID string) (*models.Context, error) {
	ctxDoc, err := s.repos.Contexts.FindByID(ctx, contextID)
	if err != nil {
		return nil, ErrContextNotFound
	}
	return ctxDoc, nil
}

// List lists contexts for an organization
func (s *ContextService) List(ctx context.Context, limit, offset int) (*models.ContextListResponse, error) {
	contexts, total, err := s.repos.Contexts.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	summaries := make([]models.ContextSummaryResponse, 0, len(contexts))
	for _, c := range contexts {
		var totalSize int64
		for _, f := range c.Files {
			totalSize += f.Size
		}
		summaries = append(summaries, models.ContextSummaryResponse{
			ContextID: c.ContextID,
			Name:      c.Name,
			Status:    c.Status,
			FileCount: len(c.Files),
			TotalSize: totalSize,
			Summary:   c.Summary,
			CreatedAt: c.CreatedAt,
		})
	}

	return &models.ContextListResponse{
		Contexts: summaries,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

// Delete deletes a context and its files from GCS
func (s *ContextService) Delete(ctx context.Context, contextID string) error {
	// Get context to verify it exists
	ctxDoc, err := s.repos.Contexts.FindByID(ctx, contextID)
	if err != nil {
		return ErrContextNotFound
	}

	// Delete files from GCS
	if err := s.gcs.DeletePrefix(ctx, ctxDoc.GCSPath); err != nil {
		s.logger.Error("failed to delete context files from GCS",
			zap.String("contextId", contextID),
			zap.String("gcsPath", ctxDoc.GCSPath),
			zap.Error(err),
		)
		// Continue with MongoDB deletion even if GCS fails
	}

	// Delete from MongoDB
	if err := s.repos.Contexts.Delete(ctx, contextID); err != nil {
		return err
	}

	return nil
}

// GetFile returns a reader for a specific file in a context
func (s *ContextService) GetFile(ctx context.Context, contextID, fileName string) (io.ReadCloser, string, error) {
	ctxDoc, err := s.repos.Contexts.FindByID(ctx, contextID)
	if err != nil {
		return nil, "", ErrContextNotFound
	}

	// Find the file
	var file *models.ContextFile
	for i := range ctxDoc.Files {
		if ctxDoc.Files[i].Name == fileName {
			file = &ctxDoc.Files[i]
			break
		}
	}

	if file == nil {
		return nil, "", fmt.Errorf("file not found: %s", fileName)
	}

	reader, err := s.gcs.DownloadFile(ctx, file.GCSObjectName)
	if err != nil {
		return nil, "", err
	}

	return reader, file.ContentType, nil
}

// GetContextsForScenarioGeneration retrieves and validates contexts for scenario generation
func (s *ContextService) GetContextsForScenarioGeneration(ctx context.Context, contextIDs []string) ([]models.Context, error) {
	if len(contextIDs) == 0 {
		return nil, nil
	}

	contexts, err := s.repos.Contexts.FindByIDs(ctx, contextIDs)
	if err != nil {
		return nil, err
	}

	// Check if all requested contexts were found
	if len(contexts) != len(contextIDs) {
		foundIDs := make(map[string]bool)
		for _, c := range contexts {
			foundIDs[c.ContextID] = true
		}
		var missing []string
		for _, id := range contextIDs {
			if !foundIDs[id] {
				missing = append(missing, id)
			}
		}
		return nil, fmt.Errorf("contexts not found or not ready: %v", missing)
	}

	return contexts, nil
}

// AddFiles adds new files to an existing context
func (s *ContextService) AddFiles(ctx context.Context, contextID string, files []*multipart.FileHeader) (*models.Context, error) {
	if len(files) == 0 {
		return nil, ErrNoFilesProvided
	}

	// Verify context exists
	ctxDoc, err := s.repos.Contexts.FindByID(ctx, contextID)
	if err != nil {
		s.logger.Debug("context not found for AddFiles",
			zap.String("contextId", contextID),
			zap.Error(err),
		)
		return nil, ErrContextNotFound
	}

	// Get existing file names to check for duplicates
	existingNames := make(map[string]bool)
	for _, f := range ctxDoc.Files {
		existingNames[f.Name] = true
	}

	// Prepare new file metadata
	newFiles := make([]models.ContextFile, 0, len(files))

	for _, fh := range files {
		// Check for duplicate names
		if existingNames[fh.Filename] {
			return nil, fmt.Errorf("file already exists: %s", fh.Filename)
		}

		// Validate file size
		if fh.Size > MaxFileSize {
			return nil, fmt.Errorf("%w: %s exceeds %dMB limit", ErrFileTooLarge, fh.Filename, MaxFileSize/(1024*1024))
		}

		// Get content type
		contentType := getContentType(fh)
		if !models.SupportedContextFileTypes[contentType] {
			return nil, fmt.Errorf("%w: %s (%s)", ErrUnsupportedFileType, fh.Filename, contentType)
		}

		newFiles = append(newFiles, models.ContextFile{
			Name:          fh.Filename,
			Size:          fh.Size,
			ContentType:   contentType,
			Status:        models.FileStatusUploading,
			GCSObjectName: storage.BuildFilePath(contextID, fh.Filename),
		})
	}

	// Add files to context in MongoDB
	if err := s.repos.Contexts.AddFiles(ctx, contextID, newFiles); err != nil {
		return nil, fmt.Errorf("failed to add files: %w", err)
	}

	// Upload files to GCS in background
	go s.uploadFilesInBackground(contextID, files)

	// Return updated context
	return s.repos.Contexts.FindByID(ctx, contextID)
}

// getContentType determines the content type of a file
func getContentType(fh *multipart.FileHeader) string {
	// First try the declared content type
	contentType := fh.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/octet-stream" {
		// Normalize markdown content types
		if strings.Contains(contentType, "markdown") {
			return "text/markdown"
		}
		return contentType
	}

	// Try extension-based detection
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if ct, ok := models.FileExtensionToContentType[ext]; ok {
		return ct
	}

	// Try content sniffing (magic bytes detection)
	if ct := detectContentTypeFromFile(fh); ct != "" {
		return ct
	}

	return "application/octet-stream"
}

// detectContentTypeFromFile reads the first bytes of a file to detect content type
func detectContentTypeFromFile(fh *multipart.FileHeader) string {
	file, err := fh.Open()
	if err != nil {
		return ""
	}
	defer file.Close()

	// Read first 512 bytes for detection
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil || n == 0 {
		return ""
	}
	buf = buf[:n]

	// Check for PDF magic bytes
	if len(buf) >= 4 && string(buf[:4]) == "%PDF" {
		return "application/pdf"
	}

	// Check for ZIP-based formats (xlsx, docx, pptx)
	if len(buf) >= 4 && buf[0] == 0x50 && buf[1] == 0x4B && buf[2] == 0x03 && buf[3] == 0x04 {
		// It's a ZIP file - could be xlsx, docx, etc.
		// For now, default to xlsx as most common for data contexts
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	// Check for JSON (starts with { or [)
	trimmed := strings.TrimSpace(string(buf))
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return "application/json"
	}

	// Check for CSV (contains commas and newlines, no binary)
	if looksLikeCSV(buf) {
		return "text/csv"
	}

	// Check for plain text / markdown (printable ASCII)
	if looksLikeText(buf) {
		return "text/plain"
	}

	return ""
}

// looksLikeCSV checks if content appears to be CSV
func looksLikeCSV(buf []byte) bool {
	s := string(buf)
	hasComma := strings.Contains(s, ",")
	hasNewline := strings.Contains(s, "\n")
	// Check it's mostly printable
	printable := 0
	for _, b := range buf {
		if b >= 32 && b < 127 || b == '\n' || b == '\r' || b == '\t' {
			printable++
		}
	}
	return hasComma && hasNewline && float64(printable)/float64(len(buf)) > 0.95
}

// looksLikeText checks if content appears to be plain text
func looksLikeText(buf []byte) bool {
	printable := 0
	for _, b := range buf {
		if b >= 32 && b < 127 || b == '\n' || b == '\r' || b == '\t' {
			printable++
		}
	}
	return float64(printable)/float64(len(buf)) > 0.90
}
