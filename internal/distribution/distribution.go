// Package distribution contains the release pipeline's distribution
// channel adapters. A channel is a downstream projection of the
// release's artifacts: the same archives the release builds, published
// to a place a user's package manager can read.
//
// # The design
//
// The release pipeline is the single source of truth. A channel never
// builds its own artifact; it reads the release's archives, its
// checksums, and its signatures, and produces a manifest or a commit
// that names them. A channel that built its own artifact would be a
// parallel build, and a parallel build drifts.
//
// Six channels, each with its own manifest format and its own
// auto-update mechanism:
//
//   - Direct: the GitHub Release. The release pipeline produces it;
//     no distribution adapter is needed. The channel is here so the
//     installation guide can name it.
//
//   - Homebrew: a formula in a tap (forge-cli/homebrew-tap). The
//     adapter renders the formula; the pipeline commits it.
//
//   - Scoop: a manifest in a bucket (forge-cli/scoop-bucket). The
//     adapter renders the JSON manifest; the pipeline commits it.
//
//   - winget: a manifest in microsoft/winget-pkgs. The adapter
//     renders the YAML manifest; the pipeline opens a PR.
//
//   - apt: a signed Debian repository at apt.forge.dev. The adapter
//     renders the Packages and Release files and signs the latter
//     with the release key; the pipeline commits them.
//
//   - Docker: an OCI image at ghcr.io/forge-cli/forge. The adapter
//     renders the Dockerfile's build args; the pipeline builds and
//     pushes the image.
//
//   - GoInstall: the module's own path, which `go install
//     github.com/forge-cli/forge/cmd/forge@latest` resolves. The
//     adapter verifies the module path and the v2+ suffix; the
//     pipeline does not update anything.
//
// # Why the adapters live in one package
//
// The adapters share three things: the release's input shape (the
// archives, their checksums, their signatures), the version's shape
// (the tag, the channel), and the output shape (a file or a commit
// the pipeline writes). Putting them in one package lets them share
// the release-input parser and the channel-config type. A separate
// package per channel would duplicate both.
//
// # Why the adapters are a library, not a binary
//
// The release workflow invokes one binary,
// cmd/forge-internal-distribute, with a --channel flag. The binary
// calls the adapter. A separate binary per channel would multiply the
// workflow's steps by six and the build's matrix by six; one binary
// with a flag is one step and one build.
package distribution

import (
	"errors"
	"fmt"
)

// Channel names the six distribution channels. It is a string type so
// a typo is caught by the compiler rather than by a test.
type Channel string

// The channels. The order is the order a user is likely to encounter
// them: direct download first, then the package managers in the order
// the installation guide lists them.
const (
	// ChannelDirect is the GitHub Release. It has no adapter: the
	// release pipeline's publish job produces it.
	ChannelDirect Channel = "direct"

	// ChannelHomebrew is the Homebrew tap at
	// forge-cli/homebrew-tap. Install: brew install forge-cli/tap/forge.
	ChannelHomebrew Channel = "homebrew"

	// ChannelScoop is the Scoop bucket at forge-cli/scoop-bucket.
	// Install: scoop install forge.
	ChannelScoop Channel = "scoop"

	// ChannelWinget is the manifest in microsoft/winget-pkgs.
	// Install: winget install Forge.Forge.
	ChannelWinget Channel = "winget"

	// ChannelApt is the signed Debian repository at apt.forge.dev.
	// Install: apt install forge.
	ChannelApt Channel = "apt"

	// ChannelDocker is the OCI image at ghcr.io/forge-cli/forge.
	// Install: docker pull ghcr.io/forge-cli/forge.
	ChannelDocker Channel = "docker"

	// ChannelGoInstall is the module's own path. Install: go install
	// github.com/forge-cli/forge/cmd/forge@latest. It has no adapter:
	// the module path is the release's, and the pipeline verifies it.
	ChannelGoInstall Channel = "goinstall"
)

// AllChannels returns every channel, in the order the installation
// guide lists them. The function returns a fresh slice so a caller
// cannot mutate the canonical list.
func AllChannels() []Channel {
	return []Channel{
		ChannelDirect,
		ChannelHomebrew,
		ChannelScoop,
		ChannelWinget,
		ChannelApt,
		ChannelDocker,
		ChannelGoInstall,
	}
}

// IsValidChannel reports whether c is one of the six channels.
func IsValidChannel(c Channel) bool {
	for _, x := range AllChannels() {
		if x == c {
			return true
		}
	}
	return false
}

// Release is the input every adapter reads. It is a snapshot of the
// release's artifacts and metadata at the moment the distribution job
// runs.
//
// The adapter does not read the filesystem directly; it reads this
// struct. The struct is built by ParseRelease, which reads the
// release's dist/ directory and the repository's go.mod. The
// indirection lets a test build a Release in memory without a dist/
// directory on disk.
type Release struct {
	// Version is the tag, e.g. "v1.4.2". It is the release's
	// identity; every channel's manifest names it.
	Version string

	// Channel is the release channel, e.g. "stable". It determines
	// whether a package manager should publish the release at all:
	// a nightly is not published to Homebrew, and a prerelease is
	// not published to winget.
	Channel string

	// PublishedAt is the release's timestamp, in RFC 3339 UTC. The
	// Homebrew formula and the Scoop manifest record it.
	PublishedAt string

	// Archives is every platform's archive, keyed by the platform's
	// Name (e.g. "linux/amd64"). Every channel reads it to find the
	// archive whose checksum it names.
	Archives map[string]Archive

	// Checksums is the SHA256SUMS file's contents, parsed into a
	// map from archive name to hex digest. It is the source of
	// truth for every manifest's checksum field.
	Checksums map[string]string

	// GPGFingerprint is the release key's fingerprint. The apt
	// adapter embeds it in the repository's Release file's
	// signature metadata; the installation guide tells the user to
	// fetch the key with it.
	GPGFingerprint string

	// ModulePath is the Go module path from go.mod, e.g.
	// "github.com/forge-cli/forge". The goinstall adapter reads it
	// to verify the module path has not changed without an ADR; the
	// installation guide documents it as the path a user's
	// `go install` resolves. The field is not read by any other
	// adapter.
	ModulePath string
}

// Archive is one platform's archive.
type Archive struct {
	// Name is the archive's filename, e.g.
	// "forge-linux-amd64.tar.gz". It is the key in Release.Checksums.
	Name string

	// GOOS is the target operating system, e.g. "linux".
	GOOS string

	// GOARCH is the target architecture, e.g. "amd64".
	GOARCH string

	// SHA256 is the archive's hex digest. It is a copy of the value
	// in Release.Checksums[Name], materialized here so an adapter
	// does not have to look it up.
	SHA256 string

	// SizeBytes is the archive's size. The winget manifest records
	// it; a future Homebrew formula could.
	SizeBytes int64
}

// Manifest is what an adapter produces. It is a file the pipeline
// writes to the channel's repository, or a commit the pipeline
// creates.
//
// The struct is deliberately narrow: a path (relative to the
// channel's repository root) and the file's content. An adapter that
// needs to produce several files returns several Manifests. An
// adapter that needs to open a PR returns a Manifest per file the PR
// changes.
type Manifest struct {
	// Path is the file's path in the channel's repository. For
	// Homebrew it is "Formula/forge.rb"; for Scoop it is
	// "bucket/forge.json"; for winget it is the manifest's path
	// under "manifests/f/Forge/Forge/<version>/"; for apt it is a
	// path under the repository's root.
	Path string

	// Content is the file's bytes.
	Content []byte

	// Mode is the file's mode. Manifests are 0o644; an apt Release
	// file is 0o644; a Dockerfile is 0o644.
	Mode uint32
}

// ErrNotApplicable is returned by an adapter when the release's
// channel excludes the channel. A nightly is not published to
// Homebrew; a prerelease is not published to winget. The error is
// sentinel so the caller can distinguish "the channel is not
// applicable to this release" from "the adapter failed".
var ErrNotApplicable = errors.New("distribution: channel not applicable to this release")

// ErrUnsupportedPlatform is returned by an adapter when the release
// is missing an archive the channel requires. A channel that needs
// linux/amd64 and does not find it returns this error; the caller
// records a blocker.
var ErrUnsupportedPlatform = errors.New("distribution: release is missing a required platform")

// Adapter is the interface every channel's adapter implements.
//
// The interface has one method. An adapter that needs several phases
// (the Docker adapter builds and pushes; the winget adapter opens a
// PR) performs them in the caller, not in the adapter. The adapter's
// job is to produce the manifests; the caller's job is to publish
// them.
type Adapter interface {
	// Channel returns the channel the adapter implements.
	Channel() Channel

	// Manifests returns the files the channel's repository must
	// contain for this release.
	//
	// The function returns ErrNotApplicable when the release's
	// channel excludes this channel. It returns
	// ErrUnsupportedPlatform when the release is missing a required
	// platform.
	Manifests(r Release) ([]Manifest, error)
}

// For returns the adapter for a channel. The function returns nil for
// ChannelDirect and ChannelGoInstall: those channels have no adapter,
// because the release pipeline's publish job and the module's own
// path are the adapters.
func For(c Channel) Adapter {
	switch c {
	case ChannelHomebrew:
		return homebrewAdapter{}
	case ChannelScoop:
		return scoopAdapter{}
	case ChannelWinget:
		return wingetAdapter{}
	case ChannelApt:
		return aptAdapter{}
	case ChannelDocker:
		return dockerAdapter{}
	case ChannelDirect, ChannelGoInstall:
		return nil
	default:
		return nil
	}
}

// ChannelConfig is the per-channel configuration the pipeline reads.
// It is the yaml's shape; docs/reference/distribution.yaml is the
// file.
type ChannelConfig struct {
	// Repository is the channel's repository, e.g.
	// "forge-cli/homebrew-tap" for Homebrew.
	Repository string `yaml:"repository"`

	// Enabled is whether the channel is published for this release.
	// A stable release enables every channel; a prerelease disables
	// the channels that do not accept prereleases.
	Enabled bool `yaml:"enabled"`

	// PublishChannels is the release channels the distribution
	// channel accepts. Homebrew accepts "stable" and "prerelease";
	// winget accepts "stable" only; apt accepts "stable" and
	// "prerelease"; Docker accepts all three.
	PublishChannels []string `yaml:"publish_channels"`
}

// AppliesTo reports whether the channel accepts the release's
// channel.
func (c ChannelConfig) AppliesTo(releaseChannel string) bool {
	for _, x := range c.PublishChannels {
		if x == releaseChannel {
			return true
		}
	}
	return false
}

// VersionError is returned by an adapter when the version is
// malformed. Every manifest names the version; a malformed version
// produces a malformed manifest.
type VersionError struct {
	Version string
	Reason  string
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("distribution: version %q: %s", e.Version, e.Reason)
}
