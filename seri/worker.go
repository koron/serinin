package seri

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
)

// Worker provides set of workers to make HTTP request and store result to (redis) store.
type Worker struct {
	n  int
	ch chan WorkFn
	wg sync.WaitGroup

	mode   int32
	cancel context.CancelFunc
}

// NewWorker creates a Worker manager.
func NewWorker(n int) *Worker {
	return &Worker{
		n:  n,
		ch: make(chan WorkFn),
	}
}

// Start starts all workers.
func (w *Worker) Start() {
	if !atomic.CompareAndSwapInt32(&w.mode, workerModeIdle, workerModeStarting) {
		log.Printf("[ERROR] failed to Worker.Start: invalid state")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < w.n; i++ {
		w.wg.Add(1)
		go w.run(ctx)
	}
	w.cancel = cancel
	atomic.StoreInt32(&w.mode, workerModeRunning)
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	for {
		select {
		case fn := <-w.ch:
			fn()
		case <-ctx.Done():
			return
		}
	}
}

// Close stops all workers gracefully.
func (w *Worker) Close() {
	if !atomic.CompareAndSwapInt32(&w.mode, workerModeRunning, workerModeClosing) {
		log.Printf("[ERROR] failed to Worker.Close: invalid state")
		return
	}
	w.cancel()
	w.wg.Wait()
	close(w.ch)
	atomic.StoreInt32(&w.mode, workerModeClosed)
}

// ErrWorkerNotStarted shows that Run() is called for not running worker.
var ErrWorkerNotStarted = errors.New("worker isn't started")

// ErrWorkerQueueFailed shows a queue exceeded.
var ErrWorkerQueueFailed = errors.New("worker failed to queue a job")

// Run reserves to execute a job.
func (w *Worker) Run(fn WorkFn) error {
	if atomic.LoadInt32(&w.mode) != 2 {
		return ErrWorkerNotStarted
	}
	select {
	case w.ch <- fn:
		return nil
	default:
		return ErrWorkerQueueFailed
	}
}

// WorkFn provides entry point of job.
type WorkFn func()

const (
	workerModeIdle int32 = iota
	workerModeStarting
	workerModeRunning
	workerModeClosing
	workerModeClosed
)
