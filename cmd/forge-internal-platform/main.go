// Command forge-internal-platform is an internal build tool for the
// Forge supported-platform matrix. It is not distributed to users.
//
// Usage:
//
//	forge-internal-platform --verify
//	forge-internal-platform --format=gha-matrix [--tier N]
//	forge-internal-platform --format=names
//	forge-internal-platform --format=yaml
//	forge-internal-platform --fmt
//
// Exit codes:
//
//	0  success
//	1  validation or I/O failure
//	2  usage error
//
// This tool is invoked by scripts/platform/gen-ci-matrix.sh and by the
// Taskfile targets platform:verify, platform:fmt, and
// platform:gen-ci-matrix. It lives in a separate cmd/ directory so it
// does not link into the user-facing cmd/forge binary and does not
// appear in the release artifact.
//
// The user-facing CLI (cmd/forge) must not grow a --format=gha-matrix
// flag. cmd/forge is the process entry point and the exit boundary
// (ADR-020); its only job is to bridge os.Exit to cli.Run. Build-time
// matrix logic belongs here.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/thapelomagqazana/forge/internal/platform"
	"gopkg.in/yaml.v3"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the testable entry point. It returns the process exit code and
// writes to the supplied streams, so a future test can drive it without
// spawning a subprocess.
func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("forge-internal-platform", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		verify = fs.Bool("verify", false, "validate the canonical matrix and exit")
		format = fs.String("format", "", "output format: gha-matrix | names | yaml")
		tier   = fs.Int("tier", 0, "filter to tier 1, 2, or 3 (0 = all)")
		doFmt  = fs.Bool("fmt", false, "sort channels and signing_host_requirements in place")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := findModuleRoot()
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-platform: %v\n", err)
		return 1
	}
	path := filepath.Join(root, "docs", "reference", "platforms.yaml")

	switch {
	case *doFmt:
		return doFormat(path, stderr)
	case *verify:
		if _, err := platform.Load(path); err != nil {
			fmt.Fprintf(stderr, "forge-internal-platform: %v\n", err)
			return 1
		}
		return 0
	}

	m, err := platform.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-platform: %v\n", err)
		return 1
	}

	switch *format {
	case "names":
		for _, n := range m.Names() {
			fmt.Fprintln(stdout, n)
		}
	case "yaml":
		enc := yaml.NewEncoder(stdout)
		enc.SetIndent(2)
		if err := enc.Encode(m); err != nil {
			fmt.Fprintf(stderr, "forge-internal-platform: encode: %v\n", err)
			return 1
		}
	case "gha-matrix":
		return writeGHAMatrix(stdout, stderr, m, *tier)
	default:
		fmt.Fprintln(stderr, "forge-internal-platform: --format required (gha-matrix | names | yaml)")
		return 2
	}
	return 0
}

// doFormat loads the matrix, sorts every sortable list, and rewrites the
// file. It is idempotent: running it twice produces the same bytes.
func doFormat(path string, stderr *os.File) int {
	m, err := platform.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-platform: %v\n", err)
		return 1
	}
	for i := range m.Platforms {
		sort.Strings(m.Platforms[i].Channels)
		sort.Strings(m.Platforms[i].SigningHostRequirements)
	}
	sort.Strings(m.Policy.BuildTags)

	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-platform: create %s: %v\n", tmp, err)
		return 1
	}
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	if err := enc.Encode(m); err != nil {
		f.Close()
		os.Remove(tmp)
		fmt.Fprintf(stderr, "forge-internal-platform: encode: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		fmt.Fprintf(stderr, "forge-internal-platform: close: %v\n", err)
		return 1
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		fmt.Fprintf(stderr, "forge-internal-platform: rename: %v\n", err)
		return 1
	}
	return 0
}

// writeGHAMatrix emits a GitHub Actions strategy.matrix object.
//
// The shape is deliberately minimal: name, goos, goarch, runner, tier.
// Nothing else belongs in a CI matrix.
func writeGHAMatrix(stdout, stderr *os.File, m *platform.Matrix, tier int) int {
	type entry struct {
		Name   string `json:"name"`
		GOOS   string `json:"goos"`
		GOARCH string `json:"goarch"`
		Runner string `json:"runner"`
		Tier   int    `json:"tier"`
	}
	out := struct {
		Include []entry `json:"include"`
	}{}
	for _, p := range m.Platforms {
		if tier != 0 && int(p.Tier) != tier {
			continue
		}
		out.Include = append(out.Include, entry{
			Name:   p.Name,
			GOOS:   p.GOOS,
			GOARCH: p.GOARCH,
			Runner: p.CIRunner,
			Tier:   int(p.Tier),
		})
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintf(stderr, "forge-internal-platform: encode json: %v\n", err)
		return 1
	}
	return 0
}

// findModuleRoot walks up from the current working directory looking for
// a go.mod file. It is the same algorithm used by internal/platform and
// by the test helpers in this repository.
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
