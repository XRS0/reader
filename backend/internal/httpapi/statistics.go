package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerStatisticsRoutes(r *gin.RouterGroup) {
	r.GET("/statistics/overview", s.statisticsOverview)
	r.GET("/statistics/daily", s.statisticsDaily)
	r.GET("/statistics/weekly", s.statisticsWeekly)
	r.GET("/statistics/monthly", s.statisticsMonthly)
	r.GET("/statistics/books", s.statisticsBooks)
	r.GET("/statistics/sessions", s.statisticsSessions)
	r.GET("/statistics/streak", s.statisticsStreak)
	r.GET("/statistics/dictionary", s.statisticsDictionary)
}
func (s *Server) statisticsOverview(c *gin.Context) {
	o, err := s.services.Statistics.Overview(c.Request.Context(), userID(c))
	if err != nil {
		s.statisticsError(c, err)
		return
	}
	timezone := c.DefaultQuery("timezone", "UTC")
	streak, _ := s.services.Statistics.Streak(c.Request.Context(), userID(c), timezone)
	dictionary, _ := s.services.Statistics.Dictionary(c.Request.Context(), userID(c))
	wpm := 0.0
	if o.ActiveSeconds > 0 {
		wpm = float64(o.WordsRead) * 60 / float64(o.ActiveSeconds)
	}
	c.JSON(http.StatusOK, gin.H{"total_reading_seconds": o.ActiveSeconds + o.IdleSeconds, "active_reading_seconds": o.ActiveSeconds, "idle_seconds": o.IdleSeconds, "sessions_count": o.SessionCount, "books_started": o.BooksStarted, "books_completed": o.BooksCompleted, "words_read_estimate": o.WordsRead, "pages_read_estimate": o.PagesRead, "current_streak_days": streak.Current, "longest_streak_days": streak.Longest, "average_session_seconds": o.AverageSessionSeconds, "median_session_seconds": 0, "average_words_per_minute": wpm, "dictionary_words": o.DictionaryWords, "learned_words": dictionary.Known + dictionary.Mastered, "translations_count": 0})
}
func (s *Server) statisticsDaily(c *gin.Context)   { s.statisticsBuckets(c, "day") }
func (s *Server) statisticsWeekly(c *gin.Context)  { s.statisticsBuckets(c, "week") }
func (s *Server) statisticsMonthly(c *gin.Context) { s.statisticsBuckets(c, "month") }
func (s *Server) statisticsBuckets(c *gin.Context, group string) {
	from, to, timezone, ok := dateRange(c)
	if !ok {
		return
	}
	items, err := s.services.Statistics.Buckets(c.Request.Context(), userID(c), from, to, timezone, group)
	if err != nil {
		s.statisticsError(c, err)
		return
	}
	if group == "day" {
		result := make([]gin.H, len(items))
		for i, item := range items {
			result[i] = gin.H{"date": item.Period.Format("2006-01-02"), "active_seconds": item.ActiveSeconds, "idle_seconds": item.IdleSeconds, "sessions_count": item.SessionCount, "words_read_estimate": item.WordsRead}
		}
		c.JSON(http.StatusOK, result)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "timezone": timezone, "grouping": group})
}
func (s *Server) statisticsBooks(c *gin.Context) {
	limit, offset := parsePage(c)
	items, err := s.services.Statistics.Books(c.Request.Context(), userID(c), limit, offset)
	if err != nil {
		s.statisticsError(c, err)
		return
	}
	result := make([]gin.H, len(items))
	for i, item := range items {
		wpm := 0.0
		if item.ActiveSeconds > 0 {
			wpm = float64(item.WordsRead) * 60 / float64(item.ActiveSeconds)
		}
		result[i] = gin.H{"book_id": item.BookID, "book_title": item.Title, "active_seconds": item.ActiveSeconds, "sessions_count": item.SessionCount, "progress_percent": item.ProgressPercent, "average_words_per_minute": wpm, "last_read_at": item.LastReadAt}
	}
	c.JSON(http.StatusOK, result)
}
func (s *Server) statisticsSessions(c *gin.Context) {
	limit, offset := parsePage(c)
	items, err := s.services.Statistics.Sessions(c.Request.Context(), userID(c), limit, offset)
	if err != nil {
		s.statisticsError(c, err)
		return
	}
	var next *string
	if len(items) == limit {
		value := strconv.Itoa(offset + len(items))
		next = &value
	}
	dtos := make([]gin.H, len(items))
	for i, item := range items {
		dtos[i] = sessionDTO(item)
	}
	c.JSON(http.StatusOK, gin.H{"items": dtos, "has_more": len(items) == limit, "next_cursor": next})
}
func (s *Server) statisticsStreak(c *gin.Context) {
	timezone := c.DefaultQuery("timezone", "UTC")
	item, err := s.services.Statistics.Streak(c.Request.Context(), userID(c), timezone)
	if err != nil {
		s.statisticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func (s *Server) statisticsDictionary(c *gin.Context) {
	item, err := s.services.Statistics.Dictionary(c.Request.Context(), userID(c))
	if err != nil {
		s.statisticsError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
func dateRange(c *gin.Context) (time.Time, time.Time, string, bool) {
	timezone := c.DefaultQuery("timezone", "UTC")
	to := time.Now().UTC()
	from := to.AddDate(0, -1, 0)
	var err error
	if raw := c.Query("from"); raw != "" {
		from, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			fail(c, http.StatusBadRequest, "INVALID_DATE_RANGE", "from must be RFC3339", nil)
			return time.Time{}, time.Time{}, "", false
		}
	}
	if raw := c.Query("to"); raw != "" {
		to, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			fail(c, http.StatusBadRequest, "INVALID_DATE_RANGE", "to must be RFC3339", nil)
			return time.Time{}, time.Time{}, "", false
		}
	}
	return from, to, timezone, true
}
func (s *Server) statisticsError(c *gin.Context, err error) {
	fail(c, http.StatusBadRequest, "STATISTICS_REQUEST_FAILED", err.Error(), nil)
}
