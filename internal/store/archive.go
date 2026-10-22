package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"agent-board/internal/model"
)

type ArchiveOutcome struct {
	Result   model.ArchiveResult
	Archived []model.Message
}

func (s *Store) GetArchivePolicy(ctx context.Context) (model.ArchivePolicy, error) {
	values, err := s.getSettings(ctx, "archive.retain_days", "archive.max_items", "archive.auto", "archive.interval_minutes", "archive.pinned_protected")
	if err != nil {
		return model.ArchivePolicy{}, err
	}
	policy := model.ArchivePolicy{
		RetainDays:      atoiDefault(values["archive.retain_days"], 7),
		MaxItems:        atoiDefault(values["archive.max_items"], 200),
		AutoArchive:     values["archive.auto"] != "false",
		IntervalMinutes: atoiDefault(values["archive.interval_minutes"], 5),
		PinnedProtected: values["archive.pinned_protected"] == "true",
	}
	if policy.RetainDays < 1 {
		policy.RetainDays = 1
	}
	if policy.MaxItems < 1 {
		policy.MaxItems = 1
	}
	if policy.IntervalMinutes < 1 {
		policy.IntervalMinutes = 1
	}
	return policy, nil
}

func (s *Store) SetArchivePolicy(ctx context.Context, policy model.ArchivePolicy) (model.ArchivePolicy, error) {
	if policy.RetainDays < 1 || policy.RetainDays > 3650 {
		return policy, errors.New("retain_days must be between 1 and 3650")
	}
	if policy.MaxItems < 1 || policy.MaxItems > 100000 {
		return policy, errors.New("max_items must be between 1 and 100000")
	}
	if policy.IntervalMinutes < 1 || policy.IntervalMinutes > 1440 {
		return policy, errors.New("interval_minutes must be between 1 and 1440")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return policy, err
	}
	defer tx.Rollback()
	values := map[string]string{
		"archive.retain_days":      strconv.Itoa(policy.RetainDays),
		"archive.max_items":        strconv.Itoa(policy.MaxItems),
		"archive.auto":             strconv.FormatBool(policy.AutoArchive),
		"archive.interval_minutes": strconv.Itoa(policy.IntervalMinutes),
		"archive.pinned_protected": strconv.FormatBool(policy.PinnedProtected),
	}
	for k, v := range values {
		if _, err := tx.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES (?,?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, k, v); err != nil {
			return policy, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(type,actor,detail,created_at)
		VALUES ('policy.updated','system',?,?)`,
		fmt.Sprintf("retention=%dd max=%d auto=%t", policy.RetainDays, policy.MaxItems, policy.AutoArchive), nowMS()); err != nil {
		return policy, err
	}
	if err := tx.Commit(); err != nil {
		return policy, err
	}
	return s.GetArchivePolicy(ctx)
}

func (s *Store) getSettings(ctx context.Context, keys ...string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))
	for i, key := range keys {
		placeholders[i] = "?"
		args[i] = key
	}
	rows, err := s.db.QueryContext(ctx, "SELECT key,value FROM settings WHERE key IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func atoiDefault(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n == 0 {
		return fallback
	}
	return n
}

type archiveCandidate struct {
	model.Message
}

// RunArchive applies the retention policy and the active-item cap in one
// transaction. Oldest first, optionally protecting pinned notes. It returns
// every archived message so the realtime hub can notify connected clients.
func (s *Store) RunArchive(ctx context.Context, trigger string) (ArchiveOutcome, error) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ArchiveOutcome{}, err
	}
	defer tx.Rollback()
	policy, err := getPolicyTx(ctx, tx)
	if err != nil {
		return ArchiveOutcome{}, err
	}
	now := nowMS()
	runRes, err := tx.ExecContext(ctx, `INSERT INTO archive_runs(trigger_type,started_at) VALUES (?,?)`, trigger, now)
	if err != nil {
		return ArchiveOutcome{}, err
	}
	runID, _ := runRes.LastInsertId()

	rows, err := tx.QueryContext(ctx, "SELECT "+messageColumns+" FROM messages WHERE deleted_at IS NULL AND archived_at IS NULL ORDER BY created_at ASC, id ASC")
	if err != nil {
		return ArchiveOutcome{}, err
	}
	active := make([]model.Message, 0, 64)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			rows.Close()
			return ArchiveOutcome{}, err
		}
		active = append(active, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ArchiveOutcome{}, err
	}
	rows.Close()

	cutoff := time.Now().UTC().Add(-time.Duration(policy.RetainDays) * 24 * time.Hour).UnixMilli()
	toArchive := make(map[string]string)
	for _, m := range active {
		if policy.PinnedProtected && m.Pinned {
			continue
		}
		if m.CreatedAt.UnixMilli() < cutoff {
			toArchive[m.ID] = "retention_days"
		}
	}
	remaining := len(active) - len(toArchive)
	for _, m := range active {
		if remaining <= policy.MaxItems {
			break
		}
		if _, already := toArchive[m.ID]; already {
			continue
		}
		if policy.PinnedProtected && m.Pinned {
			continue
		}
		toArchive[m.ID] = "max_items"
		remaining--
	}

	archived := make([]model.Message, 0, len(toArchive))
	retentionCount, overflowCount := 0, 0
	for _, m := range active {
		reason, ok := toArchive[m.ID]
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, "UPDATE messages SET archived_at=?, archived_reason=?, updated_at=? WHERE id=?",
			now, reason, now, m.ID); err != nil {
			return ArchiveOutcome{}, err
		}
		_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,message_id,title,actor,detail,created_at)
			VALUES ('message.archived',?,?,?,?,?)`, m.ID, displayTitle(m.Title, m.Content), "system", reason, now)
		_, _ = tx.ExecContext(ctx, "DELETE FROM edit_locks WHERE message_id = ?", m.ID)
		m.ArchivedAt = ptrTime(msTime(now))
		m.ArchivedReason = reason
		m.UpdatedAt = msTime(now)
		archived = append(archived, m)
		if reason == "retention_days" {
			retentionCount++
		} else {
			overflowCount++
		}
	}
	finished := nowMS()
	_, err = tx.ExecContext(ctx, `UPDATE archive_runs SET finished_at=?, archived_count=?, retention_count=?,
		overflow_count=?, remaining=? WHERE id=?`, finished, len(archived), retentionCount, overflowCount, remaining, runID)
	if err != nil {
		return ArchiveOutcome{}, err
	}
	_, _ = tx.ExecContext(ctx, `INSERT INTO events(type,actor,detail,created_at)
		VALUES ('archive.run','system',?,?)`,
		fmt.Sprintf("archived %d messages", len(archived)), finished)
	_, _ = tx.ExecContext(ctx, `DELETE FROM archive_runs WHERE id NOT IN (
		SELECT id FROM archive_runs ORDER BY id DESC LIMIT 200
	)`)
	if err := tx.Commit(); err != nil {
		return ArchiveOutcome{}, err
	}
	result := model.ArchiveResult{
		ArchivedCount:  len(archived),
		RetentionCount: retentionCount,
		OverflowCount:  overflowCount,
		Remaining:      remaining,
		Trigger:        trigger,
		DurationMS:     time.Since(started).Milliseconds(),
	}
	return ArchiveOutcome{Result: result, Archived: archived}, nil
}

func getPolicyTx(ctx context.Context, tx *sql.Tx) (model.ArchivePolicy, error) {
	keys := []string{"archive.retain_days", "archive.max_items", "archive.auto", "archive.interval_minutes", "archive.pinned_protected"}
	values := map[string]string{}
	for _, key := range keys {
		var value string
		err := tx.QueryRowContext(ctx, "SELECT value FROM settings WHERE key=?", key).Scan(&value)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return model.ArchivePolicy{}, err
		}
		values[key] = value
	}
	policy := model.ArchivePolicy{
		RetainDays:      atoiDefault(values["archive.retain_days"], 7),
		MaxItems:        atoiDefault(values["archive.max_items"], 200),
		AutoArchive:     values["archive.auto"] != "false",
		IntervalMinutes: atoiDefault(values["archive.interval_minutes"], 5),
		PinnedProtected: values["archive.pinned_protected"] == "true",
	}
	return policy, nil
}

func (s *Store) ListArchiveRuns(ctx context.Context, limit int) ([]model.ArchiveRun, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,trigger_type,started_at,finished_at,archived_count,
		retention_count,overflow_count,remaining,error FROM archive_runs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.ArchiveRun, 0, limit)
	for rows.Next() {
		var (
			r        model.ArchiveRun
			started  int64
			finished sql.NullInt64
		)
		if err := rows.Scan(&r.ID, &r.Trigger, &started, &finished, &r.ArchivedCount,
			&r.RetentionCount, &r.OverflowCount, &r.Remaining, &r.Error); err != nil {
			return nil, err
		}
		r.StartedAt = msTime(started)
		r.FinishedAt = nullTime(finished)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) LastArchiveRun(ctx context.Context) (*model.ArchiveRun, error) {
	runs, err := s.ListArchiveRuns(ctx, 1)
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return &runs[0], nil
}

func ptrTime(t time.Time) *time.Time { return &t }
