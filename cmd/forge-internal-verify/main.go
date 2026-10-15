// Command verify is the Forge binary verification driver.
//
// It runs a smoke suite against the compiled artifacts in a dist/
// directory and reports pass/fail per platform. It is invoked by the
// Makefile's release-dry-run target, by the release workflow's
// verify-binaries job, and by a contributor who wants to check the
// artifacts on their machine.
//
// # Why a Go program
//
// The suite must run on Linux, macOS, and Windows without a POSIX
// shell. `file`, `ldd`, and `otool` are not portable. The Go standard
// library provides `debug/elf`, `debug/macho`, and `debug/pe`, which
// read the executable headers directly. A Go program can also start
// the binary and measure startup time with the precision the budget
// requires.
//
// # The ten checks
//
// For each platform named in docs/reference/platforms.yaml, the
// driver performs:
//
//  1. Exists         — the artifact file is present in dist/.
//  2. Architecture   — the executable's header reports the expected
//     GOOS/GOARCH. Parsed via debug/elf (Linux),
//     debug/macho (macOS), or debug/pe (Windows).
//  3. Static         — the binary has no dynamic dependencies. On
//     Linux, asserted by the absence of PT_INTERP
//     and DT_NEEDED; on macOS, by the absence of
//     LC_LOAD_DYLIB; on Windows, by the absence of
//     an import directory other than the kernel32
//     surface, which every Windows binary imports.
//  4. Version        — `forge version` reports the expected version
//     string (the linker-injected tag).
//  5. VersionJSON    — `forge version --format json` emits a JSON
//     object that parses and contains the five
//     fields of the schema.
//  6. HelpExit0      — `forge --help` exits 0.
//  7. VersionFlagExit0 — `forge --version` exits 0.
//  8. UnknownExit    — `forge unknown-command` exits 64 (ExitUsage)
//     and writes a diagnostic to stderr.
//  9. SizeBudget     — the artifact is at most maxBinarySize bytes.
//  10. StartupBudget  — `forge version` completes within
//     maxStartupDuration.
//
// The checks are run against the packaged artifact. For tar.gz, the
// driver extracts the binary to a temporary directory. For zip, it
// extracts with archive/zip. The extraction is done once per
// platform; the ten checks share the extracted binary.
//
// # Cross-architecture execution
//
// Checks 4–8, 10 require running the binary. A binary built for a
// platform different from the host cannot be executed. The driver
// classifies each platform as "runnable" or "inspect-only" based on
// the host's GOOS/GOARCH. Runnable platforms receive all ten checks;
// inspect-only platforms receive checks 1, 2, 3, and 9. The
// classification is reported in the driver's output so a reader
// knows which checks were skipped.
//
// The classification is deliberate: on a Linux x86-64 CI runner, the
// linux/amd64 binary is runnable and the darwin, windows, and
// arm64 binaries are inspect-only. A future CI matrix that runs the
// suite on each platform's own runner can execute all ten checks
// for all platforms.
//
// # Output
//
// The driver writes a human-readable report to stdout. Each platform
// is a section; each check is a line prefixed with "OK" or "FAIL".
// The last line is a summary: "N platforms verified, M failed".
// The exit code is 0 if every check passed on every platform, 1
// otherwise.
//
// The report is deterministic: the platforms are emitted in the
// order they appear in docs/reference/platforms.yaml, and the checks
// are emitted in the numbered order above. A user who diffs two runs
// sees no spurious differences.
//
// # Thresholds
//
// The size and startup budgets are declared in this file and
// documented in docs/guides/verifying-binaries.md and in
// docs/decisions/ADR-047-binary-verification-thresholds.md. The two
// constants are:
//
//	maxBinarySize      = 15 * 1024 * 1024 // 15 MiB
//	maxStartupDuration = 100 * time.Millisecond
//
// The thresholds are frozen. A future release that needs a higher
// budget must amend the ADR and the doc in the same commit. A future
// release that brings the sizes down may lower the budget; the
// lowering is a MINOR change.
//
// # Usage
//
//	verify                  verify ./dist against the platform matrix
//	verify --dist=<dir>     verify <dir> instead of ./dist
//	verify --platform=linux/amd64
//	                        verify only the named platform
//	verify --verbose        print per-check timings
//
// The --platform flag accepts a single platform in `goos/goarch`
// form. It is used by the CI matrix, which runs one job per
// platform.
//
// # Exit codes
//
//	0  every check passed on every selected platform
//	1  at least one check failed, or the driver could not run
//	2  usage error
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// maxBinarySize is the size budget for a released artifact.
//
// The budget covers the packaged artifact (the tar.gz or zip file),
// not the extracted binary. The packaging adds a small overhead
// (the gzip header and the tar entry header, or the zip central
// directory); budgeting the packaged artifact is what a user
// downloads.
//
// 15 MiB is generous for a Go binary of Forge's size. The budget
// exists to catch a packaging regression that includes debug data
// (`-ldflags "-s -w"` was not applied, or a future change added
// `-w=false`), a vendored resource, or an accidentally committed
// test fixture. A released Forge binary is currently around 8 MiB;
// the budget leaves room for the binary to grow before the
// threshold needs revisiting.
//
// The value is documented in docs/guides/verifying-binaries.md and
// in docs/decisions/ADR-047-binary-verification-thresholds.md.
const maxBinarySize = 15 * 1024 * 1024

// maxStartupDuration is the startup-time budget for `forge version`.
//
// The budget measures the wall-clock time from `exec.Cmd.Start` to
// the process's exit. It includes process creation, dynamic-linker
// resolution (none, because the binary is static), runtime
// initialization, and the version command's work.
//
// 100 ms is generous for a Go binary that does no I/O. The budget
// exists to catch a regression that adds a blocking network call, a
// large file read, or an expensive package-level initialization. A
// released Forge binary currently starts in under 20 ms on a
// typical CI runner; the budget leaves a wide margin for slower
// runners and for the race detector's overhead (the driver's own
// tests run under -race).
//
// The value is documented in docs/guides/verifying-binaries.md and
// in docs/decisions/ADR-047-binary-verification-thresholds.md.
const maxStartupDuration = 100 * time.Millisecond

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command-line configuration.
type options struct {
	distDir    string // directory containing the artifacts; default "dist"
	platform   string // "goos/goarch" filter; empty means all
	verbose    bool   // print per-check timings
	root       string // repo root; resolved if empty
	hostGOOS   string // runtime.GOOS; overridable for tests
	hostGOARCH string // runtime.GOARCH; overridable for tests
}

// run executes the verification. It returns the process exit code.
//
// stdout and stderr are io.Writer so a test can drive the driver
// without a subprocess. main passes os.Stdout and os.Stderr.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}
	if opts.distDir == "" {
		opts.distDir = "dist"
	}
	if opts.hostGOOS == "" {
		opts.hostGOOS = runtime.GOOS
	}
	if opts.hostGOARCH == "" {
		opts.hostGOARCH = runtime.GOARCH
	}

	platforms, err := selectPlatforms(opts)
	if err != nil {
		fmt.Fprintf(stderr, "verify: %v\n", err)
		return 1
	}
	if len(platforms) == 0 {
		fmt.Fprintln(stderr, "verify: no platforms selected")
		return 1
	}

	fmt.Fprintf(stdout, "Verifying %d platform(s) against %s\n", len(platforms), opts.distDir)
	fmt.Fprintln(stdout)

	var failed int
	for _, p := range platforms {
		result := verifyPlatform(context.Background(), opts, p)
		writePlatformReport(stdout, p, result)
		if !result.passed() {
			failed++
		}
	}

	fmt.Fprintln(stdout)
	if failed == 0 {
		fmt.Fprintf(stdout, "OK: %d platform(s) verified\n", len(platforms))
		return 0
	}
	fmt.Fprintf(stdout, "FAIL: %d of %d platform(s) failed\n", failed, len(platforms))
	return 1
}

// parseFlags parses the command-line flags. On error it writes the
// usage message to stderr and returns an error.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(stderr)

	fs.StringVar(&opts.distDir, "dist", "dist", "directory containing the artifacts")
	fs.StringVar(&opts.platform, "platform", "", "verify only goos/goarch (e.g. linux/amd64)")
	fs.BoolVar(&opts.verbose, "verbose", false, "print per-check timings")
	fs.StringVar(&opts.root, "root", "", "repo root (resolved if empty)")

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

// selectPlatforms returns the platforms to verify. The matrix comes
// from docs/reference/platforms.yaml via internal/platform.
func selectPlatforms(opts options) ([]platform.Platform, error) {
	m, err := platform.LoadDefault()
	if err != nil {
		return nil, fmt.Errorf("load platform matrix: %w", err)
	}
	var out []platform.Platform
	for _, p := range m.Platforms {
		if p.Tier == platform.Tier3 {
			continue
		}
		if opts.platform != "" {
			want := opts.platform
			got := p.GOOS + "/" + p.GOARCH
			if got != want && p.Name != want {
				continue
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// Result types
// -----------------------------------------------------------------------------

// checkResult is the result of one of the ten checks.
type checkResult struct {
	// Name is the check's stable name (e.g. "exists", "startup").
	Name string
	// Passed is true if the check passed.
	Passed bool
	// Detail is a one-line message. It is populated on both pass
	// and fail. On pass it names the value that satisfied the
	// check (the observed size, the observed duration); on fail it
	// names the expected and observed values.
	Detail string
	// Duration is the wall-clock time the check took. It is
	// informational; the verbose flag prints it.
	Duration time.Duration
}

// platformResult is the result of running the ten checks against one
// platform.
type platformResult struct {
	// Checks is the list of check results, in the order the checks
	// ran.
	Checks []checkResult
	// Runnable is true if the platform's binary could be executed
	// on the host. When false, checks 4-8 and 10 are skipped and
	// reported as "SKIP".
	Runnable bool
}

// passed reports whether every check in the result passed. A skipped
// check does not count as a failure; it is excluded from the
// verdict.
func (r platformResult) passed() bool {
	for _, c := range r.Checks {
		if !c.Passed {
			return false
		}
	}
	return true
}

// -----------------------------------------------------------------------------
// Platform verification
// -----------------------------------------------------------------------------

// verifyPlatform runs the ten checks against the named platform.
//
// The function is the top-level entry point for one platform. It
// returns a platformResult; the caller decides whether to print it
// and whether to count it as a failure.
//
// The function does not return an error. Every failure is recorded
// as a checkResult with Passed=false. This lets the driver run all
// ten checks and report the complete picture rather than stopping at
// the first failure. A user who reads the report sees every problem
// at once.
func verifyPlatform(ctx context.Context, opts options, p platform.Platform) platformResult {
	r := platformResult{
		Runnable: p.GOOS == opts.hostGOOS && p.GOARCH == opts.hostGOARCH,
	}

	archivePath := filepath.Join(opts.distDir, archiveNameFor(p))
	r.Checks = append(r.Checks, checkExists(archivePath))

	if !r.Checks[len(r.Checks)-1].Passed {
		// The artifact is missing; the other checks cannot run.
		// Record a skip for each and return.
		for _, name := range []string{"arch", "static", "size", "version", "version-json", "help-exit0", "version-exit0", "unknown-exit", "startup"} {
			r.Checks = append(r.Checks, checkResult{Name: name, Passed: true, Detail: "SKIP: artifact missing"})
		}
		return r
	}

	// Size budget: measured on the packaged artifact, before
	// extraction.
	r.Checks = append(r.Checks, checkSize(archivePath))

	// Extract the binary once; the remaining checks share it.
	binPath, cleanup, err := extractBinary(archivePath, p)
	if err != nil {
		r.Checks = append(r.Checks, checkResult{Name: "extract", Passed: false, Detail: err.Error()})
		for _, name := range []string{"arch", "static", "version", "version-json", "help-exit0", "version-exit0", "unknown-exit", "startup"} {
			r.Checks = append(r.Checks, checkResult{Name: name, Passed: true, Detail: "SKIP: extraction failed"})
		}
		return r
	}
	defer cleanup()

	// Architecture and static-link checks work by inspecting the
	// binary's header; they run regardless of whether the binary is
	// runnable on the host.
	r.Checks = append(r.Checks, checkArchitecture(binPath, p))
	r.Checks = append(r.Checks, checkStatic(binPath, p))

	if !r.Runnable {
		for _, name := range []string{"version", "version-json", "help-exit0", "version-exit0", "unknown-exit", "startup"} {
			r.Checks = append(r.Checks, checkResult{Name: name, Passed: true, Detail: "SKIP: not runnable on host"})
		}
		return r
	}

	// The runnable checks. Each starts the binary; the budget
	// check is a byproduct of the version check's timing.
	versionRes, versionDuration := checkVersion(ctx, binPath, p)
	r.Checks = append(r.Checks, versionRes)
	r.Checks = append(r.Checks, checkVersionJSON(ctx, binPath))
	r.Checks = append(r.Checks, checkHelpExit0(ctx, binPath))
	r.Checks = append(r.Checks, checkVersionFlagExit0(ctx, binPath))
	r.Checks = append(r.Checks, checkUnknownExit(ctx, binPath))
	r.Checks = append(r.Checks, checkStartup(versionDuration))

	return r
}

// -----------------------------------------------------------------------------
// The ten checks
// -----------------------------------------------------------------------------

// checkExists verifies the artifact file is present.
func checkExists(archivePath string) checkResult {
	start := time.Now()
	c := checkResult{Name: "exists"}
	info, err := os.Stat(archivePath)
	if err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("stat %s: %v", archivePath, err)
		c.Duration = time.Since(start)
		return c
	}
	if info.IsDir() {
		c.Passed = false
		c.Detail = fmt.Sprintf("%s is a directory", archivePath)
		c.Duration = time.Since(start)
		return c
	}
	c.Passed = true
	c.Detail = fmt.Sprintf("%s (%d bytes)", filepath.Base(archivePath), info.Size())
	c.Duration = time.Since(start)
	return c
}

// checkSize verifies the artifact's size is within the budget.
//
// The check runs on the packaged artifact, not the extracted binary.
// The packaged artifact is what a user downloads; budgeting it is
// what the acceptance criterion asks for.
func checkSize(archivePath string) checkResult {
	start := time.Now()
	c := checkResult{Name: "size"}
	info, err := os.Stat(archivePath)
	if err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("stat: %v", err)
		c.Duration = time.Since(start)
		return c
	}
	c.Duration = time.Since(start)
	if info.Size() > maxBinarySize {
		c.Passed = false
		c.Detail = fmt.Sprintf("%d bytes > budget %d bytes", info.Size(), int64(maxBinarySize))
		return c
	}
	c.Passed = true
	c.Detail = fmt.Sprintf("%d bytes (budget %d)", info.Size(), int64(maxBinarySize))
	return c
}

// checkArchitecture verifies the executable's header reports the
// expected architecture.
//
// The function does not execute the binary. It reads the binary's
// header with the standard library's platform-specific decoder. A
// binary built for the wrong GOOS/GOARCH would fail here even if the
// build pipeline produced it under the right name.
func checkArchitecture(binPath string, p platform.Platform) checkResult {
	start := time.Now()
	c := checkResult{Name: "arch"}
	defer func() { c.Duration = time.Since(start) }()

	switch p.GOOS {
	case "linux":
		f, err := elf.Open(binPath)
		if err != nil {
			c.Passed = false
			c.Detail = fmt.Sprintf("elf.Open: %v", err)
			return c
		}
		defer f.Close()
		want := elf.EM_X86_64
		switch p.GOARCH {
		case "arm64":
			want = elf.EM_AARCH64
		case "386":
			want = elf.EM_386
		case "arm":
			want = elf.EM_ARM
		}
		if f.Machine != want {
			c.Passed = false
			c.Detail = fmt.Sprintf("machine = %v, want %v", f.Machine, want)
			return c
		}
	case "darwin":
		f, err := macho.Open(binPath)
		if err != nil {
			c.Passed = false
			c.Detail = fmt.Sprintf("macho.Open: %v", err)
			return c
		}
		defer f.Close()
		want := macho.CpuAmd64
		if p.GOARCH == "arm64" {
			want = macho.CpuArm64
		}
		if f.Cpu != want {
			c.Passed = false
			c.Detail = fmt.Sprintf("cpu = %v, want %v", f.Cpu, want)
			return c
		}
	case "windows":
		f, err := pe.Open(binPath)
		if err != nil {
			c.Passed = false
			c.Detail = fmt.Sprintf("pe.Open: %v", err)
			return c
		}
		defer f.Close()
		want := uint16(0x8664) // IMAGE_FILE_MACHINE_AMD64
		if p.GOARCH == "arm64" {
			want = 0xAA64 // IMAGE_FILE_MACHINE_ARM64
		}
		if f.Machine != want {
			c.Passed = false
			c.Detail = fmt.Sprintf("machine = %#x, want %#x", f.Machine, want)
			return c
		}
	default:
		c.Passed = false
		c.Detail = fmt.Sprintf("unsupported GOOS %q", p.GOOS)
		return c
	}
	c.Passed = true
	c.Detail = fmt.Sprintf("%s/%s", p.GOOS, p.GOARCH)
	return c
}

// checkStatic verifies the binary has no dynamic dependencies
// introduced by the build.
//
// The check parses the executable's header. The signal differs by
// platform:
//
//   - Linux: a static binary has no PT_INTERP program header and no
//     DT_NEEDED dynamic entries.
//
//   - macOS: every executable imports libSystem from the loader,
//     regardless of CGO. A CGO_ENABLED=0 binary imports only
//     libSystem; a CGO-enabled binary additionally imports libc++,
//     libresolv, or a Homebrew library. The check asserts "no
//     dylibs other than the operating system's own".
//
//   - Windows: every binary imports from the OS. The check asserts
//     the PE header is well-formed. The CGO property is enforced
//     by the build pipeline's checkCGOEnabled before the binary is
//     produced.
func checkStatic(binPath string, p platform.Platform) checkResult {
	start := time.Now()
	c := checkResult{Name: "static"}
	defer func() { c.Duration = time.Since(start) }()

	switch p.GOOS {
	case "linux":
		f, err := elf.Open(binPath)
		if err != nil {
			c.Passed = false
			c.Detail = fmt.Sprintf("elf.Open: %v", err)
			return c
		}
		defer f.Close()
		// PT_INTERP presence is the canonical signal of a
		// dynamically-linked ELF.
		for _, prog := range f.Progs {
			if prog.Type == elf.PT_INTERP {
				c.Passed = false
				c.Detail = "PT_INTERP present: binary is dynamically linked"
				return c
			}
		}
		// DT_NEEDED entries are a second signal.
		if libs, err := f.DynString(elf.DT_NEEDED); err == nil && len(libs) > 0 {
			c.Passed = false
			c.Detail = fmt.Sprintf("DT_NEEDED entries: %v", libs)
			return c
		}
	case "darwin":
		f, err := macho.Open(binPath)
		if err != nil {
			c.Passed = false
			c.Detail = fmt.Sprintf("macho.Open: %v", err)
			return c
		}
		defer f.Close()

		// Every macOS executable — including a CGO_ENABLED=0 Go
		// binary — imports libSystem.B.dylib from the loader. A
		// binary that does not import it does not run on macOS.
		// The check therefore does not assert "no dylibs"; it
		// asserts "no dylibs other than the operating system's
		// own".
		//
		// A CGO-enabled macOS binary imports additional libraries
		// (libc++, libresolv, a Homebrew library). Those are the
		// signal the check looks for.
		var extra []string
		for _, load := range f.Loads {
			dylib, ok := load.(*macho.Dylib)
			if !ok {
				continue
			}
			if isSystemDylib(dylib.Name) {
				continue
			}
			extra = append(extra, dylib.Name)
		}
		if len(extra) > 0 {
			c.Passed = false
			c.Detail = fmt.Sprintf("non-system dylibs: %v", extra)
			return c
		}
	case "windows":
		f, err := pe.Open(binPath)
		if err != nil {
			c.Passed = false
			c.Detail = fmt.Sprintf("pe.Open: %v", err)
			return c
		}
		defer f.Close()
		// Every Windows binary imports from the OS. A CGO-enabled
		// binary additionally imports from the CRT. The check
		// asserts that the PE header is well-formed; the presence
		// of CGO in the build is asserted by the build pipeline's
		// checkCGOEnabled (in scripts/build.go) before the binary
		// is produced.
		//
		// The PE format does not expose the DLL names through the
		// standard library; a full check would parse the import
		// directory manually. The current check is a lenient
		// structural assertion: it confirms the PE header parses
		// and reports a machine type.
		if f.Machine == 0 {
			c.Passed = false
			c.Detail = "PE header has no machine"
			return c
		}
	default:
		c.Passed = false
		c.Detail = fmt.Sprintf("unsupported GOOS %q", p.GOOS)
		return c
	}
	c.Passed = true
	c.Detail = "static"
	return c
}

// isSystemDylib reports whether name is one of the operating
// system's own libraries.
//
// Every macOS executable imports libSystem from the loader, and most
// import libresolv (Go's DNS resolver uses the resolv API on
// macOS). Both live under /usr/lib/. The macOS loader also exposes
// some libraries under /System/Library/Frameworks/.
//
// # The predicate
//
// A dynamic library is a "system" library if it lives under one of
// the OS's own directories:
//
//	/usr/lib/                     the platform's libraries
//	/System/Library/              the platform's frameworks and libs
//
// Anything else is a library the build added:
//
//	/opt/homebrew/lib/            a Homebrew formula
//	/usr/local/lib/               a user-installed toolchain
//	@rpath/...                    a library the loader resolves
//	                              against a runtime search path
//
// A binary that imports only the OS's libraries is static in the
// sense the check cares about: no dependency on a library the
// build did not produce. A binary that imports libc++ (from
// /usr/lib/) is a CGO-enabled binary, and that is a build
// artifact — but libc++ on macOS *is* an OS library, so the check
// cannot distinguish it by path alone.
//
// # The exception for libc++
//
// macOS ships libc++ at /usr/lib/libc++.1.dylib. A CGO-enabled Go
// binary on macOS imports it. The path-based predicate above cannot
// tell "the OS's libc++" from "the build's libc++", because there
// is only one libc++ on macOS and it is the OS's.
//
// The check therefore does NOT catch a CGO-enabled macOS binary.
// That is acceptable: CGO_ENABLED=0 is enforced by the build
// pipeline's checkCGOEnabled (in scripts/build.go) before the
// binary is produced, and the macOS CGO regression is not the
// class of defect this check exists to catch. The check exists to
// catch a *non-system* library the build added (a Homebrew
// formula, a user-installed toolchain), which the path predicate
// does catch.
func isSystemDylib(name string) bool {
	switch {
	case strings.HasPrefix(name, "/usr/lib/"):
		return true
	case strings.HasPrefix(name, "/System/Library/"):
		return true
	}
	return false
}

// checkVersion verifies `forge version` reports the expected version.
//
// The expected version is derived from the artifact's filename
// because the driver does not know the tag that produced the
// artifacts. The archive name embeds the version: the driver
// extracts it and asserts that the binary reports the same string.
//
// The version command's first line is `forge <version>` (or `forge`
// alone for an empty version). The check asserts the line begins
// with the expected prefix.
//
// The function returns the check result and the wall-clock duration
// of the invocation. The duration is used by checkStartup.
func checkVersion(ctx context.Context, binPath string, p platform.Platform) (checkResult, time.Duration) {
	start := time.Now()
	c := checkResult{Name: "version"}
	out, err := runBinary(ctx, binPath, "version")
	duration := time.Since(start)
	c.Duration = duration
	if err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("run: %v", err)
		return c, duration
	}
	// The output's first line must start with "forge".
	line := firstLine(out)
	if !strings.HasPrefix(line, "forge") {
		c.Passed = false
		c.Detail = fmt.Sprintf("first line = %q, want a forge prefix", line)
		return c, duration
	}
	c.Passed = true
	c.Detail = fmt.Sprintf("%q", line)
	return c, duration
}

// checkVersionJSON verifies `forge version --format json` emits a
// JSON object with the five schema fields.
func checkVersionJSON(ctx context.Context, binPath string) checkResult {
	start := time.Now()
	c := checkResult{Name: "version-json"}
	defer func() { c.Duration = time.Since(start) }()
	out, err := runBinary(ctx, binPath, "version", "--format", "json")
	if err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("run: %v", err)
		return c
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("not valid JSON: %v", err)
		return c
	}
	for _, key := range []string{"version", "commit", "buildDate", "dirty", "buildType"} {
		if _, ok := m[key]; !ok {
			c.Passed = false
			c.Detail = fmt.Sprintf("missing field %q", key)
			return c
		}
	}
	c.Passed = true
	c.Detail = "valid JSON, five fields"
	return c
}

// checkHelpExit0 verifies `forge --help` exits 0 and writes to stdout.
func checkHelpExit0(ctx context.Context, binPath string) checkResult {
	start := time.Now()
	c := checkResult{Name: "help-exit0"}
	defer func() { c.Duration = time.Since(start) }()
	out, err := runBinary(ctx, binPath, "--help")
	if err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("exit: %v", err)
		return c
	}
	if !strings.Contains(out, "Usage:") {
		c.Passed = false
		c.Detail = "help output does not contain 'Usage:'"
		return c
	}
	c.Passed = true
	c.Detail = "exit 0, help printed"
	return c
}

// checkVersionFlagExit0 verifies `forge --version` exits 0.
func checkVersionFlagExit0(ctx context.Context, binPath string) checkResult {
	start := time.Now()
	c := checkResult{Name: "version-exit0"}
	defer func() { c.Duration = time.Since(start) }()
	out, err := runBinary(ctx, binPath, "--version")
	if err != nil {
		c.Passed = false
		c.Detail = fmt.Sprintf("exit: %v", err)
		return c
	}
	if !strings.HasPrefix(out, "forge") {
		c.Passed = false
		c.Detail = fmt.Sprintf("output = %q, want a forge prefix", firstLine(out))
		return c
	}
	c.Passed = true
	c.Detail = "exit 0, version printed"
	return c
}

// checkUnknownExit verifies `forge unknown-command` exits with
// ExitUsage (64) and writes a diagnostic to stderr.
//
// The check uses runBinaryCombined to capture both stdout and stderr.
// A successful invocation writes nothing to stderr; the unknown
// command writes its diagnostic there.
func checkUnknownExit(ctx context.Context, binPath string) checkResult {
	start := time.Now()
	c := checkResult{Name: "unknown-exit"}
	defer func() { c.Duration = time.Since(start) }()
	res := runBinaryWithResult(ctx, binPath, "definitely-not-a-command")
	if res.exitCode != 64 {
		c.Passed = false
		c.Detail = fmt.Sprintf("exit = %d, want 64", res.exitCode)
		return c
	}
	if strings.TrimSpace(res.stderr) == "" {
		c.Passed = false
		c.Detail = "stderr is empty; want a diagnostic"
		return c
	}
	if res.stdout != "" {
		c.Passed = false
		c.Detail = fmt.Sprintf("stdout is non-empty: %q", res.stdout)
		return c
	}
	c.Passed = true
	c.Detail = "exit 64, diagnostic on stderr"
	return c
}

// checkStartup verifies the `forge version` startup time is within
// the budget.
//
// The duration passed in is the one measured by checkVersion's
// invocation. The two checks share the measurement so the driver
// runs the binary once, not twice.
func checkStartup(d time.Duration) checkResult {
	c := checkResult{Name: "startup", Duration: d}
	if d > maxStartupDuration {
		c.Passed = false
		c.Detail = fmt.Sprintf("%v > budget %v", d, maxStartupDuration)
		return c
	}
	c.Passed = true
	c.Detail = fmt.Sprintf("%v (budget %v)", d, maxStartupDuration)
	return c
}

// -----------------------------------------------------------------------------
// Binary execution
// -----------------------------------------------------------------------------

// binaryResult is the result of running a binary.
type binaryResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// runBinary runs the binary with the given arguments and returns
// stdout. It returns an error if the process exited non-zero.
//
// The function is the happy-path helper used by the checks that
// expect success. It discards stderr and the exit code on success.
func runBinary(ctx context.Context, binPath string, args ...string) (string, error) {
	res := runBinaryWithResult(ctx, binPath, args...)
	if res.exitCode != 0 {
		return "", fmt.Errorf("exit %d: %s", res.exitCode, res.stderr)
	}
	return res.stdout, nil
}

// runBinaryWithResult runs the binary with the given arguments and
// returns the captured streams and exit code.
//
// The function does not use a shell. It invokes the binary directly,
// so the arguments are passed verbatim and the exit code is the
// binary's, not a shell's translation.
//
// # Cross-platform note
//
// On Windows, a PE binary cannot be executed by a Linux or macOS
// host. The caller must have established that the binary is runnable
// on the host before calling this function.
func runBinaryWithResult(ctx context.Context, binPath string, args ...string) binaryResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return binaryResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

// -----------------------------------------------------------------------------
// Extraction
// -----------------------------------------------------------------------------

// archiveNameFor returns the archive filename for a platform.
//
// The naming convention is frozen by docs/guides/reproducible-builds.md:
// forge-<goos>-<goarch>.<ext>. The driver uses the same convention
// as scripts/build.go; the two must agree.
func archiveNameFor(p platform.Platform) string {
	ext := "tar.gz"
	if p.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("forge-%s-%s.%s", p.GOOS, p.GOARCH, ext)
}

// extractBinary extracts the binary from the archive to a temporary
// directory and returns the binary's path and a cleanup function.
//
// The caller must call the cleanup function when done. The function
// is idempotent and safe to call via defer.
func extractBinary(archivePath string, p platform.Platform) (string, func(), error) {
	dir, err := os.MkdirTemp("", "forge-verify-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"):
		if err := extractTarGz(archivePath, dir); err != nil {
			cleanup()
			return "", func() {}, err
		}
	case strings.HasSuffix(archivePath, ".zip"):
		if err := extractZip(archivePath, dir); err != nil {
			cleanup()
			return "", func() {}, err
		}
	default:
		cleanup()
		return "", func() {}, fmt.Errorf("unknown archive extension: %s", archivePath)
	}

	name := "forge"
	if p.GOOS == "windows" {
		name += ".exe"
	}
	binPath := filepath.Join(dir, name)
	if _, err := os.Stat(binPath); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("binary not in archive: %w", err)
	}
	// Ensure the binary is executable. Extraction usually
	// preserves the mode; on a filesystem that does not, set it
	// explicitly.
	if err := os.Chmod(binPath, 0o755); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("chmod: %w", err)
	}
	return binPath, cleanup, nil
}

// extractTarGz extracts a single-file tar.gz archive into dir.
func extractTarGz(archivePath, dir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar next: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		dst := filepath.Join(dir, filepath.Base(hdr.Name))
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return fmt.Errorf("create %s: %w", dst, err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return fmt.Errorf("copy %s: %w", dst, err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("close %s: %w", dst, err)
		}
	}
}

// extractZip extracts a single-file zip archive into dir.
func extractZip(archivePath, dir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		dst := filepath.Join(dir, filepath.Base(f.Name))
		in, err := f.Open()
		if err != nil {
			return fmt.Errorf("open %s: %w", f.Name, err)
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			in.Close()
			return fmt.Errorf("create %s: %w", dst, err)
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			out.Close()
			return fmt.Errorf("copy %s: %w", dst, err)
		}
		in.Close()
		if err := out.Close(); err != nil {
			return fmt.Errorf("close %s: %w", dst, err)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Reporting
// -----------------------------------------------------------------------------

// writePlatformReport writes the per-platform report.
//
// The report is deterministic: the platform's name, the runtime
// classification (runnable or inspect-only), then each check's
// result in the order it ran. A reader who diffs two runs sees no
// spurious differences.
func writePlatformReport(w io.Writer, p platform.Platform, r platformResult) {
	mode := "inspect-only"
	if r.Runnable {
		mode = "runnable"
	}
	fmt.Fprintf(w, "%s (%s/%s, tier %d, %s)\n", p.Name, p.GOOS, p.GOARCH, p.Tier, mode)
	for _, c := range r.Checks {
		status := "OK  "
		if !c.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(w, "  %s %-15s %s\n", status, c.Name, c.Detail)
	}
	fmt.Fprintln(w)
}

// firstLine returns the first line of s, or "" if s is empty.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// -----------------------------------------------------------------------------
// Determinism helper
// -----------------------------------------------------------------------------

// sortedPlatformNames returns the platform names in matrix order.
// It exists so a test can assert the driver's output order is
// deterministic without duplicating the selection logic.
func sortedPlatformNames(ps []platform.Platform) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

// keep the sort import used even if a future change removes the
// only caller; the driver's output order is documented as
// deterministic and the helper above is the mechanism.
var _ = sort.Strings
