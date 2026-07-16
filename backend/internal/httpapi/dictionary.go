package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/XRS0/reader/backend/internal/dictionary"
	"github.com/gin-gonic/gin"
)

func (s *Server) registerDictionaryRoutes(r *gin.RouterGroup) {
	r.GET("/dictionary", s.listDictionary)
	r.POST("/dictionary", s.createDictionary)
	r.GET("/dictionary/:entryId", s.getDictionary)
	r.PATCH("/dictionary/:entryId", s.patchDictionary)
	r.DELETE("/dictionary/:entryId", s.deleteDictionary)
	r.POST("/dictionary/:entryId/restore", s.restoreDictionary)
	r.POST("/dictionary/:entryId/occurrences", s.addOccurrence)
	r.GET("/dictionary/:entryId/occurrences", s.listOccurrences)
	r.POST("/dictionary/export", s.exportDictionary)
}
func (s *Server) listDictionary(c *gin.Context) {
	limit, offset := parsePage(c)
	language := c.Query("source_language")
	if language == "" {
		language = c.Query("language")
	}
	items, total, err := s.services.Dictionary.List(c.Request.Context(), userID(c), c.Query("search"), c.Query("status"), language, limit, offset)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	var next *string
	if offset+len(items) < total {
		value := strconv.Itoa(offset + len(items))
		next = &value
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total_count": total, "has_more": offset+len(items) < total, "next_cursor": next})
}
func (s *Server) createDictionary(c *gin.Context) {
	var in dictionary.CreateInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Dictionary.Create(c.Request.Context(), userID(c), in)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (s *Server) getDictionary(c *gin.Context) {
	id, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	item, err := s.services.Dictionary.Get(c.Request.Context(), userID(c), id)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) patchDictionary(c *gin.Context) {
	id, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	var in dictionary.UpdateInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Dictionary.Update(c.Request.Context(), userID(c), id, in)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) deleteDictionary(c *gin.Context) {
	id, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	if err := s.services.Dictionary.Delete(c.Request.Context(), userID(c), id); err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (s *Server) restoreDictionary(c *gin.Context) {
	id, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	item, err := s.services.Dictionary.Restore(c.Request.Context(), userID(c), id)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) addOccurrence(c *gin.Context) {
	id, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	var in dictionary.OccurrenceInput
	if !decodeJSON(c, &in, 64*1024) {
		return
	}
	item, err := s.services.Dictionary.AddOccurrence(c.Request.Context(), userID(c), id, in)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.JSON(http.StatusCreated, item)
}
func (s *Server) listOccurrences(c *gin.Context) {
	id, ok := parseID(c, "entryId")
	if !ok {
		return
	}
	limit, offset := parsePage(c)
	items, err := s.services.Dictionary.Occurrences(c.Request.Context(), userID(c), id, limit, offset)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	var next *string
	if len(items) == limit {
		value := strconv.Itoa(offset + len(items))
		next = &value
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "has_more": len(items) == limit, "next_cursor": next})
}
func (s *Server) exportDictionary(c *gin.Context) {
	items, _, err := s.services.Dictionary.List(c.Request.Context(), userID(c), "", "", "", 100, 0)
	if err != nil {
		s.dictionaryError(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="bookflow-dictionary.json"`)
	c.JSON(http.StatusOK, gin.H{"schema_version": 1, "exported_entries": items})
}
func (s *Server) dictionaryError(c *gin.Context, err error) {
	if errors.Is(err, dictionary.ErrNotFound) {
		fail(c, http.StatusNotFound, "DICTIONARY_ENTRY_NOT_FOUND", "Dictionary entry was not found", nil)
		return
	}
	fail(c, http.StatusBadRequest, "DICTIONARY_OPERATION_FAILED", err.Error(), nil)
}
