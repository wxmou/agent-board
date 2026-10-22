package model

import (
	"encoding/json"
	"time"
)

type Message struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Content         string     `json:"content"`
	Author          string     `json:"author"`
	EditedBy        string     `json:"edited_by,omitempty"`
	Tags            []string   `json:"tags"`
	Priority        string     `json:"priority"`
	Pinned          bool       `json:"pinned"`
	Version         int        `json:"version"`
	ClientRequestID string     `json:"client_request_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ArchivedAt      *time.Time `json:"archived_at,omitempty"`
	ArchivedReason  string     `json:"archived_reason,omitempty"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

type Lock struct {
	MessageID string    `json:"message_id"`
	Owner     string    `json:"owner"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	ID        int64           `json:"id"`
	Type      string          `json:"type"`
	MessageID string          `json:"message_id,omitempty"`
	Title     string          `json:"title,omitempty"`
	Actor     string          `json:"actor"`
	Detail    string          `json:"detail,omitempty"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type ArchivePolicy struct {
	RetainDays      int  `json:"retain_days"`
	MaxItems        int  `json:"max_items"`
	AutoArchive     bool `json:"auto_archive"`
	IntervalMinutes int  `json:"interval_minutes"`
	PinnedProtected bool `json:"pinned_protected"`
}

type ArchiveRun struct {
	ID             int64      `json:"id"`
	Trigger        string     `json:"trigger"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	ArchivedCount  int        `json:"archived_count"`
	RetentionCount int        `json:"retention_count"`
	OverflowCount  int        `json:"overflow_count"`
	Remaining      int        `json:"remaining"`
	Error          string     `json:"error,omitempty"`
}

type ArchiveResult struct {
	ArchivedCount  int    `json:"archived_count"`
	RetentionCount int    `json:"retention_count"`
	OverflowCount  int    `json:"overflow_count"`
	Remaining      int    `json:"remaining"`
	Trigger        string `json:"trigger"`
	DurationMS     int64  `json:"duration_ms"`
}

type AgentSummary struct {
	Name       string    `json:"name"`
	Color      string    `json:"color"`
	Role       string    `json:"role,omitempty"`
	Messages   int       `json:"messages"`
	Active     int       `json:"active"`
	Edits      int       `json:"edits"`
	LastActive time.Time `json:"last_active,omitempty"`
	Configured bool      `json:"configured"`
}

type DayPoint struct {
	Date     string `json:"date"`
	Created  int    `json:"created"`
	Edited   int    `json:"edited"`
	Archived int    `json:"archived"`
}

type Stats struct {
	Active          int            `json:"active"`
	Archived        int            `json:"archived"`
	Deleted         int            `json:"deleted"`
	Agents          int            `json:"agents"`
	Pinned          int            `json:"pinned"`
	CreatedToday    int            `json:"created_today"`
	Edits24h        int            `json:"edits_24h"`
	TotalAllTime    int            `json:"total_all_time"`
	Locked          int            `json:"locked"`
	StorageBytes    int64          `json:"storage_bytes"`
	Trend           []DayPoint     `json:"trend"`
	TopAgents       []AgentSummary `json:"top_agents"`
	RecentEvents    []Event        `json:"recent_events"`
	Policy          ArchivePolicy  `json:"policy"`
	LastArchiveRun  *ArchiveRun    `json:"last_archive_run,omitempty"`
	GeneratedAt     time.Time      `json:"generated_at"`
	UptimeSeconds   int64          `json:"uptime_seconds"`
	ServerVersion   string         `json:"server_version"`
	DBPath          string         `json:"-"`
	ActiveCapacity  int            `json:"active_capacity"`
	CapacityPercent float64        `json:"capacity_percent"`
}

type ListResult struct {
	Items      []Message `json:"items"`
	Total      int       `json:"total"`
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
	NextCursor string    `json:"next_cursor,omitempty"`
	HasMore    bool      `json:"has_more"`
}

type ListOptions struct {
	Status         string
	Query          string
	Author         string
	Tag            string
	Priority       string
	ArchivedReason string
	Pinned         *bool
	Sort           string
	Limit          int
	Offset         int
	Cursor         string
}

type CreateInput struct {
	Title           string   `json:"title"`
	Content         string   `json:"content"`
	Tags            []string `json:"tags"`
	Priority        string   `json:"priority"`
	ClientRequestID string   `json:"client_request_id"`
}

type UpdateInput struct {
	Title           *string   `json:"title"`
	Content         *string   `json:"content"`
	Tags            *[]string `json:"tags"`
	Priority        *string   `json:"priority"`
	Pinned          *bool     `json:"pinned"`
	ExpectedVersion int       `json:"expected_version"`
	Force           bool      `json:"force"`
}

type MCPKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Role       string     `json:"role"`
	Prefix     string     `json:"prefix"`
	CreatedBy  string     `json:"created_by"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	RevokedBy  string     `json:"revoked_by,omitempty"`
	Status     string     `json:"status"`
}

type CreateMCPKeyInput struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	ExpiresDays int    `json:"expires_days"`
}

type MCPKeyCreated struct {
	Key    string `json:"key"`
	Record MCPKey `json:"record"`
}
