package preferences

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/XRS0/reader/backend/internal/model"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Preferences struct {
	Theme              string    `json:"theme"`
	BackgroundColor    string    `json:"background_color"`
	TextColor          string    `json:"text_color"`
	AccentColor        string    `json:"accent_color"`
	FontFamily         string    `json:"font_family"`
	FontSize           float64   `json:"font_size"`
	FontWeight         int       `json:"font_weight"`
	LineHeight         float64   `json:"line_height"`
	LetterSpacing      float64   `json:"letter_spacing"`
	ContentWidth       int       `json:"content_width"`
	PageMargin         int       `json:"page_margin"`
	TextAlign          string    `json:"text_align"`
	ReadingMode        string    `json:"reading_mode"`
	ShowProgress       bool      `json:"show_progress"`
	ShowRemainingTime  bool      `json:"show_remaining_time"`
	ControlsBrightness float64   `json:"controls_brightness"`
	UpdatedAt          time.Time `json:"updated_at"`
}
type Service struct {
	db  *bun.DB
	now func() time.Time
}

func NewService(db *bun.DB) *Service { return &Service{db: db, now: time.Now} }
func Default() Preferences {
	return Preferences{Theme: "light", BackgroundColor: "#ffffff", TextColor: "#252525", AccentColor: "#3468c0", FontFamily: "system", FontSize: 18, FontWeight: 400, LineHeight: 1.6, ContentWidth: 720, PageMargin: 32, TextAlign: "left", ReadingMode: "scroll", ShowProgress: true, ShowRemainingTime: true, ControlsBrightness: .8}
}
func (s *Service) Get(ctx context.Context, userID uuid.UUID) (Preferences, error) {
	var m model.ReaderPreferences
	err := s.db.NewSelect().Model(&m).Where("user_id=?", userID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		d := Default()
		d.UpdatedAt = s.now().UTC()
		m = toModel(userID, d)
		_, err = s.db.NewInsert().Model(&m).On("CONFLICT (user_id) DO NOTHING").Exec(ctx)
		if err != nil {
			return Preferences{}, err
		}
		return d, nil
	}
	return fromModel(m), err
}
func (s *Service) Put(ctx context.Context, userID uuid.UUID, p Preferences) (Preferences, error) {
	if err := Validate(p); err != nil {
		return Preferences{}, err
	}
	p.UpdatedAt = s.now().UTC()
	m := toModel(userID, p)
	_, err := s.db.NewInsert().Model(&m).On("CONFLICT (user_id) DO UPDATE").Set("theme=EXCLUDED.theme").Set("background_color=EXCLUDED.background_color").Set("text_color=EXCLUDED.text_color").Set("accent_color=EXCLUDED.accent_color").Set("font_family=EXCLUDED.font_family").Set("font_size=EXCLUDED.font_size").Set("font_weight=EXCLUDED.font_weight").Set("line_height=EXCLUDED.line_height").Set("letter_spacing=EXCLUDED.letter_spacing").Set("content_width=EXCLUDED.content_width").Set("margin_size=EXCLUDED.margin_size").Set("text_align=EXCLUDED.text_align").Set("navigation_mode=EXCLUDED.navigation_mode").Set("show_progress=EXCLUDED.show_progress").Set("show_remaining_time=EXCLUDED.show_remaining_time").Set("ui_intensity=EXCLUDED.ui_intensity").Set("updated_at=EXCLUDED.updated_at").Exec(ctx)
	return p, err
}
func (s *Service) GetBook(ctx context.Context, userID, bookID uuid.UUID) (Preferences, error) {
	var owns bool
	if err := s.db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM books WHERE id=? AND user_id=? AND deleted_at IS NULL)", bookID, userID).Scan(ctx, &owns); err != nil {
		return Preferences{}, err
	}
	if !owns {
		return Preferences{}, sql.ErrNoRows
	}
	var m model.ReaderPreferences
	err := s.db.NewSelect().TableExpr("book_reader_preferences AS pref").ColumnExpr("pref.user_id, pref.theme, pref.background_color, pref.text_color, pref.accent_color, pref.font_family, pref.font_size, pref.font_weight, pref.line_height, pref.letter_spacing, pref.content_width, pref.margin_size, pref.text_align, pref.navigation_mode, pref.show_progress, pref.show_remaining_time, pref.ui_intensity, pref.updated_at").Where("pref.user_id=? AND pref.book_id=?", userID, bookID).Scan(ctx, &m)
	if errors.Is(err, sql.ErrNoRows) {
		return s.Get(ctx, userID)
	}
	return fromModel(m), err
}
func (s *Service) PutBook(ctx context.Context, userID, bookID uuid.UUID, p Preferences) (Preferences, error) {
	if err := Validate(p); err != nil {
		return Preferences{}, err
	}
	var owns bool
	if err := s.db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM books WHERE id=? AND user_id=? AND deleted_at IS NULL)", bookID, userID).Scan(ctx, &owns); err != nil {
		return Preferences{}, err
	}
	if !owns {
		return Preferences{}, sql.ErrNoRows
	}
	p.UpdatedAt = s.now().UTC()
	m := toModel(userID, p)
	_, err := s.db.NewRaw(`INSERT INTO book_reader_preferences(user_id,book_id,theme,background_color,text_color,accent_color,font_family,font_size,font_weight,line_height,letter_spacing,content_width,margin_size,text_align,navigation_mode,show_progress,show_remaining_time,ui_intensity,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,book_id) DO UPDATE SET theme=EXCLUDED.theme,background_color=EXCLUDED.background_color,text_color=EXCLUDED.text_color,accent_color=EXCLUDED.accent_color,font_family=EXCLUDED.font_family,font_size=EXCLUDED.font_size,font_weight=EXCLUDED.font_weight,line_height=EXCLUDED.line_height,letter_spacing=EXCLUDED.letter_spacing,content_width=EXCLUDED.content_width,margin_size=EXCLUDED.margin_size,text_align=EXCLUDED.text_align,navigation_mode=EXCLUDED.navigation_mode,show_progress=EXCLUDED.show_progress,show_remaining_time=EXCLUDED.show_remaining_time,ui_intensity=EXCLUDED.ui_intensity,updated_at=EXCLUDED.updated_at`, userID, bookID, m.Theme, m.BackgroundColor, m.TextColor, m.AccentColor, m.FontFamily, m.FontSize, m.FontWeight, m.LineHeight, m.LetterSpacing, m.ContentWidth, m.MarginSize, m.TextAlign, m.NavigationMode, m.ShowProgress, m.ShowRemainingTime, m.UIIntensity, m.UpdatedAt).Exec(ctx)
	return p, err
}
func Validate(p Preferences) error {
	allowed := map[string]bool{"light": true, "warm": true, "sepia": true, "dark": true, "custom": true}
	if !allowed[p.Theme] {
		return errors.New("invalid theme")
	}
	color := regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	if !color.MatchString(p.BackgroundColor) || !color.MatchString(p.TextColor) || !color.MatchString(p.AccentColor) {
		return errors.New("colors must use #RRGGBB")
	}
	if p.Theme == "custom" && contrastRatio(p.BackgroundColor, p.TextColor) < 4.5 {
		return errors.New("custom theme contrast must be at least 4.5:1")
	}
	if p.FontSize < 10 || p.FontSize > 48 || p.FontWeight < 100 || p.FontWeight > 900 || p.LineHeight < 1 || p.LineHeight > 3 || p.LetterSpacing < -2 || p.LetterSpacing > 10 || p.ContentWidth < 320 || p.ContentWidth > 1400 || p.PageMargin < 0 || p.PageMargin > 200 || p.ControlsBrightness < 0 || p.ControlsBrightness > 1 {
		return errors.New("reader preference is out of range")
	}
	if p.TextAlign != "left" && p.TextAlign != "justify" {
		return errors.New("invalid text alignment")
	}
	if p.ReadingMode != "scroll" && p.ReadingMode != "paged" {
		return errors.New("invalid reading mode")
	}
	fonts := map[string]bool{"system": true, "serif": true, "Georgia": true, "Arial": true, "Inter": true, "Source Serif 4": true}
	if !fonts[p.FontFamily] {
		return errors.New("invalid font family")
	}
	if p.FontWeight != 400 && p.FontWeight != 500 && p.FontWeight != 600 {
		return errors.New("invalid font weight")
	}
	return nil
}
func contrastRatio(a, b string) float64 {
	l1, l2 := luminance(a), luminance(b)
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + .05) / (l2 + .05)
}
func luminance(color string) float64 {
	component := func(raw string) float64 {
		value, _ := strconv.ParseUint(raw, 16, 8)
		v := float64(value) / 255
		if v <= .03928 {
			return v / 12.92
		}
		return math.Pow((v+.055)/1.055, 2.4)
	}
	return .2126*component(color[1:3]) + .7152*component(color[3:5]) + .0722*component(color[5:7])
}
func toModel(userID uuid.UUID, p Preferences) model.ReaderPreferences {
	return model.ReaderPreferences{UserID: userID, Theme: p.Theme, BackgroundColor: p.BackgroundColor, TextColor: p.TextColor, AccentColor: p.AccentColor, FontFamily: p.FontFamily, FontSize: p.FontSize, FontWeight: p.FontWeight, LineHeight: p.LineHeight, LetterSpacing: p.LetterSpacing, ContentWidth: p.ContentWidth, MarginSize: p.PageMargin, TextAlign: p.TextAlign, NavigationMode: p.ReadingMode, ShowProgress: p.ShowProgress, ShowRemainingTime: p.ShowRemainingTime, UIIntensity: p.ControlsBrightness, UpdatedAt: p.UpdatedAt}
}
func fromModel(m model.ReaderPreferences) Preferences {
	return Preferences{Theme: m.Theme, BackgroundColor: m.BackgroundColor, TextColor: m.TextColor, AccentColor: m.AccentColor, FontFamily: m.FontFamily, FontSize: m.FontSize, FontWeight: m.FontWeight, LineHeight: m.LineHeight, LetterSpacing: m.LetterSpacing, ContentWidth: m.ContentWidth, PageMargin: m.MarginSize, TextAlign: m.TextAlign, ReadingMode: m.NavigationMode, ShowProgress: m.ShowProgress, ShowRemainingTime: m.ShowRemainingTime, ControlsBrightness: m.UIIntensity, UpdatedAt: m.UpdatedAt}
}
