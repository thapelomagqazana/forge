package distribution

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// wingetAdapter renders the winget manifests for the release.
//
// # The manifest's shape
//
// winget's manifest format is YAML, split across three files:
// a version manifest, an installer manifest, and a locale manifest.
// The three are submitted together as a pull request to
// microsoft/winget-pkgs; the repository's CI validates them, and a
// maintainer merges the PR.
//
// # Why a PR, not a commit
//
// microsoft/winget-pkgs is not Forge's repository. Every change to it
// is a pull request. The pipeline opens the PR; a human merges it.
// This is by design: winget's maintainers validate every manifest
// before it is published to the winget client.
//
// # Why three files
//
// winget's schema requires the split. A single file would not
// validate. The three files are the version manifest (the metadata),
// the installer manifest (the per-architecture URLs and hashes), and
// the locale manifest (the description in en-US).
type wingetAdapter struct{}

// Channel returns ChannelWinget.
func (wingetAdapter) Channel() Channel { return ChannelWinget }

// Manifests returns the three manifests.
//
// The manifests' paths are under
// manifests/f/Forge/Forge/<version>/. The path's shape is winget's:
// the first letter of the publisher, the publisher, the package, the
// version.
//
// The three filenames are winget's convention:
//
//   - <PackageIdentifier>.yaml                  — the version manifest
//   - <PackageIdentifier>.installer.yaml        — the installer manifest
//   - <PackageIdentifier>.locale.<locale>.yaml  — the locale manifest
//
// For PackageIdentifier "Forge.Forge" and locale "en-US", the three
// names are "Forge.Forge.yaml", "Forge.Forge.installer.yaml", and
// "Forge.Forge.locale.en-US.yaml".
//
// The function returns ErrNotApplicable when the release's channel is
// not "stable": winget accepts stable releases only. A prerelease
// would be rejected by winget's maintainers; the adapter records the
// reason.
func (wingetAdapter) Manifests(r Release) ([]Manifest, error) {
	if r.Channel != "stable" {
		return nil, fmt.Errorf("%w: winget accepts stable releases only, got %q", ErrNotApplicable, r.Channel)
	}
	required := []string{"windows/amd64", "windows/arm64"}
	urls, err := platformURLs(r, required)
	if err != nil {
		return nil, err
	}

	version := strings.TrimPrefix(r.Version, "v")
	dir := fmt.Sprintf("manifests/f/Forge/Forge/%s", version)

	data := wingetData{
		Version:      version,
		Publisher:    "Forge",
		PackageName:  "Forge",
		PublisherURL: "https://forge.dev",
		Description:  "Forge is a foundation manager for software projects.",
		License:      "MPL-2.0",
		LicenseURL:   "https://github.com/forge-cli/forge/blob/main/LICENSE",
		Copyright:    "Copyright (c) Forge contributors",
		AMD64URL:     urls["windows/amd64"].URL,
		AMD64SHA:     urls["windows/amd64"].SHA256,
		ARM64URL:     urls["windows/arm64"].URL,
		ARM64SHA:     urls["windows/arm64"].SHA256,
	}

	var out []Manifest
	for _, t := range []struct {
		name string
		tmpl *template.Template
	}{
		{"version", wingetVersionTmpl},
		{"installer", wingetInstallerTmpl},
		{"locale.en-US", wingetLocaleTmpl},
	} {
		var buf bytes.Buffer
		if err := t.tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("winget: render %s: %w", t.name, err)
		}
		out = append(out, Manifest{
			Path:    fmt.Sprintf("%s/Forge.Forge%s", dir, wingetKindSuffix(t.name)),
			Content: buf.Bytes(),
			Mode:    0o644,
		})
	}
	return out, nil
}

// wingetKindSuffix returns the suffix that distinguishes a manifest
// kind's filename from the version manifest's. The version manifest
// has no distinguishing suffix; the other two have one between
// "Forge.Forge" and ".yaml".
//
// The function's name says "suffix", not "name", because the return
// value is appended to "Forge.Forge" and includes the ".yaml"
// extension. A future reader who wonders why the version case's
// return value starts with a dot finds the answer in the caller's
// format string.
//
// The function's return values are winget's schema's vocabulary; a
// change to them is a change to every published manifest's filename.
func wingetKindSuffix(name string) string {
	switch name {
	case "version":
		return ".yaml"
	case "installer":
		return ".installer.yaml"
	case "locale.en-US":
		return ".locale.en-US.yaml"
	}
	// A future manifest kind that is not one of the three above
	// gets a default suffix built from its name. The default is a
	// guess; the switch's three cases are the ones the adapter
	// produces today.
	return "." + name + ".yaml"
}

type wingetData struct {
	Version      string
	Publisher    string
	PackageName  string
	PublisherURL string
	Description  string
	License      string
	LicenseURL   string
	Copyright    string
	AMD64URL     string
	AMD64SHA     string
	ARM64URL     string
	ARM64SHA     string
}

// wingetVersionTmpl is the version manifest's template.
//
// The manifest's schema is documented at
// https://github.com/microsoft/winget-pkgs/tree/master/doc. The
// fields the template emits are the schema's required fields; a
// future field (a release-notes URL, a tags list) would add a line to
// the template and to wingetData.
var wingetVersionTmpl = template.Must(template.New("winget-version").Parse(wingetVersionManifest))

const wingetVersionManifest = `PackageIdentifier: Forge.Forge
PackageVersion: {{ .Version }}
PackageLocale: en-US
Publisher: {{ .Publisher }}
PackageName: {{ .PackageName }}
License: {{ .License }}
ShortDescription: {{ .Description }}
Moniker: forge
Tags:
  - cli
  - foundation
  - project
ManifestType: version
ManifestVersion: 1.6.0
`

// wingetInstallerTmpl is the installer manifest's template.
var wingetInstallerTmpl = template.Must(template.New("winget-installer").Parse(wingetInstallerManifest))

const wingetInstallerManifest = `PackageIdentifier: Forge.Forge
PackageVersion: {{ .Version }}
InstallerLocale: en-US
Platform:
  - Windows.Desktop
MinimumOSVersion: 10.0.0.0
InstallerType: zip
InstallModes:
  - interactive
  - silent
  - silentWithProgress
UpgradeBehavior: install
Installers:
  - Architecture: x64
    InstallerUrl: {{ .AMD64URL }}
    InstallerSha256: {{ .AMD64SHA }}
  - Architecture: arm64
    InstallerUrl: {{ .ARM64URL }}
    InstallerSha256: {{ .ARM64SHA }}
NestedInstallerFiles:
  - RelativeFilePath: forge.exe
    PortableCommandAlias: forge
ManifestType: installer
ManifestVersion: 1.6.0
`

// wingetLocaleTmpl is the locale manifest's template.
var wingetLocaleTmpl = template.Must(template.New("winget-locale").Parse(wingetLocaleManifest))

const wingetLocaleManifest = `PackageIdentifier: Forge.Forge
PackageVersion: {{ .Version }}
PackageLocale: en-US
Publisher: {{ .Publisher }}
PublisherUrl: {{ .PublisherURL }}
PackageName: {{ .PackageName }}
License: {{ .License }}
LicenseUrl: {{ .LicenseURL }}
Copyright: {{ .Copyright }}
ShortDescription: {{ .Description }}
ManifestType: defaultLocale
ManifestVersion: 1.6.0
`
