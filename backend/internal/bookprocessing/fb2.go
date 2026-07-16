package bookprocessing

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"html"
	"path/filepath"
	"strings"
)

type fb2Document struct {
	XMLName     xml.Name `xml:"FictionBook"`
	Description struct {
		TitleInfo struct {
			Genre  []string `xml:"genre"`
			Author []struct {
				First  string `xml:"first-name"`
				Middle string `xml:"middle-name"`
				Last   string `xml:"last-name"`
			} `xml:"author"`
			BookTitle  string  `xml:"book-title"`
			Annotation fb2Text `xml:"annotation"`
			Lang       string  `xml:"lang"`
			Cover      struct {
				Images []struct {
					Href string `xml:"href,attr"`
				} `xml:"image"`
			} `xml:"coverpage"`
		} `xml:"title-info"`
	} `xml:"description"`
	Bodies []struct {
		Name     string       `xml:"name,attr"`
		Sections []fb2Section `xml:"section"`
	} `xml:"body"`
	Binaries []struct {
		ID          string `xml:"id,attr"`
		ContentType string `xml:"content-type,attr"`
		Data        string `xml:",chardata"`
	} `xml:"binary"`
}
type fb2Text struct {
	Paragraphs []string `xml:"p"`
}
type fb2Section struct {
	ID          string       `xml:"id,attr"`
	Title       fb2Text      `xml:"title"`
	Paragraphs  []string     `xml:"p"`
	Subsections []fb2Section `xml:"section"`
}
type FB2Parser struct{}

func NewFB2Parser() *FB2Parser { return &FB2Parser{} }
func (*FB2Parser) Detect(_ context.Context, f BookFile) (bool, error) {
	sample := string(f.Data)
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	ok := strings.EqualFold(filepath.Ext(f.Name), ".fb2") && strings.Contains(sample, "<FictionBook")
	return ok, nil
}
func decodeFB2(f BookFile) (fb2Document, error) {
	if len(f.Data) == 0 {
		return fb2Document{}, ErrCorruptBook
	}
	var d fb2Document
	dec := xml.NewDecoder(strings.NewReader(string(f.Data)))
	dec.Strict = true
	if err := dec.Decode(&d); err != nil {
		return d, fmt.Errorf("%w: invalid FB2 XML: %w", ErrCorruptBook, err)
	}
	if d.XMLName.Local != "FictionBook" {
		return d, fmt.Errorf("%w: missing FictionBook root", ErrCorruptBook)
	}
	return d, nil
}
func (*FB2Parser) ParseMetadata(_ context.Context, f BookFile) (BookMetadata, error) {
	d, err := decodeFB2(f)
	if err != nil {
		return BookMetadata{}, err
	}
	ti := d.Description.TitleInfo
	authors := make([]string, 0, len(ti.Author))
	for _, a := range ti.Author {
		name := strings.TrimSpace(strings.Join([]string{a.First, a.Middle, a.Last}, " "))
		if name != "" {
			authors = append(authors, name)
		}
	}
	cover := ""
	if len(ti.Cover.Images) > 0 {
		cover = strings.TrimPrefix(ti.Cover.Images[0].Href, "#")
	}
	return BookMetadata{Title: strings.TrimSpace(ti.BookTitle), Authors: authors, Language: strings.TrimSpace(ti.Lang), Description: strings.Join(ti.Annotation.Paragraphs, "\n"), CoverRef: cover}, nil
}
func (p *FB2Parser) ParseTableOfContents(ctx context.Context, f BookFile) ([]TOCItem, error) {
	chapters, err := p.ExtractChapters(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]TOCItem, len(chapters))
	for i, c := range chapters {
		out[i] = TOCItem{Title: c.Title, SourceRef: c.SourceRef, Ordinal: i}
	}
	return out, nil
}
func (*FB2Parser) ExtractChapters(_ context.Context, f BookFile) ([]Chapter, error) {
	d, err := decodeFB2(f)
	if err != nil {
		return nil, err
	}
	var out []Chapter
	var add func(fb2Section)
	add = func(s fb2Section) {
		title := strings.TrimSpace(strings.Join(s.Title.Paragraphs, " "))
		if title == "" {
			title = fmt.Sprintf("Chapter %d", len(out)+1)
		}
		var h strings.Builder
		h.WriteString("<section><h2>")
		h.WriteString(html.EscapeString(title))
		h.WriteString("</h2>")
		for _, p := range s.Paragraphs {
			if p = strings.TrimSpace(p); p != "" {
				h.WriteString("<p>")
				h.WriteString(html.EscapeString(p))
				h.WriteString("</p>")
			}
		}
		h.WriteString("</section>")
		plain := strings.TrimSpace(title + "\n" + strings.Join(s.Paragraphs, "\n"))
		ref := s.ID
		if ref == "" {
			ref = fmt.Sprintf("section-%d", len(out)+1)
		}
		out = append(out, Chapter{Title: title, SourceRef: ref, Ordinal: len(out), HTML: h.String(), PlainText: plain})
		for _, sub := range s.Subsections {
			add(sub)
		}
	}
	for _, body := range d.Bodies {
		for _, s := range body.Sections {
			add(s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: FB2 contains no sections", ErrCorruptBook)
	}
	return out, nil
}
func (*FB2Parser) ExtractAssets(_ context.Context, f BookFile) ([]Asset, error) {
	d, err := decodeFB2(f)
	if err != nil {
		return nil, err
	}
	out := make([]Asset, 0, len(d.Binaries))
	coverRef := ""
	if len(d.Description.TitleInfo.Cover.Images) > 0 {
		coverRef = strings.TrimPrefix(d.Description.TitleInfo.Cover.Images[0].Href, "#")
	}
	for _, b := range d.Binaries {
		if len(b.Data) > 80*1024*1024 {
			return nil, ErrArchiveLimit
		}
		data, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(b.Data), ""))
		if err != nil {
			return nil, fmt.Errorf("%w: invalid FB2 binary %q", ErrCorruptBook, b.ID)
		}
		if len(data) > 50*1024*1024 {
			return nil, ErrArchiveLimit
		}
		media := strings.ToLower(b.ContentType)
		if !strings.HasPrefix(media, "image/") {
			continue
		}
		out = append(out, Asset{SourceRef: b.ID, MediaType: media, Data: data, IsCover: b.ID == coverRef})
	}
	return out, nil
}
func (p *FB2Parser) ExtractPlainText(ctx context.Context, f BookFile) (string, error) {
	chapters, err := p.ExtractChapters(ctx, f)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(chapters))
	for i, c := range chapters {
		parts[i] = c.PlainText
	}
	return strings.Join(parts, "\n\n"), nil
}
