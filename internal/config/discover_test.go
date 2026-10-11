// Package config contains unit tests for the CLI configuration
// type.
//
// This file pins the configuration file discovery algorithm (WBS
// 8.4.1). The algorithm's tests use a synthetic env function and
// temporary directories for the file-existence checks; the tests
// do not read the real environment or the real working directory.
//
// # How the tests work
//
// Each test constructs an env function that returns the values the
// test wants, constructs a cwd (a temporary directory), creates
// the files the test wants to exist, and calls Discover. The tests
// assert the return value and (where applicable) the error.
//
// # Platform awareness
//
// The Windows-specific candidate is tested only on Windows; the
// test uses runtime.GOOS to skip on other platforms. The other
// candidates are platform-neutral (their paths are joined with
// the platform's separator by filepath.Join).
package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// Test helpers
// =============================================================================

// envFunc returns a function that looks up names in m. It is the
// synthetic env function the tests pass to Discover.
//
// # Why a synthetic env
//
// The real os.Getenv reads the process's environment. A test that
// used it would depend on the environment's state, which is not
// reproducible. The synthetic function lets the test control the
// values Discover sees.
func envFunc(m map[string]string) func(string) string {
	return func(name string) string {
		return m[name]
	}
}

// writeTempFile creates a file at path with the given content and
// fails the test if the creation errors.
//
// The helper is used by the tests that need an existing file at a
// candidate path. The file's content is irrelevant to Discover;
// the file's existence is what matters.
func writeTempFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// =============================================================================
// Explicit sources
// =============================================================================

// TestDiscover_ExplicitTakesPriority verifies that the explicit
// argument (the --config flag's value) is the highest-priority
// candidate.
//
// The test creates a file at the explicit path and at every other
// candidate path; the explicit path wins.
func TestDiscover_ExplicitTakesPriority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	explicit := filepath.Join(dir, "explicit.yaml")
	writeTempFile(t, explicit)

	// Create files at every other candidate so that a
	// wrong-order algorithm would pick a different one.
	envDir := filepath.Join(dir, "env")
	writeTempFile(t, filepath.Join(envDir, xdgConfigRelPath))
	writeTempFile(t, filepath.Join(envDir, homeConfigRelPath))
	writeTempFile(t, filepath.Join(dir, projectLocalRelPath))

	env := envFunc(map[string]string{
		EnvVarConfigPath:  filepath.Join(envDir, "from-env.yaml"),
		"XDG_CONFIG_HOME": envDir,
		"HOME":            envDir,
	})

	got, err := Discover(explicit, env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != explicit {
		t.Errorf("Discover returned %q; want %q", got, explicit)
	}
}

// TestDiscover_EnvTakesSecondPriority verifies that the
// FORGE_CONFIG environment variable is the second-priority
// candidate.
//
// The test does not supply an explicit argument; the env var's
// path wins over the optional candidates.
func TestDiscover_EnvTakesSecondPriority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envPath := filepath.Join(dir, "from-env.yaml")
	writeTempFile(t, envPath)

	// Create files at the optional candidates.
	writeTempFile(t, filepath.Join(dir, projectLocalRelPath))

	env := envFunc(map[string]string{
		EnvVarConfigPath: envPath,
	})

	got, err := Discover("", env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != envPath {
		t.Errorf("Discover returned %q; want %q", got, envPath)
	}
}

// =============================================================================
// Optional sources
// =============================================================================

// TestDiscover_ProjectLocalTakesThirdPriority verifies that the
// project-local path is the third-priority candidate.
func TestDiscover_ProjectLocalTakesThirdPriority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	projectPath := filepath.Join(dir, projectLocalRelPath)
	writeTempFile(t, projectPath)

	env := envFunc(map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(dir, "xdg-never-exists"),
		"HOME":            filepath.Join(dir, "home-never-exists"),
	})

	got, err := Discover("", env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != projectPath {
		t.Errorf("Discover returned %q; want %q", got, projectPath)
	}
}

// TestDiscover_XDGTakesFourthPriority verifies that the XDG path
// is the fourth-priority candidate.
func TestDiscover_XDGTakesFourthPriority(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	xdgDir := filepath.Join(dir, "xdg")
	xdgPath := filepath.Join(xdgDir, xdgConfigRelPath)
	writeTempFile(t, xdgPath)

	// The project-local path is not created; the XDG path wins.
	env := envFunc(map[string]string{
		"XDG_CONFIG_HOME": xdgDir,
		"HOME":            filepath.Join(dir, "home-never-exists"),
	})

	got, err := Discover("", env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != xdgPath {
		t.Errorf("Discover returned %q; want %q", got, xdgPath)
	}
}

// TestDiscover_HomeFallback verifies that the HOME fallback is the
// fifth-priority candidate.
func TestDiscover_HomeFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")
	homePath := filepath.Join(homeDir, homeConfigRelPath)
	writeTempFile(t, homePath)

	// XDG is not set; the HOME fallback applies.
	env := envFunc(map[string]string{
		"HOME": homeDir,
	})

	got, err := Discover("", env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != homePath {
		t.Errorf("Discover returned %q; want %q", got, homePath)
	}
}

// TestDiscover_WindowsAppData verifies that the Windows %APPDATA%
// path is the sixth-priority candidate. The test is skipped on
// non-Windows platforms.
func TestDiscover_WindowsAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only candidate; skipping on " + runtime.GOOS)
	}
	t.Parallel()

	dir := t.TempDir()
	appDataDir := filepath.Join(dir, "appdata")
	appDataPath := filepath.Join(appDataDir, appDataConfigRelPath)
	writeTempFile(t, appDataPath)

	env := envFunc(map[string]string{
		"APPDATA": appDataDir,
	})

	got, err := Discover("", env, "")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != appDataPath {
		t.Errorf("Discover returned %q; want %q", got, appDataPath)
	}
}

// =============================================================================
// No candidate
// =============================================================================

// TestDiscover_NoCandidateReturnsEmpty verifies that the function
// returns ("", nil) when no candidate exists.
func TestDiscover_NoCandidateReturnsEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := envFunc(map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(dir, "xdg-never-exists"),
		"HOME":            filepath.Join(dir, "home-never-exists"),
	})

	got, err := Discover("", env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != "" {
		t.Errorf("Discover returned %q; want empty", got)
	}
}

// =============================================================================
// Explicit sources with missing files
// =============================================================================

// TestDiscover_ExplicitMissingFileErrors verifies that Discover
// returns an E1 error when the explicit path names a missing
// file.
//
// # Why the sentinel is ErrConfigNotFound, not ErrExplicitConfigMissing
//
// The config error taxonomy (WBS 8.5.1) defines E1
// (ErrConfigNotFound) as the sentinel for a missing config file.
// An earlier draft of the discovery algorithm returned
// ErrExplicitConfigMissing for the explicit-source case; the
// draft predated the taxonomy. The current implementation
// returns the taxonomy's E1 error (via NewConfigNotFound), and
// the test checks for that sentinel.
//
// The behaviour is unchanged: a missing explicit path is an
// error, and the error names the path. Only the sentinel the
// test compares against has changed.
func TestDiscover_ExplicitMissingFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.yaml")

	_, err := Discover(missing, func(string) string { return "" }, dir)
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("Discover: err = %v; want ErrConfigNotFound", err)
	}
}

// TestDiscover_EnvMissingFileErrors verifies that Discover
// returns an E1 error when the FORGE_CONFIG environment variable
// names a missing file.
//
// See TestDiscover_ExplicitMissingFileErrors for the rationale
// on the sentinel.
func TestDiscover_EnvMissingFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "env-does-not-exist.yaml")

	env := func(key string) string {
		if key == EnvVarConfigPath {
			return missing
		}
		return ""
	}

	_, err := Discover("", env, dir)
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("Discover: err = %v; want ErrConfigNotFound", err)
	}
}

// =============================================================================
// Directory is not a file
// =============================================================================

// TestDiscover_DirectoryIsNotAFile verifies that Discover treats
// an explicit path that names a directory as a missing file.
//
// # Why a directory is not a valid config file
//
// The discovery algorithm's job is to find a config file. A
// directory is not a file; os.Stat's IsRegular check rejects it.
// The explicit source treats the rejection as an error, same as
// a missing file: the user named a path, and the path does not
// name a file.
//
// # The sentinel
//
// The error is the taxonomy's E1 (ErrConfigNotFound). See
// TestDiscover_ExplicitMissingFileErrors for the rationale.
func TestDiscover_DirectoryIsNotAFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	explicitDir := filepath.Join(dir, "explicit-dir")
	if err := os.Mkdir(explicitDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := Discover(explicitDir, func(string) string { return "" }, dir)
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("explicit directory: err = %v; want ErrConfigNotFound", err)
	}
}

// =============================================================================
// Symlinks
// =============================================================================

// TestDiscover_SymlinkIsFollowed verifies that a symlink to an
// existing file is treated as the file.
func TestDiscover_SymlinkIsFollowed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target.yaml")
	writeTempFile(t, target)

	link := filepath.Join(dir, "link.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	got, err := Discover(link, envFunc(nil), "")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != link {
		t.Errorf("Discover returned %q; want %q", got, link)
	}
}

// =============================================================================
// Purity
// =============================================================================

// TestDiscover_Pure verifies that the function reads only its
// arguments.
//
// # How the check works
//
// The test reads discover.go and asserts that the source does not
// reference os.Getenv, os.Getwd, or os.Args outside comments. The
// check is textual; a future contributor who adds a direct read of
// the process's environment sees the failure and either removes
// the read or routes the value through the function's arguments.
//
// # What the check does not catch
//
// The check does not catch every way of reading global state (for
// example, a helper function in another file that reads
// os.Getenv). The check's scope is discover.go; the package's
// other files are the caller's responsibility.
func TestDiscover_Pure(t *testing.T) {
	t.Parallel()

	src := readFile(t, "discover.go")
	code := stripComments(src)

	for _, forbidden := range []string{
		"os.Getenv",
		"os.Getwd",
		"os.Args",
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("discover.go references %s; "+
				"the function must read only its arguments "+
				"(WBS 8.4.1)", forbidden)
		}
	}
}

// =============================================================================
// The algorithm's order
// =============================================================================

// TestDiscover_FirstExistingWins verifies that the first existing
// candidate wins, not the first specified.
//
// The test creates files at two candidates: the project-local path
// and the HOME fallback. The project-local path is earlier in the
// algorithm; it wins.
func TestDiscover_FirstExistingWins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := filepath.Join(dir, "home")
	homePath := filepath.Join(homeDir, homeConfigRelPath)
	writeTempFile(t, homePath)

	projectPath := filepath.Join(dir, projectLocalRelPath)
	writeTempFile(t, projectPath)

	env := envFunc(map[string]string{
		"HOME": homeDir,
	})

	got, err := Discover("", env, dir)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if got != projectPath {
		t.Errorf("Discover returned %q; want %q (project-local wins)",
			got, projectPath)
	}
}

// =============================================================================
// The platformDiscoveryPaths helper
// =============================================================================

// TestDiscover_PlatformPathsOrder verifies that the
// platformDiscoveryPaths helper returns the candidates in the
// algorithm's order.
func TestDiscover_PlatformPathsOrder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	env := envFunc(map[string]string{
		EnvVarConfigPath:  "/explicit/env",
		"XDG_CONFIG_HOME": "/xdg",
		"HOME":            "/home",
	})

	paths := platformDiscoveryPaths("/explicit/flag", env, dir)

	// The first two candidates are the explicit sources.
	if len(paths) < 2 {
		t.Fatalf("platformDiscoveryPaths returned %d paths; want at least 2",
			len(paths))
	}
	if paths[0] != "/explicit/flag" {
		t.Errorf("paths[0] = %q; want the explicit flag", paths[0])
	}
	if paths[1] != "/explicit/env" {
		t.Errorf("paths[1] = %q; want the env var", paths[1])
	}
}
