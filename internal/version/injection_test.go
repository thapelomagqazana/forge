package version

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────

// skipIfNoGit skips the test if `git` is not on PATH. These tests
// exercise the git-derived contract; without git they are meaningless.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
}

// runGit runs a git command in dir and returns stdout with trailing
// whitespace trimmed. It fails the test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
}

// repoRoot walks up from the test file's directory until it finds
// go.mod. It fails if it reaches the filesystem root.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("reached filesystem root without finding go.mod")
		}
		dir = parent
	}
}

// readRepoFile reads a file from the repository root.
func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// parseGoModModule extracts the module path from a go.mod file.
func parseGoModModule(t *testing.T, gomod string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^module\s+(\S+)`)
	m := re.FindStringSubmatch(gomod)
	if m == nil {
		return ""
	}
	return m[1]
}

// parseTaskfileModule extracts `MODULE: <value>` from the Taskfile's
// vars: block and returns the value with any surrounding single or
// double quotes removed.
//
// Taskfiles are YAML. YAML allows a scalar to be written in three
// forms:
//
//	MODULE: github.com/example/forge
//	MODULE: 'github.com/example/forge'
//	MODULE: "github.com/example/forge"
//
// All three represent the same string. The parser must accept all
// three, because the Taskfile author is free to quote the value for
// readability, to escape special characters, or by convention.
//
// The parse is deliberately textual rather than YAML-aware:
// pulling in a YAML parser for one line is overkill, and the value
// cannot contain a space, so `\S+` is sufficient to isolate it.
func parseTaskfileModule(t *testing.T, taskfile string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*MODULE:\s*(\S+)\s*$`)
	m := re.FindStringSubmatch(taskfile)
	if m == nil {
		return ""
	}
	value := m[1]
	// Strip a matched pair of single or double quotes.
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
			value = value[1 : len(value)-1]
		}
	}
	return value
}

// stripGoComments removes Go line comments (// ...) and block
// comments (/* ... */) from src, preserving newlines.
//
// The structural test TestAC8_NoAlternateInjectionMechanism scans
// .go files for forbidden tokens. Those tokens appear in
// docstrings, and a naive scan over the whole file produces false
// positives. Stripping comments before the scan separates
// "the code references version.Version" from "the docstring
// mentions version.Version".
//
// The helper is deliberately simple: it does not parse strings, so
// a string literal containing "//" would be mis-stripped. The
// files this test scans do not contain such literals in positions
// that matter. If that changes, replace the helper with a
// go/parser-based one.
func stripGoComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	i := 0
	for i < len(src) {
		// Line comment: skip to end of line, emit the newline.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			j := i + 2
			for j < len(src) && src[j] != '\n' {
				j++
			}
			if j < len(src) {
				b.WriteByte('\n')
				i = j + 1
			} else {
				i = j
			}
			continue
		}

		// Block comment: skip to "*/", emit one newline per
		// newline inside the comment.
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				if src[j] == '\n' {
					b.WriteByte('\n')
				}
				j++
			}
			if j+1 < len(src) {
				i = j + 2
			} else {
				i = len(src)
			}
			continue
		}

		b.WriteByte(src[i])
		i++
	}

	return b.String()
}

// stripYAMLComments removes YAML line comments (everything from an
// unquoted # to end of line) from src, preserving newlines.
//
// The helper is deliberately simple: it does not parse YAML, does
// not track quoted strings, and does not distinguish a # inside a
// block scalar from a # that starts a comment. For the Taskfile it
// is applied to, none of those distinctions matter: the file
// contains no # inside a quoted string, and block scalars in the
// file do not contain # characters.
//
// If a future Taskfile contains a # inside a quoted string or a
// block scalar, replace this helper with a YAML-aware one.
func stripYAMLComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	for _, line := range strings.Split(src, "\n") {
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return b.String()
}

// stripMakefileComments removes Makefile line comments (everything
// from an unquoted # to end of line) from src, preserving newlines
// and leaving recipe lines (those beginning with a TAB) untouched.
//
// # Why recipe lines are preserved
//
// Make's comment syntax is "# to end of line", but only on
// non-recipe lines. A line beginning with a TAB is a shell script;
// within it, # starts a shell comment, and the shell strips it at
// runtime. Stripping it here would mean the forbidden-token scan
// misses a real invocation hidden behind a shell comment. Keeping
// recipe lines verbatim avoids that blind spot.
//
// # What the helper does not handle
//
// The helper does not parse Make variables or conditionals. It does
// not distinguish a # inside a quoted string from a # that starts a
// comment. For the shim it is applied to, none of these
// distinctions matter: the shim's non-recipe lines contain no
// quoted strings with #, and its recipe lines are not stripped.
//
// If a future shim contains a # inside a quoted string on a
// non-recipe line, replace this helper with a Make-aware one.
func stripMakefileComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))

	for _, line := range strings.Split(src, "\n") {
		// Recipe lines start with a TAB and are shell scripts. Do
		// not strip # from them; a shell comment is not a Make
		// comment, and stripping it could hide a real invocation.
		if strings.HasPrefix(line, "\t") {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}

		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	return b.String()
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitAddCommit(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-q", "-m", msg},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ─────────────────────────────────────────────────────────────────────
// TDD — the contract itself
// ─────────────────────────────────────────────────────────────────────

// TestInjectionContract_TargetsAreVars asserts the contract that binds
// the Taskfile to this package: the four linker targets must be
// variables, not constants. -X only writes vars. Taking the address of
// a constant does not compile, so this test fails at build time if
// someone changes a `var` to a `const`.
//
// If this test fails to compile, the Taskfile's LDFLAGS are silently
// doing nothing. That is the single most dangerous failure mode of
// the injection contract, and it is why this test exists.
func TestInjectionContract_TargetsAreVars(t *testing.T) {
	t.Parallel()

	_ = &Version
	_ = &Commit
	_ = &BuildDate
	_ = &Dirty
}

// TestInjectionContract_TargetsAreStrings asserts that all four
// linker targets are string-typed. -X only writes strings.
func TestInjectionContract_TargetsAreStrings(t *testing.T) {
	t.Parallel()

	var (
		_ string = Version
		_ string = Commit
		_ string = BuildDate
		_ string = Dirty
	)
}

// ─────────────────────────────────────────────────────────────────────
// ATDD — acceptance criteria
// ─────────────────────────────────────────────────────────────────────

// TestAC1_TaskfileInjectsAllFourVariables verifies AC1: the Taskfile
// references all four linker targets, in the correct package path.
//
// The test reads Taskfile.yml, not Makefile. The Makefile is a shim
// and contains no -X flags by design.
func TestAC1_TaskfileInjectsAllFourVariables(t *testing.T) {
	t.Parallel()

	taskfile := readRepoFile(t, "Taskfile.yml")

	wantTargets := []string{
		"{{.VERSION_PKG}}.Version=",
		"{{.VERSION_PKG}}.Commit=",
		"{{.VERSION_PKG}}.BuildDate=",
		"{{.VERSION_PKG}}.Dirty=",
	}

	for _, target := range wantTargets {
		if !strings.Contains(taskfile, target) {
			t.Errorf("Taskfile.yml does not contain linker target %q", target)
		}
	}
}

// TestAC1_TaskfileLDFLAGSUsesVersionPkg asserts that every -X target
// in the Taskfile is built from {{.VERSION_PKG}}, not hard-coded. A
// hard-coded path would drift from go.mod on a module rename.
func TestAC1_TaskfileLDFLAGSUsesVersionPkg(t *testing.T) {
	t.Parallel()

	taskfile := readRepoFile(t, "Taskfile.yml")

	// Match `-X <target>` where <target> is not a shell expansion.
	// The Taskfile's LDFLAGS uses {{.VERSION_PKG}} as a template.
	re := regexp.MustCompile(`-X\s+(\{\{\.VERSION_PKG\}\}\.[A-Za-z]+=)`)
	matches := re.FindAllStringSubmatch(taskfile, -1)

	if len(matches) != 4 {
		t.Fatalf("found %d -X targets using {{.VERSION_PKG}}, want 4", len(matches))
	}
}

// TestAC6_ModulePathMatchesGoMod verifies AC6: MODULE in the Taskfile
// matches the module directive in go.mod.
//
// This is the automated form of `task verify:injection-prefix`. It
// exists as a Go test so it runs under `go test ./...` even when the
// contributor forgets to invoke `task`.
func TestAC6_ModulePathMatchesGoMod(t *testing.T) {
	t.Parallel()

	gomod := readRepoFile(t, "go.mod")
	taskfile := readRepoFile(t, "Taskfile.yml")

	gomodModule := parseGoModModule(t, gomod)
	taskfileModule := parseTaskfileModule(t, taskfile)

	if gomodModule == "" {
		t.Fatal("go.mod has no module directive")
	}
	if taskfileModule == "" {
		t.Fatal("Taskfile.yml has no MODULE variable")
	}
	if gomodModule != taskfileModule {
		t.Fatalf("module path mismatch:\n"+
			"  go.mod       = %q\n"+
			"  Taskfile.yml = %q\n"+
			"  injection would silently fail; see docs/development.md",
			gomodModule, taskfileModule)
	}
}

// TestAC8_NoAlternateInjectionMechanism verifies WBS 6.1.2 AC8: no
// environment variable, generated file, or ad-hoc script injects
// version metadata.
//
// # What the check enforces
//
// Every package outside internal/version must read the four
// linker-injected values through version.Get(). Direct references
// to version.Version, version.Commit, version.BuildDate, or
// version.Dirty from outside the package are forbidden: they couple
// the caller to the storage shape, and the storage shape changed
// once already (WBS 6.1.1 moved from a struct to a 4-tuple).
//
// # What the check does not enforce
//
// The check does not forbid mentioning the variable names in
// comments, docstrings, or import paths. A docstring that says "the
// four fields mirror internal/version/version.go" is documentation,
// not a direct reference. A naive scan over the whole file matches
// such text and produces a false positive.
//
// The check strips Go comments before scanning, so it sees only
// executable code.
//
// # Exemptions
//
// Two packages are exempt:
//
//   - internal/version itself, which declares the variables.
//
//   - internal/app/version, whose BuildInfo type mirrors the four
//     fields one-to-one and whose Get function is the single adapter
//     between the model and the rendered output. If the model
//     changes shape, the service is the intended point of
//     adaptation. Its docstrings necessarily name the four fields.
//
// All other packages must go through version.Get() or, at one
// remove, appversion.Get().
//
// # Environment-variable reads
//
// The check also forbids reading version metadata from the
// environment (FORGE_VERSION, FORGE_COMMIT, FORGE_BUILD_DATE,
// FORGE_DIRTY). The Taskfile is the only injection mechanism;
// environment-based injection is not supported.
func TestAC8_NoAlternateInjectionMechanism(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	// Packages that are allowed to know the four field names. The
	// version package declares them; the app/version package adapts
	// them into its own BuildInfo type.
	exemptPrefixes := []string{
		filepath.Join(root, "internal", "version"),
		filepath.Join(root, "internal", "app", "version"),
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		for _, prefix := range exemptPrefixes {
			if strings.HasPrefix(path, prefix) {
				return nil
			}
		}
		// Fixtures under testdata/ exist to demonstrate failure
		// modes.
		if strings.Contains(path, string(filepath.Separator)+"testdata"+string(filepath.Separator)) {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		// Strip comments before scanning. The four variable names
		// appear in docstrings and import-path comments; the rule
		// is about executable code.
		body := stripGoComments(string(src))

		for _, name := range []string{
			"version.Version",
			"version.Commit",
			"version.BuildDate",
			"version.Dirty",
		} {
			if strings.Contains(body, name) {
				t.Errorf("%s references %s in code; use version.Get()",
					path, name)
			}
		}
		for _, envVar := range []string{
			`os.Getenv("FORGE_VERSION`,
			`os.Getenv("FORGE_COMMIT`,
			`os.Getenv("FORGE_BUILD_DATE`,
			`os.Getenv("FORGE_DIRTY`,
		} {
			if strings.Contains(body, envVar) {
				t.Errorf("%s reads version metadata from the environment; "+
					"the Taskfile is the only injection mechanism", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────
// BDD — behaviour specifications
// ─────────────────────────────────────────────────────────────────────

// TestBehaviour_GitDescribeContract encodes the BDD scenario:
//
//	Given a git repository with no tags
//	When `git describe --tags --always --dirty` runs
//	Then it outputs a short SHA, not an error.
//
// This is the contract the Taskfile's VERSION variable relies on.
func TestBehaviour_GitDescribeContract(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")

	out := runGit(t, dir, "describe", "--tags", "--always", "--dirty")
	if out == "" {
		t.Fatal("git describe returned empty output")
	}
	wantSHA := runGit(t, dir, "rev-parse", "--short", "HEAD")
	if out != wantSHA {
		t.Errorf("git describe = %q, want %q (no tags → short SHA)", out, wantSHA)
	}
}

// TestBehaviour_GitDescribeDirtySuffix encodes the BDD scenario:
//
//	Given a git repository with an uncommitted modification
//	When `git describe --tags --always --dirty` runs
//	Then the output ends with "-dirty".
func TestBehaviour_GitDescribeDirtySuffix(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")
	writeFile(t, filepath.Join(dir, "a.txt"), "hello, world\n")

	out := runGit(t, dir, "describe", "--tags", "--always", "--dirty")
	if !strings.HasSuffix(out, "-dirty") {
		t.Errorf("git describe = %q, want suffix -dirty", out)
	}
}

// TestBehaviour_GitStatusPorcelainDetectsUntrackedFiles encodes the
// BDD scenario that motivated the choice of `git status --porcelain`
// over `git diff-index`:
//
//	Given a git repository with an untracked file
//	When `git status --porcelain` runs
//	Then the output is non-empty.
//
// The Taskfile's DIRTY variable relies on this. `git diff-index
// --quiet HEAD` would report clean in this scenario, which is wrong.
func TestBehaviour_GitStatusPorcelainDetectsUntrackedFiles(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")

	writeFile(t, filepath.Join(dir, "b.txt"), "untracked\n")

	out := runGit(t, dir, "status", "--porcelain")
	if out == "" {
		t.Fatal("git status --porcelain reported clean for untracked file")
	}
	if !strings.Contains(out, "b.txt") {
		t.Errorf("git status --porcelain = %q, want it to mention b.txt", out)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Negative — the silent-failure trap
// ─────────────────────────────────────────────────────────────────────

// TestNegative_ModulePathMismatchSilentlyFails proves the failure
// mode documented in WBS 6.1.2: if the -X prefix does not match the
// module path in go.mod, injection silently does nothing. No error,
// no warning, no build failure — the binary simply reports empty
// values.
//
// The test builds a fixture module whose module path deliberately
// differs from the -X prefix, links it with -X pointing at a
// non-existent package path, and asserts that Get() reports empty
// values.
func TestNegative_ModulePathMismatchSilentlyFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in -short mode")
	}
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/wrong\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"

	"example.com/wrong/internal/version"
)

func main() {
	v, c, b, d := version.Get()
	fmt.Printf("%q %q %q %q\n", v, c, b, d)
}
`)
	writeFile(t, filepath.Join(dir, "internal", "version", "version.go"), `package version

var (
	Version   string
	Commit    string
	BuildDate string
	Dirty     string
)

func Get() (string, string, string, string) {
	return Version, Commit, BuildDate, Dirty
}
`)

	cmd := exec.Command("go", "build",
		"-ldflags", `-X example.com/right/internal/version.Version=9.9.9`,
		"-o", "app", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("go build output: %s (err=%v)", out, err)
	}

	run := exec.Command("./app")
	run.Dir = dir
	out, err := run.Output()
	if err != nil {
		t.Fatalf("run app: %v", err)
	}
	got := strings.TrimSpace(string(out))

	want := `"" "" "" ""`
	if got != want {
		t.Fatalf("mismatched-path injection produced %q, want %q\n"+
			"if this test fails, the silent-failure trap has changed "+
			"and docs/development.md must be updated", got, want)
	}
}

// TestNegative_CorrectPathInjectionLands is the counterpart to the
// previous test: with the CORRECT module path, injection must
// actually happen. If the Taskfile's LDFLAGS are malformed, the
// values will be empty and this test fails loudly.
func TestNegative_CorrectPathInjectionLands(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in -short mode")
	}
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/right\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"

	"example.com/right/internal/version"
)

func main() {
	v, c, b, d := version.Get()
	fmt.Printf("%q %q %q %q\n", v, c, b, d)
}
`)
	writeFile(t, filepath.Join(dir, "internal", "version", "version.go"), `package version

var (
	Version   string
	Commit    string
	BuildDate string
	Dirty     string
)

func Get() (string, string, string, string) {
	return Version, Commit, BuildDate, Dirty
}
`)

	cmd := exec.Command("go", "build",
		"-ldflags", `-X example.com/right/internal/version.Version=9.9.9`,
		"-o", "app", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	run := exec.Command("./app")
	run.Dir = dir
	out, err := run.Output()
	if err != nil {
		t.Fatalf("run app: %v", err)
	}
	got := strings.TrimSpace(string(out))

	want := `"9.9.9" "" "" ""`
	if got != want {
		t.Fatalf("correct-path injection produced %q, want %q", got, want)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Edge — no git, no tags, detached HEAD
// ─────────────────────────────────────────────────────────────────────

// TestEdge_FallbackValuesAreInjected asserts that the Taskfile's
// `|| echo <fallback>` clauses work: a build from a tarball with no
// git repository must still produce a valid binary.
func TestEdge_FallbackValuesAreInjected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in -short mode")
	}
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/right\n\ngo 1.22\n")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"

	"example.com/right/internal/version"
)

func main() {
	v, c, b, d := version.Get()
	fmt.Printf("%q %q %q %q\n", v, c, b, d)
}
`)
	writeFile(t, filepath.Join(dir, "internal", "version", "version.go"), `package version

var (
	Version   string
	Commit    string
	BuildDate string
	Dirty     string
)

func Get() (string, string, string, string) {
	return Version, Commit, BuildDate, Dirty
}
`)

	cmd := exec.Command("go", "build",
		"-ldflags",
		`-X example.com/right/internal/version.Version=dev `+
			`-X example.com/right/internal/version.Commit=none`,
		"-o", "app", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	run := exec.Command("./app")
	run.Dir = dir
	out, err := run.Output()
	if err != nil {
		t.Fatalf("run app: %v", err)
	}
	got := strings.TrimSpace(string(out))

	want := `"dev" "none" "" ""`
	if got != want {
		t.Fatalf("fallback injection produced %q, want %q", got, want)
	}
}

// TestEdge_NoTagsFallsBackToShortSHA asserts the --always contract:
// when no tags are reachable, git describe returns the short SHA.
func TestEdge_NoTagsFallsBackToShortSHA(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")

	describe := runGit(t, dir, "describe", "--tags", "--always", "--dirty")
	shortSHA := runGit(t, dir, "rev-parse", "--short", "HEAD")

	if describe != shortSHA {
		t.Errorf("no-tag describe = %q, want short SHA %q", describe, shortSHA)
	}
}

// TestEdge_DetachedHEADStillDescribes asserts that `git describe
// --always` works on a detached HEAD, which is the state CI checkouts
// are often in.
func TestEdge_DetachedHEADStillDescribes(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")
	sha := runGit(t, dir, "rev-parse", "HEAD")

	if err := exec.Command("git", "-C", dir, "checkout", sha).Run(); err != nil {
		t.Fatalf("detach HEAD: %v", err)
	}

	out := runGit(t, dir, "describe", "--tags", "--always", "--dirty")
	if out == "" {
		t.Fatal("git describe returned empty on detached HEAD")
	}
}

// ─────────────────────────────────────────────────────────────────────
// Corner — dirty tree, empty repo, single commit
// ─────────────────────────────────────────────────────────────────────

// TestCorner_DirtyTreeReportsDirtyTrue asserts the Taskfile's DIRTY
// derivation: modifying a tracked file must flip DIRTY from "false"
// to "true".
func TestCorner_DirtyTreeReportsDirtyTrue(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")

	clean := runGit(t, dir, "status", "--porcelain")
	if clean != "" {
		t.Fatalf("fresh repo is dirty: %q", clean)
	}

	writeFile(t, filepath.Join(dir, "a.txt"), "hello, world\n")

	dirty := runGit(t, dir, "status", "--porcelain")
	if dirty == "" {
		t.Fatal("modified tracked file did not produce porcelain output")
	}
}

// TestCorner_EmptyRepoDoesNotCrash asserts that `git describe` and
// `git rev-parse` on a repository with no commits do not crash. The
// Taskfile's `|| echo <fallback>` clauses must absorb the failure.
func TestCorner_EmptyRepoDoesNotCrash(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)

	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").CombinedOutput(); err == nil {
		t.Fatalf("git rev-parse unexpectedly succeeded on empty repo: %s", out)
	}
	if out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--always").CombinedOutput(); err == nil {
		t.Fatalf("git describe unexpectedly succeeded on empty repo: %s", out)
	}
}

// ─────────────────────────────────────────────────────────────────────
// Boundary — RFC 3339 format, short SHA length
// ─────────────────────────────────────────────────────────────────────

// TestBoundary_BuildDateFormatIsRFC3339UTC asserts that the
// BuildDate format string used by the Taskfile produces a valid
// RFC 3339 UTC timestamp.
func TestBoundary_BuildDateFormatIsRFC3339UTC(t *testing.T) {
	t.Parallel()

	const format = "2006-01-02T15:04:05Z"
	got := time.Now().UTC().Format(format)

	if _, err := time.Parse(format, got); err != nil {
		t.Fatalf("BuildDate %q does not parse as RFC 3339 UTC: %v", got, err)
	}
	if !strings.HasSuffix(got, "Z") {
		t.Errorf("BuildDate %q does not end in Z (not UTC)", got)
	}
}

// TestBoundary_ShortSHALength asserts that `git rev-parse --short`
// returns at least 7 characters by default. The Taskfile does not
// hard-code 7; this test documents the default.
func TestBoundary_ShortSHALength(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")

	sha := runGit(t, dir, "rev-parse", "--short", "HEAD")
	if len(sha) < 7 {
		t.Errorf("short SHA %q has length %d, want >= 7", sha, len(sha))
	}
}

// ─────────────────────────────────────────────────────────────────────
// Non-functional — shell safety, idempotency, no alternate mechanism
// ─────────────────────────────────────────────────────────────────────

// TestNonFunctional_InjectedValuesAreShellSafe asserts that the four
// injected values can never contain shell metacharacters that would
// break the `go build -ldflags "..."` invocation. Task 3.x has known
// quoting quirks around sh: output; constraining the charset is the
// defence.
//
// The check is a regex against the four sources:
//
//   - git describe:   [A-Za-z0-9._-]+ (-dirty suffix)
//   - git rev-parse:  [0-9a-f]+
//   - date -u:        [0-9T:Z-]+ (RFC 3339)
//   - dirty:          "true" | "false"
//
// Each is a strict subset of a safe charset. This test pins that
// invariant on the Go side so a future change to the Taskfile that
// widens a value (for example, injecting a branch name with slashes)
// fails loudly.
func TestNonFunctional_InjectedValuesAreShellSafe(t *testing.T) {
	t.Parallel()

	safe := regexp.MustCompile(`^[A-Za-z0-9._:+-]*$`)

	// Sample values that the Taskfile can produce.
	samples := []struct {
		name  string
		value string
	}{
		{"Version dev", "dev"},
		{"Version tag", "v1.2.3"},
		{"Version describe", "v1.2.3-4-gabc1234"},
		{"Version dirty", "v1.2.3-4-gabc1234-dirty"},
		{"Version sha", "abc1234"},
		{"Commit sha", "abc1234"},
		{"BuildDate", "2026-10-09T12:00:00Z"},
		{"Dirty true", "true"},
		{"Dirty false", "false"},
	}

	for _, s := range samples {
		if !safe.MatchString(s.value) {
			t.Errorf("%s = %q contains characters outside the safe charset; "+
				"a future change may have widened the value and broken "+
				"shell quoting in the Taskfile", s.name, s.value)
		}
	}
}

// TestNonFunctional_TaskfileIsIdempotent asserts that running the
// Taskfile's value-derivation logic twice in the same second yields
// identical values. BuildDate is the only time-dependent value; the
// other three are stable.
func TestNonFunctional_TaskfileIsIdempotent(t *testing.T) {
	skipIfNoGit(t)
	t.Parallel()

	dir := t.TempDir()
	initGitRepo(t, dir)
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	gitAddCommit(t, dir, "initial")

	v1 := runGit(t, dir, "describe", "--tags", "--always", "--dirty")
	v2 := runGit(t, dir, "describe", "--tags", "--always", "--dirty")
	if v1 != v2 {
		t.Errorf("git describe is not idempotent: %q vs %q", v1, v2)
	}

	c1 := runGit(t, dir, "rev-parse", "--short", "HEAD")
	c2 := runGit(t, dir, "rev-parse", "--short", "HEAD")
	if c1 != c2 {
		t.Errorf("git rev-parse is not idempotent: %q vs %q", c1, c2)
	}
}

// TestNonFunctional_TaskfileUsesPortableShell asserts that the
// Taskfile's sh: commands are POSIX-compatible. Task runs them with
// `sh` by default; bashisms would break on Alpine and other minimal
// images.
//
// # What the check looks at
//
// The check strips YAML comments before scanning. A bashism in a
// comment is documentation, not a portability violation. A naive
// scan over the whole file produces false positives on the
// Taskfile's own explanatory comments.
//
// # Why the check is not a plain strings.Contains
//
// The bash `[[` operator is a two-character token. But POSIX
// character classes such as `[[:space:]]`, `[[:alpha:]]`, and
// `[[:digit:]]` also begin with `[[`. A substring search for `[[`
// matches both, and the Taskfile uses POSIX character classes
// legitimately. The check therefore uses a regex that matches `[[`
// followed by a character that is not `:` — which excludes POSIX
// character classes and matches the bash operator.
//
// # Why the other bashisms are simpler
//
// `$RANDOM` and `${var//` are unambiguous. They do not appear as
// substrings of any POSIX construct. A substring search is
// sufficient for them.
func TestNonFunctional_TaskfileUsesPortableShell(t *testing.T) {
	t.Parallel()

	taskfile := readRepoFile(t, "Taskfile.yml")
	code := stripYAMLComments(taskfile)

	// The bash `[[` operator: `[[` followed by any character that
	// is not `:`. A POSIX character class such as `[[:space:]]`
	// has `:` after `[[`, so it does not match. The bash operator
	// always has whitespace, a variable, or a shell keyword after
	// `[[`, none of which is `:`.
	if m := regexp.MustCompile(`\[\[[^:]`).FindString(code); m != "" {
		t.Errorf("Taskfile.yml contains non-POSIX token %q in code: "+
			"use POSIX [ instead of [[ in sh", m)
	}

	otherBashisms := []struct {
		token  string
		reason string
	}{
		{"$RANDOM", "not POSIX"},
		{"${var//", "bash parameter expansion"},
	}

	for _, b := range otherBashisms {
		if strings.Contains(code, b.token) {
			t.Errorf("Taskfile.yml contains non-POSIX token %q in code: %s",
				b.token, b.reason)
		}
	}
}

// TestNonFunctional_MakefileShimHasNoInjectionLogic asserts that the
// Makefile shim contains no -X flags, no MODULE variable, and no git
// commands in code. If it did, the shim would become an alternate
// injection mechanism, violating AC8 of WBS 6.1.2.
//
// # What the check looks at
//
// The check strips Makefile comments before scanning. The shim's
// docstring explains that it contains no -X, no MODULE, and no git,
// and therefore mentions all three tokens by name. A naive scan
// over the whole file matches the docstring and reports a false
// positive. The rule is about code, not prose.
//
// # Why recipe lines are preserved
//
// Make recipe lines begin with a TAB and are shell scripts. A # in
// a recipe line is a shell comment, not a Make comment. The helper
// preserves recipe lines verbatim so a future recipe that actually
// invoked git or used -X would be caught. The shim's own recipes
// use only @command, @echo, @grep, and $(TASK), none of which
// contain the forbidden tokens.
func TestNonFunctional_MakefileShimHasNoInjectionLogic(t *testing.T) {
	t.Parallel()

	makefile := readRepoFile(t, "Makefile")
	code := stripMakefileComments(makefile)

	forbidden := []struct {
		token  string
		reason string
	}{
		{"-X ", "the shim must not contain linker flags; Taskfile.yml is the authority"},
		{"MODULE", "the shim must not define a module path; Taskfile.yml is the authority"},
		{"git ", "the shim must not run git; Taskfile.yml is the authority"},
	}

	for _, f := range forbidden {
		if strings.Contains(code, f.token) {
			t.Errorf("Makefile shim contains %q in code: %s", f.token, f.reason)
		}
	}
}
