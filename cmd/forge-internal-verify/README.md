# cmd/forge-internal-verify

Internal build tool. **Not distributed to users.**

Runs the ten-check binary verification suite against the compiled
artifacts in a `dist/` directory.

- `forge-internal-verify` — verify every Tier 1/2 platform
- `forge-internal-verify --dist=<dir>` — verify a different directory
- `forge-internal-verify --platform=linux/amd64` — verify one platform
- `forge-internal-verify --verbose` — print per-check timings

Invoked by the Makefile's `release-dry-run` target, by `Taskfile.yml`'s
`release:verify-binaries` target, and by `.github/workflows/verify-binaries.yml`.

Kept in a separate `cmd/` directory so it does not link into the
user-facing `cmd/forge` binary and does not collide with the
`scripts/build.go` driver, which is `package main` in `scripts/`.
