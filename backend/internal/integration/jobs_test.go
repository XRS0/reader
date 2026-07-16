//go:build integration

package integration_test

import (
	"errors"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/jobs"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPostgresJobLifecycleRetryDeadCompleteAndRecoverStale(t *testing.T) {
	resetDatabase(t)
	queue := jobs.NewQueue(integrationDB)
	runnableAt := time.Now().UTC().Add(-time.Minute)
	payload := struct {
		BookID string `json:"book_id"`
	}{BookID: "book-integration-1"}

	enqueued, err := queue.Enqueue(testContext(t), "process_book", payload, 10, 2, runnableAt)
	require.NoError(t, err)
	require.Equal(t, "queued", enqueued.Status)
	require.Equal(t, 2, enqueued.MaxAttempts)
	depth, err := queue.Depth(testContext(t))
	require.NoError(t, err)
	require.Equal(t, int64(1), depth)

	claimed, err := queue.Claim(testContext(t), "worker-a")
	require.NoError(t, err)
	require.Equal(t, enqueued.ID, claimed.ID)
	require.Equal(t, "running", claimed.Status)
	require.Equal(t, 1, claimed.Attempts)
	require.Equal(t, "worker-a", claimed.LockedBy)
	require.NotNil(t, claimed.LockedAt)
	decoded, err := jobs.DecodePayload[struct {
		BookID string `json:"book_id"`
	}](claimed)
	require.NoError(t, err)
	require.Equal(t, payload.BookID, decoded.BookID)

	_, err = queue.Claim(testContext(t), "worker-b")
	require.ErrorIs(t, err, jobs.ErrNoJob, "a leased job must not be claimed by another worker")

	failureStarted := time.Now().UTC()
	require.NoError(t, queue.Fail(testContext(t), claimed, "worker-a", errors.New("temporary parser failure")))
	retry := loadJob(t, claimed.ID)
	require.Equal(t, "retry", retry.Status)
	require.Equal(t, 1, retry.Attempts)
	require.Equal(t, "temporary parser failure", retry.LastError)
	require.Empty(t, retry.LockedBy)
	require.Nil(t, retry.LockedAt)
	require.Nil(t, retry.FinishedAt)
	require.WithinDuration(t, failureStarted.Add(jobs.RetryDelay(1)), retry.RunAfter, time.Second)
	makeJobDeferred(t, retry.ID)

	depth, err = queue.Depth(testContext(t))
	require.NoError(t, err)
	require.Equal(t, int64(1), depth, "scheduled retries remain visible in queue depth")
	_, err = queue.Claim(testContext(t), "worker-b")
	require.ErrorIs(t, err, jobs.ErrNoJob, "retry backoff must prevent an early claim")
	makeJobRunnable(t, retry.ID)

	secondAttempt, err := queue.Claim(testContext(t), "worker-b")
	require.NoError(t, err)
	require.Equal(t, retry.ID, secondAttempt.ID)
	require.Equal(t, 2, secondAttempt.Attempts)
	require.Equal(t, "running", secondAttempt.Status)
	require.NoError(t, queue.Fail(testContext(t), secondAttempt, "worker-b", errors.New("permanent parser failure")))
	dead := loadJob(t, secondAttempt.ID)
	require.Equal(t, "dead", dead.Status)
	require.Equal(t, 2, dead.Attempts)
	require.Equal(t, "permanent parser failure", dead.LastError)
	require.NotNil(t, dead.FinishedAt)
	require.Empty(t, dead.LockedBy)
	require.Nil(t, dead.LockedAt)
	depth, err = queue.Depth(testContext(t))
	require.NoError(t, err)
	require.Zero(t, depth)
	_, err = queue.Claim(testContext(t), "worker-c")
	require.ErrorIs(t, err, jobs.ErrNoJob)

	completion, err := queue.Enqueue(testContext(t), "cleanup_book", map[string]string{"book_id": "book-integration-2"}, 0, 3, runnableAt)
	require.NoError(t, err)
	completion, err = queue.Claim(testContext(t), "worker-complete")
	require.NoError(t, err)
	require.NoError(t, queue.Complete(testContext(t), completion.ID, "worker-complete"))
	completed := loadJob(t, completion.ID)
	require.Equal(t, "completed", completed.Status)
	require.NotNil(t, completed.FinishedAt)
	require.Empty(t, completed.LockedBy)
	require.Nil(t, completed.LockedAt)
	require.Error(t, queue.Complete(testContext(t), completion.ID, "worker-complete"), "duplicate completion must not mutate an already completed job")
	require.Equal(t, "completed", loadJob(t, completion.ID).Status)

	staleJob, err := queue.Enqueue(testContext(t), "process_book", map[string]string{"book_id": "book-integration-3"}, 0, 2, runnableAt)
	require.NoError(t, err)
	staleJob, err = queue.Claim(testContext(t), "worker-crashed")
	require.NoError(t, err)
	makeLeaseStale(t, staleJob.ID)
	recovered, err := queue.RecoverStale(testContext(t), time.Minute)
	require.NoError(t, err)
	require.Equal(t, int64(1), recovered)
	staleRetry := loadJob(t, staleJob.ID)
	require.Equal(t, "retry", staleRetry.Status)
	require.Equal(t, "worker lease expired", staleRetry.LastError)
	require.Nil(t, staleRetry.FinishedAt)

	staleSecondAttempt, err := queue.Claim(testContext(t), "worker-after-crash")
	require.NoError(t, err)
	require.Equal(t, 2, staleSecondAttempt.Attempts)
	makeLeaseStale(t, staleSecondAttempt.ID)
	recovered, err = queue.RecoverStale(testContext(t), time.Minute)
	require.NoError(t, err)
	require.Equal(t, int64(1), recovered)
	staleDead := loadJob(t, staleSecondAttempt.ID)
	require.Equal(t, "dead", staleDead.Status)
	require.Equal(t, "worker lease expired", staleDead.LastError)
	require.NotNil(t, staleDead.FinishedAt)
}

func loadJob(t *testing.T, id uuid.UUID) model.Job {
	t.Helper()
	var job model.Job
	err := integrationDB.NewSelect().Model(&job).Where("id=?", id).Scan(testContext(t))
	require.NoError(t, err)
	return job
}

func makeJobRunnable(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := integrationDB.NewUpdate().Table("book_processing_jobs").
		Set("run_after=now() - interval '1 second'").
		Where("id=?", id).
		Exec(testContext(t))
	require.NoError(t, err)
}

func makeJobDeferred(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := integrationDB.NewUpdate().Table("book_processing_jobs").
		Set("run_after=now() + interval '1 hour'").
		Where("id=?", id).
		Exec(testContext(t))
	require.NoError(t, err)
}

func makeLeaseStale(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := integrationDB.NewUpdate().Table("book_processing_jobs").
		Set("locked_at=now() - interval '10 minutes'").
		Where("id=?", id).
		Exec(testContext(t))
	require.NoError(t, err)
}
