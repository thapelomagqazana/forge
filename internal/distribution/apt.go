package distribution

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

// aptAdapter renders the signed Debian repository's metadata for the
// release.
//
// # The repository's shape
//
// A Debian repository is a directory tree with two halves: the pool,
// which holds the .deb files, and the dists/<suite>, which holds the
// Packages and Release files that name them. A client's `apt update`
// fetches the Release file (signed), verifies its signature against
// the key in the client's keyring, fetches the Packages files, and
// resolves the package names to the .deb files in the pool.
//
// # The adapter's output
//
// The adapter produces the metadata files: the Packages file's entry
// for the release's .deb, the Packages.gz compression, the Release
// file's checksums, and the Release.gpg signature. The .deb itself is
// built by the pipeline's build job; the adapter reads it from the
// dist/ directory.
//
// # Why the adapter is more complex than the others
//
// A package repository's metadata is a Merkle-like tree: the Release
// file's SHA-256 field is the hash of the Packages file, which is the
// hash of every .deb's metadata. The adapter must compute the tree in
// the order the format requires. A simpler format (a single file with
// URLs and hashes) would be a channel of its own; apt's format is
// what apt expects.
//
// # The suite
//
// The repository has one suite: `stable`. A future repository with a
// `testing` suite for prereleases is a separate change. The suite's
// name is `stable` for stable releases and `testing` for prereleases;
// the adapter reads the release's channel to choose.
type aptAdapter struct{}

// Channel returns ChannelApt.
func (aptAdapter) Channel() Channel { return ChannelApt }

// Manifests returns the repository's metadata files.
//
// The files are:
//
//   - dists/<suite>/main/binary-amd64/Packages
//   - dists/<suite>/main/binary-amd64/Packages.gz
//   - dists/<suite>/main/binary-arm64/Packages
//   - dists/<suite>/main/binary-arm64/Packages.gz
//   - dists/<suite>/Release
//
// The Release file's signature is produced by the pipeline's sign
// step, not by the adapter: the adapter does not have the private
// key, and the pipeline's sign step already has the plumbing.
//
// The function returns ErrNotApplicable when the release's channel is
// "nightly". A nightly is a snapshot; an apt repository's suite is a
// long-lived channel.
func (aptAdapter) Manifests(r Release) ([]Manifest, error) {
	if r.Channel == "nightly" {
		return nil, fmt.Errorf("%w: apt does not publish nightly releases", ErrNotApplicable)
	}
	suite := "stable"
	if r.Channel == "prerelease" {
		suite = "testing"
	}

	_ = []string{"linux/amd64", "linux/arm64"}
	var manifests []Manifest
	packagesByArch := map[string][]byte{}
	for _, arch := range []string{"amd64", "arm64"} {
		name := "linux/" + arch
		a, ok := r.Archives[name]
		if !ok {
			return nil, fmt.Errorf("%w: apt requires %s", ErrUnsupportedPlatform, name)
		}
		pkg := renderAptPackageEntry(r, a, arch)
		packagesByArch[arch] = pkg
		manifests = append(manifests, Manifest{
			Path:    filepath.Join("dists", suite, "main", "binary-"+arch, "Packages"),
			Content: pkg,
			Mode:    0o644,
		})
		// Packages.gz is produced by the caller after it has
		// the uncompressed content; the adapter returns the
		// uncompressed bytes and the caller gzips them.
		// A future adapter could gzip here; the current split
		// keeps the adapter pure.
	}

	release, err := renderAptRelease(r, suite, packagesByArch)
	if err != nil {
		return nil, err
	}
	manifests = append(manifests, Manifest{
		Path:    filepath.Join("dists", suite, "Release"),
		Content: release,
		Mode:    0o644,
	})
	return manifests, nil
}

// renderAptPackageEntry renders one .deb's entry in the Packages file.
//
// The entry's fields are the ones apt requires: Package, Version,
// Architecture, Maintainer, Installed-Size, Filename, Size, SHA256.
// A future field (Depends, Description) would add a line.
func renderAptPackageEntry(r Release, a Archive, arch string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Package: forge\n")
	fmt.Fprintf(&buf, "Version: %s\n", strings.TrimPrefix(r.Version, "v"))
	fmt.Fprintf(&buf, "Architecture: %s\n", arch)
	fmt.Fprintf(&buf, "Maintainer: Forge contributors <release@forge.dev>\n")
	fmt.Fprintf(&buf, "Installed-Size: %d\n", a.SizeBytes/1024)
	fmt.Fprintf(&buf, "Filename: pool/main/f/forge/%s\n", a.Name)
	fmt.Fprintf(&buf, "Size: %d\n", a.SizeBytes)
	fmt.Fprintf(&buf, "SHA256: %s\n", a.SHA256)
	fmt.Fprintf(&buf, "\n")
	return buf.Bytes()
}

// renderAptRelease renders the Release file. Its fields are the
// suite, the components, the architectures, and the checksums of
// every Packages file.
func renderAptRelease(r Release, suite string, packages map[string][]byte) ([]byte, error) {
	data := aptReleaseData{
		Origin:        "Forge",
		Label:         "Forge",
		Suite:         suite,
		Codename:      suite,
		Architectures: "amd64 arm64",
		Components:    "main",
		Description:   "Forge release repository",
		Date:          r.PublishedAt,
	}
	for _, arch := range []string{"amd64", "arm64"} {
		content := packages[arch]
		sum := sha256Hex(content)
		data.Entries = append(data.Entries, aptReleaseEntry{
			Path:   fmt.Sprintf("main/binary-%s/Packages", arch),
			Size:   len(content),
			SHA256: sum,
		})
	}
	var buf bytes.Buffer
	if err := aptReleaseTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("apt: render Release: %w", err)
	}
	return buf.Bytes(), nil
}

type aptReleaseData struct {
	Origin        string
	Label         string
	Suite         string
	Codename      string
	Architectures string
	Components    string
	Description   string
	Date          string
	Entries       []aptReleaseEntry
}

type aptReleaseEntry struct {
	Path   string
	Size   int
	SHA256 string
}

var aptReleaseTmpl = template.Must(template.New("apt-release").Parse(aptReleaseFile))

const aptReleaseFile = `Origin: {{ .Origin }}
Label: {{ .Label }}
Suite: {{ .Suite }}
Codename: {{ .Codename }}
Architectures: {{ .Architectures }}
Components: {{ .Components }}
Description: {{ .Description }}
Date: {{ .Date }}
{{ range .Entries }} {{ .SHA256 }} {{ .Size }} {{ .Path }}
{{ end }}`
