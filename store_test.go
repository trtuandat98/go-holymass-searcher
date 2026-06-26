package searcher

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func day(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }

// fakeSource returns a fixed set of Masses, ignoring the requested year.
type fakeSource struct{ masses []Mass }

func (f fakeSource) FetchYear(context.Context, int) ([]Mass, error) { return f.masses, nil }

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	// :memory: is per-connection; pin to one so schema + data share a DB.
	db.SetMaxOpenConns(1)

	s := NewStore(db)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s
}

func seed(t *testing.T, s *Store, masses ...Mass) {
	t.Helper()
	if err := s.Save(context.Background(), masses); err != nil {
		t.Fatalf("save: %v", err)
	}
}

func TestStoreDay(t *testing.T) {
	s := newTestStore(t)
	seed(t,
		s,
		Mass{Date: day(2026, 1, 2), Name: "Ngày 2", Weekday: "Thứ Sáu", Feast: "Lễ nhớ", Season: "Mùa Thường Niên"},
		Mass{Date: day(2026, 1, 3), Name: "Ngày 3"},
	)

	m, err := s.Day(context.Background(), day(2026, 1, 2))
	if err != nil {
		t.Fatalf("Day: %v", err)
	}
	if m.Name != "Ngày 2" || m.Weekday != "Thứ Sáu" || m.Feast != "Lễ nhớ" || m.Season != "Mùa Thường Niên" {
		t.Errorf("round-trip mismatch: %+v", m)
	}

	// A civil date with a clock time / different zone still matches.
	loc := time.FixedZone("ICT", 7*3600)
	m, err = s.Day(context.Background(), time.Date(2026, 1, 3, 9, 30, 0, 0, loc))
	if err != nil || m.Name != "Ngày 3" {
		t.Errorf("got (%q, %v), want Ngày 3", m.Name, err)
	}
}

func TestStoreDayNotFound(t *testing.T) {
	s := newTestStore(t)
	seed(t, s, Mass{Date: day(2026, 1, 1), Name: "Tết"})
	if _, err := s.Day(context.Background(), day(2026, 6, 15)); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStoreRange(t *testing.T) {
	s := newTestStore(t)
	seed(t,
		s,
		Mass{Date: day(2026, 1, 1), Name: "Tết"},
		Mass{Date: day(2026, 1, 2), Name: "Ngày 2"},
		Mass{Date: day(2026, 1, 3), Name: "Ngày 3"},
		Mass{Date: day(2026, 12, 31), Name: "Cuối năm"},
		Mass{Date: day(2027, 1, 1), Name: "Năm mới"},
	)

	got, err := s.Range(context.Background(), day(2026, 1, 1), day(2026, 1, 3))
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if len(got) != 3 || got[0].Name != "Tết" || got[2].Name != "Ngày 3" {
		t.Errorf("want [Tết..Ngày 3], got %v", names(got))
	}

	// Reversed args yield the same window.
	if rev, _ := s.Range(context.Background(), day(2026, 1, 3), day(2026, 1, 1)); len(rev) != 3 {
		t.Errorf("reversed args: want 3, got %d", len(rev))
	}

	// Spanning the year boundary.
	span, err := s.Range(context.Background(), day(2026, 12, 31), day(2027, 1, 1))
	if err != nil {
		t.Fatalf("Range span: %v", err)
	}
	if len(span) != 2 || span[0].Name != "Cuối năm" || span[1].Name != "Năm mới" {
		t.Errorf("want [Cuối năm, Năm mới], got %v", names(span))
	}
}

func TestSnapshotYearReplaces(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	src := fakeSource{masses: []Mass{
		{Date: day(2026, 1, 1), Name: "A"},
		{Date: day(2026, 1, 2), Name: "B"},
	}}
	n, err := s.SnapshotYear(ctx, src, 2026)
	if err != nil || n != 2 {
		t.Fatalf("SnapshotYear: n=%d err=%v", n, err)
	}

	// Re-snapshotting the same year replaces rather than duplicating, and drops
	// days no longer present.
	src.masses = []Mass{{Date: day(2026, 1, 1), Name: "A2"}}
	if n, err = s.SnapshotYear(ctx, src, 2026); err != nil || n != 1 {
		t.Fatalf("re-snapshot: n=%d err=%v", n, err)
	}

	m, err := s.Day(ctx, day(2026, 1, 1))
	if err != nil || m.Name != "A2" {
		t.Errorf("want A2, got (%q, %v)", m.Name, err)
	}
	if _, err := s.Day(ctx, day(2026, 1, 2)); !errors.Is(err, ErrNotFound) {
		t.Errorf("day 2 should be gone, got %v", err)
	}
}

func names(ms []Mass) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Name
	}
	return out
}
