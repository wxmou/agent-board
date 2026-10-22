package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-board/internal/model"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound     = errors.New("message not found")
	ErrConflict     = errors.New("message version conflict")
	ErrArchived     = errors.New("message is archived")
	ErrLocked       = errors.New("message is being edited")
	ErrUnauthorized = errors.New("not allowed")
	ErrPrecondition = errors.New("expected_version is required")
)

type ConflictError struct {
	Current *model.Message
}

func (e *ConflictError) Error() string { return "message changed since it was loaded" }
func (e *ConflictError) Unwrap() error { return ErrConflict }

type LockedError struct {
	Lock *model.Lock
}

func (e *LockedError) Error() string { return "message is locked by another editor" }
func (e *LockedError) Unwrap() error { return ErrLocked }

type Store struct {
	db     *sql.DB
	dbPath string
}

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 5000;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    author TEXT NOT NULL,
    edited_by TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '[]',
    priority TEXT NOT NULL DEFAULT 'normal',
    pinned INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    client_request_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    archived_at INTEGER,
    archived_reason TEXT NOT NULL DEFAULT '',
    deleted_at INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_client_request
    ON messages(author, client_request_id) WHERE client_request_id <> '';
CREATE INDEX IF NOT EXISTS idx_messages_active
    ON messages(deleted_at, archived_at, pinned DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_archived
    ON messages(deleted_at, archived_at, archived_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_author ON messages(author);

CREATE TABLE IF NOT EXISTS message_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT NOT NULL,
    version INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    editor TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_revisions_message ON message_revisions(message_id, version DESC);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    message_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    metadata TEXT NOT NULL DEFAULT '{}',
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS archive_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    trigger_type TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    finished_at INTEGER,
    archived_count INTEGER NOT NULL DEFAULT 0,
    retention_count INTEGER NOT NULL DEFAULT 0,
    overflow_count INTEGER NOT NULL DEFAULT 0,
    remaining INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS edit_locks (
    message_id TEXT PRIMARY KEY,
    owner TEXT NOT NULL,
    token TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS mcp_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    role TEXT NOT NULL,
    prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER,
    expires_at INTEGER,
    revoked_at INTEGER,
    revoked_by TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_keys_active_name
    ON mcp_keys(name) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_mcp_keys_created ON mcp_keys(created_at DESC);

`

func Open(path string, defaults model.ArchivePolicy) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}
	dsn := path
	if path != ":memory:" {
		dsn = "file:" + filepath.ToSlash(path) + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// A single connection keeps SQLite writes deterministic and avoids SQLITE_BUSY
	// under a burst of agent writes. Read latency is negligible at board scale.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}
	s := &Store{db: db, dbPath: path}
	if defaults.RetainDays <= 0 {
		defaults.RetainDays = 7
	}
	if defaults.MaxItems <= 0 {
		defaults.MaxItems = 200
	}
	if defaults.IntervalMinutes <= 0 {
		defaults.IntervalMinutes = 5
	}
	seed := map[string]string{
		"archive.retain_days":      strconv.Itoa(defaults.RetainDays),
		"archive.max_items":        strconv.Itoa(defaults.MaxItems),
		"archive.auto":             strconv.FormatBool(defaults.AutoArchive),
		"archive.interval_minutes": strconv.Itoa(defaults.IntervalMinutes),
		"archive.pinned_protected": strconv.FormatBool(defaults.PinnedProtected),
	}
	for key, value := range seed {
		if _, err := db.Exec("INSERT OR IGNORE INTO settings(key,value) VALUES (?,?)", key, value); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("seed settings: %w", err)
		}
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) DBPath() string { return s.dbPath }

func nowMS() int64 { return time.Now().UTC().UnixMilli() }

func msTime(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func nullTime(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := msTime(n.Int64)
	return &t
}

type scanner interface {
	Scan(dest ...any) error
}

const messageColumns = `id, title, content, author, edited_by, tags, priority, pinned, version,
	client_request_id, created_at, updated_at, archived_at, archived_reason, deleted_at`

func scanMessage(row scanner) (model.Message, error) {
	var (
		m        model.Message
		tags     string
		pinned   int
		created  int64
		updated  int64
		archived sql.NullInt64
		deleted  sql.NullInt64
	)
	err := row.Scan(&m.ID, &m.Title, &m.Content, &m.Author, &m.EditedBy, &tags, &m.Priority,
		&pinned, &m.Version, &m.ClientRequestID, &created, &updated, &archived, &m.ArchivedReason, &deleted)
	if err != nil {
		return m, err
	}
	m.Pinned = pinned == 1
	m.CreatedAt = msTime(created)
	m.UpdatedAt = msTime(updated)
	m.ArchivedAt = nullTime(archived)
	m.DeletedAt = nullTime(deleted)
	if tags == "" {
		tags = "[]"
	}
	if err := json.Unmarshal([]byte(tags), &m.Tags); err != nil || m.Tags == nil {
		m.Tags = []string{}
	}
	return m, nil
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(strings.TrimPrefix(tag, "#"))
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > 32 {
			tag = string([]rune(tag)[:32])
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
		if len(out) == 8 {
			break
		}
	}
	return out
}

func validatePriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "urgent":
		return "urgent"
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "normal"
	}
}

func (s *Store) CreateMessage(ctx context.Context, author string, in model.CreateInput) (model.Message, bool, error) {
	title := strings.TrimSpace(in.Title)
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return model.Message{}, false, errors.New("content must not be empty")
	}
	if len([]rune(content)) > 32000 {
		return model.Message{}, false, errors.New("content exceeds 32000 characters")
	}
	if len([]rune(title)) > 240 {
		return model.Message{}, false, errors.New("title exceeds 240 characters")
	}
	in.Title = title
	in.Content = content
	in.Tags = normalizeTags(in.Tags)
	in.Priority = validatePriority(in.Priority)
	in.ClientRequestID = strings.TrimSpace(in.ClientRequestID)
	if len(in.ClientRequestID) > 200 {
		return model.Message{}, false, errors.New("client_request_id exceeds 200 characters")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, false, err
	}
	defer tx.Rollback()

	if in.ClientRequestID != "" {
		row := tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE author = ? AND client_request_id = ?", author, in.ClientRequestID)
		existing, scanErr := scanMessage(row)
		if scanErr == nil {
			return existing, false, nil
		}
		if !errors.Is(scanErr, sql.ErrNoRows) {
			return model.Message{}, false, scanErr
		}
	}

	id := newID()
	now := nowMS()
	tags, _ := json.Marshal(in.Tags)
	_, err = tx.ExecContext(ctx, `INSERT INTO messages
		(id,title,content,author,edited_by,tags,priority,pinned,version,client_request_id,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,0,1,?,?,?)`,
		id, in.Title, in.Content, author, "", string(tags), in.Priority, in.ClientRequestID, now, now)
	if err != nil {
		return model.Message{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO message_revisions
		(message_id,version,title,content,editor,created_at) VALUES (?,?,?,?,?,?)`,
		id, 1, in.Title, in.Content, author, now); err != nil {
		return model.Message{}, false, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(type,message_id,title,actor,detail,created_at)
		VALUES ('message.created',?,?,?,?,?)`, id, displayTitle(in.Title, in.Content), author, "created message", now); err != nil {
		return model.Message{}, false, err
	}
	row := tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	msg, err := scanMessage(row)
	if err != nil {
		return model.Message{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, false, err
	}
	return msg, true, nil
}

func displayTitle(title, content string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}
	r := []rune(strings.TrimSpace(strings.ReplaceAll(content, "\n", " ")))
	if len(r) > 48 {
		return string(r[:48]) + "…"
	}
	return string(r)
}

func (s *Store) GetMessage(ctx context.Context, id string) (model.Message, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return m, ErrNotFound
	}
	return m, err
}

type ListResult struct {
	Items      []model.Message
	Total      int
	Limit      int
	Offset     int
	NextCursor string
	HasMore    bool
}

func (s *Store) ListMessages(ctx context.Context, opts model.ListOptions) (ListResult, error) {
	if opts.Limit <= 0 {
		opts.Limit = 20
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	status := strings.ToLower(strings.TrimSpace(opts.Status))
	if status == "" {
		status = "active"
	}
	where := []string{"deleted_at IS NULL"}
	switch status {
	case "active":
		where = append(where, "archived_at IS NULL")
	case "archived":
		where = append(where, "archived_at IS NOT NULL")
	case "all":
		// active + archived, deleted still excluded
	default:
		return ListResult{}, errors.New("status must be active, archived or all")
	}
	args := []any{}
	if q := strings.TrimSpace(opts.Query); q != "" {
		like := "%" + strings.ReplaceAll(strings.ReplaceAll(q, "%", "\\%"), "_", "\\_") + "%"
		where = append(where, "(title LIKE ? ESCAPE '\\' OR content LIKE ? ESCAPE '\\' OR author LIKE ? ESCAPE '\\' OR tags LIKE ? ESCAPE '\\')")
		args = append(args, like, like, like, like)
	}
	if author := strings.TrimSpace(opts.Author); author != "" {
		where = append(where, "author = ?")
		args = append(args, author)
	}
	if tag := strings.TrimSpace(opts.Tag); tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, "%\""+strings.ReplaceAll(tag, "\"", "")+"\"%")
	}
	if p := strings.TrimSpace(opts.Priority); p != "" && p != "all" {
		where = append(where, "priority = ?")
		args = append(args, validatePriority(p))
	}
	if reason := strings.TrimSpace(opts.ArchivedReason); reason != "" {
		where = append(where, "archived_reason = ?")
		args = append(args, reason)
	}
	if opts.Pinned != nil {
		where = append(where, "pinned = ?")
		if *opts.Pinned {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	cursorUsed := false
	if opts.Cursor != "" {
		cur, err := decodeCursor(opts.Cursor)
		if err != nil {
			return ListResult{}, fmt.Errorf("invalid cursor: %w", err)
		}
		where = append(where, "(pinned < ? OR (pinned = ? AND (created_at < ? OR (created_at = ? AND id < ?))))")
		args = append(args, cur.Pinned, cur.Pinned, cur.CreatedAt, cur.CreatedAt, cur.ID)
		cursorUsed = true
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	countArgs := append([]any(nil), args...)
	if cursorUsed {
		// total is a filter total, independent of the cursor location.
		countArgs = countArgs[:len(countArgs)-5]
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM messages WHERE "+strings.Join(whereWithoutCursor(where, cursorUsed), " AND "), countArgs...).Scan(&total); err != nil {
		return ListResult{}, err
	}

	order := "pinned DESC, created_at DESC, id DESC"
	switch strings.ToLower(opts.Sort) {
	case "oldest":
		order = "pinned DESC, created_at ASC, id ASC"
	case "updated":
		order = "pinned DESC, updated_at DESC, id DESC"
	case "priority":
		order = "pinned DESC, CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END, created_at DESC"
	}
	queryArgs := append([]any(nil), args...)
	queryArgs = append(queryArgs, opts.Limit+1, opts.Offset)
	rows, err := s.db.QueryContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE "+whereSQL+" ORDER BY "+order+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()
	items := make([]model.Message, 0, opts.Limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	hasMore := len(items) > opts.Limit
	if hasMore {
		items = items[:opts.Limit]
	}
	result := ListResult{Items: items, Total: total, Limit: opts.Limit, Offset: opts.Offset, HasMore: hasMore}
	if hasMore && len(items) > 0 && strings.ToLower(opts.Sort) != "updated" && strings.ToLower(opts.Sort) != "oldest" && strings.ToLower(opts.Sort) != "priority" {
		last := items[len(items)-1]
		result.NextCursor = encodeCursor(cursor{
			Pinned:    boolInt(last.Pinned),
			CreatedAt: last.CreatedAt.UnixMilli(),
			ID:        last.ID,
		})
	}
	return result, nil
}

func whereWithoutCursor(where []string, hadCursor bool) []string {
	if hadCursor && len(where) > 0 {
		return where[:len(where)-1]
	}
	return where
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (s *Store) UpdateMessage(ctx context.Context, id, actor string, in model.UpdateInput) (model.Message, error) {
	if in.ExpectedVersion <= 0 {
		return model.Message{}, ErrPrecondition
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	current, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Message{}, ErrNotFound
	}
	if err != nil {
		return model.Message{}, err
	}
	if current.DeletedAt != nil {
		return model.Message{}, ErrNotFound
	}
	if current.ArchivedAt != nil {
		return model.Message{}, ErrArchived
	}
	if in.ExpectedVersion != current.Version {
		return model.Message{}, &ConflictError{Current: &current}
	}

	title, content := current.Title, current.Content
	tags := current.Tags
	priority := current.Priority
	pinned := current.Pinned
	if in.Title != nil {
		title = strings.TrimSpace(*in.Title)
	}
	if in.Content != nil {
		content = strings.TrimSpace(*in.Content)
	}
	if in.Tags != nil {
		tags = normalizeTags(*in.Tags)
	}
	if in.Priority != nil {
		priority = validatePriority(*in.Priority)
	}
	if in.Pinned != nil {
		pinned = *in.Pinned
	}
	if content == "" {
		return model.Message{}, errors.New("content must not be empty")
	}
	if len([]rune(content)) > 32000 {
		return model.Message{}, errors.New("content exceeds 32000 characters")
	}
	if len([]rune(title)) > 240 {
		return model.Message{}, errors.New("title exceeds 240 characters")
	}
	now := nowMS()
	encodedTags, _ := json.Marshal(tags)
	res, err := tx.ExecContext(ctx, `UPDATE messages SET title=?, content=?, tags=?, priority=?, pinned=?,
		edited_by=?, version=version+1, updated_at=? WHERE id=? AND version=?`,
		title, content, string(encodedTags), priority, boolInt(pinned), actor, now, id, in.ExpectedVersion)
	if err != nil {
		return model.Message{}, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		latest, getErr := s.GetMessage(ctx, id)
		if getErr != nil {
			return model.Message{}, getErr
		}
		return model.Message{}, &ConflictError{Current: &latest}
	}
	newVersion := current.Version + 1
	if _, err := tx.ExecContext(ctx, `INSERT INTO message_revisions(message_id,version,title,content,editor,created_at)
		VALUES (?,?,?,?,?,?)`, id, newVersion, title, content, actor, now); err != nil {
		return model.Message{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(type,message_id,title,actor,detail,created_at)
		VALUES ('message.updated',?,?,?,?,?)`, id, displayTitle(title, content), actor, "updated to v"+strconv.Itoa(newVersion), now); err != nil {
		return model.Message{}, err
	}
	row = tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	updated, err := scanMessage(row)
	if err != nil {
		return model.Message{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, err
	}
	return updated, nil
}

func (s *Store) DeleteMessage(ctx context.Context, id, actor string) error {
	now := nowMS()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && m.DeletedAt != nil) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, "UPDATE messages SET deleted_at=?, updated_at=? WHERE id=? AND deleted_at IS NULL", now, now, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,message_id,title,actor,detail,created_at)
		VALUES ('message.deleted',?,?,?,?,?)`, id, displayTitle(m.Title, m.Content), actor, "deleted message", now)
	_, _ = tx.ExecContext(ctx, "DELETE FROM edit_locks WHERE message_id = ?", id)
	return tx.Commit()
}

func (s *Store) RestoreMessage(ctx context.Context, id, actor string) (model.Message, error) {
	now := nowMS()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && m.DeletedAt != nil) {
		return model.Message{}, ErrNotFound
	}
	if err != nil {
		return model.Message{}, err
	}
	if m.ArchivedAt == nil {
		return m, nil
	}
	_, err = tx.ExecContext(ctx, "UPDATE messages SET archived_at=NULL, archived_reason='', updated_at=? WHERE id=?", now, id)
	if err != nil {
		return model.Message{}, err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,message_id,title,actor,detail,created_at)
		VALUES ('message.restored',?,?,?,?,?)`, id, displayTitle(m.Title, m.Content), actor, "restored from archive", now)
	row = tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	restored, err := scanMessage(row)
	if err != nil {
		return model.Message{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, err
	}
	return restored, nil
}

func (s *Store) ArchiveMessage(ctx context.Context, id, actor, reason string) (model.Message, error) {
	now := nowMS()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Message{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ? AND deleted_at IS NULL", id)
	m, err := scanMessage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Message{}, ErrNotFound
	}
	if err != nil {
		return model.Message{}, err
	}
	if m.ArchivedAt != nil {
		return m, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual"
	}
	_, err = tx.ExecContext(ctx, "UPDATE messages SET archived_at=?, archived_reason=?, updated_at=? WHERE id=?", now, reason, now, id)
	if err != nil {
		return model.Message{}, err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,message_id,title,actor,detail,created_at)
		VALUES ('message.archived',?,?,?,?,?)`, id, displayTitle(m.Title, m.Content), actor, reason, now)
	_, _ = tx.ExecContext(ctx, "DELETE FROM edit_locks WHERE message_id = ?", id)
	row = tx.QueryRowContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE id = ?", id)
	archived, err := scanMessage(row)
	if err != nil {
		return model.Message{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Message{}, err
	}
	return archived, nil
}
