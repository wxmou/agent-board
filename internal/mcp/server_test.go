package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"agent-board/internal/config"
	"agent-board/internal/hub"
	"agent-board/internal/model"
	"agent-board/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "board.db"), model.ArchivePolicy{
		RetainDays:      7,
		MaxItems:        200,
		AutoArchive:     true,
		IntervalMinutes: 5,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(config.Config{}, st, hub.New(), nil, "test")
}

func seedMessage(t *testing.T, srv *Server, title, content string) model.Message {
	t.Helper()
	msg, _, err := srv.store.CreateMessage(context.Background(), "agent-alpha", model.CreateInput{
		Title: title, Content: content, Tags: []string{"api"},
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	return msg
}

func callList(t *testing.T, srv *Server, arguments map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"name": "board_list_messages", "arguments": arguments})
	if err != nil {
		t.Fatal(err)
	}
	result, err := srv.callTool(context.Background(), config.PrincipalFor("agent-alpha", config.RoleWriter), raw)
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	return result
}

// payload round-trips a tool result through JSON so assertions see the wire
// shape rather than the typed Go values.
func payload(t *testing.T, result map[string]any) (map[string]any, string) {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded, string(raw)
}

func structured(t *testing.T, result map[string]any) map[string]any {
	t.Helper()
	decoded, _ := payload(t, result)
	value, ok := decoded["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("structuredContent type = %T", decoded["structuredContent"])
	}
	return value
}

func items(t *testing.T, result map[string]any) []map[string]any {
	t.Helper()
	raw, ok := structured(t, result)["items"].([]any)
	if !ok {
		t.Fatalf("items type = %T", structured(t, result)["items"])
	}
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		item, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("item type = %T", entry)
		}
		out = append(out, item)
	}
	return out
}

func TestListReturnsSummariesWithoutBodies(t *testing.T) {
	srv := testServer(t)
	body := strings.Repeat("详情", 900) // 1800 runes, far beyond an excerpt
	seedMessage(t, srv, "接口契约已冻结", body)

	result := callList(t, srv, map[string]any{"limit": 10})
	_, raw := payload(t, result)
	if strings.Contains(raw, body) {
		t.Fatal("list response leaked the full message body")
	}
	// Only the excerpt may appear inside structuredContent.
	sc, err := json.Marshal(structured(t, result))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(sc), "详情"); count > model.SummaryExcerptRunes/2 {
		t.Fatalf("body fragments beyond the excerpt: %d occurrences", count)
	}

	list := items(t, result)
	if len(list) != 1 {
		t.Fatalf("items = %d, want 1", len(list))
	}
	item := list[0]
	if _, exists := item["content"]; exists {
		t.Fatal("summary item must not carry a content field")
	}
	if item["title"] != "接口契约已冻结" {
		t.Fatalf("title = %v", item["title"])
	}
	excerpt, ok := item["excerpt"].(string)
	if !ok || excerpt == "" {
		t.Fatalf("excerpt = %v", item["excerpt"])
	}
	if got := len([]rune(excerpt)); got != model.SummaryExcerptRunes+1 {
		t.Fatalf("excerpt runes = %d, want %d", got, model.SummaryExcerptRunes+1)
	}
	if item["content_chars"].(float64) != 1800 {
		t.Fatalf("content_chars = %v, want 1800", item["content_chars"])
	}
	if item["author"] != "agent-alpha" || item["priority"] != "normal" {
		t.Fatalf("metadata missing: %+v", item)
	}
}

func TestListIncludeContentReturnsBodies(t *testing.T) {
	srv := testServer(t)
	body := strings.Repeat("详情", 900)
	msg := seedMessage(t, srv, "接口契约已冻结", body)

	result := callList(t, srv, map[string]any{"limit": 10, "include_content": true})
	list := items(t, result)
	if len(list) != 1 {
		t.Fatalf("items = %d, want 1", len(list))
	}
	if list[0]["content"] != body {
		t.Fatal("include_content=true must return the full body")
	}
	if list[0]["id"] != msg.ID {
		t.Fatalf("id = %v, want %s", list[0]["id"], msg.ID)
	}
}

func TestGetMessageStillReturnsFullBody(t *testing.T) {
	srv := testServer(t)
	body := strings.Repeat("详情", 50)
	msg := seedMessage(t, srv, "", body)

	raw, err := json.Marshal(map[string]any{
		"name": "board_get_message", "arguments": map[string]any{"id": msg.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := srv.callTool(context.Background(), config.PrincipalFor("agent-alpha", config.RoleWriter), raw)
	if err != nil {
		t.Fatalf("get message: %v", err)
	}
	if structured(t, result)["content"] != body {
		t.Fatal("board_get_message must return the full body")
	}
}

func TestListDerivesTitleForUntitledMessages(t *testing.T) {
	srv := testServer(t)
	seedMessage(t, srv, "", "  请前端在 18:00 前完成联调  ")

	list := items(t, callList(t, srv, map[string]any{"limit": 10}))
	if list[0]["title"] != "请前端在 18:00 前完成联调" {
		t.Fatalf("derived title = %v", list[0]["title"])
	}
}

func TestServeHTTPListReturnsSummaries(t *testing.T) {
	srv := testServer(t)
	seedMessage(t, srv, "接口契约已冻结", strings.Repeat("详情", 40))

	created, err := srv.store.CreateMCPKey(context.Background(), model.CreateMCPKeyInput{
		Name: "dev-alice", Role: config.RoleWriter, ExpiresDays: 30,
	}, "admin")
	if err != nil {
		t.Fatalf("create mcp key: %v", err)
	}

	body := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"board_list_messages","arguments":{"limit":10}}}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+created.Key)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Result struct {
			StructuredContent map[string]any `json:"structuredContent"`
			IsError           bool           `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Result.IsError {
		t.Fatal("list returned a business error")
	}
	rawItems, ok := resp.Result.StructuredContent["items"].([]any)
	if !ok {
		t.Fatalf("items type = %T", resp.Result.StructuredContent["items"])
	}
	item, ok := rawItems[0].(map[string]any)
	if !ok {
		t.Fatalf("item type = %T", rawItems[0])
	}
	if _, exists := item["content"]; exists {
		t.Fatal("HTTP list response leaked message bodies")
	}
	if item["title"] != "接口契约已冻结" {
		t.Fatalf("title = %v", item["title"])
	}
	if item["author"] != "agent-alpha" {
		t.Fatalf("author = %v, want agent-alpha", item["author"])
	}
}
