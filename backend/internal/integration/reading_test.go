//go:build integration

package integration_test

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/reading"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReadingProgressOptimisticConcurrency(t *testing.T) {
	resetDatabase(t)
	user := createUser(t, "UTC")
	book := createBook(t, user.ID, "Concurrency")
	chapter := createChapter(t, book.ID, 0)
	device := createDevice(t, user.ID)
	service := reading.NewProgressService(integrationDB)

	initial, err := service.Get(testContext(t), user.ID, book.ID)
	require.NoError(t, err)
	require.Zero(t, initial.Revision)
	require.JSONEq(t, `{}`, string(initial.Locator))
	require.Equal(t, "chapter_offset", initial.LocatorType)

	first, err := service.Put(testContext(t), user.ID, book.ID, reading.ProgressInput{
		ChapterID:       &chapter.ID,
		LocatorType:     "character_offset",
		Locator:         json.RawMessage(`{"offset":100}`),
		CharacterOffset: 100,
		ChapterProgress: 10,
		ProgressPercent: 10,
		ScrollPercent:   10,
		Revision:        0,
		ClientID:        "first-client",
		DeviceID:        &device.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), first.Revision)

	second, err := service.Put(testContext(t), user.ID, book.ID, reading.ProgressInput{
		ChapterID:       &chapter.ID,
		LocatorType:     "character_offset",
		Locator:         json.RawMessage(`{"offset":200}`),
		CharacterOffset: 200,
		ChapterProgress: 20,
		ProgressPercent: 20,
		ScrollPercent:   20,
		Revision:        first.Revision,
		ClientID:        "second-client",
		DeviceID:        &device.ID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), second.Revision)

	_, err = service.Put(testContext(t), user.ID, book.ID, reading.ProgressInput{
		ChapterID:       &chapter.ID,
		LocatorType:     "character_offset",
		Locator:         json.RawMessage(`{"offset":150}`),
		CharacterOffset: 150,
		ProgressPercent: 15,
		Revision:        first.Revision,
		ClientID:        "stale-client",
		DeviceID:        &device.ID,
	})
	require.ErrorIs(t, err, reading.ErrRevisionConflict)
	var staleConflict *reading.ConflictError
	require.ErrorAs(t, err, &staleConflict)
	require.Equal(t, int64(2), staleConflict.Current.Revision)
	require.Equal(t, int64(200), staleConflict.Current.CharacterOffset)

	inputs := []reading.ProgressInput{
		{
			ChapterID:       &chapter.ID,
			LocatorType:     "character_offset",
			Locator:         json.RawMessage(`{"offset":300}`),
			CharacterOffset: 300,
			ProgressPercent: 30,
			Revision:        second.Revision,
			ClientID:        "concurrent-a",
			DeviceID:        &device.ID,
		},
		{
			ChapterID:       &chapter.ID,
			LocatorType:     "character_offset",
			Locator:         json.RawMessage(`{"offset":400}`),
			CharacterOffset: 400,
			ProgressPercent: 40,
			Revision:        second.Revision,
			ClientID:        "concurrent-b",
			DeviceID:        &device.ID,
		},
	}
	type outcome struct {
		revision int64
		err      error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, len(inputs))
	var wg sync.WaitGroup
	for _, input := range inputs {
		input := input
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, putErr := service.Put(testContext(t), user.ID, book.ID, input)
			outcomes <- outcome{revision: result.Revision, err: putErr}
		}()
	}
	close(start)
	wg.Wait()
	close(outcomes)

	var successes, conflicts int
	for result := range outcomes {
		switch {
		case result.err == nil:
			successes++
			require.Equal(t, int64(3), result.revision)
		case errors.Is(result.err, reading.ErrRevisionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", result.err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	current, err := service.Get(testContext(t), user.ID, book.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), current.Revision)
	require.Contains(t, []int64{300, 400}, current.CharacterOffset)
}

func TestReadingProgressAndSessionOwnership(t *testing.T) {
	resetDatabase(t)
	owner := createUser(t, "UTC")
	other := createUser(t, "UTC")
	ownedBook := createBook(t, owner.ID, "Owned")
	foreignBook := createBook(t, other.ID, "Foreign")
	ownedChapter := createChapter(t, ownedBook.ID, 0)
	foreignChapter := createChapter(t, foreignBook.ID, 0)
	ownedDevice := createDevice(t, owner.ID)
	foreignDevice := createDevice(t, other.ID)
	progress := reading.NewProgressService(integrationDB)

	_, err := progress.Get(testContext(t), owner.ID, foreignBook.ID)
	require.ErrorIs(t, err, reading.ErrNotFound)

	_, err = progress.Put(testContext(t), owner.ID, ownedBook.ID, reading.ProgressInput{
		ChapterID:       &foreignChapter.ID,
		LocatorType:     "character_offset",
		Locator:         json.RawMessage(`{"offset":10}`),
		CharacterOffset: 10,
		ProgressPercent: 1,
		Revision:        0,
		ClientID:        "ownership-chapter",
		DeviceID:        &ownedDevice.ID,
	})
	require.Error(t, err, "a chapter from another user's book must be rejected")

	_, err = progress.Put(testContext(t), owner.ID, ownedBook.ID, reading.ProgressInput{
		ChapterID:       &ownedChapter.ID,
		LocatorType:     "character_offset",
		Locator:         json.RawMessage(`{"offset":10}`),
		CharacterOffset: 10,
		ProgressPercent: 1,
		Revision:        0,
		ClientID:        "ownership-device",
		DeviceID:        &foreignDevice.ID,
	})
	require.Error(t, err, "another user's device must be rejected")

	sessions := reading.NewSessionService(integrationDB, integrationReadingConfig())
	_, err = sessions.Start(testContext(t), owner.ID, reading.StartSessionInput{
		BookID:          foreignBook.ID,
		DeviceID:        &ownedDevice.ID,
		Locator:         json.RawMessage(`{"offset":0}`),
		ProgressPercent: 0,
	})
	require.ErrorIs(t, err, reading.ErrNotFound)

	_, err = sessions.Start(testContext(t), owner.ID, reading.StartSessionInput{
		BookID:          ownedBook.ID,
		DeviceID:        &foreignDevice.ID,
		Locator:         json.RawMessage(`{"offset":0}`),
		ProgressPercent: 0,
	})
	require.Error(t, err, "another user's device must not be attachable to a reading session")
}

func TestReadingSessionHeartbeatIdempotencyAccountingFinishAndStale(t *testing.T) {
	resetDatabase(t)
	owner := createUser(t, "UTC")
	other := createUser(t, "UTC")
	book := createBook(t, owner.ID, "Session accounting")
	device := createDevice(t, owner.ID)
	service := reading.NewSessionService(integrationDB, integrationReadingConfig())

	session, err := service.Start(testContext(t), owner.ID, reading.StartSessionInput{
		BookID:          book.ID,
		DeviceID:        &device.ID,
		Locator:         json.RawMessage(`{"offset":0}`),
		ProgressPercent: 0,
	})
	require.NoError(t, err)
	require.Equal(t, "active", session.Status)

	setSessionHeartbeatAgo(t, session.ID, 10*time.Second, true)
	heartbeat := reading.HeartbeatInput{
		Locator:           json.RawMessage(`{"offset":600}`),
		ProgressPercent:   10,
		Visible:           true,
		Focused:           true,
		UserActive:        true,
		LastInteractionMS: 500,
		ClientTimestamp:   time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		IdempotencyKey:    "heartbeat-0001",
		Sequence:          1,
		CharactersRead:    600,
	}
	first, err := service.Heartbeat(testContext(t), owner.ID, session.ID, heartbeat)
	require.NoError(t, err)
	require.Equal(t, "active", first.Status)
	require.GreaterOrEqual(t, first.ActiveSeconds, int64(9))
	require.LessOrEqual(t, first.ActiveSeconds, int64(12))
	require.Zero(t, first.IdleSeconds)
	require.Equal(t, int64(600), first.CharactersRead)
	require.Equal(t, int64(100), first.WordsReadEstimate)
	require.InDelta(t, 0.4, first.PagesReadEstimate, 0.0001)

	duplicate, err := service.Heartbeat(testContext(t), owner.ID, session.ID, heartbeat)
	require.NoError(t, err)
	require.Equal(t, first.ActiveSeconds, duplicate.ActiveSeconds)
	require.Equal(t, first.IdleSeconds, duplicate.IdleSeconds)
	require.Equal(t, first.LastSequence, duplicate.LastSequence)
	require.Equal(t, int64(1), countEvents(t, session.ID, "heartbeat"))

	badSequence := heartbeat
	badSequence.IdempotencyKey = "heartbeat-stale-sequence"
	_, err = service.Heartbeat(testContext(t), owner.ID, session.ID, badSequence)
	require.ErrorIs(t, err, reading.ErrSequence)

	setSessionHeartbeatAgo(t, session.ID, 10*time.Second, true)
	idle := heartbeat
	idle.IdempotencyKey = "heartbeat-0002-idle"
	idle.Sequence = 2
	idle.Visible = true
	idle.Focused = true
	idle.UserActive = true
	idle.LastInteractionMS = int64((2 * time.Minute) / time.Millisecond)
	idleResult, err := service.Heartbeat(testContext(t), owner.ID, session.ID, idle)
	require.NoError(t, err)
	require.Equal(t, "idle", idleResult.Status)
	require.Equal(t, first.ActiveSeconds, idleResult.ActiveSeconds, "idle interval must not be credited as active")
	idleAdded := idleResult.IdleSeconds - first.IdleSeconds
	require.GreaterOrEqual(t, idleAdded, int64(9))
	require.LessOrEqual(t, idleAdded, int64(12))

	setSessionHeartbeatAgo(t, session.ID, 5*time.Minute, true)
	resumed := heartbeat
	resumed.IdempotencyKey = "heartbeat-0003-resumed"
	resumed.Sequence = 3
	resumed.ProgressPercent = 20
	resumed.CharactersRead = 1200
	resumedResult, err := service.Heartbeat(testContext(t), owner.ID, session.ID, resumed)
	require.NoError(t, err)
	require.Equal(t, "active", resumedResult.Status)
	require.Equal(t, first.ActiveSeconds+30, resumedResult.ActiveSeconds, "active credit must be capped by HeartbeatMaxGap")
	gapIdleAdded := resumedResult.IdleSeconds - idleResult.IdleSeconds
	require.GreaterOrEqual(t, gapIdleAdded, int64(269))
	require.LessOrEqual(t, gapIdleAdded, int64(273), "the uncredited gap remains idle")

	_, err = service.Heartbeat(testContext(t), other.ID, session.ID, reading.HeartbeatInput{
		Locator:        json.RawMessage(`{}`),
		Visible:        true,
		Focused:        true,
		UserActive:     true,
		IdempotencyKey: "heartbeat-foreign-user",
		Sequence:       4,
	})
	require.ErrorIs(t, err, reading.ErrNotFound)

	setSessionHeartbeatAgo(t, session.ID, 10*time.Second, true)
	activeBeforeFinish := resumedResult.ActiveSeconds
	finished, err := service.Finish(testContext(t), owner.ID, session.ID, reading.FinishInput{
		Locator:         json.RawMessage(`{"offset":1300}`),
		ProgressPercent: 25,
		CloseReason:     "user_closed_reader",
		IdempotencyKey:  "finish-session-0001",
		Sequence:        4,
	})
	require.NoError(t, err)
	require.Equal(t, "finished", finished.Status)
	require.NotNil(t, finished.EndedAt)
	require.Equal(t, "user_closed_reader", finished.CloseReason)
	require.Equal(t, activeBeforeFinish, finished.ActiveSeconds, "finish must not manufacture active time after the last heartbeat")

	finishedAgain, err := service.Finish(testContext(t), owner.ID, session.ID, reading.FinishInput{
		Locator:         json.RawMessage(`{"offset":1300}`),
		ProgressPercent: 25,
		CloseReason:     "user_closed_reader",
		IdempotencyKey:  "finish-session-0001",
		Sequence:        4,
	})
	require.NoError(t, err)
	require.Equal(t, finished.ActiveSeconds, finishedAgain.ActiveSeconds)
	require.Equal(t, finished.IdleSeconds, finishedAgain.IdleSeconds)
	require.Equal(t, int64(1), countEvents(t, session.ID, "session_finished"))

	postFinish := heartbeat
	postFinish.IdempotencyKey = "heartbeat-after-finish"
	postFinish.Sequence = 5
	_, err = service.Heartbeat(testContext(t), owner.ID, session.ID, postFinish)
	require.ErrorIs(t, err, reading.ErrSessionFinished)

	staleSession, err := service.Start(testContext(t), owner.ID, reading.StartSessionInput{
		BookID:          book.ID,
		DeviceID:        &device.ID,
		Locator:         json.RawMessage(`{}`),
		ProgressPercent: 25,
	})
	require.NoError(t, err)
	setSessionHeartbeatAgo(t, staleSession.ID, 10*time.Minute, true)
	finalized, err := service.FinalizeStale(testContext(t))
	require.NoError(t, err)
	require.Equal(t, int64(1), finalized)
	stale, err := service.Get(testContext(t), owner.ID, staleSession.ID)
	require.NoError(t, err)
	require.Equal(t, "stale", stale.Status)
	require.Equal(t, "stale_session_finalized", stale.CloseReason)
	require.NotNil(t, stale.EndedAt)
	require.WithinDuration(t, stale.LastHeartbeatAt, *stale.EndedAt, time.Millisecond)
}

func integrationReadingConfig() config.Reading {
	return config.Reading{
		HeartbeatInterval: time.Second,
		HeartbeatMaxGap:   30 * time.Second,
		IdleThreshold:     time.Minute,
		StaleAfter:        2 * time.Minute,
	}
}

func setSessionHeartbeatAgo(t *testing.T, sessionID uuid.UUID, ago time.Duration, lastWasActive bool) {
	t.Helper()
	_, err := integrationDB.NewUpdate().Table("reading_sessions").
		Set("last_heartbeat_at=date_trunc('second', now()) - (? * interval '1 microsecond')", ago.Microseconds()).
		Set("last_activity_at=date_trunc('second', now()) - (? * interval '1 microsecond')", ago.Microseconds()).
		Set("last_was_active=?", lastWasActive).
		Where("id=?", sessionID).
		Exec(testContext(t))
	require.NoError(t, err)
}

func countEvents(t *testing.T, sessionID uuid.UUID, eventType string) int64 {
	t.Helper()
	var count int64
	err := integrationDB.NewSelect().Table("reading_events").
		ColumnExpr("count(*)").
		Where("session_id=? AND type=?", sessionID, eventType).
		Scan(testContext(t), &count)
	require.NoError(t, err)
	return count
}
