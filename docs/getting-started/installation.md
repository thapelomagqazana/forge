<!--
  Installing Forge
  ================
  Public guide. Every command in this document MUST work against a
  real release. A command that fails is a defect in this document.

  Normative references:
    - Verifying releases:   docs/reference/verifying-releases.md
    - Verifying binaries:   docs/guides/verifying-binaries.md
    - Release channels:     docs/reference/release-channels.md
    - Distribution ADR:     docs/decisions/ADR-049-distribution-channels.md
    - Security policy:      SECURITY.md

  Editing rules:
    1. Every channel's section MUST have an "Install", an
       "Uninstall", and a "Verify" subsection. A user who installs a
       package must know how to remove it, and how to confirm the
       install succeeded.
    2. Every command in a channel's section MUST work against a real
       release on a clean machine of the channel's target platform.
       A command that fails is a defect in this document.
    3. Every checksum, fingerprint, and version in this document MUST
       be a placeholder or be read from the release's artifacts. A
       hard-coded value is a defect: it goes stale.
    4. The channels MUST be listed in the order a user is likely to
       encounter them: direct first, then the package managers in
       alphabetical order. The order is the guide's contract; a
       future change that reorders it is a change to the reader's
       mental model.
    5. Every channel's "Verify" subsection MUST link to the
       release-verification guide's step that covers that channel's
       proof. An installation that cannot be verified is not an
       installation.
-->

# Installing Forge

Forge is distributed through six channels. Choose the one that fits
your environment. Every channel publishes the same binary; the
verification procedure is the same.

| Channel | Best for | Install command |
|---|---|---|
| [Direct download](#direct-download) | Any platform; a user who wants the archive | See §"Direct download" |
| [Homebrew](#homebrew) | macOS and Linux | `brew install forge-cli/tap/forge` |
| [Scoop](#scoop) | Windows | `scoop install forge` |
| [winget](#winget) | Windows | `winget install Forge.Forge` |
| [apt](#apt) | Debian and Ubuntu | See §"apt" |
| [Docker](#docker) | A container | `docker pull ghcr.io/forge-cli/forge:latest` |
| [`go install`](#go-install) | A Go developer | `go install github.com/forge-cli/forge/cmd/forge@latest` |

After installing, verify the binary with
[`docs/getting-started/verify-installation.md`](verify-installation.md).
The verification is per-channel; the page links to the right
procedure for the channel you chose.

---

## Direct download

The GitHub Releases page has the archive for every supported
platform:

```text
https://github.com/forge-cli/forge/releases/latest
```

Download the archive for your platform, extract it, and put the
`forge` binary on your `PATH`.

```sh
# Example: linux/amd64
curl --fail --silent --show-error --location \
  -o forge-linux-amd64.tar.gz \
  https://github.com/forge-cli/forge/releases/latest/download/forge-linux-amd64.tar.gz
tar -xzf forge-linux-amd64.tar.gz
sudo install -m 0755 forge /usr/local/bin/forge
```

The archive's name does not carry the version. The version is
embedded in the binary and appears in the release's URL path. The
name is `forge-<goos>-<goarch>.<ext>`, where `<ext>` is `tar.gz` for
Linux and macOS and `zip` for Windows.

### Verify

The archive's SHA-256 is in `SHA256SUMS` on the release page. The
per-archive `.sha256` file next to the archive is a convenience for
a user who downloads one archive.

```sh
# The combined list, if you downloaded every archive.
sha256sum -c SHA256SUMS

# The per-archive file, if you downloaded one.
sha256sum -c forge-linux-amd64.tar.gz.sha256
```

The full procedure — the checksum, the GPG signature, the cosign
signature, the SLSA provenance, and (on macOS and Windows) the
platform's own signature — is in
[`docs/reference/verifying-releases.md`](../reference/verifying-releases.md).
The direct-download channel is the one place all four proofs are
exercised; the package-manager channels below exercise a subset.

### Uninstall

Remove the binary:

```sh
sudo rm /usr/local/bin/forge
```

---

## Homebrew

The Homebrew tap is `forge-cli/homebrew-tap`. The formula is
auto-updated by the release pipeline; a new release is available
after the tap's `git push` and Homebrew's cache refresh.

```sh
brew install forge-cli/tap/forge
```

The tap is not in Homebrew's default set; the `forge-cli/tap/`
prefix tells Homebrew where to find the formula. A user who wants
to run `brew install forge` without the prefix can add the tap
first:

```sh
brew tap forge-cli/tap
brew install forge
```

The two forms install the same formula. The first form is the
tap's "qualified" name; the second is the "short" name after the
tap is added. A user who installs several formulae from the tap
should add it once and use the short names.

The tap's formula installs from source: Homebrew downloads the
release archive and extracts the binary. The formula's `install`
method is `bin.install "forge"`; there is no compilation step.

### Verify

Homebrew's `brew test` runs the formula's `test` block, which runs
`forge version --format=json` and asserts the output is valid JSON:

```sh
brew test forge-cli/tap/forge
```

The formula's SHA-256 fields name the release archive's digest,
which is the same digest in `SHA256SUMS`. Homebrew verifies the
digest at install time; a mismatch aborts the install.

The release's GPG signature over `SHA256SUMS` is the proof that the
digest came from Forge. Homebrew does not check the GPG signature;
a user who wants the full proof follows
[`docs/reference/verifying-releases.md`](../reference/verifying-releases.md)
§"Step 3" against the release page's `SHA256SUMS.asc`.

### Uninstall

```sh
brew uninstall forge
brew untap forge-cli/tap
```

The second command removes the tap; a user who installed other
formulae from it should not run it.

---

## Scoop

The Scoop bucket is `forge-cli/scoop-bucket`. The manifest is
auto-updated by the release pipeline.

```sh
scoop bucket add forge https://github.com/forge-cli/scoop-bucket
scoop install forge
```

A user who prefers to install without adding the bucket can use
Scoop's URL form:

```sh
scoop install https://raw.githubusercontent.com/forge-cli/scoop-bucket/main/bucket/forge.json
```

The two forms install the same manifest. The first form is the
bucket's "named" form; the second is the "URL" form. The first
form is recommended: `scoop update` refreshes the bucket and
picks up new releases; the URL form does not.

The manifest's `architecture` object names both Windows
architectures (`64bit` and `arm64`); Scoop selects the right one
for the host.

### Verify

Scoop's `scoop which forge` prints the binary's path:

```sh
scoop which forge
```

The manifest's `hash` fields name the release archive's SHA-256,
which is the same digest in `SHA256SUMS`. Scoop verifies the digest
at install time; a mismatch aborts the install.

The release's GPG signature over `SHA256SUMS` is the proof that the
digest came from Forge. Scoop does not check the GPG signature; a
user who wants the full proof follows
[`docs/reference/verifying-releases.md`](../reference/verifying-releases.md)
§"Step 3".

### Uninstall

```sh
scoop uninstall forge
scoop bucket rm forge
```

The second command removes the bucket; a user who installed other
packages from it should not run it.

---

## winget

The winget manifest is in `microsoft/winget-pkgs`. The release
pipeline opens a PR per stable release; a winget maintainer reviews
and merges it. A prerelease is not published to winget.

```sh
winget install Forge.Forge
```

The install is per-user by default. An administrator who wants a
machine-wide install uses:

```sh
winget install Forge.Forge --scope machine
```

winget's manifest uses the package identifier `Forge.Forge`: the
publisher `Forge` and the package `Forge`. The publisher's name is
the manifest's `Publisher` field; it is not the repository's owner.

### Verify

`winget show Forge.Forge` prints the manifest's metadata, including
the version:

```sh
winget show Forge.Forge
```

The manifest's `InstallerSha256` fields name the release archive's
SHA-256, which is the same digest in `SHA256SUMS`. winget verifies
the digest at install time.

The release's GPG signature over `SHA256SUMS` is the proof that the
digest came from Forge. winget does not check the GPG signature; a
user who wants the full proof follows
[`docs/reference/verifying-releases.md`](../reference/verifying-releases.md)
§"Step 3".

### Uninstall

```sh
winget uninstall Forge.Forge
```

---

## apt

The apt repository is at `apt.forge.dev`. It is signed with the
Forge release key; the key's fingerprint is in
[`SECURITY.md`](../../SECURITY.md) §"Release key".

The repository has two suites: `stable` and `testing`. A stable
release is published to `stable`; a prerelease is published to
`testing`. A nightly is not published to apt.

### Install

```sh
# 1. Import the release key.
curl --fail --silent --show-error --location \
  https://keys.openpgp.org/vks/v1/by-fingerprint/<FINGERPRINT> \
  | sudo gpg --dearmor -o /usr/share/keyrings/forge-archive-keyring.gpg

# 2. Add the stable repository.
echo "deb [signed-by=/usr/share/keyrings/forge-archive-keyring.gpg] https://apt.forge.dev stable main" \
  | sudo tee /etc/apt/sources.list.d/forge.list

# 3. Install.
sudo apt update
sudo apt install forge
```

A user who wants prereleases adds the `testing` suite alongside
`stable`:

```sh
echo "deb [signed-by=/usr/share/keyrings/forge-archive-keyring.gpg] https://apt.forge.dev testing main" \
  | sudo tee /etc/apt/sources.list.d/forge-testing.list
sudo apt update
```

A user who wants only prereleases uses the `testing` suite alone.
The two suites are separate `sources.list.d` files; removing one
does not affect the other.

The `<FINGERPRINT>` placeholder is the release key's fingerprint.
The guide writes it as a placeholder rather than a literal because
a key rotation changes it; the current fingerprint is in
[`SECURITY.md`](../../SECURITY.md).

### Verify

```sh
apt-cache policy forge
```

The output names the installed version and the candidate version.
The candidate is the highest version in the repository; the
installed is what `apt install` selected. A user who wants to
confirm the package's signature runs:

```sh
apt-get download forge
dpkg-sig --verify forge_*.deb
```

`dpkg-sig` verifies the `.deb`'s own signature, which the pipeline
adds before publishing. The repository's `Release` file's signature
is verified by apt itself during `apt update`; a bad signature
aborts the update and prints the reason.

The release's GPG signature over `SHA256SUMS` is the proof that the
archive's digest came from Forge. The apt repository's `Release`
file's signature is the proof that the repository's metadata came
from Forge. The two signatures are over different files; the
release key signs both.

### Uninstall

```sh
sudo apt remove forge
sudo rm /etc/apt/sources.list.d/forge.list
```

A user who added the `testing` suite also removes its file:

```sh
sudo rm /etc/apt/sources.list.d/forge-testing.list
```

---

## Docker

The image is `ghcr.io/forge-cli/forge`. It is multi-arch
(`linux/amd64` and `linux/arm64`); Docker selects the right image
for the host.

```sh
docker pull ghcr.io/forge-cli/forge:latest
docker run --rm ghcr.io/forge-cli/forge version
```

The image's `ENTRYPOINT` is `forge`; the `CMD` is `--help`. A user
who runs `docker run ghcr.io/forge-cli/forge` sees the help text.

The image is a `scratch` base with the binary and the CA bundle. A
user who needs a shell uses a different channel; the image has
none. The binary is static (`CGO_ENABLED=0`); the image has no
dynamic libraries.

### Tags

| Tag | Meaning |
|---|---|
| `latest` | The most recent stable release. |
| `stable` | The most recent stable release; same as `latest`. |
| `prerelease` | The most recent prerelease. |
| `vX.Y.Z` | The release's exact version. A user who wants a pin uses this tag. |

A user who wants a specific version uses the version tag:

```sh
docker pull ghcr.io/forge-cli/forge:v1.4.2
```

### Verify

The image's version label is `docker inspect`-able:

```sh
docker inspect ghcr.io/forge-cli/forge:latest \
  --format '{{ index .Config.Labels "org.opencontainers.image.version" }}'
```

The release's cosign signature over the archive is the proof that
the archive's bytes came from the release pipeline. The image
itself is not cosign-signed; the binary inside it is the archive's
binary, and the archive's cosign signature is the proof. A user who
wants the full proof follows
[`docs/reference/verifying-releases.md`](../reference/verifying-releases.md)
§"Step 4" against the archive, then confirms the image's binary
matches by running `forge version --format=json` inside the
container and comparing the commit to the archive's.

### Uninstall

Remove the image:

```sh
docker rmi ghcr.io/forge-cli/forge:latest
```

A user who pulled a specific version removes that tag:

```sh
docker rmi ghcr.io/forge-cli/forge:v1.4.2
```

---

## go install

The module path is `github.com/forge-cli/forge`. The binary's
package is `cmd/forge`.

```sh
go install github.com/forge-cli/forge/cmd/forge@latest
```

The binary is installed to `$(go env GOPATH)/bin/forge`. A user
whose `PATH` includes `$(go env GOPATH)/bin` runs `forge` directly.
A user who does not adds the directory:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

The `go install` command resolves the module path against the Go
module proxy. The proxy serves the module's source at the version
`@latest` selects; `go install` compiles it locally. The resulting
binary is built on the user's machine, not downloaded.

A user who wants a specific version uses the version tag:

```sh
go install github.com/forge-cli/forge/cmd/forge@v1.4.2
```

The version tag and the release tag are the same tag. A user who
wants the release's exact binary uses a package-manager channel or
the direct download; `go install` builds from source, and the
source at a tag may produce a binary that differs from the release
archive in build metadata (the archive's binary is built with
`-trimpath` and the release's `-ldflags`; a local `go install` uses
its own defaults).

### Verify

Before installing, a user can inspect the module path the proxy
serves:

```sh
go list -m github.com/forge-cli/forge@latest
```

The output is the module's path and version. A user who wants the
module's checksum runs:

```sh
go mod download -json github.com/forge-cli/forge@latest
```

The output includes a `Sum` field, which is the module's checksum
in the Go module proxy's checksum database. The checksum database
is the Go module system's own transparency log; a mismatch between
the proxy's module and the database's entry aborts the install.

The release's SLSA provenance is the proof that the release's
archive was built from a specific commit. The `go install` channel
builds from the module's source; the provenance does not cover a
user's local build. A user who wants the release's exact binary
uses a channel that publishes the archive.

### Uninstall

Remove the binary:

```sh
rm "$(go env GOPATH)/bin/forge"
```

The `go install` command does not maintain a cache of installed
binaries; removing the binary is the whole uninstall.

---

## Choosing a channel

The channels differ in three ways: what they install, how they
update, and what they verify.

| Channel | Installs | Updates | Verifies |
|---|---|---|---|
| Direct download | The release archive's binary | The user re-downloads | The checksum, the GPG signature, the cosign signature, the SLSA provenance |
| Homebrew | The release archive's binary | `brew upgrade` | The checksum (via the formula's `sha256`) |
| Scoop | The release archive's binary | `scoop update` | The checksum (via the manifest's `hash`) |
| winget | The release archive's binary | `winget upgrade` | The checksum (via the manifest's `InstallerSha256`) |
| apt | The `.deb` built from the archive's binary | `apt upgrade` | The repository's `Release` signature |
| Docker | An image containing the archive's binary | `docker pull` | The image's label |
| `go install` | A binary built from the module's source | `go install` again | The Go module proxy's checksum database |

A user who needs the strongest verification uses the direct
download and follows the full procedure. A user who wants the
easiest install uses a package manager; the package manager's
verification is the checksum, and the release's GPG signature is
available separately.

---

## Related documents

| Document | Role |
|---|---|
| [`docs/getting-started/verify-installation.md`](verify-installation.md) | Per-channel verification after install. |
| [`docs/reference/verifying-releases.md`](../reference/verifying-releases.md) | The release's four proofs. |
| [`docs/reference/release-channels.md`](../reference/release-channels.md) | The channels and the support window. |
| [`docs/guides/reproducible-builds.md`](../guides/reproducible-builds.md) | Rebuilding an archive from source. |
| [`SECURITY.md`](../../SECURITY.md) | The release key's fingerprint. |
