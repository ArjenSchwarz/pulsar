package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupArchive(t *testing.T) string {
	t.Helper()
	archive := t.TempDir()
	cfg := Config{Dir: archive}

	src := t.TempDir()
	writeReviewHTML(t, src, "a.html", `{
"title":"first","repoUrl":"https://github.com/ArjenSchwarz/fog","pr":1,
"severity":"lgtm","summary":"good"}`)
	writeReviewHTML(t, src, "b.html", `{
"title":"second","repoUrl":"https://github.com/ArjenSchwarz/fog","pr":2,
"severity":"blocking","summary":"bad"}`)
	if _, err := publish(cfg, filepath.Join(src, "a.html"), time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("publish a: %v", err)
	}
	if _, err := publish(cfg, filepath.Join(src, "b.html"), time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("publish b: %v", err)
	}
	return archive
}

func TestServe_FeedXML(t *testing.T) {
	archive := setupArchive(t)
	cfg := Config{
		Dir:         archive,
		BaseURL:     "https://mac.tailnet.ts.net",
		Title:       "Code Reviews",
		Description: "test",
		MaxItems:    10,
	}
	srv := httptest.NewServer(newServeHandler(cfg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/feed.xml")
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/rss+xml") {
		t.Errorf("content-type = %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, "<rss") || !strings.Contains(s, "<channel>") {
		t.Errorf("not an rss feed: %s", s)
	}
	if !strings.Contains(s, "fog#1") || !strings.Contains(s, "fog#2") {
		t.Errorf("missing items: %s", s)
	}
}

func TestServe_StaticFile(t *testing.T) {
	archive := setupArchive(t)
	cfg := Config{Dir: archive, BaseURL: "https://x", Title: "x"}
	srv := httptest.NewServer(newServeHandler(cfg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/2026-05/2026-05-10-github-fog-pr1.html")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "first") {
		t.Errorf("body missing content")
	}
}

func TestServe_PathTraversalRejected(t *testing.T) {
	archive := setupArchive(t)
	// Drop a secret file outside the archive — at the parent of the archive temp dir.
	parent := filepath.Dir(archive)
	secret := filepath.Join(parent, "secret.txt")
	if err := os.WriteFile(secret, []byte("nope"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	defer os.Remove(secret)

	cfg := Config{Dir: archive, BaseURL: "https://x"}
	srv := httptest.NewServer(newServeHandler(cfg))
	defer srv.Close()

	// http.Client cleans paths, so build the request manually.
	req, _ := http.NewRequest("GET", srv.URL+"/", nil)
	req.URL.Opaque = "//" + req.URL.Host + "/..%2Fsecret.txt"
	// Easier: just probe ".." in the path via raw URL parsing.
	resp, err := http.Get(srv.URL + "/..%2Fsecret.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("traversal succeeded; body=%q", body)
	}
}

func TestServe_Index(t *testing.T) {
	archive := setupArchive(t)
	cfg := Config{Dir: archive, BaseURL: "https://x", Title: "Code Reviews"}
	srv := httptest.NewServer(newServeHandler(cfg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<h1>Code Reviews</h1>") {
		t.Errorf("missing title: %s", body)
	}
	if !strings.Contains(string(body), "/feed.xml") {
		t.Errorf("missing feed link")
	}
}

func TestScanArchive_SkipsInvalid(t *testing.T) {
	archive := t.TempDir()
	sub := filepath.Join(archive, "2026-05")
	os.MkdirAll(sub, 0o755)
	if err := os.WriteFile(filepath.Join(sub, "no-meta.html"), []byte("<html>no</html>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "bad.html"), []byte(`<script id="review-meta">{not json}</script>`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	good := `<script id="review-meta">{"title":"x","repoUrl":"https://github.com/a/b","pr":1,"severity":"lgtm","summary":"s","date":"2026-05-15T00:00:00Z"}</script>`
	if err := os.WriteFile(filepath.Join(sub, "good.html"), []byte(good), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := scanArchive(archive)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
	if entries[0].Meta.Title != "x" {
		t.Errorf("wrong entry: %+v", entries[0])
	}
}
