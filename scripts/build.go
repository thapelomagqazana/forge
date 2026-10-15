// Command build is the Forge cross-compile driver.
//
// It is invoked by the Makefile's release targets and by the release
// workflow. It is a Go program, not a shell script, because:
//
//  1. It must run on Linux, macOS, and Windows without a POSIX shell.
//     `make release` on Windows with `nmake` cannot run a `for` loop;
//     a Go program can.
//
//  2. It must read docs/reference/platforms.yaml to know which
//     platforms to build. A Go program can call internal/platform
//     directly; a shell script would need to parse YAML.
//
//  3. It must inject the four version variables via -ldflags -X. The
//     variable names live in internal/version; the driver names them
//     in exactly one place (buildOne) so a rename is a one-line change.
//
//  4. It must fail on go.mod/go.sum drift. A Go program can back up
//     go.mod and go.sum, run `go mod tidy`, compare, and restore; a
//     shell script would need `mktemp` and `cp`, which behave
//     differently on Windows.
//
// Usage:
//
//	build --all                 cross-compile every Tier 1/2 platform
//	build --os=linux            cross-compile only linux
//	build --os=linux --arch=arm64
//	                            cross-compile only linux/arm64
//	build --checksums-only      produce <dist>/SHA256SUMS
//	build --sign-only           sign <dist>/forge-* with cosign
//	build --sbom-only           produce <dist>/forge.sbom.json
//	build --publish             upload the artifacts to the release
//	build --dist=<dir>          write to <dir> instead of ./dist
//
// Every flag may be combined. `--all --checksums-only` builds and then
// writes sums; `--all --sign-only --sbom-only` builds, signs, and
// produces an SBOM. The driver performs each requested step in order
// and fails on the first error.
//
// # Reproducibility
//
// The build is reproducible given the same source commit, the same Go
// toolchain, and the same SOURCE_DATE_EPOCH. The relevant flags are
// applied in buildOne:
//
//   - -trimpath            strip absolute host paths
//   - -buildvcs=true       embed the VCS revision (commit-derived time)
//   - -buildid=            clear the Go build ID
//   - -ldflags "-s -w"     strip symbol table and DWARF
//   - -X ...BuildDate=...  inject a SOURCE_DATE_EPOCH-derived timestamp
//   - CGO_ENABLED=0        no libc version embedded
//
// The archive entry ModTime is derived from SOURCE_DATE_EPOCH, not from
// the binary's mtime. See docs/guides/reproducible-builds.md.
//
// # Environment
//
// CGO_ENABLED MUST be 0. The driver refuses to run otherwise. This is
// the hard rule from docs/reference/platforms.yaml.
//
// SOURCE_DATE_EPOCH is read if set; otherwise the driver derives it
// from `git log -1 --format=%ct`. If neither is available, the driver
// fails rather than falling back to time.Now(): a binary that embeds
// the wall clock is not reproducible.
//
// # Output layout
//
// The default output directory is "dist"; --dist overrides it.
//
//	<dist>/
//	  forge-linux-amd64.tar.gz
//	  forge-linux-arm64.tar.gz
//	  forge-darwin-amd64.tar.gz
//	  forge-darwin-arm64.tar.gz
//	  forge-windows-amd64.zip
//	  forge-windows-arm64.zip
//	  SHA256SUMS
//	  forge.sbom.json
//	  forge-linux-amd64.tar.gz.sig
//	  forge-linux-amd64.tar.gz.pem
//	  ...
//
// # Exit codes
//
//	0  success
//	1  build, signing, or checksum failure
//	2  usage error
//
// # Testability
//
// run, parseFlags, runSteps, and every step function accept io.Writer
// for stdout and stderr. main passes os.Stdout and os.Stderr, which
// satisfy io.Writer. Tests pass io.Discard or a bytes.Buffer, so the
// step functions are exercised without a subprocess.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// defaultDistDir is the output directory when --dist is not given. The
// release pipeline uses the default; the reproducibility verifier
// passes --dist=<tempdir> to isolate two builds.
const defaultDistDir = "dist"

// binaryName is the name of the binary inside the archive. The archive
// itself is named forge-<goos>-<goarch>.<ext>; the version does not
// appear in the name (see archiveName).
const binaryName = "forge"

// modulePath is the import path of the module. It is used to build the
// -X linker targets. It MUST match the module directive in go.mod; a
// mismatch makes -X a silent no-op, so the driver asserts the two
// agree before any build runs.
const modulePath = "github.com/thapelomagqazana/forge"

// versionPkg is the package that declares the four linker variables.
// The driver targets this package for every -X flag.
const versionPkg = modulePath + "/internal/version"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command-line configuration. It is passed to
// every step so the driver remains a pure function of its inputs.
type options struct {
	all           bool
	osFilter      string // comma-separated: linux,darwin,windows
	archFilter    string // comma-separated: amd64,arm64
	checksumsOnly bool
	signOnly      bool
	sbomOnly      bool
	publish       bool
	version       string // overrides git describe; used by tests
	commit        string // overrides git rev-parse; used by tests
	buildDate     string // overrides SOURCE_DATE_EPOCH; used by tests
	root          string // repo root; resolved if empty
	dist          string // output directory; defaults to defaultDistDir
}

// run parses args, resolves the repo root, and executes each requested
// step in order. It returns the process exit code.
//
// stdout and stderr are io.Writer, not *os.File, so tests can drive
// the driver with io.Discard or a bytes.Buffer. main passes os.Stdout
// and os.Stderr, which satisfy io.Writer.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}

	if opts.root == "" {
		root, err := findModuleRoot()
		if err != nil {
			fmt.Fprintf(stderr, "build: %v\n", err)
			return 1
		}
		opts.root = root
	}
	if opts.dist == "" {
		opts.dist = defaultDistDir
	}

	if err := checkModulePath(opts.root); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	if err := checkCGOEnabled(); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	if err := checkModTidy(opts.root); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	if err := resolveBuildMetadata(&opts); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}

	if err := runSteps(opts, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "build: %v\n", err)
		return 1
	}
	return 0
}

// parseFlags parses the command-line flags. On error it writes the
// usage message to stderr and returns an error.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.BoolVar(&opts.all, "all", false, "build every Tier 1 and Tier 2 platform")
	fs.StringVar(&opts.osFilter, "os", "", "comma-separated GOOS list (linux,darwin,windows)")
	fs.StringVar(&opts.archFilter, "arch", "", "comma-separated GOARCH list (amd64,arm64)")
	fs.BoolVar(&opts.checksumsOnly, "checksums-only", false, "produce <dist>/SHA256SUMS")
	fs.BoolVar(&opts.signOnly, "sign-only", false, "sign <dist>/forge-* with cosign")
	fs.BoolVar(&opts.sbomOnly, "sbom-only", false, "produce <dist>/forge.sbom.json")
	fs.BoolVar(&opts.publish, "publish", false, "upload artifacts to the release")
	fs.StringVar(&opts.version, "version", "", "override git describe (for tests)")
	fs.StringVar(&opts.commit, "commit", "", "override git rev-parse (for tests)")
	fs.StringVar(&opts.buildDate, "build-date", "", "override SOURCE_DATE_EPOCH (for tests)")
	fs.StringVar(&opts.root, "root", "", "repo root (resolved if empty)")
	fs.StringVar(&opts.dist, "dist", "", "output directory (default \"dist\")")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

// runSteps executes the requested steps in a deterministic order:
// build, checksums, sbom, sign, publish. Each step sees the output of
// the previous step.
func runSteps(opts options, stdout, stderr io.Writer) error {
	buildRequested := opts.all || opts.osFilter != "" || opts.archFilter != ""
	if buildRequested {
		if err := stepBuild(opts, stdout, stderr); err != nil {
			return err
		}
	}
	if opts.checksumsOnly || buildRequested {
		if err := stepChecksums(opts, stdout, stderr); err != nil {
			return err
		}
	}
	if opts.sbomOnly {
		if err := stepSBOM(opts, stdout, stderr); err != nil {
			return err
		}
	}
	if opts.signOnly {
		if err := stepSign(opts, stdout, stderr); err != nil {
			return err
		}
	}
	if opts.publish {
		if err := stepPublish(opts, stdout, stderr); err != nil {
			return err
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Preconditions
// -----------------------------------------------------------------------------

// checkModulePath asserts that the module path the driver embeds in
// -X flags matches the module directive in go.mod. A mismatch makes
// every -X a silent no-op: the binary is produced, but forge version
// reports empty strings. This is the silent-failure trap described in
// docs/development.md "Build Metadata Injection".
func checkModulePath(root string) error {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return fmt.Errorf("read go.mod: %w", err)
	}
	got := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			got = strings.TrimSpace(strings.TrimPrefix(line, "module "))
			break
		}
	}
	if got == "" {
		return errors.New("go.mod has no module directive")
	}
	if got != modulePath {
		return fmt.Errorf("module path mismatch: driver has %q, go.mod has %q", modulePath, got)
	}
	return nil
}

// checkCGOEnabled fails if the process environment has CGO_ENABLED
// set to anything other than "0". This is the hard rule from
// docs/reference/platforms.yaml.
//
// Unset is accepted: the driver sets CGO_ENABLED=0 for each subprocess
// explicitly, so the ambient value does not matter. The check is a
// guard for a release engineer who set CGO_ENABLED=1 in their shell.
func checkCGOEnabled() error {
	v := os.Getenv("CGO_ENABLED")
	if v == "" {
		return nil
	}
	if v != "0" {
		return fmt.Errorf("CGO_ENABLED=%q, want 0 (release builds are static)", v)
	}
	return nil
}

// checkModTidy runs `go mod tidy` in a way that does not mutate the
// working tree: it backs up go.mod and go.sum, runs the command, and
// compares. If either file changed, the backup is restored and an
// error is returned.
//
// # Why not -mod=readonly
//
// `go mod tidy` does not accept -mod=readonly. The -mod flag is a
// build flag, not a `go mod` flag; passing it fails with
//
//	flag provided but not defined: -mod
//
// `go mod tidy` writes unconditionally. The only portable way to check
// for drift is the backup-and-restore pattern below.
//
// # Why this is safe
//
// The backup is taken in a temp directory, not alongside go.mod. If
// the process is killed between the backup and the restore, the temp
// directory is cleaned up by the OS or by a later `go clean`; go.mod
// and go.sum are restored by the defer before the function returns,
// even on error.
func checkModTidy(root string) error {
	backup, err := os.MkdirTemp("", "forge-tidy-")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(backup)

	restore, err := backupFiles(backup, root, "go.mod", "go.sum")
	if err != nil {
		return err
	}
	defer restore()

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed:\n%s", out)
	}

	for _, name := range []string{"go.mod", "go.sum"} {
		same, err := filesEqual(filepath.Join(backup, name), filepath.Join(root, name))
		if err != nil {
			return fmt.Errorf("compare %s: %w", name, err)
		}
		if !same {
			return fmt.Errorf("go mod tidy modified %s; run 'task tidy' on a clean branch and commit the result", name)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Build metadata
// -----------------------------------------------------------------------------

// resolveBuildMetadata fills in version, commit, and buildDate
// deterministically, unless the caller overrode them.
//
// # Determinism
//
// version comes from `git describe --tags --always --dirty`. On a
// tagged commit it is the tag; the -dirty suffix is empty because the
// release pipeline refuses to run on a dirty tree. Given the same
// commit, `git describe` is deterministic.
//
// commit comes from `git rev-parse --short HEAD`. Given the same
// commit, it is deterministic.
//
// buildDate is derived from SOURCE_DATE_EPOCH, which the
// reproducible-builds specification defines as "the last modification
// time of the source, in seconds since the epoch, in UTC". The release
// pipeline sets it to the committer timestamp of HEAD:
//
//	export SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)
//
// If SOURCE_DATE_EPOCH is unset, the driver falls back to the
// committer timestamp read directly from git. If neither is available
// (a non-git checkout), the driver refuses to run rather than fall
// back to time.Now(): a binary that embeds the wall clock is not
// reproducible.
//
// # Why not time.Now()
//
// An earlier version used time.Now().UTC(). Two builds of the same
// commit produced binaries with different BuildDate strings, so their
// SHA-256 sums differed. That is the defect WBS 17.3.1 exists to fix.
func resolveBuildMetadata(opts *options) error {
	if opts.version == "" {
		v, err := gitDescribe(opts.root)
		if err != nil {
			return fmt.Errorf("git describe: %w", err)
		}
		opts.version = v
	}
	if opts.commit == "" {
		c, err := gitRevParse(opts.root)
		if err != nil {
			return fmt.Errorf("git rev-parse: %w", err)
		}
		opts.commit = c
	}
	if opts.buildDate == "" {
		bd, err := sourceDateEpoch(opts.root)
		if err != nil {
			return fmt.Errorf("SOURCE_DATE_EPOCH: %w", err)
		}
		opts.buildDate = bd
	}
	if opts.version == "" {
		return errors.New("version is empty")
	}
	if opts.commit == "" {
		return errors.New("commit is empty")
	}
	if _, err := time.Parse(time.RFC3339, opts.buildDate); err != nil {
		return fmt.Errorf("buildDate %q is not RFC 3339: %w", opts.buildDate, err)
	}
	return nil
}

// sourceDateEpoch returns the build timestamp in RFC 3339 UTC.
//
// Priority:
//
//  1. $SOURCE_DATE_EPOCH, if set and parseable as an integer. The
//     value is Unix seconds; it is converted to RFC 3339 with a
//     trailing Z. This is the reproducible-builds standard.
//
//  2. `git log -1 --format=%ct` in root. %ct is the committer
//     timestamp in Unix seconds. Same conversion.
//
//  3. An error. A non-git checkout with no SOURCE_DATE_EPOCH is not
//     reproducible; the driver refuses rather than silently embedding
//     time.Now().
func sourceDateEpoch(root string) (string, error) {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return "", fmt.Errorf("SOURCE_DATE_EPOCH=%q is not an integer: %w", v, err)
		}
		return time.Unix(n, 0).UTC().Format(time.RFC3339), nil
	}
	cmd := exec.Command("git", "log", "-1", "--format=%ct")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git log: %w (set SOURCE_DATE_EPOCH for non-git checkouts)", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return "", fmt.Errorf("git log output is not an integer: %w", err)
	}
	return time.Unix(n, 0).UTC().Format(time.RFC3339), nil
}

// sourceDateUnix converts an RFC 3339 timestamp back to Unix seconds.
// It is used to set SOURCE_DATE_EPOCH for the go build subprocess from
// the resolved buildDate. On any parse failure it returns 0, which is
// a valid Unix timestamp (1970-01-01) and will be visible if the
// contract is violated.
func sourceDateUnix(buildDate string) int64 {
	t, err := time.Parse(time.RFC3339, buildDate)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// -----------------------------------------------------------------------------
// Artifact naming
// -----------------------------------------------------------------------------

// archiveName returns the deterministic archive name for a platform.
//
// The name is forge-<goos>-<goarch>.<ext>, where ext is "zip" for
// Windows and "tar.gz" for everything else. The version does not
// appear in the name: it is embedded in the binary via -X and readable
// with `forge version`. Keeping the archive name version-independent
// makes the archive's SHA-256 the only thing that changes between
// releases, which is the property a package manager wants.
//
// The inner binary is named forge on Unix and forge.exe on Windows.
//
// The convention is frozen by docs/guides/reproducible-builds.md §
// "Artifact naming" and by docs/reference/release-channels.md.
// Changing it is a MAJOR change: it breaks every script that parses
// the archive name.
func archiveName(goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s-%s-%s.%s", binaryName, goos, goarch, ext)
}

// archiveModTime returns the ModTime to embed in tar and zip entries.
// It is derived from the build's SOURCE_DATE_EPOCH, so two builds of
// the same commit produce archives with identical timestamps.
//
// Using time.Time{} (the Unix epoch) would also be deterministic but
// would produce archives with an obviously wrong timestamp. Deriving
// from the commit time keeps the timestamp meaningful.
func archiveModTime(buildDate string) time.Time {
	t, err := time.Parse(time.RFC3339, buildDate)
	if err != nil {
		return time.Unix(0, 0).UTC()
	}
	return t.UTC()
}

// -----------------------------------------------------------------------------
// Step: build
// -----------------------------------------------------------------------------

// stepBuild cross-compiles the selected platforms. For each platform
// it runs `go build` with the correct GOOS/GOARCH, -trimpath,
// -buildvcs=true, -buildid=, and the four -X linker flags. It packages
// the result as .tar.gz (Linux, macOS) or .zip (Windows).
//
// The output directory (opts.dist) is removed and recreated at the
// start, so a partial previous run cannot mask a failure.
func stepBuild(opts options, stdout, stderr io.Writer) error {
	targets, err := selectTargets(opts)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("no platforms selected")
	}

	if err := os.RemoveAll(opts.dist); err != nil {
		return fmt.Errorf("clean %s: %w", opts.dist, err)
	}
	if err := os.MkdirAll(opts.dist, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", opts.dist, err)
	}

	for _, p := range targets {
		if err := buildOne(opts, p, stdout, stderr); err != nil {
			return fmt.Errorf("%s: %w", p.Name, err)
		}
	}
	return nil
}

// selectTargets returns the platforms to build, filtered by --all,
// --os, and --arch. The matrix comes from docs/reference/platforms.yaml
// via internal/platform.
func selectTargets(opts options) ([]platform.Platform, error) {
	m, err := platform.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load platform matrix: %w", err)
	}

	osSet := parseCSV(opts.osFilter)
	archSet := parseCSV(opts.archFilter)

	var out []platform.Platform
	for _, p := range m.Platforms {
		// Tier 3 is best-effort and not produced by default.
		if p.Tier == platform.Tier3 {
			continue
		}
		if !opts.all {
			if len(osSet) > 0 {
				if _, ok := osSet[p.GOOS]; !ok {
					continue
				}
			}
			if len(archSet) > 0 {
				if _, ok := archSet[p.GOARCH]; !ok {
					continue
				}
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// buildOne cross-compiles one platform and packages the result.
//
// # Reproducibility
//
// The build is reproducible given the same commit, the same Go
// toolchain, and the same SOURCE_DATE_EPOCH. The flags that make it so
// are:
//
//   - -trimpath            strips absolute host paths
//   - -buildvcs=true       embeds the VCS revision, whose timestamp
//     comes from the commit
//   - -ldflags "-buildid=" clears the Go build ID (a linker flag, not
//     a `go build` flag)
//   - -ldflags "-s -w"     strips the symbol table and DWARF
//   - -X ...BuildDate=...  injects the SOURCE_DATE_EPOCH timestamp
//   - CGO_ENABLED=0        disables CGO, so no libc version is embedded
//
// The archive timestamp is derived from SOURCE_DATE_EPOCH, not from
// the wall clock.
//
// See docs/guides/reproducible-builds.md.
func buildOne(opts options, p platform.Platform, stdout, stderr io.Writer) error {
	fmt.Fprintf(stdout, "==> %s (%s/%s, tier %d)\n", p.Name, p.GOOS, p.GOARCH, p.Tier)

	binary := binaryName
	if p.GOOS == "windows" {
		binary += ".exe"
	}
	binPath := filepath.Join(opts.dist, binary)

	// The linker flags. -buildid= is a linker flag, not a `go build`
	// flag: it clears the Go build ID so two builds of the same
	// commit produce byte-identical binaries. Passing it to
	// `go build` directly fails with:
	//
	//     flag provided but not defined: -buildid
	//
	// The five -X flags inject the four version variables. -s and -w
	// strip the symbol table and DWARF data.
	ldflags := strings.Join([]string{
		"-s", "-w",
		"-buildid=",
		fmt.Sprintf("-X %s.Version=%s", versionPkg, opts.version),
		fmt.Sprintf("-X %s.Commit=%s", versionPkg, opts.commit),
		fmt.Sprintf("-X %s.BuildDate=%s", versionPkg, opts.buildDate),
		fmt.Sprintf("-X %s.Dirty=false", versionPkg),
	}, " ")

	cmd := exec.Command("go", "build",
		"-trimpath",
		"-buildvcs=true",
		"-ldflags", ldflags,
		"-o", binPath,
		"./cmd/forge",
	)
	cmd.Dir = opts.root
	cmd.Env = append(os.Environ(),
		"GOOS="+p.GOOS,
		"GOARCH="+p.GOARCH,
		"CGO_ENABLED=0",
		"SOURCE_DATE_EPOCH="+strconv.FormatInt(sourceDateUnix(opts.buildDate), 10),
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}

	name := archiveName(p.GOOS, p.GOARCH)
	path := filepath.Join(opts.dist, name)
	modTime := archiveModTime(opts.buildDate)
	if p.GOOS == "windows" {
		if err := writeZip(path, binPath, modTime); err != nil {
			return err
		}
	} else {
		if err := writeTarGz(path, binPath, modTime); err != nil {
			return err
		}
	}
	if err := os.Remove(binPath); err != nil {
		return fmt.Errorf("remove intermediate binary: %w", err)
	}
	fmt.Fprintf(stdout, "    %s\n", name)
	return nil
}

// -----------------------------------------------------------------------------
// Step: checksums
// -----------------------------------------------------------------------------

// stepChecksums writes <dist>/SHA256SUMS containing the SHA-256 of
// every <dist>/forge-* file. The format is the standard `sha256sum`
// format so a user can verify with `sha256sum -c SHA256SUMS`.
func stepChecksums(opts options, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "==> checksums")

	entries, err := os.ReadDir(opts.dist)
	if err != nil {
		return fmt.Errorf("read %s: %w", opts.dist, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, binaryName+"-") {
			continue
		}
		if strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".pem") {
			continue
		}
		files = append(files, name)
	}
	sort.Strings(files)

	out := filepath.Join(opts.dist, "SHA256SUMS")
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	defer f.Close()
	for _, name := range files {
		sum, err := sha256File(filepath.Join(opts.dist, name))
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(f, "%s  %s\n", sum, name); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
	}
	fmt.Fprintf(stdout, "    %d files\n", len(files))
	return nil
}

// -----------------------------------------------------------------------------
// Step: SBOM
// -----------------------------------------------------------------------------

// sbomComponent is one entry in the CycloneDX component list.
type sbomComponent struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Hashes maps an algorithm name to the hex-encoded digest. The
	// driver records SHA-256 for every archive it produces so a
	// verifier can cross-check SHA256SUMS against the SBOM.
	Hashes map[string]string `json:"hashes,omitempty"`
}

// sbomDocument is the CycloneDX 1.5 document the driver writes.
//
// It is minimal but valid: it names the application, its version, the
// Go module dependencies from go.mod, and each produced archive with
// its SHA-256. A future WBS item may replace it with a call to
// `cyclonedx-gomod`, which produces a richer SBOM. The interface is
// stable: the file is <dist>/forge.sbom.json and it is CycloneDX JSON.
type sbomDocument struct {
	BOMFormat   string          `json:"bomFormat"`
	SpecVersion string          `json:"specVersion"`
	Version     int             `json:"version"`
	Metadata    sbomMetadata    `json:"metadata"`
	Components  []sbomComponent `json:"components"`
}

type sbomMetadata struct {
	Component sbomComponent `json:"component"`
}

// stepSBOM writes <dist>/forge.sbom.json in CycloneDX 1.5 JSON format.
func stepSBOM(opts options, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "==> sbom")

	deps, err := goModDependencies(opts.root)
	if err != nil {
		return err
	}

	doc := sbomDocument{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: sbomMetadata{
			Component: sbomComponent{Type: "application", Name: "forge", Version: opts.version},
		},
	}
	for _, d := range deps {
		doc.Components = append(doc.Components, sbomComponent{
			Type: "library", Name: d.path, Version: d.version,
		})
	}
	if err := recordArtifacts(&doc, opts.dist, opts.version); err != nil {
		return err
	}

	out := filepath.Join(opts.dist, "forge.sbom.json")
	f, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encode sbom: %w", err)
	}
	fmt.Fprintf(stdout, "    %s (%d components)\n", out, len(doc.Components))
	return nil
}

// recordArtifacts adds one component per <dist>/forge-* archive to the
// SBOM, with the SHA-256 recorded in the Hashes map. The component
// name is the archive name; the version is the build version.
func recordArtifacts(doc *sbomDocument, distDir, version string) error {
	entries, err := os.ReadDir(distDir)
	if err != nil {
		return fmt.Errorf("read %s: %w", distDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, binaryName+"-") {
			continue
		}
		if strings.HasSuffix(name, ".sig") || strings.HasSuffix(name, ".pem") {
			continue
		}
		sum, err := sha256File(filepath.Join(distDir, name))
		if err != nil {
			return err
		}
		doc.Components = append(doc.Components, sbomComponent{
			Type:    "file",
			Name:    name,
			Version: version,
			Hashes:  map[string]string{"SHA-256": sum},
		})
	}
	return nil
}

// -----------------------------------------------------------------------------
// Step: sign
// -----------------------------------------------------------------------------

// stepSign signs every <dist>/forge-* archive with cosign keyless. It
// requires cosign on PATH and a valid OIDC token (in CI, the
// id-token: write permission provides it).
//
// Signing is skipped for nightly builds (the caller decides; the
// driver does not read the channel).
func stepSign(opts options, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "==> sign")

	if _, err := exec.LookPath("cosign"); err != nil {
		return fmt.Errorf("cosign not found on PATH: %w", err)
	}
	entries, err := os.ReadDir(opts.dist)
	if err != nil {
		return fmt.Errorf("read %s: %w", opts.dist, err)
	}
	signed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, binaryName+"-") {
			continue
		}
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".zip") {
			continue
		}
		path := filepath.Join(opts.dist, name)
		cmd := exec.Command("cosign", "sign-blob",
			"--yes",
			"--output-signature", path+".sig",
			"--output-certificate", path+".pem",
			path,
		)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cosign sign %s: %w", name, err)
		}
		signed++
	}
	fmt.Fprintf(stdout, "    %d signatures\n", signed)
	return nil
}

// -----------------------------------------------------------------------------// Step: publish
// -----------------------------------------------------------------------------

// stepPublish uploads the artifacts to the GitHub Release for the
// current tag. It requires gh on PATH and an authenticated session.
//
// The driver does not decide the channel: the caller (release.yml)
// has already routed the tag. Publishing is the same command for every
// channel; the channel's effect is in which artifacts exist and
// whether they are signed.
func stepPublish(opts options, stdout, stderr io.Writer) error {
	fmt.Fprintln(stdout, "==> publish")

	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh not found on PATH: %w", err)
	}
	entries, err := os.ReadDir(opts.dist)
	if err != nil {
		return fmt.Errorf("read %s: %w", opts.dist, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(opts.dist, e.Name()))
	}
	args := append([]string{"release", "upload", opts.version, "--clobber"}, files...)
	cmd := exec.Command("gh", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh release upload: %w", err)
	}
	fmt.Fprintf(stdout, "    uploaded %d files to %s\n", len(files), opts.version)
	return nil
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// parseCSV parses a comma-separated list into a set. It trims spaces
// and ignores empty entries, so "linux,,darwin" and " linux , darwin "
// both produce {linux, darwin}.
func parseCSV(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out[part] = struct{}{}
		}
	}
	return out
}

// findModuleRoot walks up from the current working directory to find
// go.mod. It is the same algorithm used elsewhere in the repo.
func findModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// gitDescribe runs `git describe --tags --always --dirty`. The output
// is the version string embedded in the binary.
func gitDescribe(root string) (string, error) {
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// gitRevParse runs `git rev-parse --short HEAD`.
func gitRevParse(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// writeTarGz writes a single-file .tar.gz archive containing binPath.
//
// The archive's entry uses the binary's base name, mode 0o755, and the
// supplied modTime. modTime MUST come from archiveModTime so two
// builds of the same commit produce identical archives.
//
// Uname and Gname are left empty; the archive must not embed the
// building user.
func writeTarGz(archivePath, binPath string, modTime time.Time) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	info, err := os.Stat(binPath)
	if err != nil {
		return err
	}
	hdr := &tar.Header{
		Name:    filepath.Base(binPath),
		Mode:    0o755,
		Size:    info.Size(),
		ModTime: modTime,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	in, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err := io.Copy(tw, in); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// writeZip writes a single-file .zip archive containing binPath.
//
// The archive's entry uses the binary's base name and the supplied
// modTime. modTime MUST come from archiveModTime so two builds of the
// same commit produce identical archives.
func writeZip(archivePath, binPath string, modTime time.Time) error {
	f, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	info, err := os.Stat(binPath)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	hdr.Name = filepath.Base(binPath)
	hdr.Method = zip.Deflate
	// zip.FileInfoHeader sets Modified from info.ModTime(); we
	// override it so the archive is reproducible.
	hdr.Modified = modTime
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	in, err := os.Open(binPath)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err := io.Copy(w, in); err != nil {
		return err
	}
	return zw.Close()
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

// backupFiles copies each name from srcDir into dstDir and returns a
// function that restores every file from dstDir back into srcDir.
// Missing source files are skipped: a repository may legitimately have
// go.mod without go.sum, or vice versa.
//
// The returned restore function is idempotent and safe to call via
// defer. It ignores errors from the restore itself, because the caller
// has already decided the outcome and a restore failure is not
// actionable at the call site.
func backupFiles(dstDir, srcDir string, names ...string) (func(), error) {
	restore := func() {}
	for _, name := range names {
		src := filepath.Join(srcDir, name)
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return restore, fmt.Errorf("stat %s: %w", src, err)
		}
		dst := filepath.Join(dstDir, name)
		if err := copyFile(src, dst); err != nil {
			return restore, fmt.Errorf("backup %s: %w", src, err)
		}
		// Capture name and dst for the closure.
		n, d := name, dst
		prev := restore
		restore = func() {
			prev()
			_ = copyFile(d, filepath.Join(srcDir, n))
		}
	}
	return restore, nil
}

// copyFile copies src to dst, preserving the file mode.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// filesEqual reports whether the contents of a and b are byte-identical.
func filesEqual(a, b string) (bool, error) {
	ab, err := os.ReadFile(a)
	if err != nil {
		return false, err
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ab, bb), nil
}

// goModDep is a single require directive from go.mod.
type goModDep struct {
	path    string
	version string
}

// goModDependencies parses go.mod and returns the require directives
// inside the `require (...)` block. Single-line `require x v` outside
// a block is also handled. The parser is minimal by design: gofmt
// produces the block form, and the driver only needs the module paths
// and versions for the SBOM.
func goModDependencies(root string) ([]goModDep, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, err
	}
	var deps []goModDep
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if line == "require (" {
			inBlock = true
			continue
		}
		if inBlock && line == ")" {
			inBlock = false
			continue
		}
		if inBlock {
			fields := strings.Fields(line)
			if len(fields) >= 2 && !strings.HasPrefix(fields[0], "//") {
				deps = append(deps, goModDep{path: fields[0], version: fields[1]})
			}
			continue
		}
		if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "(") {
			fields := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(fields) >= 2 {
				deps = append(deps, goModDep{path: fields[0], version: fields[1]})
			}
		}
	}
	return deps, nil
}
