# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Pulsar is a single Go binary that turns a directory of locally-generated code review HTML files into an RSS feed served over Tailscale. Two main responsibilities: `publish` validates and archives a review HTML file; `serve` runs the HTTP server that exposes the archive and `/feed.xml`. `install` / `uninstall` manage a macOS LaunchAgent.

Full design lives in `docs/pulsar-implementation-plan.md`. The contract the review-generating agent must follow is in `docs/agent-contract.md`. Implementation-level notes (gotchas, test layout) are in `docs/agent-notes/overview.md` — read that before non-trivial changes.

## Commands

Use the Makefile, not raw `go` commands:

- `make build` — build with version ldflags injected
- `make test` — unit tests
- `make test-integration` — sets `INTEGRATION=1`; integration tests skip themselves unless that env var is set
- `make test-coverage` — generates `coverage.html`
- `make lint` — runs `golangci-lint` (config requires v2; the binary fails loudly if v1 is on PATH)
- `make check` — `fmt` + `vet` + `lint` + `test` (the full validation suite to run before pushing)
- `make build-release VERSION=x.y.z` — release build; refuses to run with `VERSION=dev`

Run a single test: `go test -run TestName ./...` (everything is in `package main` at the repo root, so there are no subpackages to target).

## Architecture

Flat layout — every `.go` file is in `package main` at the repo root. The split is functional, not by package:

- `main.go` — cobra root, wires subcommands, exposes `Version`/`BuildTime`/`GitCommit` set via ldflags.
- `config.go` — viper setup. Resolution order: flags → `PULSAR_*` env vars → `$HOME/.config/pulsar/config.yaml` → built-in defaults. `loadConfig()` is the single read point.
- `metadata.go` — extraction, validation, and replacement of the `<script id="review-meta">` JSON block. Uses a regex (not an HTML parser) and requires double-quoted `id="review-meta"`.
- `naming.go` — pure string helpers: SSH→HTTPS URL normalisation, slugify, canonical filename / glob / GUID construction. Stateless and the most heavily tested.
- `feed.go` — RSS 2.0 generation via `github.com/gorilla/feeds`. `buildFeed(cfg, entries, now)` is the testable core.
- `publish.go` — `publish(cfg, src, now)` is the testable core for the `publish` subcommand. Writes atomically via temp file + rename in the destination directory.
- `serve.go` — `newServeHandler(cfg)` returns the `http.Handler` for `httptest`. Handler scans the archive on every `/feed.xml` request (no in-memory cache).
- `install.go` — generates a LaunchAgent plist via `text/template`, then `launchctl load`s it. Auto-detects the tailnet hostname by parsing `tailscale status`.

Subcommand entry points (`newPublishCmd`, `newServeCmd`, `newInstallCmd`, `newUninstallCmd`) are thin wrappers that call the testable pure functions. Keep this separation — tests target the pure functions, not the cobra commands.

## Non-obvious behaviour

- `Metadata.PR` is `*int` so `nil` means "absent". Exactly one of `pr` or `branch` must be set; `validate()` enforces this.
- Re-publishing the same review produces a **different GUID** (timestamp baked into the GUID), so feed readers treat it as a new item. This is intentional.
- `feeds.RssFeed` doesn't accept a `Generator` from the default constructor. We build the `RssFeed` ourselves, set `Generator = "pulsar"` and `LastBuildDate`, then wrap in `rssFeedXML` so `feeds.ToXML` doesn't discard those fields. Don't "simplify" this back to the default constructor.
- `serveArchiveFile` uses `filepath.IsLocal` for traversal protection and refuses symlinks via `os.Lstat`. Both checks are load-bearing — don't remove either.
- Publish writes via `os.CreateTemp` + `os.Rename` in the **same directory** so the rename is atomic on the same filesystem. Don't move the temp to `/tmp`.

## Project conventions

- `package main` everywhere; no subpackages. New files go in the repo root.
- Tests live alongside source (`*_test.go`) in the same package, so they can access unexported helpers like `intPtr` and `writeReviewHTML`.
- Integration tests gate themselves with `os.Getenv("INTEGRATION")` rather than build tags.
- `.golangci.yml` is `version: "2"` and disables `errcheck` and `unused`. Don't add `// nolint` comments — fix the underlying issue or update the config.
- Go language rules: prefer `any` over `interface{}`, `for range n`, `slices.Contains`, `strings.SplitSeq`/`FieldsSeq` in range loops, `t.Context()` in tests. The repo already follows these — keep new code consistent.
