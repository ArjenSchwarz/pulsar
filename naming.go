package main

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// normalizeRepoURL converts SSH or HTTP repo URLs to canonical HTTPS form
// without a trailing .git or slash. Returns an error if the URL has no host
// or path segment after normalisation.
func normalizeRepoURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("missing required field 'repoUrl' in metadata")
	}

	// SSH form: user@host:path → https://host/path
	if user, rest, ok := strings.Cut(raw, "@"); ok && user != "" && !strings.ContainsAny(user, "/:") {
		if host, path, ok := strings.Cut(rest, ":"); ok && host != "" && path != "" {
			raw = "https://" + host + "/" + path
		}
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("repoUrl is not a valid URL: %w", err)
	}
	u.Scheme = "https"
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.Path = strings.TrimSuffix(u.Path, ".git")
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""

	if u.Host == "" {
		return "", fmt.Errorf("repoUrl must have a host and a path segment")
	}
	if u.Path == "" || u.Path == "/" {
		return "", fmt.Errorf("repoUrl must have a host and a path segment")
	}

	return u.String(), nil
}

// hostLabel returns the first label of the hostname (e.g. "github" from "github.com").
func hostLabel(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	label, _, _ := strings.Cut(u.Hostname(), ".")
	return strings.ToLower(label)
}

// repoName returns the last path component of repoURL.
func repoName(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	return path.Base(strings.Trim(u.Path, "/"))
}

// slugify lowercases the input, replaces runs of non-alphanumeric characters
// with a single dash, and trims leading/trailing dashes.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// monthSubdir returns the YYYY-MM segment used as the archive subdirectory.
func monthSubdir(date string) string {
	if len(date) < 7 {
		return ""
	}
	return date[:7]
}

// dateSegment returns the YYYY-MM-DD prefix used in filenames.
func dateSegment(date string) string {
	if len(date) < 10 {
		return ""
	}
	return date[:10]
}

// prFilename builds the canonical filename for a PR review.
// date should be YYYY-MM-DD.
func prFilename(date, repoURL string, pr int) string {
	return fmt.Sprintf("%s-%s-%s-pr%d.html", date, hostLabel(repoURL), repoName(repoURL), pr)
}

// branchFilename builds the canonical filename for a branch review.
func branchFilename(date, repoURL, branch string) string {
	return fmt.Sprintf("%s-%s-%s-branch-%s.html", date, hostLabel(repoURL), repoName(repoURL), slugify(branch))
}

// prGlob returns the glob pattern used to find prior PR review files.
// archiveDir is the root archive directory.
func prGlob(archiveDir, repoURL string, pr int) string {
	return fmt.Sprintf("%s/*/*-%s-%s-pr%d.html", archiveDir, hostLabel(repoURL), repoName(repoURL), pr)
}

// branchGlob returns the glob pattern used to find prior branch review files.
func branchGlob(archiveDir, repoURL, branch string) string {
	return fmt.Sprintf("%s/*/*-%s-%s-branch-%s.html", archiveDir, hostLabel(repoURL), repoName(repoURL), slugify(branch))
}

// prGUID builds the GUID for a PR review using the unix timestamp.
func prGUID(repoURL string, pr int, unix int64) string {
	return fmt.Sprintf("%s#pr-%d-%d", repoURL, pr, unix)
}

// branchGUID builds the GUID for a branch review using the unix timestamp.
func branchGUID(repoURL, branch string, unix int64) string {
	return fmt.Sprintf("%s#branch-%s-%d", repoURL, slugify(branch), unix)
}
