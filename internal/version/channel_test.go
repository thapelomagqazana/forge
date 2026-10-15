package version_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/version"
)

// --- Positive cases -------------------------------------------------------

// TestChannelOf_Table is the canonical table of tag -> channel mappings.
// It is the executable form of docs/reference/release-channels.md.
func TestChannelOf_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tag  string
		want version.Channel
	}{
		// Stable: vX.Y.Z.
		{"v0.1.0", version.ChannelStable},
		{"v1.0.0", version.ChannelStable},
		{"v1.4.2", version.ChannelStable},
		{"v10.20.30", version.ChannelStable},

		// Prerelease: vX.Y.Z-(alpha|beta|rc).N.
		{"v1.0.0-alpha.1", version.ChannelPrerelease},
		{"v1.0.0-alpha.10", version.ChannelPrerelease},
		{"v1.0.0-beta.3", version.ChannelPrerelease},
		{"v1.0.0-rc.2", version.ChannelPrerelease},
		{"v2.0.0-rc.1", version.ChannelPrerelease},

		// Nightly: vX.Y.Z-nightly.YYYYMMDD.
		{"v1.5.0-nightly.20260601", version.ChannelNightly},
		{"v1.5.0-nightly.19991231", version.ChannelNightly},
		{"v2.0.0-nightly.21000101", version.ChannelNightly},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			t.Parallel()
			got, err := version.ChannelOf(tc.tag)
			if err != nil {
				t.Fatalf("ChannelOf(%q): %v", tc.tag, err)
			}
			if got != tc.want {
				t.Errorf("ChannelOf(%q) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

// TestChannelOf_Deterministic asserts repeated calls agree.
func TestChannelOf_Deterministic(t *testing.T) {
	t.Parallel()
	for _, tag := range []string{"v1.0.0", "v1.0.0-rc.1", "v1.0.0-nightly.20260601"} {
		a, _ := version.ChannelOf(tag)
		b, _ := version.ChannelOf(tag)
		if a != b {
			t.Errorf("ChannelOf(%q) not deterministic: %q vs %q", tag, a, b)
		}
	}
}

// --- Negative cases -------------------------------------------------------

// TestChannelOf_Rejects asserts a tag matching no channel pattern is
// rejected with an error that names the tag.
func TestChannelOf_Rejects(t *testing.T) {
	t.Parallel()
	bad := []string{
		"",                          // empty
		"1.0.0",                     // missing v
		"v1.0",                      // missing patch
		"v1",                        // missing minor and patch
		"v1.0.0.0",                  // four segments
		"v1.0.0-",                   // dangling hyphen
		"v1.0.0-alpha",              // no .N
		"v1.0.0-alpha.",             // dangling dot
		"v1.0.0-ALPHA.1",            // uppercase identifier
		"v1.0.0-gamma.1",            // unknown identifier
		"v1.0.0-rc.1.2",             // extra dot
		"v1.0.0-nightly.2026060",    // 7-digit date
		"v1.0.0-nightly.202606011",  // 9-digit date
		"v1.0.0-nightly.2026-06-01", // hyphens in date
		"v1.0.0+build",              // build metadata not allowed
		"release-1.0.0",             // not a version tag
		"v1.0.0\n",                  // trailing newline
		" v1.0.0",                   // leading space
	}
	for _, tag := range bad {
		tag := tag // capture for the subtest closure (Go < 1.22)
		t.Run(strconv.Quote(tag), func(t *testing.T) {
			t.Parallel()
			_, err := version.ChannelOf(tag)
			if err == nil {
				t.Fatalf("ChannelOf(%q) = nil error, want rejection", tag)
			}
			// The error message renders the tag with %q, so the
			// substring to look for is the quoted form, not the raw
			// form. For most tags the two are identical; for tags
			// containing control characters they differ.
			if tag != "" && !strings.Contains(err.Error(), strconv.Quote(tag)) {
				t.Errorf("error %v does not name the tag %q", err, tag)
			}
		})
	}
}

// --- Edge cases -----------------------------------------------------------

// TestChannelOf_EmptyTag asserts the empty tag is rejected without panic.
func TestChannelOf_EmptyTag(t *testing.T) {
	t.Parallel()
	_, err := version.ChannelOf("")
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestChannelOf_NeverPanics asserts ChannelOf is total.
func TestChannelOf_NeverPanics(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"", "v", "v0", "v0.", "v0.0", "v0.0.", "v0.0.0",
		"\x00", "🎉", strings.Repeat("v", 1000),
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ChannelOf(%q) panicked: %v", in, r)
				}
			}()
			_, _ = version.ChannelOf(in)
		}()
	}
}

// --- Boundary cases -------------------------------------------------------

// TestChannelOf_DisjointPatterns asserts the three channel patterns do
// not overlap on a corpus of plausible tags.
func TestChannelOf_DisjointPatterns(t *testing.T) {
	t.Parallel()
	tags := []string{
		"v1.0.0", "v1.0.0-alpha.1", "v1.0.0-beta.1",
		"v1.0.0-rc.1", "v1.0.0-nightly.20260601",
	}
	seen := map[version.Channel]int{}
	for _, tag := range tags {
		c, err := version.ChannelOf(tag)
		if err != nil {
			t.Fatalf("ChannelOf(%q): %v", tag, err)
		}
		seen[c]++
	}
	if seen[version.ChannelStable] != 1 || seen[version.ChannelPrerelease] != 3 || seen[version.ChannelNightly] != 1 {
		t.Errorf("disjointness violated: %v", seen)
	}
}

// --- PreReleaseOf --------------------------------------------------------

// TestPreReleaseOf_Table asserts the identifier extraction.
func TestPreReleaseOf_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tag  string
		want version.PreRelease
	}{
		{"v1.0.0", version.PreReleaseNone},
		{"v1.0.0-alpha.1", version.PreReleaseAlpha},
		{"v1.0.0-beta.1", version.PreReleaseBeta},
		{"v1.0.0-rc.1", version.PreReleaseRC},
		{"v1.0.0-nightly.20260601", version.PreReleaseNightly},
	}
	for _, tc := range cases {
		t.Run(tc.tag, func(t *testing.T) {
			t.Parallel()
			got, err := version.PreReleaseOf(tc.tag)
			if err != nil {
				t.Fatalf("PreReleaseOf(%q): %v", tc.tag, err)
			}
			if got != tc.want {
				t.Errorf("PreReleaseOf(%q) = %q, want %q", tc.tag, got, tc.want)
			}
		})
	}
}

// TestPreReleaseOf_RejectsBadTag asserts a bad tag is rejected.
func TestPreReleaseOf_RejectsBadTag(t *testing.T) {
	t.Parallel()
	_, err := version.PreReleaseOf("not-a-tag")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Non-functional ------------------------------------------------------

// TestChannelOf_AllocsBound asserts ChannelOf does not allocate on the
// happy path. The regexes are package-level, so a call that matches
// reuses them and does not allocate a new match buffer.
//
// This test is NOT parallel. testing.AllocsPerRun is documented as
// incompatible with t.Parallel(): the allocator counters are
// process-global, so a parallel test would observe other tests'
// allocations and the number would be meaningless. The panic
//
//	testing: AllocsPerRun called during parallel test
//
// is the testing package enforcing that rule. Removing t.Parallel()
// is the only correct fix; the test runs in microseconds and
// parallelizing it buys nothing.
func TestChannelOf_AllocsBound(t *testing.T) {
	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = version.ChannelOf("v1.0.0")
	})
	if allocs > 0 {
		t.Errorf("ChannelOf allocated %v times, want 0", allocs)
	}
}

// TestAllChannels_ReturnsCopy asserts the caller may modify the slice.
func TestAllChannels_ReturnsCopy(t *testing.T) {
	t.Parallel()
	a := version.AllChannels()
	a[0] = "mutated"
	b := version.AllChannels()
	if b[0] != version.ChannelStable {
		t.Fatal("AllChannels returned a shared slice")
	}
}

// TestChannel_Valid asserts Valid's truth table.
func TestChannel_Valid(t *testing.T) {
	t.Parallel()
	valid := []version.Channel{version.ChannelStable, version.ChannelPrerelease, version.ChannelNightly}
	for _, c := range valid {
		if !c.Valid() {
			t.Errorf("%q.Valid() = false, want true", c)
		}
	}
	for _, c := range []version.Channel{"", "nightly ", "Stable", "unknown"} {
		if c.Valid() {
			t.Errorf("%q.Valid() = true, want false", c)
		}
	}
}

// TestParseChannel asserts case-insensitive parsing.
func TestParseChannel(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"stable", "STABLE", " Stable "} {
		c, err := version.ParseChannel(s)
		if err != nil {
			t.Errorf("ParseChannel(%q): %v", s, err)
			continue
		}
		if c != version.ChannelStable {
			t.Errorf("ParseChannel(%q) = %q, want stable", s, c)
		}
	}
	if _, err := version.ParseChannel("nope"); err == nil {
		t.Fatal("expected error")
	}
}

// --- Errors are not sentinels --------------------------------------------

// TestChannelErrorsAreNotSentinels asserts the errors cannot be
// accidentally matched by callers.
func TestChannelErrorsAreNotSentinels(t *testing.T) {
	t.Parallel()
	_, err := version.ChannelOf("nope")
	if errors.Is(err, errors.New("")) {
		t.Fatal("error matched an unrelated sentinel")
	}
}
