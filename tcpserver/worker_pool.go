package tcpserver

import (
	"fmt"
	"sync"
	"time"
)

// Task represents a unit of work to be processed by a worker
type Task struct {
	ID       uint64
	Data     []byte
	ConnID   uint64
	Response chan []byte
	Error    chan error
}

// Worker represents a worker goroutine
type Worker struct {
	ID         int
	TaskQueue  chan *Task
	WorkerPool chan chan *Task
	QuitChan   chan bool
	processor  TaskProcessor
}

// TaskProcessor defines the interface for processing tasks
type TaskProcessor interface {
	ProcessTask(task *Task) ([]byte, error)
}

// NewWorker creates a new worker
func NewWorker(id int, workerPool chan chan *Task, processor TaskProcessor) *Worker {
	return &Worker{
		ID:         id,
		TaskQueue:  make(chan *Task),
		WorkerPool: workerPool,
		QuitChan:   make(chan bool),
		processor:  processor,
	}
}

// Start starts the worker
func (w *Worker) Start() {
	go func() {
		for {
			// Register the current worker into the worker queue
			w.WorkerPool <- w.TaskQueue

			select {
			case task := <-w.TaskQueue:
				// Process the task
				result, err := w.processor.ProcessTask(task)
				if err != nil {
					select {
					case task.Error <- err:
					case <-time.After(time.Second):
						// Timeout sending error
					}
				} else {
					select {
					case task.Response <- result:
					case <-time.After(time.Second):
						// Timeout sending response
					}
				}

			case <-w.QuitChan:
				// Stop the worker
				return
			}
		}
	}()
}

// Stop stops the worker
func (w *Worker) Stop() {
	go func() {
		w.QuitChan <- true
	}()
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	WorkerQueue chan chan *Task
	TaskQueue   chan *Task
	Workers     []*Worker
	QuitChan    chan bool
	processor   TaskProcessor
	maxWorkers  int
	taskID      uint64
	mu          sync.Mutex
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(maxWorkers int, processor TaskProcessor) *WorkerPool {
	pool := &WorkerPool{
		WorkerQueue: make(chan chan *Task, maxWorkers),
		TaskQueue:   make(chan *Task, maxWorkers*10), // Increase buffer size significantly
		QuitChan:    make(chan bool),
		processor:   processor,
		maxWorkers:  maxWorkers,
	}

	// Create and start workers
	for i := 0; i < maxWorkers; i++ {
		worker := NewWorker(i, pool.WorkerQueue, processor)
		pool.Workers = append(pool.Workers, worker)
		worker.Start()
	}

	// Start the dispatcher
	go pool.dispatch()

	return pool
}

// dispatch dispatches tasks to available workers
func (wp *WorkerPool) dispatch() {
	for {
		select {
		case task := <-wp.TaskQueue:
			// Get an available worker
			go func() {
				workerTaskQueue := <-wp.WorkerQueue
				workerTaskQueue <- task
			}()

		case <-wp.QuitChan:
			// Stop all workers
			for _, worker := range wp.Workers {
				worker.Stop()
			}
			return
		}
	}
}

// SubmitTask submits a task to the worker pool
func (wp *WorkerPool) SubmitTask(data []byte, connID uint64) *Task {
	wp.mu.Lock()
	wp.taskID++
	taskID := wp.taskID
	wp.mu.Unlock()

	task := &Task{
		ID:       taskID,
		Data:     data,
		ConnID:   connID,
		Response: make(chan []byte, 1),
		Error:    make(chan error, 1),
	}

	// Try to submit task, with longer timeout
	select {
	case wp.TaskQueue <- task:
		return task
	case <-time.After(1 * time.Second):
		// Task queue is full, return error task
		go func() {
			task.Error <- ErrTaskQueueFull
		}()
		return task
	}
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.QuitChan <- true
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"max_workers":     wp.maxWorkers,
		"active_workers":  len(wp.Workers),
		"pending_tasks":   len(wp.TaskQueue),
		"available_slots": cap(wp.TaskQueue) - len(wp.TaskQueue),
	}
}

var (
	ErrTaskQueueFull = fmt.Errorf("task queue is full")
)
