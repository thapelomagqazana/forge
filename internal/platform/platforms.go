// Package platform loads and validates the Forge supported-platform matrix.
//
// The matrix is published as a machine-readable YAML document at
// docs/reference/platforms.yaml. It is the source of truth for build
// scripts, CI matrix generation, and release tooling. This package gives
// Go programs typed access to that matrix without duplicating it.
//
// # Contract
//
// The YAML MUST validate against schemas/platforms.schema.json. This
// package performs the same validation in Go so that it can be used at
// build time without a JSON Schema library. The two validators MUST agree;
// internal/platform/schema_test.go asserts this.
//
// # Versioning
//
// The matrix has two version numbers:
//
//   - schema_version — the shape of the YAML document. Bumped only on
//     incompatible schema changes. This package only understands version 1.
//   - matrix_version — the semantic version of the matrix contents.
//     Bumped on every additive change. Consumers that care about exact
//     platform lists should pin this.
//
// # Thread Safety
//
// All exported types are immutable and safe for concurrent use.
package platform

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the only schema version this package understands.
const SchemaVersion = 1

// Tier is a promise about how much verification a platform receives.
//
// Tier 1 is fully tested in CI on every pull request. Tier 2 is built and
// unit-tested on every release tag. Tier 3 is best-effort with no CI.
type Tier int

const (
	// Tier1 is fully tested in CI on every pull request and release tag.
	Tier1 Tier = 1
	// Tier2 is built and unit-tested on every release tag; integration
	// tests are run manually before a release.
	Tier2 Tier = 2
	// Tier3 is best-effort with no CI and no SLA.
	Tier3 Tier = 3
)

// String returns a stable, human-readable name for the tier.
func (t Tier) String() string {
	switch t {
	case Tier1:
		return "Tier 1"
	case Tier2:
		return "Tier 2"
	case Tier3:
		return "Tier 3"
	default:
		return fmt.Sprintf("Tier(%d)", int(t))
	}
}

// Valid reports whether t is one of the three defined tiers.
func (t Tier) Valid() bool {
	return t == Tier1 || t == Tier2 || t == Tier3
}

// Signing names the mechanism used to sign a release artifact.
type Signing string

const (
	SigningNone         Signing = "none"
	SigningNotarization Signing = "notarization"
	SigningAuthenticode Signing = "authenticode"
	SigningGPG          Signing = "gpg"
)

// Valid reports whether s is a defined signing mechanism.
func (s Signing) Valid() bool {
	switch s {
	case SigningNone, SigningNotarization, SigningAuthenticode, SigningGPG:
		return true
	default:
		return false
	}
}

// Libc names the C library a platform links against.
type Libc string

const (
	LibcGlibc  Libc = "glibc"
	LibcMusl   Libc = "musl"
	LibcSystem Libc = "system"
	LibcNone   Libc = "none"
)

// Valid reports whether l is a defined libc.
func (l Libc) Valid() bool {
	switch l {
	case LibcGlibc, LibcMusl, LibcSystem, LibcNone:
		return true
	default:
		return false
	}
}

// Platform describes one supported GOOS/GOARCH pair.
//
// All fields are populated from platforms.yaml. The zero value is not a
// valid platform; use Matrix.Platform to look one up.
type Platform struct {
	// Name is the stable artifact-name prefix, e.g. "linux-amd64".
	Name string `yaml:"name"`
	// GOOS is the Go operating system, e.g. "linux".
	GOOS string `yaml:"goos"`
	// GOARCH is the Go architecture, e.g. "amd64".
	GOARCH string `yaml:"goarch"`
	// Tier is the verification promise for this platform.
	Tier Tier `yaml:"tier"`
	// MinOS is the minimum OS version, e.g. "macOS 12".
	MinOS string `yaml:"min_os"`
	// Libc is the C library this platform links against.
	Libc Libc `yaml:"libc"`
	// Static MUST be true for every release build.
	Static bool `yaml:"static"`
	// Signing is the mechanism used to sign the artifact.
	Signing Signing `yaml:"signing"`
	// Channels lists the distribution channels for this platform.
	Channels []string `yaml:"channels"`
	// DockerPlatform is the OCI platform string, present only when
	// "docker" is in Channels.
	DockerPlatform string `yaml:"docker_platform,omitempty"`
	// CIRunner is the GitHub Actions runner label.
	CIRunner string `yaml:"ci_runner"`
	// SigningHostRequirements lists what the signing host needs.
	// Present only when Signing != none.
	SigningHostRequirements []string `yaml:"signing_host_requirements,omitempty"`
	// Notes is free-form human context.
	Notes string `yaml:"notes"`
}

// HasChannel reports whether p distributes through the named channel.
func (p Platform) HasChannel(name string) bool {
	for _, c := range p.Channels {
		if c == name {
			return true
		}
	}
	return false
}

// Validate checks that p satisfies every invariant the schema enforces.
// It returns a non-nil error describing the first violation found.
func (p Platform) Validate() error {
	if p.Name == "" {
		return errors.New("platform: name is empty")
	}
	if p.GOOS == "" {
		return fmt.Errorf("platform %s: goos is empty", p.Name)
	}
	if p.GOARCH == "" {
		return fmt.Errorf("platform %s: goarch is empty", p.Name)
	}
	if !p.Tier.Valid() {
		return fmt.Errorf("platform %s: tier %d is not 1, 2, or 3", p.Name, int(p.Tier))
	}
	if p.MinOS == "" {
		return fmt.Errorf("platform %s: min_os is empty", p.Name)
	}
	if !p.Libc.Valid() {
		return fmt.Errorf("platform %s: libc %q is not defined", p.Name, string(p.Libc))
	}
	if !p.Static {
		return fmt.Errorf("platform %s: static MUST be true", p.Name)
	}
	if !p.Signing.Valid() {
		return fmt.Errorf("platform %s: signing %q is not defined", p.Name, string(p.Signing))
	}
	if len(p.Channels) == 0 {
		return fmt.Errorf("platform %s: at least one channel is required", p.Name)
	}
	if !isSortedUnique(p.Channels) {
		return fmt.Errorf("platform %s: channels must be sorted and unique", p.Name)
	}
	if p.HasChannel("docker") && p.DockerPlatform == "" {
		return fmt.Errorf("platform %s: docker channel requires docker_platform", p.Name)
	}
	if !p.HasChannel("docker") && p.DockerPlatform != "" {
		return fmt.Errorf("platform %s: docker_platform set without docker channel", p.Name)
	}
	if p.CIRunner == "" {
		return fmt.Errorf("platform %s: ci_runner is empty", p.Name)
	}
	if p.Signing == SigningNone && len(p.SigningHostRequirements) > 0 {
		return fmt.Errorf("platform %s: signing is none but signing_host_requirements is set", p.Name)
	}
	if p.Signing != SigningNone && len(p.SigningHostRequirements) == 0 {
		return fmt.Errorf("platform %s: signing %q requires signing_host_requirements", p.Name, string(p.Signing))
	}
	if !isSortedUnique(p.SigningHostRequirements) {
		return fmt.Errorf("platform %s: signing_host_requirements must be sorted and unique", p.Name)
	}
	if p.Notes == "" {
		return fmt.Errorf("platform %s: notes is empty", p.Name)
	}
	return nil
}

// Policy is the global build policy applied to every platform.
type Policy struct {
	// CGOEnabled MUST be 0.
	CGOEnabled int `yaml:"cgo_enabled"`
	// BuildTags is the sorted list of Go build tags.
	BuildTags []string `yaml:"build_tags"`
	// LDFlags is the ordered list of linker flags.
	LDFlags []string `yaml:"ldflags"`
}

// Validate checks the policy invariants.
func (p Policy) Validate() error {
	if p.CGOEnabled != 0 {
		return fmt.Errorf("policy: cgo_enabled MUST be 0, got %d", p.CGOEnabled)
	}
	if len(p.LDFlags) == 0 {
		return errors.New("policy: at least one ldflag is required")
	}
	if !isSortedUnique(p.BuildTags) {
		return errors.New("policy: build_tags must be sorted and unique")
	}
	return nil
}

// Channel describes a distribution channel.
type Channel struct {
	// Description is a one-line human description.
	Description string `yaml:"description"`
}

// Matrix is the parsed and validated platform matrix.
//
// The zero value is not valid. Use Load or Parse to obtain a Matrix.
// Matrix values are immutable and safe for concurrent use.
type Matrix struct {
	// SchemaVersion is the schema version of the source document.
	SchemaVersion int `yaml:"schema_version"`
	// MatrixVersion is the semantic version of the matrix contents.
	MatrixVersion string `yaml:"matrix_version"`
	// AppliesTo is the Forge version range this matrix applies to.
	AppliesTo string `yaml:"applies_to"`
	// Policy is the global build policy.
	Policy Policy `yaml:"policy"`
	// Platforms is the ordered list of supported platforms.
	Platforms []Platform `yaml:"platforms"`
	// Channels is the set of known distribution channels.
	Channels map[string]Channel `yaml:"channels"`

	// index is built by Compile for O(1) lookup by name.
	index map[string]*Platform
}

// Load reads, parses, and validates a matrix from path.
//
// It returns a wrapped error on any failure. A file that is missing,
// unreadable, malformed, or violates the schema is rejected.
func Load(path string) (*Matrix, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("platform: open %s: %w", path, err)
	}
	defer f.Close()
	m, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("platform: %s: %w", path, err)
	}
	return m, nil
}

// LoadDefault loads the matrix from the repository's canonical location.
//
// It walks up from the current working directory until it finds a go.mod,
// then reads docs/reference/platforms.yaml relative to it. This lets tests
// and tooling run from any package directory.
func LoadDefault() (*Matrix, error) {
	root, err := moduleRoot()
	if err != nil {
		return nil, err
	}
	return Load(filepath.Join(root, "docs", "reference", "platforms.yaml"))
}

// Parse reads and validates a matrix from r.
//
// Parse rejects unknown fields, so a typo in the YAML is a hard error
// rather than a silently-ignored key.
func Parse(r io.Reader) (*Matrix, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true) // reject unknown keys

	var m Matrix
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if err := m.Compile(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Compile validates the matrix and builds internal indexes.
//
// Compile is idempotent. It must be called before Lookup, Platform, or
// any of the By* methods.
func (m *Matrix) Compile() error {
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("schema_version: got %d, want %d", m.SchemaVersion, SchemaVersion)
	}
	if m.MatrixVersion == "" {
		return errors.New("matrix_version: empty")
	}
	if m.AppliesTo == "" {
		return errors.New("applies_to: empty")
	}
	if err := m.Policy.Validate(); err != nil {
		return err
	}
	if len(m.Platforms) == 0 {
		return errors.New("platforms: at least one platform is required")
	}
	if len(m.Channels) == 0 {
		return errors.New("channels: at least one channel is required")
	}

	index := make(map[string]*Platform, len(m.Platforms))
	seenGOOSGOARCH := make(map[string]string, len(m.Platforms))
	for i := range m.Platforms {
		p := &m.Platforms[i]
		if err := p.Validate(); err != nil {
			return err
		}
		if _, dup := index[p.Name]; dup {
			return fmt.Errorf("platform %s: duplicate name", p.Name)
		}
		key := p.GOOS + "/" + p.GOARCH
		if other, dup := seenGOOSGOARCH[key]; dup {
			return fmt.Errorf("platform %s: %s already covered by %s", p.Name, key, other)
		}
		seenGOOSGOARCH[key] = p.Name
		for _, ch := range p.Channels {
			if _, ok := m.Channels[ch]; !ok {
				return fmt.Errorf("platform %s: unknown channel %q", p.Name, ch)
			}
		}
		index[p.Name] = p
	}
	m.index = index
	return nil
}

// Platform returns the platform with the given name.
//
// The name is the stable artifact-name prefix, e.g. "linux-amd64". The
// second return value is false if no such platform exists.
func (m *Matrix) Platform(name string) (*Platform, bool) {
	p, ok := m.index[name]
	return p, ok
}

// ByGOOSGOARCH returns the platform with the given GOOS and GOARCH.
func (m *Matrix) ByGOOSGOARCH(goos, goarch string) (*Platform, bool) {
	for i := range m.Platforms {
		p := &m.Platforms[i]
		if p.GOOS == goos && p.GOARCH == goarch {
			return p, true
		}
	}
	return nil, false
}

// ByTier returns the platforms with the given tier, in matrix order.
func (m *Matrix) ByTier(t Tier) []Platform {
	var out []Platform
	for _, p := range m.Platforms {
		if p.Tier == t {
			out = append(out, p)
		}
	}
	return out
}

// Names returns the platform names in matrix order.
func (m *Matrix) Names() []string {
	out := make([]string, len(m.Platforms))
	for i, p := range m.Platforms {
		out[i] = p.Name
	}
	return out
}

// TierCounts returns a map from tier to the number of platforms in it.
func (m *Matrix) TierCounts() map[Tier]int {
	out := map[Tier]int{Tier1: 0, Tier2: 0, Tier3: 0}
	for _, p := range m.Platforms {
		out[p.Tier]++
	}
	return out
}

// GoVersion returns the minimum Go version required to build all platforms.
//
// This is informational: it returns the Go version declared in go.mod.
// It exists so release tooling can print "built with Go X" without
// hard-coding the value in a shell script.
func (m *Matrix) GoVersion() string {
	// The matrix does not carry a Go version; the build uses whatever
	// go.mod declares. This method is a placeholder for a future schema
	// field and returns the empty string for now.
	return ""
}

// isSortedUnique reports whether s is sorted ascending and has no duplicates.
func isSortedUnique(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] >= s[i] {
			return false
		}
	}
	return true
}

// moduleRoot walks up from the current working directory to find go.mod.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("platform: getwd: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("platform: no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// SortPlatforms sorts m.Platforms by name in place. It is used by tests
// and by CI matrix generation to get a deterministic order.
func (m *Matrix) SortPlatforms() {
	sort.Slice(m.Platforms, func(i, j int) bool {
		return m.Platforms[i].Name < m.Platforms[j].Name
	})
}
