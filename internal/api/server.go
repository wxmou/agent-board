package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"agent-board/internal/config"
	"agent-board/internal/hub"
	"agent-board/internal/model"
	"agent-board/internal/store"
)

type ctxKey string

const principalKey ctxKey = "principal"

type Server struct {
	cfg       config.Config
	store     *store.Store
	hub       *hub.Hub
	version   string
	startedAt time.Time
	kick      chan struct{}
	mcp       http.Handler
}

func New(cfg config.Config, st *store.Store, h *hub.Hub, version string, kick chan struct{}) *Server {
	if kick == nil {
		kick = make(chan struct{}, 1)
	}
	return &Server{
		cfg:       cfg,
		store:     st,
		hub:       h,
		version:   version,
		startedAt: time.Now(),
		kick:      kick,
	}
}

func (s *Server) SetMCPHandler(handler http.Handler) { s.mcp = handler }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.Handle("GET /api/v1/auth/me", s.auth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/v1/messages", s.auth(http.HandlerFunc(s.handleListMessages)))
	mux.Handle("POST /api/v1/messages", s.auth(http.HandlerFunc(s.handleCreateMessage)))
	mux.Handle("GET /api/v1/messages/{id}", s.auth(http.HandlerFunc(s.handleGetMessage)))
	mux.Handle("PATCH /api/v1/messages/{id}", s.auth(http.HandlerFunc(s.handleUpdateMessage)))
	mux.Handle("DELETE /api/v1/messages/{id}", s.auth(http.HandlerFunc(s.handleDeleteMessage)))
	mux.Handle("GET /api/v1/messages/{id}/revisions", s.auth(http.HandlerFunc(s.handleRevisions)))
	mux.Handle("POST /api/v1/messages/{id}/lock", s.auth(http.HandlerFunc(s.handleAcquireLock)))
	mux.Handle("DELETE /api/v1/messages/{id}/lock", s.auth(http.HandlerFunc(s.handleReleaseLock)))
	mux.Handle("GET /api/v1/messages/{id}/lock", s.auth(http.HandlerFunc(s.handleGetLock)))
	mux.Handle("POST /api/v1/messages/{id}/archive", s.auth(s.adminOnly(http.HandlerFunc(s.handleArchiveMessage))))
	mux.Handle("POST /api/v1/messages/{id}/restore", s.auth(s.adminOnly(http.HandlerFunc(s.handleRestoreMessage))))
	mux.Handle("GET /api/v1/stats", s.auth(http.HandlerFunc(s.handleStats)))
	mux.Handle("GET /api/v1/agents", s.auth(http.HandlerFunc(s.handleAgents)))
	mux.Handle("GET /api/v1/events", s.auth(http.HandlerFunc(s.handleEvents)))
	mux.Handle("GET /api/v1/stream", s.auth(http.HandlerFunc(s.handleStream)))
	mux.Handle("GET /api/v1/locks", s.auth(http.HandlerFunc(s.handleListLocks)))
	mux.Handle("GET /api/v1/settings/archive", s.auth(http.HandlerFunc(s.handleGetPolicy)))
	mux.Handle("PATCH /api/v1/settings/archive", s.auth(s.adminOnly(http.HandlerFunc(s.handleSetPolicy))))
	mux.Handle("POST /api/v1/archive/run", s.auth(s.adminOnly(http.HandlerFunc(s.handleRunArchive))))
	mux.Handle("GET /api/v1/archive/runs", s.auth(http.HandlerFunc(s.handleArchiveRuns)))
	mux.Handle("GET /api/v1/export", s.auth(s.adminOnly(http.HandlerFunc(s.handleExport))))
	if s.mcp != nil {
		mux.Handle("/mcp", s.mcp)
		mux.Handle("/mcp/", s.mcp)
	}
	mux.Handle("/", staticHandler())
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" || strings.HasPrefix(r.URL.Path, "/mcp/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			for _, allowed := range s.cfg.CORSOrigins {
				if allowed == "*" || strings.EqualFold(allowed, origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key, MCP-Protocol-Version, Mcp-Session-Id, Last-Event-ID")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, ETag")
					w.Header().Set("Access-Control-Max-Age", "600")
					break
				}
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path != "/api/health" && r.URL.Path != "/api/v1/health" && s.cfg.IsProduction() {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'self'; frame-ancestors 'self'")
		}
		rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && time.Since(start) > 2*time.Second {
			slog.Warn("slow request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start), "status", rec.status)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.Header.Get("X-API-Key")
		}
		principal, ok := s.cfg.Authenticate(token)
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="agent-board"`)
			writeError(w, http.StatusUnauthorized, "unauthorized", "缺少或无效的 API Token")
			return
		}
		ctx := context.WithValue(r.Context(), principalKey, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principal(r)
		if p.Role != config.RoleAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "该操作需要管理员权限")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func principal(r *http.Request) config.Principal {
	p, _ := r.Context().Value(principalKey).(config.Principal)
	return p
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.store.DB().PingContext(r.Context()); err != nil {
		dbStatus = "error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"version":        s.version,
		"time":           time.Now().UTC(),
		"uptime_seconds": int64(time.Since(s.startedAt).Seconds()),
		"database":       dbStatus,
		"realtime":       s.hub.Count(),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"principal":    p,
		"server":       map[string]any{"version": s.version, "mcp_url": s.cfg.PublicURL + "/mcp", "uptime_seconds": int64(time.Since(s.startedAt).Seconds())},
		"capabilities": map[string]any{"archive": true, "realtime": true, "mcp": true},
	})
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := model.ListOptions{
		Status:         q.Get("status"),
		Query:          q.Get("q"),
		Author:         q.Get("author"),
		Tag:            q.Get("tag"),
		Priority:       q.Get("priority"),
		ArchivedReason: q.Get("reason"),
		Sort:           q.Get("sort"),
		Cursor:         q.Get("cursor"),
	}
	opts.Limit = intQuery(q.Get("limit"), 20)
	opts.Offset = intQuery(q.Get("offset"), 0)
	if raw := q.Get("pinned"); raw != "" {
		v := raw == "true" || raw == "1"
		opts.Pinned = &v
	}
	result, err := s.store.ListMessages(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	out := model.ListResult{
		Items:      result.Items,
		Total:      result.Total,
		Limit:      result.Limit,
		Offset:     result.Offset,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	var in model.CreateInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	p := principal(r)
	msg, created, err := s.store.CreateMessage(r.Context(), p.Name, in)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_message", err.Error())
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
		s.hub.Publish("message.created", msg)
		s.KickArchive()
	}
	writeJSON(w, status, msg)
}

func (s *Server) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	msg, err := s.store.GetMessage(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "留言不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if msg.DeletedAt != nil {
		writeError(w, http.StatusNotFound, "not_found", "留言已删除")
		return
	}
	w.Header().Set("ETag", fmt.Sprintf(`"v%d"`, msg.Version))
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleUpdateMessage(w http.ResponseWriter, r *http.Request) {
	var in model.UpdateInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if in.ExpectedVersion == 0 {
		in.ExpectedVersion = intQuery(r.Header.Get("If-Match"), 0)
	}
	if in.ExpectedVersion == 0 && r.Header.Get("If-Match") != "" {
		if _, err := fmt.Sscanf(strings.Trim(r.Header.Get("If-Match"), `"`), "v%d", &in.ExpectedVersion); err != nil {
			writeError(w, http.StatusPreconditionRequired, "precondition_required", "缺少 expected_version 或有效的 If-Match")
			return
		}
	}
	if in.ExpectedVersion <= 0 {
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "更新留言必须提供当前 expected_version")
		return
	}
	p := principal(r)
	msg, err := s.store.UpdateMessage(r.Context(), r.PathValue("id"), p.Name, in)
	var conflict *store.ConflictError
	switch {
	case errors.As(err, &conflict):
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":   map[string]any{"code": "version_conflict", "message": "留言已被其他 Agent 更新，请基于最新版本重试", "current": conflict.Current},
			"current": conflict.Current,
		})
		return
	case errors.Is(err, store.ErrArchived):
		writeError(w, http.StatusConflict, "archived", "归档留言不可编辑，请先恢复")
		return
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "留言不存在")
		return
	case errors.Is(err, store.ErrPrecondition):
		writeError(w, http.StatusPreconditionRequired, "precondition_required", "缺少 expected_version")
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, "invalid_message", err.Error())
		return
	}
	s.hub.Publish("message.updated", msg)
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	msg, err := s.store.GetMessage(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) || (err == nil && msg.DeletedAt != nil) {
		writeError(w, http.StatusNotFound, "not_found", "留言不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if p.Role != config.RoleAdmin && msg.Author != p.Name {
		writeError(w, http.StatusForbidden, "forbidden", "只有作者或管理员可以删除留言")
		return
	}
	if err := s.store.DeleteMessage(r.Context(), msg.ID, p.Name); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.hub.Publish("message.deleted", map[string]any{"id": msg.ID, "actor": p.Name})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": msg.ID})
}

func (s *Server) handleRevisions(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Revisions(r.Context(), r.PathValue("id"), intQuery(r.URL.Query().Get("limit"), 30))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleAcquireLock(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TTLSeconds int `json:"ttl_seconds"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}
	ttl := time.Duration(body.TTLSeconds) * time.Second
	p := principal(r)
	lock, err := s.store.AcquireLock(r.Context(), r.PathValue("id"), p.Name, ttl)
	var locked *store.LockedError
	if errors.As(err, &locked) {
		writeJSON(w, http.StatusLocked, map[string]any{
			"error": map[string]any{"code": "locked", "message": locked.Lock.Owner + " 正在编辑这条留言", "lock": locked.Lock},
			"lock":  locked.Lock,
		})
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "留言不存在或已归档")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.hub.Publish("lock.acquired", map[string]any{"message_id": lock.MessageID, "owner": lock.Owner, "expires_at": lock.ExpiresAt})
	writeJSON(w, http.StatusOK, lock)
}

func (s *Server) handleReleaseLock(w http.ResponseWriter, r *http.Request) {
	p := principal(r)
	var body struct {
		Token string `json:"token"`
	}
	if r.ContentLength > 0 {
		_ = decodeJSON(r, &body)
	}
	if body.Token == "" {
		body.Token = r.URL.Query().Get("token")
	}
	err := s.store.ReleaseLock(r.Context(), r.PathValue("id"), p.Name, body.Token, p.Role == config.RoleAdmin)
	if errors.Is(err, store.ErrUnauthorized) {
		writeError(w, http.StatusForbidden, "forbidden", "无法释放其他 Agent 的编辑锁")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.hub.Publish("lock.released", map[string]any{"message_id": r.PathValue("id"), "owner": p.Name})
	writeJSON(w, http.StatusOK, map[string]any{"released": true})
}

func (s *Server) handleGetLock(w http.ResponseWriter, r *http.Request) {
	lock, err := s.store.GetLock(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusOK, map[string]any{"lock": nil})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	lock.Token = ""
	writeJSON(w, http.StatusOK, map[string]any{"lock": lock})
}

func (s *Server) handleListLocks(w http.ResponseWriter, r *http.Request) {
	locks, err := s.store.ListLocks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": locks})
}

func (s *Server) handleArchiveMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	_ = decodeJSON(r, &body)
	msg, err := s.store.ArchiveMessage(r.Context(), r.PathValue("id"), principal(r).Name, body.Reason)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "留言不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.hub.Publish("message.archived", msg)
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleRestoreMessage(w http.ResponseWriter, r *http.Request) {
	msg, err := s.store.RestoreMessage(r.Context(), r.PathValue("id"), principal(r).Name)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "留言不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.hub.Publish("message.restored", msg)
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	configured := make(map[string]string, len(s.cfg.Tokens))
	for _, t := range s.cfg.Tokens {
		configured[t.Name] = t.Role
	}
	stats, err := s.store.Stats(r.Context(), configured)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	stats.ServerVersion = s.version
	stats.UptimeSeconds = int64(time.Since(s.startedAt).Seconds())
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	configured := make(map[string]string, len(s.cfg.Tokens))
	for _, t := range s.cfg.Tokens {
		configured[t.Name] = t.Role
	}
	agents, err := s.store.Agents(r.Context(), configured)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": agents, "total": len(agents)})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.ListEvents(r.Context(), intQuery(r.URL.Query().Get("limit"), 30))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) handleGetPolicy(w http.ResponseWriter, r *http.Request) {
	policy, err := s.store.GetArchivePolicy(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (s *Server) handleSetPolicy(w http.ResponseWriter, r *http.Request) {
	var policy model.ArchivePolicy
	if err := decodeJSON(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	updated, err := s.store.SetArchivePolicy(r.Context(), policy)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_policy", err.Error())
		return
	}
	s.hub.Publish("policy.updated", updated)
	s.KickArchive()
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRunArchive(w http.ResponseWriter, r *http.Request) {
	outcome, err := s.store.RunArchive(r.Context(), "manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "archive_failed", err.Error())
		return
	}
	for _, msg := range outcome.Archived {
		s.hub.Publish("message.archived", msg)
	}
	s.hub.Publish("archive.run", outcome.Result)
	writeJSON(w, http.StatusOK, outcome.Result)
}

func (s *Server) handleArchiveRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ListArchiveRuns(r.Context(), intQuery(r.URL.Query().Get("limit"), 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListMessages(r.Context(), model.ListOptions{Status: "all", Limit: 200, Sort: "oldest"})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="agent-board-export.json"`)
	writeJSON(w, http.StatusOK, map[string]any{"exported_at": time.Now().UTC(), "messages": items.Items})
}

// KickArchive coalesces archive checks triggered by new writes.
func (s *Server) KickArchive() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

func (s *Server) RunArchive(ctx context.Context) (model.ArchiveResult, error) {
	outcome, err := s.store.RunArchive(ctx, "scheduled")
	if err != nil {
		return model.ArchiveResult{}, err
	}
	for _, msg := range outcome.Archived {
		s.hub.Publish("message.archived", msg)
	}
	s.hub.Publish("archive.run", outcome.Result)
	return outcome.Result, nil
}

func (s *Server) ArchiveLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var lastRun time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.kick:
			// Debounce bursts of writes before checking capacity.
			select {
			case <-time.After(800 * time.Millisecond):
			case <-ctx.Done():
				return
			}
			policy, err := s.store.GetArchivePolicy(ctx)
			if err == nil && policy.AutoArchive {
				if _, err := s.RunArchive(ctx); err != nil {
					slog.Error("archive run failed", "error", err)
				} else {
					lastRun = time.Now()
				}
			}
		case <-ticker.C:
			policy, err := s.store.GetArchivePolicy(ctx)
			if err != nil || !policy.AutoArchive {
				continue
			}
			if time.Since(lastRun) < time.Duration(policy.IntervalMinutes)*time.Minute {
				continue
			}
			if _, err := s.RunArchive(ctx); err != nil {
				slog.Error("scheduled archive failed", "error", err)
			} else {
				lastRun = time.Now()
			}
		}
	}
}

func decodeJSON(r *http.Request, dest any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
	if err := dec.Decode(dest); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message}})
}

func intQuery(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil {
		return fallback
	}
	return n
}
