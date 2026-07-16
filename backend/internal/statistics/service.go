package statistics

import (
	"context"
	"errors"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Service struct{ db *bun.DB }

func NewService(db *bun.DB) *Service { return &Service{db: db} }

type Overview struct {
	ActiveSeconds         int64   `json:"active_seconds"`
	IdleSeconds           int64   `json:"idle_seconds"`
	SessionCount          int64   `json:"session_count"`
	AverageSessionSeconds float64 `json:"average_session_seconds"`
	WordsRead             int64   `json:"words_read"`
	PagesRead             float64 `json:"pages_read"`
	BooksTotal            int64   `json:"books_total"`
	BooksStarted          int64   `json:"books_started"`
	BooksCompleted        int64   `json:"books_completed"`
	DictionaryWords       int64   `json:"dictionary_words"`
	DictionaryMastered    int64   `json:"dictionary_mastered"`
}
type TimeBucket struct {
	Period        time.Time `json:"period"`
	ActiveSeconds int64     `json:"active_seconds"`
	IdleSeconds   int64     `json:"idle_seconds"`
	SessionCount  int64     `json:"session_count"`
	WordsRead     int64     `json:"words_read"`
}
type BookStat struct {
	BookID          uuid.UUID  `json:"book_id"`
	Title           string     `json:"title"`
	Format          string     `json:"format"`
	Language        string     `json:"language"`
	ProgressPercent float64    `json:"progress_percent"`
	ActiveSeconds   int64      `json:"active_seconds"`
	IdleSeconds     int64      `json:"idle_seconds"`
	SessionCount    int64      `json:"session_count"`
	WordsRead       int64      `json:"words_read"`
	LastReadAt      *time.Time `json:"last_read_at,omitempty"`
}
type Streak struct {
	Current int      `json:"current"`
	Longest int      `json:"longest"`
	Dates   []string `json:"dates"`
}
type DictionaryStat struct {
	Total      int64 `json:"total"`
	Unknown    int64 `json:"unknown"`
	Learning   int64 `json:"learning"`
	Known      int64 `json:"known"`
	Mastered   int64 `json:"mastered"`
	Ignored    int64 `json:"ignored"`
	Encounters int64 `json:"encounters"`
}

func (s *Service) Overview(ctx context.Context, userID uuid.UUID) (Overview, error) {
	var o Overview
	err := s.db.NewRaw(`SELECT COALESCE(sum(active_seconds),0) AS active_seconds,COALESCE(sum(idle_seconds),0) AS idle_seconds,count(*) AS session_count,COALESCE(avg(active_seconds),0) AS average_session_seconds,COALESCE(sum(words_read_estimate),0) AS words_read,COALESCE(sum(pages_read_estimate),0) AS pages_read FROM reading_sessions WHERE user_id=? AND status IN ('finished','stale','finalized')`, userID).Scan(ctx, &o)
	if err != nil {
		return o, err
	}
	if err = s.db.NewRaw(`SELECT count(*) AS books_total,count(*) FILTER(WHERE COALESCE(rp.progress_percent,0)>0) AS books_started,count(*) FILTER(WHERE COALESCE(rp.progress_percent,0)>=99.5) AS books_completed FROM books b LEFT JOIN reading_progress rp ON rp.book_id=b.id AND rp.user_id=b.user_id WHERE b.user_id=? AND b.deleted_at IS NULL`, userID).Scan(ctx, &o); err != nil {
		return o, err
	}
	err = s.db.NewRaw(`SELECT count(*) AS dictionary_words,count(*) FILTER(WHERE status='mastered') AS dictionary_mastered FROM dictionary_entries WHERE user_id=? AND deleted_at IS NULL`, userID).Scan(ctx, &o)
	return o, err
}
func (s *Service) Buckets(ctx context.Context, userID uuid.UUID, from, to time.Time, timezone, group string) ([]TimeBucket, error) {
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, errors.New("invalid timezone")
	}
	switch group {
	case "day", "week", "month":
	default:
		return nil, errors.New("invalid grouping")
	}
	unit := group
	if from.IsZero() {
		from = time.Now().UTC().AddDate(0, -1, 0)
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if !to.After(from) || to.Sub(from) > 366*24*time.Hour {
		return nil, errors.New("invalid date range")
	}
	var items []TimeBucket
	err := s.db.NewRaw(`SELECT (date_trunc(?, timezone(?, started_at)) AT TIME ZONE ?) AS period,COALESCE(sum(active_seconds),0) AS active_seconds,COALESCE(sum(idle_seconds),0) AS idle_seconds,count(*) AS session_count,COALESCE(sum(words_read_estimate),0) AS words_read FROM reading_sessions WHERE user_id=? AND started_at>=? AND started_at<? AND status IN ('finished','stale','finalized') GROUP BY 1 ORDER BY 1`, unit, timezone, timezone, userID, from, to).Scan(ctx, &items)
	return items, err
}
func (s *Service) Books(ctx context.Context, userID uuid.UUID, limit, offset int) ([]BookStat, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var items []BookStat
	err := s.db.NewRaw(`SELECT b.id AS book_id,b.title,b.format,b.language,COALESCE(rp.progress_percent,0) AS progress_percent,COALESCE(sum(rs.active_seconds),0) AS active_seconds,COALESCE(sum(rs.idle_seconds),0) AS idle_seconds,count(rs.id) AS session_count,COALESCE(sum(rs.words_read_estimate),0) AS words_read,max(rs.started_at) AS last_read_at FROM books b LEFT JOIN reading_progress rp ON rp.book_id=b.id AND rp.user_id=b.user_id LEFT JOIN reading_sessions rs ON rs.book_id=b.id AND rs.user_id=b.user_id AND rs.status IN ('finished','stale','finalized') WHERE b.user_id=? AND b.deleted_at IS NULL GROUP BY b.id,rp.progress_percent ORDER BY last_read_at DESC NULLS LAST,b.created_at DESC LIMIT ? OFFSET ?`, userID, limit, offset).Scan(ctx, &items)
	return items, err
}
func (s *Service) Sessions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]model.ReadingSession, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	var items []model.ReadingSession
	err := s.db.NewSelect().Model(&items).Where("user_id=?", userID).Order("started_at DESC").Limit(limit).Offset(offset).Scan(ctx)
	return items, err
}
func (s *Service) Streak(ctx context.Context, userID uuid.UUID, timezone string) (Streak, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return Streak{}, errors.New("invalid timezone")
	}
	var dates []time.Time
	err = s.db.NewRaw(`SELECT DISTINCT timezone(?,started_at)::date AS date FROM reading_sessions WHERE user_id=? AND active_seconds>0 AND status IN ('finished','stale','finalized') ORDER BY date`, timezone, userID).Scan(ctx, &dates)
	if err != nil {
		return Streak{}, err
	}
	result := Streak{Dates: make([]string, 0, len(dates))}
	longest, currentRun := 0, 0
	var previous time.Time
	for _, date := range dates {
		local := date.In(loc)
		result.Dates = append(result.Dates, local.Format("2006-01-02"))
		if previous.IsZero() || sameDate(local, previous.AddDate(0, 0, 1)) {
			currentRun++
		} else {
			currentRun = 1
		}
		if currentRun > longest {
			longest = currentRun
		}
		previous = local
	}
	result.Longest = longest
	today := time.Now().In(loc)
	if !previous.IsZero() && (sameDate(previous, today) || sameDate(previous, today.AddDate(0, 0, -1))) {
		result.Current = currentRun
	}
	return result, nil
}
func (s *Service) Dictionary(ctx context.Context, userID uuid.UUID) (DictionaryStat, error) {
	var d DictionaryStat
	err := s.db.NewRaw(`SELECT count(*) AS total,count(*) FILTER(WHERE status='unknown') AS unknown,count(*) FILTER(WHERE status='learning') AS learning,count(*) FILTER(WHERE status='known') AS known,count(*) FILTER(WHERE status='mastered') AS mastered,count(*) FILTER(WHERE status='ignored') AS ignored,COALESCE(sum(encounter_count),0) AS encounters FROM dictionary_entries WHERE user_id=? AND deleted_at IS NULL`, userID).Scan(ctx, &d)
	return d, err
}
func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
func (s *Service) Recompute(ctx context.Context) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var locked bool
		if err := tx.NewSelect().ColumnExpr("pg_try_advisory_xact_lock(42424201)").Scan(ctx, &locked); err != nil {
			return err
		}
		if !locked {
			return nil
		}
		if _, err := tx.ExecContext(ctx, `TRUNCATE daily_reading_stats,book_reading_stats`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO book_reading_stats(user_id,book_id,active_seconds,idle_seconds,session_count,words_read,pages_read,last_read_at,updated_at) SELECT user_id,book_id,sum(active_seconds),sum(idle_seconds),count(*),sum(words_read_estimate),sum(pages_read_estimate),max(started_at),now() FROM reading_sessions WHERE status IN ('finished','stale','finalized') GROUP BY user_id,book_id`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO daily_reading_stats(user_id,local_date,timezone,active_seconds,idle_seconds,session_count,words_read,pages_read,updated_at) SELECT rs.user_id,timezone(u.timezone,rs.started_at)::date,u.timezone,sum(rs.active_seconds),sum(rs.idle_seconds),count(*),sum(rs.words_read_estimate),sum(rs.pages_read_estimate),now() FROM reading_sessions rs JOIN users u ON u.id=rs.user_id WHERE rs.status IN ('finished','stale','finalized') GROUP BY rs.user_id,timezone(u.timezone,rs.started_at)::date,u.timezone`)
		return err
	})
}
func (s *Service) CleanupExpiredTranslations(ctx context.Context) (int64, error) {
	res, err := s.db.NewDelete().Table("translation_cache").Where("expires_at < now() OR invalidated_at IS NOT NULL").Exec(ctx)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
