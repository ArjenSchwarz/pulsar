package main

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

// FeedEntry pairs a parsed metadata block with the archive-relative path
// where the HTML file lives. The path is used to construct the public link.
type FeedEntry struct {
	Meta         Metadata
	RelativePath string // e.g. "2026-05/2026-05-16-github-fog-pr42.html"
}

// severityEmoji maps a severity to its display emoji.
func severityEmoji(s Severity) string {
	switch s {
	case SeverityLGTM:
		return "✅"
	case SeveritySuggestions:
		return "💡"
	case SeverityNeedsChanges:
		return "⚠️"
	case SeverityBlocking:
		return "🚫"
	}
	return ""
}

// prURLForHost builds the host-specific URL for viewing the PR/MR.
// Returns "" when the host isn't recognised.
func prURLForHost(repoURL string, pr int) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	switch u.Hostname() {
	case "github.com":
		return fmt.Sprintf("%s/pull/%d", repoURL, pr)
	case "gitlab.com":
		return fmt.Sprintf("%s/-/merge_requests/%d", repoURL, pr)
	}
	return ""
}

// itemTitle builds the human-readable feed item title.
func itemTitle(m Metadata) string {
	repo := repoName(m.RepoURL)
	emoji := severityEmoji(m.Severity)
	var ident string
	if m.PR != nil {
		ident = fmt.Sprintf("%s#%d", repo, *m.PR)
	} else {
		ident = fmt.Sprintf("%s@%s", repo, slugify(m.Branch))
	}
	return fmt.Sprintf("%s %s — %s", emoji, ident, m.Title)
}

// itemDescription builds the rich HTML description for an item.
func itemDescription(m Metadata) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<p><strong>Severity:</strong> %s</p>", html.EscapeString(string(m.Severity)))
	fmt.Fprintf(&b, "<p>%s</p>", html.EscapeString(m.Summary))
	if m.PR != nil {
		if link := prURLForHost(m.RepoURL, *m.PR); link != "" {
			fmt.Fprintf(&b, "<p><a href=\"%s\">View PR on %s</a></p>", html.EscapeString(link), html.EscapeString(hostDisplayName(m.RepoURL)))
		}
	}
	return b.String()
}

func hostDisplayName(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return ""
	}
	switch u.Hostname() {
	case "github.com":
		return "GitHub"
	case "gitlab.com":
		return "GitLab"
	}
	return u.Hostname()
}

// itemGUID builds the GUID for a metadata entry, using the date timestamp.
func itemGUID(m Metadata) string {
	t, err := time.Parse(time.RFC3339, m.Date)
	var ts int64
	if err == nil {
		ts = t.Unix()
	}
	if m.PR != nil {
		return prGUID(m.RepoURL, *m.PR, ts)
	}
	return branchGUID(m.RepoURL, m.Branch, ts)
}

// buildFeed assembles an RSS 2.0 feed from the given entries.
// entries are sorted in descending date order and truncated to maxItems.
func buildFeed(cfg Config, entries []FeedEntry, now time.Time) (string, error) {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Meta.Date > entries[j].Meta.Date
	})
	if cfg.MaxItems > 0 && len(entries) > cfg.MaxItems {
		entries = entries[:cfg.MaxItems]
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	feed := &feeds.Feed{
		Title:       cfg.Title,
		Link:        &feeds.Link{Href: baseURL + "/"},
		Description: cfg.Description,
		Created:     now,
	}

	for _, e := range entries {
		t, err := time.Parse(time.RFC3339, e.Meta.Date)
		if err != nil {
			t = now
		}
		link := baseURL + "/" + strings.TrimLeft(e.RelativePath, "/")
		item := &feeds.Item{
			Title:       itemTitle(e.Meta),
			Link:        &feeds.Link{Href: link},
			Description: itemDescription(e.Meta),
			Id:          itemGUID(e.Meta),
			Created:     t,
		}
		feed.Items = append(feed.Items, item)
	}

	rss := (&feeds.Rss{Feed: feed}).RssFeed()
	rss.Generator = "pulsar"
	return feeds.ToXML(&rssFeedXML{RssFeed: rss})
}

// rssFeedXML wraps RssFeed so the XmlFeed interface (FeedXml) is satisfied
// without going through gorilla/feeds' RSS converter again — we want the
// Generator field we just set to survive.
type rssFeedXML struct {
	*feeds.RssFeed
}

func (r *rssFeedXML) FeedXml() interface{} {
	return r.RssFeed.FeedXml()
}
