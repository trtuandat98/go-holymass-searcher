// Package searcher crawls a whole year of Catholic liturgical Masses from a
// configurable source and persists them in any database/sql backend for
// single-day and date-range queries.
//
// External users wire it up to their own choices:
//
//	src, _ := searcher.NewSource(searcher.SourceCGVDT)  // or SourceGCatholic
//	db, _ := sql.Open("sqlite", "masses.db")            // any database/sql driver
//	store := searcher.NewStore(db, searcher.WithDialect(searcher.SQLite))
//	_ = store.Migrate(ctx)
//	_, _ = store.SnapshotYear(ctx, src, 2026)           // crawl + persist the year
//	mass, _ := store.Day(ctx, time.Now())               // query back out
package searcher

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Source fetches a whole year of Masses, ordered from 1 Jan to 31 Dec.
type Source interface {
	FetchYear(ctx context.Context, year int) ([]Mass, error)
}

// SourceName identifies a built-in calendar source. Use it instead of bare
// strings so callers get compile-time names and a single source of truth.
type SourceName string

const (
	// SourceGCatholic scrapes gcatholic.org.
	SourceGCatholic SourceName = "gcatholic"
	// SourceCGVDT scrapes cgvdt.vn (Vietnamese, with full "Nội dung lễ").
	SourceCGVDT SourceName = "cgvdt"
)

const (
	// htmlClientTimeout bounds a single page fetch.
	htmlClientTimeout = 20 * time.Second
	// userAgent is sent on every source request.
	userAgent = "Mozilla/5.0 (searcher liturgical calendar crawler)"
)

// SourceNames returns every built-in source name. Handy for validating config
// or building a help message.
func SourceNames() []SourceName {
	return []SourceName{SourceGCatholic, SourceCGVDT}
}

// NewSource creates a built-in Source from its name, so callers can select one
// from configuration. Matching is case-insensitive.
func NewSource(name SourceName) (Source, error) {
	switch SourceName(strings.ToLower(string(name))) {
	case SourceGCatholic:
		return NewGCatholic(""), nil
	case SourceCGVDT:
		return NewCGVDT(), nil
	default:
		return nil, fmt.Errorf("searcher: unknown source %q (want %v)", name, SourceNames())
	}
}
