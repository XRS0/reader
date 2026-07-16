package translation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIProviderRetriesAndDecodesBoundedStructuredOutput(t *testing.T) {
	var calls atomic.Int64
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-secret", r.Header.Get("Authorization"))
		call := calls.Add(1)
		if call == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		structured, _ := json.Marshal(WordResult{Original: "hello", Normalized: "hello", Lemma: "hello", Translation: "привет", Alternatives: []string{}, Language: "en", Confidence: .99})
		_ = json.NewEncoder(w).Encode(map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": string(structured)}}}}})
	}))
	defer server.Close()

	provider, err := NewOpenAIProvider(server.URL, "test-secret", "test-model", 2*time.Second, 1)
	require.NoError(t, err)
	result, err := provider.TranslateWord(context.Background(), Request{Text: "hello", SourceLanguage: "en", TargetLanguage: "ru", SurroundingContext: strings.Repeat("x", 5000)})
	require.NoError(t, err)
	require.Equal(t, "привет", result.Translation)
	require.Equal(t, int64(2), calls.Load())
	require.Less(t, strings.Count(string(receivedBody), "x"), 1100, "provider must bound external context")
	require.NotContains(t, string(receivedBody), "test-secret")
}

func TestOpenAIProviderErrorDoesNotLeakAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
	defer server.Close()
	provider, err := NewOpenAIProvider(server.URL, "highly-sensitive-key", "test-model", time.Second, 0)
	require.NoError(t, err)
	_, err = provider.DetectLanguage(context.Background(), "hello")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "highly-sensitive-key")
}
