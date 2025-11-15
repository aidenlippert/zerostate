package orchestration

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	ErrQueueClosed  = errors.New("task queue is closed")
	ErrTaskNotFound = errors.New("task not found")
	ErrQueueFull    = errors.New("task queue is full")
)

// TaskQueue manages pending tasks with priority-based scheduling
type TaskQueue struct {
	// Priority queue
	queue   *priorityQueue
	queueMu sync.RWMutex

	// Task storage
	tasks   map[string]*Task
	tasksMu sync.RWMutex

	// Configuration
	maxSize int
	logger  *zap.Logger

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	closed  bool
	closeMu sync.RWMutex

	// Notifications
	notifyCh chan struct{}
}

// NewTaskQueue creates a new task queue
func NewTaskQueue(ctx context.Context, maxSize int, logger *zap.Logger) *TaskQueue {
	if logger == nil {
		logger = zap.NewNop()
	}

	queueCtx, cancel := context.WithCancel(ctx)

	pq := &priorityQueue{}
	heap.Init(pq)

	return &TaskQueue{
		queue:    pq,
		tasks:    make(map[string]*Task),
		maxSize:  maxSize,
		logger:   logger,
		ctx:      queueCtx,
		cancel:   cancel,
		closed:   false,
		notifyCh: make(chan struct{}, 100), // Buffered channel
	}
}

// Enqueue adds a task to the queue
func (tq *TaskQueue) Enqueue(task *Task) error {
	tq.closeMu.RLock()
	if tq.closed {
		tq.closeMu.RUnlock()
		return ErrQueueClosed
	}
	tq.closeMu.RUnlock()

	tq.queueMu.Lock()
	if tq.maxSize > 0 && tq.queue.Len() >= tq.maxSize {
		tq.queueMu.Unlock()
		return ErrQueueFull
	}

	// Add to priority queue
	item := &queueItem{
		task:     task,
		priority: int(task.Priority),
		index:    -1,
	}
	heap.Push(tq.queue, item)
	queueSize := tq.queue.Len() // Get size while holding lock
	tq.queueMu.Unlock()

	// Store task
	tq.tasksMu.Lock()
	tq.tasks[task.ID] = task
	tq.tasksMu.Unlock()

	// Update status
	task.UpdateStatus(TaskStatusQueued)

	tq.logger.Info("task enqueued",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", int(task.Priority)),
		zap.Int("queue_size", queueSize),
	)

	// Notify waiting consumers
	select {
	case tq.notifyCh <- struct{}{}:
	default:
	}

	return nil
}

// Dequeue removes and returns the highest priority task
func (tq *TaskQueue) Dequeue() (*Task, error) {
	tq.closeMu.RLock()
	if tq.closed {
		tq.closeMu.RUnlock()
		return nil, ErrQueueClosed
	}
	tq.closeMu.RUnlock()

	tq.queueMu.Lock()
	defer tq.queueMu.Unlock()

	if tq.queue.Len() == 0 {
		return nil, nil
	}

	item := heap.Pop(tq.queue).(*queueItem)
	task := item.task

	queueSize := tq.queue.Len() // Get size while holding lock

	tq.logger.Info("task dequeued",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", int(task.Priority)),
		zap.Int("queue_size", queueSize),
	)

	return task, nil
}

// DequeueWait waits for a task to be available, blocking until one is ready
func (tq *TaskQueue) DequeueWait(ctx context.Context) (*Task, error) {
	for {
		// Try to dequeue
		task, err := tq.Dequeue()
		if err != nil {
			return nil, err
		}
		if task != nil {
			return task, nil
		}

		// Wait for notification or context cancellation
		select {
		case <-tq.notifyCh:
			// New task available, retry
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-tq.ctx.Done():
			return nil, ErrQueueClosed
		}
	}
}

// Get retrieves a task by ID
func (tq *TaskQueue) Get(taskID string) (*Task, error) {
	tq.tasksMu.RLock()
	defer tq.tasksMu.RUnlock()

	task, exists := tq.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	return task, nil
}

// Update updates a task's information
func (tq *TaskQueue) Update(task *Task) error {
	tq.tasksMu.Lock()
	defer tq.tasksMu.Unlock()

	if _, exists := tq.tasks[task.ID]; !exists {
		return ErrTaskNotFound
	}

	task.UpdatedAt = time.Now()
	tq.tasks[task.ID] = task

	return nil
}

// Cancel removes a task from the queue
func (tq *TaskQueue) Cancel(taskID string) error {
	tq.tasksMu.Lock()
	task, exists := tq.tasks[taskID]
	if !exists {
		tq.tasksMu.Unlock()
		return ErrTaskNotFound
	}

	// Mark as canceled
	task.UpdateStatus(TaskStatusCanceled)
	tq.tasksMu.Unlock()

	// Remove from queue if still queued
	tq.queueMu.Lock()
	defer tq.queueMu.Unlock()

	for i := 0; i < tq.queue.Len(); i++ {
		item := (*tq.queue)[i]
		if item.task.ID == taskID {
			heap.Remove(tq.queue, i)
			tq.logger.Info("task canceled and removed from queue",
				zap.String("task_id", taskID),
			)
			break
		}
	}

	return nil
}

// List returns tasks matching the filter
func (tq *TaskQueue) List(filter *TaskFilter) ([]*Task, error) {
	tq.tasksMu.RLock()
	defer tq.tasksMu.RUnlock()

	var result []*Task
	for _, task := range tq.tasks {
		if tq.matchesFilter(task, filter) {
			result = append(result, task)
		}
	}

	// Apply limit and offset
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []*Task{}, nil
		}
		result = result[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

// matchesFilter checks if a task matches the filter criteria
func (tq *TaskQueue) matchesFilter(task *Task, filter *TaskFilter) bool {
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

// Size returns the number of queued tasks
func (tq *TaskQueue) Size() int {
	tq.queueMu.RLock()
	defer tq.queueMu.RUnlock()
	return tq.queue.Len()
}

// TotalTasks returns the total number of tasks (queued + processing + completed)
func (tq *TaskQueue) TotalTasks() int {
	tq.tasksMu.RLock()
	defer tq.tasksMu.RUnlock()
	return len(tq.tasks)
}

// Close shuts down the task queue
func (tq *TaskQueue) Close() error {
	tq.closeMu.Lock()
	defer tq.closeMu.Unlock()

	if tq.closed {
		return nil
	}

	tq.closed = true
	tq.cancel()
	close(tq.notifyCh)

	tq.logger.Info("task queue closed",
		zap.Int("remaining_tasks", tq.TotalTasks()),
	)

	return nil
}

// Priority queue implementation using container/heap

type queueItem struct {
	task     *Task
	priority int // Higher value = higher priority
	index    int // Index in heap
}

type priorityQueue []*queueItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Higher priority comes first
	if pq[i].priority != pq[j].priority {
		return pq[i].priority > pq[j].priority
	}
	// If priority is equal, older tasks come first (FIFO)
	return pq[i].task.CreatedAt.Before(pq[j].task.CreatedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*queueItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}
