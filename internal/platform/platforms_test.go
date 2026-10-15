package platform_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// --- Test helpers ---------------------------------------------------------

const (
	validMatrixYAML = `schema_version: 1
matrix_version: "1.0.0"
applies_to: ">=1.0.0"
policy:
  cgo_enabled: 0
  build_tags: []
  ldflags:
    - "-s"
platforms:
  - name: linux-amd64
    goos: linux
    goarch: amd64
    tier: 1
    min_os: "glibc 2.28+"
    libc: glibc
    static: true
    signing: none
    channels: [direct]
    ci_runner: "ubuntu-22.04"
    notes: "Primary Linux target."
channels:
  direct:
    description: "Direct download."
`
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "platforms.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return path
}

// --- Positive cases -------------------------------------------------------

// TestLoad_ValidMatrix asserts that a well-formed matrix loads and compiles.
func TestLoad_ValidMatrix(t *testing.T) {
	t.Parallel()
	m, err := platform.Load(writeTemp(t, validMatrixYAML))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.MatrixVersion != "1.0.0" {
		t.Errorf("MatrixVersion = %q, want 1.0.0", m.MatrixVersion)
	}
	if len(m.Platforms) != 1 {
		t.Fatalf("len(Platforms) = %d, want 1", len(m.Platforms))
	}
	if m.Platforms[0].Name != "linux-amd64" {
		t.Errorf("Platforms[0].Name = %q, want linux-amd64", m.Platforms[0].Name)
	}
}

// TestLoad_CanonicalMatrix asserts that the repository's own
// docs/reference/platforms.yaml loads and contains the six frozen platforms.
func TestLoad_CanonicalMatrix(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	want := []string{
		"linux-amd64", "linux-arm64",
		"darwin-amd64", "darwin-arm64",
		"windows-amd64", "windows-arm64",
	}
	got := m.Names()
	if len(got) != len(want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("names[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestCanonicalMatrix_Tiers asserts the tier assignment is frozen.
func TestCanonicalMatrix_Tiers(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	want := map[string]platform.Tier{
		"linux-amd64":   platform.Tier1,
		"linux-arm64":   platform.Tier1,
		"darwin-amd64":  platform.Tier2,
		"darwin-arm64":  platform.Tier1,
		"windows-amd64": platform.Tier1,
		"windows-arm64": platform.Tier2,
	}
	for name, tier := range want {
		p, ok := m.Platform(name)
		if !ok {
			t.Errorf("platform %s not found", name)
			continue
		}
		if p.Tier != tier {
			t.Errorf("%s tier = %v, want %v", name, p.Tier, tier)
		}
	}
}

// TestCanonicalMatrix_CGODisabled asserts the hard CGO_ENABLED=0 rule.
func TestCanonicalMatrix_CGODisabled(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if m.Policy.CGOEnabled != 0 {
		t.Errorf("CGOEnabled = %d, want 0", m.Policy.CGOEnabled)
	}
}

// TestCanonicalMatrix_AllStatic asserts every platform is static.
func TestCanonicalMatrix_AllStatic(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	for _, p := range m.Platforms {
		if !p.Static {
			t.Errorf("%s: static = false, want true", p.Name)
		}
	}
}

// TestCanonicalMatrix_SigningRequirements asserts the signing invariant.
func TestCanonicalMatrix_SigningRequirements(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	for _, p := range m.Platforms {
		if p.Signing == platform.SigningNone {
			if len(p.SigningHostRequirements) != 0 {
				t.Errorf("%s: signing none but requirements present", p.Name)
			}
			continue
		}
		if len(p.SigningHostRequirements) == 0 {
			t.Errorf("%s: signing %s but no host requirements", p.Name, p.Signing)
		}
	}
}

// TestCanonicalMatrix_DockerPlatform asserts the docker invariant.
func TestCanonicalMatrix_DockerPlatform(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	for _, p := range m.Platforms {
		if p.HasChannel("docker") && p.DockerPlatform == "" {
			t.Errorf("%s: docker channel without docker_platform", p.Name)
		}
		if !p.HasChannel("docker") && p.DockerPlatform != "" {
			t.Errorf("%s: docker_platform without docker channel", p.Name)
		}
	}
}

// TestCanonicalMatrix_ChannelsAreDeclared asserts channel references resolve.
func TestCanonicalMatrix_ChannelsAreDeclared(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	for _, p := range m.Platforms {
		for _, ch := range p.Channels {
			if _, ok := m.Channels[ch]; !ok {
				t.Errorf("%s: undeclared channel %q", p.Name, ch)
			}
		}
	}
}

// TestByTier asserts tier filtering.
func TestByTier(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	if got := len(m.ByTier(platform.Tier1)); got != 4 {
		t.Errorf("Tier1 count = %d, want 4", got)
	}
	if got := len(m.ByTier(platform.Tier2)); got != 2 {
		t.Errorf("Tier2 count = %d, want 2", got)
	}
	if got := len(m.ByTier(platform.Tier3)); got != 0 {
		t.Errorf("Tier3 count = %d, want 0", got)
	}
}

// TestByGOOSGOARCH asserts GOOS/GOARCH lookup.
func TestByGOOSGOARCH(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	p, ok := m.ByGOOSGOARCH("darwin", "arm64")
	if !ok {
		t.Fatal("darwin/arm64 not found")
	}
	if p.Name != "darwin-arm64" {
		t.Errorf("name = %q, want darwin-arm64", p.Name)
	}
}

// TestTierCounts asserts the tier histogram.
func TestTierCounts(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	counts := m.TierCounts()
	if counts[platform.Tier1] != 4 || counts[platform.Tier2] != 2 || counts[platform.Tier3] != 0 {
		t.Errorf("counts = %v", counts)
	}
}

// --- Negative cases -------------------------------------------------------

// TestLoad_MissingFile asserts a missing file is rejected.
func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := platform.Load(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error = %v, want open failure", err)
	}
}

// TestParse_UnknownField asserts unknown keys are rejected.
func TestParse_UnknownField(t *testing.T) {
	t.Parallel()
	_, err := platform.Parse(strings.NewReader(validMatrixYAML + "bogus: true\n"))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

// TestParse_BadSchemaVersion asserts a wrong schema version is rejected.
func TestParse_BadSchemaVersion(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "schema_version: 1", "schema_version: 2", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for schema_version 2")
	}
}

// TestParse_CGOEnabledOne asserts cgo_enabled != 0 is rejected.
func TestParse_CGOEnabledOne(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "cgo_enabled: 0", "cgo_enabled: 1", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for cgo_enabled 1")
	}
}

// TestParse_DuplicatePlatformName asserts duplicate names are rejected.
func TestParse_DuplicatePlatformName(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML,
		"channels:\n  direct:",
		`  - name: linux-amd64
    goos: linux
    goarch: amd64
    tier: 1
    min_os: "x"
    libc: glibc
    static: true
    signing: none
    channels: [direct]
    ci_runner: "x"
    notes: "x"
channels:
  direct:`, 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

// TestParse_DuplicateGOOSGOARCH asserts a second platform for the same pair
// is rejected.
func TestParse_DuplicateGOOSGOARCH(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML,
		"channels:\n  direct:",
		`  - name: linux-amd64-v2
    goos: linux
    goarch: amd64
    tier: 1
    min_os: "x"
    libc: glibc
    static: true
    signing: none
    channels: [direct]
    ci_runner: "x"
    notes: "x"
channels:
  direct:`, 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for duplicate goos/goarch")
	}
}

// TestParse_UnknownChannel asserts an undeclared channel reference is rejected.
func TestParse_UnknownChannel(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "channels: [direct]", "channels: [nope]", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for unknown channel")
	}
}

// TestParse_SigningWithoutRequirements asserts the signing invariant.
func TestParse_SigningWithoutRequirements(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "signing: none", "signing: notarization", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for signing without requirements")
	}
}

// TestParse_DockerWithoutPlatform asserts the docker invariant.
func TestParse_DockerWithoutPlatform(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "channels: [direct]", "channels: [direct, docker]", 1)
	bad = strings.Replace(bad, "  direct:\n    description", "  docker:\n    description: d\n  direct:\n    description", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for docker without docker_platform")
	}
}

// TestParse_StaticFalse asserts static: false is rejected.
func TestParse_StaticFalse(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "static: true", "static: false", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for static: false")
	}
}

// TestParse_TierZero asserts tier 0 is rejected.
func TestParse_TierZero(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "tier: 1", "tier: 0", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for tier 0")
	}
}

// TestParse_ChannelsUnsorted asserts unsorted channels are rejected.
func TestParse_ChannelsUnsorted(t *testing.T) {
	t.Parallel()
	bad := strings.Replace(validMatrixYAML, "channels: [direct]", "channels: [direct, apt]", 1)
	bad = strings.Replace(bad, "  direct:\n    description", "  apt:\n    description: a\n  direct:\n    description", 1)
	_, err := platform.Parse(strings.NewReader(bad))
	if err == nil {
		t.Fatal("expected error for unsorted channels")
	}
}

// --- Edge cases -----------------------------------------------------------

// TestParse_EmptyReader asserts an empty document is rejected.
func TestParse_EmptyReader(t *testing.T) {
	t.Parallel()
	_, err := platform.Parse(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

// TestParse_OnlyComments asserts a comment-only document is rejected.
func TestParse_OnlyComments(t *testing.T) {
	t.Parallel()
	_, err := platform.Parse(strings.NewReader("# nothing here\n"))
	if err == nil {
		t.Fatal("expected error for comment-only input")
	}
}

// TestTierString asserts tier string rendering.
func TestTierString(t *testing.T) {
	t.Parallel()
	cases := map[platform.Tier]string{
		platform.Tier1:   "Tier 1",
		platform.Tier2:   "Tier 2",
		platform.Tier3:   "Tier 3",
		platform.Tier(9): "Tier(9)",
	}
	for in, want := range cases {
		if got := in.String(); got != want {
			t.Errorf("Tier(%d).String() = %q, want %q", int(in), got, want)
		}
	}
}

// TestPlatformValidate_Table drives the validator through every field.
func TestPlatformValidate_Table(t *testing.T) {
	t.Parallel()
	base := platform.Platform{
		Name: "x", GOOS: "linux", GOARCH: "amd64", Tier: platform.Tier1,
		MinOS: "x", Libc: platform.LibcGlibc, Static: true,
		Signing: platform.SigningNone, Channels: []string{"direct"},
		CIRunner: "x", Notes: "x",
	}
	cases := []struct {
		name   string
		mutate func(*platform.Platform)
		want   string
	}{
		{"empty name", func(p *platform.Platform) { p.Name = "" }, "name is empty"},
		{"empty goos", func(p *platform.Platform) { p.GOOS = "" }, "goos is empty"},
		{"empty goarch", func(p *platform.Platform) { p.GOARCH = "" }, "goarch is empty"},
		{"bad tier", func(p *platform.Platform) { p.Tier = 9 }, "tier 9"},
		{"empty min_os", func(p *platform.Platform) { p.MinOS = "" }, "min_os is empty"},
		{"bad libc", func(p *platform.Platform) { p.Libc = "wat" }, "libc"},
		{"not static", func(p *platform.Platform) { p.Static = false }, "static MUST be true"},
		{"bad signing", func(p *platform.Platform) { p.Signing = "wat" }, "signing"},
		{"no channels", func(p *platform.Platform) { p.Channels = nil }, "at least one channel"},
		{"unsorted channels", func(p *platform.Platform) { p.Channels = []string{"z", "a"} }, "sorted and unique"},
		{"docker no platform", func(p *platform.Platform) {
			p.Channels = []string{"direct", "docker"}
			p.DockerPlatform = ""
		}, "docker channel requires"},
		{"platform no docker", func(p *platform.Platform) { p.DockerPlatform = "linux/amd64" }, "without docker channel"},
		{"empty ci_runner", func(p *platform.Platform) { p.CIRunner = "" }, "ci_runner is empty"},
		{"empty notes", func(p *platform.Platform) { p.Notes = "" }, "notes is empty"},
		{"none with requirements", func(p *platform.Platform) { p.SigningHostRequirements = []string{"x"} }, "signing is none but"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := base
			p.Channels = append([]string(nil), base.Channels...)
			tc.mutate(&p)
			err := p.Validate()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// --- Non-functional -------------------------------------------------------

// TestLoad_IsPure asserts Load does not mutate its input path.
func TestLoad_IsPure(t *testing.T) {
	t.Parallel()
	path := writeTemp(t, validMatrixYAML)
	orig := path
	if _, err := platform.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if path != orig {
		t.Errorf("path mutated: %q -> %q", orig, path)
	}
}

// TestLoad_IsDeterministic asserts two loads produce identical results.
func TestLoad_IsDeterministic(t *testing.T) {
	t.Parallel()
	path := writeTemp(t, validMatrixYAML)
	a, err := platform.Load(path)
	if err != nil {
		t.Fatalf("Load a: %v", err)
	}
	b, err := platform.Load(path)
	if err != nil {
		t.Fatalf("Load b: %v", err)
	}
	if a.MatrixVersion != b.MatrixVersion || len(a.Platforms) != len(b.Platforms) {
		t.Fatal("non-deterministic load")
	}
}

// TestLoad_Concurrent asserts the loaded matrix is safe for concurrent reads.
func TestLoad_Concurrent(t *testing.T) {
	t.Parallel()
	m, err := platform.LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault: %v", err)
	}
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = m.Names()
				_, _ = m.Platform("linux-amd64")
				_ = m.ByTier(platform.Tier1)
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestValidate_ErrorIsDescriptive asserts errors name the offending field.
func TestValidate_ErrorIsDescriptive(t *testing.T) {
	t.Parallel()
	p := platform.Platform{Name: "x"}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "x") {
		t.Errorf("error %v does not name the platform", err)
	}
}

// TestErrorsAreNotSentinels asserts validation errors are not wrapped
// sentinels that callers might accidentally match on.
func TestErrorsAreNotSentinels(t *testing.T) {
	t.Parallel()
	p := platform.Platform{}
	err := p.Validate()
	if errors.Is(err, os.ErrNotExist) {
		t.Fatal("validation error matched os.ErrNotExist")
	}
}
