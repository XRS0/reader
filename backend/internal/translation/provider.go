package translation

import (
	"context"
	"errors"
	"strings"
	"sync"
)

type Request struct {
	SourceLanguage     string `json:"source_language"`
	TargetLanguage     string `json:"target_language"`
	Text               string `json:"text"`
	Context            string `json:"context,omitempty"`
	SurroundingContext string `json:"surrounding_context,omitempty"`
	BookID             string `json:"book_id,omitempty"`
	ChapterID          string `json:"chapter_id,omitempty"`
	Locator            any    `json:"locator,omitempty"`
}
type WordResult struct {
	Original      string   `json:"original"`
	Normalized    string   `json:"normalized"`
	Lemma         string   `json:"lemma"`
	Translation   string   `json:"translation"`
	Transcription string   `json:"transcription,omitempty"`
	PartOfSpeech  string   `json:"part_of_speech,omitempty"`
	Definition    string   `json:"definition,omitempty"`
	Alternatives  []string `json:"alternatives,omitempty"`
	Example       string   `json:"example,omitempty"`
	Language      string   `json:"language"`
	Confidence    float64  `json:"confidence,omitempty"`
	Cached        bool     `json:"cached"`
}
type TextResult struct {
	Original         string `json:"original"`
	Translation      string `json:"translation"`
	DetectedLanguage string `json:"detected_language"`
	Explanation      string `json:"explanation,omitempty"`
	Cached           bool   `json:"cached"`
}
type DetectionResult struct {
	Language   string  `json:"language"`
	Confidence float64 `json:"confidence"`
}
type Provider interface {
	TranslateWord(context.Context, Request) (WordResult, error)
	TranslateText(context.Context, Request) (TextResult, error)
	DetectLanguage(context.Context, string) (DetectionResult, error)
	Name() string
	Model() string
}
type MockProvider struct {
	model string
	Calls int
	mu    sync.Mutex
}

func NewMockProvider(model string) *MockProvider {
	if model == "" {
		model = "mock-v1"
	}
	return &MockProvider{model: model}
}
func (m *MockProvider) Name() string  { return "mock" }
func (m *MockProvider) Model() string { return m.model }
func (m *MockProvider) TranslateWord(_ context.Context, r Request) (WordResult, error) {
	m.mu.Lock()
	m.Calls++
	m.mu.Unlock()
	n := Normalize(r.Text)
	if n == "" {
		return WordResult{}, errors.New("empty text")
	}
	known := map[string]string{"hello": "привет", "world": "мир", "book": "книга", "reader": "читатель"}
	translated := known[n]
	if translated == "" {
		translated = "[" + r.TargetLanguage + "] " + n
	}
	return WordResult{Original: r.Text, Normalized: n, Lemma: n, Translation: translated, Language: r.SourceLanguage, Confidence: 1}, nil
}
func (m *MockProvider) TranslateText(_ context.Context, r Request) (TextResult, error) {
	m.mu.Lock()
	m.Calls++
	m.mu.Unlock()
	if Normalize(r.Text) == "" {
		return TextResult{}, errors.New("empty text")
	}
	return TextResult{Original: r.Text, Translation: "[" + r.TargetLanguage + "] " + strings.TrimSpace(r.Text), DetectedLanguage: r.SourceLanguage}, nil
}
func (m *MockProvider) DetectLanguage(_ context.Context, text string) (DetectionResult, error) {
	m.mu.Lock()
	m.Calls++
	m.mu.Unlock()
	for _, r := range text {
		if r >= 'А' && r <= 'я' {
			return DetectionResult{Language: "ru", Confidence: .9}, nil
		}
	}
	return DetectionResult{Language: "en", Confidence: .8}, nil
}
