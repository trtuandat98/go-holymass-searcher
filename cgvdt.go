package searcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const cgvdtURL = "https://www.cgvdt.vn/lich-cong-giao"

// CGVDT scrapes the Vietnamese liturgical calendar from cgvdt.vn. The page has
// one table per month with columns Ngày | Loại | Áo lễ | Nội dung lễ. Unlike
// GCatholic, it captures the full "Nội dung lễ" column as a Markdown
// description on each Mass.
type CGVDT struct {
	httpClient *http.Client
	url        string
}

func NewCGVDT() *CGVDT {
	return &CGVDT{
		httpClient: &http.Client{Timeout: htmlClientTimeout},
		url:        cgvdtURL,
	}
}

func (c *CGVDT) FetchYear(ctx context.Context, year int) ([]Mass, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, fmt.Errorf("cgvdt: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cgvdt: fetch %s: %w", c.url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cgvdt: %s returned %d", c.url, resp.StatusCode)
	}
	return parseCGVDT(resp.Body, year)
}

// Header keywords (normalized: lowercased, accent-stripped) for column lookup.
const (
	colDate    = "ngay"        // Ngày
	colType    = "loai"        // Loại (rank + season)
	colContent = "noi dung le" // Nội dung lễ
)

// parseCGVDT walks every month table and emits one Mass per day, capturing the
// "Nội dung lễ" column as a Markdown description.
func parseCGVDT(r io.Reader, year int) ([]Mass, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("cgvdt: parse html: %w", err)
	}

	tables := tablesWithHeader(doc, colContent)
	if len(tables) == 0 {
		return nil, fmt.Errorf("cgvdt: no table with a %q column found", colContent)
	}

	var masses []Mass
	for _, table := range tables {
		headers := headerIndex(table)
		dateIdx, hasDate := headers[colDate]
		typeIdx, hasType := headers[colType]
		contentIdx := headers[colContent]

		for _, tr := range rowsWithin(table) {
			if isHeaderRow(tr) {
				continue
			}
			tds := childCells(tr)
			if contentIdx >= len(tds) {
				continue
			}

			content := tds[contentIdx]
			removeNodes(content, func(n *html.Node) bool { return hasClass(n, "mt-2") })

			m := Mass{
				Name:        firstSpanText(content),
				Description: nodeToMarkdown(content),
			}
			if hasDate && dateIdx < len(tds) {
				m.Date, m.Weekday = parseNgay(tds[dateIdx], year)
			}
			if hasType && typeIdx < len(tds) {
				m.Feast, m.Season = parseLoai(tds[typeIdx])
			}
			if m.Description == "" && m.Name == "" {
				continue // spacer / empty row
			}
			masses = append(masses, m)
		}
	}

	if len(masses) == 0 {
		return nil, fmt.Errorf("cgvdt: no masses parsed for %d", year)
	}
	return masses, nil
}

var dayMonthRE = regexp.MustCompile(`(\d{1,2})\s*/\s*(\d{1,2})`)

// parseNgay reads a "Ngày" cell ("Thứ Năm <br> 1/1") into a date and weekday.
func parseNgay(td *html.Node, year int) (time.Time, string) {
	lines := textLines(td)
	weekday := ""
	if len(lines) > 0 {
		weekday = strings.TrimSpace(dayMonthRE.ReplaceAllString(lines[0], ""))
	}

	match := dayMonthRE.FindStringSubmatch(strings.Join(lines, " "))
	if match == nil {
		return time.Time{}, weekday
	}
	day, _ := strconv.Atoi(match[1])
	month, _ := strconv.Atoi(match[2])
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, weekday
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), weekday
}

// parseLoai splits a "Loại" cell ("Lễ trọng <br> Mùa Giáng Sinh") into the
// feast rank and the liturgical season.
func parseLoai(td *html.Node) (feast, season string) {
	for _, l := range textLines(td) {
		if isSeason(l) {
			if season == "" {
				season = l
			}
		} else if feast == "" {
			feast = l
		}
	}
	return feast, season
}

func isSeason(s string) bool {
	return strings.HasPrefix(s, "Mùa") || strings.HasPrefix(s, "Tam nhật")
}

// firstSpanText returns the text of the first <span> in a cell (the celebration
// name on cgvdt.vn), falling back to the first text line.
func firstSpanText(td *html.Node) string {
	if sp := firstMatch(td, func(n *html.Node) bool { return n.Data == "span" }); sp != nil {
		if s := cleanText(text(sp)); s != "" {
			return s
		}
	}
	if lines := textLines(td); len(lines) > 0 {
		return lines[0]
	}
	return ""
}

// textLines renders a node to Markdown and returns its non-empty lines. Handy
// for cells that pack several values separated by <br>.
func textLines(n *html.Node) []string {
	var out []string
	for _, l := range strings.Split(nodeToMarkdown(n), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// tablesWithHeader returns every <table> whose header row contains headerKey.
func tablesWithHeader(root *html.Node, headerKey string) []*html.Node {
	var tables []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			if h := headerIndex(n); h != nil {
				if _, ok := h[headerKey]; ok {
					tables = append(tables, n)
					return // don't recurse into a matched table
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return tables
}

// headerIndex maps each header cell's normalized text to its column index,
// reading the first row that contains a "Nội dung lễ" cell.
func headerIndex(table *html.Node) map[string]int {
	for _, tr := range rowsWithin(table) {
		var cells []*html.Node
		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && (c.Data == "th" || c.Data == "td") {
				cells = append(cells, c)
			}
		}
		if len(cells) == 0 {
			continue
		}
		idx := make(map[string]int, len(cells))
		for i, cell := range cells {
			idx[normalizeVN(cleanText(text(cell)))] = i
		}
		if _, ok := idx[colContent]; ok {
			return idx
		}
	}
	return nil
}

func isHeaderRow(tr *html.Node) bool {
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "th" {
			return true
		}
	}
	return false
}

// removeNodes detaches every descendant element matching pred.
func removeNodes(n *html.Node, pred func(*html.Node) bool) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		if c.Type == html.ElementNode && pred(c) {
			n.RemoveChild(c)
			continue
		}
		removeNodes(c, pred)
	}
}

// normalizeVN lowercases and strips Vietnamese diacritics so header matching is
// robust to accents (e.g. "Nội dung lễ" -> "noi dung le").
func normalizeVN(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if base, ok := vnFold[r]; ok {
			b.WriteRune(base)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// vnFold maps accented Vietnamese letters to their base ASCII letter.
var vnFold = func() map[rune]rune {
	groups := map[rune]string{
		'a': "àáạảãâầấậẩẫăằắặẳẵ",
		'e': "èéẹẻẽêềếệểễ",
		'i': "ìíịỉĩ",
		'o': "òóọỏõôồốộổỗơờớợởỡ",
		'u': "ùúụủũưừứựửữ",
		'y': "ỳýỵỷỹ",
		'd': "đ",
	}
	m := make(map[rune]rune)
	for base, accented := range groups {
		for _, r := range accented {
			m[r] = base
		}
	}
	return m
}()
