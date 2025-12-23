package mongo

import (
	"context"
	"time"

	"github.com/compfly-ai/crosswind/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ScenariosRepository handles scenario set database operations
type ScenariosRepository struct {
	collection *mongo.Collection
}

// NewScenariosRepository creates a new scenarios repository
func NewScenariosRepository(db *mongo.Database) *ScenariosRepository {
	return &ScenariosRepository{
		collection: db.Collection("scenarioSets"),
	}
}

// Create creates a new scenario set
func (r *ScenariosRepository) Create(ctx context.Context, set *models.ScenarioSet) error {
	now := time.Now()
	set.CreatedAt = now
	set.UpdatedAt = now

	_, err := r.collection.InsertOne(ctx, set)
	return err
}

// FindBySetID finds a scenario set by ID
func (r *ScenariosRepository) FindBySetID(ctx context.Context, setID string) (*models.ScenarioSet, error) {
	var set models.ScenarioSet
	err := r.collection.FindOne(ctx, bson.M{"setId": setID}).Decode(&set)
	if err != nil {
		return nil, err
	}
	return &set, nil
}

// ListByAgent lists scenario sets for an agent
func (r *ScenariosRepository) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]models.ScenarioSet, int64, error) {
	filter := bson.M{
		"agentId": agentID,
	}

	// Count total
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// Find with pagination
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var sets []models.ScenarioSet
	if err := cursor.All(ctx, &sets); err != nil {
		return nil, 0, err
	}

	return sets, total, nil
}

// Update updates a scenario set
func (r *ScenariosRepository) Update(ctx context.Context, setID string, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"setId": setID},
		bson.M{"$set": update},
	)
	return err
}

// UpdateStatus updates the status of a scenario set
func (r *ScenariosRepository) UpdateStatus(ctx context.Context, setID, status string) error {
	return r.Update(ctx, setID, bson.M{"status": status})
}

// UpdateStatusWithError updates the status and stores an error message
func (r *ScenariosRepository) UpdateStatusWithError(ctx context.Context, setID, status, errorMsg string) error {
	return r.Update(ctx, setID, bson.M{"status": status, "error": errorMsg})
}

// UpdateProgress updates generation progress for live tracking
// Uses dot notation to avoid overwriting other progress fields (stage, message, plan)
func (r *ScenariosRepository) UpdateProgress(ctx context.Context, setID string, generated, total int) error {
	return r.Update(ctx, setID, bson.M{
		"progress.generated":   generated,
		"progress.total":       total,
		"progress.lastUpdated": time.Now(),
	})
}

// UpdateStage updates the generation stage and message for streaming progress
func (r *ScenariosRepository) UpdateStage(ctx context.Context, setID, stage, message string) error {
	return r.Update(ctx, setID, bson.M{
		"progress.stage":       stage,
		"progress.message":     message,
		"progress.lastUpdated": time.Now(),
	})
}

// AppendScenario appends a single scenario to the set (for streaming updates)
func (r *ScenariosRepository) AppendScenario(ctx context.Context, setID string, scenario models.Scenario) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"setId": setID},
		bson.M{
			"$push": bson.M{"scenarios": scenario},
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		},
	)
	return err
}

// UpdateScenarios updates the scenarios and summary
func (r *ScenariosRepository) UpdateScenarios(ctx context.Context, setID string, scenarios []models.Scenario, summary models.ScenarioSummary) error {
	return r.Update(ctx, setID, bson.M{
		"scenarios": scenarios,
		"summary":   summary,
		"status":    models.ScenarioStatusReady,
	})
}

// UpdateScenarioEnabled updates the enabled status of a specific scenario
func (r *ScenariosRepository) UpdateScenarioEnabled(ctx context.Context, setID, scenarioID string, enabled bool) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"setId":        setID,
			"scenarios.id": scenarioID,
		},
		bson.M{
			"$set": bson.M{
				"scenarios.$.enabled": enabled,
				"updatedAt":           time.Now(),
			},
		},
	)
	return err
}

// AddScenarios adds new scenarios to an existing set
func (r *ScenariosRepository) AddScenarios(ctx context.Context, setID string, scenarios []models.Scenario) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"setId": setID},
		bson.M{
			"$push": bson.M{
				"scenarios": bson.M{"$each": scenarios},
			},
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		},
	)
	return err
}

// RemoveScenario removes a scenario from a set
func (r *ScenariosRepository) RemoveScenario(ctx context.Context, setID, scenarioID string) error {
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"setId": setID},
		bson.M{
			"$pull": bson.M{
				"scenarios": bson.M{"id": scenarioID},
			},
			"$set": bson.M{
				"updatedAt": time.Now(),
			},
		},
	)
	return err
}

// UpdateScenario updates specific fields of a scenario within a set
func (r *ScenariosRepository) UpdateScenario(ctx context.Context, setID, scenarioID string, update bson.M) error {
	// Build the update document with scenarios.$ prefix for each field
	setFields := bson.M{
		"updatedAt": time.Now(),
	}
	for key, value := range update {
		setFields["scenarios.$."+key] = value
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"setId":        setID,
			"scenarios.id": scenarioID,
		},
		bson.M{
			"$set": setFields,
		},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// UpdateSummary updates only the summary of a scenario set
func (r *ScenariosRepository) UpdateSummary(ctx context.Context, setID string, summary models.ScenarioSummary) error {
	return r.Update(ctx, setID, bson.M{"summary": summary})
}

// UpdatePlan stores the generation plan in progress
func (r *ScenariosRepository) UpdatePlan(ctx context.Context, setID string, plan *models.GenerationPlan) error {
	return r.Update(ctx, setID, bson.M{
		"progress.plan":  plan,
		"progress.total": plan.RecommendedCount,
	})
}

// UpdateBatches updates the batch status in the plan
func (r *ScenariosRepository) UpdateBatches(ctx context.Context, setID string, batches []models.GenerationBatch) error {
	return r.Update(ctx, setID, bson.M{"progress.plan.batches": batches})
}

// Delete deletes a scenario set
func (r *ScenariosRepository) Delete(ctx context.Context, setID string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{"setId": setID})
	return err
}
