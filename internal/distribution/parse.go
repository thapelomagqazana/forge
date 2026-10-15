package distribution

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ParseRelease reads a dist/ directory and returns the Release the
// adapters consume.
//
// The function is the single place a channel reads the filesystem.
// Every adapter reads the Release struct, not the disk. The
// indirection is what makes an adapter testable without a dist/
// directory.
//
// The function reads:
//
//   - dist/SHA256SUMS, which names every archive and its digest.
//   - The archives themselves, for their sizes.
//   - go.mod, for the module path (used by the goinstall adapter).
//
// It does not read the GPG signatures: the adapters do not name
// them; the installation guide links to them separately.
func ParseRelease(distDir, version, channel, gpgFingerprint string) (Release, error) {
	checksums, err := readChecksums(filepath.Join(distDir, "SHA256SUMS"))
	if err != nil {
		return Release{}, fmt.Errorf("parse release: %w", err)
	}
	archives := map[string]Archive{}
	for name, sum := range checksums {
		platform, ok := platformOf(name)
		if !ok {
			continue
		}
		size, err := fileSize(filepath.Join(distDir, name))
		if err != nil {
			return Release{}, fmt.Errorf("parse release: %s: %w", name, err)
		}
		archives[platform] = Archive{
			Name:      name,
			GOOS:      strings.SplitN(platform, "/", 2)[0],
			GOARCH:    strings.SplitN(platform, "/", 2)[1],
			SHA256:    sum,
			SizeBytes: size,
		}
	}

	modulePath, err := readModulePath(filepath.Join(distDir, ".."))
	if err != nil {
		return Release{}, fmt.Errorf("parse release: %w", err)
	}

	return Release{
		Version:        version,
		Channel:        channel,
		PublishedAt:    "", // filled by the caller from the Release's timestamp
		Archives:       archives,
		Checksums:      checksums,
		GPGFingerprint: gpgFingerprint,
		ModulePath:     modulePath,
	}, nil
}

// readChecksums parses a SHA256SUMS file. The format is the
// `sha256sum` format: "<hex>  <name>" per line.
func readChecksums(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: malformed line %q", path, line)
		}
		out[fields[1]] = fields[0]
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

// platformOf extracts the "goos/goarch" pair from an archive's
// filename. It returns false for a name that is not an archive.
//
// The naming convention is in internal/release/release.go. The
// function is a second implementation of the same convention; a
// future refactor could call release.ArchiveName's inverse.
func platformOf(name string) (string, bool) {
	// forge-<goos>-<goarch>.tar.gz or .zip
	if !strings.HasPrefix(name, "forge-") {
		return "", false
	}
	rest := strings.TrimPrefix(name, "forge-")
	rest = strings.TrimSuffix(rest, ".tar.gz")
	rest = strings.TrimSuffix(rest, ".zip")
	parts := strings.SplitN(rest, "-", 2)
	if len(parts) != 2 {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

// fileSize returns a file's size in bytes.
func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// readModulePath reads the module path from go.mod. It returns the
// path, or an error if go.mod does not exist or has no module
// directive.
func readModulePath(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("go.mod has no module directive")
}

// sha256Hex returns the hex-encoded SHA-256 of b.
func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Unused import guard: keep the io import for a future refactor that
// reads the archives' bytes rather than their sizes.
var _ = io.Discard
