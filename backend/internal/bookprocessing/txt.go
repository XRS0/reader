package bookprocessing

import (
	"bufio"
	"context"
	"fmt"
	"html"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type TXTParser struct{}

func NewTXTParser() *TXTParser { return &TXTParser{} }
func (*TXTParser) Detect(_ context.Context, f BookFile) (bool, error) {
	data := strings.TrimPrefix(string(f.Data), "\ufeff")
	return strings.EqualFold(filepath.Ext(f.Name), ".txt") && utf8.ValidString(data), nil
}
func (*TXTParser) ParseMetadata(_ context.Context, f BookFile) (BookMetadata, error) {
	title := strings.TrimSuffix(filepath.Base(f.Name), filepath.Ext(f.Name))
	scanner := bufio.NewScanner(strings.NewReader(strings.TrimPrefix(string(f.Data), "\ufeff")))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" && utf8.RuneCountInString(line) <= 200 {
			title = line
			break
		}
	}
	return BookMetadata{Title: title}, scanner.Err()
}
func (p *TXTParser) ParseTableOfContents(ctx context.Context, f BookFile) ([]TOCItem, error) {
	chapters, err := p.ExtractChapters(ctx, f)
	if err != nil {
		return nil, err
	}
	toc := make([]TOCItem, len(chapters))
	for i, c := range chapters {
		toc[i] = TOCItem{Title: c.Title, SourceRef: c.SourceRef, Ordinal: i}
	}
	return toc, nil
}
func (*TXTParser) ExtractChapters(_ context.Context, f BookFile) ([]Chapter, error) {
	text := normalizeWhitespace(strings.TrimPrefix(string(f.Data), "\ufeff"))
	lines := strings.Split(text, "\n")
	heading := regexp.MustCompile(`(?i)^(chapter|part|глава|часть)\s+([0-9ivxlcdmа-яё-]+)(?:\s+.*)?$`)
	type section struct {
		title string
		lines []string
	}
	sections := []section{{title: "Text"}}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if heading.MatchString(line) && len(sections[len(sections)-1].lines) > 0 {
			sections = append(sections, section{title: line})
			continue
		}
		sections[len(sections)-1].lines = append(sections[len(sections)-1].lines, line)
	}
	out := make([]Chapter, 0, len(sections))
	for i, s := range sections {
		if s.title == "Text" && i == 0 {
			base := strings.TrimSuffix(filepath.Base(f.Name), filepath.Ext(f.Name))
			if base != "" {
				s.title = base
			}
		}
		paragraphs := strings.Split(strings.Join(s.lines, "\n"), "\n\n")
		var h strings.Builder
		h.WriteString("<section>")
		for _, paragraph := range paragraphs {
			if paragraph = strings.TrimSpace(paragraph); paragraph != "" {
				h.WriteString("<p>")
				h.WriteString(html.EscapeString(strings.ReplaceAll(paragraph, "\n", " ")))
				h.WriteString("</p>")
			}
		}
		h.WriteString("</section>")
		out = append(out, Chapter{Title: s.title, SourceRef: fmt.Sprintf("txt:%d", i), Ordinal: i, HTML: h.String(), PlainText: strings.TrimSpace(strings.Join(s.lines, "\n"))})
	}
	return out, nil
}
func (*TXTParser) ExtractAssets(context.Context, BookFile) ([]Asset, error) { return nil, nil }
func (*TXTParser) ExtractPlainText(_ context.Context, f BookFile) (string, error) {
	return normalizeWhitespace(strings.TrimPrefix(string(f.Data), "\ufeff")), nil
}
