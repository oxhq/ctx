package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/oxhq/ctx/internal/model"
)

type DB struct {
	dir     string
	facts   *sql.DB
	sources *sql.DB
}

func Open(ctxDir string) (*DB, error) {
	if err := os.MkdirAll(ctxDir, 0o755); err != nil {
		return nil, err
	}
	facts, err := sql.Open("sqlite", filepath.Join(ctxDir, "facts.db"))
	if err != nil {
		return nil, err
	}
	sources, err := sql.Open("sqlite", filepath.Join(ctxDir, "sources.db"))
	if err != nil {
		_ = facts.Close()
		return nil, err
	}
	db := &DB{dir: ctxDir, facts: facts, sources: sources}
	if err := db.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return errors.Join(db.facts.Close(), db.sources.Close())
}

func (db *DB) Dir() string {
	return db.dir
}

func (db *DB) migrate() error {
	factDDL := `
CREATE TABLE IF NOT EXISTS facts (
	id TEXT PRIMARY KEY,
	key TEXT NOT NULL,
	value TEXT NOT NULL,
	source TEXT NOT NULL,
	source_path TEXT NOT NULL,
	source_hash TEXT NOT NULL,
	confidence REAL NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	expires_at TEXT,
	stale INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_facts_key ON facts(key);
CREATE TABLE IF NOT EXISTS states (
	ref TEXT PRIMARY KEY,
	packet TEXT NOT NULL,
	created_at TEXT NOT NULL
);`
	sourceDDL := `
CREATE TABLE IF NOT EXISTS sources (
	id TEXT PRIMARY KEY,
	root TEXT NOT NULL,
	path TEXT NOT NULL,
	abs_path TEXT NOT NULL,
	hash TEXT NOT NULL,
	kind TEXT NOT NULL,
	size INTEGER NOT NULL,
	stale INTEGER NOT NULL DEFAULT 0,
	modified_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_path ON sources(root, path);`
	if _, err := db.facts.Exec(factDDL); err != nil {
		return err
	}
	_, err := db.sources.Exec(sourceDDL)
	return err
}

func (db *DB) UpsertFacts(facts []model.Fact, sources []model.Source) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	roots := map[string]bool{}
	txSources, err := db.sources.Begin()
	if err != nil {
		return err
	}
	defer txSources.Rollback()
	for _, source := range sources {
		roots[source.Root] = true
		_, err := txSources.Exec(`INSERT INTO sources (id, root, path, abs_path, hash, kind, size, stale, modified_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)
ON CONFLICT(id) DO UPDATE SET root=excluded.root, path=excluded.path, abs_path=excluded.abs_path, hash=excluded.hash, kind=excluded.kind, size=excluded.size, stale=0, modified_at=excluded.modified_at`,
			source.ID, source.Root, source.Path, source.AbsPath, source.Hash, source.Kind, source.Size, source.Modified.Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
	}
	for root := range roots {
		seen := map[string]bool{}
		for _, source := range sources {
			if source.Root == root {
				seen[source.ID] = true
			}
		}
		rows, err := txSources.Query(`SELECT id FROM sources WHERE root = ? AND stale = 0`, root)
		if err != nil {
			return err
		}
		var missing []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return err
			}
			if !seen[id] {
				missing = append(missing, id)
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, id := range missing {
			if _, err := txSources.Exec(`UPDATE sources SET stale = 1 WHERE id = ?`, id); err != nil {
				return err
			}
			if _, err := db.facts.Exec(`UPDATE facts SET stale = 1, updated_at = ? WHERE source = ?`, now, id); err != nil {
				return err
			}
		}
	}
	if err := txSources.Commit(); err != nil {
		return err
	}
	if len(facts) == 0 && len(sources) == 0 {
		if _, err := db.sources.Exec(`UPDATE sources SET stale = 1 WHERE stale = 0`); err != nil {
			return err
		}
		if _, err := db.facts.Exec(`UPDATE facts SET stale = 1, updated_at = ? WHERE stale = 0`, now); err != nil {
			return err
		}
	}

	txFacts, err := db.facts.Begin()
	if err != nil {
		return err
	}
	defer txFacts.Rollback()
	for _, fact := range facts {
		expiresAt := ""
		if fact.ExpiresAt != nil {
			expiresAt = fact.ExpiresAt.Format(time.RFC3339Nano)
		}
		created := fact.CreatedAt.Format(time.RFC3339Nano)
		if fact.CreatedAt.IsZero() {
			created = now
		}
		updated := fact.UpdatedAt.Format(time.RFC3339Nano)
		if fact.UpdatedAt.IsZero() {
			updated = now
		}
		_, err := txFacts.Exec(`INSERT INTO facts (id, key, value, source, source_path, source_hash, confidence, created_at, updated_at, expires_at, stale)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
ON CONFLICT(id) DO UPDATE SET key=excluded.key, value=excluded.value, source=excluded.source, source_path=excluded.source_path, source_hash=excluded.source_hash, confidence=excluded.confidence, updated_at=excluded.updated_at, expires_at=excluded.expires_at, stale=0`,
			fact.ID, fact.Key, string(fact.Value), fact.Source, fact.SourcePath, fact.SourceHash, fact.Confidence, created, updated, expiresAt)
		if err != nil {
			return err
		}
	}
	return txFacts.Commit()
}

func (db *DB) CountFacts() (int, error) {
	var count int
	err := db.facts.QueryRow(`SELECT COUNT(*) FROM facts`).Scan(&count)
	return count, err
}

func (db *DB) StaleSources() ([]model.Source, error) {
	return db.querySources(`SELECT id, root, path, abs_path, hash, kind, size, stale, modified_at FROM sources WHERE stale = 1 ORDER BY path`)
}

func (db *DB) Sources(includeStale bool) ([]model.Source, error) {
	query := `SELECT id, root, path, abs_path, hash, kind, size, stale, modified_at FROM sources`
	if !includeStale {
		query += ` WHERE stale = 0`
	}
	query += ` ORDER BY path`
	return db.querySources(query)
}

func (db *DB) Facts(includeStale bool) ([]model.Fact, error) {
	query := `SELECT id, key, value, source, source_path, source_hash, confidence, created_at, updated_at, expires_at, stale FROM facts`
	if !includeStale {
		query += ` WHERE stale = 0`
	}
	query += ` ORDER BY key, source_path, id`
	rows, err := db.facts.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var facts []model.Fact
	for rows.Next() {
		var fact model.Fact
		var value string
		var created, updated, expires string
		var stale int
		if err := rows.Scan(&fact.ID, &fact.Key, &value, &fact.Source, &fact.SourcePath, &fact.SourceHash, &fact.Confidence, &created, &updated, &expires, &stale); err != nil {
			return nil, err
		}
		fact.Value = json.RawMessage(value)
		fact.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		fact.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		if expires != "" {
			parsed, _ := time.Parse(time.RFC3339Nano, expires)
			fact.ExpiresAt = &parsed
		}
		fact.Stale = stale == 1
		facts = append(facts, fact)
	}
	return facts, rows.Err()
}

func (db *DB) PutState(ref string, packet []byte) error {
	_, err := db.facts.Exec(`INSERT INTO states (ref, packet, created_at) VALUES (?, ?, ?)
ON CONFLICT(ref) DO UPDATE SET packet=excluded.packet, created_at=excluded.created_at`, ref, string(packet), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (db *DB) GetState(ref string) ([]byte, error) {
	var packet string
	if err := db.facts.QueryRow(`SELECT packet FROM states WHERE ref = ?`, ref).Scan(&packet); err != nil {
		return nil, err
	}
	return []byte(packet), nil
}

func (db *DB) querySources(query string, args ...any) ([]model.Source, error) {
	rows, err := db.sources.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sources []model.Source
	for rows.Next() {
		var source model.Source
		var modified string
		var stale int
		if err := rows.Scan(&source.ID, &source.Root, &source.Path, &source.AbsPath, &source.Hash, &source.Kind, &source.Size, &stale, &modified); err != nil {
			return nil, err
		}
		source.Stale = stale == 1
		source.Modified, _ = time.Parse(time.RFC3339Nano, modified)
		sources = append(sources, source)
	}
	return sources, rows.Err()
}
