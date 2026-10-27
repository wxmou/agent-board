package model

import (
	"strings"
	"testing"
	"time"
)

func TestSummarizeKeepsTitleAndBoundsBody(t *testing.T) {
	body := strings.Repeat("契约", 900) // 1800 runes
	m := Message{
		ID:        "m-1",
		Title:     "接口契约已冻结",
		Content:   body,
		Author:    "agent-alpha",
		Tags:      []string{"api"},
		Priority:  "high",
		Pinned:    true,
		Version:   3,
		CreatedAt: time.Unix(0, 0).UTC(),
	}
	s := m.Summarize()

	if s.Title != "接口契约已冻结" {
		t.Fatalf("title = %q, want the explicit title", s.Title)
	}
	if s.ContentChars != 1800 {
		t.Fatalf("content_chars = %d, want 1800", s.ContentChars)
	}
	if got := len([]rune(s.Excerpt)); got != SummaryExcerptRunes+1 {
		t.Fatalf("excerpt runes = %d, want %d (limit + ellipsis)", got, SummaryExcerptRunes+1)
	}
	if !strings.HasSuffix(s.Excerpt, "…") {
		t.Fatalf("excerpt should be marked as truncated: %q", s.Excerpt)
	}
	if s.Version != 3 || s.Author != "agent-alpha" || !s.Pinned {
		t.Fatalf("metadata was not carried over: %+v", s)
	}
}

func TestSummarizeDerivesTitleWhenMissing(t *testing.T) {
	m := Message{ID: "m-2", Content: "  请前端在 18:00 前完成联调  "}
	s := m.Summarize()
	if s.Title != "请前端在 18:00 前完成联调" {
		t.Fatalf("derived title = %q", s.Title)
	}
	if s.Excerpt != "请前端在 18:00 前完成联调" {
		t.Fatalf("excerpt = %q", s.Excerpt)
	}

	long := Message{ID: "m-3", Content: strings.Repeat("很长的正文", 100)}
	derived := long.Summarize().Title
	if got := len([]rune(derived)); got != SummaryTitleRunes+1 {
		t.Fatalf("derived title runes = %d, want %d", got, SummaryTitleRunes+1)
	}
}

func TestExcerptCollapsesNewlines(t *testing.T) {
	got := Excerpt("第一行\n\n第二行\t第三行", 100)
	if got != "第一行 第二行 第三行" {
		t.Fatalf("excerpt = %q", got)
	}
}

func TestTruncateRunesBoundaries(t *testing.T) {
	if got := TruncateRunes("abcdef", 6); got != "abcdef" {
		t.Fatalf("exact length should not be marked: %q", got)
	}
	if got := TruncateRunes("abcdef", 3); got != "abc…" {
		t.Fatalf("truncated = %q", got)
	}
	if got := TruncateRunes("中文内容", 2); got != "中文…" {
		t.Fatalf("multibyte truncation = %q", got)
	}
	if got := TruncateRunes("anything", 0); got != "" {
		t.Fatalf("zero limit = %q", got)
	}
}
