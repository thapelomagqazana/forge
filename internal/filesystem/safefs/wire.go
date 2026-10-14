package safefs

import (
	"github.com/thapelomagqazana/forge/internal/filesystem"
)

// This file exists for exactly one reason: to keep the string "os"
// out of the import block of every other file in this package.
//
// # Why a whole file for one function
//
// The layering rule (ADR-035) is enforced by
// scripts/deps/verify-fs-layering.sh, which scans each package's
// import list for `"os"` and fails the build if the package is not
// on the allow-list. The allow-list contains exactly two entries:
//
//   - internal/filesystem (the OSFS implementation)
//   - cmd/forge          (the process entrypoint)
//
// safefs is deliberately NOT on the allow-list. safefs must be
// able to construct an OSFS without importing os, which is what
// newOSFS does. The function is a one-line passthrough today; it
// is a named function rather than a direct call because a future
// platform-specific constructor (for example, a Linux-only
// chroot-aware OSFS, or a Windows-only OSFS that handles long
// paths) can be introduced here without touching New's body or
// changing the package's import surface.
//
// # Why the indirection is worth its cost
//
// The alternative — importing filesystem directly in root.go and
// calling filesystem.NewOSFS(abs) — would work. It would also
// mean that every reader of root.go must scan its import block
// to check whether os is present. The CI rule operates at the
// package level, so a single violation anywhere in the package
// fails the build; the file boundary does not change that. What
// the file boundary does change is human reviewability: a
// reviewer who wants to confirm that safefs does not touch os
// directly can `grep -R '"os"' internal/filesystem/safefs` and
// see zero results. If the constructor were inlined, the grep
// would still return zero results — because the import is
// indirect through filesystem — but the reviewer's confidence
// would rest on the reviewer having read filesystem/osfs.go.
// Concentrating the indirection in one file, with this comment,
// makes the confidence rest on reading one file.
//
// # What this function must never do
//
// newOSFS must never return anything other than a
// filesystem.FS. It must never leak a concrete *os.File, an
// *os.Root, or any other os-typed value. It must never grow a
// parameter that names an os type. If a future constructor needs
// to return a platform-specific type, that type is defined in
// internal/filesystem, not here; this function's return type is
// the interface, and the interface is the boundary.
//
// newOSFS must also never gain a caller outside New. It is
// unexported and named for its single purpose. A second caller
// would be a second place that safefs constructs a filesystem,
// and a second place to keep in sync with policy changes. The
// one-caller discipline is checked by inspection; the function
// is short enough that the check is trivial.

// newOSFS returns the production filesystem.FS implementation
// bound to root.
//
// root is the absolute, cleaned directory that New has already
// validated. newOSFS does not re-validate it: doing so would
// duplicate the validation in New and create a second place where
// the invariant must be kept correct.
//
// The returned FS is the only thing safefs ever asks the
// operating system to do. Every method on Root routes through
// it. There is no other path from safefs to the disk, and the
// layering lint is what keeps it that way.
func newOSFS(root string) filesystem.FS {
	return filesystem.NewOSFS(root)
}
