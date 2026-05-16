package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Severity is the review severity classification.
type Severity string

const (
	SeverityLGTM         Severity = "lgtm"
	SeveritySuggestions  Severity = "suggestions"
	SeverityNeedsChanges Severity = "needs-changes"
	SeverityBlocking     Severity = "blocking"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityLGTM, SeveritySuggestions, SeverityNeedsChanges, SeverityBlocking:
		return true
	}
	return false
}

// Metadata is the JSON payload embedded in each review HTML file.
// Pointer fields differentiate "absent" from "zero".
type Metadata struct {
	Title    string   `json:"title"`
	Date     string   `json:"date,omitempty"`
	RepoURL  string   `json:"repoUrl"`
	PR       *int     `json:"pr,omitempty"`
	Branch   string   `json:"branch,omitempty"`
	Severity Severity `json:"severity"`
	Summary  string   `json:"summary"`
}

// metaScriptRE matches the metadata script block. The id attribute may
// appear before or after the type attribute.
var metaScriptRE = regexp.MustCompile(`(?is)<script\b[^>]*\bid\s*=\s*"review-meta"[^>]*>(.*?)</script>`)

// extractMetadata finds and parses the review-meta script block.
func extractMetadata(html []byte) (Metadata, error) {
	var m Metadata
	loc := metaScriptRE.FindSubmatchIndex(html)
	if loc == nil {
		return m, fmt.Errorf("no metadata block found (looking for <script id=\"review-meta\">)")
	}
	body := html[loc[2]:loc[3]]
	if err := json.Unmarshal(body, &m); err != nil {
		return m, fmt.Errorf("metadata block is not valid JSON: %w", err)
	}
	return m, nil
}

// validate checks the required fields and enum values on metadata.
// repoUrl is expected to already be normalised by the caller.
func (m Metadata) validate() error {
	if strings.TrimSpace(m.Title) == "" {
		return fmt.Errorf("missing required field 'title' in metadata")
	}
	if strings.TrimSpace(m.RepoURL) == "" {
		return fmt.Errorf("missing required field 'repoUrl' in metadata")
	}
	if strings.TrimSpace(m.Summary) == "" {
		return fmt.Errorf("missing required field 'summary' in metadata")
	}
	if !m.Severity.Valid() {
		return fmt.Errorf("severity must be one of: lgtm, suggestions, needs-changes, blocking")
	}
	hasPR := m.PR != nil
	hasBranch := strings.TrimSpace(m.Branch) != ""
	if hasPR == hasBranch {
		return fmt.Errorf("exactly one of 'pr' or 'branch' must be set")
	}
	return nil
}

// replaceMetadata returns a copy of html with the review-meta script body
// replaced by the JSON serialisation of m.
func replaceMetadata(html []byte, m Metadata) ([]byte, error) {
	loc := metaScriptRE.FindSubmatchIndex(html)
	if loc == nil {
		return nil, fmt.Errorf("no metadata block found (looking for <script id=\"review-meta\">)")
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(html)-(loc[3]-loc[2])+len(body))
	out = append(out, html[:loc[2]]...)
	out = append(out, '\n')
	out = append(out, body...)
	out = append(out, '\n')
	out = append(out, html[loc[3]:]...)
	return out, nil
}
