package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a durable Store backed by a SQLite database file.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens or creates the SQLite database at dbPath and ensures
// the schema is up to date.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=2000")
	if err != nil {
		return nil, fmt.Errorf("memory: open sqlite: %w", err)
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database connection.
func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_entries (
			id             TEXT PRIMARY KEY,
			type           TEXT NOT NULL DEFAULT 'short_term',
			domain         TEXT NOT NULL DEFAULT '',
			cognitive_kind TEXT NOT NULL DEFAULT '',
			title          TEXT NOT NULL DEFAULT '',
			content        TEXT NOT NULL DEFAULT '',
			source         TEXT NOT NULL DEFAULT '',
			session_id     TEXT NOT NULL DEFAULT '',
			cwd            TEXT NOT NULL DEFAULT '',
			embedding      BLOB,
			metadata       TEXT NOT NULL DEFAULT '{}',
			ttl_ns         INTEGER NOT NULL DEFAULT 0,
			created_at     TEXT NOT NULL,
			updated_at     TEXT NOT NULL,
			expire_at      TEXT NOT NULL DEFAULT ''
		);
		CREATE INDEX IF NOT EXISTS idx_memory_type ON memory_entries(type);
		CREATE INDEX IF NOT EXISTS idx_memory_domain ON memory_entries(domain);
		CREATE INDEX IF NOT EXISTS idx_memory_session ON memory_entries(session_id);
		CREATE INDEX IF NOT EXISTS idx_memory_expire ON memory_entries(expire_at) WHERE expire_at != '';
	`)
	return err
}

func (s *SQLiteStore) Upsert(ctx context.Context, entry Entry) error {
	if entry.ID == "" {
		return nil
	}
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	if entry.TTL > 0 && entry.ExpireAt.IsZero() {
		entry.ExpireAt = now.Add(entry.TTL)
	}
	metaJSON, _ := json.Marshal(entry.Metadata)
	if metaJSON == nil {
		metaJSON = []byte("{}")
	}
	var emb []byte
	if len(entry.Embedding) > 0 {
		emb = float32ToBytes(entry.Embedding)
	}
	expireAt := ""
	if !entry.ExpireAt.IsZero() {
		expireAt = entry.ExpireAt.Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_entries
			(id, type, domain, cognitive_kind, title, content, source, session_id, cwd, embedding, metadata, ttl_ns, created_at, updated_at, expire_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type=excluded.type, domain=excluded.domain, cognitive_kind=excluded.cognitive_kind,
			title=excluded.title, content=excluded.content, source=excluded.source,
			session_id=excluded.session_id, cwd=excluded.cwd, embedding=excluded.embedding,
			metadata=excluded.metadata, ttl_ns=excluded.ttl_ns, updated_at=excluded.updated_at,
			expire_at=excluded.expire_at`,
		entry.ID, string(entry.Type), string(entry.Domain), string(entry.CognitiveKind),
		entry.Title, entry.Content, string(entry.Source), entry.SessionID, entry.CWD,
		emb, string(metaJSON), int64(entry.TTL), entry.CreatedAt.Format(time.RFC3339Nano),
		entry.UpdatedAt.Format(time.RFC3339Nano), expireAt)
	return err
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (Entry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, type, domain, cognitive_kind, title, content, source, session_id, cwd, embedding, metadata, ttl_ns, created_at, updated_at, expire_at FROM memory_entries WHERE id=?`, id)
	return scanEntry(row)
}

func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memory_entries WHERE id=?`, id)
	return err
}

func (s *SQLiteStore) Query(ctx context.Context, f Filter) ([]Entry, error) {
	where, args := buildWhere(f)
	query := `SELECT id, type, domain, cognitive_kind, title, content, source, session_id, cwd, embedding, metadata, ttl_ns, created_at, updated_at, expire_at FROM memory_entries` + where + ` ORDER BY updated_at DESC`
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	if f.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", f.Offset)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []Entry
	for rows.Next() {
		e, err := scanRow(rows)
		if err != nil {
			return entries, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (s *SQLiteStore) QuerySimilar(ctx context.Context, text string, f Filter, k int) ([]Hit, error) {
	entries, err := s.Query(ctx, f)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(entries))
	textLower := strings.ToLower(text)
	for _, e := range entries {
		score := simpleTextScore(textLower, e)
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{Entry: e, Score: score})
	}
	if k > 0 && len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

func (s *SQLiteStore) Count(ctx context.Context, f Filter) (int, error) {
	where, args := buildWhere(f)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_entries`+where, args...).Scan(&count)
	return count, err
}

func (s *SQLiteStore) PurgeExpired(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM memory_entries WHERE expire_at != '' AND expire_at < ?`, time.Now().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	n, _ := result.RowsAffected()
	return int(n), nil
}

// Internal helpers

func buildWhere(f Filter) (string, []any) {
	var clauses []string
	var args []any
	addClause := func(sql string, arg any) { clauses = append(clauses, sql); args = append(args, arg) }
	if len(f.Types) > 0 {
		placeholders := make([]string, len(f.Types))
		for i, t := range f.Types {
			placeholders[i] = "?"
			args = append(args, string(t))
		}
		clauses = append(clauses, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(f.Domains) > 0 {
		placeholders := make([]string, len(f.Domains))
		for i, d := range f.Domains {
			placeholders[i] = "?"
			args = append(args, string(d))
		}
		clauses = append(clauses, fmt.Sprintf("domain IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(f.CognitiveKinds) > 0 {
		placeholders := make([]string, len(f.CognitiveKinds))
		for i, c := range f.CognitiveKinds {
			placeholders[i] = "?"
			args = append(args, string(c))
		}
		clauses = append(clauses, fmt.Sprintf("cognitive_kind IN (%s)", strings.Join(placeholders, ",")))
	}
	if f.SessionID != "" {
		addClause("session_id = ?", f.SessionID)
	}
	if f.CWD != "" {
		addClause("cwd = ?", f.CWD)
	}
	if f.TitleContains != "" {
		addClause("title LIKE ?", "%"+f.TitleContains+"%")
	}
	if f.ContentContains != "" {
		addClause("content LIKE ?", "%"+f.ContentContains+"%")
	}
	clauses = append(clauses, "(expire_at = '' OR expire_at > ?)")
	args = append(args, time.Now().Format(time.RFC3339Nano))
	if len(clauses) == 1 {
		// Only the expire clause; no user filter.
	}
	if len(clauses) > 0 {
		return " WHERE " + strings.Join(clauses, " AND "), args
	}
	return "", nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(row rowScanner) (Entry, error) {
	var (
		id, typ, domain, cogKind, title, content, source, sessionID, cwd string
		emb                                                               []byte
		metaJSON, createdAt, updatedAt, expireAt                         string
		ttlNs                                                             int64
	)
	err := row.Scan(&id, &typ, &domain, &cogKind, &title, &content, &source, &sessionID, &cwd, &emb, &metaJSON, &ttlNs, &createdAt, &updatedAt, &expireAt)
	if err != nil {
		return Entry{}, err
	}
	e := Entry{
		ID:            id,
		Type:          Type(typ),
		Domain:        Domain(domain),
		CognitiveKind: CognitiveKind(cogKind),
		Title:         title,
		Content:       content,
		Source:        Source(source),
		SessionID:     sessionID,
		CWD:           cwd,
		Embedding:     bytesToFloat32(emb),
		TTL:           time.Duration(ttlNs),
	}
	if metaJSON != "" {
		_ = json.Unmarshal([]byte(metaJSON), &e.Metadata)
	}
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		e.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		e.UpdatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, expireAt); err == nil {
		e.ExpireAt = t
	}
	return e, nil
}

func scanEntry(row *sql.Row) (Entry, error) {
	var (
		id, typ, domain, cogKind, title, content, source, sessionID, cwd string
		emb                                                               []byte
		metaJSON, createdAt, updatedAt, expireAt                         string
		ttlNs                                                             int64
	)
	err := row.Scan(&id, &typ, &domain, &cogKind, &title, &content, &source, &sessionID, &cwd, &emb, &metaJSON, &ttlNs, &createdAt, &updatedAt, &expireAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Entry{}, nil
		}
		return Entry{}, err
	}
	e := Entry{
		ID:            id,
		Type:          Type(typ),
		Domain:        Domain(domain),
		CognitiveKind: CognitiveKind(cogKind),
		Title:         title,
		Content:       content,
		Source:        Source(source),
		SessionID:     sessionID,
		CWD:           cwd,
		Embedding:     bytesToFloat32(emb),
		TTL:           time.Duration(ttlNs),
	}
	if metaJSON != "" {
		_ = json.Unmarshal([]byte(metaJSON), &e.Metadata)
	}
	if t, err := time.Parse(time.RFC3339Nano, createdAt); err == nil {
		e.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		e.UpdatedAt = t
	}
	if t, err := time.Parse(time.RFC3339Nano, expireAt); err == nil {
		e.ExpireAt = t
	}
	return e, nil
}

func float32ToBytes(f []float32) []byte {
	if len(f) == 0 {
		return nil
	}
	b := make([]byte, len(f)*4)
	for i, v := range f {
		bits := math.Float32bits(v)
		b[i*4] = byte(bits)
		b[i*4+1] = byte(bits >> 8)
		b[i*4+2] = byte(bits >> 16)
		b[i*4+3] = byte(bits >> 24)
	}
	return b
}

func bytesToFloat32(b []byte) []float32 {
	if len(b) == 0 || len(b)%4 != 0 {
		return nil
	}
	f := make([]float32, len(b)/4)
	for i := range f {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		f[i] = math.Float32frombits(bits)
	}
	return f
}
