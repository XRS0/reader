package bookprocessing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeHTMLExtractsXHTMLBodyWithoutDoubleEscaping(t *testing.T) {
	raw := `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head>
    <title/>
    <link rel="stylesheet" type="text/css" href="../styles/book.css"/>
    <style type="text/css"/>
    <script>headSecret()</script>
  </head>
  <body class="book" onload="steal()">
    <style type="text/css"/>
    <div class="title" id="chapter-one" style="color:red" onclick="steal()">Semantic <em>chapter</em></div>
    <div class="title6">Deep heading</div>
    <p class="p1">Tom &amp; Jerry read <strong>carefully</strong>.<script>bodySecret()</script></p>
    <p class="empty-line"/>
    <ul><li>First</li><li>Second</li></ul>
    <blockquote>Quoted<sup>2</sup><sub>x</sub></blockquote>
    <img src="../images/cover.jpg" alt="Cover" onerror="steal()" style="display:none"/>
    <a href="javascript:alert(1)">unsafe URL</a>
    <a href="../text/chapter-two.xhtml#next" onclick="steal()">next chapter</a>
  </body>
</html>`

	safe := SanitizeHTML(raw)
	lower := strings.ToLower(safe)

	require.Contains(t, safe, `<h1 id="chapter-one">Semantic <em>chapter</em></h1>`)
	require.Contains(t, safe, `<h6>Deep heading</h6>`)
	require.Contains(t, safe, `<p>Tom &amp; Jerry read <strong>carefully</strong>.</p>`)
	require.Contains(t, safe, `<p><br`)
	require.Contains(t, safe, "<ul><li>First</li><li>Second</li></ul>")
	require.Contains(t, safe, "<blockquote>Quoted<sup>2</sup><sub>x</sub></blockquote>")
	require.Contains(t, safe, `src="../images/cover.jpg"`)
	require.Contains(t, safe, `href="../text/chapter-two.xhtml#next"`)
	require.NotContains(t, safe, "&lt;h1")
	require.NotContains(t, safe, "&amp;amp;")
	require.NotContains(t, lower, "<html")
	require.NotContains(t, lower, "<head")
	require.NotContains(t, lower, "<body")
	require.NotContains(t, lower, "<link")
	require.NotContains(t, lower, "<script")
	require.NotContains(t, lower, "<style")
	require.NotContains(t, lower, "headsecret")
	require.NotContains(t, lower, "bodysecret")
	require.NotContains(t, lower, "javascript:")
	require.NotContains(t, lower, "onload")
	require.NotContains(t, lower, "onclick")
	require.NotContains(t, lower, "onerror")
	require.NotContains(t, lower, "style=")
	require.NotContains(t, lower, "class=")
}

func TestSanitizeHTMLFallsBackForHTMLWithNamedEntities(t *testing.T) {
	raw := `<html><head><title/></head><body><h2>Fallback</h2><p>one&nbsp;two &copy; three</p><script/><style/><textarea/><p>AFTER</p></body></html>`

	safe := SanitizeHTML(raw)

	require.Contains(t, safe, "<h2>Fallback</h2>")
	require.Contains(t, safe, "<p>one\u00a0two © three</p>")
	require.Contains(t, safe, "<p>AFTER</p>")
	require.NotContains(t, safe, "&lt;h2")
	require.NotContains(t, safe, "&lt;p")
	require.NotContains(t, strings.ToLower(safe), "<script")
	require.NotContains(t, strings.ToLower(safe), "<style")
	require.NotContains(t, strings.ToLower(safe), "<textarea")
}

func TestSanitizeHTMLFlattensMultiParagraphEPUBTitle(t *testing.T) {
	raw := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title/></head><body><div class="title"><p class="p1">Author</p><p class="p1"><strong>Book title</strong></p></div></body></html>`

	safe := SanitizeHTML(raw)

	require.Equal(t, `<h1>Author<br/><strong>Book title</strong></h1>`, safe)
	require.NotContains(t, safe, "<h1><p")
}

func TestSanitizeHTMLRendersVoidElementsOnce(t *testing.T) {
	raw := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title/></head><body><p>A<br/>B<img src="../images/p.png" alt="P"/></p></body></html>`

	safe := SanitizeHTML(raw)

	require.Equal(t, 1, strings.Count(strings.ToLower(safe), "<br"))
	require.Equal(t, 1, strings.Count(strings.ToLower(safe), "<img"))
	require.NotContains(t, strings.ToLower(safe), "</br>")
	require.NotContains(t, strings.ToLower(safe), "</img>")
}

func TestSanitizeHTMLUnwrapsParagraphInsideNormalizedHeading(t *testing.T) {
	raw := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title/></head><body>
<div class="title"><p class="p1">ВВЕДЕНИЕ <em>в книгу</em></p></div>
<div class="title6"><p class="p">Глубокий заголовок</p></div>
</body></html>`

	safe := SanitizeHTML(raw)

	require.Contains(t, safe, `<h1>ВВЕДЕНИЕ <em>в книгу</em></h1>`)
	require.Contains(t, safe, `<h6>Глубокий заголовок</h6>`)
	require.NotContains(t, safe, "<h1><p")
	require.NotContains(t, safe, "<h6><p")
}
