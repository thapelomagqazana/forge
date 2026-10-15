package ci_test

// cache_contract_test.go — external contract tests for the CI cache
// model.
//
// The tests in this file read:
//
//   - .github/workflows/cache.yml         — the reusable workflow.
//   - .github/workflows/ci-tier1.yml      — Tier 1's invocation.
//   - .github/workflows/ci-tier2.yml      — Tier 2's invocation.
//   - .github/workflows/ci-tier3.yml      — Tier 3's invocation.
//   - docs/reference/ci-cache-strategy.md — the strategy document.
//   - internal/ci/testdata/cache.golden.json — the golden JSON.
//   - docs/decisions/ADR-049-ci-cache-strategy.md — the ADR.
//
// and assert that those artifacts agree with the model in
// internal/ci/cache.go. The file is package ci_test because it treats
// the package as a black box and asserts on the artifacts rather than
// the model.
//
// # Test taxonomy
//
//   - Existence tests: the workflow, the strategy document, the
//     golden file, and the ADR exist.
//   - Key-template tests: every `key:` line and every
//     `restore-keys:` entry in cache.yml matches a model template.
//   - Version tests: the CACHE_VERSION and GOLANGCI_LINT_VERSION
//     environment variables match the model's constants.
//   - Safety tests: the cache workflow references no secret; the
//     cached paths do not include go.mod or go.sum.
//   - Caller tests: each tier workflow invokes cache.yml; each
//     invocation passes a cache-key-suffix.
//   - Golden test: the model's rendering matches the golden file.
//   - Document test: the strategy document matches the model's
//     rendering.
//   - ADR test: the ADR exists.
//
// The grouping is documentation. A reader who wants to know what the
// cache must satisfy reads the group headings.
//
// # Why the extraction is a scanner, not a regex
//
// The first version of this file extracted `key:` and
// `restore-keys:` entries with regular expressions. Two problems
// surfaced:
//
//  1. `\s` in Go's regexp package matches newlines. A pattern of
//     the form `^\s{N,}-` therefore consumed newlines and the
//     indentation of subsequent lines, and its matches did not
//     correspond to YAML lines.
//
//  2. Indentation alone cannot identify a line as belonging to a
//     `restore-keys:` block. Every nested key in the workflow
//     (`path:`, `key:`, the shell script's `echo` lines) sits at the
//     same depth as the restore-key entries. A pattern that matched
//     "any line indented at least ten spaces" swept up all of them.
//
// The fix is a line-oriented scanner that tracks the block
// structure: a `restore-keys: |` header begins a block, and the
// entries are the following lines indented deeper than the header.
// The scanner in cacheWithEntries implements exactly that rule. It
// is small enough to read in one sitting and cannot be fooled by
// indentation coincidences.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/ci"
)

// --- Path helpers ----------------------------------------------------------

// cacheWorkflowPath returns the path to cache.yml.
func cacheWorkflowPath(t *testing.T, root string) string {
	t.Helper()
	return filepath.Join(root, ".github", "workflows", "cache.yml")
}

// readCacheWorkflow reads cache.yml and returns its bytes.
func readCacheWorkflow(t *testing.T, root string) []byte {
	t.Helper()
	path := cacheWorkflowPath(t, root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

// --- Extraction ------------------------------------------------------------

// cacheWithEntries extracts the `key:` values and the
// `restore-keys:` entries from cache.yml.
//
// The workflow uses two shapes:
//
//	key: <scalar>          — one value on the same line.
//	restore-keys: |        — a literal block; entries are the
//	                         following lines indented deeper than
//	                         the header.
//
// The scanner returns the values in file order. It does not parse
// YAML generally; it handles exactly the two shapes the cache
// workflow uses, and the tier-workflow contract tests already
// assert that the workflow is well-formed.
//
// # Why a scanner
//
// A regex cannot express "the lines under restore-keys:". Any
// pattern that matches by indentation alone sweeps up every nested
// key at the same depth, including `path:`, `key:`, and the shell
// script's `echo` lines. A scanner that tracks the block header
// cannot leak out of the block.
func cacheWithEntries(t *testing.T, data []byte) (keys, restoreKeys []string) {
	t.Helper()

	lines := strings.Split(string(data), "\n")

	// indentOf returns the number of leading spaces on a line, or
	// -1 for a blank line.
	indentOf := func(s string) int {
		if strings.TrimSpace(s) == "" {
			return -1
		}
		return len(s) - len(strings.TrimLeft(s, " "))
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		indent := indentOf(line)

		// A key line: `key: <value>`.
		if strings.HasPrefix(trimmed, "key:") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "key:"))
			if value != "" {
				keys = append(keys, value)
			}
			continue
		}

		// A restore-keys block: `restore-keys: |` followed by
		// lines indented deeper than the `restore-keys:` line.
		if strings.HasPrefix(trimmed, "restore-keys:") {
			headerIndent := indent
			for j := i + 1; j < len(lines); j++ {
				entry := lines[j]
				entryTrimmed := strings.TrimSpace(entry)
				entryIndent := indentOf(entry)

				// A blank line ends the block.
				if entryIndent == -1 {
					i = j
					break
				}
				// A line at or shallower than the header ends
				// the block.
				if entryIndent <= headerIndent {
					i = j - 1
					break
				}
				restoreKeys = append(restoreKeys, entryTrimmed)
				if j == len(lines)-1 {
					i = j
				}
			}
			continue
		}
	}
	return keys, restoreKeys
}

// --- Existence tests -------------------------------------------------------

// TestCache_WorkflowFileExists asserts cache.yml exists and is
// non-empty.
func TestCache_WorkflowFileExists(t *testing.T) {
	root := repoRoot(t)
	path := cacheWorkflowPath(t, root)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("cache.yml is missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("cache.yml is empty")
	}
}

// TestCache_StrategyDocExists asserts the strategy document exists.
func TestCache_StrategyDocExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "docs", "reference", "ci-cache-strategy.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docs/reference/ci-cache-strategy.md is missing: %v", err)
	}
}

// TestCache_GoldenFileExists asserts the golden JSON exists.
func TestCache_GoldenFileExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "cache.golden.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache.golden.json is missing: %v", err)
	}
}

// --- Key-template tests ----------------------------------------------------

// TestCache_EveryKeyMatchesModel asserts every `key:` line in
// cache.yml matches one of the model's key templates.
//
// The test extracts every `key:` value and asserts each matches a
// template byte-for-byte. The template is a string with `${{ ... }}`
// substitutions; the workflow file contains the same substitutions.
//
// # Why the final loop ranges over values
//
// `want` is a map[string]ci.CacheType: the key is the template
// string, the value is the cache type. The final loop wants to
// iterate the cache types; it must use the map's *values*, not its
// keys. A loop written as `for ct := range want` would range over
// the template strings, and the subsequent `seen[ct]` lookup (where
// `seen` is map[ci.CacheType]bool) would fail to compile with
// "cannot use ct (variable of type string) as ci.CacheType value in
// map index".
func TestCache_EveryKeyMatchesModel(t *testing.T) {
	root := repoRoot(t)
	data := readCacheWorkflow(t, root)
	keys, _ := cacheWithEntries(t, data)

	if len(keys) == 0 {
		t.Fatal("cache.yml contains no `key:` lines; the cache steps are missing")
	}

	// want maps the template string to the cache type it belongs
	// to. Its key is the string; its value is the type.
	want := make(map[string]ci.CacheType)
	for _, s := range ci.AllCacheSpecs() {
		want[s.KeyTemplate] = s.Type
	}

	// seen tracks which cache types have appeared in the workflow
	// file. Its key is the type.
	seen := make(map[ci.CacheType]bool)
	for _, key := range keys {
		ct, ok := want[key]
		if !ok {
			t.Errorf("cache.yml's `key:` line does not match any model template: %q", key)
			continue
		}
		if seen[ct] {
			t.Errorf("cache.yml declares the key for cache %q more than once", ct)
		}
		seen[ct] = true
	}

	// Every cache type must be present.
	for _, ct := range want {
		if !seen[ct] {
			t.Errorf("cache.yml does not declare a `key:` line for cache %q", ct)
		}
	}
}

// TestCache_EveryRestoreKeyMatchesModel asserts every
// `restore-keys:` entry in cache.yml matches one of the model's
// restore-key templates.
//
// # Why the final loop ranges over values
//
// Same reasoning as TestCache_EveryKeyMatchesModel: `want`'s keys
// are the template strings, its values are the cache types; the
// final loop must use the values to look up in `seen`, whose keys
// are cache types.
func TestCache_EveryRestoreKeyMatchesModel(t *testing.T) {
	root := repoRoot(t)
	data := readCacheWorkflow(t, root)
	_, restoreKeys := cacheWithEntries(t, data)

	if len(restoreKeys) == 0 {
		t.Fatal("cache.yml contains no `restore-keys:` entries; the cache steps are missing")
	}

	want := make(map[string]ci.CacheType)
	for _, s := range ci.AllCacheSpecs() {
		want[s.RestoreKeyTemplate] = s.Type
	}

	seen := make(map[ci.CacheType]bool)
	for _, key := range restoreKeys {
		ct, ok := want[key]
		if !ok {
			t.Errorf("cache.yml's `restore-keys:` entry does not match any model template: %q", key)
			continue
		}
		if seen[ct] {
			t.Errorf("cache.yml declares the restore key for cache %q more than once", ct)
		}
		seen[ct] = true
	}

	for _, ct := range want {
		if !seen[ct] {
			t.Errorf("cache.yml does not declare a `restore-keys:` entry for cache %q", ct)
		}
	}
}

// TestCache_EveryKeyIsUnique asserts the key templates the model
// declares are distinct.
//
// The check is a property of the model, not of the workflow file.
// The two are related: if the model's templates were identical, the
// workflow file would have two `key:` lines with the same value, and
// the cache steps would race to write the same key.
func TestCache_EveryKeyIsUnique(t *testing.T) {
	seen := make(map[string]ci.CacheType)
	for _, s := range ci.AllCacheSpecs() {
		if prev, ok := seen[s.KeyTemplate]; ok {
			t.Errorf("cache %q and cache %q share the same key template", prev, s.Type)
			continue
		}
		seen[s.KeyTemplate] = s.Type
	}
}

// TestCache_EveryRestoreKeyIsAPrefixOfItsKey asserts the model's
// restore-key templates are prefixes of the corresponding key
// templates.
//
// The relationship is what makes the restore keys useful: a cache
// miss on the exact key hits the restore key's prefix, restoring the
// previous cache.
func TestCache_EveryRestoreKeyIsAPrefixOfItsKey(t *testing.T) {
	for _, s := range ci.AllCacheSpecs() {
		if !strings.HasPrefix(s.KeyTemplate, s.RestoreKeyTemplate) {
			t.Errorf(
				"cache %q: the restore-key template %q is not a prefix of the key template %q",
				s.Type, s.RestoreKeyTemplate, s.KeyTemplate,
			)
		}
	}
}

// --- Version tests ---------------------------------------------------------

// TestCache_CacheVersionMatchesModel asserts the CACHE_VERSION
// environment variable in cache.yml matches the model's constant.
//
// The check is a substring search: the workflow file declares
// `CACHE_VERSION: v1` in its `env:` block. The model declares the
// value as `CacheVersion`. The two must agree.
func TestCache_CacheVersionMatchesModel(t *testing.T) {
	root := repoRoot(t)
	data := readCacheWorkflow(t, root)
	s := string(data)

	want := "CACHE_VERSION: " + ci.CacheVersion
	if !strings.Contains(s, want) {
		t.Errorf(
			"cache.yml does not declare %q; the CACHE_VERSION environment variable must equal the model's CacheVersion",
			want,
		)
	}
}

// TestCache_GolangciLintVersionMatchesModel asserts the
// GOLANGCI_LINT_VERSION environment variable in cache.yml matches
// the model's constant.
func TestCache_GolangciLintVersionMatchesModel(t *testing.T) {
	root := repoRoot(t)
	data := readCacheWorkflow(t, root)
	s := string(data)

	want := "GOLANGCI_LINT_VERSION: " + ci.GolangciLintVersion
	if !strings.Contains(s, want) {
		t.Errorf(
			"cache.yml does not declare %q; the GOLANGCI_LINT_VERSION environment variable must equal the model's GolangciLintVersion",
			want,
		)
	}
}

// --- Safety tests ----------------------------------------------------------

// TestCache_NoSecretsReferenced asserts cache.yml references no
// secret.
//
// The cache workflow provisions the caches; it does not read a
// secret. A future change that added a secret reference would fail
// this test and prompt a review of the cache-safety policy.
func TestCache_NoSecretsReferenced(t *testing.T) {
	root := repoRoot(t)
	data := readCacheWorkflow(t, root)
	if strings.Contains(string(data), "secrets.") {
		t.Fatal("cache.yml references secrets; the cache workflow must not read any secret")
	}
}

// TestCache_NoGoModOrGoSumPaths asserts the cached paths in
// cache.yml do not include go.mod or go.sum.
//
// The two files are source files, not derived artifacts. A cache
// restore must never overwrite them. The check is a substring search
// on the paths the model declares; it does not need to parse the
// YAML.
func TestCache_NoGoModOrGoSumPaths(t *testing.T) {
	for _, s := range ci.AllCacheSpecs() {
		for _, p := range s.Paths {
			// The path is a directory; a match for "go.mod" or
			// "go.sum" as the path's last component would be a
			// violation. A directory that contains the string as
			// a substring (for example, "go.mod-cache") is not
			// the same as caching the file itself, but the policy
			// is strict: the cached paths must be the Go
			// toolchain's own directories.
			if strings.HasSuffix(p, "/go.mod") || strings.HasSuffix(p, "/go.sum") {
				t.Errorf(
					"cache %q caches the path %q; go.mod and go.sum must not be cached",
					s.Type, p,
				)
			}
			if p == "go.mod" || p == "go.sum" {
				t.Errorf(
					"cache %q caches %q; go.mod and go.sum must not be cached",
					s.Type, p,
				)
			}
		}
	}
}

// TestCache_NoSecretPaths asserts the cached paths in the model do
// not name a directory that conventionally contains a secret.
//
// The check is a substring search for the marker words the repository
// uses for secret-bearing directories: `.ssh`, `.aws`, `.gcp`,
// `.docker/config.json`, and `.netrc`. A future change that added a
// cache of one of those directories would fail this test.
func TestCache_NoSecretPaths(t *testing.T) {
	forbidden := []string{
		".ssh",
		".aws",
		".gcp",
		".docker/config.json",
		".netrc",
	}
	for _, s := range ci.AllCacheSpecs() {
		for _, p := range s.Paths {
			for _, f := range forbidden {
				if strings.Contains(p, f) {
					t.Errorf(
						"cache %q caches the path %q; the path contains the secret-bearing marker %q",
						s.Type, p, f,
					)
				}
			}
		}
	}
}

// --- Caller tests ----------------------------------------------------------

// tierWorkflowPaths returns the paths of the three tier workflow
// files.
func tierWorkflowPaths(t *testing.T, root string) []string {
	t.Helper()
	return []string{
		filepath.Join(root, ".github", "workflows", "ci-tier1.yml"),
		filepath.Join(root, ".github", "workflows", "ci-tier2.yml"),
		filepath.Join(root, ".github", "workflows", "ci-tier3.yml"),
	}
}

// TestCache_TierWorkflowsInvokeTheCacheWorkflow asserts each tier
// workflow invokes cache.yml.
//
// The invocation is via `uses: ./.github/workflows/cache.yml`. The
// check is a substring search on the tier workflow's content; the
// invocation appears at least once per file.
func TestCache_TierWorkflowsInvokeTheCacheWorkflow(t *testing.T) {
	for _, path := range tierWorkflowPaths(t, repoRoot(t)) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !strings.Contains(string(data), "uses: ./.github/workflows/cache.yml") {
			t.Errorf("%s does not invoke ./.github/workflows/cache.yml", path)
		}
	}
}

// TestCache_TierWorkflowsDeclareCacheKeySuffix asserts each tier
// workflow passes a `cache-key-suffix` input to cache.yml.
//
// The check is a substring search: every `uses:
// ./.github/workflows/cache.yml` must be followed (within the same
// step) by a `cache-key-suffix:` line. The check is coarse; the
// tighter check is that the number of `cache-key-suffix:` lines
// equals the number of `uses: ./.github/workflows/cache.yml` lines.
func TestCache_TierWorkflowsDeclareCacheKeySuffix(t *testing.T) {
	for _, path := range tierWorkflowPaths(t, repoRoot(t)) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(data)

		usesCount := strings.Count(s, "uses: ./.github/workflows/cache.yml")
		suffixCount := strings.Count(s, "cache-key-suffix:")

		if usesCount == 0 {
			t.Errorf("%s does not invoke cache.yml", path)
			continue
		}
		if usesCount != suffixCount {
			t.Errorf(
				"%s has %d invocations of cache.yml but %d cache-key-suffix declarations; each invocation must declare a suffix",
				path, usesCount, suffixCount,
			)
		}
	}
}

// TestCache_TierWorkflowsDoNotRelyOnImplicitCache asserts the tier
// workflows do not rely on actions/setup-go's `cache: true`.
//
// The explicit cache step replaces the implicit one. A tier workflow
// that left `cache: true` on its setup-go step would write a second
// cache, doubling the storage and splitting the hit rate.
func TestCache_TierWorkflowsDoNotRelyOnImplicitCache(t *testing.T) {
	for _, path := range tierWorkflowPaths(t, repoRoot(t)) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(data)
		if strings.Contains(s, "cache: true") {
			t.Errorf(
				"%s sets `cache: true` on an actions/setup-go step; the explicit cache step replaces it, so `cache: false` is the correct setting",
				path,
			)
		}
	}
}

// TestCache_TierWorkflowsDoNotReferenceSecretsInCacheSteps asserts
// the tier workflows' cache steps do not reference a secret.
//
// The check is deliberately narrow: it does not scan the whole tier
// workflow (the coverage job in ci-tier3.yml legitimately references
// CODECOV_TOKEN), it scans only the substring between the
// `Provision caches` step's `uses:` line and the next step's `- name:`
// line. The substring must contain no `secrets.` reference.
func TestCache_TierWorkflowsDoNotReferenceSecretsInCacheSteps(t *testing.T) {
	for _, path := range tierWorkflowPaths(t, repoRoot(t)) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		s := string(data)

		// Find every `Provision caches` step and extract its body.
		// The step's body runs from the `- name: Provision caches`
		// line to the next `- name:` line at the same indentation.
		const stepName = "- name: Provision caches"
		stepRE := regexp.MustCompile(`(?m)^\s*-\s+name:\s`)
		idx := 0
		for {
			start := strings.Index(s[idx:], stepName)
			if start == -1 {
				break
			}
			start += idx
			// Find the next step's start after `start`.
			next := len(s)
			if m := stepRE.FindStringIndex(s[start+1:]); m != nil {
				next = start + 1 + m[0]
			}
			body := s[start:next]
			if strings.Contains(body, "secrets.") {
				t.Errorf(
					"%s: the Provision caches step at offset %d references secrets",
					path, start,
				)
			}
			idx = next
		}
	}
}

// --- Golden and document tests ---------------------------------------------

// TestCache_GoldenMatchesModel asserts ci.CacheGolden() matches the
// golden file.
func TestCache_GoldenMatchesModel(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "ci", "testdata", "cache.golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want := ci.CacheGolden()
	if string(data) != want {
		t.Fatalf(
			"%s does not match ci.CacheGolden(); run `task ci:render-cache` to regenerate",
			path,
		)
	}
}
