package searcher

import (
	"strings"

	"golang.org/x/net/html"
)

// attr returns the value of the named attribute, or "" if absent.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// hasClass reports whether n's class attribute contains the given class.
func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

// text returns the concatenated text content of n and its descendants.
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// cleanText trims and collapses internal whitespace to single spaces.
func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// firstMatch returns the first descendant of n (preorder) for which match is
// true, or nil.
func firstMatch(n *html.Node, match func(*html.Node) bool) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && match(c) {
			return c
		}
		if found := firstMatch(c, match); found != nil {
			return found
		}
	}
	return nil
}

// elementByClass returns the first element with the given tag and class.
func elementByClass(root *html.Node, tag, class string) *html.Node {
	return firstMatch(root, func(n *html.Node) bool {
		return n.Data == tag && hasClass(n, class)
	})
}

// childCells returns the direct <td> children of a <tr>.
func childCells(tr *html.Node) []*html.Node {
	var tds []*html.Node
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			tds = append(tds, c)
		}
	}
	return tds
}

// rowsWithin collects every <tr> descendant of root in document order, without
// descending into nested <table> elements (so footer/inner tables are skipped).
func rowsWithin(root *html.Node) []*html.Node {
	var rows []*html.Node
	var walk func(n *html.Node, isRoot bool)
	walk = func(n *html.Node, isRoot bool) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			if c.Data == "table" && !isRoot {
				continue // don't recurse into nested tables
			}
			if c.Data == "tr" {
				rows = append(rows, c)
			}
			walk(c, false)
		}
	}
	walk(root, true)
	return rows
}
