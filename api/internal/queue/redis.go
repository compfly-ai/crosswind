package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// EvalJobsQueue is the name of the evaluation jobs queue
	EvalJobsQueue = "eval_jobs"

	// IdempotencyKeyPrefix is the prefix for idempotency keys
	IdempotencyKeyPrefix = "idempotency:eval:"

	// IdempotencyKeyTTL is how long idempotency keys are kept (24 hours)
	// This prevents duplicate job enqueueing for the same runID
	IdempotencyKeyTTL = 24 * time.Hour
)

var (
	// ErrJobAlreadyEnqueued is returned when attempting to enqueue a duplicate job
	ErrJobAlreadyEnqueued = errors.New("job already enqueued")
)

// EvalJob represents a job in the evaluation queue
type EvalJob struct {
	RunID                  string   `json:"runId"`
	AgentID                string   `json:"agentId"`
	Mode                   string   `json:"mode"`
	EvalType               string   `json:"evalType"`
	DatasetIDs             []string `json:"datasetIds,omitempty"`
	ScenarioSetIDs         []string `json:"scenarioSetIds,omitempty"` // Generated scenario set IDs
	IncludeBuiltInDatasets bool     `json:"includeBuiltInDatasets"`   // Include mode-default datasets
}

// RedisQueue handles Redis queue operations
type RedisQueue struct {
	client *redis.Client
}

// NewRedisQueue creates a new Redis queue client
func NewRedisQueue(url string) (*RedisQueue, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisQueue{client: client}, nil
}

// Close closes the Redis connection
func (q *RedisQueue) Close() error {
	return q.client.Close()
}

// Ping checks if Redis is reachable
func (q *RedisQueue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

// EnqueueEvalJob adds an evaluation job to the queue with idempotency protection.
// If a job with the same runID has already been enqueued, returns ErrJobAlreadyEnqueued.
func (q *RedisQueue) EnqueueEvalJob(ctx context.Context, job EvalJob) error {
	// Use SETNX for idempotency - only set if key doesn't exist
	idempotencyKey := IdempotencyKeyPrefix + job.RunID
	set, err := q.client.SetNX(ctx, idempotencyKey, "1", IdempotencyKeyTTL).Result()
	if err != nil {
		return fmt.Errorf("failed to check idempotency key: %w", err)
	}
	if !set {
		// Key already exists - job was already enqueued
		return ErrJobAlreadyEnqueued
	}

	data, err := json.Marshal(job)
	if err != nil {
		// Clean up idempotency key on marshal failure
		q.client.Del(ctx, idempotencyKey)
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Use LPUSH to add to the left of the list (FIFO with BRPOP)
	if err := q.client.LPush(ctx, EvalJobsQueue, data).Err(); err != nil {
		// Clean up idempotency key on enqueue failure
		q.client.Del(ctx, idempotencyKey)
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// DequeueEvalJob retrieves and removes an evaluation job from the queue
// This blocks until a job is available or the context is cancelled
func (q *RedisQueue) DequeueEvalJob(ctx context.Context) (*EvalJob, error) {
	// BRPOP blocks until an element is available
	result, err := q.client.BRPop(ctx, 0, EvalJobsQueue).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to dequeue job: %w", err)
	}

	// result[0] is the queue name, result[1] is the value
	if len(result) < 2 {
		return nil, fmt.Errorf("unexpected result format from BRPOP")
	}

	var job EvalJob
	if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// GetQueueLength returns the number of jobs in the queue
func (q *RedisQueue) GetQueueLength(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, EvalJobsQueue).Result()
}

// SetProgress sets the progress for an evaluation run
func (q *RedisQueue) SetProgress(ctx context.Context, runID string, progress map[string]interface{}) error {
	key := fmt.Sprintf("progress:%s", runID)
	return q.client.HSet(ctx, key, progress).Err()
}

// GetProgress gets the progress for an evaluation run
func (q *RedisQueue) GetProgress(ctx context.Context, runID string) (map[string]string, error) {
	key := fmt.Sprintf("progress:%s", runID)
	return q.client.HGetAll(ctx, key).Result()
}

// SetRateLimitTokens sets the token count for rate limiting per agent
func (q *RedisQueue) SetRateLimitTokens(ctx context.Context, agentID string, tokens float64) error {
	key := fmt.Sprintf("ratelimit:%s:tokens", agentID)
	return q.client.Set(ctx, key, tokens, 0).Err()
}

// GetRateLimitTokens gets the token count for rate limiting per agent
func (q *RedisQueue) GetRateLimitTokens(ctx context.Context, agentID string) (float64, error) {
	key := fmt.Sprintf("ratelimit:%s:tokens", agentID)
	return q.client.Get(ctx, key).Float64()
}

// Client returns the underlying Redis client for custom operations
func (q *RedisQueue) Client() *redis.Client {
	return q.client
}
