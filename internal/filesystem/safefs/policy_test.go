package safefs

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// This file exercises the policy and option surface of safefs.
//
// It is in `package safefs` (internal), not `package safefs_test`,
// because the `config` struct and the `defaultConfig` function are
// unexported. Every other test file in the package is external.
// The two files do not share helpers: the internal file has its
// own, because the external test package cannot see the helpers
// here and vice versa.
//
// The test taxonomy:
//
//   - Positive:    a valid option applies the value it names.
//   - Negative:    an invalid option or invalid combination is
//                  rejected with ErrInvalidOption.
//   - Edge:        an out-of-range policy value, a nil Option, an
//                  unknown string form.
//   - Corner:      later options override earlier ones for the
//                  same field; defaults are preserved when no
//                  option touches a field.
//   - Boundary:    zero means unbounded for the three numeric
//                  bounds; the first positive value is accepted;
//                  -1 is rejected.
//   - Non-functional: String and Validate are deterministic and
//                     do not allocate per call (no assertions on
//                     allocation, but the tests are shaped so a
//                     benchmark could be added).

// -----------------------------------------------------------------------------
// SymlinkPolicy — String
// -----------------------------------------------------------------------------

// TestSymlinkPolicy_String_Table verifies the documented string
// form of every defined policy. The strings are part of the
// diagnostics surface and are asserted elsewhere, so a rename is
// a deliberate breaking change.
func TestSymlinkPolicy_String_Table(t *testing.T) {
	cases := []struct {
		policy SymlinkPolicy
		want   string
	}{
		{NoFollow, "no-follow"},
		{FollowInRoot, "follow-in-root"},
		{FollowAnywhereForbidden, "no-follow"}, // alias
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.policy.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSymlinkPolicy_String_UnknownValue verifies the documented
// behaviour for a policy that is not one of the defined values:
// the method renders a numeric form rather than panicking. This
// matters because the numeric form is greppable in logs and does
// not crash a process that is trying to report a diagnostic.
func TestSymlinkPolicy_String_UnknownValue(t *testing.T) {
	p := SymlinkPolicy(200)
	got := p.String()
	if !strings.HasPrefix(got, "symlink-policy(") {
		t.Fatalf("String() = %q, want prefix %q", got, "symlink-policy(")
	}
	if !strings.Contains(got, "200") {
		t.Fatalf("String() = %q, want to contain %q", got, "200")
	}
}

// TestSymlinkPolicy_String_Deterministic verifies the trivial
// property that the string form of a given value does not change
// across calls. It exists to catch a future implementation that
// formats the value based on process state.
func TestSymlinkPolicy_String_Deterministic(t *testing.T) {
	p := FollowInRoot
	first := p.String()
	for i := 0; i < 32; i++ {
		if got := p.String(); got != first {
			t.Fatalf("String() changed: %q vs %q", first, got)
		}
	}
}

// -----------------------------------------------------------------------------
// SymlinkPolicy — Validate
// -----------------------------------------------------------------------------

// TestSymlinkPolicy_Validate_AcceptsDefinedValues verifies that
// every defined policy passes validation. A new policy added to
// the enum without a corresponding branch here fails this test.
func TestSymlinkPolicy_Validate_AcceptsDefinedValues(t *testing.T) {
	for _, p := range []SymlinkPolicy{NoFollow, FollowInRoot, FollowAnywhereForbidden} {
		t.Run(p.String(), func(t *testing.T) {
			if err := p.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

// TestSymlinkPolicy_Validate_RejectsUnknown verifies that an
// out-of-range policy value is rejected. The error wraps
// ErrInvalidOption so callers can match it with errors.Is.
func TestSymlinkPolicy_Validate_RejectsUnknown(t *testing.T) {
	cases := []SymlinkPolicy{2, 3, 200, 255}
	for _, p := range cases {
		t.Run(p.String(), func(t *testing.T) {
			err := p.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !errors.Is(err, ErrInvalidOption) {
				t.Fatalf("error = %v, want wrapping ErrInvalidOption", err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// defaultConfig
// -----------------------------------------------------------------------------

// TestDefaultConfig_DocumentedDefaults verifies every default
// against the values the package documents. A change to a default
// fails this test, forcing the change to be deliberate.
func TestDefaultConfig_DocumentedDefaults(t *testing.T) {
	c := defaultConfig()

	if c.policy != NoFollow {
		t.Errorf("policy = %v, want NoFollow", c.policy)
	}
	if c.maxDepth != DefaultMaxDepth {
		t.Errorf("maxDepth = %d, want %d", c.maxDepth, DefaultMaxDepth)
	}
	if c.maxEntries != DefaultMaxEntries {
		t.Errorf("maxEntries = %d, want %d", c.maxEntries, DefaultMaxEntries)
	}
	if c.maxSymlinkChain != DefaultMaxSymlinkChain {
		t.Errorf("maxSymlinkChain = %d, want %d", c.maxSymlinkChain, DefaultMaxSymlinkChain)
	}
	if c.detectCaseCollisions {
		t.Error("detectCaseCollisions = true, want false")
	}
	if c.fs != nil {
		t.Error("fs is non-nil, want nil so New constructs an OSFS")
	}
}

// TestDefaultConfig_ConstrainedDefaults verifies the shape of the
// defaults: the policy is the safest one, and every numeric bound
// is positive. This is the property the package doc calls out as
// deliberate; a future change to a permissive default fails here.
func TestDefaultConfig_ConstrainedDefaults(t *testing.T) {
	c := defaultConfig()

	if c.policy != NoFollow {
		t.Errorf("default policy is %v; the default must be the strictest option",
			c.policy)
	}
	if c.maxDepth <= 0 {
		t.Errorf("default maxDepth = %d; a positive bound is required",
			c.maxDepth)
	}
	if c.maxEntries <= 0 {
		t.Errorf("default maxEntries = %d; a positive bound is required",
			c.maxEntries)
	}
	if c.maxSymlinkChain <= 0 {
		t.Errorf("default maxSymlinkChain = %d; a positive bound is required",
			c.maxSymlinkChain)
	}
	if c.detectCaseCollisions {
		t.Error("default detectCaseCollisions is true; the default must be off")
	}
}

// TestDefaultConfig_ReturnsFreshValue verifies that two calls to
// defaultConfig do not share state. A future implementation that
// returns a package-level value by reference would fail here.
func TestDefaultConfig_ReturnsFreshValue(t *testing.T) {
	a := defaultConfig()
	b := defaultConfig()

	a.policy = FollowInRoot
	a.maxDepth = 1

	if b.policy != NoFollow {
		t.Errorf("b.policy changed to %v after a.policy was mutated", b.policy)
	}
	if b.maxDepth != DefaultMaxDepth {
		t.Errorf("b.maxDepth changed to %d after a.maxDepth was mutated",
			b.maxDepth)
	}
}

// -----------------------------------------------------------------------------
// With* functions
// -----------------------------------------------------------------------------

// TestWithFS_SetsFS verifies that WithFS installs the supplied
// filesystem. A later WithFS call overrides an earlier one; this
// is the documented last-write-wins rule for scalar settings and
// is tested in TestWith_LastWriteWins below.
func TestWithFS_SetsFS(t *testing.T) {
	m := newTestMemFS()
	c := defaultConfig()
	WithFS(m)(&c)
	if c.fs != m {
		t.Fatal("WithFS did not install the supplied FS")
	}
}

// TestWithSymlinkPolicy_SetsPolicy verifies that the policy is
// installed verbatim, including an out-of-range value. Validation
// happens in New, not in the With* function.
func TestWithSymlinkPolicy_SetsPolicy(t *testing.T) {
	for _, p := range []SymlinkPolicy{NoFollow, FollowInRoot, FollowAnywhereForbidden, 200} {
		t.Run(p.String(), func(t *testing.T) {
			c := defaultConfig()
			WithSymlinkPolicy(p)(&c)
			if c.policy != p {
				t.Fatalf("policy = %v, want %v", c.policy, p)
			}
		})
	}
}

// TestWithMaxDepth_SetsValue verifies that the depth bound is
// installed, including the zero and negative values that
// validation will later accept or reject.
func TestWithMaxDepth_SetsValue(t *testing.T) {
	for _, v := range []int{0, 1, 2, 64, 1 << 20, -1} {
		c := defaultConfig()
		WithMaxDepth(v)(&c)
		if c.maxDepth != v {
			t.Fatalf("maxDepth = %d, want %d", c.maxDepth, v)
		}
	}
}

// TestWithMaxEntries_SetsValue verifies the entries bound.
func TestWithMaxEntries_SetsValue(t *testing.T) {
	for _, v := range []int{0, 1, 2, 64, 1 << 20, -1} {
		c := defaultConfig()
		WithMaxEntries(v)(&c)
		if c.maxEntries != v {
			t.Fatalf("maxEntries = %d, want %d", c.maxEntries, v)
		}
	}
}

// TestWithMaxSymlinkChain_SetsValue verifies the chain bound.
func TestWithMaxSymlinkChain_SetsValue(t *testing.T) {
	for _, v := range []int{0, 1, 2, 64, 1 << 20, -1} {
		c := defaultConfig()
		WithMaxSymlinkChain(v)(&c)
		if c.maxSymlinkChain != v {
			t.Fatalf("maxSymlinkChain = %d, want %d", c.maxSymlinkChain, v)
		}
	}
}

// TestWithCaseCollisionDetection_SetsFlag verifies both the true
// and the false case.
func TestWithCaseCollisionDetection_SetsFlag(t *testing.T) {
	for _, v := range []bool{true, false} {
		c := defaultConfig()
		WithCaseCollisionDetection(v)(&c)
		if c.detectCaseCollisions != v {
			t.Fatalf("detectCaseCollisions = %v, want %v",
				c.detectCaseCollisions, v)
		}
	}
}

// TestWith_LastWriteWins verifies the documented rule that later
// options override earlier ones for scalar settings.
func TestWith_LastWriteWins(t *testing.T) {
	c := defaultConfig()
	WithMaxDepth(1)(&c)
	WithMaxDepth(2)(&c)
	WithMaxDepth(3)(&c)
	if c.maxDepth != 3 {
		t.Fatalf("maxDepth = %d, want 3", c.maxDepth)
	}
}

// TestWith_OrderMattersAcrossFields verifies that applying
// options in a different order for *different* fields produces
// the same config, because the fields are independent.
//
// # Why the comparison is field-by-field
//
// config gained slice fields in WBS 13.4.1 (the deny-list and
// allow-list patterns). Go's == and != operators do not work on
// structs containing slices, so a whole-struct comparison no
// longer compiles. The helper configsEqual compares every field
// explicitly, using slices.Equal for the slice fields. A future
// addition to config is a visible change to the helper, which is
// the desired friction: the helper's tests are the only place
// that needs to know the struct's shape.
func TestWith_OrderMattersAcrossFields(t *testing.T) {
	a := defaultConfig()
	WithMaxDepth(1)(&a)
	WithMaxEntries(2)(&a)

	b := defaultConfig()
	WithMaxEntries(2)(&b)
	WithMaxDepth(1)(&b)

	if !configsEqual(a, b) {
		t.Fatalf("configs differ when only the order of options for different fields differs:\n"+
			"a = %+v\nb = %+v", a, b)
	}
}

// -----------------------------------------------------------------------------
// config.validate
// -----------------------------------------------------------------------------

// TestConfigValidate_AcceptsDefaults verifies that a config
// produced by defaultConfig passes validation. This is the base
// case every Root relies on.
func TestConfigValidate_AcceptsDefaults(t *testing.T) {
	c := defaultConfig()
	if err := c.validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
}

// TestConfigValidate_AcceptsZeroBounds verifies that zero means
// unbounded for each of the three numeric bounds. This is the
// boundary case: the smallest non-negative value is accepted.
func TestConfigValidate_AcceptsZeroBounds(t *testing.T) {
	c := defaultConfig()
	c.maxDepth = 0
	c.maxEntries = 0
	c.maxSymlinkChain = 0
	if err := c.validate(); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
}

// TestConfigValidate_RejectsNegativeBounds verifies that each
// negative bound is rejected, and that the error names the field.
func TestConfigValidate_RejectsNegativeBounds(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*config)
		wantSub string
	}{
		{
			name:    "maxDepth",
			mutate:  func(c *config) { c.maxDepth = -1 },
			wantSub: "MaxDepth",
		},
		{
			name:    "maxEntries",
			mutate:  func(c *config) { c.maxEntries = -1 },
			wantSub: "MaxEntries",
		},
		{
			name:    "maxSymlinkChain",
			mutate:  func(c *config) { c.maxSymlinkChain = -1 },
			wantSub: "MaxSymlinkChain",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := defaultConfig()
			tc.mutate(&c)
			err := c.validate()
			if err == nil {
				t.Fatal("validate() = nil, want error")
			}
			if !errors.Is(err, ErrInvalidOption) {
				t.Fatalf("error = %v, want wrapping ErrInvalidOption", err)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error = %q, want to mention %q", err, tc.wantSub)
			}
		})
	}
}

// TestConfigValidate_RejectsInvalidPolicy verifies that an
// out-of-range policy is rejected by validate even when it was
// installed through the With* function without complaint.
func TestConfigValidate_RejectsInvalidPolicy(t *testing.T) {
	c := defaultConfig()
	c.policy = SymlinkPolicy(200)
	err := c.validate()
	if err == nil {
		t.Fatal("validate() = nil, want error")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("error = %v, want wrapping ErrInvalidOption", err)
	}
}

// TestConfigValidate_FirstProblemWins verifies the documented
// order of checks: policy is validated before the numeric bounds,
// so an invalid policy is reported even when a bound is also
// invalid. The order is fixed and is part of the contract for
// callers that rely on the first-error message.
func TestConfigValidate_FirstProblemWins(t *testing.T) {
	c := defaultConfig()
	c.policy = SymlinkPolicy(200)
	c.maxDepth = -1
	err := c.validate()
	if err == nil {
		t.Fatal("validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "symlink policy") {
		t.Fatalf("error = %q, want to mention %q", err, "symlink policy")
	}
}

// -----------------------------------------------------------------------------
// Integration: New wires the option pipeline
// -----------------------------------------------------------------------------

// TestNew_AppliesOptions verifies that the With* functions reach
// the assembled Root. This is the only test in the file that
// exercises an exported constructor; it exists so that the option
// pipeline is verified end-to-end, not just at the config layer.
func TestNew_AppliesOptions(t *testing.T) {
	m := newTestMemFS()
	root, err := New("/tmp/project",
		WithFS(m),
		WithSymlinkPolicy(FollowInRoot),
		WithMaxDepth(1),
		WithMaxEntries(1),
		WithCaseCollisionDetection(true),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := root.Policy(); got != FollowInRoot {
		t.Fatalf("Policy() = %v, want FollowInRoot", got)
	}
	if root.maxDepth != 1 {
		t.Fatalf("root.maxDepth = %d, want 1", root.maxDepth)
	}
	if root.maxEntries != 1 {
		t.Fatalf("root.maxEntries = %d, want 1", root.maxEntries)
	}
	if !root.collide {
		t.Fatal("root.collide = false, want true")
	}
}

// TestNew_DefaultsWhenNoOptions verifies that a Root built
// without options carries the documented defaults.
func TestNew_DefaultsWhenNoOptions(t *testing.T) {
	m := newTestMemFS()
	root, err := New("/tmp/project", WithFS(m))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := root.Policy(); got != NoFollow {
		t.Fatalf("Policy() = %v, want NoFollow", got)
	}
	if root.maxDepth != DefaultMaxDepth {
		t.Fatalf("maxDepth = %d, want %d", root.maxDepth, DefaultMaxDepth)
	}
	if root.maxEntries != DefaultMaxEntries {
		t.Fatalf("maxEntries = %d, want %d", root.maxEntries, DefaultMaxEntries)
	}
	if root.maxSymlinkChain != DefaultMaxSymlinkChain {
		t.Fatalf("maxSymlinkChain = %d, want %d",
			root.maxSymlinkChain, DefaultMaxSymlinkChain)
	}
	if root.collide {
		t.Fatal("collide = true, want false")
	}
}

// TestNew_SkipsNilOption verifies that a nil Option in the slice
// does not panic and does not affect the resulting config.
func TestNew_SkipsNilOption(t *testing.T) {
	m := newTestMemFS()
	root, err := New("/tmp/project",
		WithFS(m),
		nil,
		WithMaxDepth(1),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if root.maxDepth != 1 {
		t.Fatalf("maxDepth = %d, want 1", root.maxDepth)
	}
}

// TestNew_RejectsInvalidConfig verifies that New surfaces the
// config validation error rather than constructing a Root with an
// invalid config.
func TestNew_RejectsInvalidConfig(t *testing.T) {
	m := newTestMemFS()
	_, err := New("/tmp/project",
		WithFS(m),
		WithMaxDepth(-1),
	)
	if err == nil {
		t.Fatal("New succeeded, want error")
	}
	if !errors.Is(err, ErrInvalidOption) {
		t.Fatalf("error = %v, want wrapping ErrInvalidOption", err)
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// newTestMemFS returns a fresh in-memory filesystem. It exists so
// that the tests in this file do not depend on the adapter's
// construction details; if the adapter's constructor changes, one
// helper is updated instead of every call site.
func newTestMemFS() filesystem.FS {
	return filesystem.NewMemFS(nil)
}

// configsEqual reports whether two configs are equal field by
// field. It exists because config contains slice fields (the
// deny-list and allow-list patterns) and Go's == operator does
// not work on structs containing slices.
//
// # What "equal" means
//
// Two configs are equal when every field compares equal. For
// slices, equality is element-by-element in order. The helper
// uses slices.Equal for the slices and == for the scalars.
//
// # What the helper is not
//
// It is not a general-purpose config comparison. It is written
// for the specific test that needs it (TestWith_OrderMattersAcrossFields)
// and is not exported. A future test that needs to compare two
// configs uses the same helper; a future test that needs to
// compare two configs *semantically* (ignoring field order in a
// slice, say) writes its own comparison.
func configsEqual(a, b config) bool {
	if a.fs != b.fs {
		return false
	}
	if a.policy != b.policy {
		return false
	}
	if a.maxDepth != b.maxDepth {
		return false
	}
	if a.maxEntries != b.maxEntries {
		return false
	}
	if a.maxSymlinkChain != b.maxSymlinkChain {
		return false
	}
	if a.detectCaseCollisions != b.detectCaseCollisions {
		return false
	}
	if !slices.Equal(a.denyPatterns, b.denyPatterns) {
		return false
	}
	if !slices.Equal(a.additionalDenyPatterns, b.additionalDenyPatterns) {
		return false
	}
	if a.denyExplicit != b.denyExplicit {
		return false
	}
	if !slices.Equal(a.allowPatterns, b.allowPatterns) {
		return false
	}
	if a.allowExplicit != b.allowExplicit {
		return false
	}
	return true
}
