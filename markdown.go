package searcher

import (
	"strings"
	"unicode"

	"golang.org/x/net/html"
)

// nodeToMarkdown renders an HTML node's contents as Markdown. It is used to
// turn rich "Nội dung lễ" cells into a Markdown description. Supported markup:
// p/div (paragraphs), br (line break), b/strong, i/em, a (links), ul/ol/li
// (bullets) and h1–h6. Unknown elements are passed through as their content.
func nodeToMarkdown(n *html.Node) string {
	var b strings.Builder
	renderMarkdown(&b, n)
	return normalizeMarkdown(b.String())
}

func renderMarkdown(b *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.WriteString(collapseInline(n.Data))
		return
	case html.ElementNode:
		switch n.Data {
		case "br":
			b.WriteByte('\n')
			return
		case "p", "div":
			b.WriteString("\n\n")
			renderChildren(b, n)
			b.WriteString("\n\n")
			return
		case "b", "strong":
			wrapInline(b, n, "**")
			return
		case "i", "em":
			wrapInline(b, n, "*")
			return
		case "a":
			b.WriteByte('[')
			renderChildren(b, n)
			b.WriteString("](")
			b.WriteString(attr(n, "href"))
			b.WriteByte(')')
			return
		case "li":
			b.WriteString("\n- ")
			renderChildren(b, n)
			return
		case "ul", "ol":
			b.WriteByte('\n')
			renderChildren(b, n)
			b.WriteByte('\n')
			return
		case "h1", "h2", "h3", "h4", "h5", "h6":
			level := int(n.Data[1] - '0')
			b.WriteString("\n\n" + strings.Repeat("#", level) + " ")
			renderChildren(b, n)
			b.WriteString("\n\n")
			return
		case "script", "style":
			return
		}
	}
	renderChildren(b, n)
}

func renderChildren(b *strings.Builder, n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderMarkdown(b, c)
	}
}

// wrapInline renders n's children surrounded by marker, keeping any surrounding
// whitespace outside the markers so "**bold** next" stays valid.
func wrapInline(b *strings.Builder, n *html.Node, marker string) {
	var inner strings.Builder
	renderChildren(&inner, n)
	s := inner.String()
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		b.WriteString(s)
		return
	}
	if strings.HasPrefix(s, " ") {
		b.WriteByte(' ')
	}
	b.WriteString(marker + trimmed + marker)
	if strings.HasSuffix(s, " ") {
		b.WriteByte(' ')
	}
}

// collapseInline collapses any run of whitespace to a single space, preserving
// a single leading/trailing space so inline elements stay separated. Real line
// breaks come from block/br elements, not source-formatting whitespace.
func collapseInline(s string) string {
	var b strings.Builder
	pending := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			pending = true
			continue
		}
		if pending {
			b.WriteByte(' ')
			pending = false
		}
		b.WriteRune(r)
	}
	if pending {
		b.WriteByte(' ')
	}
	return b.String()
}

// normalizeMarkdown trims each line, drops blank lines down to at most one, and
// trims the whole string.
func normalizeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(l)
	}
	s = strings.Join(lines, "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}
