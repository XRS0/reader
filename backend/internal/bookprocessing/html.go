package bookprocessing

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var bookPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("id", "title").OnElements("section", "article", "p", "h1", "h2", "h3", "h4", "h5", "h6", "span", "div", "blockquote", "pre", "code", "ol", "ul", "li", "table", "thead", "tbody", "tr", "th", "td", "sup", "sub")
	p.AllowAttrs("href").Matching(bluemonday.SpaceSeparatedTokens).OnElements("a")
	p.AllowAttrs("src", "alt", "title").OnElements("img")
	return p
}()

var selfClosingRawTextElement = regexp.MustCompile(`(?i)<(script|style|title|textarea|xmp|iframe|noembed|noframes)([^<>]*?)/\s*>`)

// SanitizeHTML accepts either an HTML fragment or a complete EPUB XHTML
// document. XHTML frequently uses XML self-closing tags such as <title/> and
// <style/>. Passing a complete such document directly to an HTML5 tokenizer
// makes the tokenizer enter a raw-text state and treat the rest of the file as
// text, which double-escapes the actual chapter markup. Extracting the body as
// XML first preserves the document semantics and keeps all head resources out
// of the reader fragment.
func SanitizeHTML(raw string) string {
	fragment := extractBodyFragment(raw)
	// An XML self-closing raw-text element is not self-closing in HTML5. Expand
	// it before bluemonday's HTML tokenizer sees it, otherwise malformed
	// HTML-ish EPUB fallback content after the tag would become escaped text.
	fragment = selfClosingRawTextElement.ReplaceAllString(fragment, `<$1$2></$1>`)
	return strings.TrimSpace(bookPolicy.Sanitize(fragment))
}

func extractBodyFragment(raw string) string {
	if body, ok := extractXMLBody(raw); ok {
		return body
	}
	if body, ok := sliceBodyFragment(raw); ok {
		return body
	}
	return raw
}

func extractXMLBody(raw string) (string, bool) {
	decoder := xml.NewDecoder(strings.NewReader(normalizeHTMLEntitiesForXML(raw)))
	root := &html.Node{Type: html.DocumentNode}
	stack := []*html.Node{root}
	inBody := false
	depth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", false
		}

		switch typed := token.(type) {
		case xml.StartElement:
			if !inBody {
				if strings.EqualFold(typed.Name.Local, "body") {
					inBody = true
					depth = 1
				}
				continue
			}
			depth++
			element, emptyLine := normalizeXHTMLElement(typed)
			node := &html.Node{Type: html.ElementNode, Data: element.Name.Local, Attr: make([]html.Attribute, 0, len(element.Attr))}
			for _, attribute := range element.Attr {
				node.Attr = append(node.Attr, html.Attribute{Key: attribute.Name.Local, Val: attribute.Value})
			}
			stack[len(stack)-1].AppendChild(node)
			stack = append(stack, node)
			if emptyLine {
				node.AppendChild(&html.Node{Type: html.ElementNode, Data: "br"})
			}
		case xml.EndElement:
			if !inBody {
				continue
			}
			if depth == 1 {
				if !strings.EqualFold(typed.Name.Local, "body") {
					return "", false
				}
				unwrapRedundantHeadingParagraphs(root)
				return renderHTMLChildren(root)
			}
			if len(stack) <= 1 {
				return "", false
			}
			stack = stack[:len(stack)-1]
			depth--
		case xml.CharData:
			if inBody {
				stack[len(stack)-1].AppendChild(&html.Node{Type: html.TextNode, Data: string(typed)})
			}
		case xml.Comment:
			// Reader content does not need comments, and omitting them prevents
			// conditional-comment tricks from reaching the HTML sanitizer.
		case xml.Directive, xml.ProcInst:
			// XML declarations and processing instructions are document-level
			// data and are never part of a rendered chapter.
		}
	}
	return "", false
}

// Some EPUB generators express a semantic title as a classed block wrapping
// one or more paragraphs, for example
// <div class="title"><p>Author</p><p>Title</p></div>. Once the outer block is
// normalized to a heading, retaining those paragraphs would produce invalid
// HTML that browsers repair inconsistently. Flatten an all-paragraph heading,
// preserving inline markup and separating source paragraphs with <br>.
func unwrapRedundantHeadingParagraphs(root *html.Node) {
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		unwrapRedundantHeadingParagraphs(node)
	}
	if root.Type != html.ElementNode || len(root.Data) != 2 || root.Data[0] != 'h' || root.Data[1] < '1' || root.Data[1] > '6' {
		return
	}

	paragraphs := make([]*html.Node, 0, 2)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode && strings.TrimSpace(child.Data) == "" {
			continue
		}
		if child.Type != html.ElementNode || child.Data != "p" {
			return
		}
		paragraphs = append(paragraphs, child)
	}
	if len(paragraphs) == 0 {
		return
	}

	for child := root.FirstChild; child != nil; {
		next := child.NextSibling
		root.RemoveChild(child)
		child = next
	}
	for index, paragraph := range paragraphs {
		if index > 0 {
			root.AppendChild(&html.Node{Type: html.ElementNode, Data: "br"})
		}
		for child := paragraph.FirstChild; child != nil; {
			next := child.NextSibling
			paragraph.RemoveChild(child)
			root.AppendChild(child)
			child = next
		}
	}
}

func renderHTMLChildren(root *html.Node) (string, bool) {
	var buffer bytes.Buffer
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&buffer, child); err != nil {
			return "", false
		}
	}
	return buffer.String(), true
}

// normalizeHTMLEntitiesForXML converts named HTML entities that XML does not
// know into numeric character references. Numeric references preserve whether
// the entity appeared in text or an attribute and cannot turn escaped markup
// into parsed elements.
func normalizeHTMLEntitiesForXML(raw string) string {
	var output strings.Builder
	output.Grow(len(raw))
	written := 0
	for cursor := 0; cursor < len(raw); cursor++ {
		if raw[cursor] != '&' || cursor+1 >= len(raw) || raw[cursor+1] == '#' {
			continue
		}
		semicolon := strings.IndexByte(raw[cursor+1:], ';')
		if semicolon < 0 || semicolon > 64 {
			continue
		}
		semicolon += cursor + 1
		candidate := raw[cursor : semicolon+1]
		decoded := html.UnescapeString(candidate)
		if decoded == candidate {
			continue
		}
		output.WriteString(raw[written:cursor])
		for _, value := range decoded {
			output.WriteString("&#x")
			output.WriteString(strconv.FormatInt(int64(value), 16))
			output.WriteByte(';')
		}
		written = semicolon + 1
		cursor = semicolon
	}
	if written == 0 {
		return raw
	}
	output.WriteString(raw[written:])
	return output.String()
}

func normalizeXHTMLElement(element xml.StartElement) (xml.StartElement, bool) {
	originalName := strings.ToLower(element.Name.Local)
	classNames := ""
	for _, attribute := range element.Attr {
		if attribute.Name.Space == "" && strings.EqualFold(attribute.Name.Local, "class") {
			classNames = attribute.Value
			break
		}
	}
	element.Name = xml.Name{Local: semanticElementName(originalName, classNames)}
	attributes := make([]xml.Attr, 0, len(element.Attr))
	for _, attribute := range element.Attr {
		if attribute.Name.Local == "xmlns" || attribute.Name.Space == "xmlns" {
			continue
		}
		if attribute.Name.Space != "" {
			// Preserve xml:lang as ordinary lang. Other namespaced attributes
			// are EPUB/XML metadata and are not needed by reader HTML.
			if attribute.Name.Space == "http://www.w3.org/XML/1998/namespace" && attribute.Name.Local == "lang" {
				attribute.Name = xml.Name{Local: "lang"}
			} else {
				continue
			}
		} else {
			attribute.Name = xml.Name{Local: attribute.Name.Local}
		}
		attributes = append(attributes, attribute)
	}
	element.Attr = attributes
	return element, originalName == "p" && hasClass(classNames, "empty-line")
}

func semanticElementName(original, classNames string) string {
	if original != "div" && original != "p" && original != "span" {
		return original
	}
	for _, className := range strings.Fields(strings.ToLower(classNames)) {
		switch className {
		case "title", "title1":
			return "h1"
		case "title2":
			return "h2"
		case "title3":
			return "h3"
		case "title4":
			return "h4"
		case "title5":
			return "h5"
		case "title6":
			return "h6"
		}
	}
	return original
}

func hasClass(classNames, expected string) bool {
	for _, className := range strings.Fields(classNames) {
		if strings.EqualFold(className, expected) {
			return true
		}
	}
	return false
}

// sliceBodyFragment is a tolerant fallback for HTML-ish EPUB files that are
// not valid XML (for example, they contain undeclared named entities). It only
// strips the outer body element; the resulting fragment is still passed
// through the allowlist sanitizer.
func sliceBodyFragment(raw string) (string, bool) {
	_, contentStart, ok := findBodyTag(raw, 0, false)
	if !ok {
		return "", false
	}
	closingStart, _, ok := findBodyTag(raw, contentStart, true)
	if !ok {
		return raw[contentStart:], true
	}
	return raw[contentStart:closingStart], true
}

func findBodyTag(raw string, from int, closing bool) (int, int, bool) {
	for from < len(raw) {
		relative := strings.IndexByte(raw[from:], '<')
		if relative < 0 {
			return 0, 0, false
		}
		start := from + relative
		cursor := start + 1
		if cursor >= len(raw) {
			return 0, 0, false
		}
		isClosing := raw[cursor] == '/'
		if isClosing {
			cursor++
		}
		if isClosing != closing || cursor >= len(raw) || raw[cursor] == '!' || raw[cursor] == '?' {
			from = start + 1
			continue
		}
		nameStart := cursor
		for cursor < len(raw) && isTagNameByte(raw[cursor]) {
			cursor++
		}
		if cursor == nameStart {
			from = start + 1
			continue
		}
		name := raw[nameStart:cursor]
		if colon := strings.LastIndexByte(name, ':'); colon >= 0 {
			name = name[colon+1:]
		}
		if !strings.EqualFold(name, "body") || (cursor < len(raw) && !isTagBoundary(raw[cursor])) {
			from = start + 1
			continue
		}
		end, ok := findTagEnd(raw, cursor)
		if !ok {
			return 0, 0, false
		}
		return start, end + 1, true
	}
	return 0, 0, false
}

func findTagEnd(raw string, from int) (int, bool) {
	var quote byte
	for i := from; i < len(raw); i++ {
		switch {
		case quote != 0 && raw[i] == quote:
			quote = 0
		case quote == 0 && (raw[i] == '\'' || raw[i] == '"'):
			quote = raw[i]
		case quote == 0 && raw[i] == '>':
			return i, true
		}
	}
	return 0, false
}

func isTagNameByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == ':' || value == '_' || value == '-' || value == '.'
}

func isTagBoundary(value byte) bool {
	return value == '>' || value == '/' || value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func PlainTextFromHTML(raw string) string {
	root, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	var b bytes.Buffer
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			v := strings.TrimSpace(n.Data)
			if v != "" {
				if b.Len() > 0 {
					b.WriteByte(' ')
				}
				b.WriteString(v)
			}
		}
		if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div" || n.Data == "section" || n.Data == "br" || strings.HasPrefix(n.Data, "h")) {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return normalizeWhitespace(b.String())
}
func normalizeWhitespace(s string) string {
	var b strings.Builder
	lastSpace := false
	newlines := 0
	for _, r := range strings.ReplaceAll(s, "\r\n", "\n") {
		if r == '\n' {
			if b.Len() > 0 && newlines < 2 {
				b.WriteRune(r)
			}
			newlines++
			lastSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteByte(' ')
			}
			lastSpace = true
			continue
		}
		b.WriteRune(r)
		lastSpace = false
		newlines = 0
	}
	return strings.TrimSpace(b.String())
}
