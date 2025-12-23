package mongo

import (
	"context"
	"time"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ContextsRepository handles context document operations
type ContextsRepository struct {
	collection *mongo.Collection
}

// NewContextsRepository creates a new contexts repository
func NewContextsRepository(db *mongo.Database) *ContextsRepository {
	coll := db.Collection("contexts")

	// Create indexes
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "contextId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "createdAt", Value: -1}},
		},
		{
			// TTL index for automatic expiration (if expiresAt is set)
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
	}

	coll.Indexes().CreateMany(ctx, indexes)

	return &ContextsRepository{collection: coll}
}

// Create creates a new context
func (r *ContextsRepository) Create(ctx context.Context, ctxDoc *models.Context) error {
	ctxDoc.CreatedAt = time.Now()
	ctxDoc.UpdatedAt = time.Now()

	_, err := r.collection.InsertOne(ctx, ctxDoc)
	return err
}

// FindByID finds a context by context ID
func (r *ContextsRepository) FindByID(ctx context.Context, contextID string) (*models.Context, error) {
	var ctxDoc models.Context
	err := r.collection.FindOne(ctx, bson.M{
		"contextId": contextID,
	}).Decode(&ctxDoc)
	if err != nil {
		return nil, err
	}
	return &ctxDoc, nil
}

// List lists all contexts
func (r *ContextsRepository) List(ctx context.Context, limit, offset int) ([]models.Context, int64, error) {
	filter := bson.M{}

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

	var contexts []models.Context
	if err := cursor.All(ctx, &contexts); err != nil {
		return nil, 0, err
	}

	return contexts, total, nil
}

// Update updates a context
func (r *ContextsRepository) Update(ctx context.Context, contextID string, update bson.M) error {
	update["updatedAt"] = time.Now()
	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"contextId": contextID},
		bson.M{"$set": update},
	)
	return err
}

// UpdateStatus updates a context status and optionally error message
func (r *ContextsRepository) UpdateStatus(ctx context.Context, contextID, status, errorMsg string) error {
	update := bson.M{
		"status":    status,
		"updatedAt": time.Now(),
	}
	if errorMsg != "" {
		update["error"] = errorMsg
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"contextId": contextID},
		bson.M{"$set": update},
	)
	return err
}

// UpdateFileStatus updates a specific file's status within a context
func (r *ContextsRepository) UpdateFileStatus(ctx context.Context, contextID, fileName, status string, metadata map[string]interface{}) error {
	update := bson.M{
		"updatedAt":           time.Now(),
		"files.$.status":      status,
	}

	// Add optional metadata fields
	if text, ok := metadata["extractedText"]; ok {
		update["files.$.extractedText"] = text
	}
	if chars, ok := metadata["extractedChars"]; ok {
		update["files.$.extractedChars"] = chars
	}
	if pages, ok := metadata["pageCount"]; ok {
		update["files.$.pageCount"] = pages
	}
	if rows, ok := metadata["rowCount"]; ok {
		update["files.$.rowCount"] = rows
	}
	if errMsg, ok := metadata["error"]; ok {
		update["files.$.error"] = errMsg
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{
			"contextId":  contextID,
			"files.name": fileName,
		},
		bson.M{"$set": update},
	)
	return err
}

// UpdateSummary updates the context summary
func (r *ContextsRepository) UpdateSummary(ctx context.Context, contextID string, summary *models.ContextSummary) error {
	update := bson.M{
		"summary":   summary,
		"updatedAt": time.Now(),
	}

	_, err := r.collection.UpdateOne(
		ctx,
		bson.M{"contextId": contextID},
		bson.M{"$set": update},
	)
	return err
}

// Delete permanently removes a context
func (r *ContextsRepository) Delete(ctx context.Context, contextID string) error {
	_, err := r.collection.DeleteOne(ctx, bson.M{
		"contextId": contextID,
	})
	return err
}

// Exists checks if a context exists
func (r *ContextsRepository) Exists(ctx context.Context, contextID string) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"contextId": contextID,
	})
	return count > 0, err
}

// FindByIDs finds multiple contexts by their IDs
func (r *ContextsRepository) FindByIDs(ctx context.Context, contextIDs []string) ([]models.Context, error) {
	filter := bson.M{
		"contextId": bson.M{"$in": contextIDs},
		"status":    models.ContextStatusReady, // Only return ready contexts
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var contexts []models.Context
	if err := cursor.All(ctx, &contexts); err != nil {
		return nil, err
	}

	return contexts, nil
}

// AddFiles appends new files to an existing context
func (r *ContextsRepository) AddFiles(ctx context.Context, contextID string, files []models.ContextFile) error {
	update := bson.M{
		"$push": bson.M{
			"files": bson.M{"$each": files},
		},
		"$set": bson.M{
			"status":    models.ContextStatusProcessing,
			"updatedAt": time.Now(),
		},
	}

	result, err := r.collection.UpdateOne(
		ctx,
		bson.M{"contextId": contextID},
		update,
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
