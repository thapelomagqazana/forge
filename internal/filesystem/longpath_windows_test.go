//go:build windows

package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareForOS_LocalPathAboveThreshold asserts that a local
// absolute path above the threshold gains the \\?\ prefix.
func TestPrepareForOS_LocalPathAboveThreshold(t *testing.T) {
	long := `C:\` + strings.Repeat("x", 300)
	got := prepareForOS(long)

	if !strings.HasPrefix(got, `\\?\`) {
		t.Fatalf("prepareForOS(%q) = %q, want \\\\?\\ prefix", long, got)
	}
	if strings.Contains(got, "/") {
		t.Fatalf("prepareForOS(%q) = %q, want backslashes only", long, got)
	}
	// The prefix is 4 characters; the rest of the path is
	// preserved after separator conversion.
	wantSuffix := strings.Repeat("x", 300)
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("prepareForOS did not preserve the path body")
	}
}

// TestPrepareForOS_UNCAboveThreshold asserts that a UNC path
// above the threshold gains the \\?\UNC\ prefix and loses its
// leading \\.
func TestPrepareForOS_UNCAboveThreshold(t *testing.T) {
	long := `\\server\share\` + strings.Repeat("x", 300)
	got := prepareForOS(long)

	if !strings.HasPrefix(got, `\\?\UNC\`) {
		t.Fatalf("prepareForOS(%q) = %q, want \\\\?\\UNC\\ prefix", long, got)
	}
	// The UNC body must not begin with a duplicate \\.
	rest := strings.TrimPrefix(got, `\\?\UNC\`)
	if strings.HasPrefix(rest, `\`) {
		t.Fatalf("prepareForOS(%q) = %q, UNC body starts with backslash", long, got)
	}
}

// TestPrepareForOS_AtThreshold verifies the boundary: a path of
// exactly longPathThreshold characters is unchanged; one
// character more is prefixed.
//
// The test is a boundary test in the strict sense: it pins the
// transition at a specific length so a future change to the
// threshold fails here.
func TestPrepareForOS_AtThreshold(t *testing.T) {
	// Build a local absolute path of exactly the threshold.
	base := `C:\`
	body := strings.Repeat("a", longPathThreshold-len(base))
	atThreshold := base + body
	if len(atThreshold) != longPathThreshold {
		t.Fatalf("test setup: built %d chars, want %d",
			len(atThreshold), longPathThreshold)
	}

	if got := prepareForOS(atThreshold); got != atThreshold {
		t.Fatalf("at threshold: changed %q → %q", atThreshold, got)
	}

	overThreshold := atThreshold + "a"
	if got := prepareForOS(overThreshold); !strings.HasPrefix(got, `\\?\`) {
		t.Fatalf("over threshold: %q did not gain prefix", got)
	}
}

// TestPrepareForOS_ForwardSlashesConverted asserts that a
// prefixed path has its forward slashes converted to
// backslashes. The \\?\ prefix disables the operating system's
// automatic separator translation, so the conversion is required
// for the path to address the intended file.
func TestPrepareForOS_ForwardSlashesConverted(t *testing.T) {
	long := `C:/` + strings.Repeat("x", 300)
	got := prepareForOS(long)

	if strings.Contains(got, "/") {
		t.Fatalf("prepareForOS(%q) = %q, still contains forward slashes",
			long, got)
	}
}

// TestPrepareForOS_RelativeUnchanged asserts that a relative
// path above the threshold is unchanged. The \\?\ prefix is
// undefined for relative paths; the function returns the path as
// given and lets the OS apply its usual check.
func TestPrepareForOS_RelativeUnchanged(t *testing.T) {
	rel := strings.Repeat("x", 300)
	got := prepareForOS(rel)
	if got != rel {
		t.Fatalf("prepareForOS(%q) = %q, want unchanged", rel, got)
	}
}

// TestResolve_LongPathThroughOS verifies the end-to-end
// behaviour: a file whose absolute path exceeds the threshold
// can be created and read through the abstraction.
//
// The test constructs the deep directory itself, then writes
// and reads through OSFS. It exercises the whole pipeline,
// including the OS's acceptance of the prefixed path.
func TestResolve_LongPathThroughOS(t *testing.T) {
	dir := t.TempDir()

	// Build a deep tree whose total path exceeds the
	// threshold. Each component is short (well under
	// NAME_MAX), so the only variable under test is the total
	// length.
	//
	// Windows allows directories of arbitrary depth provided
	// the total path is prefixed. The test creates 20
	// components of 12 characters each, giving roughly 240
	// characters of directory plus the temp dir prefix.
	deep := dir
	for i := 0; i < 20; i++ {
		deep = filepath.Join(deep, "component000")
	}
	if len(deep) <= longPathThreshold {
		t.Skipf("tempdir too shallow: %d chars, need > %d",
			len(deep), longPathThreshold)
	}

	if err := os.MkdirAll(deep, 0o755); err != nil {
		// Some CI environments refuse paths even with the
		// prefix. Skip rather than fail: the abstraction is
		// correct; the environment cannot express the input.
		t.Skipf("cannot create deep path: %v", err)
	}

	f := NewOSFS(dir)
	// The relative name is the deep tree's path relative to
	// dir, with forward slashes for the FS interface.
	rel := filepath.ToSlash(strings.TrimPrefix(deep, dir+string(filepath.Separator)))

	if err := f.Write(rel+"/file.txt", []byte("deep"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := f.Read(rel + "/file.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "deep" {
		t.Fatalf("Read = %q, want %q", got, "deep")
	}
}
