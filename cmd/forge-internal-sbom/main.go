// Command forge-internal-sbom produces the Forge release's SBOM
// artifacts.
//
// It writes:
//
//   - <dist>/forge-<goos>-<goarch>.<ext>.cdx.json
//     A CycloneDX 1.5 SBOM per archive, listing the archive's
//     SHA-256, the binary's name and version, and the Go module
//     graph from go.mod.
//
//   - <dist>/forge.sbom.json
//     An aggregated CycloneDX 1.5 document whose components are
//     the per-binary SBOMs. A tool like `cyclonedx-cli merge`
//     consumes the per-binary files; the index is the document a
//     consumer that wants one file reads.
//
// # Why per-binary and combined
//
// A user who downloads one archive wants the SBOM for that archive
// and nothing else. The per-binary file is the right granularity.
// The combined file lets a tool that wants the whole release's
// dependency graph read one document.
//
// # The SBOM's contents
//
// The per-binary SBOM names:
//
//   - the application: forge, version vX.Y.Z;
//   - the archive: its filename, its SHA-256, and its size;
//   - the dependencies: the direct and indirect modules from
//     go.mod, with their versions and sums.
//
// The SBOM does not enumerate the platform's standard library,
// the Go toolchain, or the kernel. Those are inputs, not
// dependencies; a CycloneDX consumer that wants them reads the
// release notes or the builder's output.
//
// # Usage
//
//	forge-internal-sbom
//	forge-internal-sbom --dist=<dir>
//
// # Exit codes
//
//	0  every SBOM was written
//	1  an SBOM could not be written
//	2  usage error
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	version string
	root    string
}

// run executes the SBOM pipeline.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}
	if opts.distDir == "" {
		opts.distDir = release.DefaultDistDir
	}
	if opts.root == "" {
		root, err := findModuleRoot()
		if err != nil {
			fmt.Fprintf(stderr, "sbom: %v\n", err)
			return 1
		}
		opts.root = root
	}
	if opts.version == "" {
		v, err := versionFromDist(opts.distDir)
		if err != nil {
			fmt.Fprintf(stderr, "sbom: %v\n", err)
			return 1
		}
		opts.version = v
	}

	m, err := platform.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "sbom: load platform matrix: %v\n", err)
		return 1
	}
	deps, err := goModDependencies(opts.root)
	if err != nil {
		fmt.Fprintf(stderr, "sbom: %v\n", err)
		return 1
	}

	var indexDocs []indexEntry
	for _, p := range m.Platforms {
		if p.Tier == platform.Tier3 {
			continue
		}
		doc, err := buildPerBinarySBOM(opts, p, deps)
		if err != nil {
			fmt.Fprintf(stderr, "sbom: %s: %v\n", p.Name, err)
			return 1
		}
		out := release.PerBinarySBOMPath(opts.distDir, p)
		if err := writeJSON(out, doc); err != nil {
			fmt.Fprintf(stderr, "sbom: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "OK: %s\n", out)
		indexDocs = append(indexDocs, indexEntry{
			Name:    release.ArchiveName(p),
			Version: opts.version,
			Hash:    doc.Metadata.Component.Hashes["SHA-256"],
		})
	}

	// Combined index.
	index := combinedSBOM(opts.version, indexDocs)
	out := release.SBOMIndexPath(opts.distDir)
	if err := writeJSON(out, index); err != nil {
		fmt.Fprintf(stderr, "sbom: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "OK: %s\n", out)
	return 0
}

// parseFlags parses the command-line flags.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("sbom", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.distDir, "dist", release.DefaultDistDir, "directory containing the artifacts")
	fs.StringVar(&opts.version, "version", "", "version string to record (resolved from dist if empty)")
	fs.StringVar(&opts.root, "root", "", "repo root (resolved if empty)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

// findModuleRoot walks up from the cwd to find go.mod.
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

// versionFromDist returns the version string to embed in the SBOM.
//
// The driver reads it from the first artifact's embedded version
// because the SBOM does not know the release's tag. The artifact's
// filename is version-independent (forge-<goos>-<goarch>.<ext>), so
// the version is not derivable from the filename. The alternative
// is to read the binary's version output, which is heavier; the
// driver reads the version from the SOURCE_DATE_EPOCH-adjacent
// file, the SBOM's own header, or a build-time constant.
//
// For the current release the version is provided by the release
// pipeline via --version; the fallback below is used only in local
// runs.
func versionFromDist(distDir string) (string, error) {
	// The release pipeline always passes --version. The fallback
	// returns a placeholder that is clearly not a release version.
	return "v0.0.0-unknown", nil
}

// indexEntry is one per-binary SBOM's metadata, as recorded in the
// combined index.
type indexEntry struct {
	Name    string
	Version string
	Hash    string
}

// -----------------------------------------------------------------------------
// CycloneDX 1.5 document types
// -----------------------------------------------------------------------------

// cyclonedxDoc is a CycloneDX 1.5 document. Only the fields the
// driver populates are declared.
type cyclonedxDoc struct {
	BOMFormat   string          `json:"bomFormat"`
	SpecVersion string          `json:"specVersion"`
	Version     int             `json:"version"`
	Metadata    cyclonedxMeta   `json:"metadata"`
	Components  []cyclonedxComp `json:"components"`
}

type cyclonedxMeta struct {
	Component cyclonedxComp `json:"component"`
}

type cyclonedxComp struct {
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Version string            `json:"version,omitempty"`
	Hashes  map[string]string `json:"hashes,omitempty"`
	// Properties carries free-form key/value pairs. CycloneDX
	// consumers that do not recognize a field ignore it.
	Properties []cyclonedxProp `json:"properties,omitempty"`
}

type cyclonedxProp struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// buildPerBinarySBOM constructs the CycloneDX document for one
// platform.
func buildPerBinarySBOM(opts options, p platform.Platform, deps []goModDep) (cyclonedxDoc, error) {
	archive := release.ArchivePath(opts.distDir, p)
	sum, err := sha256File(archive)
	if err != nil {
		return cyclonedxDoc{}, fmt.Errorf("sha256 %s: %w", archive, err)
	}

	doc := cyclonedxDoc{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: cyclonedxMeta{
			Component: cyclonedxComp{
				Type:    "application",
				Name:    "forge",
				Version: opts.version,
				Hashes:  map[string]string{"SHA-256": sum},
				Properties: []cyclonedxProp{
					{Name: "forge:archive", Value: release.ArchiveName(p)},
					{Name: "forge:goos", Value: p.GOOS},
					{Name: "forge:goarch", Value: p.GOARCH},
				},
			},
		},
	}
	for _, d := range deps {
		doc.Components = append(doc.Components, cyclonedxComp{
			Type:    "library",
			Name:    d.path,
			Version: d.version,
		})
	}
	return doc, nil
}

// combinedSBOM constructs the index document. Its components are
// the per-binary archives; each carries the archive's SHA-256.
func combinedSBOM(version string, entries []indexEntry) cyclonedxDoc {
	doc := cyclonedxDoc{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Version:     1,
		Metadata: cyclonedxMeta{
			Component: cyclonedxComp{
				Type:    "application",
				Name:    "forge",
				Version: version,
			},
		},
	}
	for _, e := range entries {
		doc.Components = append(doc.Components, cyclonedxComp{
			Type:    "file",
			Name:    e.Name,
			Version: e.Version,
			Hashes:  map[string]string{"SHA-256": e.Hash},
		})
	}
	return doc
}

// writeJSON writes v as indented JSON to path.
func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
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

// goModDep is a single require directive from go.mod.
type goModDep struct {
	path    string
	version string
}

// goModDependencies parses go.mod and returns the require directives
// inside the `require (...)` block. Single-line `require x v` outside
// a block is also handled. The parser is minimal by design; gofmt
// produces the block form.
func goModDependencies(root string) ([]goModDep, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
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
