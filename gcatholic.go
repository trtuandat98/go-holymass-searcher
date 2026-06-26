package searcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/html"
)

const (
	gcatholicBaseURL = "https://gcatholic.org/calendar"
	gcatholicDiocese = "VN-than0-vi" // Archdiocese of Hồ Chí Minh City (Vietnamese)
	gcatholicSunday  = "Chủ Nhật"
)

// GCatholic scrapes a year of Masses from gcatholic.org.
type GCatholic struct {
	httpClient *http.Client
	diocese    string
}

// NewGCatholic returns a GCatholic source. An empty diocese uses the default
// (Archdiocese of Hồ Chí Minh City).
func NewGCatholic(diocese string) *GCatholic {
	if diocese == "" {
		diocese = gcatholicDiocese
	}
	return &GCatholic{
		httpClient: &http.Client{Timeout: htmlClientTimeout},
		diocese:    diocese,
	}
}

func (g *GCatholic) FetchYear(ctx context.Context, year int) ([]Mass, error) {
	url := fmt.Sprintf("%s/%d/%s", gcatholicBaseURL, year, g.diocese)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gcatholic: build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gcatholic: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcatholic: %s returned %d", url, resp.StatusCode)
	}
	return parseGCatholic(resp.Body, year)
}

// parseGCatholic walks the table.tb calendar and emits one Mass per day (the
// primary, first-listed celebration), from 1 Jan to 31 Dec.
func parseGCatholic(r io.Reader, year int) ([]Mass, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("gcatholic: parse html: %w", err)
	}
	table := elementByClass(doc, "table", "tb")
	if table == nil {
		return nil, fmt.Errorf("gcatholic: table.tb not found")
	}

	var (
		masses     []Mass
		season     string
		curDate    time.Time
		curWeekday string
		haveDate   bool
		dayDone    bool
	)

	for _, tr := range rowsWithin(table) {
		// Season rows carry the season forward until the next one.
		if s := gcSeason(tr); s != "" {
			season = s
			continue
		}
		if hasClass(tr, "tbhd") {
			continue // month header
		}

		tds := childCells(tr)

		// A row with id="MMDD" starts a new day.
		if id := attr(tr, "id"); isMonthDay(id) {
			if d, ok := monthDay(year, id); ok {
				curDate, haveDate, dayDone = d, true, false
			}
			if wd := gcWeekday(tds); wd != "" {
				curWeekday = wd
			}
		}
		if !haveDate || dayDone {
			continue
		}

		// Emit the first celebration found for this day.
		ni := nameCellIndex(tds)
		if ni < 0 {
			continue
		}
		name := cleanText(text(tds[ni]))
		if name == "" {
			continue
		}
		feast := ""
		if ni > 0 {
			feast = gcRank(tds[ni-1])
		}
		if feast == "" && curWeekday == gcatholicSunday {
			feast = gcatholicSunday
		}

		masses = append(masses, Mass{
			Date:    curDate,
			Weekday: curWeekday,
			Name:    name,
			Season:  season,
			Feast:   feast,
		})
		dayDone = true
	}

	if len(masses) == 0 {
		return nil, fmt.Errorf("gcatholic: no masses parsed for %d", year)
	}
	return masses, nil
}

// gcSeason returns the season text of a season-only row, or "".
func gcSeason(tr *html.Node) string {
	if div := elementByClass(tr, "div", "season"); div != nil {
		return cleanText(text(div))
	}
	return ""
}

// gcWeekday returns the weekday from the cell that holds a span.zdate inside a
// center-aligned cell (the date-number cell is right-aligned, so it's excluded).
func gcWeekday(tds []*html.Node) string {
	for _, td := range tds {
		if attr(td, "align") != "center" {
			continue
		}
		if sp := elementByClass(td, "span", "zdate"); sp != nil {
			return cleanText(text(sp))
		}
	}
	return ""
}

// gcRank returns the celebration rank from a rank cell, e.g. "Lễ trọng". An
// empty cell (feria) yields "".
func gcRank(td *html.Node) string {
	if a := firstMatch(td, func(n *html.Node) bool { return n.Data == "a" }); a != nil {
		return cleanText(attr(a, "title"))
	}
	return ""
}

// nameCellIndex returns the index of the cell holding the celebration name
// (a <p class="indent">), or -1.
func nameCellIndex(tds []*html.Node) int {
	for i, td := range tds {
		if elementByClass(td, "p", "indent") != nil {
			return i
		}
	}
	return -1
}

// isMonthDay reports whether s is a 4-digit "MMDD" id.
func isMonthDay(s string) bool {
	if len(s) != 4 {
		return false
	}
	_, err := strconv.Atoi(s)
	return err == nil
}

// monthDay builds a date from a year and an "MMDD" id.
func monthDay(year int, id string) (time.Time, bool) {
	month, err1 := strconv.Atoi(id[:2])
	day, err2 := strconv.Atoi(id[2:])
	if err1 != nil || err2 != nil || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), true
}
