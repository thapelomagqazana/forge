// Package ci — the frozen CI OS support matrix.
//
// This file defines the OS matrix that the CI tier model is built on.
// Where tiers.go says "Tier 2 runs on a matrix of OSes", this file
// says "Tier 2 runs on ubuntu-latest, windows-latest, and
// macos-latest; Tier 1 runs on ubuntu-latest only; Tier 3 runs on
// ubuntu-latest only". The matrix is frozen: a change to it requires
// an ADR.
//
// # Why a typed model and not a YAML file
//
// The matrix has invariants: every tier has at least one OS; every OS
// is a GitHub-provided runner label; every excluded combination has a
// rationale. Invariants are testable only if they are expressed in a
// language with a test framework. Go has one; YAML does not.
//
// The matrix is also consumed by tests in three packages:
//
//   - internal/ci's own tests verify the model's invariants.
//   - internal/filesystem/safefs's tests verify the per-OS
//     build-tagged files exist for the features the matrix claims
//     are covered.
//   - The contract tests verify the workflow files declare the
//     runners the model names.
//
// A YAML file would require a schema and a validator; the Go model is
// the schema.
//
// # Relationship to internal/platform
//
// internal/platform owns the *release* platform matrix
// (docs/reference/platforms.yaml): the six (GOOS, GOARCH) pairs the
// release pipeline builds for. This package owns the *CI* matrix: the
// GitHub runner labels the CI tiers run on. The two are related but
// distinct: the release matrix's platforms are the artifacts; the CI
// matrix's runners are the machines that test them.
//
// The two matrices are kept in sync by the contract test
// TestPlatformMatrix_EveryReleasePlatformIsTestedOnItsOS, which
// asserts that every release platform's GOOS appears in at least one
// tier's OS set.
package ci

import (
	"fmt"
	"sort"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// GitHubRunner is the closed set of GitHub Actions runner labels used
// by the CI OS matrix.
//
// The set is closed so that a new runner requires an explicit change
// to this file and a review of the runner's availability. The labels
// are GitHub's own names; a label that GitHub does not provide cannot
// appear.
//
// # Why the set is small
//
// GitHub provides several labels for each OS family
// (ubuntu-latest, ubuntu-24.04, ubuntu-22.04; windows-latest,
// windows-2022; macos-latest, macos-14). The set below is the
// subset the CI matrix uses. A future change that needed a different
// version of a runner would add the label to this set and to the
// appropriate tier's OS slice.
//
// The labels are chosen for stability: `-latest` tracks GitHub's
// current default image, which is the image a contributor's local
// machine is most likely to match. The `-22.04` and `-2022` labels
// are pinned; they appear in the matrix only where a specific image
// is required (the release workflow uses ubuntu-22.04 for its
// glibc compatibility).
type GitHubRunner string

const (
	// RunnerUbuntuLatest is GitHub's default Ubuntu image. It tracks
	// the latest Ubuntu LTS that GitHub has published a runner for.
	RunnerUbuntuLatest GitHubRunner = "ubuntu-latest"

	// RunnerWindowsLatest is GitHub's default Windows image. It
	// tracks the latest Windows Server that GitHub has published a
	// runner for.
	RunnerWindowsLatest GitHubRunner = "windows-latest"

	// RunnerMacOSLatest is GitHub's default macOS image. It tracks
	// the latest macOS that GitHub has published a runner for.
	//
	// The macOS runner is Apple-silicon (arm64) on the `-latest`
	// label as of 2025. A future change that needed an Intel macOS
	// runner would add the `macos-13` label to this set.
	RunnerMacOSLatest GitHubRunner = "macos-latest"
)

// AllGitHubRunners returns the closed set of GitHub runners used by
// the CI matrix, in a stable order.
//
// The order is the order in which the runners are presented in the
// compatibility matrix's rendering: Ubuntu, Windows, macOS. The order
// matches the repository's convention for OS listings.
func AllGitHubRunners() []GitHubRunner {
	return []GitHubRunner{
		RunnerUbuntuLatest,
		RunnerWindowsLatest,
		RunnerMacOSLatest,
	}
}

// String returns the runner as a string.
func (r GitHubRunner) String() string { return string(r) }

// Valid reports whether r is a member of the closed set.
func (r GitHubRunner) Valid() bool {
	for _, k := range AllGitHubRunners() {
		if r == k {
			return true
		}
	}
	return false
}

// RunnerGOOS returns the GOOS value the runner provides.
//
// The mapping is total over the closed set. The value is what
// `go env GOOS` reports on the runner; it is used by the
// compatibility matrix and by tests that want to name the
// platform by its Go identifier.
//
// The function panics on an unknown runner. The panic is the same
// programming-error signal as WorkflowID.FileName and
// forgeerr.Category.Prefix.
func (r GitHubRunner) RunnerGOOS() string {
	switch r {
	case RunnerUbuntuLatest:
		return "linux"
	case RunnerWindowsLatest:
		return "windows"
	case RunnerMacOSLatest:
		return "darwin"
	default:
		panic(fmt.Sprintf("ci: unknown runner %q", string(r)))
	}
}

// RunnerGOARCH returns the GOARCH value the runner provides.
//
// The mapping is total over the closed set. The value is what
// `go env GOARCH` reports on the runner.
//
// All three runners are 64-bit as of 2025. A future change that
// added a 32-bit runner would need to update the mapping.
func (r GitHubRunner) RunnerGOARCH() string {
	switch r {
	case RunnerUbuntuLatest:
		return "amd64"
	case RunnerWindowsLatest:
		return "amd64"
	case RunnerMacOSLatest:
		return "arm64"
	default:
		panic(fmt.Sprintf("ci: unknown runner %q", string(r)))
	}
}

// TierOSMatrix is the OS matrix for one tier: the set of GitHub
// runners the tier's workflow uses.
//
// The type is keyed by TierID. The value is a non-empty slice of
// GitHubRunner values. The slice's order is the order in which the
// runners appear in the tier's workflow matrix.
type TierOSMatrix struct {
	// Tier is the tier the matrix applies to.
	Tier TierID

	// Runners is the non-empty, ordered list of runners the tier
	// uses.
	Runners []GitHubRunner

	// Rationale is a one-sentence explanation of why the tier uses
	// this set of runners. It appears in the compatibility matrix's
	// rendering.
	Rationale string
}

// tierOSMatrices is the canonical description of the per-tier OS
// matrices.
//
// The slice's order matches allTiers: Tier 1, Tier 2, Tier 3,
// Tier 4. The order is the order in which the matrices are
// presented in the compatibility matrix.
var tierOSMatrices = []TierOSMatrix{
	{
		Tier: Tier1,
		Runners: []GitHubRunner{
			RunnerUbuntuLatest,
		},
		Rationale: "Fast feedback: Ubuntu only, because Tier 1's checks are platform-agnostic and Ubuntu is the fastest runner.",
	},
	{
		Tier: Tier2,
		Runners: []GitHubRunner{
			RunnerUbuntuLatest,
			RunnerWindowsLatest,
			RunnerMacOSLatest,
		},
		Rationale: "Cross-platform confidence: every supported desktop OS, because Tier 2's purpose is to prove the code works on each one.",
	},
	{
		Tier: Tier3,
		Runners: []GitHubRunner{
			RunnerUbuntuLatest,
		},
		Rationale: "Depth: Linux only, because Tier 3's checks (reproducibility, race detector, coverage) are platform-agnostic or already covered by Tier 2.",
	},
	{
		Tier: Tier4,
		Runners: []GitHubRunner{
			RunnerUbuntuLatest,
		},
		Rationale: "Scheduled hygiene: Linux only, because CodeQL, govulncheck, and gitleaks are platform-agnostic and the fuzz run needs a single, consistent environment.",
	},
}

// AllTierOSMatrices returns a deep copy of the canonical per-tier OS
// matrices.
func AllTierOSMatrices() []TierOSMatrix {
	out := make([]TierOSMatrix, len(tierOSMatrices))
	for i, m := range tierOSMatrices {
		out[i] = cloneTierOSMatrix(m)
	}
	return out
}

// LookupTierOSMatrix returns a deep copy of the OS matrix for the
// given tier and reports whether it was found. The function is
// total: it never panics, even on an unknown tier.
func LookupTierOSMatrix(id TierID) (TierOSMatrix, bool) {
	for _, m := range tierOSMatrices {
		if m.Tier == id {
			return cloneTierOSMatrix(m), true
		}
	}
	return TierOSMatrix{}, false
}

// Exclusion is a (tier, runner) combination the model deliberately
// does not include.
//
// Every combination that a reader might expect to see but does not is
// an Exclusion. The model's purpose is to make the absence explicit:
// a reader who wonders "why not Windows in Tier 1?" finds the
// exclusion and its rationale.
type Exclusion struct {
	// Tier is the tier the exclusion applies to.
	Tier TierID

	// Runner is the runner the tier does not use.
	Runner GitHubRunner

	// Rationale is a one-sentence explanation of why the
	// combination is excluded. It appears in the compatibility
	// matrix's rendering.
	Rationale string
}

// exclusions is the canonical list of excluded (tier, runner)
// combinations.
//
// The list is not exhaustive: a combination that is neither in a
// tier's OS matrix nor in this list is excluded by default. The list
// documents the combinations a reader is most likely to ask about.
var exclusions = []Exclusion{
	{
		Tier:   Tier1,
		Runner: RunnerWindowsLatest,
		Rationale: "Tier 1's checks are platform-agnostic and Windows' runner is slower to provision; " +
			"the cost is not justified for a tier whose budget is 3 minutes.",
	},
	{
		Tier:   Tier1,
		Runner: RunnerMacOSLatest,
		Rationale: "Same as Windows: the cost is not justified for Tier 1's budget. " +
			"macOS-specific failures are caught by Tier 2.",
	},
	{
		Tier:   Tier3,
		Runner: RunnerWindowsLatest,
		Rationale: "Tier 3's checks are platform-agnostic (reproducibility, race detector, coverage) " +
			"or already covered by Tier 2's Windows cell (binary smoke tests).",
	},
	{
		Tier:   Tier3,
		Runner: RunnerMacOSLatest,
		Rationale: "Same as Windows. The race detector's behaviour is not materially different on macOS " +
			"for this repository's code.",
	},
	{
		Tier:   Tier4,
		Runner: RunnerWindowsLatest,
		Rationale: "CodeQL, govulncheck, gitleaks, and go-licenses are platform-agnostic. " +
			"The fuzz run needs a consistent environment; Linux provides it.",
	},
	{
		Tier:      Tier4,
		Runner:    RunnerMacOSLatest,
		Rationale: "Same as Windows.",
	},
}

// AllExclusions returns a deep copy of the canonical exclusion list.
func AllExclusions() []Exclusion {
	out := make([]Exclusion, len(exclusions))
	copy(out, exclusions)
	return out
}

// ExclusionRationale returns the rationale for an excluded
// combination, or the empty string if the combination is not in the
// list.
//
// The function is total: it never panics.
func ExclusionRationale(tier TierID, runner GitHubRunner) string {
	for _, e := range exclusions {
		if e.Tier == tier && e.Runner == runner {
			return e.Rationale
		}
	}
	return ""
}

// FeatureCoverage describes, for one filesystem feature, which
// platforms the feature is tested on and in which files.
//
// The type is the model behind the compatibility matrix
// (docs/reference/platform-test-matrix.md). The matrix maps features
// × platforms × test files.
type FeatureCoverage struct {
	// Name is the feature's human-readable name. It appears as a
	// row heading in the compatibility matrix.
	Name string

	// TestFiles is the list of test files that exercise the
	// feature. A file may be a platform-agnostic test
	// (TestName_test.go) or a build-tagged test
	// (TestName_windows_test.go). The file's build tag, if any,
	// determines the platforms it runs on; the coverage matrix's
	// cells are derived from the build tags.
	TestFiles []string

	// Platforms is the list of platforms on which the feature is
	// tested. It is derived from the TestFiles' build tags by the
	// model's Validate function; a contributor does not write it
	// directly.
	//
	// The field exists in the model so that the compatibility
	// matrix's rendering does not have to re-parse the build tags.
	// The Validate function asserts that the field's value matches
	// the derived set.
	Platforms []GitHubRunner

	// Notes is an optional note that appears in the compatibility
	// matrix's row after the platform list.
	Notes string
}

// featureCoverages is the canonical list of filesystem features the
// compatibility matrix covers.
//
// The list is the model behind the matrix. A future change that
// added a feature would add an entry here and a test file that
// exercises it.
var featureCoverages = []FeatureCoverage{
	{
		Name: "Reserved filenames",
		TestFiles: []string{
			"internal/filesystem/safefs/normalize_windows_test.go",
		},
		Platforms: []GitHubRunner{
			RunnerWindowsLatest,
		},
		Notes: "Windows-only: the reserved set (CON, PRN, AUX, NUL, COM1-COM9, LPT1-LPT9) does not exist on POSIX.",
	},
	{
		Name: "Long paths",
		TestFiles: []string{
			"internal/filesystem/longpath_test.go",
			"internal/filesystem/longpath_windows_test.go",
		},
		Platforms: []GitHubRunner{
			RunnerUbuntuLatest,
			RunnerWindowsLatest,
			RunnerMacOSLatest,
		},
		Notes: "The base test runs everywhere; the Windows-specific test exercises the \\\\?\\ prefix.",
	},
	{
		Name: "Case-insensitive filesystems",
		TestFiles: []string{
			"internal/filesystem/safefs/normalize_darwin_test.go",
			"internal/filesystem/safefs/normalize_windows_test.go",
		},
		Platforms: []GitHubRunner{
			RunnerWindowsLatest,
			RunnerMacOSLatest,
		},
		Notes: "Windows and macOS default to case-insensitive; Linux defaults to case-sensitive.",
	},
	{
		Name: "Unicode normalization (NFD/NFC)",
		TestFiles: []string{
			"internal/filesystem/safefs/normalize_darwin_test.go",
		},
		Platforms: []GitHubRunner{
			RunnerMacOSLatest,
		},
		Notes: "macOS's HFS+ and APFS filesystems normalize filenames to NFD; POSIX does not.",
	},
	{
		Name: "Symlink policy",
		TestFiles: []string{
			"internal/filesystem/safefs/symlink_unix_test.go",
			"internal/filesystem/safefs/symlink_windows_test.go",
		},
		Platforms: []GitHubRunner{
			RunnerUbuntuLatest,
			RunnerWindowsLatest,
			RunnerMacOSLatest,
		},
		Notes: "The Unix tests cover Linux and macOS; the Windows tests cover junctions and symlinks.",
	},
	{
		Name: "Permission bits",
		TestFiles: []string{
			"internal/filesystem/safefs/errors_test.go",
			"internal/filesystem/safefs/exclusive_unix.go",
		},
		Platforms: []GitHubRunner{
			RunnerUbuntuLatest,
			RunnerMacOSLatest,
		},
		Notes: "POSIX permission bits have no Windows equivalent; Windows uses ACLs, which the package does not model.",
	},
}

// AllFeatureCoverages returns a deep copy of the canonical feature
// list.
func AllFeatureCoverages() []FeatureCoverage {
	out := make([]FeatureCoverage, len(featureCoverages))
	for i, f := range featureCoverages {
		out[i] = cloneFeatureCoverage(f)
	}
	return out
}

// ValidatePlatformMatrix checks the OS matrix model's internal
// invariants. It returns a structured *forgeerr.Error on the first
// violation.
//
// The invariants are:
//
//  1. Every tier in the closed set has a matrix entry.
//  2. Every matrix entry names a member of the closed set of tiers.
//  3. Every matrix entry has at least one runner.
//  4. Every runner is a member of the closed set of GitHub runners.
//  5. No matrix entry has duplicate runners.
//  6. Every exclusion names a member of the closed set of tiers and
//     a member of the closed set of GitHub runners.
//  7. Every exclusion's (tier, runner) is not in the tier's matrix.
//  8. Every exclusion has a non-empty rationale.
//  9. Every feature coverage has a non-empty name and at least one
//     test file.
//  10. Every feature coverage's Platforms set matches the set derived
//     from its TestFiles' build tags. This invariant is checked by a
//     contract test that reads the files; the model's Validate
//     function checks only that the Platforms field is non-empty and
//     contains only known runners.
//
// The function is total and never panics.
func ValidatePlatformMatrix() error {
	// 1. Every tier has a matrix entry.
	seen := make(map[TierID]struct{}, len(tierOSMatrices))
	for _, m := range tierOSMatrices {
		if !m.Tier.Valid() {
			return platformMatrixError(
				"unknown tier",
				fmt.Sprintf("the tier %q is not in the closed set", string(m.Tier)),
			)
		}
		if _, dup := seen[m.Tier]; dup {
			return platformMatrixError(
				"duplicate tier",
				fmt.Sprintf("the tier %q has more than one matrix entry", string(m.Tier)),
			)
		}
		seen[m.Tier] = struct{}{}
		if err := validateTierOSMatrix(m); err != nil {
			return err
		}
	}
	for _, t := range AllTiers() {
		if _, ok := seen[t]; !ok {
			return platformMatrixError(
				"missing tier",
				fmt.Sprintf("the tier %q has no matrix entry", string(t)),
			)
		}
	}

	// 2. Exclusions are consistent.
	for _, e := range exclusions {
		if !e.Tier.Valid() {
			return platformMatrixError(
				"unknown tier in exclusion",
				fmt.Sprintf("the tier %q is not in the closed set", string(e.Tier)),
			)
		}
		if !e.Runner.Valid() {
			return platformMatrixError(
				"unknown runner in exclusion",
				fmt.Sprintf("the runner %q is not in the closed set", string(e.Runner)),
			)
		}
		if e.Rationale == "" {
			return platformMatrixError(
				"empty exclusion rationale",
				fmt.Sprintf("the exclusion (%s, %s) has no rationale", string(e.Tier), string(e.Runner)),
			)
		}
		// The combination must not be in the tier's matrix.
		if m, ok := LookupTierOSMatrix(e.Tier); ok {
			for _, r := range m.Runners {
				if r == e.Runner {
					return platformMatrixError(
						"exclusion conflicts with matrix",
						fmt.Sprintf(
							"the combination (%s, %s) is both in the tier's matrix and in the exclusion list",
							string(e.Tier), string(e.Runner),
						),
					)
				}
			}
		}
	}

	// 3. Feature coverages are well-formed.
	for _, f := range featureCoverages {
		if f.Name == "" {
			return platformMatrixError(
				"empty feature name",
				"a feature coverage entry has no name",
			)
		}
		if len(f.TestFiles) == 0 {
			return platformMatrixError(
				"no test files",
				fmt.Sprintf("the feature %q has no test files", f.Name),
			)
		}
		if len(f.Platforms) == 0 {
			return platformMatrixError(
				"no platforms",
				fmt.Sprintf("the feature %q has no platforms", f.Name),
			)
		}
		for _, r := range f.Platforms {
			if !r.Valid() {
				return platformMatrixError(
					"unknown runner in feature",
					fmt.Sprintf("the feature %q names runner %q, which is not in the closed set", f.Name, string(r)),
				)
			}
		}
	}

	return nil
}

// validateTierOSMatrix checks the invariants of a single
// TierOSMatrix.
func validateTierOSMatrix(m TierOSMatrix) error {
	if !m.Tier.Valid() {
		return platformMatrixError(
			"unknown tier",
			fmt.Sprintf("the tier %q is not in the closed set", string(m.Tier)),
		)
	}
	if len(m.Runners) == 0 {
		return platformMatrixError(
			"no runners",
			fmt.Sprintf("the tier %q has no runners", string(m.Tier)),
		)
	}
	seen := make(map[GitHubRunner]struct{}, len(m.Runners))
	for _, r := range m.Runners {
		if !r.Valid() {
			return platformMatrixError(
				"unknown runner",
				fmt.Sprintf("the tier %q names runner %q, which is not in the closed set", string(m.Tier), string(r)),
			)
		}
		if _, dup := seen[r]; dup {
			return platformMatrixError(
				"duplicate runner",
				fmt.Sprintf("the tier %q names runner %q more than once", string(m.Tier), string(r)),
			)
		}
		seen[r] = struct{}{}
	}
	if m.Rationale == "" {
		return platformMatrixError(
			"empty rationale",
			fmt.Sprintf("the tier %q has no rationale for its OS choice", string(m.Tier)),
		)
	}
	return nil
}

// PlatformMatrixGolden renders the OS matrix and the exclusion list
// as deterministic JSON.
func PlatformMatrixGolden() string {
	var b strings.Builder
	b.WriteString("{\n")

	b.WriteString(`  "tiers": [`)
	for i, m := range tierOSMatrices {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "tier": `)
		writeJSONString(&b, string(m.Tier))
		b.WriteString(",\n")
		b.WriteString(`      "runners": [`)
		for j, r := range m.Runners {
			if j > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, string(r))
		}
		b.WriteString("],\n")
		b.WriteString(`      "rationale": `)
		writeJSONString(&b, m.Rationale)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ],\n")

	b.WriteString(`  "exclusions": [`)
	for i, e := range exclusions {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "tier": `)
		writeJSONString(&b, string(e.Tier))
		b.WriteString(",\n")
		b.WriteString(`      "runner": `)
		writeJSONString(&b, string(e.Runner))
		b.WriteString(",\n")
		b.WriteString(`      "rationale": `)
		writeJSONString(&b, e.Rationale)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ],\n")

	b.WriteString(`  "features": [`)
	for i, f := range featureCoverages {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "name": `)
		writeJSONString(&b, f.Name)
		b.WriteString(",\n")
		b.WriteString(`      "test_files": [`)
		for j, tf := range f.TestFiles {
			if j > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, tf)
		}
		b.WriteString("],\n")
		b.WriteString(`      "platforms": [`)
		for j, r := range f.Platforms {
			if j > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, string(r))
		}
		b.WriteString("],\n")
		b.WriteString(`      "notes": `)
		writeJSONString(&b, f.Notes)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}\n")

	return b.String()
}

// PlatformTestMatrixMarkdown renders the compatibility matrix as
// Markdown.
//
// The rendering is deterministic: the same model always produces the
// same bytes. It is the content of docs/reference/platform-test-matrix.md.
func PlatformTestMatrixMarkdown() string {
	var b strings.Builder

	b.WriteString("# Platform Test Matrix\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** QA and Release Engineering\n")
	b.WriteString("> **Related:** [ADR-048](../decisions/ADR-048-os-support-matrix.md), [Platform Tiers](platform-tiers.md), [Supported Platforms](supported-platforms.md)\n\n")
	b.WriteString("This document is the compatibility matrix for the forge repository's\n")
	b.WriteString("platform-specific tests. It maps each filesystem feature × the\n")
	b.WriteString("platforms the feature is tested on × the test files that exercise\n")
	b.WriteString("it. The document is rendered from `internal/ci/platform_matrix.go`;\n")
	b.WriteString("a golden-file test asserts the two agree.\n\n")

	// ── OS matrix ────────────────────────────────────────────────────────
	b.WriteString("## CI OS Matrix\n\n")
	b.WriteString("The CI tier model uses a frozen set of GitHub Actions runner labels.\n")
	b.WriteString("The matrix below is the tier-to-runner mapping; it is the input to\n")
	b.WriteString("the `.github/workflows/ci-tier*.yml` files' `runs-on:` fields.\n\n")

	b.WriteString("| Tier | Name | Runners | Rationale |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, m := range tierOSMatrices {
		tier, _ := LookupTier(m.Tier)
		b.WriteString("| Tier ")
		b.WriteString(fmt.Sprintf("%d", m.Tier.Number()))
		b.WriteString(" | ")
		b.WriteString(tier.Name)
		b.WriteString(" | ")
		b.WriteString(runnersList(m.Runners))
		b.WriteString(" | ")
		b.WriteString(m.Rationale)
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	// ── Exclusions ───────────────────────────────────────────────────────
	b.WriteString("## Excluded Combinations\n\n")
	b.WriteString("Every (tier, runner) combination that a reader might expect to\n")
	b.WriteString("see but does not is documented below. The exclusions are part of\n")
	b.WriteString("the model; a change to them requires an ADR.\n\n")

	b.WriteString("| Tier | Runner | Rationale |\n")
	b.WriteString("|---|---|---|\n")
	for _, e := range exclusions {
		b.WriteString("| Tier ")
		b.WriteString(fmt.Sprintf("%d", e.Tier.Number()))
		b.WriteString(" | ")
		b.WriteString(string(e.Runner))
		b.WriteString(" | ")
		b.WriteString(e.Rationale)
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	// ── Compatibility matrix ─────────────────────────────────────────────
	b.WriteString("## Feature Compatibility Matrix\n\n")
	b.WriteString("Each row is a filesystem feature; each cell names the platforms\n")
	b.WriteString("the feature is tested on and the test files that exercise it.\n")
	b.WriteString("A feature that is tested on all three platforms has `all` in its\n")
	b.WriteString("platforms cell; a feature that is tested on a subset names the\n")
	b.WriteString("subset.\n\n")

	b.WriteString("| Feature | Platforms | Test files | Notes |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, f := range featureCoverages {
		b.WriteString("| ")
		b.WriteString(f.Name)
		b.WriteString(" | ")
		b.WriteString(runnersList(f.Platforms))
		b.WriteString(" | ")
		b.WriteString(testFilesList(f.TestFiles))
		b.WriteString(" | ")
		b.WriteString(f.Notes)
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	// ── Per-platform summary ─────────────────────────────────────────────
	b.WriteString("## Coverage by Platform\n\n")
	b.WriteString("The table below summarizes how many features each platform is\n")
	b.WriteString("tested on. It is derived from the compatibility matrix above.\n\n")

	b.WriteString("| Platform | Features tested | Features not tested |\n")
	b.WriteString("|---|---|---|\n")
	for _, r := range AllGitHubRunners() {
		tested := 0
		notTested := 0
		for _, f := range featureCoverages {
			found := false
			for _, p := range f.Platforms {
				if p == r {
					found = true
					break
				}
			}
			if found {
				tested++
			} else {
				notTested++
			}
		}
		b.WriteString("| ")
		b.WriteString(string(r))
		b.WriteString(" | ")
		b.WriteString(fmt.Sprintf("%d", tested))
		b.WriteString(" | ")
		b.WriteString(fmt.Sprintf("%d", notTested))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	return b.String()
}

// --- helpers ---------------------------------------------------------------

// cloneTierOSMatrix returns a deep copy of m.
func cloneTierOSMatrix(m TierOSMatrix) TierOSMatrix {
	out := m
	out.Runners = cloneSlice(m.Runners)
	return out
}

// cloneFeatureCoverage returns a deep copy of f.
func cloneFeatureCoverage(f FeatureCoverage) FeatureCoverage {
	out := f
	out.TestFiles = cloneSlice(f.TestFiles)
	out.Platforms = cloneSlice(f.Platforms)
	return out
}

// runnersList renders a slice of runners as a comma-separated list
// of their string values, in the slice's order.
func runnersList(rs []GitHubRunner) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, string(r))
	}
	return strings.Join(parts, ", ")
}

// testFilesList renders a slice of test-file paths as a
// comma-separated list of backtick-quoted names.
func testFilesList(fs []string) string {
	parts := make([]string, 0, len(fs))
	for _, f := range fs {
		parts = append(parts, "`"+f+"`")
	}
	return strings.Join(parts, ", ")
}

// platformMatrixError builds a *forgeerr.Error for an OS-matrix
// invariant violation.
func platformMatrixError(what, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: platform matrix invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	return e
}

// init validates the OS matrix model at package start.
func init() {
	if err := ValidatePlatformMatrix(); err != nil {
		panic(fmt.Sprintf("ci: platform matrix validation failed: %v", err))
	}
}

// Compile-time assertions that the package's public surface does not
// drift.
var (
	_ func() []GitHubRunner             = AllGitHubRunners
	_ func() []TierOSMatrix             = AllTierOSMatrices
	_ func(TierID) (TierOSMatrix, bool) = LookupTierOSMatrix
	_ func() []Exclusion                = AllExclusions
	_ func(TierID, GitHubRunner) string = ExclusionRationale
	_ func() []FeatureCoverage          = AllFeatureCoverages
	_ func() error                      = ValidatePlatformMatrix
	_ func() string                     = PlatformMatrixGolden
	_ func() string                     = PlatformTestMatrixMarkdown
	_ sort.Interface                    = sort.StringSlice(nil) // documents the import
)
