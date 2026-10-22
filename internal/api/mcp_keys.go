package api

import (
	"errors"
	"net/http"
	"strings"

	"agent-board/internal/model"
	"agent-board/internal/store"
)

func (s *Server) handleListMCPKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.store.ListMCPKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	active := 0
	for _, key := range keys {
		if key.Status == "active" {
			active++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": keys, "total": len(keys), "active": active})
}

func (s *Server) handleCreateMCPKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Role        string `json:"role"`
		ExpiresDays *int   `json:"expires_days"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	days := 90
	if body.ExpiresDays != nil {
		days = *body.ExpiresDays
	}
	created, err := s.store.CreateMCPKey(r.Context(), model.CreateMCPKeyInput{
		Name:        body.Name,
		Role:        body.Role,
		ExpiresDays: days,
	}, principal(r).Name)
	if errors.Is(err, store.ErrKeyNameTaken) {
		writeError(w, http.StatusConflict, "key_name_taken", "已存在同名且未撤销的 MCP Key")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_key", err.Error())
		return
	}
	s.hub.Publish("mcp_key.created", map[string]any{"id": created.Record.ID, "name": created.Record.Name, "role": created.Record.Role})
	writeJSON(w, http.StatusCreated, map[string]any{
		"key":     created.Key,
		"record":  created.Record,
		"mcp_url": s.mcpURL(r),
		"warning": "This key is shown only once. Store it in the agent's secret manager.",
	})
}

func (s *Server) handleRevokeMCPKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.store.RevokeMCPKey(r.Context(), r.PathValue("id"), principal(r).Name)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "MCP Key 不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	s.hub.Publish("mcp_key.revoked", map[string]any{"id": key.ID, "name": key.Name})
	writeJSON(w, http.StatusOK, key)
}

func (s *Server) handleRotateMCPKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExpiresDays *int `json:"expires_days"`
	}
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}
	days := 90
	if body.ExpiresDays != nil {
		days = *body.ExpiresDays
	}
	created, err := s.store.RotateMCPKey(r.Context(), r.PathValue("id"), principal(r).Name, days)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "MCP Key 不存在")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "rotate_failed", err.Error())
		return
	}
	s.hub.Publish("mcp_key.rotated", map[string]any{"id": created.Record.ID, "name": created.Record.Name})
	writeJSON(w, http.StatusCreated, map[string]any{
		"key":     created.Key,
		"record":  created.Record,
		"mcp_url": s.mcpURL(r),
		"warning": "The previous key was revoked. This replacement is shown only once.",
	})
}

func (s *Server) mcpURL(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL + "/mcp"
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/mcp"
}
