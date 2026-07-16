package translation

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/stretchr/testify/require"
)

type memoryCache struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (c *memoryCache) Get(_ context.Context, k string, d any) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(v, d)
}
func (c *memoryCache) Put(_ context.Context, k, _ string, _ Request, _ Provider, v any, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	raw, _ := json.Marshal(v)
	c.m[k] = raw
	return nil
}
func TestTranslationCacheAvoidsProviderCall(t *testing.T) {
	p := NewMockProvider("mock-v1")
	cache := &memoryCache{m: map[string][]byte{}}
	s := NewService(p, cache, config.Translation{MaxWordRunes: 128, MaxTextRunes: 1000, CacheTTL: time.Hour, PromptVersion: "v1", RequestTimeout: time.Second})
	r := Request{Text: "Hello", SourceLanguage: "en", TargetLanguage: "ru"}
	first, err := s.TranslateWord(context.Background(), r)
	require.NoError(t, err)
	second, err := s.TranslateWord(context.Background(), r)
	require.NoError(t, err)
	require.Equal(t, first.Translation, second.Translation)
	require.True(t, second.Cached)
	require.Equal(t, 1, p.Calls)
}
