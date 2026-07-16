package httpapi

import (
	"net/http"
	"time"

	"github.com/XRS0/reader/backend/internal/translation"
	"github.com/gin-gonic/gin"
)

func (s *Server) registerTranslationRoutes(r *gin.RouterGroup, limit gin.HandlerFunc) {
	r.POST("/translations/word", limit, s.translateWord)
	r.POST("/translations/text", limit, s.translateText)
	r.POST("/translations/detect-language", limit, s.detectLanguage)
}
func (s *Server) translateWord(c *gin.Context) {
	var req translation.Request
	if !decodeJSON(c, &req, 32*1024) {
		return
	}
	started := time.Now()
	result, err := s.services.Translations.TranslateWord(c.Request.Context(), req)
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	s.metrics.TranslationDuration.WithLabelValues("word", outcome).Observe(time.Since(started).Seconds())
	if err != nil {
		fail(c, http.StatusBadGateway, "TRANSLATION_FAILED", err.Error(), nil)
		return
	}
	cache := "miss"
	if result.Cached {
		cache = "hit"
	}
	s.metrics.TranslationCache.WithLabelValues(cache).Inc()
	c.JSON(http.StatusOK, gin.H{"original_text": result.Original, "normalized_form": result.Normalized, "lemma": result.Lemma, "translation": result.Translation, "transcription": result.Transcription, "part_of_speech": result.PartOfSpeech, "definition": result.Definition, "alternatives": result.Alternatives, "example": result.Example, "source_language": result.Language, "target_language": req.TargetLanguage, "confidence": result.Confidence, "cached": result.Cached})
}
func (s *Server) translateText(c *gin.Context) {
	var req translation.Request
	if !decodeJSON(c, &req, 64*1024) {
		return
	}
	started := time.Now()
	result, err := s.services.Translations.TranslateText(c.Request.Context(), req)
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	s.metrics.TranslationDuration.WithLabelValues("text", outcome).Observe(time.Since(started).Seconds())
	if err != nil {
		fail(c, http.StatusBadGateway, "TRANSLATION_FAILED", err.Error(), nil)
		return
	}
	cache := "miss"
	if result.Cached {
		cache = "hit"
	}
	s.metrics.TranslationCache.WithLabelValues(cache).Inc()
	c.JSON(http.StatusOK, gin.H{"original_text": result.Original, "translation": result.Translation, "detected_language": result.DetectedLanguage, "explanation": result.Explanation, "cached": result.Cached})
}
func (s *Server) detectLanguage(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if !decodeJSON(c, &req, 16*1024) {
		return
	}
	result, err := s.services.Translations.Detect(c.Request.Context(), req.Text)
	if err != nil {
		fail(c, http.StatusBadRequest, "LANGUAGE_DETECTION_FAILED", err.Error(), nil)
		return
	}
	c.JSON(http.StatusOK, result)
}
