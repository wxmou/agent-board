package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"agent-board/internal/model"
)

// AcquireLock creates or extends an advisory edit lease. Locks are visible in
// the UI but do not replace optimistic concurrency: every write still requires
// the current expected_version, so an expired or bypassed lock cannot overwrite
// another agent's work silently.
func (s *Store) AcquireLock(ctx context.Context, messageID, owner string, ttl time.Duration) (model.Lock, error) {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	if ttl > 10*time.Minute {
		ttl = 10 * time.Minute
	}
	var archived sql.NullInt64
	var deleted sql.NullInt64
	err := s.db.QueryRowContext(ctx, "SELECT archived_at, deleted_at FROM messages WHERE id=?", messageID).Scan(&archived, &deleted)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (archived.Valid || deleted.Valid)) {
		return model.Lock{}, ErrNotFound
	}
	if err != nil {
		return model.Lock{}, err
	}
	now := time.Now().UTC()
	nowMSValue := now.UnixMilli()
	expires := now.Add(ttl)
	token := newID()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Lock{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO edit_locks(message_id,owner,token,expires_at,created_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(message_id) DO UPDATE SET owner=excluded.owner, token=excluded.token,
		expires_at=excluded.expires_at, created_at=excluded.created_at
		WHERE edit_locks.expires_at < ? OR edit_locks.owner = excluded.owner`,
		messageID, owner, token, expires.UnixMilli(), nowMSValue, nowMSValue)
	if err != nil {
		return model.Lock{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		current, err := getLockTx(ctx, tx, messageID)
		if err != nil {
			return model.Lock{}, err
		}
		return model.Lock{}, &LockedError{Lock: &current}
	}
	lock := model.Lock{
		MessageID: messageID,
		Owner:     owner,
		Token:     token,
		ExpiresAt: expires,
		CreatedAt: now,
	}
	if err := tx.Commit(); err != nil {
		return model.Lock{}, err
	}
	return lock, nil
}

func getLockTx(ctx context.Context, tx *sql.Tx, messageID string) (model.Lock, error) {
	var (
		lock      model.Lock
		expiresAt int64
		createdAt int64
	)
	err := tx.QueryRowContext(ctx, `SELECT message_id,owner,token,expires_at,created_at
		FROM edit_locks WHERE message_id=?`, messageID).
		Scan(&lock.MessageID, &lock.Owner, &lock.Token, &expiresAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return lock, ErrNotFound
	}
	if err != nil {
		return lock, err
	}
	lock.ExpiresAt = msTime(expiresAt)
	lock.CreatedAt = msTime(createdAt)
	return lock, nil
}

func (s *Store) ReleaseLock(ctx context.Context, messageID, owner, token string, admin bool) error {
	lock, err := s.GetLock(ctx, messageID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !admin && lock.Owner != owner {
		return ErrUnauthorized
	}
	if !admin && token != "" && lock.Token != token {
		return ErrUnauthorized
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM edit_locks WHERE message_id=?", messageID)
	return err
}

func (s *Store) GetLock(ctx context.Context, messageID string) (model.Lock, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Lock{}, err
	}
	defer tx.Rollback()
	lock, err := getLockTx(ctx, tx, messageID)
	if err != nil {
		return lock, err
	}
	if lock.ExpiresAt.Before(time.Now().UTC()) {
		_, _ = tx.ExecContext(ctx, "DELETE FROM edit_locks WHERE message_id=?", messageID)
		_ = tx.Commit()
		return model.Lock{}, ErrNotFound
	}
	return lock, nil
}

func (s *Store) ListLocks(ctx context.Context) ([]model.Lock, error) {
	now := nowMS()
	_, _ = s.db.ExecContext(ctx, "DELETE FROM edit_locks WHERE expires_at <= ?", now)
	rows, err := s.db.QueryContext(ctx, `SELECT message_id,owner,token,expires_at,created_at
		FROM edit_locks WHERE expires_at > ? ORDER BY created_at DESC`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Lock, 0, 8)
	for rows.Next() {
		var (
			lock      model.Lock
			expiresAt int64
			createdAt int64
		)
		if err := rows.Scan(&lock.MessageID, &lock.Owner, &lock.Token, &expiresAt, &createdAt); err != nil {
			return nil, err
		}
		lock.Token = "" // never expose another editor's lease token
		lock.ExpiresAt = msTime(expiresAt)
		lock.CreatedAt = msTime(createdAt)
		out = append(out, lock)
	}
	return out, rows.Err()
}

func (s *Store) CleanupLocks(ctx context.Context) {
	_, _ = s.db.ExecContext(ctx, "DELETE FROM edit_locks WHERE expires_at <= ?", nowMS())
}
