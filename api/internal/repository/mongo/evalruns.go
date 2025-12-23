package mongo

import (
	"context"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EvalRunsRepository handles evaluation run data operations
type EvalRunsRepository struct {
	collection *mongo.Collection
}

// NewEvalRunsRepository creates a new eval runs repository
func NewEvalRunsRepository(db *mongo.Database) *EvalRunsRepository {
	coll := db.Collection("evalRuns")

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
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: 1}},
		},
	}

	coll.Indexes().CreateMany(ctx, indexes)

	return &EvalRunsRepository{collection: coll}
}

// Create creates a new evaluation run
func (r *EvalRunsRepository) Create(ctx context.Context, run *models.EvalRun) error {
	run.CreatedAt = time.Now()
	run.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, run)
	return err
}

// FindByRunID finds an evaluation run by its run ID
func (r *EvalRunsRepository) FindByRunID(ctx context.Context, runID string) (*models.EvalRun, error) {
	var run models.EvalRun
	err := r.collection.FindOne(ctx, bson.M{"runId": runID}).Decode(&run)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// ListByAgent lists evaluation runs for an agent
func (r *EvalRunsRepository) ListByAgent(ctx context.Context, agentID string, status string, limit, offset int) ([]models.EvalRun, int64, error) {
	filter := bson.M{
		"agentId": agentID,
	}
	if status != "" {
		filter["status"] = status
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

	var runs []models.EvalRun
	if err := cursor.All(ctx, &runs); err != nil {
		return nil, 0, err
	}

	return runs, total, nil
}

// Update updates an evaluation run
func (r *EvalRunsRepository) Update(ctx context.Context, runID string, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"runId": runID},
		bson.M{"$set": update},
	)
	return err
}

// UpdateProgress updates the progress of an evaluation run
func (r *EvalRunsRepository) UpdateProgress(ctx context.Context, runID string, progress models.EvalProgress) error {
	progress.LastUpdated = time.Now()
	return r.Update(ctx, runID, bson.M{
		"progress": progress,
	})
}

// UpdateStatus updates the status of an evaluation run
func (r *EvalRunsRepository) UpdateStatus(ctx context.Context, runID, status string) error {
	update := bson.M{"status": status}

	if status == models.EvalStatusRunning {
		now := time.Now()
		update["startedAt"] = now
	} else if status == models.EvalStatusCompleted || status == models.EvalStatusFailed || status == models.EvalStatusCancelled {
		now := time.Now()
		update["completedAt"] = now
	}

	return r.Update(ctx, runID, update)
}

// HasActiveRun checks if an agent has an active (pending or running) evaluation run
func (r *EvalRunsRepository) HasActiveRun(ctx context.Context, agentID string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"agentId": agentID,
		"status":  bson.M{"$in": []string{models.EvalStatusPending, models.EvalStatusRunning}},
	})
	return count > 0, err
}

// GetLatestRun gets the most recent evaluation run for an agent
func (r *EvalRunsRepository) GetLatestRun(ctx context.Context, agentID string) (*models.EvalRun, error) {
	var run models.EvalRun
	opts := options.FindOne().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	err := r.collection.FindOne(ctx, bson.M{
		"agentId": agentID,
	}, opts).Decode(&run)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetLatestRunsByAgentIDs gets the most recent evaluation run for multiple agents in a single query.
// Uses MongoDB aggregation to efficiently retrieve the latest run for each agent.
// Returns a map of agentID -> EvalRun. Agents with no runs are not included in the map.
func (r *EvalRunsRepository) GetLatestRunsByAgentIDs(ctx context.Context, agentIDs []string) (map[string]*models.EvalRun, error) {
	if len(agentIDs) == 0 {
		return make(map[string]*models.EvalRun), nil
	}

	// Use aggregation pipeline to get the latest run for each agent
	pipeline := mongo.Pipeline{
		// Match only the agents we're interested in
		{{Key: "$match", Value: bson.M{
			"agentId": bson.M{"$in": agentIDs},
		}}},
		// Sort by agentId and createdAt descending
		{{Key: "$sort", Value: bson.D{
			{Key: "agentId", Value: 1},
			{Key: "createdAt", Value: -1},
		}}},
		// Group by agentId, taking the first (most recent) document
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$agentId"},
			{Key: "doc", Value: bson.M{"$first": "$$ROOT"}},
		}}},
		// Replace root with the actual document
		{{Key: "$replaceRoot", Value: bson.M{"newRoot": "$doc"}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	result := make(map[string]*models.EvalRun, len(agentIDs))
	for cursor.Next(ctx) {
		var run models.EvalRun
		if err := cursor.Decode(&run); err != nil {
			return nil, err
		}
		result[run.AgentID] = &run
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
