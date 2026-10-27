package config

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	RoleAdmin  = "admin"
	RoleWriter = "writer"
)

// Principal is an authenticated API/MCP caller. Tokens are intentionally kept
// out of this struct so they can never be serialized by accident.
type Principal struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Color string `json:"color"`
}

type tokenEntry struct {
	Principal
	hash [32]byte
}

func PrincipalFor(name, role string) Principal {
	return Principal{Name: name, Role: role, Color: colorFor(name)}
}

type Config struct {
	Addr            string
	DBPath          string
	PublicURL       string
	CORSOrigins     []string
	Tokens          []tokenEntry
	SeedDemo        bool
	RetentionDays   int
	MaxItems        int
	AutoArchive     bool
	ArchiveInterval time.Duration
	MCPInstructions string
	AppEnv          string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            env("AGENT_BOARD_ADDR", ":8080"),
		DBPath:          env("AGENT_BOARD_DB", "data/agent-board.db"),
		PublicURL:       strings.TrimRight(env("AGENT_BOARD_PUBLIC_URL", ""), "/"),
		CORSOrigins:     splitCSV(env("AGENT_BOARD_CORS_ORIGINS", "")),
		SeedDemo:        envBool("AGENT_BOARD_SEED_DEMO", false),
		RetentionDays:   envInt("AGENT_BOARD_RETENTION_DAYS", 7),
		MaxItems:        envInt("AGENT_BOARD_MAX_ITEMS", 200),
		AutoArchive:     envBool("AGENT_BOARD_AUTO_ARCHIVE", true),
		ArchiveInterval: time.Duration(envInt("AGENT_BOARD_ARCHIVE_INTERVAL_MINUTES", 5)) * time.Minute,
		AppEnv:          env("APP_ENV", "development"),
		ShutdownTimeout: 10 * time.Second,
	}
	cfg.MCPInstructions = "AgentBoard is a shared message board for multiple AI agents. " +
		"Use board_list_messages to read recent notes (each item is a summary: title, excerpt, metadata — call board_get_message with the id for the full body), " +
		"board_create_message to leave a note, " +
		"and board_update_message with the current expected_version to edit safely. " +
		"Messages are automatically archived by age or count; board_list_messages with status=archived reaches the archive."

	if cfg.RetentionDays < 1 || cfg.MaxItems < 1 {
		return cfg, errors.New("retention days and max items must be positive")
	}
	if cfg.ArchiveInterval < 30*time.Second {
		cfg.ArchiveInterval = 30 * time.Second
	}

	tokens, err := parseTokens(env("AGENT_BOARD_TOKENS", ""))
	if err != nil {
		return cfg, err
	}
	if len(tokens) == 0 {
		if cfg.AppEnv == "production" {
			return cfg, errors.New("AGENT_BOARD_TOKENS is required when APP_ENV=production")
		}
		// Development-only credentials. They are loud in logs and documented in README.
		tokens = append(tokens,
			newToken("admin", RoleAdmin, "dev-admin-token"),
			newToken("agent-alpha", RoleWriter, "dev-agent-token"),
		)
	}
	cfg.Tokens = tokens

	return cfg, nil
}

func (c Config) Authenticate(raw string) (Principal, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	if raw == "" {
		return Principal{}, false
	}
	sum := sha256.Sum256([]byte(raw))
	for _, entry := range c.Tokens {
		if subtle.ConstantTimeCompare(sum[:], entry.hash[:]) == 1 {
			return entry.Principal, true
		}
	}
	return Principal{}, false
}

func (c Config) IsProduction() bool { return c.AppEnv == "production" }

func parseTokens(raw string) ([]tokenEntry, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []tokenEntry
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid AGENT_BOARD_TOKENS entry %q: expected name:role:token", item)
		}
		name := strings.TrimSpace(parts[0])
		role := strings.ToLower(strings.TrimSpace(parts[1]))
		secret := strings.TrimSpace(parts[2])
		if name == "" || secret == "" {
			return nil, errors.New("token name and value must not be empty")
		}
		if role != RoleAdmin && role != RoleWriter {
			return nil, fmt.Errorf("invalid role %q for %s (want admin or writer)", role, name)
		}
		out = append(out, newToken(name, role, secret))
	}
	return out, nil
}

func newToken(name, role, secret string) tokenEntry {
	return tokenEntry{
		Principal: Principal{Name: name, Role: role, Color: colorFor(name)},
		hash:      sha256.Sum256([]byte(secret)),
	}
}

var palette = []string{"#2dd4bf", "#60a5fa", "#a78bfa", "#f472b6", "#fbbf24", "#34d399", "#fb7185", "#38bdf8", "#c084fc", "#f59e0b"}

func colorFor(name string) string {
	var n int
	for _, r := range name {
		n = n*31 + int(r)
	}
	if n < 0 {
		n = -n
	}
	return palette[n%len(palette)]
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return b
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
