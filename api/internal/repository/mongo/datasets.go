package mongo

import (
	"context"
	"time"

	"github.com/compfly-ai/crosswind/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DatasetsRepository handles dataset data operations
type DatasetsRepository struct {
	datasetsCollection *mongo.Collection
	promptsCollection  *mongo.Collection
}

// NewDatasetsRepository creates a new datasets repository
func NewDatasetsRepository(db *mongo.Database) *DatasetsRepository {
	datasetsCol := db.Collection("datasets")
	promptsCol := db.Collection("datasetPrompts")

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Datasets indexes
	datasetsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "datasetId", Value: 1}, {Key: "version", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "isShared", Value: 1}, {Key: "isActive", Value: 1}, {Key: "category", Value: 1}},
		},
	})

	// Prompts indexes
	promptsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "datasetId", Value: 1}, {Key: "version", Value: 1}, {Key: "promptId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "datasetId", Value: 1}, {Key: "version", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "severity", Value: 1}, {Key: "category", Value: 1}},
		},
	})

	return &DatasetsRepository{
		datasetsCollection: datasetsCol,
		promptsCollection:  promptsCol,
	}
}

// --- Dataset Operations ---

// ListDatasets lists available datasets
func (r *DatasetsRepository) ListDatasets(ctx context.Context, category string, isActive bool) ([]models.Dataset, int64, error) {
	filter := bson.M{
		"isShared": true,
		"isActive": isActive,
	}
	if category != "" {
		filter["category"] = category
	}

	total, err := r.datasetsCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	cursor, err := r.datasetsCollection.Find(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var datasets []models.Dataset
	if err := cursor.All(ctx, &datasets); err != nil {
		return nil, 0, err
	}

	return datasets, total, nil
}

// FindDataset finds a dataset by ID
func (r *DatasetsRepository) FindDataset(ctx context.Context, datasetID string) (*models.Dataset, error) {
	var dataset models.Dataset
	err := r.datasetsCollection.FindOne(ctx, bson.M{
		"datasetId": datasetID,
		"isActive":  true,
	}).Decode(&dataset)
	if err != nil {
		return nil, err
	}
	return &dataset, nil
}

// CreateDataset creates a new dataset
func (r *DatasetsRepository) CreateDataset(ctx context.Context, dataset *models.Dataset) error {
	dataset.CreatedAt = time.Now()
	dataset.UpdatedAt = time.Now()

	_, err := r.datasetsCollection.InsertOne(ctx, dataset)
	return err
}

// --- Prompt Operations ---

// GetPrompts gets prompts for a dataset with pagination
func (r *DatasetsRepository) GetPrompts(ctx context.Context, datasetID, version string, limit, offset int) ([]models.DatasetPrompt, int64, error) {
	filter := bson.M{
		"datasetId": datasetID,
		"version":   version,
	}

	total, err := r.promptsCollection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.promptsCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var prompts []models.DatasetPrompt
	if err := cursor.All(ctx, &prompts); err != nil {
		return nil, 0, err
	}

	return prompts, total, nil
}

// GetAllPrompts gets all prompts for a dataset (for eval runs)
func (r *DatasetsRepository) GetAllPrompts(ctx context.Context, datasetID, version string) ([]models.DatasetPrompt, error) {
	cursor, err := r.promptsCollection.Find(ctx, bson.M{
		"datasetId": datasetID,
		"version":   version,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var prompts []models.DatasetPrompt
	if err := cursor.All(ctx, &prompts); err != nil {
		return nil, err
	}

	return prompts, nil
}

// GetSampledPrompts gets a random sample of prompts from a dataset
func (r *DatasetsRepository) GetSampledPrompts(ctx context.Context, datasetID, version string, sampleSize int) ([]models.DatasetPrompt, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"datasetId": datasetID, "version": version}},
		{"$sample": bson.M{"size": sampleSize}},
	}

	cursor, err := r.promptsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var prompts []models.DatasetPrompt
	if err := cursor.All(ctx, &prompts); err != nil {
		return nil, err
	}

	return prompts, nil
}

// BulkInsertPrompts inserts multiple prompts
func (r *DatasetsRepository) BulkInsertPrompts(ctx context.Context, prompts []models.DatasetPrompt) error {
	if len(prompts) == 0 {
		return nil
	}

	docs := make([]interface{}, len(prompts))
	for i, p := range prompts {
		docs[i] = p
	}

	_, err := r.promptsCollection.InsertMany(ctx, docs)
	return err
}

// GetDistinctCategories returns all unique categories from active shared datasets
func (r *DatasetsRepository) GetDistinctCategories(ctx context.Context, isActive bool) ([]string, error) {
	filter := bson.M{
		"isShared": true,
		"isActive": isActive,
	}

	values, err := r.datasetsCollection.Distinct(ctx, "category", filter)
	if err != nil {
		return nil, err
	}

	categories := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			categories = append(categories, s)
		}
	}
	return categories, nil
}

// GetDistinctEvalTypes returns all unique eval types from active shared datasets
func (r *DatasetsRepository) GetDistinctEvalTypes(ctx context.Context, isActive bool) ([]string, error) {
	filter := bson.M{
		"isShared": true,
		"isActive": isActive,
	}

	values, err := r.datasetsCollection.Distinct(ctx, "evalType", filter)
	if err != nil {
		return nil, err
	}

	evalTypes := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok && s != "" {
			evalTypes = append(evalTypes, s)
		}
	}
	return evalTypes, nil
}

