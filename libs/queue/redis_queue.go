package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	ErrQueueClosed  = errors.New("redis task queue is closed")
	ErrTaskNotFound = errors.New("task not found in redis")
	ErrQueueFull    = errors.New("redis task queue is full")
)

// RedisTaskQueue implements task queue backed by Redis
type RedisTaskQueue struct {
	client    *redis.Client
	logger    *zap.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	closed    bool
	maxSize   int

	// Redis key prefixes
	queueKey   string // Sorted set for priority queue
	tasksKey   string // Hash for task storage
	pubSubChan string // Pub/Sub channel for notifications
}

// RedisQueueConfig configures Redis task queue
type RedisQueueConfig struct {
	RedisAddr     string // Redis server address (localhost:6379)
	RedisPassword string // Redis password
	RedisDB       int    // Redis database number
	QueueKey      string // Redis key for queue (default: "zerostate:queue")
	TasksKey      string // Redis key for tasks (default: "zerostate:tasks")
	PubSubChannel string // Pub/Sub channel (default: "zerostate:notify")
	MaxSize       int    // Maximum queue size (0 = unlimited)
}

// DefaultRedisQueueConfig returns default configuration
func DefaultRedisQueueConfig() *RedisQueueConfig {
	return &RedisQueueConfig{
		RedisAddr:     "localhost:6379",
		RedisPassword: "",
		RedisDB:       0,
		QueueKey:      "zerostate:queue",
		TasksKey:      "zerostate:tasks",
		PubSubChannel: "zerostate:notify",
		MaxSize:       10000,
	}
}

// NewRedisTaskQueue creates a new Redis-backed task queue
func NewRedisTaskQueue(ctx context.Context, config *RedisQueueConfig, logger *zap.Logger) (*RedisTaskQueue, error) {
	if config == nil {
		config = DefaultRedisQueueConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("connected to Redis",
		zap.String("addr", config.RedisAddr),
		zap.Int("db", config.RedisDB),
	)

	queueCtx, cancel := context.WithCancel(ctx)

	return &RedisTaskQueue{
		client:     client,
		logger:     logger,
		ctx:        queueCtx,
		cancel:     cancel,
		closed:     false,
		maxSize:    config.MaxSize,
		queueKey:   config.QueueKey,
		tasksKey:   config.TasksKey,
		pubSubChan: config.PubSubChannel,
	}, nil
}

// Enqueue adds a task to the Redis queue
func (rq *RedisTaskQueue) Enqueue(task *orchestration.Task) error {
	if rq.closed {
		return ErrQueueClosed
	}

	// Check queue size
	if rq.maxSize > 0 {
		size, err := rq.client.ZCard(rq.ctx, rq.queueKey).Result()
		if err != nil {
			return fmt.Errorf("failed to get queue size: %w", err)
		}
		if int(size) >= rq.maxSize {
			return ErrQueueFull
		}
	}

	// Serialize task
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	// Use Redis pipeline for atomic operations
	pipe := rq.client.Pipeline()

	// Add to sorted set (score = priority + timestamp for FIFO within priority)
	// Higher priority = higher score
	score := float64(task.Priority)*1000000.0 + float64(task.CreatedAt.Unix())
	pipe.ZAdd(rq.ctx, rq.queueKey, redis.Z{
		Score:  score,
		Member: task.ID,
	})

	// Store task data in hash
	pipe.HSet(rq.ctx, rq.tasksKey, task.ID, taskJSON)

	// Execute pipeline
	if _, err := pipe.Exec(rq.ctx); err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	// Update status
	task.UpdateStatus(orchestration.TaskStatusQueued)

	// Notify consumers via Pub/Sub
	if err := rq.client.Publish(rq.ctx, rq.pubSubChan, task.ID).Err(); err != nil {
		rq.logger.Warn("failed to publish notification",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
	}

	queueSize, _ := rq.client.ZCard(rq.ctx, rq.queueKey).Result()
	rq.logger.Info("task enqueued to Redis",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", int(task.Priority)),
		zap.Int64("queue_size", queueSize),
	)

	return nil
}

// Dequeue removes and returns the highest priority task from Redis
func (rq *RedisTaskQueue) Dequeue() (*orchestration.Task, error) {
	if rq.closed {
		return nil, ErrQueueClosed
	}

	// Use ZPOPMAX to get highest score (highest priority)
	// Returns empty array if queue is empty
	results, err := rq.client.ZPopMax(rq.ctx, rq.queueKey, 1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue task: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // Queue empty
	}

	taskID := results[0].Member.(string)

	// Get task data from hash
	taskJSON, err := rq.client.HGet(rq.ctx, rq.tasksKey, taskID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task data: %w", err)
	}

	// Deserialize task
	var task orchestration.Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	queueSize, _ := rq.client.ZCard(rq.ctx, rq.queueKey).Result()
	rq.logger.Info("task dequeued from Redis",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", int(task.Priority)),
		zap.Int64("queue_size", queueSize),
	)

	return &task, nil
}

// DequeueWait waits for a task to be available, blocking until one is ready
func (rq *RedisTaskQueue) DequeueWait(ctx context.Context) (*orchestration.Task, error) {
	// Subscribe to notifications
	pubsub := rq.client.Subscribe(rq.ctx, rq.pubSubChan)
	defer pubsub.Close()

	// First try to dequeue immediately
	task, err := rq.Dequeue()
	if err != nil {
		return nil, err
	}
	if task != nil {
		return task, nil
	}

	// Wait for notification
	ch := pubsub.Channel()

	for {
		select {
		case <-ch:
			// New task notification received, try to dequeue
			task, err := rq.Dequeue()
			if err != nil {
				return nil, err
			}
			if task != nil {
				return task, nil
			}
			// Task was consumed by another worker, continue waiting
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-rq.ctx.Done():
			return nil, ErrQueueClosed
		}
	}
}

// Get retrieves a task by ID from Redis
func (rq *RedisTaskQueue) Get(taskID string) (*orchestration.Task, error) {
	taskJSON, err := rq.client.HGet(rq.ctx, rq.tasksKey, taskID).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	var task orchestration.Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// Update updates a task's information in Redis
func (rq *RedisTaskQueue) Update(task *orchestration.Task) error {
	// Check if task exists
	exists, err := rq.client.HExists(rq.ctx, rq.tasksKey, task.ID).Result()
	if err != nil {
		return fmt.Errorf("failed to check task existence: %w", err)
	}
	if !exists {
		return ErrTaskNotFound
	}

	// Update timestamp
	task.UpdatedAt = time.Now()

	// Serialize and store
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	if err := rq.client.HSet(rq.ctx, rq.tasksKey, task.ID, taskJSON).Err(); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// Cancel removes a task from the Redis queue
func (rq *RedisTaskQueue) Cancel(taskID string) error {
	// Get task first
	task, err := rq.Get(taskID)
	if err != nil {
		return err
	}

	// Mark as canceled
	task.UpdateStatus(orchestration.TaskStatusCanceled)

	// Use pipeline for atomic operations
	pipe := rq.client.Pipeline()

	// Remove from queue (if still queued)
	pipe.ZRem(rq.ctx, rq.queueKey, taskID)

	// Update task data
	taskJSON, _ := json.Marshal(task)
	pipe.HSet(rq.ctx, rq.tasksKey, taskID, taskJSON)

	if _, err := pipe.Exec(rq.ctx); err != nil {
		return fmt.Errorf("failed to cancel task: %w", err)
	}

	rq.logger.Info("task canceled in Redis",
		zap.String("task_id", taskID),
	)

	return nil
}

// List returns tasks matching the filter from Redis
func (rq *RedisTaskQueue) List(filter *orchestration.TaskFilter) ([]*orchestration.Task, error) {
	// Get all task IDs from hash
	taskIDs, err := rq.client.HKeys(rq.ctx, rq.tasksKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get task IDs: %w", err)
	}

	// Retrieve all tasks
	var tasks []*orchestration.Task
	for _, taskID := range taskIDs {
		task, err := rq.Get(taskID)
		if err != nil {
			continue // Skip tasks that can't be retrieved
		}

		if rq.matchesFilter(task, filter) {
			tasks = append(tasks, task)
		}
	}

	// Apply offset and limit
	if filter.Offset > 0 {
		if filter.Offset >= len(tasks) {
			return []*orchestration.Task{}, nil
		}
		tasks = tasks[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(tasks) {
		tasks = tasks[:filter.Limit]
	}

	return tasks, nil
}

// matchesFilter checks if a task matches the filter criteria
func (rq *RedisTaskQueue) matchesFilter(task *orchestration.Task, filter *orchestration.TaskFilter) bool {
	if filter == nil {
		return true
	}

	if filter.UserID != "" && task.UserID != filter.UserID {
		return false
	}

	if filter.Status != "" && task.Status != filter.Status {
		return false
	}

	if filter.Priority != 0 && task.Priority != filter.Priority {
		return false
	}

	if filter.Type != "" && task.Type != filter.Type {
		return false
	}

	if filter.AssignedTo != "" && task.AssignedTo != filter.AssignedTo {
		return false
	}

	if filter.CreatedAfter != nil && task.CreatedAt.Before(*filter.CreatedAfter) {
		return false
	}

	if filter.CreatedBefore != nil && task.CreatedAt.After(*filter.CreatedBefore) {
		return false
	}

	return true
}

// Size returns the number of queued tasks in Redis
func (rq *RedisTaskQueue) Size() int {
	size, err := rq.client.ZCard(rq.ctx, rq.queueKey).Result()
	if err != nil {
		rq.logger.Error("failed to get queue size", zap.Error(err))
		return 0
	}
	return int(size)
}

// TotalTasks returns the total number of tasks in Redis
func (rq *RedisTaskQueue) TotalTasks() int {
	total, err := rq.client.HLen(rq.ctx, rq.tasksKey).Result()
	if err != nil {
		rq.logger.Error("failed to get total tasks", zap.Error(err))
		return 0
	}
	return int(total)
}

// Close shuts down the Redis task queue
func (rq *RedisTaskQueue) Close() error {
	if rq.closed {
		return nil
	}

	rq.closed = true
	rq.cancel()

	remainingTasks := rq.TotalTasks()

	if err := rq.client.Close(); err != nil {
		rq.logger.Error("error closing Redis client", zap.Error(err))
		return err
	}

	rq.logger.Info("Redis task queue closed",
		zap.Int("remaining_tasks", remainingTasks),
	)

	return nil
}

// ClearAll removes all tasks from Redis (for testing/maintenance)
func (rq *RedisTaskQueue) ClearAll() error {
	pipe := rq.client.Pipeline()
	pipe.Del(rq.ctx, rq.queueKey)
	pipe.Del(rq.ctx, rq.tasksKey)

	if _, err := pipe.Exec(rq.ctx); err != nil {
		return fmt.Errorf("failed to clear Redis queue: %w", err)
	}

	rq.logger.Info("Redis queue cleared")
	return nil
}
