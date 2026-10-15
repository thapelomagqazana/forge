// Package version provides Forge's version metadata and release-channel
// scheme.
//
// The package has three responsibilities:
//
//  1. Expose the four variables injected at link time (see version.go).
//  2. Parse a version string into its SemVer components.
//  3. Classify a git tag into exactly one release channel.
//
// # Channel is a pure function of the tag
//
// Channel selection MUST NOT depend on any input other than the tag
// string. No branch name, no environment variable, no CI context. This
// is what makes the release pipeline auditable: given a tag, the
// channel is determined by a table.
//
// The three channel patterns are disjoint regular expressions. A tag
// matching none is rejected; a tag matching more than one is
// impossible by construction.
//
// # Relationship to SemVer precedence
//
// The channel is a string ("stable", "prerelease", "nightly"), not a
// numeric rank. Precedence between two versions is SemVer's job and is
// not implemented here. Forge never compares a nightly to a
// prerelease, because nightlies are not published to package managers
// and prereleases are.
//
// # Thread safety
//
// Every exported function and value in this file is pure or
// compile-time constant. The package is safe for concurrent use.
package version

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Channel names a release channel. See docs/reference/release-channels.md.
type Channel string

const (
	// ChannelStable is the default channel. Tag pattern: vX.Y.Z.
	ChannelStable Channel = "stable"

	// ChannelPrerelease is the prerelease channel. Tag pattern:
	// vX.Y.Z-(alpha|beta|rc).N.
	ChannelPrerelease Channel = "prerelease"

	// ChannelNightly is the nightly channel. Tag pattern:
	// vX.Y.Z-nightly.YYYYMMDD.
	ChannelNightly Channel = "nightly"
)

// AllChannels returns the three defined channels in a stable order.
// The caller receives a fresh slice and may modify it.
func AllChannels() []Channel {
	return []Channel{ChannelStable, ChannelPrerelease, ChannelNightly}
}

// String returns the channel name. It is the same string used in the
// metadata document and the release notes.
func (c Channel) String() string { return string(c) }

// Valid reports whether c is one of the three defined channels.
func (c Channel) Valid() bool {
	switch c {
	case ChannelStable, ChannelPrerelease, ChannelNightly:
		return true
	default:
		return false
	}
}

// ParseChannel parses a channel name. It is case-insensitive.
func ParseChannel(s string) (Channel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(ChannelStable):
		return ChannelStable, nil
	case string(ChannelPrerelease):
		return ChannelPrerelease, nil
	case string(ChannelNightly):
		return ChannelNightly, nil
	default:
		return "", fmt.Errorf("version: unknown channel %q", s)
	}
}

// The three channel patterns. They are disjoint by construction:
//   - stable has no "-" after the patch number.
//   - prerelease requires one of alpha/beta/rc followed by ".N".
//   - nightly requires "nightly." followed by exactly 8 digits.
//
// Anchored with ^ and $ so a tag with a trailing newline or extra
// suffix does not match.
var (
	stablePattern     = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	prereleasePattern = regexp.MustCompile(`^v\d+\.\d+\.\d+-(alpha|beta|rc)\.\d+$`)
	nightlyPattern    = regexp.MustCompile(`^v\d+\.\d+\.\d+-nightly\.\d{8}$`)
)

// ChannelOf returns the channel a git tag belongs to.
//
// The mapping is documented in docs/reference/release-channels.md and
// is a pure function of the tag string:
//
//	v1.2.3                 -> stable
//	v1.2.3-alpha.1         -> prerelease
//	v1.2.3-beta.1          -> prerelease
//	v1.2.3-rc.1            -> prerelease
//	v1.2.3-nightly.20260601 -> nightly
//	anything else          -> error
//
// The error names the tag and states that no channel matches. It does
// not attempt to guess.
func ChannelOf(tag string) (Channel, error) {
	switch {
	case stablePattern.MatchString(tag):
		return ChannelStable, nil
	case prereleasePattern.MatchString(tag):
		return ChannelPrerelease, nil
	case nightlyPattern.MatchString(tag):
		return ChannelNightly, nil
	default:
		return "", fmt.Errorf("version: tag %q matches no channel pattern", tag)
	}
}

// PreRelease identifies the prerelease identifier of a tag.
type PreRelease string

const (
	PreReleaseNone    PreRelease = ""
	PreReleaseAlpha   PreRelease = "alpha"
	PreReleaseBeta    PreRelease = "beta"
	PreReleaseRC      PreRelease = "rc"
	PreReleaseNightly PreRelease = "nightly"
)

// PreReleaseOf returns the prerelease identifier of a tag, or "" for a
// stable tag. It returns an error for a tag that matches no channel.
func PreReleaseOf(tag string) (PreRelease, error) {
	c, err := ChannelOf(tag)
	if err != nil {
		return "", err
	}
	if c == ChannelStable {
		return PreReleaseNone, nil
	}
	// Extract the identifier between "-" and ".".
	i := strings.Index(tag, "-")
	if i < 0 {
		return "", errors.New("version: prerelease tag has no '-'")
	}
	rest := tag[i+1:]
	j := strings.IndexByte(rest, '.')
	if j < 0 {
		return "", fmt.Errorf("version: prerelease tag %q has no '.'", tag)
	}
	return PreRelease(rest[:j]), nil
}
