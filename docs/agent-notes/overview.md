# Pulsar — Overview

Single Go binary, flat package layout (everything in `package main` in the repo root). Two main subcommands plus install helpers.

## Files

- `main.go` — cobra root command, wires up subcommands.
- `config.go` — viper setup, `Config` struct, flag binding. `loadConfig()` collapses all four sources into a struct.
- `naming.go` — pure string helpers: URL normalisation, slugify, filename/glob/GUID construction.
- `metadata.go` — `Metadata` struct, regex-based extraction/replacement of the `<script id="review-meta">` block, validation.
- `feed.go` — RSS 2.0 generation via `github.com/gorilla/feeds`. `buildFeed(cfg, entries, now)` returns the XML string.
- `publish.go` — the `pulsar publish` subcommand. `publish(cfg, src, now)` is the testable core.
- `serve.go` — the `pulsar serve` subcommand. `newServeHandler(cfg)` returns the `http.Handler` for httptest.
- `install.go` — `install`/`uninstall` subcommands. Generates a LaunchAgent plist via `text/template` and `launchctl load`s it.

## Test layout

Tests live in the same package (`package main`). Helpers shared across tests:

- `intPtr(int) *int` (in `metadata_test.go`).
- `writeReviewHTML(t, dir, name, metaJSON)` (in `publish_test.go`) — used by `serve_test.go` to build fixtures.

`go test ./...` runs everything; coverage is ~52% overall, higher on the pure-function code.

## Gotchas

- The metadata block is matched by a regex that requires `id="review-meta"` in double quotes; other quoting/spacing won't match. The plan calls this out as acceptable since `golang.org/x/net/html` is overkill for a single tag.
- `feeds.RssFeed` doesn't get a `Generator` from the default constructor — we build the `RssFeed` ourselves via `(&feeds.Rss{Feed:f}).RssFeed()`, set `Generator = "pulsar"`, then wrap in `rssFeedXML` (which satisfies the `XmlFeed` interface) and call `feeds.ToXML`.
- Atomic file writes use `os.CreateTemp` in the destination directory + `os.Rename`. Same-filesystem rename is atomic on macOS/Linux.
- The PR vs branch distinction in `Metadata` uses `PR *int` so `nil` means "absent" (zero PR numbers would otherwise look the same as missing).
- Path traversal protection in `serveArchiveFile`: explicitly rejects `..` segments before `filepath.Clean`, then verifies the cleaned absolute path is still under the archive root.
- The GUID timestamp comes from the `date` field; re-publishing the same review produces a different GUID so feed readers treat it as a new item — this is intentional.

## Build / test commands

```sh
go build ./...
go test ./...
go vet ./...
```

No Makefile, no lint config beyond `go vet`.
