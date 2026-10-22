package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"sort"
	"time"

	"agent-board/internal/model"
)

func (s *Store) Stats(ctx context.Context, configured map[string]string) (model.Stats, error) {
	var st model.Stats
	st.GeneratedAt = time.Now().UTC()
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayMS := dayStart.UnixMilli()
	dayAgo := now.Add(-24 * time.Hour).UnixMilli()

	queries := []struct {
		query string
		args  []any
		dest  *int
	}{
		{"SELECT COUNT(*) FROM messages WHERE deleted_at IS NULL AND archived_at IS NULL", nil, &st.Active},
		{"SELECT COUNT(*) FROM messages WHERE deleted_at IS NULL AND archived_at IS NOT NULL", nil, &st.Archived},
		{"SELECT COUNT(*) FROM messages WHERE deleted_at IS NOT NULL", nil, &st.Deleted},
		{"SELECT COUNT(DISTINCT author) FROM messages WHERE deleted_at IS NULL", nil, &st.Agents},
		{"SELECT COUNT(*) FROM messages WHERE deleted_at IS NULL AND archived_at IS NULL AND pinned=1", nil, &st.Pinned},
		{"SELECT COUNT(*) FROM messages WHERE deleted_at IS NULL AND created_at >= ?", []any{todayMS}, &st.CreatedToday},
		{"SELECT COUNT(*) FROM events WHERE type='message.updated' AND created_at >= ?", []any{dayAgo}, &st.Edits24h},
		{"SELECT COUNT(*) FROM messages WHERE deleted_at IS NULL", nil, &st.TotalAllTime},
		{"SELECT COUNT(*) FROM edit_locks WHERE expires_at > ?", []any{now.UnixMilli()}, &st.Locked},
	}
	for _, q := range queries {
		if err := s.db.QueryRowContext(ctx, q.query, q.args...).Scan(q.dest); err != nil {
			return st, err
		}
	}

	if policy, err := s.GetArchivePolicy(ctx); err == nil {
		st.Policy = policy
		st.ActiveCapacity = policy.MaxItems
		if policy.MaxItems > 0 {
			st.CapacityPercent = float64(st.Active) / float64(policy.MaxItems) * 100
			if st.CapacityPercent > 100 {
				st.CapacityPercent = 100
			}
		}
	}
	if last, err := s.LastArchiveRun(ctx); err == nil {
		st.LastArchiveRun = last
	}

	const days = 7
	st.Trend = make([]model.DayPoint, 0, days)
	for i := days - 1; i >= 0; i-- {
		start := dayStart.AddDate(0, 0, -i)
		end := start.AddDate(0, 0, 1)
		point := model.DayPoint{Date: start.Format("2006-01-02")}
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE created_at >= ? AND created_at < ?`,
			start.UnixMilli(), end.UnixMilli()).Scan(&point.Created)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE type='message.updated' AND created_at >= ? AND created_at < ?`,
			start.UnixMilli(), end.UnixMilli()).Scan(&point.Edited)
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE type='message.archived' AND created_at >= ? AND created_at < ?`,
			start.UnixMilli(), end.UnixMilli()).Scan(&point.Archived)
		st.Trend = append(st.Trend, point)
	}

	top, err := s.Agents(ctx, configured)
	if err != nil {
		return st, err
	}
	if len(top) > 6 {
		top = top[:6]
	}
	st.TopAgents = top

	events, err := s.ListEvents(ctx, 10)
	if err != nil {
		return st, err
	}
	st.RecentEvents = events
	if info, err := os.Stat(s.dbPath); err == nil {
		st.StorageBytes = info.Size()
	}
	return st, nil
}

func (s *Store) Agents(ctx context.Context, configured map[string]string) ([]model.AgentSummary, error) {
	type agentAccumulator struct {
		summary model.AgentSummary
		latest  int64
	}
	accumulators := map[string]*agentAccumulator{}
	get := func(name string) *agentAccumulator {
		if name == "" {
			name = "unknown"
		}
		if a, ok := accumulators[name]; ok {
			return a
		}
		role := configured[name]
		a := &agentAccumulator{summary: model.AgentSummary{
			Name:       name,
			Color:      agentColor(name),
			Role:       role,
			Configured: role != "",
		}}
		accumulators[name] = a
		return a
	}

	rows, err := s.db.QueryContext(ctx, `SELECT author, COUNT(*),
		SUM(CASE WHEN archived_at IS NULL AND deleted_at IS NULL THEN 1 ELSE 0 END),
		MAX(updated_at) FROM messages WHERE deleted_at IS NULL GROUP BY author`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			name   string
			count  int
			active int
			latest sql.NullInt64
		)
		if err := rows.Scan(&name, &count, &active, &latest); err != nil {
			rows.Close()
			return nil, err
		}
		a := get(name)
		a.summary.Messages = count
		a.summary.Active = active
		if latest.Valid {
			a.latest = latest.Int64
		}
	}
	rows.Close()

	rows, err = s.db.QueryContext(ctx, `SELECT editor, COUNT(*), MAX(created_at) FROM message_revisions
		WHERE version > 1 GROUP BY editor`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var (
			name   string
			count  int
			latest sql.NullInt64
		)
		if err := rows.Scan(&name, &count, &latest); err != nil {
			rows.Close()
			return nil, err
		}
		a := get(name)
		a.summary.Edits = count
		if latest.Valid && latest.Int64 > a.latest {
			a.latest = latest.Int64
		}
	}
	rows.Close()

	// Include configured tokens that have not posted yet.
	for name, role := range configured {
		a := get(name)
		if a.summary.Role == "" {
			a.summary.Role = role
		}
		a.summary.Configured = true
	}

	out := make([]model.AgentSummary, 0, len(accumulators))
	for _, a := range accumulators {
		if a.latest > 0 {
			a.summary.LastActive = msTime(a.latest)
		}
		out = append(out, a.summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Messages != out[j].Messages {
			return out[i].Messages > out[j].Messages
		}
		if out[i].Edits != out[j].Edits {
			return out[i].Edits > out[j].Edits
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func agentColor(name string) string {
	palette := []string{"#2dd4bf", "#60a5fa", "#a78bfa", "#f472b6", "#fbbf24", "#34d399", "#fb7185", "#38bdf8", "#c084fc", "#f59e0b"}
	var n int
	for _, r := range name {
		n = n*31 + int(r)
	}
	if n < 0 {
		n = -n
	}
	return palette[n%len(palette)]
}

func (s *Store) ListEvents(ctx context.Context, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,type,message_id,title,actor,detail,metadata,created_at
		FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Event, 0, limit)
	for rows.Next() {
		var (
			e         model.Event
			meta      string
			createdAt int64
		)
		if err := rows.Scan(&e.ID, &e.Type, &e.MessageID, &e.Title, &e.Actor, &e.Detail, &meta, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt = msTime(createdAt)
		if json.Valid([]byte(meta)) {
			e.Metadata = json.RawMessage(meta)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) Revisions(ctx context.Context, messageID string, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := s.db.QueryContext(ctx, `SELECT version,title,content,editor,created_at
		FROM message_revisions WHERE message_id=? ORDER BY version DESC LIMIT ?`, messageID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Message, 0, limit)
	for rows.Next() {
		var (
			version   int
			title     string
			content   string
			editor    string
			createdAt int64
		)
		if err := rows.Scan(&version, &title, &content, &editor, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, model.Message{
			ID:        messageID,
			Title:     title,
			Content:   content,
			Author:    editor,
			EditedBy:  editor,
			Version:   version,
			CreatedAt: msTime(createdAt),
			UpdatedAt: msTime(createdAt),
		})
	}
	return out, rows.Err()
}
