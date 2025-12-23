package mongo

import (
	"context"
	"time"

	"github.com/agent-eval/agent-eval/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ResultsRepository handles evaluation results data operations
type ResultsRepository struct {
	collection *mongo.Collection
}

// NewResultsRepository creates a new results repository
func NewResultsRepository(db *mongo.Database) *ResultsRepository {
	coll := db.Collection("evalResultsSummary")

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "runId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "agentId", Value: 1},
				{Key: "createdAt", Value: -1},
			},
		},
	}

	coll.Indexes().CreateMany(ctx, indexes)

	return &ResultsRepository{collection: coll}
}

// Create creates a new evaluation results summary
func (r *ResultsRepository) Create(ctx context.Context, results *models.EvalResultsSummary) error {
	results.CreatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, results)
	return err
}

// FindByRunID finds results by run ID
func (r *ResultsRepository) FindByRunID(ctx context.Context, runID string) (*models.EvalResultsSummary, error) {
	var results models.EvalResultsSummary
	err := r.collection.FindOne(ctx, bson.M{"runId": runID}).Decode(&results)
	if err != nil {
		return nil, err
	}
	return &results, nil
}

// ListByAgent lists results for an agent
func (r *ResultsRepository) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]models.EvalResultsSummary, int64, error) {
	filter := bson.M{
		"agentId": agentID,
	}

	// Get total count
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Find with pagination
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var results []models.EvalResultsSummary
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// Update updates results
func (r *ResultsRepository) Update(ctx context.Context, runID string, update bson.M) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"runId": runID},
		bson.M{"$set": update},
	)
	return err
}

// AppendFailure adds a failure to the results
func (r *ResultsRepository) AppendFailure(ctx context.Context, runID string, failure models.PromptResultDetail) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"runId": runID},
		bson.M{"$push": bson.M{"failures": failure}},
	)
	return err
}

// AppendSamplePass adds a sample pass to the results (with limit check)
func (r *ResultsRepository) AppendSamplePass(ctx context.Context, runID, category string, pass models.PromptResultDetail, maxSamplesPerCategory int) error {
	// First check if we already have enough samples for this category
	results, err := r.FindByRunID(ctx, runID)
	if err != nil {
		return err
	}

	categoryCount := 0
	for _, p := range results.SamplePasses {
		if p.Category == category {
			categoryCount++
		}
	}

	if categoryCount >= maxSamplesPerCategory {
		return nil // Skip, we have enough samples
	}

	_, err = r.collection.UpdateOne(
		ctx,
		bson.M{"runId": runID},
		bson.M{"$push": bson.M{"samplePasses": pass}},
	)
	return err
}
