package searcher

import (
	"os"
	"testing"
	"time"
)

func TestParseGCatholic_RealPage(t *testing.T) {
	f, err := os.Open("testdata/gcatholic_2026.html")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	masses, err := parseGCatholic(f, 2026)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// One Mass per day for a full (365-day) year.
	if len(masses) != 365 {
		t.Errorf("want 365 masses, got %d", len(masses))
	}
	if got := masses[0].Date; !got.Equal(date(2026, 1, 1)) {
		t.Errorf("first date = %v, want 2026-01-01", got)
	}
	if got := masses[len(masses)-1].Date; !got.Equal(date(2026, 12, 31)) {
		t.Errorf("last date = %v, want 2026-12-31", got)
	}

	byDate := func(m, d int) Mass {
		t.Helper()
		want := date(2026, m, d)
		for _, ms := range masses {
			if ms.Date.Equal(want) {
				return ms
			}
		}
		t.Fatalf("no mass for %02d-%02d", m, d)
		return Mass{}
	}

	cases := []struct {
		m, d                         int
		weekday, name, season, feast string
	}{
		{1, 1, "Thứ Năm", "Thánh Ma-ri-a, Ðức Mẹ Chúa Trời", "Mùa Giáng Sinh", "Lễ trọng"},
		{1, 4, "Chủ Nhật", "Chúa Hiển Linh", "Mùa Giáng Sinh", "Lễ trọng"},
		{1, 5, "Thứ Hai", "Thứ Hai sau Lễ Hiển Linh", "Mùa Giáng Sinh", ""},
		{1, 7, "Thứ Tư", "Thứ Tư sau Lễ Hiển Linh", "Mùa Giáng Sinh", ""}, // primary (feria), not the optional memorial
		{1, 11, "Chủ Nhật", "Chúa Giê-su Chịu Phép Rửa", "Mùa Giáng Sinh", "Lễ kính"},
	}
	for _, tc := range cases {
		ms := byDate(tc.m, tc.d)
		if ms.Weekday != tc.weekday || ms.Name != tc.name || ms.Season != tc.season || ms.Feast != tc.feast {
			t.Errorf("%02d-%02d:\n got  wd=%q name=%q season=%q feast=%q\n want wd=%q name=%q season=%q feast=%q",
				tc.m, tc.d, ms.Weekday, ms.Name, ms.Season, ms.Feast, tc.weekday, tc.name, tc.season, tc.feast)
		}
	}
}

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}
