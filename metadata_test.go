package main

import (
	"strings"
	"testing"
)

func intPtr(i int) *int { return &i }

const validHTML = `<!doctype html>
<html>
<head>
<script type="application/json" id="review-meta">
{
  "title": "Add caching layer",
  "repoUrl": "https://github.com/ArjenSchwarz/fog",
  "pr": 42,
  "severity": "blocking",
  "summary": "Race condition in cache invalidation."
}
</script>
</head>
<body>review body</body>
</html>`

func TestExtractMetadata(t *testing.T) {
	m, err := extractMetadata([]byte(validHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Title != "Add caching layer" {
		t.Errorf("title = %q", m.Title)
	}
	if m.PR == nil || *m.PR != 42 {
		t.Errorf("pr = %v", m.PR)
	}
	if m.Severity != SeverityBlocking {
		t.Errorf("severity = %q", m.Severity)
	}
}

func TestExtractMetadata_Missing(t *testing.T) {
	html := `<!doctype html><html><head></head><body></body></html>`
	if _, err := extractMetadata([]byte(html)); err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractMetadata_Malformed(t *testing.T) {
	html := `<script id="review-meta">{not json}</script>`
	if _, err := extractMetadata([]byte(html)); err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractMetadata_IDAttrOrder(t *testing.T) {
	html := `<script id="review-meta" type="application/json">{"title":"x","repoUrl":"https://github.com/a/b","pr":1,"severity":"lgtm","summary":"y"}</script>`
	if _, err := extractMetadata([]byte(html)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate(t *testing.T) {
	base := Metadata{
		Title:    "x",
		RepoURL:  "https://github.com/a/b",
		PR:       intPtr(1),
		Severity: SeverityLGTM,
		Summary:  "y",
	}
	cases := []struct {
		name    string
		mutate  func(*Metadata)
		wantErr string
	}{
		{"happy path pr", func(m *Metadata) {}, ""},
		{"happy path branch", func(m *Metadata) {
			m.PR = nil
			m.Branch = "feature/foo"
		}, ""},
		{"missing title", func(m *Metadata) { m.Title = "" }, "title"},
		{"missing summary", func(m *Metadata) { m.Summary = "" }, "summary"},
		{"missing repoUrl", func(m *Metadata) { m.RepoURL = "" }, "repoUrl"},
		{"bad severity", func(m *Metadata) { m.Severity = "wat" }, "severity"},
		{"both pr and branch", func(m *Metadata) { m.Branch = "x" }, "exactly one"},
		{"neither pr nor branch", func(m *Metadata) { m.PR = nil }, "exactly one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base
			tc.mutate(&m)
			err := m.validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestReplaceMetadata(t *testing.T) {
	m, _ := extractMetadata([]byte(validHTML))
	m.Date = "2026-05-16T12:00:00Z"
	out, err := replaceMetadata([]byte(validHTML), m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), `"date": "2026-05-16T12:00:00Z"`) {
		t.Fatalf("date not written into output: %s", out)
	}
	// Round-trip: extracting again should yield the same metadata.
	m2, err := extractMetadata(out)
	if err != nil {
		t.Fatalf("re-extract: %v", err)
	}
	if m2.Date != m.Date {
		t.Fatalf("round-trip date mismatch: %q vs %q", m2.Date, m.Date)
	}
}
