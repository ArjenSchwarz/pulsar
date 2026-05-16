package main

import (
	"strings"
	"testing"
	"time"
)

func TestSeverityEmoji(t *testing.T) {
	cases := map[Severity]string{
		SeverityLGTM:         "✅",
		SeveritySuggestions:  "💡",
		SeverityNeedsChanges: "⚠️",
		SeverityBlocking:     "🚫",
	}
	for s, want := range cases {
		if got := severityEmoji(s); got != want {
			t.Errorf("severityEmoji(%q) = %q want %q", s, got, want)
		}
	}
}

func TestPRURLForHost(t *testing.T) {
	cases := []struct {
		url  string
		pr   int
		want string
	}{
		{"https://github.com/foo/bar", 42, "https://github.com/foo/bar/pull/42"},
		{"https://gitlab.com/foo/bar", 7, "https://gitlab.com/foo/bar/-/merge_requests/7"},
		{"https://bitbucket.org/foo/bar", 1, ""},
	}
	for _, tc := range cases {
		if got := prURLForHost(tc.url, tc.pr); got != tc.want {
			t.Errorf("prURLForHost(%q,%d) = %q want %q", tc.url, tc.pr, got, tc.want)
		}
	}
}

func TestItemTitle_PR(t *testing.T) {
	m := Metadata{
		Title:    "Add caching layer",
		RepoURL:  "https://github.com/ArjenSchwarz/fog",
		PR:       intPtr(42),
		Severity: SeverityBlocking,
	}
	got := itemTitle(m)
	want := "🚫 fog#42 — Add caching layer"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestItemTitle_Branch(t *testing.T) {
	m := Metadata{
		Title:    "Try a thing",
		RepoURL:  "https://github.com/ArjenSchwarz/fog",
		Branch:   "feature/foo",
		Severity: SeverityLGTM,
	}
	got := itemTitle(m)
	want := "✅ fog@feature-foo — Try a thing"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestItemDescription_GitHubPR(t *testing.T) {
	m := Metadata{
		RepoURL:  "https://github.com/ArjenSchwarz/fog",
		PR:       intPtr(42),
		Severity: SeverityBlocking,
		Summary:  "Race condition.",
	}
	got := itemDescription(m)
	if !strings.Contains(got, "<strong>Severity:</strong> blocking") {
		t.Errorf("missing severity: %s", got)
	}
	if !strings.Contains(got, "Race condition.") {
		t.Errorf("missing summary: %s", got)
	}
	if !strings.Contains(got, "https://github.com/ArjenSchwarz/fog/pull/42") {
		t.Errorf("missing PR link: %s", got)
	}
}

func TestItemDescription_BranchOmitsLink(t *testing.T) {
	m := Metadata{
		RepoURL:  "https://github.com/ArjenSchwarz/fog",
		Branch:   "feature/x",
		Severity: SeverityLGTM,
		Summary:  "fine",
	}
	got := itemDescription(m)
	if strings.Contains(got, "View PR") {
		t.Errorf("should not contain PR link: %s", got)
	}
}

func TestItemGUID(t *testing.T) {
	m := Metadata{
		RepoURL:  "https://github.com/ArjenSchwarz/fog",
		PR:       intPtr(42),
		Date:     "2026-05-16T12:00:00Z",
		Severity: SeverityBlocking,
	}
	got := itemGUID(m)
	want := "https://github.com/ArjenSchwarz/fog#pr-42-1778932800"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestBuildFeed_EmptyAndSorting(t *testing.T) {
	cfg := Config{
		Title:       "Code Reviews",
		Description: "test",
		BaseURL:     "https://mac.tailnet.ts.net",
		MaxItems:    10,
	}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)

	out, err := buildFeed(cfg, nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "<rss") || !strings.Contains(out, "<channel>") {
		t.Errorf("not an RSS doc: %s", out)
	}
	if !strings.Contains(out, "<generator>pulsar</generator>") {
		t.Errorf("missing generator: %s", out)
	}

	entries := []FeedEntry{
		{Meta: Metadata{Title: "older", RepoURL: "https://github.com/a/b", PR: intPtr(1), Severity: SeverityLGTM, Summary: "s", Date: "2026-05-10T00:00:00Z"}, RelativePath: "2026-05/older.html"},
		{Meta: Metadata{Title: "newer", RepoURL: "https://github.com/a/b", PR: intPtr(2), Severity: SeverityLGTM, Summary: "s", Date: "2026-05-15T00:00:00Z"}, RelativePath: "2026-05/newer.html"},
	}
	out, err = buildFeed(cfg, entries, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oldIdx := strings.Index(out, "older")
	newIdx := strings.Index(out, "newer")
	if newIdx < 0 || oldIdx < 0 || newIdx > oldIdx {
		t.Errorf("expected newer to appear before older: new=%d old=%d", newIdx, oldIdx)
	}
	if !strings.Contains(out, "https://mac.tailnet.ts.net/2026-05/newer.html") {
		t.Errorf("missing link: %s", out)
	}
}

func TestBuildFeed_MaxItems(t *testing.T) {
	cfg := Config{
		Title:    "x",
		BaseURL:  "https://x",
		MaxItems: 2,
	}
	entries := []FeedEntry{
		{Meta: Metadata{Title: "a", RepoURL: "https://github.com/a/b", PR: intPtr(1), Severity: SeverityLGTM, Summary: "s", Date: "2026-05-10T00:00:00Z"}},
		{Meta: Metadata{Title: "b", RepoURL: "https://github.com/a/b", PR: intPtr(2), Severity: SeverityLGTM, Summary: "s", Date: "2026-05-11T00:00:00Z"}},
		{Meta: Metadata{Title: "c", RepoURL: "https://github.com/a/b", PR: intPtr(3), Severity: SeverityLGTM, Summary: "s", Date: "2026-05-12T00:00:00Z"}},
	}
	out, err := buildFeed(cfg, entries, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Count(out, "<item>") != 2 {
		t.Errorf("expected 2 items, got: %d\n%s", strings.Count(out, "<item>"), out)
	}
	// Truncation keeps the newest two ("c" and "b"), drops "a".
	if strings.Contains(out, "<title>✅ b#1 — a</title>") {
		t.Errorf("oldest entry should be truncated: %s", out)
	}
}
