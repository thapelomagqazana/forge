package distribution_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/distribution"
)

// TestHomebrew_RendersFormula asserts the formula's shape: the
// version, the four platform URLs, and the install method.
func TestHomebrew_RendersFormula(t *testing.T) {
	t.Parallel()
	rel := fixtureRelease("stable")
	m, err := distribution.For(distribution.ChannelHomebrew).Manifests(rel)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("got %d manifests; want 1", len(m))
	}
	if m[0].Path != "Formula/forge.rb" {
		t.Errorf("path = %q; want Formula/forge.rb", m[0].Path)
	}
	body := string(m[0].Content)
	for _, want := range []string{
		`version "1.4.2"`,
		"forge-darwin-amd64.tar.gz",
		"forge-darwin-arm64.tar.gz",
		"forge-linux-amd64.tar.gz",
		"forge-linux-arm64.tar.gz",
		`bin.install "forge"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("formula does not contain %q", want)
		}
	}
}

// TestHomebrew_RejectsNightly asserts a nightly is not applicable.
func TestHomebrew_RejectsNightly(t *testing.T) {
	t.Parallel()
	rel := fixtureRelease("nightly")
	_, err := distribution.For(distribution.ChannelHomebrew).Manifests(rel)
	if !errors.Is(err, distribution.ErrNotApplicable) {
		t.Errorf("err = %v; want ErrNotApplicable", err)
	}
}

// TestHomebrew_MissingPlatform asserts a release missing darwin/arm64
// returns ErrUnsupportedPlatform.
func TestHomebrew_MissingPlatform(t *testing.T) {
	t.Parallel()
	rel := fixtureRelease("stable")
	delete(rel.Archives, "darwin/arm64")
	_, err := distribution.For(distribution.ChannelHomebrew).Manifests(rel)
	if !errors.Is(err, distribution.ErrUnsupportedPlatform) {
		t.Errorf("err = %v; want ErrUnsupportedPlatform", err)
	}
}

// TestHomebrew_Golden asserts the formula matches the golden. A
// change to the template changes the golden; the change is reviewed.
func TestHomebrew_Golden(t *testing.T) {
	t.Parallel()
	rel := fixtureRelease("stable")
	m, err := distribution.For(distribution.ChannelHomebrew).Manifests(rel)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "homebrew.golden.rb", m[0].Content)
}
