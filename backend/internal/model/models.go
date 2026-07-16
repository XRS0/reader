package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`
	ID            uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	DisplayName   string     `json:"display_name"`
	Timezone      string     `json:"timezone"`
	Locale        string     `json:"locale"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"-"`
}
type Device struct {
	bun.BaseModel `bun:"table:devices,alias:d"`
	ID            uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	UserID        uuid.UUID  `bun:"type:uuid" json:"-"`
	DeviceKey     string     `json:"device_key"`
	Name          string     `json:"name"`
	UserAgent     string     `json:"user_agent"`
	LastSeenAt    time.Time  `json:"last_seen_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}
type RefreshToken struct {
	bun.BaseModel `bun:"table:refresh_tokens,alias:rt"`
	ID            uuid.UUID `bun:",pk,type:uuid"`
	UserID        uuid.UUID `bun:"type:uuid"`
	DeviceID      uuid.UUID `bun:"type:uuid"`
	FamilyID      uuid.UUID `bun:"type:uuid"`
	TokenHash     []byte
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	ReplacedBy    *uuid.UUID `bun:"type:uuid"`
	CreatedAt     time.Time
	LastUsedAt    *time.Time
}

type Book struct {
	bun.BaseModel     `bun:"table:books,alias:b"`
	ID                uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	UserID            uuid.UUID       `bun:"type:uuid" json:"-"`
	Title             string          `json:"title"`
	Author            string          `json:"author"`
	Language          string          `json:"language"`
	Description       string          `json:"description"`
	Format            string          `json:"format"`
	Status            string          `json:"status"`
	SHA256            string          `json:"sha256"`
	OriginalFilename  string          `json:"original_filename"`
	OriginalMIME      string          `json:"original_mime"`
	OriginalSize      int64           `json:"original_size"`
	OriginalBucket    string          `json:"-"`
	OriginalKey       string          `json:"-"`
	CoverBucket       string          `json:"-"`
	CoverKey          string          `json:"-"`
	CustomCoverBucket string          `json:"-"`
	CustomCoverKey    string          `json:"-"`
	ProcessingVersion int             `json:"processing_version"`
	ProcessingError   string          `json:"processing_error,omitempty"`
	IsFavorite        bool            `json:"is_favorite"`
	Metadata          json.RawMessage `bun:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         *time.Time      `json:"-"`
	Tags              []string        `bun:"-" json:"tags"`
	ProgressPercent   float64         `bun:"-" json:"progress_percent"`
	CurrentChapterID  *uuid.UUID      `bun:"-" json:"current_chapter_id,omitempty"`
	LastReadAt        *time.Time      `bun:"-" json:"last_read_at,omitempty"`
}
type BookChapter struct {
	bun.BaseModel  `bun:"table:book_chapters,alias:bc"`
	ID             uuid.UUID `bun:",pk,type:uuid" json:"id"`
	BookID         uuid.UUID `bun:"type:uuid" json:"book_id"`
	Version        int       `json:"version"`
	Ordinal        int       `json:"ordinal"`
	Title          string    `json:"title"`
	SourceRef      string    `json:"source_ref"`
	ContentHTML    string    `json:"content_html,omitempty"`
	ContentText    string    `json:"content_text,omitempty"`
	ContentBucket  string    `json:"-"`
	ContentKey     string    `json:"-"`
	CharacterCount int       `json:"character_count"`
	WordCount      int       `json:"word_count"`
	CreatedAt      time.Time `json:"created_at"`
}
type BookAsset struct {
	bun.BaseModel `bun:"table:book_assets,alias:ba"`
	ID            uuid.UUID `bun:",pk,type:uuid"`
	BookID        uuid.UUID `bun:"type:uuid"`
	Version       int
	SourceRef     string
	MediaType     string
	Bucket        string
	ObjectKey     string
	Size          int64
	SHA256        string
	CreatedAt     time.Time
}
type Job struct {
	bun.BaseModel `bun:"table:book_processing_jobs,alias:j"`
	ID            uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	Type          string          `json:"type"`
	Payload       json.RawMessage `bun:"type:jsonb" json:"payload"`
	Status        string          `json:"status"`
	Priority      int             `json:"priority"`
	Attempts      int             `json:"attempts"`
	MaxAttempts   int             `json:"max_attempts"`
	RunAfter      time.Time       `json:"run_after"`
	LockedAt      *time.Time      `json:"locked_at"`
	LockedBy      string          `json:"locked_by"`
	LastError     string          `json:"last_error"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	FinishedAt    *time.Time      `json:"finished_at"`
}

type ReadingProgress struct {
	bun.BaseModel   `bun:"table:reading_progress,alias:rp"`
	ID              uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	UserID          uuid.UUID       `bun:"type:uuid" json:"-"`
	BookID          uuid.UUID       `bun:"type:uuid" json:"book_id"`
	ChapterID       *uuid.UUID      `bun:"type:uuid" json:"chapter_id,omitempty"`
	LocatorType     string          `json:"locator_type"`
	Locator         json.RawMessage `bun:"type:jsonb" json:"locator"`
	CharacterOffset int64           `json:"character_offset"`
	TextAnchor      string          `json:"text_anchor"`
	ChapterProgress float64         `json:"chapter_progress"`
	ProgressPercent float64         `json:"progress_percent"`
	ScrollPercent   float64         `json:"scroll_percent"`
	Revision        int64           `json:"revision"`
	ClientID        string          `json:"client_id"`
	DeviceID        *uuid.UUID      `bun:"type:uuid" json:"device_id,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
type ReadingSession struct {
	bun.BaseModel        `bun:"table:reading_sessions,alias:rs"`
	ID                   uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	UserID               uuid.UUID       `bun:"type:uuid" json:"-"`
	BookID               uuid.UUID       `bun:"type:uuid" json:"book_id"`
	DeviceID             *uuid.UUID      `bun:"type:uuid" json:"device_id,omitempty"`
	StartedAt            time.Time       `json:"started_at"`
	LastActivityAt       time.Time       `json:"last_activity_at"`
	LastHeartbeatAt      time.Time       `json:"last_heartbeat_at"`
	EndedAt              *time.Time      `json:"ended_at,omitempty"`
	ActiveSeconds        int64           `json:"active_seconds"`
	IdleSeconds          int64           `json:"idle_seconds"`
	StartLocator         json.RawMessage `bun:"type:jsonb" json:"start_locator"`
	EndLocator           json.RawMessage `bun:"type:jsonb" json:"end_locator"`
	StartProgressPercent float64         `json:"start_progress_percent"`
	EndProgressPercent   float64         `json:"end_progress_percent"`
	CharactersRead       int64           `json:"characters_read"`
	WordsReadEstimate    int64           `json:"words_read_estimate"`
	PagesReadEstimate    float64         `json:"pages_read_estimate"`
	CloseReason          string          `json:"close_reason"`
	Status               string          `json:"status"`
	LastSequence         int64           `json:"last_sequence"`
	LastWasActive        bool            `json:"-"`
	CreatedAt            time.Time       `json:"created_at"`
	UpdatedAt            time.Time       `json:"updated_at"`
}
type ReadingEvent struct {
	bun.BaseModel  `bun:"table:reading_events,alias:re"`
	ID             uuid.UUID `bun:",pk,type:uuid"`
	SessionID      uuid.UUID `bun:"type:uuid"`
	UserID         uuid.UUID `bun:"type:uuid"`
	BookID         uuid.UUID `bun:"type:uuid"`
	Type           string
	OccurredAt     time.Time
	ReceivedAt     time.Time
	IdempotencyKey string
	SequenceNumber int64
	Metadata       json.RawMessage `bun:"type:jsonb"`
}

type ReaderPreferences struct {
	bun.BaseModel     `bun:"table:reader_preferences,alias:pref"`
	UserID            uuid.UUID `bun:",pk,type:uuid" json:"-"`
	Theme             string    `json:"theme"`
	BackgroundColor   string    `json:"background_color"`
	TextColor         string    `json:"text_color"`
	AccentColor       string    `json:"accent_color"`
	FontFamily        string    `json:"font_family"`
	FontSize          float64   `json:"font_size"`
	FontWeight        int       `json:"font_weight"`
	LineHeight        float64   `json:"line_height"`
	LetterSpacing     float64   `json:"letter_spacing"`
	ContentWidth      int       `json:"content_width"`
	MarginSize        int       `json:"margin_size"`
	TextAlign         string    `json:"text_align"`
	NavigationMode    string    `json:"navigation_mode"`
	ShowProgress      bool      `json:"show_progress"`
	ShowRemainingTime bool      `json:"show_remaining_time"`
	UIIntensity       float64   `json:"ui_intensity"`
	UpdatedAt         time.Time `json:"updated_at"`
}
type BookReaderPreferences struct {
	ReaderPreferences
	BookID uuid.UUID `bun:",pk,type:uuid" json:"book_id"`
}

type TranslationCache struct {
	bun.BaseModel  `bun:"table:translation_cache,alias:tc"`
	ID             uuid.UUID `bun:",pk,type:uuid"`
	CacheKey       string
	RequestType    string
	SourceLanguage string
	TargetLanguage string
	NormalizedText string
	Provider       string
	ProviderModel  string
	PromptVersion  string
	Result         json.RawMessage `bun:"type:jsonb"`
	ResultVersion  int
	UseCount       int64
	CreatedAt      time.Time
	LastUsedAt     time.Time
	ExpiresAt      time.Time
	InvalidatedAt  *time.Time
}
type DictionaryEntry struct {
	bun.BaseModel           `bun:"table:dictionary_entries,alias:de"`
	ID                      uuid.UUID  `bun:",pk,type:uuid" json:"id"`
	UserID                  uuid.UUID  `bun:"type:uuid" json:"-"`
	SourceLanguage          string     `json:"source_language"`
	TargetLanguage          string     `json:"target_language"`
	OriginalWord            string     `json:"original_word"`
	NormalizedWord          string     `json:"normalized_word"`
	Lemma                   string     `json:"lemma"`
	Transcription           string     `json:"transcription"`
	PartOfSpeech            string     `json:"part_of_speech"`
	Translation             string     `json:"translation"`
	AlternativeTranslations []string   `bun:"type:jsonb" json:"alternative_translations"`
	Definition              string     `json:"definition"`
	Note                    string     `json:"note"`
	Status                  string     `json:"status"`
	EncounterCount          int        `json:"encounter_count"`
	FirstSeenAt             time.Time  `json:"first_seen_at"`
	LastSeenAt              time.Time  `json:"last_seen_at"`
	NextReviewAt            *time.Time `json:"next_review_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
	DeletedAt               *time.Time `json:"deleted_at,omitempty"`
}
type WordOccurrence struct {
	bun.BaseModel     `bun:"table:word_occurrences,alias:wo"`
	ID                uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	DictionaryEntryID uuid.UUID       `bun:"type:uuid" json:"dictionary_entry_id"`
	BookID            *uuid.UUID      `bun:"type:uuid" json:"book_id,omitempty"`
	ChapterID         *uuid.UUID      `bun:"type:uuid" json:"chapter_id,omitempty"`
	Locator           json.RawMessage `bun:"type:jsonb" json:"locator"`
	Sentence          string          `json:"sentence"`
	ContextBefore     string          `json:"context_before"`
	ContextAfter      string          `json:"context_after"`
	EncounteredAt     time.Time       `json:"encountered_at"`
	CreatedAt         time.Time       `json:"created_at"`
}

type Bookmark struct {
	bun.BaseModel   `bun:"table:bookmarks,alias:bm"`
	ID              uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	UserID          uuid.UUID       `bun:"type:uuid" json:"-"`
	BookID          uuid.UUID       `bun:"type:uuid" json:"book_id"`
	ChapterID       *uuid.UUID      `bun:"type:uuid" json:"chapter_id,omitempty"`
	Locator         json.RawMessage `bun:"type:jsonb" json:"locator"`
	ProgressPercent float64         `json:"progress_percent"`
	Title           string          `json:"title"`
	Note            string          `json:"note"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
type Highlight struct {
	bun.BaseModel `bun:"table:highlights,alias:h"`
	ID            uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	UserID        uuid.UUID       `bun:"type:uuid" json:"-"`
	BookID        uuid.UUID       `bun:"type:uuid" json:"book_id"`
	ChapterID     *uuid.UUID      `bun:"type:uuid" json:"chapter_id,omitempty"`
	Locator       json.RawMessage `bun:"type:jsonb" json:"locator"`
	TextAnchor    string          `json:"text_anchor"`
	SelectedText  string          `json:"selected_text"`
	Context       string          `json:"context"`
	Color         string          `json:"color"`
	Note          string          `json:"note"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
type Note struct {
	bun.BaseModel `bun:"table:notes,alias:n"`
	ID            uuid.UUID       `bun:",pk,type:uuid" json:"id"`
	UserID        uuid.UUID       `bun:"type:uuid" json:"-"`
	BookID        *uuid.UUID      `bun:"type:uuid" json:"book_id,omitempty"`
	HighlightID   *uuid.UUID      `bun:"type:uuid" json:"highlight_id,omitempty"`
	Title         string          `json:"title"`
	SchemaVersion int             `json:"schema_version"`
	Blocks        json.RawMessage `bun:"type:jsonb" json:"blocks"`
	SearchText    string          `json:"-"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     *time.Time      `json:"deleted_at,omitempty"`
}
