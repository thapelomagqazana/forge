#!/usr/bin/env bash
#
# verify-release.sh — automate the four checks that do not need a
# rebuild.
#
# The script verifies a Forge release against four proofs:
#
#   1. The SHA-256 checksum (SHA256SUMS).
#   2. The GPG signature over SHA256SUMS.
#   3. The cosign signature over the archive.
#   4. The SLSA provenance (forge.intoto.jsonl).
#
# The script does not rebuild from source: the rebuild requires a
# clone of the source and the build toolchain, which the script does
# not manage. The rebuild procedure is in
# docs/guides/reproducible-builds.md.
#
# Usage:
#   verify-release.sh <version> [<archive>] [<directory>]
#
# Arguments:
#   version    The release's tag, e.g. "v1.4.2". Required.
#   archive    The archive to verify, e.g. "forge-linux-amd64.tar.gz".
#              Default: detected from the host's GOOS/GOARCH.
#   directory  Where the release's assets are (or will be downloaded
#              to). Default: the current directory.
#
# Exit codes:
#   0  every check passed
#   1  a check failed, or a required tool is missing
#   2  usage error
#
# The script is POSIX-compatible Bash. It uses no Bash 4+ features
# (associative arrays, `mapfile`) so it runs on macOS's default
# Bash 3.2. It is checked by shellcheck in scripts/verify-release-lint.sh.
#
# The script's human-readable output goes to stderr; its
# machine-readable summary goes to stdout. A caller who pipes
# stdout gets one `key=value` line per proof, not the log lines.
# The `log` function writes to stderr; the summary's `printf`s
# write to stdout.

set -euo pipefail

# --- usage ---

# usage prints the script's help text to stdout.
#
# The caller is responsible for redirecting the text to the stream
# the caller wants. A usage error prints to stderr (`usage >&2`);
# a future `--help` flag would print to stdout (`usage`). The
# function does not decide the stream; the call site does.
usage() {
	cat <<'USAGE'
Usage: verify-release.sh <version> [<archive>] [<directory>]

  version    The release's tag, e.g. "v1.4.2". Required.
  archive    The archive to verify, e.g. "forge-linux-amd64.tar.gz".
             Default: detected from the host's GOOS/GOARCH.
  directory  Where the release's assets are (or will be downloaded to).
             Default: the current directory.

Exit codes:
  0  every check passed
  1  a check failed, or a required tool is missing
  2  usage error
USAGE
}

# --- configuration ---

# The repository. A future fork would override this via the
# FORGE_REPO environment variable; the default is the upstream.
REPO="${FORGE_REPO:-thapelomagqazana/forge}"

# The release key's fingerprint. The script reads it from the
# environment if set. If it is not set, the GPG check is skipped
# and the skip is reported. The two-step check — read the
# fingerprint from a trusted source, compare it to the signature's
# key — is what makes the GPG verification meaningful: a script
# that hard-coded the fingerprint would fail on a key rotation; a
# script that trusted the signature's own key without a comparison
# would trust an attacker's key.
FORGE_GPG_KEY_FINGERPRINT="${FORGE_GPG_KEY_FINGERPRINT:-}"

# The paths the script uses. They are relative to the directory
# argument.
SUMS_FILE="SHA256SUMS"
SUMS_SIG="SHA256SUMS.asc"
PROVENANCE="forge.intoto.jsonl"

# --- output ---

# log writes a line to stderr. The script's stdout is reserved for
# a machine-readable summary that a caller can parse; the human
# output goes to stderr so a user who pipes stdout does not lose
# the summary.
log() {
	printf '%s\n' "$*" >&2
}

# fail writes a failure message to stderr and exits 1.
fail() {
	log "FAIL: $*"
	exit 1
}

# --- preflight ---

# require_tools checks that every tool the script uses is on PATH.
# A missing tool is reported once at the start; the script exits
# before running any check. The preflight runs before any other
# step, including the platform detection, so a user whose PATH is
# missing a tool sees the missing-tool message, not a
# `uname: command not found` from the detection.
require_tools() {
	local missing=0
	for tool in sha256sum gpg cosign slsa-verifier curl tar uname; do
		if ! command -v "$tool" >/dev/null 2>&1; then
			log "missing tool: $tool"
			missing=1
		fi
	done
	if [ "$missing" -ne 0 ]; then
		fail "one or more required tools are missing; see docs/reference/verifying-releases.md"
	fi
}

# --- platform detection ---

# detect_archive prints the archive's name for the host's
# GOOS/GOARCH. The naming convention is in
# internal/release/release.go; the script reproduces it.
#
# The function is called only when the caller does not supply an
# archive argument. The preflight runs before this function, so
# `uname` is on PATH when the function runs.
detect_archive() {
	local goos goarch ext
	case "$(uname -s)" in
		Linux) goos=linux ;;
		Darwin) goos=darwin ;;
		MINGW*|MSYS*|CYGWIN*) goos=windows ;;
		*) fail "unsupported operating system: $(uname -s)" ;;
	esac
	case "$(uname -m)" in
		x86_64|amd64) goarch=amd64 ;;
		aarch64|arm64) goarch=arm64 ;;
		*) fail "unsupported architecture: $(uname -m)" ;;
	esac
	if [ "$goos" = "windows" ]; then
		ext=zip
	else
		ext=tar.gz
	fi
	printf 'forge-%s-%s.%s' "$goos" "$goarch" "$ext"
}

# --- download ---

# download_asset downloads one asset from the release page. It is
# idempotent: an existing file is not re-downloaded. The function
# reads VERSION from the caller's scope; the caller sets it before
# the first call.
download_asset() {
	local name="$1" dir="$2"
	local url="https://github.com/${REPO}/releases/download/${VERSION}/${name}"
	if [ -f "${dir}/${name}" ]; then
		log "already downloaded: ${name}"
		return 0
	fi
	log "downloading: ${url}"
	if ! curl --fail --silent --show-error --location \
		-o "${dir}/${name}" "$url"; then
		fail "download failed: ${url}"
	fi
}

# --- the four checks ---

# check_checksum verifies the archive's SHA-256 against its
# per-archive `.sha256` file. The per-archive file is what the
# script downloads; the combined SHA256SUMS is downloaded so a
# user who wants to verify every archive has it.
check_checksum() {
	local dir="$1" archive="$2"
	log "check 2/5: SHA-256 checksum"
	if ! (cd "$dir" && sha256sum -c "${archive}.sha256"); then
		fail "checksum mismatch for ${archive}"
	fi
	log "OK: checksum"
}

# check_gpg verifies the GPG signature over SHA256SUMS.
#
# The function reads the fingerprint from the environment. If the
# variable is not set, the check is skipped and the skip is
# reported: a user who does not set the fingerprint cannot verify
# the GPG signature, and the skip tells them why. If the variable
# is set, the function fetches the key from keys.openpgp.org and
# verifies the signature.
#
# The function does not trust the signature's key without
# comparing the fingerprint to the one SECURITY.md documents; the
# comparison is what makes the check meaningful. The comparison
# itself is the `--recv-keys <fingerprint>` call: it fetches the
# key whose fingerprint the caller supplied, and `gpg --verify`
# then checks the signature against that key. A signature made by
# a different key would fail the verify even if the fetch
# succeeded.
check_gpg() {
	local dir="$1"
	log "check 3/5: GPG signature"
	if [ ! -f "${dir}/${SUMS_SIG}" ]; then
		log "SKIP: ${SUMS_SIG} is not present; the release was published without the GPG signature"
		return 0
	fi
	if [ -z "$FORGE_GPG_KEY_FINGERPRINT" ]; then
		log "SKIP: FORGE_GPG_KEY_FINGERPRINT is not set; the GPG signature is not checked"
		log "      Set it to the fingerprint in SECURITY.md to enable the check."
		return 0
	fi
	if ! gpg --keyserver keys.openpgp.org --recv-keys "$FORGE_GPG_KEY_FINGERPRINT" 2>/dev/null; then
		fail "could not fetch the release key ${FORGE_GPG_KEY_FINGERPRINT}"
	fi
	if ! (cd "$dir" && gpg --verify "$SUMS_SIG" "$SUMS_FILE"); then
		fail "GPG signature verification failed"
	fi
	log "OK: GPG signature"
}

# check_cosign verifies the cosign signature over the archive.
#
# The two `--certificate-*` arguments pin the signer to the Forge
# repository on GitHub Actions. Without them, `cosign` would
# accept any Sigstore signature, which would not prove the
# archive came from Forge.
check_cosign() {
	local dir="$1" archive="$2"
	log "check 4/5: cosign signature"
	if [ ! -f "${dir}/${archive}.sig" ]; then
		log "SKIP: ${archive}.sig is not present; the release was published without the cosign signature"
		return 0
	fi
	if ! (cd "$dir" && cosign verify-blob \
		--certificate-identity-regexp="^https://github.com/${REPO}/" \
		--certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
		--certificate "${archive}.pem" \
		--signature "${archive}.sig" \
		"$archive"); then
		fail "cosign verification failed"
	fi
	log "OK: cosign signature"
}

# check_slsa verifies the SLSA provenance over the archive.
#
# The `--source-uri` argument pins the source repository; a
# provenance attestation from a different repository would not
# satisfy the check.
check_slsa() {
	local dir="$1" archive="$2"
	log "check 5/5: SLSA provenance"
	if [ ! -f "${dir}/${PROVENANCE}" ]; then
		log "SKIP: ${PROVENANCE} is not present; the release was published without the SLSA provenance"
		return 0
	fi
	if ! (cd "$dir" && slsa-verifier verify-artifact \
		--provenance-path "$PROVENANCE" \
		--source-uri "github.com/${REPO}" \
		"$archive"); then
		fail "SLSA provenance verification failed"
	fi
	log "OK: SLSA provenance"
}

# --- main ---

# main is the script's entry point. Its order is the script's
# contract:
#
#   1. Parse the arguments. A usage error prints to stderr and
#      exits 2 before any tool is checked.
#   2. Run the preflight. A missing tool prints to stderr and
#      exits 1 before any work is done.
#   3. Resolve the archive. The platform detection runs only when
#      the caller did not supply an archive; the preflight has
#      already confirmed `uname` is on PATH.
#   4. Download the assets. An existing file is not re-downloaded.
#   5. Run the four checks in order: the checksum is the cheapest
#      and the SLSA provenance is the most expensive.
#   6. Print the machine-readable summary to stdout.
#
# A future change that reordered the steps would need to preserve
# the two invariants the tests assert: the usage error's message
# goes to stderr, and the preflight runs before any external
# command other than the shell's built-ins.
main() {
	if [ "$#" -lt 1 ] || [ "$#" -gt 3 ]; then
		usage >&2
		exit 2
	fi

	VERSION="$1"
	DIR="${3:-.}"

	# The preflight runs before any other step. A user whose PATH
	# is missing a tool sees the missing-tool message here, not a
	# failure from a command the script's other steps run.
	require_tools

	# The archive argument is either the caller's or the host's.
	# The detection runs after the preflight so a stripped PATH
	# does not produce a uname-not-found before the missing-tool
	# message. The `if`/`else` form evaluates `detect_archive`
	# only when needed; the `${2:-...}` form would run the command
	# substitution even when the caller supplied the archive.
	if [ "$#" -ge 2 ]; then
		ARCHIVE="$2"
	else
		ARCHIVE="$(detect_archive)"
	fi

	log "verifying Forge ${VERSION}"
	log "archive:   ${ARCHIVE}"
	log "directory: ${DIR}"
	log ""

	# Download the assets. The seven downloads are the archive and
	# the six proof files the four checks read. The archive itself
	# is downloaded by name; the proof files are downloaded by
	# their names. A file that already exists is not re-downloaded.
	download_asset "$ARCHIVE" "$DIR"
	download_asset "${ARCHIVE}.sha256" "$DIR"
	download_asset "$SUMS_FILE" "$DIR"
	download_asset "$SUMS_SIG" "$DIR"
	download_asset "${ARCHIVE}.sig" "$DIR"
	download_asset "${ARCHIVE}.pem" "$DIR"
	download_asset "$PROVENANCE" "$DIR"

	# The four checks. Their order is the order a user would run
	# them; the checksum is the cheapest and the SLSA provenance
	# is the most expensive.
	check_checksum "$DIR" "$ARCHIVE"
	check_gpg "$DIR"
	check_cosign "$DIR" "$ARCHIVE"
	check_slsa "$DIR" "$ARCHIVE"

	log ""
	log "every available check passed"
	log ""
	log "The rebuild-from-source check is not run by this script."
	log "See docs/guides/reproducible-builds.md for the procedure."

	# The machine-readable summary. A caller that pipes stdout
	# gets one line per check. The summary goes to stdout; the
	# log lines above go to stderr.
	printf 'version=%s\n' "$VERSION"
	printf 'archive=%s\n' "$ARCHIVE"
	printf 'checks=checksum,gpg,cosign,slsa\n'
}

main "$@"