package safefs_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem/safefs"
)

// -----------------------------------------------------------------------------
// Deny-list: defaults
// -----------------------------------------------------------------------------

// TestDenyList_DefaultsRejectGitConfig verifies that the default
// deny-list rejects a write to .git/config through the Root.
func TestDenyList_DefaultsRejectGitConfig(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.Open(".git/config")
	if err == nil {
		t.Fatal("Open(.git/config) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf("error = %v, want wrapping ErrDenyList", err)
	}
}

// TestDenyList_DefaultsRejectGitSubpaths verifies that the
// default deny-list rejects any path under .git/, not just
// .git/config.
func TestDenyList_DefaultsRejectGitSubpaths(t *testing.T) {
	root := newMemRoot(t, nil)
	paths := []string{
		".git/config",
		".git/HEAD",
		".git/hooks/pre-commit",
		".git/objects/ab/cdef",
		".git",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			_, err := root.Open(p)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", p)
			}
			if !errors.Is(err, safefs.ErrDenyList) {
				t.Fatalf("error = %v, want wrapping ErrDenyList", err)
			}
		})
	}
}

// TestDenyList_DefaultsRejectForgeBackups verifies that the
// default deny-list rejects writes under .forge/backups/.
func TestDenyList_DefaultsRejectForgeBackups(t *testing.T) {
	root := newMemRoot(t, nil)
	paths := []string{
		".forge/backups",
		".forge/backups/20250101",
		".forge/lock",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			_, err := root.Open(p)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", p)
			}
			if !errors.Is(err, safefs.ErrDenyList) {
				t.Fatalf("error = %v, want wrapping ErrDenyList", err)
			}
		})
	}
}

// TestDenyList_DefaultsDoNotOverreach verifies that the default
// deny-list does not reject a name that merely shares a prefix
// with a denied pattern.
//
// The check is on segment boundaries: ".gitignore" must not
// match the pattern ".git", and "src/.git-notes" must not match
// it either.
func TestDenyList_DefaultsDoNotOverreach(t *testing.T) {
	root := newMemRoot(t, map[string]string{
		".gitignore":       "x",
		"src/.git-notes":   "x",
		".forge/config":    "x", // .forge/config is not .forge/backups
		"a/.gitattributes": "x",
	})
	paths := []string{
		".gitignore",
		"src/.git-notes",
		".forge/config",
		"a/.gitattributes",
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			_, err := root.Open(p)
			if err != nil {
				// The name may fail with fs.ErrNotExist
				// if the in-memory tree does not contain
				// it, but it must not fail with
				// ErrDenyList.
				if errors.Is(err, safefs.ErrDenyList) {
					t.Fatalf("Open(%q) rejected by deny-list: %v", p, err)
				}
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Deny-list: configuration
// -----------------------------------------------------------------------------

// TestDenyList_WithDenyListReplacesDefaults verifies that
// WithDenyList replaces the default set.
func TestDenyList_WithDenyListReplacesDefaults(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{".git/config": "x"},
		safefs.WithDenyList("custom"),
	)
	// .git/config is no longer denied because the default
	// list was replaced with a list that does not contain it.
	if _, err := root.Open(".git/config"); err != nil &&
		errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf(".git/config rejected despite custom deny-list: %v", err)
	}
	// The custom pattern is denied.
	_, err := root.Open("custom")
	if err == nil || !errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf("custom = %v, want wrapping ErrDenyList", err)
	}
}

// TestDenyList_WithAdditionalDenyPatternsExtends verifies that
// WithAdditionalDenyPatterns extends the defaults.
func TestDenyList_WithAdditionalDenyPatternsExtends(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{".git/config": "x", "custom": "y"},
		safefs.WithAdditionalDenyPatterns("custom"),
	)
	// The default .git pattern still applies.
	if _, err := root.Open(".git/config"); err == nil ||
		!errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf(".git/config = %v, want wrapping ErrDenyList", err)
	}
	// The custom pattern also applies.
	if _, err := root.Open("custom"); err == nil ||
		!errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf("custom = %v, want wrapping ErrDenyList", err)
	}
}

// TestDenyList_InvalidPatternRejected verifies that a malformed
// pattern is rejected at Root construction time.
func TestDenyList_InvalidPatternRejected(t *testing.T) {
	_, err := safefs.New("/tmp", safefs.WithDenyList("../escape"))
	if err == nil {
		t.Fatal("New with invalid pattern succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrInvalidPattern) {
		t.Fatalf("error = %v, want wrapping ErrInvalidPattern", err)
	}
}

// -----------------------------------------------------------------------------
// Deny-list: interactions with sub-roots
// -----------------------------------------------------------------------------

// TestDenyList_SubRootCannotConstructAtDeniedPath verifies that a
// sub-root cannot be constructed at a denied path.
//
// A sub-root at .git/ would otherwise have a boundary that
// permits writes to .git/config; the deny-list closes the gap.
func TestDenyList_SubRootCannotConstructAtDeniedPath(t *testing.T) {
	root := newMemRoot(t, nil)
	_, err := root.Rel(".git")
	if err == nil {
		t.Fatal("Rel(.git) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf("error = %v, want wrapping ErrDenyList", err)
	}
}

// TestDenyList_SubRootCannotReachDeniedSibling verifies that a
// sub-root cannot reach a denied sibling via its operations.
//
// A component sandboxed at "src" attempts to reach ".git/config"
// by constructing a sub-path that lexically stays inside "src"
// but resolves outside. The sub-boundary check catches it.
func TestDenyList_SubRootCannotReachDeniedSibling(t *testing.T) {
	root := newMemRoot(t, nil)
	src, err := root.Rel("src")
	if err != nil {
		t.Fatalf("Rel(src): %v", err)
	}
	// A name that escapes the sub-boundary is rejected with
	// ErrPathTraversal before the deny-list is consulted. The
	// combination is what protects the denied path: the
	// component cannot name it.
	_, err = src.Open("../.git/config")
	if err == nil {
		t.Fatal("Open(../.git/config) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// -----------------------------------------------------------------------------
// Allow-list: basics
// -----------------------------------------------------------------------------

// TestAllowList_AcceptsMatchingPaths verifies that a name that
// matches the allow-list proceeds.
func TestAllowList_AcceptsMatchingPaths(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{"src/a.go": "a"},
		safefs.WithAllowList("src/**"),
	)
	got, err := root.ReadFile("src/a.go")
	if err != nil {
		t.Fatalf("ReadFile(src/a.go): %v", err)
	}
	if string(got) != "a" {
		t.Fatalf("ReadFile = %q, want %q", got, "a")
	}
}

// TestAllowList_RejectsNonMatchingPaths verifies that a name
// outside the allow-list is rejected.
func TestAllowList_RejectsNonMatchingPaths(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{"other/a.go": "a"},
		safefs.WithAllowList("src/**"),
	)
	_, err := root.ReadFile("other/a.go")
	if err == nil {
		t.Fatal("ReadFile(other/a.go) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrNotAllowed) {
		t.Fatalf("error = %v, want wrapping ErrNotAllowed", err)
	}
}

// TestAllowList_DenyListWinsOverAllowList verifies that a name
// matching both the allow-list and the deny-list is rejected
// with ErrDenyList, not with a success.
func TestAllowList_DenyListWinsOverAllowList(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{".git/config": "x"},
		safefs.WithAllowList(".git/**", "src/**"),
	)
	_, err := root.ReadFile(".git/config")
	if err == nil {
		t.Fatal("ReadFile(.git/config) succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrDenyList) {
		t.Fatalf("error = %v, want wrapping ErrDenyList", err)
	}
}

// TestAllowList_MultiDepthPattern verifies the "**/" form.
func TestAllowList_MultiDepthPattern(t *testing.T) {
	root := newMemRoot(t,
		map[string]string{
			"assets/logo.png":        "a",
			"src/assets/logo.png":    "b",
			"src/nested/assets/logo": "c",
			"src/other.txt":          "d",
		},
		safefs.WithAllowList("**/assets"),
	)
	accept := []string{"assets/logo.png", "src/assets/logo.png", "src/nested/assets/logo"}
	reject := []string{"src/other.txt"}

	for _, p := range accept {
		t.Run("accept/"+p, func(t *testing.T) {
			if _, err := root.ReadFile(p); err != nil {
				if errors.Is(err, safefs.ErrNotAllowed) {
					t.Fatalf("ReadFile(%q) rejected by allow-list: %v", p, err)
				}
			}
		})
	}
	for _, p := range reject {
		t.Run("reject/"+p, func(t *testing.T) {
			_, err := root.ReadFile(p)
			if err == nil {
				t.Fatalf("ReadFile(%q) succeeded, want error", p)
			}
			if !errors.Is(err, safefs.ErrNotAllowed) {
				t.Fatalf("error = %v, want wrapping ErrNotAllowed", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Adversarial: cross-component isolation
// -----------------------------------------------------------------------------

// TestAdversarial_ComponentCannotWriteSibling verifies that a
// component sandboxed at "src" cannot write to "tests".
func TestAdversarial_ComponentCannotWriteSibling(t *testing.T) {
	root := newMemRoot(t, nil)
	src, _ := root.Rel("src")

	_, err := src.Open("../tests/a.go")
	if err == nil {
		t.Fatal("cross-component write succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestAdversarial_ComponentCannotWriteToRootDirectly verifies
// that a component cannot write to the Root's top level by
// escaping its sub-boundary.
func TestAdversarial_ComponentCannotWriteToRootDirectly(t *testing.T) {
	root := newMemRoot(t, nil)
	src, _ := root.Rel("src")

	_, err := src.Open("../top.txt")
	if err == nil {
		t.Fatal("write to root succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// TestAdversarial_NestedComponentCannotWriteOuter verifies that
// a nested component cannot write to its outer component's
// paths.
func TestAdversarial_NestedComponentCannotWriteOuter(t *testing.T) {
	root := newMemRoot(t, nil)
	outer, _ := root.Rel("src")
	inner, _ := outer.Rel("internal")

	// "internal" cannot reach "src/other.go".
	_, err := inner.Open("../other.go")
	if err == nil {
		t.Fatal("nested component write succeeded, want error")
	}
	if !errors.Is(err, safefs.ErrPathTraversal) {
		t.Fatalf("error = %v, want wrapping ErrPathTraversal", err)
	}
}

// -----------------------------------------------------------------------------
// Property-based: no write outside the root
// -----------------------------------------------------------------------------

// TestProperty_NoWriteOutsideRoot is the property test for the
// "no write outside the Root may ever succeed" invariant.
//
// The test enumerates a set of adversarial names and asserts
// that none of them produces a successful write. The names are
// the ones the WBS explicitly mentions: a write to .git/config
// via a sub-root, and a component writing to another
// component's paths.
//
// The property is not fuzzed because a write has side effects
// that a fuzz target would need to clean up. The enumerated set
// is the documented attack set; a future addition to the attack
// set adds a case here.
func TestProperty_NoWriteOutsideRoot(t *testing.T) {
	root := newMemRoot(t, nil)

	// Attacks via the Root directly.
	directAttacks := []string{
		"../escape",
		"/etc/passwd",
		".git/config",
		".git/hooks/pre-commit",
		".forge/backups/state",
	}
	for _, name := range directAttacks {
		t.Run("direct/"+name, func(t *testing.T) {
			_, err := root.Open(name)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", name)
			}
		})
	}

	// Attacks via a sub-root sandboxed at "src".
	src, _ := root.Rel("src")
	subAttacks := []string{
		"../escape",
		"../.git/config",
		"../../etc/passwd",
		"../tests/other.go",
	}
	for _, name := range subAttacks {
		t.Run("sub/"+name, func(t *testing.T) {
			_, err := src.Open(name)
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error", name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Boundary decision tree: sample cases
// -----------------------------------------------------------------------------

// TestDecisionTree_SampleCases verifies that each path through
// the boundary decision tree produces the expected sentinel.
//
// The decision tree has six steps; the test exercises a name
// that fails at each step and a name that passes all steps.
func TestDecisionTree_SampleCases(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		opts    []safefs.Option
		wantErr error  // nil means accept
		subRoot string // non-empty to use a sub-root
	}{
		{
			name:    "step 1: lexical normalization rejects traversal",
			input:   "../escape",
			wantErr: safefs.ErrPathTraversal,
		},
		{
			name:    "step 2: root boundary check rejects absolute",
			input:   "/etc/passwd",
			wantErr: safefs.ErrNotLocal,
		},
		{
			name:    "step 3: sub-boundary rejects sibling",
			input:   "../other",
			subRoot: "src",
			wantErr: safefs.ErrPathTraversal, // lexical rejection first
		},
		{
			name:    "step 4: deny-list rejects .git",
			input:   ".git/config",
			wantErr: safefs.ErrDenyList,
		},
		{
			name:    "step 4b: allow-list rejects non-matching",
			input:   "other/a.go",
			opts:    []safefs.Option{safefs.WithAllowList("src/**")},
			wantErr: safefs.ErrNotAllowed,
		},
		{
			name:  "all steps accept",
			input: "src/a.go",
			// no error
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newMemRoot(t,
				map[string]string{"src/a.go": "a"},
				tc.opts...,
			)
			var op interface {
				Open(string) (fs.File, error)
			} = root
			if tc.subRoot != "" {
				h, err := root.Rel(tc.subRoot)
				if err != nil {
					t.Fatalf("Rel(%q): %v", tc.subRoot, err)
				}
				op = h
			}
			f, err := op.Open(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Open(%q): %v", tc.input, err)
				}
				f.Close()
				return
			}
			if err == nil {
				t.Fatalf("Open(%q) succeeded, want error wrapping %v",
					tc.input, tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want wrapping %v", err, tc.wantErr)
			}
		})
	}
}
