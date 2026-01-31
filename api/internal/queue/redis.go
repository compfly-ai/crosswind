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
	evalJobsPrefix = "eval_jobs:"
	evalAgentsSet  = "eval_agents"

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

// EnqueueEvalJob adds an evaluation job to the agent's per-agent queue.
// Uses idempotency protection and a pipeline to atomically LPUSH + SADD.
func (q *RedisQueue) EnqueueEvalJob(ctx context.Context, job EvalJob) error {
	// Use SETNX for idempotency - only set if key doesn't exist
	idempotencyKey := IdempotencyKeyPrefix + job.RunID
	set, err := q.client.SetNX(ctx, idempotencyKey, "1", IdempotencyKeyTTL).Result()
	if err != nil {
		return fmt.Errorf("failed to check idempotency key: %w", err)
	}
	if !set {
		return ErrJobAlreadyEnqueued
	}

	data, err := json.Marshal(job)
	if err != nil {
		q.client.Del(ctx, idempotencyKey)
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	queueKey := evalJobsPrefix + job.AgentID
	pipe := q.client.Pipeline()
	pipe.LPush(ctx, queueKey, data)
	pipe.SAdd(ctx, evalAgentsSet, job.AgentID)
	if _, err := pipe.Exec(ctx); err != nil {
		q.client.Del(ctx, idempotencyKey)
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// GetAgentQueueLength returns the number of pending jobs for a specific agent.
func (q *RedisQueue) GetAgentQueueLength(ctx context.Context, agentID string) (int64, error) {
	return q.client.LLen(ctx, evalJobsPrefix+agentID).Result()
}

// GetActiveAgents returns all agent IDs that have pending eval jobs.
func (q *RedisQueue) GetActiveAgents(ctx context.Context) ([]string, error) {
	return q.client.SMembers(ctx, evalAgentsSet).Result()
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
