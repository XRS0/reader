package bookprocessing

import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTXTParserEscapesHTML(t *testing.T) {
	p := NewTXTParser()
	f := BookFile{Name: "sample.txt", Data: []byte("Chapter 1\n<script>alert(1)</script> & text")}
	chapters, err := p.ExtractChapters(context.Background(), f)
	require.NoError(t, err)
	require.NotContains(t, chapters[0].HTML, "<script>")
	require.Contains(t, chapters[0].HTML, "&lt;script&gt;")
}

func TestFB2Parser(t *testing.T) {
	raw := `<?xml version="1.0"?><FictionBook><description><title-info><author><first-name>Ada</first-name><last-name>Lovelace</last-name></author><book-title>Example</book-title><lang>en</lang></title-info></description><body><section id="one"><title><p>First</p></title><p>Hello &amp; welcome.</p></section></body></FictionBook>`
	p := NewFB2Parser()
	f := BookFile{Name: "example.fb2", Data: []byte(raw)}
	m, err := p.ParseMetadata(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, "Example", m.Title)
	chapters, err := p.ExtractChapters(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, "First", chapters[0].Title)
	require.Contains(t, chapters[0].HTML, "Hello &amp; welcome")
}

func TestEPUBParserAndSanitization(t *testing.T) {
	f := syntheticEPUB(t, strings.Repeat("safe text ", 10)+`<script>alert(1)</script>`)
	p := NewEPUBParser(ArchiveLimits{MaxFiles: 20, MaxUnpackedBytes: 1024 * 1024, MaxCompressionRatio: 100, MaxEntryBytes: 1024 * 1024})
	ok, err := p.Detect(context.Background(), f)
	require.NoError(t, err)
	require.True(t, ok)
	m, err := p.ParseMetadata(context.Background(), f)
	require.NoError(t, err)
	require.Equal(t, "Synthetic", m.Title)
	chapters, err := p.ExtractChapters(context.Background(), f)
	require.NoError(t, err)
	require.Len(t, chapters, 1)
	require.NotContains(t, chapters[0].HTML, "script")
}

func TestEPUBParserExtractsSafeBodyFromFullXHTMLDocument(t *testing.T) {
	xhtml := `<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title/>
    <link rel="stylesheet" href="../styles/book.css"/>
    <style type="text/css"/>
  </head>
  <body>
    <div class="title2">Heading From Class</div>
    <p class="p1">Text with <em>emphasis</em> and <strong>importance</strong>.</p>
    <p class="empty-line"/>
    <ol><li>First</li><li>Second</li></ol>
    <blockquote>Quote<sup>2</sup><sub>x</sub></blockquote>
    <img src="../images/illustration.png" alt="Illustration" onerror="alert(1)"/>
    <script>alert(1)</script>
  </body>
</html>`
	f := syntheticFullDocumentEPUB(t, xhtml)
	p := NewEPUBParser(ArchiveLimits{MaxFiles: 20, MaxUnpackedBytes: 1024 * 1024, MaxCompressionRatio: 100, MaxEntryBytes: 1024 * 1024})

	chapters, err := p.ExtractChapters(context.Background(), f)
	require.NoError(t, err)
	require.Len(t, chapters, 2, "an empty spine document must be skipped")
	require.Equal(t, "NCX Semantic Label", chapters[0].Title)
	require.Equal(t, "NCX Semantic Label (continued)", chapters[1].Title)
	require.Equal(t, "OEBPS/text/continuation.xhtml", chapters[1].SourceRef)
	require.Contains(t, chapters[0].HTML, "<h2>Heading From Class</h2>")
	require.Contains(t, chapters[0].HTML, "<em>emphasis</em>")
	require.Contains(t, chapters[0].HTML, "<strong>importance</strong>")
	require.Contains(t, chapters[0].HTML, "<p><br")
	require.Contains(t, chapters[0].HTML, `src="../images/illustration.png"`)
	require.NotContains(t, chapters[0].HTML, "&lt;body")
	require.NotContains(t, chapters[0].HTML, "<link")
	require.NotContains(t, chapters[0].HTML, "<style")
	require.NotContains(t, chapters[0].HTML, "<script")
	require.NotContains(t, chapters[0].HTML, "onerror")
	require.NotContains(t, chapters[0].HTML, "class=")
	require.Contains(t, chapters[0].PlainText, "Text with emphasis and importance")
	require.Contains(t, chapters[1].PlainText, "Continuation text")

	assets, err := p.ExtractAssets(context.Background(), f)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Equal(t, "OEBPS/images/illustration.png", assets[0].SourceRef)
}

func TestEPUB3NavigationLabelsMapByPathWithoutFragment(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <body>
    <nav epub:type="landmarks"><ol><li><a href="cover.xhtml">Cover</a></li></ol></nav>
    <nav epub:type="toc"><ol><li><a href="text/chapter.xhtml#start"><span> EPUB 3 </span> Chapter </a></li></ol></nav>
  </body>
</html>`)

	links := parseEPUB3Navigation(raw)
	require.Len(t, links, 1)
	labels := map[string]string{}
	addChapterLabel(labels, "OEBPS/nav.xhtml", links[0].Reference, links[0].Label, false)
	require.Equal(t, "EPUB 3 Chapter", labels["OEBPS/text/chapter.xhtml"])
}

func TestContinuedChapterTitleUsesBookLanguage(t *testing.T) {
	require.Equal(t, "Уроборос (продолжение)", continuedChapterTitle("Уроборос", "ru-RU"))
	require.Equal(t, "Uroboros (continued)", continuedChapterTitle("Uroboros", "en"))
}

func TestExtractTitleAcceptsAllSemanticHeadingLevels(t *testing.T) {
	require.Equal(t, "Deep heading", extractTitle(`<p>Preface</p><h6>Deep <em>heading</em></h6>`))
}

func TestEPUBZipBombRejected(t *testing.T) {
	f := syntheticEPUB(t, strings.Repeat("A", 2*1024*1024))
	p := NewEPUBParser(ArchiveLimits{MaxFiles: 20, MaxUnpackedBytes: 10 * 1024 * 1024, MaxCompressionRatio: 10, MaxEntryBytes: 5 * 1024 * 1024})
	_, err := p.Detect(context.Background(), f)
	require.ErrorIs(t, err, ErrArchiveLimit)
}

func syntheticEPUB(t *testing.T, body string) BookFile {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	mime, err := w.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	require.NoError(t, err)
	_, _ = mime.Write([]byte("application/epub+zip"))
	files := map[string]string{"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`, "OEBPS/content.opf": `<?xml version="1.0"?><package><metadata><title>Synthetic</title><creator>BookFlow</creator><language>en</language></metadata><manifest><item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`, "OEBPS/c1.xhtml": "<html><body><h1>One</h1><p>" + body + "</p></body></html>"}
	for name, data := range files {
		x, err := w.Create(name)
		require.NoError(t, err)
		_, _ = x.Write([]byte(data))
	}
	require.NoError(t, w.Close())
	return BookFile{Name: "synthetic.epub", Data: b.Bytes()}
}

func syntheticFullDocumentEPUB(t *testing.T, xhtml string) BookFile {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	mimetype, err := writer.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	require.NoError(t, err)
	_, err = mimetype.Write([]byte("application/epub+zip"))
	require.NoError(t, err)
	files := map[string][]byte{
		"META-INF/container.xml":        []byte(`<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`),
		"OEBPS/content.opf":             []byte(`<?xml version="1.0"?><package><metadata><title>Full document</title><creator>BookFlow</creator><language>en</language></metadata><manifest><item id="c1" href="text/chapter.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="text/continuation.xhtml" media-type="application/xhtml+xml"/><item id="empty" href="text/empty.xhtml" media-type="application/xhtml+xml"/><item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/><item id="css" href="styles/book.css" media-type="text/css"/><item id="image" href="images/illustration.png" media-type="image/png"/></manifest><spine toc="ncx"><itemref idref="c1"/><itemref idref="c2"/><itemref idref="empty"/></spine></package>`),
		"OEBPS/toc.ncx":                 []byte(`<?xml version="1.0"?><ncx><navMap><navPoint id="one"><navLabel><text> NCX Semantic Label </text></navLabel><content src="text/chapter.xhtml#chapter-one"/></navPoint></navMap></ncx>`),
		"OEBPS/text/chapter.xhtml":      []byte(xhtml),
		"OEBPS/text/continuation.xhtml": []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title/></head><body><p class="p">Continuation text.</p></body></html>`),
		"OEBPS/text/empty.xhtml":        []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title/></head><body><p class="empty-line"/></body></html>`),
		"OEBPS/styles/book.css":         []byte(`body { color: red; }`),
		"OEBPS/images/illustration.png": []byte("synthetic-image"),
	}
	for name, data := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return BookFile{Name: "full-document.epub", MediaType: "application/epub+zip", Data: buffer.Bytes()}
}
