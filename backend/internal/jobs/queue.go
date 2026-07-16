package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

var ErrNoJob = errors.New("no job available")

type Queue struct {
	db  *bun.DB
	now func() time.Time
}

func NewQueue(db *bun.DB) *Queue { return &Queue{db: db, now: time.Now} }

func (q *Queue) Enqueue(ctx context.Context, jobType string, payload any, priority, maxAttempts int, runAfter time.Time) (model.Job, error) {
	if strings.TrimSpace(jobType) == "" {
		return model.Job{}, errors.New("job type is required")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return model.Job{}, err
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if runAfter.IsZero() {
		runAfter = q.now().UTC()
	}
	job := model.Job{ID: uuid.New(), Type: jobType, Payload: raw, Status: "queued", Priority: priority, MaxAttempts: maxAttempts, RunAfter: runAfter, CreatedAt: q.now().UTC(), UpdatedAt: q.now().UTC()}
	_, err = q.db.NewInsert().Model(&job).Exec(ctx)
	return job, err
}

func (q *Queue) Claim(ctx context.Context, workerID string) (model.Job, error) {
	if strings.TrimSpace(workerID) == "" {
		return model.Job{}, errors.New("worker ID is required")
	}
	var job model.Job
	err := q.db.NewRaw(`
WITH candidate AS (
 SELECT id FROM book_processing_jobs
 WHERE status IN ('queued','retry') AND run_after <= now()
 ORDER BY priority DESC, run_after ASC, created_at ASC
 FOR UPDATE SKIP LOCKED LIMIT 1
)
UPDATE book_processing_jobs AS j
SET status='running', locked_at=now(), locked_by=?, attempts=j.attempts+1, updated_at=now()
FROM candidate WHERE j.id=candidate.id
RETURNING j.*`, workerID).Scan(ctx, &job)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Job{}, ErrNoJob
	}
	return job, err
}

func (q *Queue) Complete(ctx context.Context, id uuid.UUID, workerID string) error {
	now := q.now().UTC()
	res, err := q.db.NewUpdate().Model((*model.Job)(nil)).Set("status='completed'").Set("finished_at=?", now).Set("updated_at=?", now).Set("locked_at=NULL").Set("locked_by='' ").Where("id=?", id).Where("status='running'").Where("locked_by=?", workerID).Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("job lease was lost")
	}
	return nil
}

func (q *Queue) Fail(ctx context.Context, job model.Job, workerID string, cause error) error {
	message := "unknown job failure"
	if cause != nil {
		message = cause.Error()
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	status := "retry"
	var finished any = nil
	if job.Attempts >= job.MaxAttempts {
		status = "dead"
		finished = q.now().UTC()
	}
	delay := RetryDelay(job.Attempts)
	res, err := q.db.NewUpdate().Model((*model.Job)(nil)).Set("status=?", status).Set("run_after=?", q.now().UTC().Add(delay)).Set("last_error=?", message).Set("updated_at=?", q.now().UTC()).Set("finished_at=?", finished).Set("locked_at=NULL").Set("locked_by='' ").Where("id=?", job.ID).Where("status='running'").Where("locked_by=?", workerID).Exec(ctx)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return errors.New("job lease was lost")
	}
	return nil
}

func RetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	seconds := math.Pow(2, float64(attempt-1))
	if seconds > 300 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

func (q *Queue) RecoverStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	res, err := q.db.NewUpdate().Model((*model.Job)(nil)).Set("status=CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'retry' END").Set("last_error='worker lease expired'").Set("run_after=now()").Set("finished_at=CASE WHEN attempts >= max_attempts THEN now() ELSE NULL END").Set("locked_at=NULL").Set("locked_by='' ").Set("updated_at=now()").Where("status='running'").Where("locked_at < ?", q.now().UTC().Add(-olderThan)).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (q *Queue) Depth(ctx context.Context) (int64, error) {
	var count int64
	err := q.db.NewSelect().Table("book_processing_jobs").ColumnExpr("count(*)").Where("status IN ('queued','retry')").Scan(ctx, &count)
	return count, err
}
func DecodePayload[T any](job model.Job) (T, error) {
	var payload T
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return payload, fmt.Errorf("decode %s job payload: %w", job.Type, err)
	}
	return payload, nil
}
