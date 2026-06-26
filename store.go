package searcher

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ErrNotFound is returned by Day when no Mass exists for the given date.
var ErrNotFound = errors.New("searcher: no mass for date")

// Dialect selects the SQL placeholder style for the caller's driver. Queries
// are written with "?" and rewritten as needed (only Postgres differs).
type Dialect int

const (
	SQLite   Dialect = iota // "?"            (modernc.org/sqlite, mattn/go-sqlite3)
	MySQL                   // "?"            (go-sql-driver/mysql)
	Postgres                // "$1", "$2", ... (lib/pq, jackc/pgx)
)

// Store persists Masses in any database/sql backend. The caller owns the
// *sql.DB (and its driver), so the same code works across SQLite, Postgres,
// MySQL, etc.
type Store struct {
	db      *sql.DB
	dialect Dialect
	table   string
}

// Option configures a Store.
type Option func(*Store)

// WithDialect sets the SQL dialect (default SQLite).
func WithDialect(d Dialect) Option { return func(s *Store) { s.dialect = d } }

// WithTable overrides the table name (default "masses"). The name is a trusted
// identifier — do not pass untrusted input.
func WithTable(name string) Option {
	return func(s *Store) {
		if isIdent(name) {
			s.table = name
		}
	}
}

// NewStore returns a Store over db. Call Migrate once before use.
func NewStore(db *sql.DB, opts ...Option) *Store {
	s := &Store{db: db, dialect: SQLite, table: "masses"}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Migrate creates the masses table if it does not already exist. The column
// types (INTEGER/TEXT) are portable across SQLite, Postgres and MySQL.
func (s *Store) Migrate(ctx context.Context) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	day_key     INTEGER PRIMARY KEY,
	date        TEXT NOT NULL,
	weekday     TEXT NOT NULL,
	name        TEXT NOT NULL,
	season      TEXT NOT NULL,
	feast       TEXT NOT NULL,
	description TEXT NOT NULL
)`, s.table)
	if _, err := s.db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("searcher: migrate: %w", err)
	}
	return nil
}

// SnapshotYear fetches a whole year from src and replaces that year's stored
// Masses in a single transaction. It returns the number of Masses persisted.
func (s *Store) SnapshotYear(ctx context.Context, src Source, year int) (int, error) {
	masses, err := src.FetchYear(ctx, year)
	if err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("searcher: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	lo, hi := year*10000+101, year*10000+1231 // [YYYY0101, YYYY1231]
	del := s.rebind(fmt.Sprintf("DELETE FROM %s WHERE day_key BETWEEN ? AND ?", s.table))
	if _, err := tx.ExecContext(ctx, del, lo, hi); err != nil {
		return 0, fmt.Errorf("searcher: clear %d: %w", year, err)
	}
	if err := s.insert(ctx, tx, masses); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("searcher: commit: %w", err)
	}
	return len(masses), nil
}

// Save upserts the given Masses (keyed by calendar day) in one transaction.
func (s *Store) Save(ctx context.Context, masses []Mass) error {
	if len(masses) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("searcher: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	del := s.rebind(fmt.Sprintf("DELETE FROM %s WHERE day_key = ?", s.table))
	for _, m := range masses {
		if _, err := tx.ExecContext(ctx, del, dayKey(m.Date)); err != nil {
			return fmt.Errorf("searcher: replace %s: %w", m.Date.Format("2006-01-02"), err)
		}
	}
	if err := s.insert(ctx, tx, masses); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("searcher: commit: %w", err)
	}
	return nil
}

// Day returns the Mass for a single calendar date, or ErrNotFound. The date's
// clock time and zone are ignored — only Y/M/D matter.
func (s *Store) Day(ctx context.Context, date time.Time) (Mass, error) {
	q := s.rebind(fmt.Sprintf(
		"SELECT date, weekday, name, season, feast, description FROM %s WHERE day_key = ?", s.table))
	m, err := scanMass(s.db.QueryRowContext(ctx, q, dayKey(date)))
	if errors.Is(err, sql.ErrNoRows) {
		return Mass{}, fmt.Errorf("%w: %s", ErrNotFound, date.Format("2006-01-02"))
	}
	return m, err
}

// Range returns every Mass with a date in [from, to] inclusive, ordered by
// date. Arguments may be passed in either order.
func (s *Store) Range(ctx context.Context, from, to time.Time) ([]Mass, error) {
	if to.Before(from) {
		from, to = to, from
	}
	q := s.rebind(fmt.Sprintf(
		"SELECT date, weekday, name, season, feast, description FROM %s WHERE day_key BETWEEN ? AND ? ORDER BY day_key",
		s.table))
	rows, err := s.db.QueryContext(ctx, q, dayKey(from), dayKey(to))
	if err != nil {
		return nil, fmt.Errorf("searcher: range: %w", err)
	}
	defer rows.Close()

	var out []Mass
	for rows.Next() {
		m, err := scanMass(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// insert writes masses with the open transaction (no delete — callers clear
// first as appropriate).
func (s *Store) insert(ctx context.Context, tx *sql.Tx, masses []Mass) error {
	ins := s.rebind(fmt.Sprintf(
		"INSERT INTO %s (day_key, date, weekday, name, season, feast, description) VALUES (?, ?, ?, ?, ?, ?, ?)",
		s.table))
	for _, m := range masses {
		_, err := tx.ExecContext(ctx, ins,
			dayKey(m.Date), m.Date.Format(time.RFC3339),
			m.Weekday, m.Name, m.Season, m.Feast, m.Description)
		if err != nil {
			return fmt.Errorf("searcher: insert %s: %w", m.Date.Format("2006-01-02"), err)
		}
	}
	return nil
}

// rebind converts "?" placeholders to the dialect's style. Only Postgres
// ("$1", "$2", ...) differs from the default.
func (s *Store) rebind(q string) string {
	if s.dialect != Postgres {
		return q
	}
	var b strings.Builder
	b.Grow(len(q) + 8)
	n := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

func scanMass(sc scanner) (Mass, error) {
	var (
		m       Mass
		dateStr string
	)
	if err := sc.Scan(&dateStr, &m.Weekday, &m.Name, &m.Season, &m.Feast, &m.Description); err != nil {
		return Mass{}, err
	}
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return Mass{}, fmt.Errorf("searcher: bad stored date %q: %w", dateStr, err)
	}
	m.Date = t
	return m, nil
}

// dayKey collapses a date to a comparable YYYYMMDD integer, ignoring clock time
// and zone so callers can pass civil dates directly.
func dayKey(t time.Time) int {
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}

// isIdent reports whether s is a safe SQL identifier ([A-Za-z_][A-Za-z0-9_]*).
func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
