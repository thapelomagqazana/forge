// Command verify-reproducible verifies that Forge's cross-compilation
// pipeline is reproducible.
//
// It builds the requested platforms twice, in two separate temporary
// directories, and compares the SHA-256 of the resulting archives. If
// any pair differs, the program reports which platforms differ and
// exits 1.
//
// Usage:
//
//	verify-reproducible                 verify every Tier 1/2 platform
//	verify-reproducible --os=linux      verify only linux
//	verify-reproducible --arch=arm64    verify only arm64
//	verify-reproducible --verbose       print per-platform timings
//
// # What "reproducible" means here
//
// Two builds of the same commit, on the same host, with the same Go
// toolchain, must produce byte-identical archives. This is a subset of
// the reproducible-builds definition: it does not yet assert cross-host
// reproducibility, which requires a CI matrix. The CI workflow in
// .github/workflows/reproducible.yml runs this verifier on every
// release tag.
//
// # How it works
//
// The verifier runs `go run ./scripts/build.go --os=... --arch=...
// --dist=<dir>` twice, each time with a fresh temporary output
// directory, and computes the SHA-256 of the resulting archive in each
// directory. It reads docs/reference/platforms.yaml (via
// internal/platform) to know which platforms to test.
//
// The two builds use the same SOURCE_DATE_EPOCH. The verifier reads it
// from the environment if set; otherwise it derives it from
// `git log -1 --format=%ct`. This matches the release pipeline's
// behavior.
//
// # Exit codes
//
//	0  every platform's two archives are byte-identical
//	1  at least one platform differs, or a build failed
//	2  usage error
//
// # Testability
//
// run, parseFlags, verifyOne, and runBuildOnce accept io.Writer for
// stdout and stderr. main passes os.Stdout and os.Stderr. Tests pass
// io.Discard or a bytes.Buffer.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/platform"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command-line configuration.
type options struct {
	osFilter   string
	archFilter string
	verbose    bool
	root       string
}

// run executes the verification. It returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}
	if opts.root == "" {
		root, err := findModuleRoot()
		if err != nil {
			fmt.Fprintf(stderr, "verify-reproducible: %v\n", err)
			return 1
		}
		opts.root = root
	}

	// Resolve SOURCE_DATE_EPOCH once, so both builds use the same
	// value. If the caller set it, use theirs. Otherwise derive from
	// the commit.
	sde, err := resolveSourceDateEpoch(opts.root)
	if err != nil {
		fmt.Fprintf(stderr, "verify-reproducible: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "SOURCE_DATE_EPOCH=%d (%s)\n",
		sde, time.Unix(sde, 0).UTC().Format(time.RFC3339))

	targets, err := selectTargets(opts)
	if err != nil {
		fmt.Fprintf(stderr, "verify-reproducible: %v\n", err)
		return 1
	}
	if len(targets) == 0 {
		fmt.Fprintln(stderr, "verify-reproducible: no platforms selected")
		return 1
	}
	fmt.Fprintf(stdout, "Verifying %d platform(s)\n", len(targets))

	failures := 0
	for _, p := range targets {
		if err := verifyOne(opts, p, sde, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "FAIL %s: %v\n", p.Name, err)
			failures++
			continue
		}
		fmt.Fprintf(stdout, "OK   %s\n", p.Name)
	}
	if failures > 0 {
		fmt.Fprintf(stderr, "\n%d platform(s) failed reproducibility\n", failures)
		return 1
	}
	fmt.Fprintln(stdout, "\nAll platforms reproduced byte-for-byte")
	return 0
}

// parseFlags parses the command-line flags.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("verify-reproducible", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.osFilter, "os", "", "comma-separated GOOS list")
	fs.StringVar(&opts.archFilter, "arch", "", "comma-separated GOARCH list")
	fs.BoolVar(&opts.verbose, "verbose", false, "print per-platform timings")
	fs.StringVar(&opts.root, "root", "", "repo root (resolved if empty)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

// verifyOne builds platform p twice and compares the SHA-256 of the
// resulting archive. It returns an error if either build fails or if
// the two archives differ.
//
// Each build writes to its own temp directory, so the two builds do
// not share any output. The driver's --dist flag isolates them.
func verifyOne(opts options, p platform.Platform, sde int64, stdout, stderr io.Writer) error {
	sums := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		dir, err := os.MkdirTemp("", fmt.Sprintf("forge-repro-%s-%d-", p.Name, i))
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)

		start := time.Now()
		if err := runBuildOnce(opts, p, sde, dir, stdout, stderr); err != nil {
			return fmt.Errorf("build %d: %w", i+1, err)
		}
		if opts.verbose {
			fmt.Fprintf(stdout, "    build %d: %s\n", i+1, time.Since(start))
		}

		name := archiveNameFor(p.GOOS, p.GOARCH)
		sum, err := sha256Of(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("sha256 of %s: %w", name, err)
		}
		sums = append(sums, sum)
	}

	if sums[0] != sums[1] {
		return fmt.Errorf("archives differ:\n  build 1: %s\n  build 2: %s", sums[0], sums[1])
	}
	return nil
}

// runBuildOnce runs the driver once with --dist=<dir>. The driver
// writes its artifacts to <dir>; the repo root is opts.root, so the
// driver can resolve ./scripts/build.go and ./cmd/forge.
//
// SOURCE_DATE_EPOCH is set on the subprocess from the resolved value,
// so both builds use the same timestamp regardless of the ambient
// environment.
func runBuildOnce(opts options, p platform.Platform, sde int64, dir string, stdout, stderr io.Writer) error {
	cmd := exec.Command("go", "run", "./scripts/build.go",
		"--os="+p.GOOS,
		"--arch="+p.GOARCH,
		"--dist="+dir,
	)
	cmd.Dir = opts.root
	cmd.Env = append(os.Environ(),
		"SOURCE_DATE_EPOCH="+strconv.FormatInt(sde, 10),
	)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// resolveSourceDateEpoch returns the Unix timestamp to use for the
// build. It prefers the SOURCE_DATE_EPOCH environment variable and
// falls back to the commit's committer timestamp.
func resolveSourceDateEpoch(root string) (int64, error) {
	if v := os.Getenv("SOURCE_DATE_EPOCH"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("SOURCE_DATE_EPOCH=%q is not an integer: %w", v, err)
		}
		return n, nil
	}
	cmd := exec.Command("git", "log", "-1", "--format=%ct")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git log: %w", err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("git log output is not an integer: %w", err)
	}
	return n, nil
}

// selectTargets returns the platforms to verify, filtered by --os and
// --arch. It uses the same platform matrix the driver uses, and skips
// Tier 3 (best-effort, not produced by default).
func selectTargets(opts options) ([]platform.Platform, error) {
	m, err := platform.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load platform matrix: %w", err)
	}
	osSet := parseCSV(opts.osFilter)
	archSet := parseCSV(opts.archFilter)
	var out []platform.Platform
	for _, p := range m.Platforms {
		if p.Tier == platform.Tier3 {
			continue
		}
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
		out = append(out, p)
	}
	return out, nil
}

// parseCSV parses a comma-separated list into a set. It trims spaces
// and ignores empty entries.
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

// findModuleRoot walks up from the current working directory looking
// for go.mod.
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
			return "", fmt.Errorf("no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// sha256Of returns the hex-encoded SHA-256 of a file.
func sha256Of(path string) (string, error) {
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

// archiveNameFor mirrors the driver's archiveName. It is duplicated
// here because the two programs are separate main packages; a future
// refactor may move the naming convention to a shared package.
//
// The convention is frozen by docs/guides/reproducible-builds.md §
// "Artifact naming". If the driver's convention changes, this function
// must change in the same commit.
func archiveNameFor(goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("forge-%s-%s.%s", goos, goarch, ext)
}
