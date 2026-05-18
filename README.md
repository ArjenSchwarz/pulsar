# Pulsar

A small Go binary that turns a directory of locally-generated code review HTML files into an RSS feed served over Tailscale.

## What it does

- **`pulsar publish <file>`** — validates a review HTML file, canonicalises the filename, drops it into the archive, and removes the previous version for the same PR/branch.
- **`pulsar serve`** — long-running HTTP server that exposes the archive as static files and an RSS 2.0 feed at `/feed.xml`. Bound to `127.0.0.1`; expose it on your tailnet with `tailscale serve`.
- **`pulsar install` / `uninstall`** — manages a macOS LaunchAgent that runs `serve` on login.

See `docs/pulsar-implementation-plan.md` for the full design and `docs/agent-contract.md` for the contract the review-generating agent must follow.

## Install

Download a pre-built binary from the [releases page](https://github.com/ArjenSchwarz/pulsar/releases). Archives are published for `linux`, `darwin`, and `windows` on both `amd64` and `arm64`. The `install` / `uninstall` subcommands only work on macOS (they manage a LaunchAgent), but the rest of the binary runs anywhere Go does.

Or install from source:

```
go install github.com/ArjenSchwarz/pulsar@latest
```

Or clone and `go build`.

## Configuration

Resolution order (highest priority first):

1. Command-line flags.
2. Environment variables (`PULSAR_DIR`, `PULSAR_PORT`, `PULSAR_BASE_URL`, `PULSAR_TITLE`, `PULSAR_DESCRIPTION`, `PULSAR_MAX_ITEMS`).
3. `$HOME/.config/pulsar/config.yaml` (optional).
4. Built-in defaults.

Example config file:

```yaml
dir: /Users/arjen/CodeReviews
port: 8765
base_url: https://mac.tailnet-name.ts.net
title: Code Reviews
description: Locally generated code reviews
max_items: 200
```

## Quick start

```sh
# 1. build
go build -o ~/bin/pulsar .

# 2. install the LaunchAgent (auto-detects base URL from `tailscale status` if available)
pulsar install --base-url https://mac.tailnet-name.ts.net

# 3. expose it on the tailnet
tailscale serve --bg --https=443 http://127.0.0.1:8765
tailscale serve status

# 4. subscribe to https://mac.tailnet-name.ts.net/feed.xml in your feed reader
```

## Publishing a review

The review-generating agent writes an HTML file containing a `<script type="application/json" id="review-meta">` metadata block, then runs:

```sh
pulsar publish path/to/review.html
```

`publish` validates the metadata, normalises `repoUrl`, stamps `date` to now, moves the file into `~/CodeReviews/YYYY-MM/`, and removes any prior version. Non-zero exit on any validation failure.

Schema and validation rules are documented in `docs/agent-contract.md`.

## Development

```sh
make build      # build with version ldflags injected
make test       # run tests
make check      # fmt + vet + lint + test
```

`make help` lists everything else (release builds, coverage, benchmarks, dependency updates).

## Tailscale Funnel

To make the feed publicly accessible (no tailnet membership required), swap `tailscale serve` for `tailscale funnel`. No changes to pulsar.
