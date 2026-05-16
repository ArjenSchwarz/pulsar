# Pulsar — Code Review Feed Server

## Overview

Pulsar is a small Go binary that turns a directory of locally-generated code review HTML files into an RSS-discoverable archive accessible over Tailscale. It has two responsibilities:

- **`publish`** — invoked by the code-review agent skill after generating a review HTML file. Validates the embedded metadata, canonicalises the filename, places the file in the central archive, and removes any prior version for the same PR/branch.
- **`serve`** — long-running HTTP server (managed by a LaunchAgent) that serves the archive directory as static files and generates an RSS 2.0 feed on demand. `tailscale serve` fronts it to expose it on the tailnet.

The agent skill writes review HTML files in its own working directory (e.g. inside a repo) and then invokes `pulsar publish <file>`. The agent does not need write access to `~/CodeReviews/`; only the `pulsar` binary does.

## Architecture

```
┌──────────────────┐   publish <file>   ┌─────────────────────────┐
│ Agent skill      │───────────────────▶│ pulsar publish          │
│ (writes HTML in  │                    │  - validate metadata    │
│  repo dir)       │                    │  - normalise repoUrl    │
└──────────────────┘                    │  - refresh date         │
                                        │  - delete prior version │
                                        │  - move to archive      │
                                        └────────────┬────────────┘
                                                     │
                                                     ▼
                                        ┌─────────────────────────┐
                                        │ ~/CodeReviews/          │
                                        │  └── 2026-05/           │
                                        │       └── *.html        │
                                        └────────────┬────────────┘
                                                     │
                                                     ▼
                                        ┌─────────────────────────┐
                                        │ pulsar serve            │
                                        │  - GET /feed.xml        │
                                        │  - GET /<subdir>/<file> │
                                        │  (bound to localhost)   │
                                        └────────────┬────────────┘
                                                     │
                                                     ▼
                                        ┌─────────────────────────┐
                                        │ tailscale serve         │
                                        │  https://mac.tailnet…   │
                                        └────────────┬────────────┘
                                                     │
                                                     ▼
                                                 Feed reader
                                            (phone via Tailscale)
```

## Repository Layout

Single Go module, flat layout. Module size doesn’t justify `cmd/` and `internal/` structure.

```
pulsar/
├── go.mod
├── go.sum
├── main.go               # cobra root command, Execute()
├── publish.go            # publish subcommand (cobra.Command + RunE)
├── serve.go              # serve subcommand
├── install.go            # install/uninstall subcommands (LaunchAgent)
├── metadata.go           # JSON metadata extraction, parsing, validation
├── naming.go             # slugify, URL normalize, filename construction
├── feed.go               # RSS generation
├── config.go             # viper setup, Config struct, flag binding
├── metadata_test.go
├── naming_test.go
├── publish_test.go
├── serve_test.go
├── feed_test.go
├── README.md
└── docs/
    └── agent-contract.md  # what the agent skill must produce
```

## Configuration

Configuration is handled via [Cobra](https://github.com/spf13/cobra) (subcommand dispatch + flag parsing) and [Viper](https://github.com/spf13/viper) (layered config). Precedence, highest to lowest:

1. Command-line flags
1. Environment variables (prefix `PULSAR_`, automatic via `viper.SetEnvPrefix("PULSAR")` and `viper.AutomaticEnv()`)
1. Config file (optional, `$HOME/.config/pulsar/config.yaml`)
1. Built-in defaults

Viper is more than v1 strictly needs, but it’s negligible weight and means a config file can be introduced later without a refactor. Cobra is the natural pair for it given pulsar has multiple subcommands.

|Setting       |Flag           |Env var             |Default                         |Subcommand|
|--------------|---------------|--------------------|--------------------------------|----------|
|Archive dir   |`--dir`        |`PULSAR_DIR`        |`$HOME/CodeReviews`             |both      |
|HTTP port     |`--port`       |`PULSAR_PORT`       |`8765`                          |serve     |
|Base URL      |`--base-url`   |`PULSAR_BASE_URL`   |(required, no default)          |serve     |
|Channel title |`--title`      |`PULSAR_TITLE`      |`Code Reviews`                  |serve     |
|Channel desc  |`--description`|`PULSAR_DESCRIPTION`|`Locally generated code reviews`|serve     |
|Max feed items|`--max-items`  |`PULSAR_MAX_ITEMS`  |`200`                           |serve     |

Config file location is conventional macOS XDG (`$HOME/.config/pulsar/config.yaml`). Loaded automatically if present; absent file is not an error. Example:

```yaml
dir: /Users/arjen/CodeReviews
port: 8765
base_url: https://mac.tailnet-name.ts.net
title: Code Reviews
description: Locally generated code reviews
max_items: 200
```

Flag binding is set up in `config.go` via `viper.BindPFlag(...)` so all four sources are unified into a single `Config` struct that the subcommands consume.

## Data Model

### Metadata Schema

Every review HTML must contain a `<script type="application/json" id="review-meta">` block in `<head>`. Schema:

```json
{
  "title":    "string, required - human-readable review title",
  "date":     "string, set by publish, ISO 8601 with timezone",
  "repoUrl":  "string, required - canonical HTTPS URL, no .git, no trailing slash",
  "pr":       "integer, optional - one of pr/branch must be set",
  "branch":   "string, optional - one of pr/branch must be set",
  "severity": "string, required - enum: lgtm|suggestions|needs-changes|blocking",
  "summary":  "string, required - short summary for feed item description"
}
```

Validation rules enforced by `publish`:

- `title`, `repoUrl`, `severity`, `summary` must be present and non-empty.
- Exactly one of `pr` (integer) or `branch` (string) must be present.
- `severity` must be one of the four enum values.
- `repoUrl` must parse as a URL with a host and at least one path segment.
- `date` is ignored on input; `publish` always sets it to now.

The `date` field is rewritten in place in the HTML by `publish` before the file is moved. (Implementation: locate the script tag, replace the JSON body, write the file. No HTML parsing needed beyond finding the script block by id — a regex or `golang.org/x/net/html` walk both work.)

### Severity Enum

|Value          |Meaning                         |
|---------------|--------------------------------|
|`lgtm`         |No concerns                     |
|`suggestions`  |Optional improvements           |
|`needs-changes`|Changes recommended before merge|
|`blocking`     |Issues that should prevent merge|

Feed item titles are prefixed with a severity marker (see Feed Format below).

### Filename Patterns

```
PR review:     <date>-<host>-<repo>-pr<num>.html
Branch review: <date>-<host>-<repo>-branch-<slug>.html
```

Where:

- `<date>` = `YYYY-MM-DD` from the publish timestamp.
- `<host>` = first label of `repoUrl` hostname (e.g. `github` from `github.com`).
- `<repo>` = last path component of `repoUrl` (e.g. `fog` from `https://github.com/ArjenSchwarz/fog`).
- `<num>` = the PR number, no padding.
- `<slug>` = slugified branch name.

Examples:

```
2026-05-16-github-fog-pr42.html
2026-05-16-github-fog-branch-feature-foo-bar.html
```

Files are stored under a month-based subdirectory:

```
~/CodeReviews/2026-05/2026-05-16-github-fog-pr42.html
```

### URL Normalisation

Applied to `repoUrl` by `publish` before further processing:

1. Convert SSH form to HTTPS: `git@host:path` → `https://host/path`.
1. Force scheme to `https`.
1. Strip trailing `.git`.
1. Strip trailing `/`.
1. Reject if no host or no path segment.

Examples:

```
git@github.com:ArjenSchwarz/fog.git     →  https://github.com/ArjenSchwarz/fog
https://github.com/ArjenSchwarz/fog/    →  https://github.com/ArjenSchwarz/fog
http://gitlab.com/foo/bar.git           →  https://gitlab.com/foo/bar
```

The normalised URL is written back into the metadata block before the file is archived.

### Slugification

Applied to branch names for filename construction:

1. Lowercase.
1. Replace runs of non-alphanumeric characters with a single `-`.
1. Trim leading/trailing `-`.

Examples:

```
feature/foo-bar      →  feature-foo-bar
Fix/Bug 123          →  fix-bug-123
arjen/spike--idea/   →  arjen-spike-idea
```

### GUID Format

```
PR:     ${repoUrl}#pr-${pr}-${unix_timestamp}
Branch: ${repoUrl}#branch-${slug}-${unix_timestamp}
```

Example: `https://github.com/ArjenSchwarz/fog#pr-42-1747396800`

Timestamp comes from the `date` field set by `publish`. Re-publishing the same review produces a different GUID (different timestamp), which is what causes feed readers to treat the new version as unread.

## Subcommands

### `pulsar publish <file>`

Flow:

1. Read the file at `<file>`.
1. Locate and parse the `<script id="review-meta">` JSON block. Fail with a clear error if missing or invalid JSON.
1. Run all validation rules. Fail with field-specific errors.
1. Normalise `repoUrl`. Validate it parses correctly post-normalisation.
1. Set `date` to `time.Now().Format(time.RFC3339)`.
1. Compute the destination filename and subdirectory.
1. Find prior versions to delete:
- PR case: `filepath.Glob(dir + "/*/*-<host>-<repo>-pr<num>.html")`.
- Branch case: `filepath.Glob(dir + "/*/*-<host>-<repo>-branch-<slug>.html")`.
- Note: do **not** clean up branch reviews when publishing a PR review for the same code — leave them as historical context per design.
1. Ensure target subdirectory exists (`MkdirAll`).
1. Write the updated HTML (with refreshed date in the metadata block) to the destination path. Use `os.WriteFile` with atomic semantics where possible — write to a temp file in the same directory, then rename.
1. Delete any prior versions found in step 7.
1. Delete the source file.
1. Print the destination path to stdout. Exit 0.

Errors are written to stderr with non-zero exit code. Common error messages:

```
pulsar: missing required field 'title' in metadata
pulsar: severity must be one of: lgtm, suggestions, needs-changes, blocking
pulsar: exactly one of 'pr' or 'branch' must be set
pulsar: repoUrl must have a host and a path segment
pulsar: file not found: /path/to/review.html
pulsar: no metadata block found (looking for <script id="review-meta">)
```

### `pulsar serve`

Flow:

1. Parse flags. `--base-url` is required and must be a valid URL.
1. Start an HTTP server bound to `127.0.0.1:<port>`.
1. Routes:
- `GET /feed.xml` — generate RSS, return with `Content-Type: application/rss+xml`.
- `GET /<subdir>/<file>` — serve static file from `<dir>/<subdir>/<file>`. Use `http.ServeFile` with path validation to prevent traversal.
- `GET /` — simple HTML index listing recent items (optional but useful for browser access; can be deferred).
1. Run until interrupted. Log requests to stdout (LaunchAgent captures to a log file).

Feed generation flow (per request):

1. Walk `<dir>` for files matching `*.html`.
1. For each file: extract the metadata block. Skip and log a warning if missing or invalid (don’t fail the whole feed).
1. Sort items by `date` descending.
1. Truncate to `--max-items`.
1. Build RSS 2.0 using `github.com/gorilla/feeds`.
1. Marshal to XML, return.

No caching in v1. Walking a few hundred files is cheap. If it becomes slow, add a simple mtime-based cache.

### `pulsar install` / `pulsar uninstall`

`install`:

1. Generate a LaunchAgent plist for `~/Library/LaunchAgents/io.arjen.pulsar.plist`.
1. The plist runs `pulsar serve` with the configured flags, captures stdout/stderr to `~/Library/Logs/pulsar.log`, and uses `KeepAlive`.
1. Run `launchctl load ~/Library/LaunchAgents/io.arjen.pulsar.plist`.
1. Print next steps for Tailscale Serve setup.

`uninstall`:

1. `launchctl unload …`
1. Remove the plist.

Plist template (key fields, embedded as a Go raw string):

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.arjen.pulsar</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/pulsar</string>
        <string>serve</string>
        <string>--base-url</string>
        <string>{{.BaseURL}}</string>
        <string>--port</string>
        <string>{{.Port}}</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>StandardOutPath</key><string>{{.LogPath}}</string>
    <key>StandardErrorPath</key><string>{{.LogPath}}</string>
</dict>
</plist>
```

`install` prompts for `--base-url` if not provided, with a suggested default derived from `tailscale status` if available.

## Feed Format

RSS 2.0. Channel structure:

```xml
<rss version="2.0">
  <channel>
    <title>Code Reviews</title>
    <link>https://mac.tailnet-name.ts.net/</link>
    <description>Locally generated code reviews</description>
    <generator>pulsar</generator>
    <lastBuildDate>...</lastBuildDate>
    <item>...</item>
    <item>...</item>
  </channel>
</rss>
```

Item structure:

```xml
<item>
  <title>🚫 fog#42 — Add caching layer</title>
  <link>https://mac.tailnet-name.ts.net/2026-05/2026-05-16-github-fog-pr42.html</link>
  <description>
    &lt;p&gt;&lt;strong&gt;Severity:&lt;/strong&gt; blocking&lt;/p&gt;
    &lt;p&gt;Cache invalidation logic has a race condition on concurrent writes.&lt;/p&gt;
    &lt;p&gt;&lt;a href="https://github.com/ArjenSchwarz/fog/pull/42"&gt;View PR on GitHub&lt;/a&gt;&lt;/p&gt;
  </description>
  <guid isPermaLink="false">https://github.com/ArjenSchwarz/fog#pr-42-1747396800</guid>
  <pubDate>Sat, 16 May 2026 12:00:00 +0000</pubDate>
</item>
```

Title format: `<emoji> <repo>#<pr-or-branch> — <metadata.title>`.

Severity emoji map:

|Severity       |Emoji|
|---------------|-----|
|`lgtm`         |✅    |
|`suggestions`  |💡    |
|`needs-changes`|⚠️    |
|`blocking`     |🚫    |

PR/branch identifier in title:

- PR: `<repo>#<pr>` → e.g. `fog#42`.
- Branch: `<repo>@<branch>` → e.g. `fog@feature-foo` (slugified for display consistency).

PR URL construction for the “View PR” link in the description:

- GitHub: `${repoUrl}/pull/${pr}`.
- GitLab: `${repoUrl}/-/merge_requests/${pr}`.
- Branch (no PR): omit the secondary link.
- Unknown host: omit the secondary link, log a warning.

Host detection by inspecting `repoUrl` hostname. Only `github.com` and `gitlab.com` are special-cased in v1. Self-hosted is a future concern.

## Tailscale Serve Setup

Documented in README, not in the binary. After `pulsar install`:

```
tailscale serve --bg --https=443 http://127.0.0.1:8765
```

This persists across reboots. Verify with `tailscale serve status`. Feed URL becomes `https://<mac-hostname>.<tailnet>.ts.net/feed.xml`.

## Agent Skill Contract

Documented in `docs/agent-contract.md` for reference by the code-review skill. Key points:

1. **No relative paths in the HTML.** The file will be moved out of its generation directory, so any relative reference (CSS, images, JS) will break. External assets are allowed, but must be referenced by absolute URL pointing to a stable, publicly reachable location (a versioned path on your own domain, a CDN, etc.). Inline assets (`<style>`, `data:` URIs) are also fine. Treat any external asset URL as a stable contract: old reviews will keep pointing at it forever, so versioned paths (`/v1/...`) are strongly preferred.
1. **Metadata block is mandatory** and must conform to the schema in this document. Validation is strict.
1. **The agent invokes `pulsar publish <path>` after writing the file.** The agent should treat non-zero exit as a hard error and surface the stderr message to the user.
1. **The agent should pick a sensible `title`** — typically the PR title or a one-line summary of what was reviewed.
1. **The agent should produce a meaningful `summary`** — 1-3 sentences highlighting the most important finding. This is what shows in the feed reader.
1. **Severity selection rubric:**
- `lgtm` — no concerns, code is acceptable as-is.
- `suggestions` — improvements possible but not necessary.
- `needs-changes` — issues that the author should address before merge.
- `blocking` — issues that would cause bugs, security problems, or significant design problems if merged.

## Testing Strategy

Unit tests (table-driven where possible):

- `naming_test.go`: slugify (various inputs, edge cases like consecutive separators, unicode), URL normalisation (SSH form, http→https, .git stripping, trailing slash), filename construction (PR and branch variants), GUID generation.
- `metadata_test.go`: metadata extraction from HTML (script block present, missing, malformed), validation (each required field missing, each enum violation, both pr+branch set, neither pr+branch set), date rewriting.
- `feed_test.go`: RSS marshalling (empty feed, single item, many items), title formatting, description HTML, GUID format, severity emoji mapping, host-specific PR URL construction.

Integration tests:

- `publish_test.go`: end-to-end publish flow using `t.TempDir()`. Cover: first publish (creates file in correct subdir), re-publish (deletes prior version, creates new), cross-month re-publish (file in old subdir is deleted, new file in new subdir is written), source file deletion, invalid metadata rejection.
- `serve_test.go`: spin up the server with `httptest.NewServer`, verify `/feed.xml` returns valid RSS with expected items, verify file serving and path traversal protection.

No mocking of the filesystem; use `t.TempDir()` throughout.

Coverage target: 80%+ on the pure-function packages (`naming`, `metadata`, `feed`), best-effort on the command flows.

## Implementation Order

Suggested sequence for a one-shot pass:

1. `go mod init`, add dependencies: `github.com/gorilla/feeds`, `github.com/spf13/cobra`, `github.com/spf13/viper`, and `golang.org/x/net/html` if used for metadata extraction.
1. `main.go` + `config.go` — cobra root command, viper setup, config struct, flag binding. Provides the skeleton for the rest.
1. `naming.go` + tests — pure functions, foundation for everything else.
1. `metadata.go` + tests — extraction, parsing, validation, date rewriting.
1. `feed.go` + tests — RSS generation given a slice of parsed metadata.
1. `publish.go` + tests — the publish flow assembled from the above.
1. `serve.go` + tests — HTTP server.
1. `install.go` — LaunchAgent management.
1. `README.md` + `docs/agent-contract.md`.

## Resolved Decisions

The following decisions were settled during planning. Documented here as part of the version-controlled design record.

1. **Binary name** — `pulsar`.
1. **Host segment in filename** — first label of the hostname only (`github`, not `github-com`). Revisit if self-hosted hosts become common.
1. **Severity marker in feed item title** — emoji prefix (✅ 💡 ⚠️ 🚫).
1. **Feed item description content** — rich format combining severity, summary, and a link to the PR/MR on the host.
1. **External assets in review HTML** — external absolute URLs allowed (no relative paths). Versioning of external assets is the agent operator’s responsibility; pulsar treats the HTML as opaque payload after metadata extraction.
1. **`install` subcommand** — included. Writes and loads the LaunchAgent plist.
1. **Feed item cap** — 200 most recent items in the feed; older items remain accessible at their URLs.
1. **Tests** — unit tests on pure functions plus integration tests for publish and serve flows.
1. **Configuration mechanism** — Cobra + Viper, with config file support available even if not used initially.

## Out of Scope (v1)

- Authentication beyond Tailscale tailnet membership.
- HTTPS termination in pulsar itself (Tailscale Serve handles this).
- Web UI for browsing reviews (the directory listing at `/` is the rough equivalent if implemented).
- Search across reviews.
- Tagging or filtering beyond severity.
- Multi-user / shared archives.
- Cleanup of branch reviews when a PR review is published for the same code (deliberately left as historical context).
- Atom feed (RSS 2.0 only).
- Pulsar rewriting or proxying external assets referenced by review HTML (the file is served as-is; the agent is responsible for using stable URLs).

## Future Considerations

- If reviews ever need to span more contexts than PR/branch (commit ranges, ad-hoc directory reviews), the schema can grow with a discriminated `ref` field while keeping `pr` and `branch` as backwards-compatible shortcuts.
- If feed generation becomes slow at scale, add mtime-based caching of parsed metadata.
- If multiple hosts (self-hosted GitLab, Bitbucket, etc.) become common, switch the host segment in filenames to the full hostname-as-slug for collision safety.
- If you want a public-facing version, swap `tailscale serve` for `tailscale funnel` — no changes to pulsar itself.