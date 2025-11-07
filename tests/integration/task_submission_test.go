package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/api"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestTaskSubmissionWorkflow(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := zap.NewNop()

	// Create p2p host
	host, err := libp2p.New()
	require.NoError(t, err)
	defer host.Close()

	// Create signer
	signer, err := identity.NewSigner(logger)
	require.NoError(t, err)

	// Create HNSW index
	hnsw := search.NewHNSWIndex(16, 200)

	// Create task queue
	taskQueue := orchestration.NewTaskQueue(ctx, 100, logger)
	defer taskQueue.Close()

	// Create handlers
	handlers := api.NewHandlers(ctx, logger, host, signer, hnsw, taskQueue)

	// Create server
	config := api.DefaultConfig()
	config.Port = 0 // Random port
	server := api.NewServer(config, handlers, logger)

	// Test cases
	t.Run("SubmitTask_Success", func(t *testing.T) {
		// Create request
		reqBody := api.SubmitTaskRequest{
			Query:    "What is the capital of France?",
			Budget:   1.50,
			Timeout:  60,
			Priority: "high",
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/submit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Execute
		server.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(t, http.StatusAccepted, w.Code)

		var resp api.SubmitTaskResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		assert.NotEmpty(t, resp.TaskID)
		assert.Equal(t, "queued", resp.Status)

		// Verify task in queue
		task, err := taskQueue.Get(resp.TaskID)
		require.NoError(t, err)
		assert.Equal(t, orchestration.PriorityHigh, task.Priority)
		assert.Equal(t, 1.50, task.Budget)
		assert.Equal(t, 60*time.Second, task.Timeout)
	})

	t.Run("SubmitTask_MissingBudget", func(t *testing.T) {
		reqBody := api.SubmitTaskRequest{
			Query:   "Test query",
			Budget:  0, // Invalid
			Timeout: 30,
		}

		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/submit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetTask_Success", func(t *testing.T) {
		// Submit a task first
		task := orchestration.NewTask(
			"test-user",
			"test-task",
			[]string{"test-capability"},
			map[string]interface{}{"data": "test"},
		)
		err := taskQueue.Enqueue(task)
		require.NoError(t, err)

		// Get task
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+task.ID, nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp orchestration.Task
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, task.ID, resp.ID)
	})

	t.Run("GetTask_NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/nonexistent-id", nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("ListTasks_WithFilters", func(t *testing.T) {
		// Submit multiple tasks
		for i := 0; i < 5; i++ {
			task := orchestration.NewTask(
				"user-123",
				"test-task",
				[]string{"capability"},
				map[string]interface{}{"index": i},
			)
			err := taskQueue.Enqueue(task)
			require.NoError(t, err)
		}

		// List tasks with user filter
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks?user_id=user-123&limit=3", nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		tasks := resp["tasks"].([]interface{})
		assert.GreaterOrEqual(t, len(tasks), 3)
	})

	t.Run("CancelTask_Success", func(t *testing.T) {
		// Submit a task
		task := orchestration.NewTask(
			"test-user",
			"cancelable-task",
			[]string{"capability"},
			map[string]interface{}{},
		)
		err := taskQueue.Enqueue(task)
		require.NoError(t, err)

		// Cancel task
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+task.ID, nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify task is canceled
		canceledTask, err := taskQueue.Get(task.ID)
		require.NoError(t, err)
		assert.Equal(t, orchestration.TaskStatusCanceled, canceledTask.Status)
	})

	t.Run("GetTaskStatus_Success", func(t *testing.T) {
		// Submit a task
		task := orchestration.NewTask(
			"test-user",
			"status-task",
			[]string{"capability"},
			map[string]interface{}{},
		)
		err := taskQueue.Enqueue(task)
		require.NoError(t, err)

		// Get status
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+task.ID+"/status", nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp api.TaskStatusResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, task.ID, resp.TaskID)
		assert.Equal(t, "queued", resp.Status)
		assert.Equal(t, 10, resp.Progress)
	})

	t.Run("GetTaskResult_NotCompleted", func(t *testing.T) {
		// Submit a task
		task := orchestration.NewTask(
			"test-user",
			"incomplete-task",
			[]string{"capability"},
			map[string]interface{}{},
		)
		err := taskQueue.Enqueue(task)
		require.NoError(t, err)

		// Try to get result
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+task.ID+"/result", nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetTaskResult_Completed", func(t *testing.T) {
		// Submit and complete a task
		task := orchestration.NewTask(
			"test-user",
			"completed-task",
			[]string{"capability"},
			map[string]interface{}{},
		)
		err := taskQueue.Enqueue(task)
		require.NoError(t, err)

		// Manually complete the task
		task.UpdateStatus(orchestration.TaskStatusCompleted)
		task.Result = map[string]interface{}{
			"answer": "42",
		}
		task.ActualCost = 0.50
		err = taskQueue.Update(task)
		require.NoError(t, err)

		// Get result
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+task.ID+"/result", nil)
		w := httptest.NewRecorder()

		server.Router().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp api.TaskResultResponse
		err = json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, task.ID, resp.TaskID)
		assert.Equal(t, "completed", resp.Status)
		assert.Equal(t, 0.50, resp.Cost)
	})
}

func TestTaskQueueConcurrency(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	queue := orchestration.NewTaskQueue(ctx, 1000, logger)
	defer queue.Close()

	// Submit 100 tasks concurrently
	numTasks := 100
	done := make(chan bool, numTasks)

	for i := 0; i < numTasks; i++ {
		go func(index int) {
			task := orchestration.NewTask(
				"concurrent-user",
				"concurrent-task",
				[]string{"capability"},
				map[string]interface{}{"index": index},
			)
			err := queue.Enqueue(task)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all tasks
	for i := 0; i < numTasks; i++ {
		<-done
	}

	// Verify queue size
	assert.Equal(t, numTasks, queue.Size())
}

func TestTaskPriorityOrdering(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	queue := orchestration.NewTaskQueue(ctx, 100, logger)
	defer queue.Close()

	// Submit tasks with different priorities
	taskLow := orchestration.NewTask("user", "low", []string{"cap"}, map[string]interface{}{})
	taskLow.Priority = orchestration.PriorityLow
	queue.Enqueue(taskLow)

	taskHigh := orchestration.NewTask("user", "high", []string{"cap"}, map[string]interface{}{})
	taskHigh.Priority = orchestration.PriorityHigh
	queue.Enqueue(taskHigh)

	taskNormal := orchestration.NewTask("user", "normal", []string{"cap"}, map[string]interface{}{})
	taskNormal.Priority = orchestration.PriorityNormal
	queue.Enqueue(taskNormal)

	taskCritical := orchestration.NewTask("user", "critical", []string{"cap"}, map[string]interface{}{})
	taskCritical.Priority = orchestration.PriorityCritical
	queue.Enqueue(taskCritical)

	// Dequeue and verify order (Critical > High > Normal > Low)
	task1, _ := queue.Dequeue()
	assert.Equal(t, "critical", task1.Type)

	task2, _ := queue.Dequeue()
	assert.Equal(t, "high", task2.Type)

	task3, _ := queue.Dequeue()
	assert.Equal(t, "normal", task3.Type)

	task4, _ := queue.Dequeue()
	assert.Equal(t, "low", task4.Type)
}
