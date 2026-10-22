package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"agent-board/internal/model"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "board.db"), model.ArchivePolicy{
		RetainDays:      7,
		MaxItems:        200,
		AutoArchive:     true,
		IntervalMinutes: 5,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestCreateIsIdempotentAndUpdateConflicts(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	in := model.CreateInput{Title: "contract", Content: "v1", ClientRequestID: "req-1", Tags: []string{"api"}}
	first, created, err := st.CreateMessage(ctx, "agent-a", in)
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	again, created, err := st.CreateMessage(ctx, "agent-a", in)
	if err != nil || created || again.ID != first.ID {
		t.Fatalf("idempotent create: created=%v id=%s err=%v", created, again.ID, err)
	}
	content := "v2"
	updated, err := st.UpdateMessage(ctx, first.ID, "agent-b", model.UpdateInput{Content: &content, ExpectedVersion: 1})
	if err != nil || updated.Version != 2 {
		t.Fatalf("update: version=%d err=%v", updated.Version, err)
	}
	_, err = st.UpdateMessage(ctx, first.ID, "agent-c", model.UpdateInput{Content: &content, ExpectedVersion: 1})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if conflict.Current == nil || conflict.Current.Version != 2 {
		t.Fatalf("conflict did not carry current message")
	}
}

func TestArchiveByCountAndRestore(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		if _, _, err := st.CreateMessage(ctx, "agent-a", model.CreateInput{
			Title: "note", Content: "message", ClientRequestID: "cap-" + string(rune('0'+i)),
		}); err != nil {
			t.Fatal(err)
		}
	}
	policy, _ := st.GetArchivePolicy(ctx)
	policy.MaxItems = 3
	if _, err := st.SetArchivePolicy(ctx, policy); err != nil {
		t.Fatal(err)
	}
	outcome, err := st.RunArchive(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result.ArchivedCount != 2 || outcome.Result.OverflowCount != 2 {
		t.Fatalf("archive counts: %+v", outcome.Result)
	}
	stats, _ := st.Stats(ctx, map[string]string{"agent-a": "writer"})
	if stats.Active != 3 || stats.Archived != 2 {
		t.Fatalf("stats after archive: active=%d archived=%d", stats.Active, stats.Archived)
	}
	restored, err := st.RestoreMessage(ctx, outcome.Archived[0].ID, "admin")
	if err != nil || restored.ArchivedAt != nil {
		t.Fatalf("restore failed: %v", err)
	}
	stats, _ = st.Stats(ctx, map[string]string{"agent-a": "writer"})
	if stats.Active != 4 || stats.Archived != 1 {
		t.Fatalf("stats after restore: active=%d archived=%d", stats.Active, stats.Archived)
	}
}

func TestArchiveByAge(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	msg, _, err := st.CreateMessage(ctx, "agent-a", model.CreateInput{Content: "old note"})
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-40 * 24 * time.Hour).UnixMilli()
	if _, err := st.DB().ExecContext(ctx, "UPDATE messages SET created_at=? WHERE id=?", old, msg.ID); err != nil {
		t.Fatal(err)
	}
	outcome, err := st.RunArchive(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Result.RetentionCount != 1 || outcome.Result.OverflowCount != 0 {
		t.Fatalf("retention counts: %+v", outcome.Result)
	}
	got, _ := st.GetMessage(ctx, msg.ID)
	if got.ArchivedAt == nil || got.ArchivedReason != "retention_days" {
		t.Fatalf("message was not archived by age: %+v", got)
	}
}

func TestEditLease(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	msg, _, err := st.CreateMessage(ctx, "agent-a", model.CreateInput{Content: "locked note"})
	if err != nil {
		t.Fatal(err)
	}
	lock, err := st.AcquireLock(ctx, msg.ID, "agent-a", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.AcquireLock(ctx, msg.ID, "agent-b", time.Minute)
	var locked *LockedError
	if !errors.As(err, &locked) || locked.Lock.Owner != "agent-a" {
		t.Fatalf("expected lock conflict, got %v", err)
	}
	if err := st.ReleaseLock(ctx, msg.ID, "agent-b", "", false); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized release, got %v", err)
	}
	if err := st.ReleaseLock(ctx, msg.ID, "agent-a", lock.Token, false); err != nil {
		t.Fatalf("release own lock: %v", err)
	}
}

func TestCursorPagination(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, _, err := st.CreateMessage(ctx, "agent-a", model.CreateInput{Content: "note", ClientRequestID: "page-" + string(rune('a'+i))}); err != nil {
			t.Fatal(err)
		}
	}
	page1, err := st.ListMessages(ctx, model.ListOptions{Limit: 3, Status: "active", Sort: "newest"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Items) != 3 || !page1.HasMore || page1.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", page1)
	}
	page2, err := st.ListMessages(ctx, model.ListOptions{Limit: 3, Status: "active", Sort: "newest", Cursor: page1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Items) != 2 || page2.HasMore {
		t.Fatalf("unexpected second page: %+v", page2)
	}
}

func TestMCPKeyLifecycle(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	created, err := st.CreateMCPKey(ctx, model.CreateMCPKeyInput{
		Name: "dev-alice", Role: "writer", ExpiresDays: 30,
	}, "admin")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if created.Key == "" || created.Record.Prefix == "" || created.Record.Status != "active" {
		t.Fatalf("invalid created key: %+v", created)
	}
	if _, err := st.CreateMCPKey(ctx, model.CreateMCPKeyInput{Name: "dev-alice", Role: "writer"}, "admin"); !errors.Is(err, ErrKeyNameTaken) {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
	key, ok, err := st.AuthenticateMCPKey(ctx, created.Key)
	if err != nil || !ok || key.Name != "dev-alice" || key.Role != "writer" {
		t.Fatalf("authenticate key: ok=%v key=%+v err=%v", ok, key, err)
	}
	keys, err := st.ListMCPKeys(ctx)
	if err != nil || len(keys) != 1 {
		t.Fatalf("list keys: %v %+v", err, keys)
	}
	if _, ok, _ := st.AuthenticateMCPKey(ctx, "ab_mcp_invalid"); ok {
		t.Fatal("invalid key authenticated")
	}
	revoked, err := st.RevokeMCPKey(ctx, created.Record.ID, "admin")
	if err != nil || revoked.Status != "revoked" {
		t.Fatalf("revoke: %+v %v", revoked, err)
	}
	if _, ok, _ := st.AuthenticateMCPKey(ctx, created.Key); ok {
		t.Fatal("revoked key authenticated")
	}
}

func TestExpiredMCPKey(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	created, err := st.CreateMCPKey(ctx, model.CreateMCPKeyInput{Name: "dev-expired", Role: "writer", ExpiresDays: 1}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Hour).UnixMilli()
	if _, err := st.DB().ExecContext(ctx, "UPDATE mcp_keys SET expires_at=? WHERE id=?", past, created.Record.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := st.AuthenticateMCPKey(ctx, created.Key); ok {
		t.Fatal("expired key authenticated")
	}
}
