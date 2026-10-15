// Package release contains the supply-chain integrity pipeline's
// shared contract: the artifact-naming convention, the dist/ layout,
// and the file-mode constants the drivers share.
//
// It is a library, not a command. The three cmd/ drivers
// (forge-internal-checksums, forge-internal-sign, forge-internal-sbom)
// import it; none of them re-implements the naming convention.
//
// # Why a shared package
//
// The archive name (forge-<goos>-<goarch>.<ext>) appears in three
// places: the builder (cmd/forge-internal-build), the verifier
// (cmd/forge-internal-verify), and the three supply-chain drivers.
// Before this package, the convention was duplicated: the builder's
// archiveName and the verifier's archiveNameFor were two functions
// with the same body. A change to the convention would have required
// edits in both. The shared package removes the duplication.
//
// # What the package is not
//
// It is not a place for driver-specific logic. The cosign
// invocation, the notarytool invocation, and the CycloneDX encoder
// live in their respective drivers. The shared package holds only
// the values that more than one driver needs.
package release

import (
	"fmt"
	"path/filepath"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// DefaultDistDir is the artifact directory the drivers read and write
// by default. A driver accepts --dist=<dir> to override it.
const DefaultDistDir = "dist"

// BinaryName is the name of the Forge binary inside the archive.
// Windows adds .exe; the archive name is unaffected.
const BinaryName = "forge"

// ArchiveExt returns the archive extension for a platform. Linux and
// macOS use .tar.gz; Windows uses .zip.
//
// The extension is part of the frozen naming convention in
// docs/guides/reproducible-builds.md §"Artifact naming". Changing it
// is a MAJOR change.
func ArchiveExt(p platform.Platform) string {
	if p.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// ArchiveName returns the archive filename for a platform.
//
// The convention is forge-<goos>-<goarch>.<ext>. The version does not
// appear in the name: it is embedded in the binary and readable with
// `forge version`. The version-independent name lets a package
// manager pin a URL whose only change between releases is the
// archive's SHA-256.
//
// The convention is frozen by docs/guides/reproducible-builds.md and
// by docs/reference/release-channels.md.
func ArchiveName(p platform.Platform) string {
	return fmt.Sprintf("%s-%s-%s.%s", BinaryName, p.GOOS, p.GOARCH, ArchiveExt(p))
}

// ArchivePath returns the absolute or relative path to a platform's
// archive inside distDir. The caller supplies distDir; the function
// does not resolve it against the cwd.
func ArchivePath(distDir string, p platform.Platform) string {
	return filepath.Join(distDir, ArchiveName(p))
}

// PerBinaryChecksumPath returns the path to a platform's per-binary
// checksum file. The file contains a single line in the standard
// `sha256sum` format:
//
//	<hex digest>  <archive name>
//
// The per-binary file exists so a user who downloads one archive can
// verify it without downloading SHA256SUMS or learning which line
// in it applies. The combined SHA256SUMS file remains the canonical
// list; the per-binary file is a convenience.
func PerBinaryChecksumPath(distDir string, p platform.Platform) string {
	return ArchivePath(distDir, p) + ".sha256"
}

// SBOMIndexPath returns the path to the aggregated SBOM index.
//
// The index names every per-binary SBOM and is the file a tool like
// `cyclonedx-cli merge` reads. The per-binary files are the primary
// artifacts; the index is a convenience for a tool that wants a
// single document.
func SBOMIndexPath(distDir string) string {
	return filepath.Join(distDir, "forge.sbom.json")
}

// PerBinarySBOMName returns the filename of a platform's per-binary
// CycloneDX SBOM.
//
// The convention is <archive name>.cdx.json. The extension .cdx.json
// is the CycloneDX community's recommended suffix; a tool that
// discovers SBOMs by globbing *.cdx.json finds every per-binary file.
func PerBinarySBOMName(p platform.Platform) string {
	return ArchiveName(p) + ".cdx.json"
}

// PerBinarySBOMPath returns the path to a platform's per-binary SBOM
// inside distDir.
func PerBinarySBOMPath(distDir string, p platform.Platform) string {
	return filepath.Join(distDir, PerBinarySBOMName(p))
}

// CombinedChecksumName is the filename of the combined checksum list.
//
// The format is the standard `sha256sum` format:
//
//	<hex digest>  <archive name>
//
// A user verifies with `sha256sum -c SHA256SUMS` in the directory
// that holds the archives.
const CombinedChecksumName = "SHA256SUMS"

// CombinedChecksumPath returns the path to the combined checksum
// list inside distDir.
func CombinedChecksumPath(distDir string) string {
	return filepath.Join(distDir, CombinedChecksumName)
}

// GPGSignatureName is the filename of the armored GPG signature over
// SHA256SUMS.
//
// The .asc extension is the conventional extension for an armored
// signature. The signature is produced with `gpg --armor --detach-sign`;
// a user verifies with `gpg --verify SHA256SUMS.asc SHA256SUMS`.
const GPGSignatureName = "SHA256SUMS.asc"

// GPGSignaturePath returns the path to the armored GPG signature
// inside distDir.
func GPGSignaturePath(distDir string) string {
	return filepath.Join(distDir, GPGSignatureName)
}

// GPGBinarySignatureName is the filename of the raw (binary) GPG
// signature over SHA256SUMS.
//
// The .gpg extension is the conventional extension for a binary
// signature. The signature is produced with `gpg --detach-sign`;
// a user verifies with `gpg --verify SHA256SUMS.gpg SHA256SUMS`.
//
// Both forms are published: some tools consume the armored form
// (which is ASCII-safe), others consume the binary form (which is
// smaller). Publishing both costs two files and avoids a downstream
// tool's having to convert.
const GPGBinarySignatureName = "SHA256SUMS.gpg"

// GPGBinarySignaturePath returns the path to the binary GPG
// signature inside distDir.
func GPGBinarySignaturePath(distDir string) string {
	return filepath.Join(distDir, GPGBinarySignatureName)
}

// CosignSignatureExt and CosignCertificateExt are the extensions
// cosign uses for its output files.
//
// cosign sign-blob writes a signature and a certificate for each
// signed artifact. The signature is an ECDSA signature over the
// artifact's bytes; the certificate proves the signer's identity
// through Sigstore's Fulcio CA and is recorded in the Rekor
// transparency log.
//
// The extensions match the ones cosign's own documentation and the
// `cosign verify-blob` examples use. A user who follows the cosign
// documentation expects the extensions to match.
const (
	CosignSignatureExt   = ".sig"
	CosignCertificateExt = ".pem"
)

// CosignSignaturePath returns the path to a platform's cosign
// signature inside distDir.
func CosignSignaturePath(distDir string, p platform.Platform) string {
	return ArchivePath(distDir, p) + CosignSignatureExt
}

// CosignCertificatePath returns the path to a platform's cosign
// certificate inside distDir.
func CosignCertificatePath(distDir string, p platform.Platform) string {
	return ArchivePath(distDir, p) + CosignCertificateExt
}

// SLSAProvenanceName is the filename of the SLSA provenance
// attestation.
//
// The .intoto.jsonl extension is the one the slsa-github-generator
// produces. The file is a JSON Lines document: one JSON object per
// line, each an in-toto attestation. The first line is the
// provenance predicate; a tool like `slsa-verifier` reads the whole
// file.
const SLSAProvenanceName = "forge.intoto.jsonl"

// SLSAProvenancePath returns the path to the SLSA provenance inside
// distDir.
func SLSAProvenancePath(distDir string) string {
	return filepath.Join(distDir, SLSAProvenanceName)
}

// FileMode is the mode the drivers use for every file they write.
//
// The value is 0o644: a readable file, not executable. Artifacts in
// dist/ are not scripts; they are data. The archive files
// themselves are written with mode 0o644 by the builder; the
// supply-chain files match.
const FileMode = 0o644
