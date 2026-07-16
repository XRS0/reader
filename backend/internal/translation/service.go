package translation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Cache interface {
	Get(context.Context, string, any) (bool, error)
	Put(context.Context, string, string, Request, Provider, any, time.Duration) error
}
type PostgresCache struct {
	db            *bun.DB
	now           func() time.Time
	promptVersion string
}

func NewPostgresCache(db *bun.DB, promptVersion ...string) *PostgresCache {
	version := "v1"
	if len(promptVersion) > 0 && promptVersion[0] != "" {
		version = promptVersion[0]
	}
	return &PostgresCache{db: db, now: time.Now, promptVersion: version}
}
func (c *PostgresCache) Get(ctx context.Context, key string, dest any) (bool, error) {
	var entry model.TranslationCache
	err := c.db.NewSelect().Model(&entry).Where("cache_key=?", key).Where("invalidated_at IS NULL").Where("expires_at > ?", c.now().UTC()).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = json.Unmarshal(entry.Result, dest); err != nil {
		return false, err
	}
	_, _ = c.db.NewUpdate().Model((*model.TranslationCache)(nil)).Set("use_count=use_count+1").Set("last_used_at=?", c.now().UTC()).Where("id=?", entry.ID).Exec(ctx)
	return true, nil
}
func (c *PostgresCache) Put(ctx context.Context, key, kind string, r Request, p Provider, result any, ttl time.Duration) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	now := c.now().UTC()
	entry := model.TranslationCache{ID: uuid.New(), CacheKey: key, RequestType: kind, SourceLanguage: r.SourceLanguage, TargetLanguage: r.TargetLanguage, NormalizedText: Normalize(r.Text), Provider: p.Name(), ProviderModel: p.Model(), PromptVersion: c.promptVersion, Result: raw, ResultVersion: 1, UseCount: 1, CreatedAt: now, LastUsedAt: now, ExpiresAt: now.Add(ttl)}
	_, err = c.db.NewInsert().Model(&entry).On("CONFLICT (cache_key) DO UPDATE").Set("result=EXCLUDED.result").Set("expires_at=EXCLUDED.expires_at").Set("last_used_at=EXCLUDED.last_used_at").Set("use_count=tc.use_count+1").Exec(ctx)
	return err
}

type Service struct {
	provider Provider
	cache    Cache
	cfg      config.Translation
	mu       sync.Mutex
	flights  map[string]*flight
	breaker  *circuitBreaker
}
type flight struct {
	done  chan struct{}
	value any
	err   error
}

func NewService(provider Provider, cache Cache, cfg config.Translation) *Service {
	return &Service{provider: provider, cache: cache, cfg: cfg, flights: map[string]*flight{}, breaker: newCircuitBreaker(5, 30*time.Second)}
}
func Normalize(v string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return unicode.ToLower(r)
	}, strings.Join(strings.Fields(v), " ")))
}
func CacheKey(kind string, r Request, p Provider, prompt string) string {
	raw := strings.Join([]string{kind, Normalize(r.Text), strings.ToLower(r.SourceLanguage), strings.ToLower(r.TargetLanguage), p.Name(), p.Model(), prompt}, "\x1f")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func (s *Service) TranslateWord(ctx context.Context, r Request) (WordResult, error) {
	if n := len([]rune(strings.TrimSpace(r.Text))); n == 0 || n > s.cfg.MaxWordRunes {
		return WordResult{}, errors.New("word translation text length is invalid")
	}
	key := CacheKey("word", r, s.provider, s.cfg.PromptVersion)
	var cached WordResult
	hit, err := s.cache.Get(ctx, key, &cached)
	if err != nil {
		return cached, err
	}
	if hit {
		cached.Cached = true
		return cached, nil
	}
	v, err := s.single(ctx, key, func(ctx context.Context) (any, error) {
		var second WordResult
		if hit, err := s.cache.Get(ctx, key, &second); err != nil {
			return nil, err
		} else if hit {
			second.Cached = true
			return second, nil
		}
		callCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
		defer cancel()
		if !s.breaker.allow(time.Now()) {
			return nil, ErrCircuitOpen
		}
		result, err := s.provider.TranslateWord(callCtx, r)
		if err != nil {
			s.breaker.failure(time.Now())
			return nil, err
		}
		s.breaker.success()
		if err = s.cache.Put(ctx, key, "word", r, s.provider, result, s.cfg.CacheTTL); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return WordResult{}, err
	}
	return v.(WordResult), nil
}
func (s *Service) TranslateText(ctx context.Context, r Request) (TextResult, error) {
	if n := len([]rune(strings.TrimSpace(r.Text))); n == 0 || n > s.cfg.MaxTextRunes {
		return TextResult{}, errors.New("text translation length is invalid")
	}
	key := CacheKey("text", r, s.provider, s.cfg.PromptVersion)
	var cached TextResult
	hit, err := s.cache.Get(ctx, key, &cached)
	if err != nil {
		return cached, err
	}
	if hit {
		cached.Cached = true
		return cached, nil
	}
	v, err := s.single(ctx, key, func(ctx context.Context) (any, error) {
		var second TextResult
		if hit, err := s.cache.Get(ctx, key, &second); err != nil {
			return nil, err
		} else if hit {
			second.Cached = true
			return second, nil
		}
		callCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
		defer cancel()
		if !s.breaker.allow(time.Now()) {
			return nil, ErrCircuitOpen
		}
		result, err := s.provider.TranslateText(callCtx, r)
		if err != nil {
			s.breaker.failure(time.Now())
			return nil, err
		}
		s.breaker.success()
		if err = s.cache.Put(ctx, key, "text", r, s.provider, result, s.cfg.CacheTTL); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return TextResult{}, err
	}
	return v.(TextResult), nil
}
func (s *Service) Detect(ctx context.Context, text string) (DetectionResult, error) {
	if len([]rune(text)) == 0 || len([]rune(text)) > s.cfg.MaxTextRunes {
		return DetectionResult{}, errors.New("text length is invalid")
	}
	callCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	if !s.breaker.allow(time.Now()) {
		return DetectionResult{}, ErrCircuitOpen
	}
	result, err := s.provider.DetectLanguage(callCtx, text)
	if err != nil {
		s.breaker.failure(time.Now())
		return result, err
	}
	s.breaker.success()
	return result, nil
}
func (s *Service) single(ctx context.Context, key string, fn func(context.Context) (any, error)) (any, error) {
	s.mu.Lock()
	if f := s.flights[key]; f != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.done:
			return f.value, f.err
		}
	}
	f := &flight{done: make(chan struct{})}
	s.flights[key] = f
	s.mu.Unlock()
	f.value, f.err = fn(ctx)
	s.mu.Lock()
	delete(s.flights, key)
	close(f.done)
	s.mu.Unlock()
	return f.value, f.err
}

var ErrCircuitOpen = errors.New("translation provider circuit is open")

type circuitBreaker struct {
	mu        sync.Mutex
	failures  int
	threshold int
	openUntil time.Time
	halfOpen  bool
	cooldown  time.Duration
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{threshold: threshold, cooldown: cooldown}
}
func (b *circuitBreaker) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.openUntil.IsZero() {
		return true
	}
	if now.Before(b.openUntil) {
		return false
	}
	if b.halfOpen {
		return false
	}
	b.halfOpen = true
	return true
}
func (b *circuitBreaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.openUntil = time.Time{}
	b.halfOpen = false
}
func (b *circuitBreaker) failure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.halfOpen || b.failures >= b.threshold {
		b.openUntil = now.Add(b.cooldown)
		b.halfOpen = false
		b.failures = 0
	}
}
