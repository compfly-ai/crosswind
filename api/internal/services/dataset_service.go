package services

import (
	"context"
	"sync"
	"time"

	"github.com/agent-eval/agent-eval/internal/models"
	"github.com/agent-eval/agent-eval/pkg/repository"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

// Cache TTL for categories/evalTypes (5 minutes - these rarely change)
const datasetMetadataCacheTTL = 5 * time.Minute

// datasetMetadataCache holds cached categories and evalTypes
type datasetMetadataCache struct {
	categories []string
	evalTypes  []string
	expiresAt  time.Time
	mu         sync.RWMutex
}

// DatasetService handles dataset business logic
type DatasetService struct {
	datasetRepo repository.DatasetRepository
	cache       *datasetMetadataCache
}

// NewDatasetService creates a new dataset service
func NewDatasetService(datasetRepo repository.DatasetRepository) *DatasetService {
	return &DatasetService{
		datasetRepo: datasetRepo,
		cache:       &datasetMetadataCache{},
	}
}

// InvalidateCache clears the metadata cache (call when datasets are added/updated)
func (s *DatasetService) InvalidateCache() {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.expiresAt = time.Time{}
}

// List lists available datasets
func (s *DatasetService) List(ctx context.Context, category string, isActive bool) (*models.DatasetListResponse, error) {
	datasets, total, err := s.datasetRepo.ListDatasets(ctx, category, isActive)
	if err != nil {
		return nil, err
	}

	summaries := make([]models.DatasetSummary, len(datasets))
	for i, ds := range datasets {
		summaries[i] = models.DatasetSummary{
			DatasetID:    ds.DatasetID,
			Version:      ds.Version,
			Name:         ds.Name,
			Category:     ds.Category,
			EvalType:     ds.EvalType,
			JudgmentMode: ds.JudgmentMode,
			PromptCount:  ds.Metadata.PromptCount,
			IsMultiturn:  ds.Metadata.IsMultiturn,
			License:      ds.License.Type,
			Source:       ds.Source.Name,
		}
	}

	// Get categories and eval types from cache or fetch fresh
	categories, evalTypes := s.getCachedMetadata(ctx, isActive)

	return &models.DatasetListResponse{
		Datasets:   summaries,
		Total:      total,
		Categories: categories,
		EvalTypes:  evalTypes,
	}, nil
}

// getCachedMetadata returns cached categories/evalTypes or fetches fresh from DB
func (s *DatasetService) getCachedMetadata(ctx context.Context, isActive bool) ([]string, []string) {
	// Check cache (read lock)
	s.cache.mu.RLock()
	if time.Now().Before(s.cache.expiresAt) {
		categories := s.cache.categories
		evalTypes := s.cache.evalTypes
		s.cache.mu.RUnlock()
		return categories, evalTypes
	}
	s.cache.mu.RUnlock()

	// Cache miss or expired - fetch fresh (write lock)
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(s.cache.expiresAt) {
		return s.cache.categories, s.cache.evalTypes
	}

	// Fetch from DB
	categories, _ := s.datasetRepo.GetDistinctCategories(ctx, isActive)
	evalTypes, _ := s.datasetRepo.GetDistinctEvalTypes(ctx, isActive)

	// Update cache
	s.cache.categories = categories
	s.cache.evalTypes = evalTypes
	s.cache.expiresAt = time.Now().Add(datasetMetadataCacheTTL)

	return categories, evalTypes
}

// Get retrieves a dataset by ID
func (s *DatasetService) Get(ctx context.Context, datasetID string) (*models.Dataset, error) {
	dataset, err := s.datasetRepo.FindDataset(ctx, datasetID)
	if err != nil {
		if err == mongodriver.ErrNoDocuments {
			return nil, ErrDatasetNotFound
		}
		return nil, err
	}
	return dataset, nil
}
