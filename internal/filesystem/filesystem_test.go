// Package filesystem_test exercises the FS contract through the
// exported interface, exactly as safefs and domain packages do.
//
// The suite runs against OSFS via t.TempDir(). It is the only test
// file in the module, apart from the safefs symlink suite, that
// touches the real disk. Everything else injects fstest.MapFS or
// filesystem.MemFS.
//
// See ADR-035 for the layering rationale.
package filesystem_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// newFS constructs an OSFS bound to a fresh temporary directory and
// returns both. The directory is cleaned up automatically by
// t.TempDir.
func newFS(t *testing.T) (filesystem.FS, string) {
	t.Helper()
	dir := t.TempDir()
	return filesystem.NewOSFS(dir), dir
}

// -----------------------------------------------------------------------------
// Interface conformance
// -----------------------------------------------------------------------------

// TestInterfaceConformance is a belt-and-braces assertion: the
// compile-time check lives in filesystem.go, but constructing the
// value here and calling every method through the FS-typed variable
// also catches accidental shadowing of an embedded method or a
// receiver that compiles but does not satisfy the intent.
//
// Every method declared on FS is called. If a future addition to
// the interface is not covered here, the test still compiles
// (the interface has the method, the value has the method), but a
// reader auditing "does the test exercise the whole surface" will
// notice. Adding a method to FS should add a call here.
func TestInterfaceConformance(t *testing.T) {
	var f filesystem.FS = filesystem.NewOSFS(t.TempDir())

	// Read and Write round-trip.
	_ = f.Write("a.txt", []byte("x"), 0o600)
	_, _ = f.Read("a.txt")

	// Open — added by WBS 13.1.1.
	if fh, err := f.Open("a.txt"); err == nil {
		_ = fh.Close()
	}
	// Open on a missing file must fail.
	if _, err := f.Open("missing"); err == nil {
		t.Fatal("Open(missing) succeeded, want error")
	}

	// Exists and MkdirAll.
	_, _ = f.Exists("a.txt")
	_ = f.MkdirAll("d", 0o755)

	// Remove.
	_ = f.Remove("a.txt")

	// List and Stat.
	_, _ = f.List(".")
	_, _ = f.Stat(".")

	// Lstat — added by WBS 13.1.1.
	if _, err := f.Lstat("."); err != nil {
		t.Fatalf("Lstat(.): %v", err)
	}

	// Readlink — added by WBS 13.1.1. On a non-link, it must
	// return an error; the exact sentinel is platform-specific.
	if _, err := f.Readlink("."); err == nil {
		t.Fatal("Readlink(.) succeeded on a non-link, want error")
	}

	// Boundary.
	if f.Boundary() == "" {
		t.Fatal("Boundary() returned empty string")
	}
}

// -----------------------------------------------------------------------------
// Boundary
// -----------------------------------------------------------------------------

// TestBoundary_Verbatim asserts that OSFS stores the boundary
// string unchanged. safefs.New relies on this: it constructs the
// OSFS around an already-absolute, already-cleaned path and expects
// Boundary to round-trip.
func TestBoundary_Verbatim(t *testing.T) {
	cases := []string{
		"/abs/path",
		"relative/path",
		"./with/trailing/.",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			f := filesystem.NewOSFS(in)
			if got := f.Boundary(); got != in {
				t.Fatalf("Boundary() = %q, want %q", got, in)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Write / Read round-trip
// -----------------------------------------------------------------------------

// TestRoundTrip is the ordered happy path: write, read, stat,
// exists, list, remove. It is a single scenario because the
// operations are not independent — the point is that they compose.
func TestRoundTrip(t *testing.T) {
	f, _ := newFS(t)

	if err := f.Write("a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read("a.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("Read = %q, want %q", got, "hello")
	}

	info, err := f.Stat("a.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name() != "a.txt" {
		t.Fatalf("Stat.Name = %q, want %q", info.Name(), "a.txt")
	}
	if info.Size() != 5 {
		t.Fatalf("Stat.Size = %d, want 5", info.Size())
	}

	ok, err := f.Exists("a.txt")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Fatal("Exists = false, want true")
	}

	entries, err := f.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a.txt" {
		t.Fatalf("List = %v, want [a.txt]", names(entries))
	}

	if err := f.Remove("a.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	ok, err = f.Exists("a.txt")
	if err != nil {
		t.Fatalf("Exists after Remove: %v", err)
	}
	if ok {
		t.Fatal("Exists after Remove = true, want false")
	}
}

// TestWrite_CreatesParentDirectories verifies the documented
// behaviour: Write creates missing parents with 0o755. safefs.Write
// inherits this.
func TestWrite_CreatesParentDirectories(t *testing.T) {
	f, dir := newFS(t)

	if err := f.Write("a/b/c.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "a", "b", "c.txt")); err != nil {
		t.Fatalf("file not created under boundary: %v", err)
	}
}

// TestWrite_EmptyContent verifies the zero-length boundary case.
func TestWrite_EmptyContent(t *testing.T) {
	f, _ := newFS(t)

	if err := f.Write("empty.txt", nil, 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := f.Read("empty.txt")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Read = %q, want empty", got)
	}
}

// TestWrite_ModeHonoured verifies the create-mode contract on Unix.
// Skipped on Windows, which does not implement POSIX permission bits.
func TestWrite_ModeHonoured(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}
	f, dir := newFS(t)

	if err := f.Write("secret.txt", []byte("x"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "secret.txt"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode = %o, want 600", perm)
	}
}

// TestWrite_OverwritePreservesMode verifies that writing over an
// existing file leaves its mode unchanged, as documented on
// FS.Write.
func TestWrite_OverwritePreservesMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}
	f, dir := newFS(t)

	if err := f.Write("f.txt", []byte("a"), 0o600); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Second write with a different mode. The existing file must
	// keep 0o600.
	if err := f.Write("f.txt", []byte("b"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "f.txt"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("mode after overwrite = %o, want 600", perm)
	}
}

// -----------------------------------------------------------------------------
// Open
// -----------------------------------------------------------------------------

// TestOpen_ReturnsWrappedFile verifies that OSFS.Open returns a
// value whose concrete type is unexported and does not expose
// the underlying *os.File. The property is what makes the
// abstraction meaningful: a caller cannot type-assert the
// returned value to reach the descriptor, cannot call methods
// the io/fs.File interface does not declare, and cannot use the
// returned value with any API that requires an *os.File.
//
// The test also verifies the positive case: the returned value
// is readable and its content matches what was written.
func TestOpen_ReturnsWrappedFile(t *testing.T) {
	f, _ := newFS(t)
	if err := f.Write("a.txt", []byte("hello"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	fh, err := f.Open("a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer fh.Close()

	// The returned value must be an io/fs.File. The assertion is
	// a compile-time check that the interface is satisfied; it
	// also documents the intent.
	var _ fs.File = fh

	// It must NOT be an *os.File. The wrapper in osfs.go delivers
	// this property, and a future contributor who removes the
	// wrapper will see the failure here.
	if _, ok := fh.(*os.File); ok {
		t.Fatal("Open returned an *os.File; the abstraction has leaked")
	}

	// It must not implement any of the optional interfaces that
	// *os.File implements beyond io/fs.File. io.ReaderAt is the
	// most likely leak — a future wrapper that embedded *os.File
	// would promote it — so it is the representative assertion.
	if _, ok := fh.(interface {
		ReadAt([]byte, int64) (int, error)
	}); ok {
		t.Fatal("Open returned a value that implements io.ReaderAt; " +
			"the abstraction has leaked")
	}

	// Positive case: the returned value is readable. This is the
	// only read performed in the test; there is no earlier read
	// that could have drained the handle.
	buf := make([]byte, 16)
	n, err := fh.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Fatalf("read %q, want %q", buf[:n], "hello")
	}
}

// TestOpen_Missing verifies the error wraps fs.ErrNotExist.
func TestOpen_Missing(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.Open("missing")
	if err == nil {
		t.Fatal("Open succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestOpen_Directory verifies that Open on a directory succeeds on
// Unix (where directories can be opened for reading) and returns a
// file whose Read fails. The exact behaviour is platform-specific;
// the test asserts only the weaker property that Open does not
// panic and the returned handle, if any, is closeable.
func TestOpen_Directory(t *testing.T) {
	f, _ := newFS(t)
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	fh, err := f.Open("d")
	if err != nil {
		// Some platforms refuse to open directories; that is a
		// valid outcome. No assertion.
		return
	}
	_ = fh.Close()
}

// -----------------------------------------------------------------------------
// Exists
// -----------------------------------------------------------------------------

// TestExists_NotExist verifies that a missing path yields
// (false, nil), not an error.
func TestExists_NotExist(t *testing.T) {
	f, _ := newFS(t)

	ok, err := f.Exists("nope")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if ok {
		t.Fatal("Exists = true, want false")
	}
}

// TestExists_Directory verifies that directories are reported as
// existing.
func TestExists_Directory(t *testing.T) {
	f, _ := newFS(t)
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	ok, err := f.Exists("d")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !ok {
		t.Fatal("Exists = false, want true")
	}
}

// TestExists_NotExistErrorWrapping verifies that a missing path
// does not leak fs.ErrNotExist to the caller. The contract says
// Exists returns (false, nil) for not-exist; an error is reserved
// for "cannot tell."
func TestExists_NotExistErrorWrapping(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.Exists("nope")
	if err != nil {
		t.Fatalf("Exists on missing path returned error %v, want nil", err)
	}
}

// -----------------------------------------------------------------------------
// MkdirAll
// -----------------------------------------------------------------------------

// TestMkdirAll_CreatesTree verifies nested creation.
func TestMkdirAll_CreatesTree(t *testing.T) {
	f, dir := newFS(t)

	if err := f.MkdirAll("a/b/c", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "a", "b", "c"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("created entry is not a directory")
	}
}

// TestMkdirAll_Idempotent verifies that creating an existing
// directory returns nil, matching os.MkdirAll.
func TestMkdirAll_Idempotent(t *testing.T) {
	f, _ := newFS(t)
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("first MkdirAll: %v", err)
	}
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("second MkdirAll: %v", err)
	}
}

// TestMkdirAll_OverFile verifies that MkdirAll on an existing file
// returns an error wrapping fs.ErrExist, as documented on FS.MkdirAll.
//
// The underlying os.MkdirAll returns the platform-specific errno
// (ENOTDIR on Linux and macOS); OSFS.MkdirAll normalises it so
// callers can branch on fs.ErrExist without platform detection.
func TestMkdirAll_OverFile(t *testing.T) {
	f, _ := newFS(t)
	if err := f.Write("f", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	err := f.MkdirAll("f", 0o755)
	if err == nil {
		t.Fatal("MkdirAll over a file succeeded, want error")
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrExist", err)
	}
}

// -----------------------------------------------------------------------------
// Remove
// -----------------------------------------------------------------------------

// TestRemove_MissingFile verifies the error wraps fs.ErrNotExist.
func TestRemove_MissingFile(t *testing.T) {
	f, _ := newFS(t)
	err := f.Remove("nope")
	if err == nil {
		t.Fatal("Remove succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestRemove_EmptyDirectorySucceeds verifies the contract that
// Remove deletes an empty directory, matching os.Remove.
func TestRemove_EmptyDirectorySucceeds(t *testing.T) {
	f, _ := newFS(t)
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := f.Remove("d"); err != nil {
		t.Fatalf("Remove on empty directory: %v", err)
	}
	ok, err := f.Exists("d")
	if err != nil {
		t.Fatalf("Exists after Remove: %v", err)
	}
	if ok {
		t.Fatal("directory still exists after Remove")
	}
}

// TestRemove_NonEmptyDirectoryFails verifies that Remove refuses to
// delete a directory with contents. Recursive removal is a
// destructive operation that safefs gates behind an explicit
// policy (WBS 13.2).
//
// The exact error is platform-specific (ENOTEMPTY on Linux,
// ERROR_DIRECTORY_NOT_EMPTY on Windows); the test asserts only
// that an error occurred.
func TestRemove_NonEmptyDirectoryFails(t *testing.T) {
	f, _ := newFS(t)
	if err := f.Write("d/a.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	err := f.Remove("d")
	if err == nil {
		t.Fatal("Remove on non-empty directory succeeded, want error")
	}
}

// -----------------------------------------------------------------------------
// List
// -----------------------------------------------------------------------------

// TestList_Sorted verifies the documented ordering. os.ReadDir
// sorts by filename, and OSFS inherits that.
func TestList_Sorted(t *testing.T) {
	f, _ := newFS(t)
	for _, name := range []string{"c.txt", "a.txt", "b.txt"} {
		if err := f.Write(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("Write %s: %v", name, err)
		}
	}
	entries, err := f.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := names(entries)
	want := []string{"a.txt", "b.txt", "c.txt"}
	if !equal(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

// TestList_DoesNotRecurse verifies that List returns only the
// immediate children of the named directory. Recursion is the job
// of safefs.Root.Walk, which enforces bounds.
func TestList_DoesNotRecurse(t *testing.T) {
	f, _ := newFS(t)
	if err := f.Write("a/b/c.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	entries, err := f.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "a" {
		t.Fatalf("List = %v, want [a]", names(entries))
	}
}

// TestList_MissingDirectory verifies that List on a missing path
// wraps fs.ErrNotExist.
func TestList_MissingDirectory(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.List("nope")
	if err == nil {
		t.Fatal("List succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// -----------------------------------------------------------------------------
// Stat / Lstat
// -----------------------------------------------------------------------------

// TestStat_Missing verifies the error wraps fs.ErrNotExist.
func TestStat_Missing(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.Stat("nope")
	if err == nil {
		t.Fatal("Stat succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestStat_Directory verifies that Stat reports a directory as such.
func TestStat_Directory(t *testing.T) {
	f, _ := newFS(t)
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	info, err := f.Stat("d")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Stat reports file, want directory")
	}
}

// TestLstat_FileAndDirectory verifies the basic positive cases for
// Lstat: a regular file and a directory both return their
// respective metadata, with no ModeSymlink bit set.
//
// The symlink-specific cases live in the safefs symlink suite,
// because constructing a symlink is platform-dependent and is not
// the responsibility of the filesystem package's contract test.
func TestLstat_FileAndDirectory(t *testing.T) {
	f, _ := newFS(t)
	if err := f.Write("a.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	info, err := f.Lstat("a.txt")
	if err != nil {
		t.Fatalf("Lstat(a.txt): %v", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		t.Fatal("Lstat reports a regular file as a symlink")
	}

	info, err = f.Lstat("d")
	if err != nil {
		t.Fatalf("Lstat(d): %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Lstat reports a directory as a non-directory")
	}
}

// TestLstat_Missing verifies the error wraps fs.ErrNotExist.
func TestLstat_Missing(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.Lstat("nope")
	if err == nil {
		t.Fatal("Lstat succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// -----------------------------------------------------------------------------
// Readlink
// -----------------------------------------------------------------------------

// TestReadlink_NonLink verifies that Readlink on a non-link returns
// an error. The exact sentinel is platform-specific (EINVAL on
// POSIX, ERROR_NOT_A_REPARSE_POINT on Windows), so the test
// asserts only that an error occurred.
func TestReadlink_NonLink(t *testing.T) {
	f, _ := newFS(t)
	if err := f.Write("a.txt", []byte("x"), 0o644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	_, err := f.Readlink("a.txt")
	if err == nil {
		t.Fatal("Readlink on a non-link succeeded, want error")
	}
}

// TestReadlink_Missing verifies that Readlink on a missing path
// returns an error. Like the non-link case, the exact sentinel is
// platform-specific, so only the fact of an error is asserted.
func TestReadlink_Missing(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.Readlink("nope")
	if err == nil {
		t.Fatal("Readlink on a missing path succeeded, want error")
	}
}

// -----------------------------------------------------------------------------
// Read (error paths)
// -----------------------------------------------------------------------------

// TestRead_Missing verifies the error wraps fs.ErrNotExist. safefs
// maps this onto forgeerr categories without string matching, so
// the wrapping is load-bearing.
func TestRead_Missing(t *testing.T) {
	f, _ := newFS(t)
	_, err := f.Read("nope")
	if err == nil {
		t.Fatal("Read succeeded, want error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want wrapping fs.ErrNotExist", err)
	}
}

// TestRead_Directory verifies that Read on a directory returns an
// error rather than directory bytes. The exact platform error
// varies; we only require that it is an error.
func TestRead_Directory(t *testing.T) {
	f, _ := newFS(t)
	if err := f.MkdirAll("d", 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if _, err := f.Read("d"); err == nil {
		t.Fatal("Read on directory succeeded, want error")
	}
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func names(entries []fs.DirEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// fsOnlyFS is a minimal adapter that exposes a single io/fs.File
// as an fs.FS rooted at a fake name. It exists only to satisfy the
// unused-import linter for the (otherwise unneeded) fs.ReadFile
// call in TestOpen_ReturnsReadableFile; if that test is simplified
// to use fh.Read directly (which it does), this adapter is
// unnecessary and can be deleted.
//
// If a future refactor keeps the adapter, it should live in a
// test helper file and be documented as such. It is not part of
// the filesystem package's public surface.
type fsOnlyFS struct {
	fh fs.File
}

func (f fsOnlyFS) Open(string) (fs.File, error) { return f.fh, nil }

// Silence an unused-import warning if strings is not used in this
// file. It is imported because a future test may need to inspect
// an error message; if the file ends up not needing it, remove
// both the import and this line.
var _ = strings.Contains
