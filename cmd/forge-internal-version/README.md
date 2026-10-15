# cmd/forge-internal-version

Internal build tool. **Not distributed to users.**

Reads `docs/reference/version-metadata.yaml` and exposes the Forge
versioning and release-channel scheme to build and CI tooling:

- `verify` — validate `version-metadata.yaml` against
  `schemas/version-metadata.schema.json`, and cross-check every
  channel's examples against `internal/version.ChannelOf`.
- `channel <tag>` — print the release channel of a git tag, or exit
  non-zero if the tag matches no channel. Used by
  `.github/workflows/release.yml` to route a tag.
- `tag-table` — print the tag-to-channel table for a fixed corpus of
  example tags.
- `fmt` — sort `channels` and `prerelease_identifiers` in place.
  Idempotent.
- `notes check <path>` — validate a filled-in release notes file
  against the required headings for its channel and bump class.

Invoked by `Taskfile.yml` targets `version:verify`, `version:tag-table`,
`version:fmt`, and by the release workflow.

Kept in a separate `cmd/` directory so it does not link into the
user-facing `cmd/forge` binary and does not appear in release artifacts.
