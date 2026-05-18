# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [1.0.0] - 2026-05-18

Initial release. Pulsar is a small Go binary that turns a directory of locally-generated code review HTML files into an RSS feed, designed to be served over Tailscale.

### Added

- `pulsar publish <file>`: validates the embedded JSON metadata block in a review HTML file, normalises `repoUrl` (SSH → HTTPS, strips `.git` / trailing slash), stamps the `date`, moves the file into `<archive>/YYYY-MM/`, and removes any prior version for the same PR or branch. Writes are atomic (temp file + rename in the destination directory).
- `pulsar serve`: HTTP server bound to `127.0.0.1` that exposes the archive as static files, a minimal HTML index at `/`, and an RSS 2.0 feed at `/feed.xml` (via `gorilla/feeds`). Path-traversal protection and symlink rejection on archive paths.
- `pulsar install` / `pulsar uninstall`: manage a macOS LaunchAgent (`io.arjen.pulsar`) that runs `serve` on login. `install` auto-detects a Tailscale-based base URL via `tailscale status` when one isn't supplied.
- Configuration via flags, `PULSAR_*` environment variables, and `$HOME/.config/pulsar/config.yaml`, resolved in that order.
- Pre-built binaries published to GitHub Releases for `linux`, `darwin`, and `windows` on both `amd64` and `arm64`. Also installable via `go install github.com/ArjenSchwarz/pulsar@latest`.
- Documentation: `README.md`, `docs/agent-contract.md` (the metadata schema and validation rules consumed by review-generating agents), and `docs/pulsar-implementation-plan.md`.
