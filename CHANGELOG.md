# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Initial implementation of the `pulsar` binary with `publish`, `serve`, `install`, and `uninstall` subcommands.
- `publish`: validates the embedded JSON metadata block in a review HTML file, normalises `repoUrl` (SSH → HTTPS, strips `.git`/trailing slash), stamps the `date`, moves the file into `<archive>/YYYY-MM/`, and removes any prior version for the same PR or branch. Atomic temp-file + rename writes.
- `serve`: HTTP server bound to `127.0.0.1` exposing `/feed.xml` (RSS 2.0 via `gorilla/feeds`), a minimal HTML index at `/`, and static review files with path-traversal protection.
- `install` / `uninstall`: generate, load, and remove a macOS LaunchAgent plist (`io.arjen.pulsar`). `install` auto-detects a Tailscale-based base URL via `tailscale status` when one isn't supplied.
- Cobra + Viper configuration with flag, environment variable (`PULSAR_*`), and `$HOME/.config/pulsar/config.yaml` support.
- Unit tests for `naming`, `metadata`, and `feed` plus integration tests for the `publish` flow and the `serve` HTTP handler.
- Documentation: `README.md`, `docs/agent-contract.md` (schema and validation rules consumed by the review-generating agent), and `docs/agent-notes/overview.md` for future agent sessions.
