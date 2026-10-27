package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"agent-board/internal/config"
	"agent-board/internal/hub"
	"agent-board/internal/model"
	"agent-board/internal/store"
)

const latestProtocol = "2025-06-18"

type Server struct {
	cfg     config.Config
	store   *store.Store
	hub     *hub.Hub
	notify  func()
	version string
}

func New(cfg config.Config, st *store.Store, h *hub.Hub, notify func(), version string) *Server {
	if notify == nil {
		notify = func() {}
	}
	return &Server{cfg: cfg, store: st, hub: h, notify: notify, version: version}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, DELETE")
		writeRPC(w, http.StatusMethodNotAllowed, response{JSONRPC: "2.0", Error: &rpcError{Code: -32600, Message: "use POST for Streamable HTTP MCP"}})
		return
	}
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.Header.Get("X-API-Key")
	}
	principal, ok := s.authenticate(r.Context(), token)
	if !ok {
		w.Header().Set("WWW-Authenticate", `Bearer realm="agent-board-mcp"`)
		writeRPC(w, http.StatusUnauthorized, response{JSONRPC: "2.0", Error: &rpcError{Code: -32001, Message: "unauthorized"}})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		writeRPC(w, http.StatusBadRequest, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "cannot read body"}})
		return
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		writeRPC(w, http.StatusBadRequest, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "empty body"}})
		return
	}
	if strings.HasPrefix(trimmed, "[") {
		var batch []request
		if err := json.Unmarshal(body, &batch); err != nil {
			writeRPC(w, http.StatusBadRequest, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			return
		}
		responses := make([]response, 0, len(batch))
		for _, req := range batch {
			if resp, hasResponse := s.dispatch(r.Context(), principal, req); hasResponse {
				responses = append(responses, resp)
			}
		}
		if len(responses) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeRPC(w, http.StatusOK, responses)
		return
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		writeRPC(w, http.StatusBadRequest, response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
		return
	}
	resp, hasResponse := s.dispatch(r.Context(), principal, req)
	if !hasResponse {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if req.Method == "initialize" {
		w.Header().Set("Mcp-Session-Id", newSessionID())
	}
	writeRPC(w, http.StatusOK, resp)
}

func (s *Server) authenticate(ctx context.Context, raw string) (config.Principal, bool) {
	if principal, ok := s.cfg.Authenticate(raw); ok {
		return principal, true
	}
	raw = strings.TrimSpace(raw)
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	if !strings.HasPrefix(raw, "ab_mcp_") {
		return config.Principal{}, false
	}
	key, ok, err := s.store.AuthenticateMCPKey(ctx, raw)
	if err != nil || !ok {
		return config.Principal{}, false
	}
	return config.PrincipalFor(key.Name, key.Role), true
}

func (s *Server) dispatch(ctx context.Context, principal config.Principal, req request) (response, bool) {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: -32600, Message: "invalid jsonrpc version"}}, true
	}
	notification := len(req.ID) == 0 || string(req.ID) == "null"
	resp := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &params)
		version := latestProtocol
		switch params.ProtocolVersion {
		case "2024-11-05", "2025-03-26", "2025-06-18":
			version = params.ProtocolVersion
		}
		resp.Result = map[string]any{
			"protocolVersion": version,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo":   map[string]any{"name": "agent-board", "version": s.version},
			"instructions": s.cfg.MCPInstructions,
		}
	case "notifications/initialized", "notifications/cancelled":
		return response{}, false
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		result, err := s.callTool(ctx, principal, req.Params)
		if err != nil {
			resp.Result = toolError(err)
		} else {
			resp.Result = result
		}
	default:
		if notification {
			return response{}, false
		}
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	if notification {
		return response{}, false
	}
	return resp, true
}

func (s *Server) callTool(ctx context.Context, principal config.Principal, raw json.RawMessage) (map[string]any, error) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, fmt.Errorf("invalid tool parameters: %w", err)
	}
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	switch params.Name {
	case "board_list_messages":
		opts := model.ListOptions{
			Status:         strArg(params.Arguments, "status", "active"),
			Query:          strArg(params.Arguments, "query", ""),
			Author:         strArg(params.Arguments, "author", ""),
			Tag:            strArg(params.Arguments, "tag", ""),
			Priority:       strArg(params.Arguments, "priority", ""),
			ArchivedReason: strArg(params.Arguments, "archive_reason", ""),
			Sort:           strArg(params.Arguments, "sort", "newest"),
			Cursor:         strArg(params.Arguments, "cursor", ""),
			Limit:          intArg(params.Arguments, "limit", 20),
			Offset:         intArg(params.Arguments, "offset", 0),
		}
		result, err := s.store.ListMessages(ctx, opts)
		if err != nil {
			return nil, err
		}
		policy, _ := s.store.GetArchivePolicy(ctx)
		var items any = model.Summarize(result.Items)
		if includeContent, ok := optionalBool(params.Arguments, "include_content"); ok && includeContent {
			items = result.Items
		}
		return toolOK(map[string]any{
			"items":       items,
			"total":       result.Total,
			"limit":       result.Limit,
			"offset":      result.Offset,
			"has_more":    result.HasMore,
			"next_cursor": result.NextCursor,
			"policy":      policy,
		})
	case "board_get_message":
		msg, err := s.store.GetMessage(ctx, strArg(params.Arguments, "id", ""))
		if errors.Is(err, store.ErrNotFound) {
			return nil, errors.New("message not found")
		}
		if err != nil {
			return nil, err
		}
		if msg.DeletedAt != nil {
			return nil, errors.New("message was deleted")
		}
		return toolOK(msg)
	case "board_create_message":
		in := model.CreateInput{
			Title:           strArg(params.Arguments, "title", ""),
			Content:         strArg(params.Arguments, "content", ""),
			Tags:            stringsArg(params.Arguments, "tags"),
			Priority:        strArg(params.Arguments, "priority", "normal"),
			ClientRequestID: strArg(params.Arguments, "client_request_id", ""),
		}
		msg, created, err := s.store.CreateMessage(ctx, principal.Name, in)
		if err != nil {
			return nil, err
		}
		if created {
			s.hub.Publish("message.created", msg)
			s.notify()
		}
		out := map[string]any{"message": msg, "created": created, "message_id": msg.ID}
		return toolOK(out)
	case "board_update_message":
		in := model.UpdateInput{
			ExpectedVersion: intArg(params.Arguments, "expected_version", 0),
		}
		if v, ok := optionalString(params.Arguments, "title"); ok {
			in.Title = &v
		}
		if v, ok := optionalString(params.Arguments, "content"); ok {
			in.Content = &v
		}
		if v, ok := optionalString(params.Arguments, "priority"); ok {
			in.Priority = &v
		}
		if v, ok := optionalBool(params.Arguments, "pinned"); ok {
			in.Pinned = &v
		}
		if v, ok := optionalStrings(params.Arguments, "tags"); ok {
			in.Tags = &v
		}
		if in.ExpectedVersion <= 0 {
			return nil, errors.New("expected_version is required; read the message first and pass its version")
		}
		msg, err := s.store.UpdateMessage(ctx, strArg(params.Arguments, "id", ""), principal.Name, in)
		var conflict *store.ConflictError
		if errors.As(err, &conflict) {
			return nil, fmt.Errorf("version_conflict: message is now at v%d; fetch it again and reapply your change", conflict.Current.Version)
		}
		if errors.Is(err, store.ErrArchived) {
			return nil, errors.New("message is archived and read-only")
		}
		if err != nil {
			return nil, err
		}
		s.hub.Publish("message.updated", msg)
		return toolOK(map[string]any{"message": msg, "message_id": msg.ID, "version": msg.Version})
	case "board_delete_message":
		id := strArg(params.Arguments, "id", "")
		msg, err := s.store.GetMessage(ctx, id)
		if err != nil {
			return nil, err
		}
		if principal.Role != config.RoleAdmin && msg.Author != principal.Name {
			return nil, errors.New("only the author or an admin can delete this message")
		}
		if err := s.store.DeleteMessage(ctx, id, principal.Name); err != nil {
			return nil, err
		}
		s.hub.Publish("message.deleted", map[string]any{"id": id, "actor": principal.Name})
		return toolOK(map[string]any{"deleted": true, "id": id})
	case "board_archive_message":
		if principal.Role != config.RoleAdmin {
			return nil, errors.New("admin role required")
		}
		msg, err := s.store.ArchiveMessage(ctx, strArg(params.Arguments, "id", ""), principal.Name, strArg(params.Arguments, "reason", "manual"))
		if err != nil {
			return nil, err
		}
		s.hub.Publish("message.archived", msg)
		return toolOK(map[string]any{"message": msg, "archived": true})
	case "board_restore_message":
		if principal.Role != config.RoleAdmin {
			return nil, errors.New("admin role required")
		}
		msg, err := s.store.RestoreMessage(ctx, strArg(params.Arguments, "id", ""), principal.Name)
		if err != nil {
			return nil, err
		}
		s.hub.Publish("message.restored", msg)
		return toolOK(map[string]any{"message": msg, "restored": true})
	case "board_list_agents":
		agents, err := s.store.Agents(ctx, s.configuredAgents())
		if err != nil {
			return nil, err
		}
		return toolOK(map[string]any{"items": agents, "total": len(agents)})
	case "board_get_stats":
		stats, err := s.store.Stats(ctx, s.configuredAgents())
		if err != nil {
			return nil, err
		}
		stats.ServerVersion = s.version
		return toolOK(stats)
	case "board_get_archive_policy":
		policy, err := s.store.GetArchivePolicy(ctx)
		if err != nil {
			return nil, err
		}
		return toolOK(policy)
	case "board_set_archive_policy":
		if principal.Role != config.RoleAdmin {
			return nil, errors.New("admin role required")
		}
		current, err := s.store.GetArchivePolicy(ctx)
		if err != nil {
			return nil, err
		}
		if v, ok := optionalInt(params.Arguments, "retain_days"); ok {
			current.RetainDays = v
		}
		if v, ok := optionalInt(params.Arguments, "max_items"); ok {
			current.MaxItems = v
		}
		if v, ok := optionalInt(params.Arguments, "interval_minutes"); ok {
			current.IntervalMinutes = v
		}
		if v, ok := optionalBool(params.Arguments, "auto_archive"); ok {
			current.AutoArchive = v
		}
		if v, ok := optionalBool(params.Arguments, "pinned_protected"); ok {
			current.PinnedProtected = v
		}
		updated, err := s.store.SetArchivePolicy(ctx, current)
		if err != nil {
			return nil, err
		}
		s.hub.Publish("policy.updated", updated)
		s.notify()
		return toolOK(updated)
	case "board_run_archive":
		if principal.Role != config.RoleAdmin {
			return nil, errors.New("admin role required")
		}
		outcome, err := s.store.RunArchive(ctx, "manual")
		if err != nil {
			return nil, err
		}
		for _, msg := range outcome.Archived {
			s.hub.Publish("message.archived", msg)
		}
		s.hub.Publish("archive.run", outcome.Result)
		return toolOK(outcome.Result)
	case "board_list_archive_runs":
		runs, err := s.store.ListArchiveRuns(ctx, intArg(params.Arguments, "limit", 20))
		if err != nil {
			return nil, err
		}
		return toolOK(map[string]any{"items": runs, "total": len(runs)})
	default:
		return nil, fmt.Errorf("unknown tool: %s", params.Name)
	}
}

func (s *Server) configuredAgents() map[string]string {
	out := make(map[string]string, len(s.cfg.Tokens))
	for _, t := range s.cfg.Tokens {
		out[t.Name] = t.Role
	}
	return out
}

func toolOK(value any) (map[string]any, error) {
	text, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": string(text)}},
		"structuredContent": value,
		"isError":           false,
	}, nil
}

func toolError(err error) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": err.Error()}},
		"isError": true,
	}
}

func writeRPC(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(value)
}

func newSessionID() string {
	return fmt.Sprintf("ab_%d", time.Now().UnixNano())
}

func strArg(args map[string]any, key, fallback string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return fallback
}

func optionalString(args map[string]any, key string) (string, bool) {
	v, ok := args[key].(string)
	return v, ok
}

func intArg(args map[string]any, key string, fallback int) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return fallback
}

func optionalInt(args map[string]any, key string) (int, bool) {
	if _, ok := args[key]; !ok {
		return 0, false
	}
	return intArg(args, key, 0), true
}

func optionalBool(args map[string]any, key string) (bool, bool) {
	v, ok := args[key].(bool)
	return v, ok
}

func stringsArg(args map[string]any, key string) []string {
	values, ok := optionalStrings(args, key)
	if !ok {
		return nil
	}
	return values
}

func optionalStrings(args map[string]any, key string) ([]string, bool) {
	raw, ok := args[key]
	if !ok {
		return nil, false
	}
	switch values := raw.(type) {
	case []any:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	case []string:
		return values, true
	case string:
		if strings.TrimSpace(values) == "" {
			return []string{}, true
		}
		parts := strings.Split(values, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out, true
	default:
		return nil, false
	}
}
