package searcher

import (
	"os"
	"strings"
	"testing"
)

func TestParseCGVDT_RealPage(t *testing.T) {
	f, err := os.Open("testdata/cgvdt_2026.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	masses, err := parseCGVDT(f, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// A full year of days across 12 month tables.
	if len(masses) < 360 || len(masses) > 366 {
		t.Errorf("want ~365 masses, got %d", len(masses))
	}
	if got := masses[0].Date; !got.Equal(date(2026, 1, 1)) {
		t.Errorf("first date = %v, want 2026-01-01", got)
	}
	if got := masses[len(masses)-1].Date; !got.Equal(date(2026, 12, 31)) {
		t.Errorf("last date = %v, want 2026-12-31", got)
	}

	// Jan 1: Đức Maria Mẹ Thiên Chúa, Lễ trọng, Mùa Giáng Sinh.
	jan1 := masses[0]
	if jan1.Weekday != "Thứ Năm" {
		t.Errorf("Jan 1 weekday = %q, want Thứ Năm", jan1.Weekday)
	}
	if jan1.Name != "Đức Maria Mẹ Thiên Chúa" {
		t.Errorf("Jan 1 name = %q", jan1.Name)
	}
	if jan1.Feast != "Lễ trọng" {
		t.Errorf("Jan 1 feast = %q, want Lễ trọng", jan1.Feast)
	}
	if jan1.Season != "Mùa Giáng Sinh" {
		t.Errorf("Jan 1 season = %q, want Mùa Giáng Sinh", jan1.Season)
	}

	// Description is Markdown: includes the name + scripture readings, and the
	// "Lời chúa (0) / Chia sẻ / Hạnh các thánh" UI counters are stripped.
	if !strings.Contains(jan1.Description, "Đức Maria Mẹ Thiên Chúa") {
		t.Errorf("description missing name:\n%s", jan1.Description)
	}
	if !strings.Contains(jan1.Description, "Lc 2:16-21") {
		t.Errorf("description missing readings:\n%s", jan1.Description)
	}
	if strings.Contains(jan1.Description, "Lời chúa") || strings.Contains(jan1.Description, "Chia sẻ") {
		t.Errorf("description should not contain UI counters:\n%s", jan1.Description)
	}

	t.Logf("Jan 1 description:\n%s", jan1.Description)
}
