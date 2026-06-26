package searcher

import (
	"fmt"
	"strings"
	"time"
)

// Mass is a single day's celebration in the liturgical calendar.
type Mass struct {
	Date        time.Time // calendar date
	Weekday     string    // Vietnamese weekday, e.g. "Chủ Nhật"
	Name        string    // celebration name, e.g. "Chúa Hiển Linh"
	Season      string    // liturgical season, e.g. "Mùa Giáng Sinh"
	Feast       string    // rank, e.g. "Lễ trọng", "Lễ nhớ", "Chủ Nhật", "" = feria
	Description string    // full "Nội dung lễ" content as Markdown (CGVDT only)
}

// String renders the Mass as a Markdown message ready to send to Zalo.
func (m Mass) String() string {
	var b strings.Builder

	// Header: "📅 Chủ Nhật, 04/01/2026"
	b.WriteString("📅 ")
	if m.Weekday != "" {
		b.WriteString(m.Weekday + ", ")
	}
	b.WriteString(m.Date.Format("02/01/2006"))
	b.WriteByte('\n')

	if m.Name != "" {
		fmt.Fprintf(&b, "**%s**\n", m.Name)
	}

	if meta := m.meta(); meta != "" {
		fmt.Fprintf(&b, "_%s_\n", meta)
	}

	if m.Description != "" {
		b.WriteByte('\n')
		b.WriteString(m.Description)
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}

// Brief renders a compact one-block summary (date, name, meta) without the full
// description — used for multi-day digests like the weekly view.
func (m Mass) Brief() string {
	var b strings.Builder
	if m.Weekday != "" {
		b.WriteString(m.Weekday + " ")
	}
	b.WriteString(m.Date.Format("02/01"))
	if m.Name != "" {
		fmt.Fprintf(&b, ": **%s**", m.Name)
	}
	if meta := m.meta(); meta != "" {
		fmt.Fprintf(&b, " _%s_", meta)
	}
	return b.String()
}

// meta joins the feast rank and season into "Lễ trọng · Mùa Giáng Sinh".
func (m Mass) meta() string {
	var parts []string
	if m.Feast != "" {
		parts = append(parts, m.Feast)
	}
	if m.Season != "" {
		parts = append(parts, m.Season)
	}
	return strings.Join(parts, " · ")
}
