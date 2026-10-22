package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"agent-board/internal/model"
)

var (
	ErrKeyNameTaken = errors.New("mcp key name already exists")
	ErrKeyExpired   = errors.New("mcp key expired")
)

const keyPrefix = "ab_mcp_"

func (s *Store) CreateMCPKey(ctx context.Context, in model.CreateMCPKeyInput, createdBy string) (model.MCPKeyCreated, error) {
	name, role, days, err := validateKeyInput(in)
	if err != nil {
		return model.MCPKeyCreated{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.MCPKeyCreated{}, err
	}
	defer tx.Rollback()

	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM mcp_keys WHERE name=? AND revoked_at IS NULL", name).Scan(&exists)
	if err == nil {
		return model.MCPKeyCreated{}, ErrKeyNameTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.MCPKeyCreated{}, err
	}
	key, record, err := buildMCPKey(name, role, createdBy, days)
	if err != nil {
		return model.MCPKeyCreated{}, err
	}
	if err := insertMCPKey(ctx, tx, key, record); err != nil {
		return model.MCPKeyCreated{}, err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,title,actor,detail,created_at)
		VALUES ('mcp_key.created',?,?,?,?)`, name, createdBy, "created MCP key for "+name, nowMS())
	if err := tx.Commit(); err != nil {
		return model.MCPKeyCreated{}, err
	}
	return model.MCPKeyCreated{Key: key, Record: record}, nil
}

func (s *Store) RotateMCPKey(ctx context.Context, id, actor string, expiresDays int) (model.MCPKeyCreated, error) {
	if expiresDays < 0 || expiresDays > 3650 {
		return model.MCPKeyCreated{}, errors.New("expires_days must be between 0 and 3650")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.MCPKeyCreated{}, err
	}
	defer tx.Rollback()
	current, err := getMCPKeyTx(ctx, tx, id)
	if err != nil {
		return model.MCPKeyCreated{}, err
	}
	if current.RevokedAt != nil {
		return model.MCPKeyCreated{}, errors.New("cannot rotate a revoked key")
	}
	now := nowMS()
	if _, err := tx.ExecContext(ctx, "UPDATE mcp_keys SET revoked_at=?, revoked_by=? WHERE id=? AND revoked_at IS NULL", now, actor, id); err != nil {
		return model.MCPKeyCreated{}, err
	}
	key, record, err := buildMCPKey(current.Name, current.Role, actor, expiresDays)
	if err != nil {
		return model.MCPKeyCreated{}, err
	}
	if err := insertMCPKey(ctx, tx, key, record); err != nil {
		return model.MCPKeyCreated{}, err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,title,actor,detail,created_at)
		VALUES ('mcp_key.rotated',?,?,?,?)`, current.Name, actor, "rotated MCP key for "+current.Name, now)
	if err := tx.Commit(); err != nil {
		return model.MCPKeyCreated{}, err
	}
	return model.MCPKeyCreated{Key: key, Record: record}, nil
}

func (s *Store) ListMCPKeys(ctx context.Context) ([]model.MCPKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,role,prefix,created_by,created_at,last_used_at,expires_at,revoked_at,revoked_by
		FROM mcp_keys ORDER BY CASE WHEN revoked_at IS NULL THEN 0 ELSE 1 END, created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.MCPKey, 0, 16)
	for rows.Next() {
		key, err := scanMCPKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, key)
	}
	return out, rows.Err()
}

func (s *Store) RevokeMCPKey(ctx context.Context, id, actor string) (model.MCPKey, error) {
	now := nowMS()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.MCPKey{}, err
	}
	defer tx.Rollback()
	current, err := getMCPKeyTx(ctx, tx, id)
	if err != nil {
		return model.MCPKey{}, err
	}
	if current.RevokedAt == nil {
		if _, err := tx.ExecContext(ctx, "UPDATE mcp_keys SET revoked_at=?, revoked_by=? WHERE id=? AND revoked_at IS NULL", now, actor, id); err != nil {
			return model.MCPKey{}, err
		}
		_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,title,actor,detail,created_at)
			VALUES ('mcp_key.revoked',?,?,?,?)`, current.Name, actor, "revoked MCP key for "+current.Name, now)
	}
	updated, err := getMCPKeyTx(ctx, tx, id)
	if err != nil {
		return model.MCPKey{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.MCPKey{}, err
	}
	return updated, nil
}

// AuthenticateMCPKey validates an MCP-only key and records usage at most once
// per minute. Only the SHA-256 hash is stored in SQLite.
func (s *Store) AuthenticateMCPKey(ctx context.Context, raw string) (model.MCPKey, bool, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, keyPrefix) || len(raw) < len(keyPrefix)+20 || len(raw) > 200 {
		return model.MCPKey{}, false, nil
	}
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	row := s.db.QueryRowContext(ctx, `SELECT id,name,role,prefix,created_by,created_at,last_used_at,expires_at,revoked_at,revoked_by
		FROM mcp_keys WHERE key_hash=? AND revoked_at IS NULL`, hash)
	key, err := scanMCPKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.MCPKey{}, false, nil
	}
	if err != nil {
		return model.MCPKey{}, false, err
	}
	now := time.Now().UTC()
	if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
		return key, false, nil
	}
	threshold := now.Add(-time.Minute).UnixMilli()
	_, _ = s.db.ExecContext(ctx, "UPDATE mcp_keys SET last_used_at=? WHERE id=? AND (last_used_at IS NULL OR last_used_at < ?)",
		now.UnixMilli(), key.ID, threshold)
	return key, true, nil
}

func validateKeyInput(in model.CreateMCPKeyInput) (string, string, int, error) {
	name := strings.TrimSpace(in.Name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 64 {
		return "", "", 0, errors.New("name must be 2-64 characters")
	}
	if strings.ContainsAny(name, "\r\n\t") {
		return "", "", 0, errors.New("name contains invalid characters")
	}
	role := strings.ToLower(strings.TrimSpace(in.Role))
	if role == "" {
		role = "writer"
	}
	if role != "admin" && role != "writer" {
		return "", "", 0, errors.New("role must be admin or writer")
	}
	if in.ExpiresDays < 0 || in.ExpiresDays > 3650 {
		return "", "", 0, errors.New("expires_days must be between 0 and 3650")
	}
	return name, role, in.ExpiresDays, nil
}

func buildMCPKey(name, role, createdBy string, days int) (string, model.MCPKey, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", model.MCPKey{}, fmt.Errorf("generate key: %w", err)
	}
	raw := keyPrefix + base64.RawURLEncoding.EncodeToString(random)
	now := time.Now().UTC()
	prefix := raw[:14]
	var expires *time.Time
	if days > 0 {
		t := now.Add(time.Duration(days) * 24 * time.Hour)
		expires = &t
	}
	record := model.MCPKey{
		ID:        newID(),
		Name:      name,
		Role:      role,
		Prefix:    prefix,
		CreatedBy: createdBy,
		CreatedAt: now,
		ExpiresAt: expires,
		Status:    "active",
	}
	return raw, record, nil
}

func insertMCPKey(ctx context.Context, tx *sql.Tx, raw string, record model.MCPKey) error {
	sum := sha256.Sum256([]byte(raw))
	var expires any
	if record.ExpiresAt != nil {
		expires = record.ExpiresAt.UnixMilli()
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO mcp_keys
		(id,name,role,prefix,key_hash,created_by,created_at,expires_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		record.ID, record.Name, record.Role, record.Prefix, hex.EncodeToString(sum[:]), record.CreatedBy,
		record.CreatedAt.UnixMilli(), expires)
	return err
}

func getMCPKeyTx(ctx context.Context, tx *sql.Tx, id string) (model.MCPKey, error) {
	row := tx.QueryRowContext(ctx, `SELECT id,name,role,prefix,created_by,created_at,last_used_at,expires_at,revoked_at,revoked_by
		FROM mcp_keys WHERE id=?`, id)
	key, err := scanMCPKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return key, ErrNotFound
	}
	return key, err
}

type keyScanner interface {
	Scan(dest ...any) error
}

func scanMCPKey(row keyScanner) (model.MCPKey, error) {
	var (
		key       model.MCPKey
		created   int64
		lastUsed  sql.NullInt64
		expires   sql.NullInt64
		revoked   sql.NullInt64
		revokedBy string
	)
	if err := row.Scan(&key.ID, &key.Name, &key.Role, &key.Prefix, &key.CreatedBy, &created,
		&lastUsed, &expires, &revoked, &revokedBy); err != nil {
		return key, err
	}
	key.CreatedAt = msTime(created)
	key.LastUsedAt = nullTime(lastUsed)
	key.ExpiresAt = nullTime(expires)
	key.RevokedAt = nullTime(revoked)
	key.RevokedBy = revokedBy
	key.Status = mcpKeyStatus(key)
	return key, nil
}

func mcpKeyStatus(key model.MCPKey) string {
	if key.RevokedAt != nil {
		return "revoked"
	}
	if key.ExpiresAt != nil && !key.ExpiresAt.After(time.Now().UTC()) {
		return "expired"
	}
	return "active"
}
