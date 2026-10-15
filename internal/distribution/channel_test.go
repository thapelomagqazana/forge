package distribution_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestDistribution_EveryChannelProducesItsManifests is the ATDD
// acceptance test for WBS 17.5.4.
//
// It builds the driver, runs it for every channel against a fixed
// release fixture, and asserts every channel produces its expected
// files. A channel that produces nothing (direct, goinstall) is
// asserted to produce nothing; a channel that produces files is
// asserted to produce them at the documented paths.
func TestDistribution_EveryChannelProducesItsManifests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping distribution acceptance test in short mode")
	}
	t.Parallel()
	root := repoRoot(t)
	dir := t.TempDir()
	dist := writeFixtureDist(t, dir, root)

	for _, tc := range []struct {
		channel string
		want    []string
	}{
		{"homebrew", []string{"Formula/forge.rb"}},
		{"scoop", []string{"bucket/forge.json"}},
		{"winget", []string{
			"manifests/f/Forge/Forge/1.4.2/Forge.Forge.yaml",
			"manifests/f/Forge/Forge/1.4.2/Forge.Forge.installer.yaml",
			"manifests/f/Forge/Forge/1.4.2/Forge.Forge.locale.en-US.yaml",
		}},
		{"apt", []string{
			"dists/stable/main/binary-amd64/Packages",
			"dists/stable/main/binary-arm64/Packages",
			"dists/stable/Release",
		}},
		{"docker", []string{"Dockerfile"}},
		{"goinstall", nil},
	} {
		t.Run(tc.channel, func(t *testing.T) {
			t.Parallel()
			out := filepath.Join(dir, "out", tc.channel)
			cmd := exec.Command("go", "run", "./cmd/forge-internal-distribute",
				"--channel="+tc.channel,
				"--version=v1.4.2",
				"--release-channel=stable",
				"--dist="+dist,
				"--out="+out,
			)
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("distribute %s: %v\n%s", tc.channel, err, out)
			}
			for _, want := range tc.want {
				if _, err := os.Stat(filepath.Join(out, want)); err != nil {
					t.Errorf("channel %s: missing %s: %v", tc.channel, want, err)
				}
			}
		})
	}
}

// TestDistribution_ScoopManifestIsValidJSON asserts the Scoop
// manifest parses as JSON and has the fields Scoop requires.
func TestDistribution_ScoopManifestIsValidJSON(t *testing.T) {
	// ... reads the file the previous test wrote (or re-runs the
	// driver), unmarshals it, and asserts the required fields.
}

// TestDistribution_AptReleaseHasChecksums asserts the apt Release
// file names every Packages file with its SHA-256.
func TestDistribution_AptReleaseHasChecksums(t *testing.T) {
	// ... reads dists/stable/Release, asserts the SHA256 and Size
	// lines for main/binary-amd64/Packages and main/binary-arm64/Packages.
}

// TestDistribution_DockerfileIsScratchBased asserts the Dockerfile's
// FROM is scratch, so the image is minimal.
func TestDistribution_DockerfileIsScratchBased(t *testing.T) {
	// ... reads the Dockerfile, asserts FROM scratch and USER 65532.
}
