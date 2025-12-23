package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NewClient creates a new MongoDB client
func NewClient(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(30 * time.Second)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return client, nil
}

// Repositories holds all repository instances
type Repositories struct {
	Agents    *AgentsRepository
	EvalRuns  *EvalRunsRepository
	Results   *ResultsRepository
	Datasets  *DatasetsRepository
	Scenarios *ScenariosRepository
	Contexts  *ContextsRepository
}

// NewRepositories creates all repository instances
func NewRepositories(client *mongo.Client, dbName string) *Repositories {
	db := client.Database(dbName)
	return &Repositories{
		Agents:    NewAgentsRepository(db),
		EvalRuns:  NewEvalRunsRepository(db),
		Results:   NewResultsRepository(db),
		Datasets:  NewDatasetsRepository(db),
		Scenarios: NewScenariosRepository(db),
		Contexts:  NewContextsRepository(db),
	}
}
