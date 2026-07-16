package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/XRS0/reader/backend/internal/annotations"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) registerAnnotationRoutes(r *gin.RouterGroup) {
	r.GET("/books/:bookId/bookmarks", s.listBookmarks)
	r.POST("/books/:bookId/bookmarks", s.createBookmark)
	r.PATCH("/bookmarks/:bookmarkId", s.patchBookmark)
	r.DELETE("/bookmarks/:bookmarkId", s.deleteBookmark)
	r.GET("/books/:bookId/highlights", s.listHighlights)
	r.POST("/books/:bookId/highlights", s.createHighlight)
	r.PATCH("/highlights/:highlightId", s.patchHighlight)
	r.DELETE("/highlights/:highlightId", s.deleteHighlight)
	r.GET("/notes", s.listNotes)
	r.POST("/notes", s.createNote)
	r.GET("/notes/:noteId", s.getNote)
	r.PATCH("/notes/:noteId", s.patchNote)
	r.DELETE("/notes/:noteId", s.deleteNote)
}
func (s *Server) listBookmarks(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	items, err := s.services.Annotations.ListBookmarks(c.Request.Context(), userID(c), bookID)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	dtos := make([]gin.H, len(items))
	for i, item := range items {
		dtos[i] = bookmarkDTO(item)
	}
	c.JSON(http.StatusOK, gin.H{"items": dtos, "next_cursor": nil, "has_more": false, "total_count": len(dtos)})
}
func (s *Server) createBookmark(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	var in annotations.BookmarkInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Annotations.CreateBookmark(c.Request.Context(), userID(c), bookID, in)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusCreated, bookmarkDTO(item))
}
func (s *Server) patchBookmark(c *gin.Context) {
	id, ok := parseID(c, "bookmarkId")
	if !ok {
		return
	}
	var in annotations.BookmarkPatch
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Annotations.PatchBookmark(c.Request.Context(), userID(c), id, in)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusOK, bookmarkDTO(item))
}
func (s *Server) deleteBookmark(c *gin.Context) {
	id, ok := parseID(c, "bookmarkId")
	if !ok {
		return
	}
	if err := s.services.Annotations.DeleteBookmark(c.Request.Context(), userID(c), id); err != nil {
		s.annotationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) listHighlights(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	items, err := s.services.Annotations.ListHighlights(c.Request.Context(), userID(c), bookID)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	dtos := make([]gin.H, len(items))
	for i, item := range items {
		dtos[i] = highlightDTO(item)
	}
	c.JSON(http.StatusOK, gin.H{"items": dtos, "next_cursor": nil, "has_more": false, "total_count": len(dtos)})
}
func (s *Server) createHighlight(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	var in annotations.HighlightInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Annotations.CreateHighlight(c.Request.Context(), userID(c), bookID, in)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusCreated, highlightDTO(item))
}
func (s *Server) patchHighlight(c *gin.Context) {
	id, ok := parseID(c, "highlightId")
	if !ok {
		return
	}
	var in annotations.HighlightPatch
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Annotations.PatchHighlight(c.Request.Context(), userID(c), id, in)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusOK, highlightDTO(item))
}
func (s *Server) deleteHighlight(c *gin.Context) {
	id, ok := parseID(c, "highlightId")
	if !ok {
		return
	}
	if err := s.services.Annotations.DeleteHighlight(c.Request.Context(), userID(c), id); err != nil {
		s.annotationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) listNotes(c *gin.Context) {
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
	items, err := s.services.Annotations.ListNotes(c.Request.Context(), userID(c), bookID, c.Query("search"), limit, offset)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	var next *string
	if len(items) == limit {
		value := strconv.Itoa(offset + len(items))
		next = &value
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "has_more": len(items) == limit, "next_cursor": next})
}
func locatorString(raw json.RawMessage) string {
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	return string(raw)
}
func bookmarkDTO(item model.Bookmark) gin.H {
	var chapter any
	if item.ChapterID != nil {
		chapter = item.ChapterID.String()
	}
	return gin.H{"id": item.ID, "book_id": item.BookID, "chapter_id": chapter, "locator": locatorString(item.Locator), "progress_percent": item.ProgressPercent, "title": item.Title, "note": item.Note, "created_at": item.CreatedAt}
}
func highlightDTO(item model.Highlight) gin.H {
	var chapter any
	if item.ChapterID != nil {
		chapter = item.ChapterID.String()
	}
	color := item.Color
	switch color {
	case "yellow":
		color = "sand"
	case "green":
		color = "sage"
	case "pink", "purple":
		color = "rose"
	case "gray":
		color = "sand"
	}
	return gin.H{"id": item.ID, "book_id": item.BookID, "chapter_id": chapter, "locator": locatorString(item.Locator), "text_anchor": item.TextAnchor, "selected_text": item.SelectedText, "context": item.Context, "color": color, "note": item.Note, "created_at": item.CreatedAt, "updated_at": item.UpdatedAt}
}
func (s *Server) createNote(c *gin.Context) {
	var in annotations.NoteInput
	if !decodeJSON(c, &in, 512*1024) {
		return
	}
	item, err := s.services.Annotations.CreateNote(c.Request.Context(), userID(c), in)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (s *Server) getNote(c *gin.Context) {
	id, ok := parseID(c, "noteId")
	if !ok {
		return
	}
	item, err := s.services.Annotations.GetNote(c.Request.Context(), userID(c), id)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) patchNote(c *gin.Context) {
	id, ok := parseID(c, "noteId")
	if !ok {
		return
	}
	var in annotations.NotePatch
	if !decodeJSON(c, &in, 512*1024) {
		return
	}
	item, err := s.services.Annotations.PatchNote(c.Request.Context(), userID(c), id, in)
	if err != nil {
		s.annotationError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) deleteNote(c *gin.Context) {
	id, ok := parseID(c, "noteId")
	if !ok {
		return
	}
	if err := s.services.Annotations.DeleteNote(c.Request.Context(), userID(c), id); err != nil {
		s.annotationError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) annotationError(c *gin.Context, err error) {
	if errors.Is(err, annotations.ErrNotFound) {
		fail(c, http.StatusNotFound, "ANNOTATION_NOT_FOUND", "Annotation was not found", nil)
		return
	}
	fail(c, http.StatusBadRequest, "ANNOTATION_OPERATION_FAILED", err.Error(), nil)
}
