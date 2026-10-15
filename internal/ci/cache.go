// Package ci — the typed cache model for the CI tiers.
//
// This file defines the cache strategy that the three tier workflows
// use. It captures what the workflows need to know and what the
// documentation needs to render: the three cache types, their key
// and restore-key templates, the version prefix that forces a clear,
// the hit-rate targets, and the branch-scope policy.
//
// The model is small on purpose. It does not describe every step of
// a workflow; it describes the parts of the cache strategy that are
// contracts: the key templates (which the contract tests check
// against the YAML), the targets (which the strategy document
// renders), and the invalidation variable (which a contributor
// changes to force a clear).
//
// # Why the model is in Go and not in YAML
//
// The cache keys are strings with substitution syntax. The workflow
// files contain the substituted form; the model contains the
// template. The contract test asserts the two agree. A YAML model
// would put the templates in a second YAML file, which a reviewer
// would have to read alongside the workflow; the Go model is
// importable by the tests and renderable by the documentation
// generator.
//
// # Relationship to the tier model
//
// internal/ci/tiers.go declares the four tiers and their budgets.
// This file declares the caches the tiers use to meet their budgets.
// The two are related: a tier's budget is achievable because its
// caches are effective. The cache strategy document
// (docs/reference/ci-cache-strategy.md) explains the relationship.
package ci

import (
	"fmt"
	"strings"

	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// CacheType is the closed set of cache types used by the CI tiers.
//
// The set is closed so that a new cache type requires an explicit
// change to this file and a review of the cache's key and
// invalidation strategy. The three current types are:
//
//   - the Go module cache (~/go/pkg/mod on Unix, %GOPATH%\pkg\mod on
//     Windows);
//   - the Go build cache (~/.cache/go-build on Unix,
//     %LocalAppData%\go-build on Windows);
//   - the golangci-lint binary cache (the installed binary's
//     directory, ~/go/bin or the action's own cache).
type CacheType string

const (
	// CacheModule is the Go module cache. It stores the extracted
	// sources of every module the repository depends on. It is
	// keyed on go.sum's hash: a dependency change invalidates the
	// cache.
	CacheModule CacheType = "module"

	// CacheBuild is the Go build cache. It stores compiled object
	// files and other intermediate artifacts. It is keyed on the
	// repository's Go sources and go.sum; a source change
	// invalidates the cache.
	CacheBuild CacheType = "build"

	// CacheLint is the golangci-lint binary cache. It stores the
	// installed binary. It is keyed on the pinned linter version;
	// a version bump invalidates the cache.
	CacheLint CacheType = "lint"
)

// AllCacheTypes returns the closed set of cache types, in the order
// in which they are presented in the strategy document: module,
// build, lint.
func AllCacheTypes() []CacheType {
	return []CacheType{CacheModule, CacheBuild, CacheLint}
}

// String returns the cache type as a string.
func (c CacheType) String() string { return string(c) }

// Valid reports whether c is a member of the closed set.
func (c CacheType) Valid() bool {
	for _, k := range AllCacheTypes() {
		if c == k {
			return true
		}
	}
	return false
}

// CacheSpec is the typed description of one cache type.
type CacheSpec struct {
	// Type is the cache type's identifier.
	Type CacheType

	// Paths is the list of directories the cache stores. The paths
	// may contain GitHub Actions substitution syntax (`${{
	// runner.os }}`, `${{ env.HOME }}`); the substituted form
	// appears in the workflow file.
	Paths []string

	// KeyTemplate is the cache key template. It may contain GitHub
	// Actions substitution syntax. The template is the contract the
	// contract test asserts.
	KeyTemplate string

	// RestoreKeyTemplate is the restore-key template. It is a
	// prefix of the key template (without the hash component), so
	// that a change to the hash component restores the previous
	// cache rather than missing entirely.
	RestoreKeyTemplate string

	// HitRateTarget is the target cache hit rate, in [0, 1]. It is
	// the rate the strategy document publishes and the metric
	// collector (a future task) measures against.
	HitRateTarget float64

	// Note is a one-sentence explanation of the cache's purpose. It
	// appears in the strategy document.
	Note string
}

// cacheSpecs is the canonical description of the three cache types.
var cacheSpecs = []CacheSpec{
	{
		Type: CacheModule,
		Paths: []string{
			"~/go/pkg/mod",
			"${{ env.HOME }}/go/pkg/mod",
		},
		KeyTemplate:        "${{ runner.os }}-go-${{ env.CACHE_VERSION }}-${{ hashFiles('**/go.sum') }}",
		RestoreKeyTemplate: "${{ runner.os }}-go-${{ env.CACHE_VERSION }}-",
		HitRateTarget:      0.90,
		Note: "Stores the extracted sources of every module the repository depends on; " +
			"keyed on go.sum's hash so that a dependency change invalidates the cache.",
	},
	{
		Type: CacheBuild,
		Paths: []string{
			"~/.cache/go-build",
			"${{ env.HOME }}/.cache/go-build",
			"${{ env.LOCALAPPDATA }}/go-build",
		},
		KeyTemplate:        "${{ runner.os }}-${{ inputs.go-version }}-build-${{ env.CACHE_VERSION }}-${{ hashFiles('**/*.go', '**/go.sum') }}",
		RestoreKeyTemplate: "${{ runner.os }}-${{ inputs.go-version }}-build-${{ env.CACHE_VERSION }}-",
		HitRateTarget:      0.60,
		Note: "Stores compiled object files and intermediate artifacts; " +
			"keyed on the repository's Go sources and go.sum so that a code change invalidates the cache.",
	},
	{
		Type: CacheLint,
		Paths: []string{
			"~/go/bin/golangci-lint",
			"${{ env.HOME }}/go/bin/golangci-lint",
		},
		KeyTemplate:        "${{ runner.os }}-golangci-lint-${{ env.CACHE_VERSION }}-${{ env.GOLANGCI_LINT_VERSION }}",
		RestoreKeyTemplate: "${{ runner.os }}-golangci-lint-${{ env.CACHE_VERSION }}-",
		HitRateTarget:      0.90,
		Note: "Stores the golangci-lint binary; " +
			"keyed on the pinned linter version so that a version bump invalidates the cache.",
	},
}

// AllCacheSpecs returns a deep copy of the canonical cache specs.
func AllCacheSpecs() []CacheSpec {
	out := make([]CacheSpec, len(cacheSpecs))
	for i, s := range cacheSpecs {
		out[i] = cloneCacheSpec(s)
	}
	return out
}

// LookupCacheSpec returns a deep copy of the cache spec with the
// given type and reports whether it was found.
func LookupCacheSpec(t CacheType) (CacheSpec, bool) {
	for _, s := range cacheSpecs {
		if s.Type == t {
			return cloneCacheSpec(s), true
		}
	}
	return CacheSpec{}, false
}

// CacheVersion is the version prefix that every cache key includes.
//
// A contributor who wants to force-clear every cache bumps this
// value (v1 -> v2) and commits. The next CI run sees a cache miss on
// every key and rebuilds from scratch. The change is one line and is
// visible in the diff.
//
// The value is a string, not a number, so that a future change that
// wanted a more descriptive version (v1-alpha, v2-2025-01) is a
// string change, not a type change.
//
// # Why the value is not an input to the reusable workflow
//
// An input would allow a caller to change the version, which would
// defeat the purpose: two callers with different CACHE_VERSION
// values would not share caches. The value's location in the
// reusable workflow is the single point of control. The constant
// here is the model's record of the value; the workflow file's
// `env:` block is the execution's source.
const CacheVersion = "v1"

// GolangciLintVersion is the pinned golangci-lint version.
//
// The value matches the version in ci-tier1.yml's lint job and in
// the pinnedActions map. A change to the version is a coordinated
// change across the three artefacts, all in the same commit.
const GolangciLintVersion = "v1.62.2"

// CacheBranchScope documents GitHub Actions' cache branch-scoping
// behaviour.
//
// The behaviour is GitHub's, not the workflow's. The workflow cannot
// change it; the documentation exists so that a reader understands
// why a PR's first push is slower than its second.
//
// # The behaviour
//
//   - A cache is created under the branch that produced it.
//   - A branch can read caches created by its ancestors (the branch
//     it was forked from, and their ancestors).
//   - A branch cannot write to a cache created by a different
//     branch.
//   - A pull request's cache is created under the PR's head branch,
//     not under the base branch.
//
// # The consequence for this repository
//
//   - A push to main writes main's cache.
//   - A PR's first push sees no cache for the PR's branch; it starts
//     cold, downloads dependencies, and writes a cache under the
//     PR's branch.
//   - A PR's second push sees the PR's branch's cache.
//   - A PR does not affect main's cache.
//
// The hit-rate targets (90% for module, 60% for build) are
// measured across all runs. A PR's first push is a miss; the
// steady-state rate is what the targets describe.
const CacheBranchScope = "GitHub Actions scopes caches by branch: a branch reads its ancestors' caches but writes only its own. " +
	"A PR's first push starts cold; subsequent pushes to the same PR hit the PR's branch's cache. " +
	"A PR never writes to main's cache."

// ValidateCache checks the cache model's internal invariants. It
// returns a structured *forgeerr.Error on the first violation.
//
// The invariants are:
//
//  1. Every cache type in the closed set has a spec entry.
//  2. Every spec names a member of the closed set of cache types.
//  3. Every spec has at least one path.
//  4. Every spec has a non-empty key template and restore-key template.
//  5. The restore-key template is a prefix of the key template (after
//     the substitution syntax is removed).
//  6. Every spec's hit-rate target is in [0, 1].
//  7. The module cache's target is higher than the build cache's
//     (the module cache is more stable).
//  8. Every spec has a non-empty note.
//  9. The cache version is non-empty.
//
// 10. The golangci-lint version is non-empty and starts with "v".
//
// The function is total and never panics.
func ValidateCache() error {
	seen := make(map[CacheType]struct{}, len(cacheSpecs))
	for _, s := range cacheSpecs {
		if !s.Type.Valid() {
			return cacheError(
				"unknown cache type",
				fmt.Sprintf("the cache type %q is not in the closed set", string(s.Type)),
			)
		}
		if _, dup := seen[s.Type]; dup {
			return cacheError(
				"duplicate cache type",
				fmt.Sprintf("the cache type %q is declared more than once", string(s.Type)),
			)
		}
		seen[s.Type] = struct{}{}
		if err := validateCacheSpec(s); err != nil {
			return err
		}
	}
	for _, t := range AllCacheTypes() {
		if _, ok := seen[t]; !ok {
			return cacheError(
				"missing cache type",
				fmt.Sprintf("the cache type %q has no spec entry", string(t)),
			)
		}
	}

	// The module cache's target must exceed the build cache's.
	module, _ := LookupCacheSpec(CacheModule)
	build, _ := LookupCacheSpec(CacheBuild)
	if module.HitRateTarget <= build.HitRateTarget {
		return cacheError(
			"module target not higher than build target",
			fmt.Sprintf(
				"the module cache's target %v is not greater than the build cache's %v; "+
					"the module cache is more stable and should have a higher target",
				module.HitRateTarget, build.HitRateTarget,
			),
		)
	}

	if CacheVersion == "" {
		return cacheError("empty cache version", "the CACHE_VERSION constant is empty")
	}
	if GolangciLintVersion == "" {
		return cacheError("empty golangci-lint version", "the GOLANGCI_LINT_VERSION constant is empty")
	}
	if !strings.HasPrefix(GolangciLintVersion, "v") {
		return cacheError(
			"golangci-lint version is not semver-prefixed",
			fmt.Sprintf("the version %q does not start with 'v'", GolangciLintVersion),
		)
	}

	return nil
}

// validateCacheSpec checks the invariants of a single CacheSpec.
func validateCacheSpec(s CacheSpec) error {
	if !s.Type.Valid() {
		return cacheError(
			"unknown cache type",
			fmt.Sprintf("the cache type %q is not in the closed set", string(s.Type)),
		)
	}
	if len(s.Paths) == 0 {
		return cacheError(
			"no paths",
			fmt.Sprintf("the cache type %q has no paths", string(s.Type)),
		)
	}
	if s.KeyTemplate == "" {
		return cacheError(
			"empty key template",
			fmt.Sprintf("the cache type %q has no key template", string(s.Type)),
		)
	}
	if s.RestoreKeyTemplate == "" {
		return cacheError(
			"empty restore-key template",
			fmt.Sprintf("the cache type %q has no restore-key template", string(s.Type)),
		)
	}
	// The restore-key template must be a prefix of the key template
	// after the substitution syntax is normalized. The simplest
	// check: the restore-key template must be a prefix of the key
	// template (the templates are written so that this holds
	// literally).
	if !strings.HasPrefix(s.KeyTemplate, s.RestoreKeyTemplate) {
		return cacheError(
			"restore key is not a prefix of the key",
			fmt.Sprintf(
				"the cache type %q has key %q and restore-key %q; "+
					"the restore-key must be a prefix of the key",
				string(s.Type), s.KeyTemplate, s.RestoreKeyTemplate,
			),
		)
	}
	if s.HitRateTarget < 0 || s.HitRateTarget > 1 {
		return cacheError(
			"hit-rate target out of range",
			fmt.Sprintf(
				"the cache type %q has target %v, which is not in [0, 1]",
				string(s.Type), s.HitRateTarget,
			),
		)
	}
	if s.Note == "" {
		return cacheError(
			"empty note",
			fmt.Sprintf("the cache type %q has no note", string(s.Type)),
		)
	}
	return nil
}

// CacheGolden renders the cache model as deterministic JSON.
func CacheGolden() string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(`  "cache_version": `)
	writeJSONString(&b, CacheVersion)
	b.WriteString(",\n")
	b.WriteString(`  "golangci_lint_version": `)
	writeJSONString(&b, GolangciLintVersion)
	b.WriteString(",\n")
	b.WriteString(`  "branch_scope": `)
	writeJSONString(&b, CacheBranchScope)
	b.WriteString(",\n")
	b.WriteString(`  "specs": [`)
	for i, s := range cacheSpecs {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n    {\n")
		b.WriteString(`      "type": `)
		writeJSONString(&b, string(s.Type))
		b.WriteString(",\n")
		b.WriteString(`      "paths": [`)
		for j, p := range s.Paths {
			if j > 0 {
				b.WriteString(", ")
			}
			writeJSONString(&b, p)
		}
		b.WriteString("],\n")
		b.WriteString(`      "key_template": `)
		writeJSONString(&b, s.KeyTemplate)
		b.WriteString(",\n")
		b.WriteString(`      "restore_key_template": `)
		writeJSONString(&b, s.RestoreKeyTemplate)
		b.WriteString(",\n")
		b.WriteString(`      "hit_rate_target": `)
		b.WriteString(fmt.Sprintf("%.2f", s.HitRateTarget))
		b.WriteString(",\n")
		b.WriteString(`      "note": `)
		writeJSONString(&b, s.Note)
		b.WriteString("\n    }")
	}
	b.WriteString("\n  ]\n}\n")
	return b.String()
}

// CacheStrategyMarkdown renders the cache strategy document as
// Markdown. It is the content of
// docs/reference/ci-cache-strategy.md.
func CacheStrategyMarkdown() string {
	var b strings.Builder

	b.WriteString("# CI Cache Strategy\n\n")
	b.WriteString("> **Status:** Accepted\n")
	b.WriteString("> **Owner:** Release Engineering\n")
	b.WriteString("> **Related:** [ADR-049](../decisions/ADR-049-ci-cache-strategy.md), [CI Tiers](ci-tiers.md), [CI Architecture](../architecture/ci-architecture.md)\n\n")
	b.WriteString("This document describes the caching strategy the CI tier\n")
	b.WriteString("workflows use. It is rendered from `internal/ci/cache.go`; a\n")
	b.WriteString("golden-file test asserts the two agree.\n\n")

	// ── Cache types ──────────────────────────────────────────────────────
	b.WriteString("## Cache Types\n\n")
	b.WriteString("The CI workflows use three caches. Each is a directory the Go\n")
	b.WriteString("toolchain (or the linter) writes to; `actions/cache` restores the\n")
	b.WriteString("directory before the tier's steps run and saves it after the run\n")
	b.WriteString("completes.\n\n")

	b.WriteString("| Cache | Paths | Key | Restore key | Hit-rate target |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, s := range cacheSpecs {
		b.WriteString("| ")
		b.WriteString(string(s.Type))
		b.WriteString(" | ")
		b.WriteString(strings.Join(s.Paths, "<br>"))
		b.WriteString(" | `")
		b.WriteString(s.KeyTemplate)
		b.WriteString("` | `")
		b.WriteString(s.RestoreKeyTemplate)
		b.WriteString("` | ")
		b.WriteString(fmt.Sprintf("%.0f%%", s.HitRateTarget*100))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	b.WriteString("Each cache's note:\n\n")
	for _, s := range cacheSpecs {
		b.WriteString("- **")
		b.WriteString(string(s.Type))
		b.WriteString(":** ")
		b.WriteString(s.Note)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// ── Invalidation ─────────────────────────────────────────────────────
	b.WriteString("## Invalidation\n\n")
	b.WriteString("Every cache key includes a `CACHE_VERSION` prefix. To force-clear\n")
	b.WriteString("every cache, bump the prefix in `.github/workflows/cache.yml`:\n\n")
	b.WriteString("```yaml\nenv:\n  CACHE_VERSION: ")
	b.WriteString(CacheVersion)
	b.WriteString("  # bump to v2 to invalidate\n```\n\n")
	b.WriteString("The next CI run sees a cache miss on every key and rebuilds from\n")
	b.WriteString("scratch. The change is one line and appears in the diff; a reviewer\n")
	b.WriteString("sees the reason in the commit message.\n\n")

	b.WriteString("The current version is **")
	b.WriteString(CacheVersion)
	b.WriteString("**.\n\n")

	// ── Branch scope ─────────────────────────────────────────────────────
	b.WriteString("## Branch Scope\n\n")
	b.WriteString(CacheBranchScope)
	b.WriteString("\n\n")
	b.WriteString("The consequence: a PR's first push is a miss; subsequent pushes\n")
	b.WriteString("to the same PR hit the PR's branch's cache. The hit-rate targets\n")
	b.WriteString("below describe the steady-state rate across all runs, not the\n")
	b.WriteString("rate for a single PR.\n\n")

	// ── Safety ───────────────────────────────────────────────────────────
	b.WriteString("## Cache Safety\n\n")
	b.WriteString("The caches store **derived artifacts only**:\n\n")
	b.WriteString("- the module cache stores extracted module sources;\n")
	b.WriteString("- the build cache stores compiled object files;\n")
	b.WriteString("- the linter cache stores the golangci-lint binary.\n\n")
	b.WriteString("None of the three contains a secret. The workflow's `env:` block\n")
	b.WriteString("declares no secret. The cache keys do not incorporate any secret.\n\n")
	b.WriteString("The repository's `go.mod` and `go.sum` are **not** cached: they are\n")
	b.WriteString("source files, not derived artifacts. A cache restore must never\n")
	b.WriteString("overwrite them. The workflow caches the directories the Go\n")
	b.WriteString("toolchain derives from them, not the files themselves.\n\n")

	b.WriteString("The `CACHE_VERSION` prefix is the mechanism that recovers from an\n")
	b.WriteString("accidentally poisoned cache. A cache whose contents are suspect\n")
	b.WriteString("is not deleted; the version is bumped and the next run rebuilds.\n\n")

	// ── Targets ──────────────────────────────────────────────────────────
	b.WriteString("## Hit-Rate Targets\n\n")
	b.WriteString("The repository targets the following steady-state hit rates.\n")
	b.WriteString("They are the targets the strategy document publishes; a future\n")
	b.WriteString("metric collector (a follow-up task) reads the workflow runs'\n")
	b.WriteString("`cache-hit` outputs and reports the actual rate.\n\n")

	b.WriteString("| Cache | Target |\n")
	b.WriteString("|---|---|\n")
	for _, s := range cacheSpecs {
		b.WriteString("| ")
		b.WriteString(string(s.Type))
		b.WriteString(" | ")
		b.WriteString(fmt.Sprintf("%.0f%%", s.HitRateTarget*100))
		b.WriteString(" |\n")
	}
	b.WriteString("\n")

	b.WriteString("The module cache's target is higher than the build cache's because\n")
	b.WriteString("the module cache changes only when `go.sum` changes, while the\n")
	b.WriteString("build cache changes whenever any Go source file changes.\n\n")

	return b.String()
}

// --- helpers ---------------------------------------------------------------

// cloneCacheSpec returns a deep copy of s.
func cloneCacheSpec(s CacheSpec) CacheSpec {
	out := s
	out.Paths = cloneSlice(s.Paths)
	return out
}

// cacheError builds a *forgeerr.Error for a cache-model invariant
// violation.
func cacheError(what, detail string) error {
	e := forgeerr.New(
		forgeerr.CategoryInternal,
		forgeerr.CodeInternalUnexpected,
		fmt.Sprintf("ci: cache model invariant violated: %s", what),
	)
	e = e.WithContext("detail", detail)
	return e
}

// init validates the cache model at package start.
func init() {
	if err := ValidateCache(); err != nil {
		panic(fmt.Sprintf("ci: cache model validation failed: %v", err))
	}
}

// Compile-time assertions that the package's public surface does not
// drift.
var (
	_ func() []CacheType                = AllCacheTypes
	_ func() []CacheSpec                = AllCacheSpecs
	_ func(CacheType) (CacheSpec, bool) = LookupCacheSpec
	_ func() error                      = ValidateCache
	_ func() string                     = CacheGolden
	_ func() string                     = CacheStrategyMarkdown
)
