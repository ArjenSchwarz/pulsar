package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeReviewHTML(t *testing.T, dir, name string, meta string) string {
	t.Helper()
	html := fmt.Sprintf(`<!doctype html><html><head>
<script type="application/json" id="review-meta">
%s
</script>
</head><body>x</body></html>`, meta)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestPublish_FirstTime(t *testing.T) {
	src := t.TempDir()
	archive := t.TempDir()
	srcFile := writeReviewHTML(t, src, "review.html", `{
  "title": "Add caching",
  "repoUrl": "git@github.com:ArjenSchwarz/fog.git",
  "pr": 42,
  "severity": "blocking",
  "summary": "race"
}`)

	cfg := Config{Dir: archive}
	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	dest, err := publish(cfg, srcFile, now)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	want := filepath.Join(archive, "2026-05", "2026-05-16-github-fog-pr42.html")
	if dest != want {
		t.Errorf("dest = %q want %q", dest, want)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("dest does not exist: %v", err)
	}
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Errorf("source file should have been removed: %v", err)
	}

	// repoUrl in the destination should be normalised.
	data, _ := os.ReadFile(dest)
	if !strings.Contains(string(data), `"repoUrl": "https://github.com/ArjenSchwarz/fog"`) {
		t.Errorf("repoUrl not normalised in output: %s", data)
	}
	if !strings.Contains(string(data), `"date": "2026-05-16T12:00:00Z"`) {
		t.Errorf("date not stamped in output: %s", data)
	}
}

func TestPublish_DeletesPriorVersion(t *testing.T) {
	src := t.TempDir()
	archive := t.TempDir()
	cfg := Config{Dir: archive}

	srcFile := writeReviewHTML(t, src, "r.html", `{
  "title": "v1",
  "repoUrl": "https://github.com/a/b",
  "pr": 7,
  "severity": "lgtm",
  "summary": "s"
}`)
	first, err := publish(cfg, srcFile, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}

	srcFile2 := writeReviewHTML(t, src, "r2.html", `{
  "title": "v2",
  "repoUrl": "https://github.com/a/b",
  "pr": 7,
  "severity": "lgtm",
  "summary": "s"
}`)
	second, err := publish(cfg, srcFile2, time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if first == second {
		t.Fatalf("expected different destinations")
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Errorf("prior version should be removed: %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Errorf("new version should exist: %v", err)
	}
}

func TestPublish_CrossMonth(t *testing.T) {
	src := t.TempDir()
	archive := t.TempDir()
	cfg := Config{Dir: archive}

	srcFile := writeReviewHTML(t, src, "r.html", `{
  "title": "v1", "repoUrl": "https://github.com/a/b", "pr": 1,
  "severity": "lgtm", "summary": "s"
}`)
	first, _ := publish(cfg, srcFile, time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC))

	srcFile2 := writeReviewHTML(t, src, "r2.html", `{
  "title": "v2", "repoUrl": "https://github.com/a/b", "pr": 1,
  "severity": "lgtm", "summary": "s"
}`)
	second, err := publish(cfg, srcFile2, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !strings.Contains(second, "2026-05/") {
		t.Errorf("expected second in 2026-05: %s", second)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Errorf("first should be removed: %v", err)
	}
}

func TestPublish_DoesNotDeleteBranchOnPRPublish(t *testing.T) {
	src := t.TempDir()
	archive := t.TempDir()
	cfg := Config{Dir: archive}

	branchSrc := writeReviewHTML(t, src, "branch.html", `{
  "title": "branch review", "repoUrl": "https://github.com/a/b",
  "branch": "feature/foo", "severity": "lgtm", "summary": "s"
}`)
	branchDest, err := publish(cfg, branchSrc, time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("branch publish: %v", err)
	}

	prSrc := writeReviewHTML(t, src, "pr.html", `{
  "title": "pr review", "repoUrl": "https://github.com/a/b",
  "pr": 99, "severity": "lgtm", "summary": "s"
}`)
	if _, err := publish(cfg, prSrc, time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("pr publish: %v", err)
	}

	if _, err := os.Stat(branchDest); err != nil {
		t.Errorf("branch review should still exist after pr publish: %v", err)
	}
}

func TestPublish_InvalidMetadataRejected(t *testing.T) {
	src := t.TempDir()
	archive := t.TempDir()
	cfg := Config{Dir: archive}

	cases := []struct {
		name string
		meta string
	}{
		{"missing title", `{"repoUrl":"https://github.com/a/b","pr":1,"severity":"lgtm","summary":"s"}`},
		{"bad severity", `{"title":"x","repoUrl":"https://github.com/a/b","pr":1,"severity":"oops","summary":"s"}`},
		{"no pr or branch", `{"title":"x","repoUrl":"https://github.com/a/b","severity":"lgtm","summary":"s"}`},
		{"both pr and branch", `{"title":"x","repoUrl":"https://github.com/a/b","pr":1,"branch":"y","severity":"lgtm","summary":"s"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcFile := writeReviewHTML(t, src, tc.name+".html", tc.meta)
			if _, err := publish(cfg, srcFile, time.Now()); err == nil {
				t.Fatal("expected error")
			}
			if _, err := os.Stat(srcFile); err != nil {
				t.Errorf("source should be retained on error: %v", err)
			}
		})
	}
}

func TestPublish_FileNotFound(t *testing.T) {
	cfg := Config{Dir: t.TempDir()}
	_, err := publish(cfg, "/no/such/file.html", time.Now())
	if err == nil || !strings.Contains(err.Error(), "file not found") {
		t.Fatalf("expected file not found, got: %v", err)
	}
}

func TestPublish_NoMetadata(t *testing.T) {
	src := t.TempDir()
	srcFile := filepath.Join(src, "x.html")
	if err := os.WriteFile(srcFile, []byte("<html><body>no meta</body></html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg := Config{Dir: t.TempDir()}
	_, err := publish(cfg, srcFile, time.Now())
	if err == nil || !strings.Contains(err.Error(), "metadata") {
		t.Fatalf("expected metadata error, got: %v", err)
	}
}
