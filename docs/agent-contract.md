# Agent Contract

This describes what a code-review agent must produce so that `pulsar publish` accepts the file.

## Workflow

1. Generate a self-contained review HTML file in any directory.
2. Run `pulsar publish <path>`.
3. Treat a non-zero exit code as a hard error. The stderr message is meant for the user.

`pulsar publish`:

- Reads the file.
- Extracts and validates the metadata block.
- Normalises `repoUrl` (SSH → HTTPS, strips `.git`, strips trailing `/`).
- Stamps `date` to the current time (RFC 3339).
- Writes the file into `<archive>/YYYY-MM/<canonical-name>.html`.
- Deletes any prior version with the same PR (or same branch).
- Deletes the source file.

The agent does **not** need write access to the archive directory.

## File requirements

### No relative asset paths

The file is moved out of its generation directory, so any relative reference will break. Allowed:

- Inline CSS (`<style>`).
- Inline JS (`<script>`).
- `data:` URIs for images.
- Absolute URLs (`https://…`) pointing to stable, publicly reachable assets.

Versioned asset paths (`/v1/style.css`) are strongly preferred — old reviews keep pointing at the URL forever.

### Mandatory metadata block

Every review must contain exactly one `<script type="application/json" id="review-meta">` tag in `<head>`.

```html
<script type="application/json" id="review-meta">
{
  "title": "Add caching layer",
  "repoUrl": "https://github.com/ArjenSchwarz/fog",
  "pr": 42,
  "severity": "blocking",
  "summary": "Cache invalidation has a race condition on concurrent writes."
}
</script>
```

## Schema

| Field      | Type    | Required | Notes |
|------------|---------|----------|-------|
| `title`    | string  | yes      | Human-readable review title (typically the PR title). |
| `repoUrl`  | string  | yes      | SSH or HTTPS git URL; will be normalised. |
| `pr`       | integer | one of   | PR/MR number. Exactly one of `pr` or `branch` must be set. |
| `branch`   | string  | one of   | Branch name. Exactly one of `pr` or `branch` must be set. |
| `severity` | enum    | yes      | `lgtm` \| `suggestions` \| `needs-changes` \| `blocking`. |
| `summary`  | string  | yes      | 1–3 sentences highlighting the main finding. Shown in the feed reader. |
| `date`     | string  | ignored  | Stamped by `publish` regardless of input. |

## Severity rubric

- `lgtm` — no concerns, code is acceptable as-is.
- `suggestions` — improvements possible but not necessary.
- `needs-changes` — issues the author should address before merge.
- `blocking` — bugs, security problems, or significant design issues that would be wrong to merge.

## Writing a good summary

The `summary` is the feed item description — the part the user reads in their feed reader to decide whether to open the review. Aim for one short paragraph: lead with the most important finding, not with what was reviewed. Bad: "I reviewed the cache module and found …". Good: "Cache invalidation has a race on concurrent writes; everything else looks fine."

## Error handling

`pulsar publish` writes a single-line error to stderr and exits non-zero. Surface that message verbatim to the user.

Common errors:

```
pulsar: missing required field 'title' in metadata
pulsar: severity must be one of: lgtm, suggestions, needs-changes, blocking
pulsar: exactly one of 'pr' or 'branch' must be set
pulsar: repoUrl must have a host and a path segment
pulsar: file not found: /path/to/review.html
pulsar: no metadata block found (looking for <script id="review-meta">)
```
