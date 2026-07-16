package bookprocessing

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported book format")
	ErrCorruptBook       = errors.New("book is corrupt")
	ErrArchiveLimit      = errors.New("archive safety limit exceeded")
)

type BookFile struct {
	Name      string
	MediaType string
	Data      []byte
}
type BookMetadata struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	Language    string   `json:"language"`
	Description string   `json:"description"`
	Identifier  string   `json:"identifier"`
	CoverRef    string   `json:"cover_ref"`
}
type TOCItem struct {
	Title     string    `json:"title"`
	SourceRef string    `json:"source_ref"`
	Ordinal   int       `json:"ordinal"`
	Children  []TOCItem `json:"children,omitempty"`
}
type Chapter struct {
	ID        uuid.UUID
	Title     string
	SourceRef string
	HTML      string
	PlainText string
	Ordinal   int
}
type Asset struct {
	ID        uuid.UUID
	SourceRef string
	MediaType string
	Data      []byte
	IsCover   bool
}

type BookParser interface {
	Detect(context.Context, BookFile) (bool, error)
	ParseMetadata(context.Context, BookFile) (BookMetadata, error)
	ParseTableOfContents(context.Context, BookFile) ([]TOCItem, error)
	ExtractChapters(context.Context, BookFile) ([]Chapter, error)
	ExtractAssets(context.Context, BookFile) ([]Asset, error)
	ExtractPlainText(context.Context, BookFile) (string, error)
}

type Registry struct{ parsers []BookParser }

func NewRegistry(limits ArchiveLimits) *Registry {
	return &Registry{parsers: []BookParser{NewEPUBParser(limits), NewFB2Parser(), NewTXTParser()}}
}
func (r *Registry) Detect(ctx context.Context, file BookFile) (BookParser, error) {
	for _, p := range r.parsers {
		ok, err := p.Detect(ctx, file)
		if err != nil {
			return nil, err
		}
		if ok {
			return p, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(file.Name))
}
func FormatFromName(name string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
}
