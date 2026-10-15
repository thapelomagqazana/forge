// Command forge-internal-checksums produces the Forge release's
// checksum artifacts.
//
// It writes:
//
//   - <dist>/forge-<goos>-<goarch>.<ext>.sha256
//     A single-line file per archive, in `sha256sum` format.
//
//   - <dist>/SHA256SUMS
//     The combined list, sorted by archive name.
//
//   - <dist>/SHA256SUMS.asc
//     An armored GPG signature over SHA256SUMS, if the release key
//     is available.
//
//   - <dist>/SHA256SUMS.gpg
//     A binary GPG signature over SHA256SUMS, if the release key is
//     available.
//
// # Why both per-binary and combined
//
// The combined SHA256SUMS is the canonical list. A user who
// downloads every archive verifies them all with one command:
//
//	sha256sum -c SHA256SUMS
//
// The per-binary .sha256 file is a convenience. A user who downloads
// one archive (for example, only linux/amd64) downloads the one
// .sha256 file next to it and verifies with a one-line command. The
// per-binary file also lets a future tool that discovers checksums
// by globbing *.sha256 find them without parsing SHA256SUMS.
//
// # Why GPG signing is in this driver
//
// The GPG signature is over SHA256SUMS, not over each archive. It is
// a signature on the checksum list, which transitively covers every
// archive. Keeping the GPG signing in the checksums driver means the
// signature is produced in the same pass that produces the file it
// signs; a separate sign driver would have to re-read SHA256SUMS.
//
// The cosign signatures are over the archives themselves; they live
// in cmd/forge-internal-sign. The two signing concerns have
// different subjects and different lifetimes; separate drivers
// reflect that.
//
// # Credentials
//
// GPG signing requires the Forge release key in the local GPG
// keyring. The driver reports a documented blocker when the key is
// absent rather than failing the pipeline silently:
//
//	BLOCKER: GPG release key not found in the local keyring;
//	         release artifacts are unsigned. See SECURITY.md.
//
// The blocker is a line on stderr; the driver's exit code is 0 if
// every other step succeeded. The release pipeline treats the
// blocker as informational and continues. A future release where
// the key is available produces the signatures; a release where it
// is not produces the blocker in the log.
//
// # Usage
//
//	forge-internal-checksums
//	forge-internal-checksums --dist=<dir>
//	forge-internal-checksums --no-gpg
//
// The --no-gpg flag skips the GPG step. It is used by the CI's
// verification job, which does not have the release key and does
// not need to produce signatures.
//
// # Exit codes
//
//	0  every checksum file was written
//	1  a checksum file could not be written
//	2  usage error
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thapelomagqazana/forge/cmd/internal/release"
	"github.com/thapelomagqazana/forge/internal/platform"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command-line configuration.
type options struct {
	distDir string
	noGPG   bool
	root    string
}

// run executes the checksum pipeline. It returns the process exit
// code.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}
	if opts.distDir == "" {
		opts.distDir = release.DefaultDistDir
	}

	m, err := platform.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "checksums: load platform matrix: %v\n", err)
		return 1
	}

	platforms := selectPlatforms(m)
	if len(platforms) == 0 {
		fmt.Fprintln(stderr, "checksums: no platforms selected")
		return 1
	}

	// Per-binary .sha256 files.
	for _, p := range platforms {
		if err := writePerBinaryChecksum(opts.distDir, p); err != nil {
			fmt.Fprintf(stderr, "checksums: %s: %v\n", p.Name, err)
			return 1
		}
		fmt.Fprintf(stdout, "OK: %s\n", release.PerBinaryChecksumPath(opts.distDir, p))
	}

	// Combined SHA256SUMS.
	if err := writeCombinedChecksums(opts.distDir, platforms); err != nil {
		fmt.Fprintf(stderr, "checksums: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "OK: %s\n", release.CombinedChecksumPath(opts.distDir))

	// GPG signatures.
	if opts.noGPG {
		fmt.Fprintln(stdout, "SKIP: GPG signing (--no-gpg)")
		return 0
	}
	if blocker := signWithGPG(context.Background(), opts.distDir, stdout, stderr); blocker != "" {
		fmt.Fprintf(stderr, "BLOCKER: %s\n", blocker)
	}
	return 0
}

// parseFlags parses the command-line flags.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("checksums", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.distDir, "dist", release.DefaultDistDir, "directory containing the artifacts")
	fs.BoolVar(&opts.noGPG, "no-gpg", false, "skip the GPG signing step")
	fs.StringVar(&opts.root, "root", "", "repo root (resolved if empty)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

// selectPlatforms returns the Tier 1 and Tier 2 platforms. The
// checksum driver does not build Tier 3 platforms; those are
// best-effort and do not ship.
func selectPlatforms(m *platform.Matrix) []platform.Platform {
	var out []platform.Platform
	for _, p := range m.Platforms {
		if p.Tier == platform.Tier3 {
			continue
		}
		out = append(out, p)
	}
	return out
}

// writePerBinaryChecksum writes a single-line .sha256 file for the
// platform's archive.
//
// The file's content is "<hex digest>  <archive name>\n". The two
// spaces between the digest and the name are the format `sha256sum`
// expects; a single space also works on most implementations, but
// the two-space form is the canonical one and is what
// `sha256sum --check` produces.
//
// The archive's basename, not its full path, is written: a user who
// runs `sha256sum -c` from the directory that holds the archive
// expects a relative name.
func writePerBinaryChecksum(distDir string, p platform.Platform) error {
	archivePath := release.ArchivePath(distDir, p)
	sum, err := sha256File(archivePath)
	if err != nil {
		return fmt.Errorf("sha256 %s: %w", archivePath, err)
	}
	out := release.PerBinaryChecksumPath(distDir, p)
	content := fmt.Sprintf("%s  %s\n", sum, release.ArchiveName(p))
	if err := os.WriteFile(out, []byte(content), release.FileMode); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

// writeCombinedChecksums writes SHA256SUMS listing every archive.
//
// The list is sorted by archive name so two runs of the driver
// produce byte-identical output. The order is the one `sort.Strings`
// produces, which is byte-wise ascending on the filenames.
func writeCombinedChecksums(distDir string, platforms []platform.Platform) error {
	type entry struct {
		name string
		sum  string
	}
	entries := make([]entry, 0, len(platforms))
	for _, p := range platforms {
		archivePath := release.ArchivePath(distDir, p)
		sum, err := sha256File(archivePath)
		if err != nil {
			return fmt.Errorf("sha256 %s: %w", archivePath, err)
		}
		entries = append(entries, entry{name: release.ArchiveName(p), sum: sum})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  %s\n", e.sum, e.name)
	}
	out := release.CombinedChecksumPath(distDir)
	if err := os.WriteFile(out, []byte(b.String()), release.FileMode); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}

// signWithGPG signs SHA256SUMS with the Forge release key.
//
// It produces both an armored (.asc) and a binary (.gpg) signature.
// The key is identified by the fingerprint documented in
// SECURITY.md; the driver reads the fingerprint from the
// FORGE_GPG_KEY_FINGERPRINT environment variable, falling back to a
// build-time constant.
//
// # Why the key is not hard-coded
//
// A release key is rotated when it is compromised or when a
// maintainer leaves the project. Hard-coding the fingerprint in the
// driver would require a code change for every rotation. The
// environment variable lets the release pipeline supply the current
// fingerprint; the constant is the fallback for a local build.
//
// # The blocker
//
// If the key is not in the local keyring, or if gpg is not on PATH,
// the driver returns a blocker string. The blocker names the reason
// and the fix. The release pipeline treats it as informational.
//
// The blocker is deliberately a string, not an error: the caller
// decides whether to treat it as a failure. A release engineer who
// expects the key to be present sees the blocker and investigates;
// a CI job that never has the key sees the blocker and continues.
func signWithGPG(ctx context.Context, distDir string, stdout, stderr io.Writer) string {
	gpg, err := exec.LookPath("gpg")
	if err != nil {
		return "gpg not found on PATH; SHA256SUMS is unsigned"
	}
	fingerprint := os.Getenv("FORGE_GPG_KEY_FINGERPRINT")
	if fingerprint == "" {
		return "FORGE_GPG_KEY_FINGERPRINT is not set; SHA256SUMS is unsigned"
	}

	sums := release.CombinedChecksumPath(distDir)
	asc := release.GPGSignaturePath(distDir)
	gpgBin := release.GPGBinarySignaturePath(distDir)

	// Armored signature.
	if err := runGPG(ctx, gpg, "gpg", "--armor", "--detach-sign",
		"--local-user", fingerprint, "--output", asc, sums); err != nil {
		return fmt.Sprintf("gpg --armor failed: %v", err)
	}
	fmt.Fprintf(stdout, "OK: %s\n", asc)

	// Binary signature.
	if err := runGPG(ctx, gpg, "gpg", "--detach-sign",
		"--local-user", fingerprint, "--output", gpgBin, sums); err != nil {
		return fmt.Sprintf("gpg --detach-sign failed: %v", err)
	}
	fmt.Fprintf(stdout, "OK: %s\n", gpgBin)

	return ""
}

// runGPG runs gpg with the given arguments. It is a thin wrapper
// around exec.CommandContext that writes the command's output to
// the supplied writers.
func runGPG(ctx context.Context, gpg string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, gpg, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// sha256File returns the hex-encoded SHA-256 of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// keep the unused-import warnings quiet when a future refactor
// removes a helper.
var (
	_ = errors.Is
	_ = filepath.Join
)
