package distribution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// scoopAdapter renders the Scoop manifest for the release.
//
// # The manifest's shape
//
// A Scoop manifest is a JSON file with a version, a description, a
// homepage, a license, an architecture map (64bit and arm64), and an
// extraction configuration. Scoop installs the archive by
// downloading it and extracting the binary.
//
// # Why one manifest, not per-architecture manifests
//
// Scoop's manifest format has an `architecture` object with one entry
// per architecture. A single manifest covers both Windows
// architectures; a second manifest per architecture would be a second
// file to update and a second place to drift.
type scoopAdapter struct{}

// Channel returns ChannelScoop.
func (scoopAdapter) Channel() Channel { return ChannelScoop }

// Manifests returns the manifest.
//
// The manifest is one file: bucket/forge.json. The path is relative
// to the bucket's repository root.
//
// The function returns ErrNotApplicable when the release's channel is
// "nightly". A nightly is a snapshot; a bucket's manifest should not
// name a snapshot.
func (scoopAdapter) Manifests(r Release) ([]Manifest, error) {
	if r.Channel == "nightly" {
		return nil, fmt.Errorf("%w: scoop does not publish nightly releases", ErrNotApplicable)
	}
	required := []string{"windows/amd64", "windows/arm64"}
	urls, err := platformURLs(r, required)
	if err != nil {
		return nil, err
	}

	doc := scoopManifest{
		Version:     strings.TrimPrefix(r.Version, "v"),
		Description: "Forge is a foundation manager for software projects.",
		Homepage:    "https://forge.dev",
		License:     "MPL-2.0",
		Architecture: map[string]scoopArch{
			"64bit": {
				URL:  urls["windows/amd64"].URL,
				Hash: urls["windows/amd64"].SHA256,
			},
			"arm64": {
				URL:  urls["windows/arm64"].URL,
				Hash: urls["windows/arm64"].SHA256,
			},
		},
		Bin: [][]string{{"forge.exe", "forge"}},
		Checkver: scoopCheckver{
			GitHub: "https://github.com/forge-cli/forge",
		},
		Autoupdate: scoopAutoupdate{
			Architecture: map[string]scoopArch{
				"64bit": {
					URL: "https://github.com/forge-cli/forge/releases/download/v$version/forge-windows-amd64.zip",
				},
				"arm64": {
					URL: "https://github.com/forge-cli/forge/releases/download/v$version/forge-windows-arm64.zip",
				},
			},
		},
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("scoop: render manifest: %w", err)
	}
	return []Manifest{{
		Path:    "bucket/forge.json",
		Content: buf.Bytes(),
		Mode:    0o644,
	}}, nil
}

// scoopManifest is the manifest's JSON shape. The fields are ordered
// as Scoop's documentation orders them; encoding/json emits them in
// the struct's field order.
type scoopManifest struct {
	Version      string               `json:"version"`
	Description  string               `json:"description"`
	Homepage     string               `json:"homepage"`
	License      string               `json:"license"`
	Architecture map[string]scoopArch `json:"architecture"`
	Bin          [][]string           `json:"bin"`
	Checkver     scoopCheckver        `json:"checkver"`
	Autoupdate   scoopAutoupdate      `json:"autoupdate"`
}

type scoopArch struct {
	URL  string `json:"url"`
	Hash string `json:"hash"`
}

type scoopCheckver struct {
	GitHub string `json:"github"`
}

type scoopAutoupdate struct {
	Architecture map[string]scoopArch `json:"architecture"`
}
