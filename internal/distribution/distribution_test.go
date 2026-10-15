package distribution_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/thapelomagqazana/forge/internal/distribution"
)

// TestAllChannels_IsClosedSet asserts the six channels are exactly
// the six the package documents.
func TestAllChannels_IsClosedSet(t *testing.T) {
	t.Parallel()
	want := []distribution.Channel{
		distribution.ChannelDirect,
		distribution.ChannelHomebrew,
		distribution.ChannelScoop,
		distribution.ChannelWinget,
		distribution.ChannelApt,
		distribution.ChannelDocker,
		distribution.ChannelGoInstall,
	}
	got := distribution.AllChannels()
	if len(got) != len(want) {
		t.Fatalf("AllChannels() = %d channels; want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllChannels()[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

// TestIsValidChannel_RejectsUnknown asserts the predicate.
func TestIsValidChannel_RejectsUnknown(t *testing.T) {
	t.Parallel()
	for _, c := range distribution.AllChannels() {
		if !distribution.IsValidChannel(c) {
			t.Errorf("IsValidChannel(%q) = false; want true", c)
		}
	}
	for _, c := range []distribution.Channel{"", "apt-get", "Helm"} {
		if distribution.IsValidChannel(c) {
			t.Errorf("IsValidChannel(%q) = true; want false", c)
		}
	}
}

// TestFor_ReturnsAdapterPerChannel asserts every channel that has an
// adapter returns a non-nil one, and every channel that does not
// returns nil.
func TestFor_ReturnsAdapterPerChannel(t *testing.T) {
	t.Parallel()
	for _, c := range distribution.AllChannels() {
		got := distribution.For(c)
		switch c {
		case distribution.ChannelDirect, distribution.ChannelGoInstall:
			if got != nil {
				t.Errorf("For(%q) = %T; want nil", c, got)
			}
		default:
			if got == nil {
				t.Errorf("For(%q) = nil; want an adapter", c)
			}
		}
	}
}

// TestChannelConfig_AppliesTo asserts the per-channel release-channel
// filter.
func TestChannelConfig_AppliesTo(t *testing.T) {
	t.Parallel()
	c := distribution.ChannelConfig{PublishChannels: []string{"stable", "prerelease"}}
	if !c.AppliesTo("stable") {
		t.Errorf("stable should apply")
	}
	if !c.AppliesTo("prerelease") {
		t.Errorf("prerelease should apply")
	}
	if c.AppliesTo("nightly") {
		t.Errorf("nightly should not apply")
	}
}

// TestErrNotApplicable_IsSentinel asserts the error is the sentinel.
func TestErrNotApplicable_IsSentinel(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("homebrew: %w", distribution.ErrNotApplicable)
	if !errors.Is(err, distribution.ErrNotApplicable) {
		t.Errorf("wrapped ErrNotApplicable does not unwrap to the sentinel")
	}
}
