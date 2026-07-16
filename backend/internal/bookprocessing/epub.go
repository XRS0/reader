package bookprocessing

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
)

type ArchiveLimits struct {
	MaxFiles            int
	MaxUnpackedBytes    int64
	MaxCompressionRatio float64
	MaxEntryBytes       int64
}
type EPUBParser struct{ limits ArchiveLimits }

func NewEPUBParser(l ArchiveLimits) *EPUBParser {
	if l.MaxEntryBytes <= 0 {
		l.MaxEntryBytes = 50 * 1024 * 1024
	}
	return &EPUBParser{limits: l}
}
func (p *EPUBParser) Detect(_ context.Context, f BookFile) (bool, error) {
	if !strings.EqualFold(filepath.Ext(f.Name), ".epub") || len(f.Data) < 4 || !bytes.Equal(f.Data[:2], []byte("PK")) {
		return false, nil
	}
	a, err := p.open(f)
	if err != nil {
		return false, err
	}
	mime, err := a.read("mimetype", 256)
	if err != nil {
		return false, fmt.Errorf("%w: EPUB has no mimetype", ErrCorruptBook)
	}
	return strings.TrimSpace(string(mime)) == "application/epub+zip", nil
}

type epubArchive struct {
	reader *zip.Reader
	files  map[string]*zip.File
	limits ArchiveLimits
}

func (p *EPUBParser) open(f BookFile) (*epubArchive, error) {
	r, err := zip.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)))
	if err != nil {
		return nil, fmt.Errorf("%w: invalid ZIP: %w", ErrCorruptBook, err)
	}
	if len(r.File) > p.limits.MaxFiles {
		return nil, fmt.Errorf("%w: %d files exceeds %d", ErrArchiveLimit, len(r.File), p.limits.MaxFiles)
	}
	a := &epubArchive{reader: r, files: map[string]*zip.File{}, limits: p.limits}
	var total uint64
	for _, zf := range r.File {
		name := strings.ReplaceAll(zf.Name, "\\", "/")
		clean := path.Clean(name)
		if strings.HasPrefix(name, "/") || clean == ".." || strings.HasPrefix(clean, "../") || strings.ContainsRune(name, '\x00') {
			return nil, fmt.Errorf("%w: unsafe path %q", ErrCorruptBook, zf.Name)
		}
		if zf.Flags&1 != 0 {
			return nil, fmt.Errorf("%w: encrypted entry", ErrCorruptBook)
		}
		total += zf.UncompressedSize64
		if total > uint64(p.limits.MaxUnpackedBytes) || zf.UncompressedSize64 > uint64(p.limits.MaxEntryBytes) {
			return nil, fmt.Errorf("%w: unpacked data too large", ErrArchiveLimit)
		}
		compressed := zf.CompressedSize64
		if compressed == 0 && zf.UncompressedSize64 > 0 {
			return nil, fmt.Errorf("%w: invalid compression ratio", ErrArchiveLimit)
		}
		if compressed > 0 && float64(zf.UncompressedSize64)/float64(compressed) > p.limits.MaxCompressionRatio {
			return nil, fmt.Errorf("%w: entry %q compression ratio", ErrArchiveLimit, zf.Name)
		}
		if !strings.HasSuffix(name, "/") {
			a.files[clean] = zf
		}
	}
	return a, nil
}
func (a *epubArchive) read(name string, max int64) ([]byte, error) {
	zf := a.files[path.Clean(strings.TrimPrefix(name, "/"))]
	if zf == nil {
		return nil, ErrCorruptBook
	}
	if int64(zf.UncompressedSize64) > max {
		return nil, ErrArchiveLimit
	}
	r, err := zf.Open()
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(r, max+1))
	closeErr := r.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(data)) > max {
		return nil, ErrArchiveLimit
	}
	return data, nil
}

type epubContainer struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}
type epubPackage struct {
	Metadata struct {
		Titles      []string `xml:"title"`
		Creators    []string `xml:"creator"`
		Language    string   `xml:"language"`
		Description string   `xml:"description"`
		Identifiers []string `xml:"identifier"`
		Meta        []struct {
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Property string `xml:"property,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest []struct {
		ID         string `xml:"id,attr"`
		Href       string `xml:"href,attr"`
		MediaType  string `xml:"media-type,attr"`
		Properties string `xml:"properties,attr"`
	} `xml:"manifest>item"`
	Spine []struct {
		IDRef  string `xml:"idref,attr"`
		Linear string `xml:"linear,attr"`
	} `xml:"spine>itemref"`
}
type parsedEPUB struct {
	a        *epubArchive
	pkg      epubPackage
	opfPath  string
	root     string
	manifest map[string]epubManifestItem
	spine    []epubManifestItem
}
type epubManifestItem struct{ ID, Path, MediaType, Properties string }

type epubNCX struct {
	Points []epubNCXPoint `xml:"navMap>navPoint"`
}

type epubNCXPoint struct {
	Label   string `xml:"navLabel>text"`
	Content struct {
		Source string `xml:"src,attr"`
	} `xml:"content"`
	Children []epubNCXPoint `xml:"navPoint"`
}

type epubTOCLink struct {
	Reference string
	Label     string
}

func (p *EPUBParser) parse(f BookFile) (parsedEPUB, error) {
	a, err := p.open(f)
	if err != nil {
		return parsedEPUB{}, err
	}
	raw, err := a.read("META-INF/container.xml", 1024*1024)
	if err != nil {
		return parsedEPUB{}, fmt.Errorf("%w: invalid container", ErrCorruptBook)
	}
	var c epubContainer
	if err := xml.Unmarshal(raw, &c); err != nil || len(c.Rootfiles) == 0 {
		return parsedEPUB{}, fmt.Errorf("%w: invalid container XML", ErrCorruptBook)
	}
	opf := path.Clean(c.Rootfiles[0].FullPath)
	raw, err = a.read(opf, 5*1024*1024)
	if err != nil {
		return parsedEPUB{}, fmt.Errorf("%w: missing package", ErrCorruptBook)
	}
	var pkg epubPackage
	if err := xml.Unmarshal(raw, &pkg); err != nil {
		return parsedEPUB{}, fmt.Errorf("%w: invalid package XML", ErrCorruptBook)
	}
	root := path.Dir(opf)
	if root == "." {
		root = ""
	}
	result := parsedEPUB{a: a, pkg: pkg, opfPath: opf, root: root, manifest: map[string]epubManifestItem{}}
	for _, m := range pkg.Manifest {
		resolved := path.Clean(path.Join(root, m.Href))
		if resolved == ".." || strings.HasPrefix(resolved, "../") {
			return parsedEPUB{}, fmt.Errorf("%w: manifest traversal", ErrCorruptBook)
		}
		item := epubManifestItem{ID: m.ID, Path: resolved, MediaType: strings.ToLower(m.MediaType), Properties: m.Properties}
		result.manifest[m.ID] = item
	}
	for _, ref := range pkg.Spine {
		if strings.EqualFold(ref.Linear, "no") {
			continue
		}
		item, ok := result.manifest[ref.IDRef]
		if !ok {
			return parsedEPUB{}, fmt.Errorf("%w: spine reference %q", ErrCorruptBook, ref.IDRef)
		}
		result.spine = append(result.spine, item)
	}
	if len(result.spine) == 0 {
		return parsedEPUB{}, fmt.Errorf("%w: empty spine", ErrCorruptBook)
	}
	return result, nil
}
func (p *EPUBParser) ParseMetadata(_ context.Context, f BookFile) (BookMetadata, error) {
	e, err := p.parse(f)
	if err != nil {
		return BookMetadata{}, err
	}
	m := BookMetadata{Authors: e.pkg.Metadata.Creators, Language: strings.TrimSpace(e.pkg.Metadata.Language), Description: strings.TrimSpace(e.pkg.Metadata.Description)}
	if len(e.pkg.Metadata.Titles) > 0 {
		m.Title = strings.TrimSpace(e.pkg.Metadata.Titles[0])
	}
	if len(e.pkg.Metadata.Identifiers) > 0 {
		m.Identifier = strings.TrimSpace(e.pkg.Metadata.Identifiers[0])
	}
	coverID := ""
	for _, meta := range e.pkg.Metadata.Meta {
		if meta.Name == "cover" {
			coverID = meta.Content
		}
	}
	for _, item := range e.manifest {
		if item.ID == coverID || strings.Contains(" "+item.Properties+" ", " cover-image ") {
			m.CoverRef = item.Path
			break
		}
	}
	return m, nil
}
func (p *EPUBParser) ExtractChapters(_ context.Context, f BookFile) ([]Chapter, error) {
	e, err := p.parse(f)
	if err != nil {
		return nil, err
	}
	labels := e.chapterLabels()
	out := make([]Chapter, 0, len(e.spine))
	previousTitle := ""
	for i, item := range e.spine {
		if item.MediaType != "application/xhtml+xml" && item.MediaType != "text/html" {
			continue
		}
		raw, err := e.a.read(item.Path, e.a.limits.MaxEntryBytes)
		if err != nil {
			return nil, fmt.Errorf("read chapter %q: %w", item.Path, err)
		}
		safe := SanitizeHTML(string(raw))
		plain := PlainTextFromHTML(safe)
		if strings.TrimSpace(plain) == "" && !strings.Contains(strings.ToLower(safe), "<img") {
			continue
		}
		title := labels[item.Path]
		if title == "" {
			title = extractTitle(safe)
		}
		switch {
		case title != "":
			previousTitle = title
		case previousTitle != "":
			title = continuedChapterTitle(previousTitle, e.pkg.Metadata.Language)
		case len(e.pkg.Metadata.Titles) > 0 && strings.TrimSpace(e.pkg.Metadata.Titles[0]) != "":
			title = strings.TrimSpace(e.pkg.Metadata.Titles[0])
			previousTitle = title
		default:
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		out = append(out, Chapter{Title: title, SourceRef: item.Path, Ordinal: len(out), HTML: safe, PlainText: plain})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no readable chapters", ErrCorruptBook)
	}
	return out, nil
}

func (e parsedEPUB) chapterLabels() map[string]string {
	labels := make(map[string]string)
	// Prefer the EPUB 3 navigation document when both EPUB 3 nav and an EPUB 2
	// NCX compatibility document are present.
	for _, item := range e.manifest {
		if !hasManifestProperty(item.Properties, "nav") {
			continue
		}
		raw, err := e.a.read(item.Path, e.a.limits.MaxEntryBytes)
		if err != nil {
			continue
		}
		for _, link := range parseEPUB3Navigation(raw) {
			addChapterLabel(labels, item.Path, link.Reference, link.Label, false)
		}
	}
	for _, item := range e.manifest {
		if item.MediaType != "application/x-dtbncx+xml" {
			continue
		}
		raw, err := e.a.read(item.Path, e.a.limits.MaxEntryBytes)
		if err != nil {
			continue
		}
		var document epubNCX
		if err := xml.Unmarshal(raw, &document); err != nil {
			continue
		}
		var addPoints func([]epubNCXPoint)
		addPoints = func(points []epubNCXPoint) {
			for _, point := range points {
				addChapterLabel(labels, item.Path, point.Content.Source, point.Label, false)
				addPoints(point.Children)
			}
		}
		addPoints(document.Points)
	}
	return labels
}

func parseEPUB3Navigation(raw []byte) []epubTOCLink {
	type navigationGroup struct {
		isTOC bool
		links []epubTOCLink
	}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	groups := make([]navigationGroup, 0, 2)
	depth := 0
	currentGroup := -1
	navigationDepth := 0
	anchorDepth := 0
	anchorReference := ""
	var anchorText strings.Builder

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil
		}
		switch typed := token.(type) {
		case xml.StartElement:
			depth++
			if strings.EqualFold(typed.Name.Local, "nav") && currentGroup < 0 {
				groups = append(groups, navigationGroup{isTOC: navigationIsTOC(typed.Attr)})
				currentGroup = len(groups) - 1
				navigationDepth = depth
				continue
			}
			if currentGroup >= 0 && anchorDepth == 0 && strings.EqualFold(typed.Name.Local, "a") {
				anchorReference = attributeValue(typed.Attr, "href")
				anchorText.Reset()
				anchorDepth = depth
			}
		case xml.CharData:
			if anchorDepth > 0 {
				anchorText.Write([]byte(typed))
			}
		case xml.EndElement:
			if anchorDepth == depth && strings.EqualFold(typed.Name.Local, "a") {
				groups[currentGroup].links = append(groups[currentGroup].links, epubTOCLink{Reference: anchorReference, Label: normalizeTOCLabel(anchorText.String())})
				anchorDepth = 0
				anchorReference = ""
			}
			if currentGroup >= 0 && navigationDepth == depth && strings.EqualFold(typed.Name.Local, "nav") {
				currentGroup = -1
				navigationDepth = 0
			}
			depth--
		}
	}
	for _, group := range groups {
		if group.isTOC && len(group.links) > 0 {
			return group.links
		}
	}
	for _, group := range groups {
		if len(group.links) > 0 {
			return group.links
		}
	}
	return nil
}

func navigationIsTOC(attributes []xml.Attr) bool {
	for _, attribute := range attributes {
		if !strings.EqualFold(attribute.Name.Local, "type") && !strings.EqualFold(attribute.Name.Local, "role") {
			continue
		}
		for _, value := range strings.Fields(strings.ToLower(attribute.Value)) {
			if value == "toc" || value == "doc-toc" {
				return true
			}
		}
	}
	return false
}

func attributeValue(attributes []xml.Attr, name string) string {
	for _, attribute := range attributes {
		if strings.EqualFold(attribute.Name.Local, name) {
			return attribute.Value
		}
	}
	return ""
}

func addChapterLabel(labels map[string]string, navigationPath, reference, label string, overwrite bool) {
	label = normalizeTOCLabel(label)
	reference = stripReferenceSuffix(strings.TrimSpace(reference))
	if label == "" || reference == "" || strings.HasPrefix(reference, "/") || strings.HasPrefix(reference, "//") || referenceHasScheme(reference) {
		return
	}
	resolved := path.Clean(path.Join(path.Dir(navigationPath), reference))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return
	}
	if _, exists := labels[resolved]; !exists || overwrite {
		labels[resolved] = label
	}
}

func stripReferenceSuffix(reference string) string {
	end := len(reference)
	if index := strings.IndexByte(reference, '#'); index >= 0 && index < end {
		end = index
	}
	if index := strings.IndexByte(reference, '?'); index >= 0 && index < end {
		end = index
	}
	return reference[:end]
}

func referenceHasScheme(reference string) bool {
	colon := strings.IndexByte(reference, ':')
	if colon < 0 {
		return false
	}
	slash := strings.IndexByte(reference, '/')
	return slash < 0 || colon < slash
}

func normalizeTOCLabel(label string) string {
	return strings.Join(strings.Fields(label), " ")
}

func hasManifestProperty(properties, expected string) bool {
	for _, property := range strings.Fields(properties) {
		if strings.EqualFold(property, expected) {
			return true
		}
	}
	return false
}
func extractTitle(raw string) string {
	lower := strings.ToLower(raw)
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6", "title"} {
		start := strings.Index(lower, "<"+tag)
		if start < 0 {
			continue
		}
		start = strings.Index(lower[start:], ">") + start + 1
		if start <= 0 {
			continue
		}
		end := strings.Index(lower[start:], "</"+tag+">")
		if end >= 0 {
			return strings.TrimSpace(PlainTextFromHTML(raw[start : start+end]))
		}
	}
	return ""
}

func continuedChapterTitle(previousTitle, language string) string {
	language = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(language), "_", "-"))
	if language == "ru" || strings.HasPrefix(language, "ru-") {
		return previousTitle + " (продолжение)"
	}
	return previousTitle + " (continued)"
}
func (p *EPUBParser) ParseTableOfContents(ctx context.Context, f BookFile) ([]TOCItem, error) {
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
func (p *EPUBParser) ExtractAssets(ctx context.Context, f BookFile) ([]Asset, error) {
	e, err := p.parse(f)
	if err != nil {
		return nil, err
	}
	meta, err := p.ParseMetadata(ctx, f)
	if err != nil {
		return nil, err
	}
	out := []Asset{}
	for _, item := range e.manifest {
		if !strings.HasPrefix(item.MediaType, "image/") {
			continue
		}
		data, err := e.a.read(item.Path, e.a.limits.MaxEntryBytes)
		if err != nil {
			return nil, err
		}
		out = append(out, Asset{SourceRef: item.Path, MediaType: item.MediaType, Data: data, IsCover: item.Path == meta.CoverRef})
	}
	return out, nil
}
func (p *EPUBParser) ExtractPlainText(ctx context.Context, f BookFile) (string, error) {
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

var _ = errors.Is
