package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/XRS0/reader/backend/internal/books"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/gin-gonic/gin"
)

type bookResponse struct {
	ID                        string     `json:"id"`
	Title                     string     `json:"title"`
	Author                    string     `json:"author"`
	Format                    string     `json:"format"`
	Language                  string     `json:"language"`
	Description               string     `json:"description,omitempty"`
	ProcessingStatus          string     `json:"processing_status"`
	ProcessingError           string     `json:"processing_error,omitempty"`
	AddedAt                   time.Time  `json:"added_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
	IsFavorite                bool       `json:"is_favorite"`
	Tags                      []string   `json:"tags"`
	ProgressPercent           float64    `json:"progress_percent"`
	CurrentChapterID          *string    `json:"current_chapter_id,omitempty"`
	EstimatedMinutesRemaining *int       `json:"estimated_minutes_remaining,omitempty"`
	LastReadAt                *time.Time `json:"last_read_at,omitempty"`
	OriginalFilename          string     `json:"original_filename"`
	OriginalSize              int64      `json:"original_size"`
	CoverURL                  string     `json:"cover_url,omitempty"`
	HasCustomCover            bool       `json:"has_custom_cover"`
	ProcessingVersion         int        `json:"processing_version"`
}

func bookDTO(b model.Book, progress float64) bookResponse {
	if progress == 0 {
		progress = b.ProgressPercent
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	dto := bookResponse{ID: b.ID.String(), Title: b.Title, Author: b.Author, Format: b.Format, Language: b.Language, Description: b.Description, ProcessingStatus: b.Status, ProcessingError: b.ProcessingError, AddedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt, IsFavorite: b.IsFavorite, Tags: b.Tags, ProgressPercent: progress, OriginalFilename: b.OriginalFilename, OriginalSize: b.OriginalSize, ProcessingVersion: b.ProcessingVersion, LastReadAt: b.LastReadAt, HasCustomCover: b.CustomCoverBucket != "" && b.CustomCoverKey != ""}
	if dto.HasCustomCover || (b.CoverBucket != "" && b.CoverKey != "") {
		dto.CoverURL = fmt.Sprintf("/api/v1/books/%s/cover?v=%d", b.ID, b.UpdatedAt.Unix())
	}
	if b.CurrentChapterID != nil {
		value := b.CurrentChapterID.String()
		dto.CurrentChapterID = &value
	}
	return dto
}
func (s *Server) registerBookRoutes(r *gin.RouterGroup) {
	r.POST("/books", s.uploadBook)
	r.GET("/books", s.listBooks)
	r.GET("/books/:bookId", s.getBook)
	r.PATCH("/books/:bookId", s.patchBook)
	r.DELETE("/books/:bookId", s.deleteBook)
	r.GET("/books/:bookId/cover", s.getBookCover)
	r.PUT("/books/:bookId/cover", s.putBookCover)
	r.DELETE("/books/:bookId/cover", s.deleteBookCover)
	r.POST("/books/:bookId/reprocess", s.reprocessBook)
	r.GET("/books/:bookId/processing-status", s.bookStatus)
	r.GET("/books/:bookId/toc", s.bookTOC)
	r.GET("/books/:bookId/chapters/:chapterId", s.bookChapter)
	r.GET("/books/:bookId/download", s.downloadBook)
	r.GET("/books/:bookId/assets/:assetId", s.bookAsset)
}

func (s *Server) getBookCover(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	data, mediaType, err := s.services.Books.Cover(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.Data(http.StatusOK, mediaType, data)
}

func (s *Server) putBookCover(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, books.MaxCoverBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(2 * 1024 * 1024); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_COVER_UPLOAD", "Cover upload is invalid or too large", nil)
		return
	}
	if c.Request.MultipartForm != nil {
		defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "FILE_REQUIRED", "Multipart field 'file' is required", nil)
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, books.MaxCoverBytes+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "UPLOAD_READ_FAILED", "Could not read uploaded cover", nil)
		return
	}
	b, err := s.services.Books.UpdateCover(c.Request.Context(), userID(c), id, books.CoverInput{
		ClientMIME: header.Header.Get("Content-Type"),
		Data:       data,
	})
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.JSON(http.StatusOK, bookDTO(b, 0))
}

func (s *Server) deleteBookCover(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	b, err := s.services.Books.DeleteCover(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.JSON(http.StatusOK, bookDTO(b, 0))
}
func (s *Server) uploadBook(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, s.cfg.Upload.MaxBytes+1024*1024)
	if err := c.Request.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		fail(c, http.StatusBadRequest, "INVALID_UPLOAD", "Multipart upload is invalid or too large", nil)
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, http.StatusBadRequest, "FILE_REQUIRED", "Multipart field 'file' is required", nil)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			s.logger.Warn("close uploaded book", "error", closeErr, "request_id", c.GetString(requestIDKey))
		}
	}()
	data, err := io.ReadAll(io.LimitReader(file, s.cfg.Upload.MaxBytes+1))
	if err != nil {
		fail(c, http.StatusBadRequest, "UPLOAD_READ_FAILED", "Could not read uploaded file", nil)
		return
	}
	if int64(len(data)) > s.cfg.Upload.MaxBytes {
		fail(c, http.StatusRequestEntityTooLarge, "BOOK_TOO_LARGE", "Book exceeds the configured upload limit", nil)
		return
	}
	result, err := s.services.Books.Upload(c.Request.Context(), userID(c), books.UploadInput{Filename: header.Filename, ClientMIME: header.Header.Get("Content-Type"), Data: data})
	if err != nil {
		s.bookError(c, err)
		return
	}
	status := http.StatusAccepted
	if result.Duplicate {
		status = http.StatusOK
	}
	c.Header("X-Book-Duplicate", strconv.FormatBool(result.Duplicate))
	c.JSON(status, bookDTO(result.Book, 0))
}
func (s *Server) listBooks(c *gin.Context) {
	limit, offset := parsePage(c)
	var favorite *bool
	if raw := c.Query("favorite"); raw != "" {
		value, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			fail(c, http.StatusBadRequest, "VALIDATION_ERROR", "favorite must be true or false", nil)
			return
		}
		favorite = &value
	}
	result, err := s.services.Books.List(c.Request.Context(), userID(c), books.ListFilter{Search: c.Query("search"), Status: c.Query("status"), Format: c.Query("format"), Limit: limit, Offset: offset, Sort: c.Query("sort"), Favorite: favorite})
	if err != nil {
		s.bookError(c, err)
		return
	}
	items := make([]bookResponse, len(result.Items))
	for i, b := range result.Items {
		items[i] = bookDTO(b, 0)
	}
	var next *string
	if result.HasMore {
		value := strconv.Itoa(result.NextOffset)
		next = &value
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total_count": result.Total, "has_more": result.HasMore, "next_cursor": next})
}
func (s *Server) getBook(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	b, err := s.services.Books.Get(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	progress := 0.0
	var currentChapter *string
	if p, pErr := s.services.Progress.Get(c.Request.Context(), userID(c), id); pErr == nil {
		progress = p.ProgressPercent
		if p.ChapterID != nil {
			value := p.ChapterID.String()
			currentChapter = &value
		}
	}
	dto := bookDTO(b, progress)
	dto.CurrentChapterID = currentChapter
	c.JSON(http.StatusOK, dto)
}
func (s *Server) patchBook(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	var in struct {
		Title       *string   `json:"title"`
		Author      *string   `json:"author"`
		Language    *string   `json:"language"`
		Description *string   `json:"description"`
		IsFavorite  *bool     `json:"is_favorite"`
		Tags        *[]string `json:"tags"`
	}
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	b, err := s.services.Books.Update(c.Request.Context(), userID(c), id, in.Title, in.Author, in.Language, in.Description, in.IsFavorite, in.Tags)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.JSON(http.StatusOK, bookDTO(b, 0))
}
func (s *Server) deleteBook(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	if err := s.services.Books.Delete(c.Request.Context(), userID(c), id); err != nil {
		s.bookError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) reprocessBook(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	_, err := s.services.Books.Reprocess(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.Status(http.StatusAccepted)
}
func (s *Server) bookStatus(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	b, err := s.services.Books.Get(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"book_id": b.ID, "processing_status": b.Status, "processing_error": b.ProcessingError, "processing_version": b.ProcessingVersion, "updated_at": b.UpdatedAt})
}
func (s *Server) bookTOC(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	chapters, err := s.services.Books.Chapters(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	items := make([]gin.H, len(chapters))
	for i, ch := range chapters {
		items[i] = gin.H{"id": ch.ID, "chapter_id": ch.ID, "title": ch.Title, "level": 0, "order": ch.Ordinal}
	}
	c.JSON(http.StatusOK, items)
}
func (s *Server) bookChapter(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	chapterID, ok := parseID(c, "chapterId")
	if !ok {
		return
	}
	ch, err := s.services.Books.Chapter(c.Request.Context(), userID(c), bookID, chapterID)
	if err != nil {
		s.bookError(c, err)
		return
	}
	chapters, listErr := s.services.Books.Chapters(c.Request.Context(), userID(c), bookID)
	if listErr != nil {
		s.bookError(c, listErr)
		return
	}
	var previous, next *string
	for i, item := range chapters {
		if item.ID == ch.ID {
			if i > 0 {
				value := chapters[i-1].ID.String()
				previous = &value
			}
			if i+1 < len(chapters) {
				value := chapters[i+1].ID.String()
				next = &value
			}
			break
		}
	}
	c.Header("Cache-Control", "private, max-age=300")
	c.JSON(http.StatusOK, gin.H{"id": ch.ID, "book_id": ch.BookID, "title": ch.Title, "order": ch.Ordinal, "html": ch.ContentHTML, "plain_text": ch.ContentText, "previous_chapter_id": previous, "next_chapter_id": next, "word_count": ch.WordCount})
}
func (s *Server) downloadBook(c *gin.Context) {
	id, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	url, err := s.services.Books.DownloadURL(c.Request.Context(), userID(c), id)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url, "expires_in": int(s.cfg.S3.PresignTTL.Seconds())})
}
func (s *Server) bookAsset(c *gin.Context) {
	bookID, ok := parseID(c, "bookId")
	if !ok {
		return
	}
	assetID, ok := parseID(c, "assetId")
	if !ok {
		return
	}
	url, err := s.services.Books.AssetURL(c.Request.Context(), userID(c), bookID, assetID)
	if err != nil {
		s.bookError(c, err)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}
func (s *Server) bookError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, books.ErrNotFound):
		fail(c, http.StatusNotFound, "BOOK_NOT_FOUND", "Book was not found", nil)
	case errors.Is(err, books.ErrTooLarge):
		fail(c, http.StatusRequestEntityTooLarge, "BOOK_TOO_LARGE", err.Error(), nil)
	case errors.Is(err, books.ErrCoverTooLarge):
		fail(c, http.StatusRequestEntityTooLarge, "COVER_TOO_LARGE", err.Error(), nil)
	case errors.Is(err, books.ErrInvalidCover):
		fail(c, http.StatusUnprocessableEntity, "INVALID_BOOK_COVER", err.Error(), nil)
	case errors.Is(err, books.ErrInvalidFile):
		fail(c, http.StatusUnprocessableEntity, "INVALID_BOOK_FILE", err.Error(), nil)
	default:
		s.logger.Error("book operation", "error", err, "request_id", c.GetString(requestIDKey))
		fail(c, http.StatusInternalServerError, "BOOK_OPERATION_FAILED", "Book operation failed", nil)
	}
}
