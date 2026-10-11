// Package config defines the CLI configuration for Forge.
//
// This file holds the configuration file discovery algorithm (WBS
// 8.4.1). The algorithm finds the path of the config file to load.
// It does not read the file's contents; the loader (WBS 8.4.2)
// reads the file and constructs a Config from it.
//
// # The discovery algorithm
//
// The algorithm tries the following paths in order, stopping at
// the first existing file:
//
//	#  Source                                Condition
//	1  --config <path> flag                   highest priority, explicit
//	2  FORGE_CONFIG environment variable      explicit
//	3  ./forge.yaml                           project-local (canonical)
//	4  ./.forge/config.yaml                   project-local (legacy)
//	5  $XDG_CONFIG_HOME/forge/config.yaml     if XDG is set
//	6  $HOME/.config/forge/config.yaml        HOME fallback
//	7  %APPDATA%\forge\config.yaml            Windows only
//	8  (none)                                 falls back to defaults
//
// An explicit source (1 or 2) that names a non-existent path
// produces an error. An optional source (3 through 7) that is not
// found is skipped silently; the algorithm continues to the next
// candidate.
//
// # Why the first existing file wins
//
// The algorithm's order is the precedence for discovery. A user
// who sets --config expects the flag to win; a user who sets
// FORGE_CONFIG expects the variable to win. The order matches the
// precedence order of WBS 8.1.2's configuration chain, except that
// the chain's first three layers (flag, env, file) are collapsed
// into the discovery algorithm's first two sources; the algorithm
// finds the file, and the loader applies the chain to the file's
// values.
//
// # Why the canonical project-local path is forge.yaml
//
// The project's config file is named forge.yaml and lives at the
// project root, alongside the project's other top-level files.
// The name matches the spec document (docs/forge-yaml-spec.md)
// and the Forge CLI's own convention. The `.forge/` directory
// holds other Forge-related state (state.yaml, backups); the
// config file does not live inside it in the current spec.
//
// # Why the legacy path is retained
//
// An earlier draft of the discovery algorithm looked for the
// config at `.forge/config.yaml`. A project that has a file at
// that path (from an earlier version of Forge) continues to load
// it. The legacy path is tried after the canonical path; a
// project with both files loads the canonical one.
//
// The legacy path is a one-release compatibility shim. A future
// WBS item removes it once no project needs it.
//
// # Why an explicit source errors on a missing file
//
// A user who types `forge --config /path/to/config.yaml` expects
// the file at that path to be loaded. If the file does not exist,
// the user has made a mistake; silently falling back to a
// different config would hide the mistake. The error is the
// correct response.
//
// # Why an optional source does not error
//
// A user who has not set --config or FORGE_CONFIG has not named a
// file; the algorithm's optional sources are defaults. A missing
// file in a default source is normal; the algorithm continues to
// the next candidate. If no candidate exists, the loader uses the
// built-in defaults (WBS 8.3.1).
//
// # Purity
//
// The function is pure with respect to its inputs. It reads only
// the three arguments (explicit, env, cwd). It does not call
// os.Getenv, os.Getwd, or os.Args. Its only I/O is os.Stat on
// each candidate path; the stat is a read-only existence check.
//
// # No package-level mutable state
//
// This file declares functions and constants. It declares no
// package-level variables.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrExplicitConfigMissing is the sentinel error returned when an
// explicit source (the --config flag or the FORGE_CONFIG
// environment variable) names a path that does not exist.
//
// # Why a sentinel
//
// The loader (WBS 8.4.2) and the CLI handler distinguish "explicit
// path missing" from other discovery failures. The sentinel lets
// a caller test for the condition with errors.Is.
//
// # Why not a typed error
//
// The ForgeError type (WBS 10.0) is not yet introduced. A sentinel
// is the Phase 2 mechanism; a future WBS item wraps the sentinel
// in a ForgeError when the type lands.
//
// # Relationship to the config error taxonomy
//
// The config error taxonomy (WBS 8.5.1) defines E1
// (ErrConfigNotFound) as the sentinel for a missing config file.
// ErrExplicitConfigMissing predates the taxonomy: WBS 8.4.1
// introduced it. A future WBS item replaces it with E1 (calling
// NewConfigNotFound(path)) so that the discovery error is
// classified by the same taxonomy the loader uses. For Phase 2,
// the two sentinels coexist; a caller that wants to catch "an
// explicit path is missing" checks ErrExplicitConfigMissing, and
// a caller that wants to catch "any config file is missing"
// checks ErrConfigNotFound (from errors.go).
var ErrExplicitConfigMissing = errors.New("config: explicit config file not found")

// projectLocalRelPath is the project-local config file's path,
// relative to the working directory.
//
// # Why forge.yaml
//
// The project's config file is named forge.yaml and lives at the
// project root, alongside the project's other top-level files.
// The filename matches the spec document (docs/forge-yaml-spec.md)
// and the Forge CLI's own convention.
//
// # Path style
//
// The constant uses forward slashes. The Discover function
// converts the value with filepath.FromSlash before joining, so
// the constant is portable across operating systems.
const projectLocalRelPath = "forge.yaml"

// projectLocalLegacyRelPath is the legacy project-local config
// file's path, relative to the working directory.
//
// # Why the legacy path is retained
//
// An earlier draft of the discovery algorithm looked for the
// config at `.forge/config.yaml`. A project that has a file at
// that path (from an earlier version of Forge) continues to load
// it. The legacy path is tried after the canonical path; a
// project with both files loads the canonical one.
//
// The legacy path is a one-release compatibility shim. A future
// WBS item removes it once no project needs it.
const projectLocalLegacyRelPath = ".forge/config.yaml"

// xdgConfigRelPath is the config file's path under
// $XDG_CONFIG_HOME, relative to the variable's value.
//
// # Why forge/config.yaml
//
// The XDG Base Directory Specification says that a user's
// configuration lives under $XDG_CONFIG_HOME/<app>/<file>. The
// path follows the specification.
const xdgConfigRelPath = "forge/config.yaml"

// homeConfigRelPath is the config file's path under $HOME,
// relative to the home directory.
//
// # Why .config/forge/config.yaml
//
// The path is the XDG fallback: when $XDG_CONFIG_HOME is not set,
// the specification says to use $HOME/.config.
const homeConfigRelPath = ".config/forge/config.yaml"

// appDataConfigRelPath is the config file's path under %APPDATA%,
// relative to the variable's value.
//
// # Why forge/config.yaml
//
// Windows's %APPDATA% points at the user's roaming application
// data directory. The path follows the convention of putting the
// application's name (forge) in a subdirectory and the config
// file (config.yaml) inside it.
//
// The path uses forward slashes; the filepath package's Join
// function converts them to the platform's separator. On Windows,
// the conversion produces backslashes.
const appDataConfigRelPath = "forge/config.yaml"

// Discover returns the path to the config file to load, or the
// empty string if no config file is found.
//
// # The algorithm
//
// The function tries the following paths in order, stopping at
// the first existing file:
//
//  1. explicit (the --config flag's value). If non-empty, it is
//     the only candidate; the function returns it if the file
//     exists, or returns an error if it does not.
//
//  2. env(EnvVarConfigPath) (the FORGE_CONFIG environment
//     variable). If non-empty, it is the only candidate; the
//     function returns it if the file exists, or returns an
//     error if it does not.
//
//  3. cwd/forge.yaml (the canonical project-local path). If cwd
//     is non-empty and the file exists, the function returns it.
//
//  4. cwd/.forge/config.yaml (the legacy project-local path).
//     If cwd is non-empty and the file exists, the function
//     returns it.
//
//  5. $XDG_CONFIG_HOME/forge/config.yaml. If the variable is set
//     and non-empty, and the file exists, the function returns
//     it.
//
//  6. $HOME/.config/forge/config.yaml. If the variable is set
//     and non-empty, and the file exists, the function returns
//     it.
//
//  7. %APPDATA%\forge\config.yaml (Windows only). If the
//     variable is set and non-empty, and the file exists, the
//     function returns it.
//
//  8. If no candidate exists, the function returns ("", nil).
//
// # The explicit sources' error
//
// If explicit is non-empty and the file at that path does not
// exist, the function returns ("", ErrExplicitConfigMissing).
// The env variable's path behaves the same way. The two explicit
// sources do not fall back to the optional sources: a user who
// named a file expects that file.
//
// # The optional sources' silence
//
// If a candidate from source 3 through 7 does not exist, the
// function continues to the next candidate. No error is returned.
//
// # The directory case
//
// If a candidate path exists but is a directory, the function
// treats the candidate as not-found. A directory is not a valid
// config file.
//
// # Symlinks
//
// The function follows symlinks. A symlink to an existing file is
// treated as the file; a symlink to a missing target is treated
// as missing. The os.Stat call (rather than os.Lstat) follows
// the symlink.
//
// # Purity
//
// The function reads only its three arguments and calls os.Stat
// on each candidate. It does not read os.Getenv, os.Getwd, or
// os.Args.
func Discover(explicit string, env func(string) string, cwd string) (string, error) {
	// Explicit source 1: the --config flag.
	if explicit != "" {
		return checkExplicit(explicit)
	}

	// Explicit source 2: the FORGE_CONFIG environment variable.
	if env != nil {
		if envPath := env(EnvVarConfigPath); envPath != "" {
			return checkExplicit(envPath)
		}
	}

	// Optional source 3: the canonical project-local path.
	if cwd != "" {
		candidate := filepath.Join(cwd, filepath.FromSlash(projectLocalRelPath))
		if isExistingFile(candidate) {
			return candidate, nil
		}
	}

	// Optional source 4: the legacy project-local path. Tried
	// after the canonical path so that a project with both files
	// loads the canonical one.
	if cwd != "" {
		candidate := filepath.Join(cwd, filepath.FromSlash(projectLocalLegacyRelPath))
		if isExistingFile(candidate) {
			return candidate, nil
		}
	}

	// Optional source 5: the XDG path.
	if env != nil {
		if xdg := env("XDG_CONFIG_HOME"); xdg != "" {
			candidate := filepath.Join(xdg, filepath.FromSlash(xdgConfigRelPath))
			if isExistingFile(candidate) {
				return candidate, nil
			}
		}
	}

	// Optional source 6: the HOME fallback.
	if env != nil {
		if home := env("HOME"); home != "" {
			candidate := filepath.Join(home, filepath.FromSlash(homeConfigRelPath))
			if isExistingFile(candidate) {
				return candidate, nil
			}
		}
	}

	// Optional source 7: the Windows %APPDATA% path.
	//
	// The candidate is only tried on Windows; the runtime.GOOS
	// check skips it on other platforms. A future build for
	// Windows adds the platform-specific test.
	if runtime.GOOS == "windows" && env != nil {
		if appData := env("APPDATA"); appData != "" {
			candidate := filepath.Join(appData, filepath.FromSlash(appDataConfigRelPath))
			if isExistingFile(candidate) {
				return candidate, nil
			}
		}
	}

	// No candidate exists. The loader falls back to the
	// built-in defaults (WBS 8.3.1).
	return "", nil
}

// checkExplicit verifies that an explicit path names an existing
// file. It returns the path and a nil error on success; an empty
// path and an E1 error on failure.
//
// # What "explicit" means
//
// An explicit path is one the user named, either via the --config
// flag or via the FORGE_CONFIG environment variable. The user's
// intent is clear: load the file at that path. A missing file is
// an error, not a fallback to another source.
//
// # The sentinel
//
// The function returns the taxonomy's E1 error
// (ErrConfigNotFound via NewConfigNotFound). An earlier draft
// returned ErrExplicitConfigMissing, which predates the taxonomy;
// the draft's sentinel is not a *configError, so the CLI's
// classifier did not recognise it as a config error. The
// taxonomy's E1 error is the correct return value: the CLI's
// classifier maps it to ExitConfig (3), and the error's
// Category() returns "config".
//
// # The directory case
//
// A directory is not a valid config file. If the path names a
// directory, the function returns the same E1 error.
func checkExplicit(path string) (string, error) {
	if !isExistingFile(path) {
		return "", NewConfigNotFound(path)
	}
	return path, nil
}

// isExistingFile reports whether path names an existing regular
// file (or a symlink to one).
//
// # What "existing file" means
//
// The function calls os.Stat on path. If the call fails, the path
// does not exist (or is inaccessible); the function returns
// false. If the call succeeds and the path's mode is a regular
// file, the function returns true. If the path is a directory or
// another non-regular file, the function returns false.
//
// # Symlinks
//
// os.Stat follows symlinks. A symlink to a regular file reports
// the target's mode (regular file); a symlink to a directory
// reports the target's mode (directory); a symlink to a missing
// target reports an error. The function's behaviour is
// consistent: it answers "does the path resolve to an existing
// regular file?".
//
// # Permissions
//
// A path that exists but is not readable by the current process
// still reports as an existing file (os.Stat does not open the
// file). The loader (WBS 8.4.2) is responsible for the read
// error when it opens the file.
func isExistingFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}

// platformDiscoveryPaths returns the platform-specific candidate
// paths for a given env and cwd, in the algorithm's order.
//
// # Why the function exists
//
// The tests iterate the algorithm's candidates without calling
// Discover (which would perform I/O on each). The function returns
// the candidates as a slice so that a test can assert the order
// without I/O.
//
// # What the function returns
//
// The function returns the candidates in the algorithm's order:
// explicit, env, project-local (canonical), project-local
// (legacy), XDG, HOME, APPDATA. An empty candidate is omitted;
// the caller iterates the non-empty ones.
//
// The function is not used by Discover itself; Discover is written
// as an if-chain for clarity. The function's purpose is to make
// the algorithm's order testable without I/O.
func platformDiscoveryPaths(explicit string, env func(string) string, cwd string) []string {
	var paths []string

	if explicit != "" {
		paths = append(paths, explicit)
	}

	if env != nil {
		if p := env(EnvVarConfigPath); p != "" {
			paths = append(paths, p)
		}
	}

	if cwd != "" {
		paths = append(paths, filepath.Join(cwd, filepath.FromSlash(projectLocalRelPath)))
		paths = append(paths, filepath.Join(cwd, filepath.FromSlash(projectLocalLegacyRelPath)))
	}

	if env != nil {
		if xdg := env("XDG_CONFIG_HOME"); xdg != "" {
			paths = append(paths, filepath.Join(xdg, filepath.FromSlash(xdgConfigRelPath)))
		}
		if home := env("HOME"); home != "" {
			paths = append(paths, filepath.Join(home, filepath.FromSlash(homeConfigRelPath)))
		}
		if runtime.GOOS == "windows" {
			if appData := env("APPDATA"); appData != "" {
				paths = append(paths, filepath.Join(appData, filepath.FromSlash(appDataConfigRelPath)))
			}
		}
	}

	return paths
}

// joinPath is a small wrapper around filepath.Join that returns
// the empty string when any argument is empty.
//
// # Why the wrapper
//
// filepath.Join("", "foo") returns "foo", which is a valid path
// but may not be the caller's intent (an empty base should mean
// "skip the candidate"). The wrapper makes the skip explicit.
//
// The wrapper is not currently used; it is provided for a future
// caller that wants the skip behaviour. Its presence is a
// placeholder for the platform-specific extensions that a future
// WBS item might add.
func joinPath(parts ...string) string {
	for _, p := range parts {
		if p == "" {
			return ""
		}
	}
	return filepath.Join(parts...)
}

// splitConfigPathParts returns the path's components for a
// diagnostic display.
//
// # Why the function exists
//
// A future `forge config` command displays the config file's path
// and (in verbose mode) the components. The function returns the
// components as a slice so that the command can render them
// without re-parsing the path.
//
// # What the function returns
//
// The function splits path on the platform's separator. An empty
// path returns an empty slice.
func splitConfigPathParts(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, string(filepath.Separator))
}
