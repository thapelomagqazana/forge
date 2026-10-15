package version_test

import (
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/thapelomagqazana/forge/internal/version"
)

// --- Positive ---------------------------------------------------------------

// TestReadProvenance_FromTestBinary asserts the reader returns a
// populated Provenance for the test binary. The test binary is always
// built with module support, so GoVersion and Module are non-empty.
func TestReadProvenance_FromTestBinary(t *testing.T) {
	t.Parallel()
	p := version.ReadProvenance()
	if p.GoVersion == "" {
		t.Errorf("GoVersion is empty")
	}
	if !strings.HasPrefix(p.GoVersion, "go1.") {
		t.Errorf("GoVersion = %q, want a go1.x prefix", p.GoVersion)
	}
	if p.Module == "" {
		t.Errorf("Module is empty")
	}
}

// TestProvenanceFromBuildInfo_Full asserts the conversion from a
// synthetic debug.BuildInfo.
func TestProvenanceFromBuildInfo_Full(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC)
	info := &debug.BuildInfo{
		GoVersion: "go1.23.4",
		Main: debug.Module{
			Path:    "github.com/thapelomagqazana/forge",
			Version: "(devel)",
		},
		Deps: []*debug.Module{
			{Path: "github.com/spf13/cobra", Version: "v1.8.1", Sum: "h1:abc"},
			{Path: "github.com/spf13/pflag", Version: "v1.0.5", Sum: "h1:def"},
		},
		Settings: []debug.BuildSetting{
			{Key: "-trimpath", Value: "true"},
			{Key: "CGO_ENABLED", Value: "0"},
			{Key: "GOOS", Value: "linux"},
			{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
			{Key: "vcs.time", Value: now.Format(time.RFC3339)},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	p := version.ProvenanceForTest(info)

	if p.GoVersion != "go1.23.4" {
		t.Errorf("GoVersion = %q", p.GoVersion)
	}
	if p.Module != "github.com/thapelomagqazana/forge" {
		t.Errorf("Module = %q", p.Module)
	}
	if p.ModuleVersion != "(devel)" {
		t.Errorf("ModuleVersion = %q", p.ModuleVersion)
	}
	if p.VCSRevision != "0123456789abcdef0123456789abcdef01234567" {
		t.Errorf("VCSRevision = %q", p.VCSRevision)
	}
	if !p.VCSTime.Equal(now) {
		t.Errorf("VCSTime = %v, want %v", p.VCSTime, now)
	}
	if p.VCSModified {
		t.Errorf("VCSModified = true, want false")
	}
	if len(p.Dependencies) != 2 {
		t.Fatalf("len(Dependencies) = %d, want 2", len(p.Dependencies))
	}
	if p.Dependencies[0].Path != "github.com/spf13/cobra" {
		t.Errorf("Dependencies[0].Path = %q", p.Dependencies[0].Path)
	}
	if p.Dependencies[0].Sum != "h1:abc" {
		t.Errorf("Dependencies[0].Sum = %q", p.Dependencies[0].Sum)
	}
	if len(p.Settings) != 6 {
		t.Fatalf("len(Settings) = %d, want 6", len(p.Settings))
	}
}

// TestProvenanceFromBuildInfo_Replace asserts the Replace field is
// rendered for both versioned and unversioned replaces.
func TestProvenanceFromBuildInfo_Replace(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{
				Path:    "github.com/old/pkg",
				Version: "v1.0.0",
				Replace: &debug.Module{Path: "github.com/new/pkg", Version: "v2.0.0"},
			},
			{
				Path:    "github.com/local/pkg",
				Version: "v1.0.0",
				Replace: &debug.Module{Path: "../local"},
			},
		},
	}
	p := version.ProvenanceForTest(info)
	if p.Dependencies[0].Replace != "github.com/new/pkg@v2.0.0" {
		t.Errorf("versioned Replace = %q", p.Dependencies[0].Replace)
	}
	if p.Dependencies[1].Replace != "../local" {
		t.Errorf("local Replace = %q", p.Dependencies[1].Replace)
	}
}

// TestProvenanceFromBuildInfo_VCSModifiedTrue asserts the boolean
// parses correctly for a modified tree.
func TestProvenanceFromBuildInfo_VCSModifiedTrue(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.modified", Value: "true"},
		},
	}
	p := version.ProvenanceForTest(info)
	if !p.VCSModified {
		t.Error("VCSModified = false, want true")
	}
}

// --- Negative ---------------------------------------------------------------

// TestProvenanceFromBuildInfo_NilBuildInfo asserts a nil
// *debug.BuildInfo yields a zero Provenance rather than a panic.
func TestProvenanceFromBuildInfo_NilBuildInfo(t *testing.T) {
	t.Parallel()
	p := version.ProvenanceForTest(nil)
	if p.GoVersion != "" || p.Module != "" || len(p.Dependencies) != 0 {
		t.Errorf("non-zero Provenance for nil BuildInfo: %+v", p)
	}
}

// TestProvenanceFromBuildInfo_NilDependencyElement asserts a nil
// element in Deps yields a zero Dependency rather than a panic.
func TestProvenanceFromBuildInfo_NilDependencyElement(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Deps: []*debug.Module{nil, {Path: "a", Version: "v1"}},
	}
	p := version.ProvenanceForTest(info)
	if len(p.Dependencies) != 2 {
		t.Fatalf("len = %d, want 2", len(p.Dependencies))
	}
	if p.Dependencies[0].Path != "" {
		t.Errorf("nil element produced %+v", p.Dependencies[0])
	}
}

// TestProvenanceFromBuildInfo_BadVCSTime asserts a malformed vcs.time
// yields the zero Time rather than an error or a panic. The reader
// tolerates the toolchain's format.
func TestProvenanceFromBuildInfo_BadVCSTime(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "not-a-timestamp"},
		},
	}
	p := version.ProvenanceForTest(info)
	if !p.VCSTime.IsZero() {
		t.Errorf("VCSTime = %v, want zero", p.VCSTime)
	}
}

// TestProvenanceFromBuildInfo_NonZeroVCSTimeKeepsUTC asserts the
// parsed time is always UTC, even if the toolchain emitted a
// non-UTC offset.
func TestProvenanceFromBuildInfo_NonZeroVCSTimeKeepsUTC(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "2026-05-15T11:30:00+02:00"},
		},
	}
	p := version.ProvenanceForTest(info)
	if p.VCSTime.Location() != time.UTC {
		t.Errorf("VCSTime location = %v, want UTC", p.VCSTime.Location())
	}
	if p.VCSTime.Hour() != 9 || p.VCSTime.Minute() != 30 {
		t.Errorf("VCSTime = %v, want 09:30 UTC", p.VCSTime)
	}
}

// --- Edge -------------------------------------------------------------------

// TestProvenanceFromBuildInfo_Empty asserts an empty BuildInfo yields
// a zero Provenance with no error.
func TestProvenanceFromBuildInfo_Empty(t *testing.T) {
	t.Parallel()
	p := version.ProvenanceForTest(&debug.BuildInfo{})
	if p.GoVersion != "" || p.Module != "" {
		t.Errorf("non-zero Provenance for empty BuildInfo: %+v", p)
	}
	if p.Dependencies != nil {
		t.Errorf("Dependencies = %v, want nil", p.Dependencies)
	}
	if p.Settings != nil {
		t.Errorf("Settings = %v, want nil", p.Settings)
	}
}

// TestProvenanceFromBuildInfo_NoDeps asserts a BuildInfo with no
// dependencies produces a nil Dependencies slice, not an empty one.
// A nil slice serializes to `null` in JSON; a consumer that expects
// `[]` needs to know the difference.
func TestProvenanceFromBuildInfo_NoDeps(t *testing.T) {
	t.Parallel()
	p := version.ProvenanceForTest(&debug.BuildInfo{GoVersion: "go1.23.4"})
	if p.Dependencies != nil {
		t.Errorf("Dependencies = %v, want nil", p.Dependencies)
	}
}

// TestSetting_Found asserts the Setting lookup returns the value.
func TestSetting_Found(t *testing.T) {
	t.Parallel()
	p := version.Provenance{
		Settings: []version.Setting{
			{Key: "-trimpath", Value: "true"},
			{Key: "CGO_ENABLED", Value: "0"},
		},
	}
	if got := p.Setting("-trimpath"); got != "true" {
		t.Errorf("Setting(-trimpath) = %q, want true", got)
	}
	if got := p.Setting("CGO_ENABLED"); got != "0" {
		t.Errorf("Setting(CGO_ENABLED) = %q, want 0", got)
	}
}

// TestSetting_Missing asserts an absent key yields the empty string.
func TestSetting_Missing(t *testing.T) {
	t.Parallel()
	p := version.Provenance{}
	if got := p.Setting("-trimpath"); got != "" {
		t.Errorf("Setting(-trimpath) = %q, want empty", got)
	}
}

// TestSetting_FirstMatchWins asserts a duplicate key returns the
// first occurrence. The runtime does not produce duplicates, but a
// test can.
func TestSetting_FirstMatchWins(t *testing.T) {
	t.Parallel()
	p := version.Provenance{
		Settings: []version.Setting{
			{Key: "x", Value: "first"},
			{Key: "x", Value: "second"},
		},
	}
	if got := p.Setting("x"); got != "first" {
		t.Errorf("Setting(x) = %q, want first", got)
	}
}

// TestTrimpath asserts the convenience method.
func TestTrimpath(t *testing.T) {
	t.Parallel()
	yes := version.Provenance{Settings: []version.Setting{{Key: "-trimpath", Value: "true"}}}
	if !yes.Trimpath() {
		t.Error("Trimpath() = false, want true")
	}
	no := version.Provenance{Settings: []version.Setting{{Key: "-trimpath", Value: "false"}}}
	if no.Trimpath() {
		t.Error("Trimpath() = true, want false")
	}
	missing := version.Provenance{}
	if missing.Trimpath() {
		t.Error("Trimpath() = true for missing setting, want false")
	}
}

// TestCGOEnabled asserts the convenience method.
func TestCGOEnabled(t *testing.T) {
	t.Parallel()
	yes := version.Provenance{Settings: []version.Setting{{Key: "CGO_ENABLED", Value: "1"}}}
	if !yes.CGOEnabled() {
		t.Error("CGOEnabled() = false, want true")
	}
	no := version.Provenance{Settings: []version.Setting{{Key: "CGO_ENABLED", Value: "0"}}}
	if no.CGOEnabled() {
		t.Error("CGOEnabled() = true, want false")
	}
}

// --- Corner -----------------------------------------------------------------

// TestProvenanceFromBuildInfo_CopiesStrings asserts the conversion
// does not retain a pointer into the runtime's internal state. The
// test mutates the input after the conversion and asserts the output
// is unchanged.
func TestProvenanceFromBuildInfo_CopiesStrings(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		GoVersion: "go1.23.4",
		Main:      debug.Module{Path: "a"},
		Deps: []*debug.Module{
			{Path: "b", Version: "v1"},
		},
	}
	p := version.ProvenanceForTest(info)
	info.GoVersion = "mutated"
	info.Main.Path = "mutated"
	info.Deps[0].Path = "mutated"
	if p.GoVersion != "go1.23.4" {
		t.Errorf("GoVersion mutated: %q", p.GoVersion)
	}
	if p.Module != "a" {
		t.Errorf("Module mutated: %q", p.Module)
	}
	if p.Dependencies[0].Path != "b" {
		t.Errorf("Dependencies[0].Path mutated: %q", p.Dependencies[0].Path)
	}
}

// --- Non-functional ---------------------------------------------------------

// TestReadProvenance_IsPure asserts two reads produce equal values.
func TestReadProvenance_IsPure(t *testing.T) {
	t.Parallel()
	a := version.ReadProvenance()
	b := version.ReadProvenance()
	if a.GoVersion != b.GoVersion || a.Module != b.Module || a.VCSRevision != b.VCSRevision {
		t.Errorf("ReadProvenance is not pure: %+v vs %+v", a, b)
	}
}

// TestReadProvenance_Concurrent asserts concurrent reads do not race.
func TestReadProvenance_Concurrent(t *testing.T) {
	t.Parallel()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 100; j++ {
				_ = version.ReadProvenance()
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestDependencies_StableOrderAcrossCalls asserts two reads of the
// test binary's provenance produce the same dependency order. The
// runtime's order is deterministic for a given binary; this test
// asserts Forge does not reorder it.
func TestDependencies_StableOrderAcrossCalls(t *testing.T) {
	t.Parallel()
	a := version.ReadProvenance()
	b := version.ReadProvenance()
	if len(a.Dependencies) != len(b.Dependencies) {
		t.Fatalf("lengths differ: %d vs %d", len(a.Dependencies), len(b.Dependencies))
	}
	for i := range a.Dependencies {
		if a.Dependencies[i].Path != b.Dependencies[i].Path {
			t.Errorf("dependency %d order differs", i)
		}
	}
}

// TestDependencies_SortableByPath asserts the caller can sort the
// slice by path. A consumer that lists dependencies sorts them; the
// reader does not, so the caller controls the order.
func TestDependencies_SortableByPath(t *testing.T) {
	t.Parallel()
	deps := []version.Dependency{
		{Path: "z"},
		{Path: "a"},
		{Path: "m"},
	}
	sort.Slice(deps, func(i, j int) bool { return deps[i].Path < deps[j].Path })
	if deps[0].Path != "a" || deps[1].Path != "m" || deps[2].Path != "z" {
		t.Errorf("sort failed: %+v", deps)
	}
}
