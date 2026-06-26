# searcher

A small Go library that crawls a whole year of Catholic liturgical **Masses**
from a configurable source and persists them in any `database/sql` backend for
single-day and date-range queries.

You choose:

- **the source** — `gcatholic` (gcatholic.org) or `cgvdt` (cgvdt.vn, Vietnamese
  with full "Nội dung lễ" content as Markdown);
- **the database** — bring your own `*sql.DB` with any driver (SQLite, Postgres,
  MySQL, …).

## Install

```sh
go get github.com/trtuandat98/searcher
```

## Usage

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trtuandat98/searcher"
	_ "modernc.org/sqlite" // any database/sql driver
)

func main() {
	ctx := context.Background()

	// 1. Pick a source.
	src, err := searcher.NewSource(searcher.SourceCGVDT) // or SourceGCatholic
	if err != nil {
		panic(err)
	}

	// 2. Open your database and build a Store.
	db, err := sql.Open("sqlite", "masses.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	store := searcher.NewStore(db, searcher.WithDialect(searcher.SQLite))
	if err := store.Migrate(ctx); err != nil {
		panic(err)
	}

	// 3. Crawl and persist a whole year (idempotent — re-running replaces it).
	n, err := store.SnapshotYear(ctx, src, 2026)
	if err != nil {
		panic(err)
	}
	fmt.Printf("stored %d masses\n", n)

	// 4. Query.
	today, _ := store.Day(ctx, time.Now())
	fmt.Println(today.String())

	week, _ := store.Range(ctx, time.Now(), time.Now().AddDate(0, 0, 7))
	fmt.Printf("%d masses this week\n", len(week))
}
```

## Sources

| Constant                  | Site         | Notes                                       |
| ------------------------- | ------------ | ------------------------------------------- |
| `searcher.SourceGCatholic`| gcatholic.org| Name, season, feast rank per day.           |
| `searcher.SourceCGVDT`    | cgvdt.vn     | Adds the full "Nội dung lễ" as Markdown.     |

`searcher.SourceNames()` lists them all; `searcher.NewSource(name)` matches
case-insensitively.

You can also construct sources directly: `searcher.NewGCatholic("")` /
`searcher.NewCGVDT()`.

## Database

The library is driver-agnostic — it never imports a driver. Pass the dialect so
placeholders are bound correctly:

| Dialect             | Drivers                                  | Placeholders |
| ------------------- | ---------------------------------------- | ------------ |
| `searcher.SQLite`   | `modernc.org/sqlite`, `mattn/go-sqlite3` | `?`          |
| `searcher.MySQL`    | `go-sql-driver/mysql`                    | `?`          |
| `searcher.Postgres` | `lib/pq`, `jackc/pgx`                    | `$1, $2, …`  |

Masses are stored in a `masses` table keyed by `day_key` (a `YYYYMMDD` integer),
which makes day and range lookups timezone-safe. Override the table name with
`searcher.WithTable("...")`.

## API

```go
src, err := searcher.NewSource(searcher.SourceCGVDT)

store := searcher.NewStore(db, opts...)        // WithDialect, WithTable
store.Migrate(ctx)                             // create table if absent
store.SnapshotYear(ctx, src, year)             // crawl + replace a year
store.Save(ctx, masses)                        // upsert specific masses
store.Day(ctx, date)                           // one day (searcher.ErrNotFound)
store.Range(ctx, from, to)                     // inclusive, ordered by date
```

`Mass` implements `String()` (full Markdown message) and `Brief()` (one-line
digest).
