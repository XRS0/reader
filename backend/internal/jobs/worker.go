package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
)

type Handler func(context.Context, model.Job) error
type Worker struct {
	queue       *Queue
	id          string
	concurrency int
	poll        time.Duration
	timeout     time.Duration
	staleLock   time.Duration
	handlers    map[string]Handler
	logger      *slog.Logger
}

func NewWorker(queue *Queue, id string, concurrency int, poll, timeout, staleLock time.Duration, logger *slog.Logger) *Worker {
	return &Worker{queue: queue, id: id, concurrency: concurrency, poll: poll, timeout: timeout, staleLock: staleLock, handlers: map[string]Handler{}, logger: logger}
}
func (w *Worker) Handle(jobType string, h Handler) { w.handlers[jobType] = h }
func (w *Worker) Run(ctx context.Context) error {
	_, _ = w.queue.RecoverStale(ctx, w.staleLock)
	var wg sync.WaitGroup
	for i := 0; i < w.concurrency; i++ {
		wg.Add(1)
		go func(index int) { defer wg.Done(); w.loop(ctx, index) }(i)
	}
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}
func (w *Worker) loop(ctx context.Context, index int) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			job, err := w.queue.Claim(ctx, w.id)
			if errors.Is(err, ErrNoJob) {
				timer.Reset(w.poll)
				continue
			}
			if err != nil {
				w.logger.Error("claim job", "error", err, "worker_index", index)
				timer.Reset(w.poll)
				continue
			}
			h := w.handlers[job.Type]
			if h == nil {
				err = errors.New("no handler for job type " + job.Type)
			} else {
				jobCtx, cancel := context.WithTimeout(ctx, w.timeout)
				err = h(jobCtx, job)
				cancel()
			}
			if err != nil {
				w.logger.Error("job failed", "job_id", job.ID, "type", job.Type, "attempt", job.Attempts, "error", err)
				if failErr := w.queue.Fail(ctx, job, w.id, err); failErr != nil {
					w.logger.Error("record job failure", "job_id", job.ID, "error", failErr)
				}
			} else if err = w.queue.Complete(ctx, job.ID, w.id); err != nil {
				w.logger.Error("complete job", "job_id", job.ID, "error", err)
			}
			timer.Reset(0)
		}
	}
}
