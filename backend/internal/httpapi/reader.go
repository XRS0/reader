package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/internal/preferences"
	"github.com/XRS0/reader/backend/internal/reading"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) registerReaderRoutes(r *gin.RouterGroup) {
	r.GET("/books/:bookId/progress", s.getProgress)
	r.PUT("/books/:bookId/progress", s.putProgress)
	r.GET("/reader/preferences", s.getPreferences)
	r.PUT("/reader/preferences", s.putPreferences)
	r.GET("/books/:bookId/reader-preferences", s.getBookPreferences)
	r.PUT("/books/:bookId/reader-preferences", s.putBookPreferences)
	r.POST("/reading-sessions", s.startSession)
	r.POST("/reading-sessions/:sessionId/heartbeat", s.heartbeat)
	r.POST("/reading-sessions/:sessionId/finish", s.finishSession)
	r.GET("/reading-sessions", s.listSessions)
	r.GET("/reading-sessions/:sessionId", s.getSession)
}
func (s *Server) getProgress(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	item, err := s.services.Progress.Get(c.Request.Context(), userID(c), bookID)
	if err != nil {
		s.readingError(c, err)
		return
	}
	c.Header("ETag", `W/"`+strconv.FormatInt(item.Revision, 10)+`"`)
	c.JSON(http.StatusOK, progressDTO(item))
}
func (s *Server) putProgress(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	var in reading.ProgressInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	if header := c.GetHeader("If-Match"); header != "" {
		var revision int64
		if _, err := fmt.Sscanf(header, `W/"%d"`, &revision); err == nil {
			in.Revision = revision
		}
	}
	item, err := s.services.Progress.Put(c.Request.Context(), userID(c), bookID, in)
	if err != nil {
		s.readingError(c, err)
		return
	}
	c.Header("ETag", `W/"`+strconv.FormatInt(item.Revision, 10)+`"`)
	c.JSON(http.StatusOK, progressDTO(item))
}
func (s *Server) getPreferences(c *gin.Context) {
	item, err := s.services.Preferences.Get(c.Request.Context(), userID(c))
	if err != nil {
		fail(c, http.StatusInternalServerError, "PREFERENCES_FAILED", "Could not load reader preferences", nil)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) putPreferences(c *gin.Context) {
	var in preferences.Preferences
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Preferences.Put(c.Request.Context(), userID(c), in)
	if err != nil {
		fail(c, http.StatusBadRequest, "INVALID_PREFERENCES", err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) getBookPreferences(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	item, err := s.services.Preferences.GetBook(c.Request.Context(), userID(c), bookID)
	if err != nil {
		fail(c, http.StatusNotFound, "BOOK_NOT_FOUND", "Book was not found", nil)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) putBookPreferences(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	var in preferences.Preferences
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Preferences.PutBook(c.Request.Context(), userID(c), bookID, in)
	if err != nil {
		fail(c, http.StatusBadRequest, "INVALID_PREFERENCES", err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, item)
}

type startSessionRequest struct {
	BookID               uuid.UUID       `json:"book_id"`
	ChapterID            *uuid.UUID      `json:"chapter_id"`
	DeviceID             *uuid.UUID      `json:"device_id"`
	ClientID             string          `json:"client_id"`
	ClientTimestamp      time.Time       `json:"client_timestamp"`
	Locator              json.RawMessage `json:"locator"`
	StartLocator         json.RawMessage `json:"start_locator"`
	ProgressPercent      float64         `json:"progress_percent"`
	StartProgressPercent *float64        `json:"start_progress_percent"`
}

func (s *Server) startSession(c *gin.Context) {
	var req startSessionRequest
	if !decodeJSON(c, &req, 64*1024) {
		return
	}
	locator := req.Locator
	if len(req.StartLocator) > 0 {
		locator = req.StartLocator
	}
	progress := req.ProgressPercent
	if req.StartProgressPercent != nil {
		progress = *req.StartProgressPercent
	}
	item, err := s.services.Sessions.Start(c.Request.Context(), userID(c), reading.StartSessionInput{BookID: req.BookID, DeviceID: req.DeviceID, Locator: locator, ProgressPercent: progress})
	if err != nil {
		s.readingError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sessionDTO(item))
}

type heartbeatRequest struct {
	Locator                      json.RawMessage `json:"locator"`
	ProgressPercent              float64         `json:"progress_percent"`
	TabVisible                   *bool           `json:"tab_visible"`
	Visible                      *bool           `json:"visible"`
	WindowFocused                *bool           `json:"window_focused"`
	Focused                      *bool           `json:"focused"`
	UserActive                   bool            `json:"user_active"`
	MillisecondsSinceInteraction *int64          `json:"milliseconds_since_interaction"`
	LastInteractionMS            *int64          `json:"last_interaction_ms"`
	ClientTimestamp              time.Time       `json:"client_timestamp"`
	IdempotencyKey               string          `json:"idempotency_key"`
	SequenceNumber               *int64          `json:"sequence_number"`
	Sequence                     *int64          `json:"sequence"`
	CharactersRead               int64           `json:"characters_read"`
}

func (s *Server) heartbeat(c *gin.Context) {
	id, ok := parseID(c, "sessionId")
	if !ok {
		return
	}
	var req heartbeatRequest
	if !decodeJSON(c, &req, 64*1024) {
		return
	}
	visible := false
	if req.TabVisible != nil {
		visible = *req.TabVisible
	} else if req.Visible != nil {
		visible = *req.Visible
	}
	focused := false
	if req.WindowFocused != nil {
		focused = *req.WindowFocused
	} else if req.Focused != nil {
		focused = *req.Focused
	}
	interaction := int64(0)
	if req.MillisecondsSinceInteraction != nil {
		interaction = *req.MillisecondsSinceInteraction
	} else if req.LastInteractionMS != nil {
		interaction = *req.LastInteractionMS
	}
	sequence := int64(0)
	if req.SequenceNumber != nil {
		sequence = *req.SequenceNumber
	} else if req.Sequence != nil {
		sequence = *req.Sequence
	}
	key := req.IdempotencyKey
	if key == "" {
		key = c.GetHeader("Idempotency-Key")
	}
	item, err := s.services.Sessions.Heartbeat(c.Request.Context(), userID(c), id, reading.HeartbeatInput{Locator: req.Locator, ProgressPercent: req.ProgressPercent, Visible: visible, Focused: focused, UserActive: req.UserActive, LastInteractionMS: interaction, ClientTimestamp: req.ClientTimestamp, IdempotencyKey: key, Sequence: sequence, CharactersRead: req.CharactersRead})
	if err != nil {
		s.readingError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionDTO(item))
}
func (s *Server) finishSession(c *gin.Context) {
	id, ok := parseID(c, "sessionId")
	if !ok {
		return
	}
	var req struct {
		Locator         json.RawMessage `json:"locator"`
		ProgressPercent float64         `json:"progress_percent"`
		CloseReason     string          `json:"close_reason"`
		Reason          string          `json:"reason"`
		IdempotencyKey  string          `json:"idempotency_key"`
		Sequence        int64           `json:"sequence"`
		SequenceNumber  int64           `json:"sequence_number"`
		ClientTimestamp time.Time       `json:"client_timestamp"`
	}
	if !decodeJSON(c, &req, 64*1024) {
		return
	}
	in := reading.FinishInput{Locator: req.Locator, ProgressPercent: req.ProgressPercent, CloseReason: req.CloseReason, IdempotencyKey: req.IdempotencyKey, Sequence: req.Sequence}
	if in.CloseReason == "" {
		in.CloseReason = req.Reason
	}
	if req.SequenceNumber > 0 {
		in.Sequence = req.SequenceNumber
	}
	if in.IdempotencyKey == "" {
		in.IdempotencyKey = c.GetHeader("Idempotency-Key")
	}
	_, err := s.services.Sessions.Finish(c.Request.Context(), userID(c), id, in)
	if err != nil {
		s.readingError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) listSessions(c *gin.Context) {
	limit, offset := parsePage(c)
	var bookID *uuid.UUID
	if raw := c.Query("book_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			fail(c, http.StatusBadRequest, "INVALID_ID", "book_id is invalid", nil)
			return
		}
		bookID = &id
	}
	items, err := s.services.Sessions.List(c.Request.Context(), userID(c), bookID, limit, offset)
	if err != nil {
		s.readingError(c, err)
		return
	}
	dtos := make([]gin.H, len(items))
	for i, item := range items {
		dtos[i] = sessionDTO(item)
	}
	var next *string
	if len(items) == limit {
		value := strconv.Itoa(offset + len(items))
		next = &value
	}
	c.JSON(http.StatusOK, gin.H{"items": dtos, "has_more": len(items) == limit, "next_cursor": next})
}
func (s *Server) getSession(c *gin.Context) {
	id, ok := parseID(c, "sessionId")
	if !ok {
		return
	}
	item, err := s.services.Sessions.Get(c.Request.Context(), userID(c), id)
	if err != nil {
		s.readingError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionDTO(item))
}
func sessionDTO(item model.ReadingSession) gin.H {
	result := gin.H{"id": item.ID, "book_id": item.BookID, "started_at": item.StartedAt, "last_activity_at": item.LastActivityAt, "last_heartbeat_at": item.LastHeartbeatAt, "ended_at": item.EndedAt, "active_seconds": item.ActiveSeconds, "idle_seconds": item.IdleSeconds, "start_progress_percent": item.StartProgressPercent, "end_progress_percent": item.EndProgressPercent, "words_read_estimate": item.WordsReadEstimate, "pages_read_estimate": item.PagesReadEstimate, "status": item.Status}
	if item.DeviceID != nil {
		result["device_id"] = item.DeviceID
	}
	if item.CloseReason != "" {
		result["close_reason"] = item.CloseReason
	}
	return result
}
func progressDTO(item model.ReadingProgress) gin.H {
	locator := string(item.Locator)
	var decoded string
	if json.Unmarshal(item.Locator, &decoded) == nil {
		locator = decoded
	}
	var chapterID any
	if item.ChapterID != nil {
		chapterID = item.ChapterID.String()
	}
	result := gin.H{"book_id": item.BookID, "chapter_id": chapterID, "locator_type": item.LocatorType, "locator": locator, "character_offset": item.CharacterOffset, "text_anchor": item.TextAnchor, "progress_percent": item.ProgressPercent, "scroll_percent": item.ScrollPercent, "revision": item.Revision, "client_id": item.ClientID, "updated_at": item.UpdatedAt, "server_timestamp": item.UpdatedAt}
	if item.DeviceID != nil {
		result["device_id"] = item.DeviceID
	}
	return result
}
func (s *Server) readingError(c *gin.Context, err error) {
	var conflict *reading.ConflictError
	switch {
	case errors.As(err, &conflict):
		fail(c, http.StatusConflict, "PROGRESS_CONFLICT", "A newer reading position already exists", map[string]any{"current": progressDTO(conflict.Current)})
	case errors.Is(err, reading.ErrNotFound):
		fail(c, http.StatusNotFound, "READING_RESOURCE_NOT_FOUND", "Reading resource was not found", nil)
	case errors.Is(err, reading.ErrSequence):
		fail(c, http.StatusConflict, "STALE_HEARTBEAT", "Heartbeat sequence is stale", nil)
	case errors.Is(err, reading.ErrSessionFinished):
		fail(c, http.StatusConflict, "SESSION_FINISHED", "Reading session is already finished", nil)
	default:
		fail(c, http.StatusBadRequest, "READING_REQUEST_INVALID", err.Error(), nil)
	}
}
