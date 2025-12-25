package mongo

import (
	"context"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AgentsRepository handles agent data operations
type AgentsRepository struct {
	collection *mongo.Collection
}

// NewAgentsRepository creates a new agents repository
func NewAgentsRepository(db *mongo.Database) *AgentsRepository {
	coll := db.Collection("agents")

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "industry", Value: 1}},
		},
	}

	// Ignore index creation errors (indexes may already exist)
	_, _ = coll.Indexes().CreateMany(ctx, indexes)

	return &AgentsRepository{collection: coll}
}

// Create creates a new agent
func (r *AgentsRepository) Create(ctx context.Context, agent *models.Agent) error {
	agent.CreatedAt = time.Now()
	agent.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, agent)
	return err
}

// FindByID finds an agent by agent ID
func (r *AgentsRepository) FindByID(ctx context.Context, agentID string) (*models.Agent, error) {
	var agent models.Agent
	err := r.collection.FindOne(ctx, bson.M{
		"agentId": agentID,
	}).Decode(&agent)
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// List lists all agents
func (r *AgentsRepository) List(ctx context.Context, status string, limit, offset int) ([]models.Agent, int64, error) {
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	} else {
		// By default, exclude deleted agents
		filter["status"] = bson.M{"$ne": models.AgentStatusDeleted}
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

	var agents []models.Agent
	if err := cursor.All(ctx, &agents); err != nil {
		return nil, 0, err
	}

	return agents, total, nil
}

// Update updates an agent
func (r *AgentsRepository) Update(ctx context.Context, agentID string, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"agentId": agentID},
		bson.M{"$set": update},
	)
	return err
}

// Delete soft deletes an agent by setting status to deleted
func (r *AgentsRepository) Delete(ctx context.Context, agentID string) error {
	return r.Update(ctx, agentID, bson.M{
		"status": models.AgentStatusDeleted,
	})
}

// Exists checks if an active (non-deleted) agent exists
func (r *AgentsRepository) Exists(ctx context.Context, agentID string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"agentId": agentID,
		"status":  bson.M{"$ne": models.AgentStatusDeleted},
	})
	return count > 0, err
}

// HardDelete permanently removes an agent document (used to clean up deleted agents before re-creation)
func (r *AgentsRepository) HardDelete(ctx context.Context, agentID string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{
		"agentId": agentID,
		"status":  models.AgentStatusDeleted,
	})
	return err
}
