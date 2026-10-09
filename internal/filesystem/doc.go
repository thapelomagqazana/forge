// Package filesystem defines the Forge filesystem abstraction.
//
// # What this package is
//
// This package defines the FS interface and provides an operating-
// system-backed implementation. Every filesystem operation in Forge
// flows through this interface.
//
// The interface exists for three reasons:
//
//  1. Testability. Tests can substitute an in-memory implementation
//     for the operating system's filesystem, which makes tests fast
//     and deterministic.
//
//  2. Security. All filesystem access flows through one place, so
//     path-traversal and symlink-escape protections can be enforced
//     consistently.
//
//  3. Replaceability. If Forge ever needs to support a non-OS
//     filesystem (for example, a virtual filesystem for testing or a
//     remote filesystem for a future cloud mode), the interface
//     allows that without changing the application layer.
//
// # What this package is not
//
// This package is not the complete filesystem abstraction. Phase 2
// implements the interface, but boundary enforcement and atomic
// writes are deferred to WBS 13.0. The interface is stable; the
// implementation will be strengthened without changing the contract.
//
// # Relationship to other packages
//
// This package is imported by:
//
//   - internal/cli, which provides an FS implementation to commands
//     through the Dependencies struct.
//   - internal/renderer, which reads template source files through
//     the FS interface (Phase 4).
//   - internal/update, which reads and writes repository files
//     through the FS interface (Phase 14).
//
// This package imports the standard library only. It does not import
// any other Forge package.
package filesystem
