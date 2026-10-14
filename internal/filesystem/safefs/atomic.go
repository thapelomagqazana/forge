package safefs

import (
	"crypto/rand"
	"encoding/hex"
	"io/fs"
)

// This file defines the atomic-write operations on Root.
//
// # What "atomic" means here
//
// An atomic write is one where a concurrent reader sees either the
// old content of the target file or the new content, but never a
// partial write. The guarantee is provided by the write-and-rename
// sequence: the data is written to a temporary file in the target's
// directory, fsynced, and then renamed over the target. On POSIX
// systems, rename(2) is atomic within a single filesystem; on
// Windows, MoveFileEx provides the same guarantee.
//
// # What "durable" means here
//
// A durable write is one that survives a power failure after the
// operation returns. The guarantee is provided by two fsyncs: the
// file's content is fsynced before the rename, and the parent
// directory is fsynced after the rename. The first makes the data
// durable; the second makes the directory entry (the rename)
// durable. Both are required; a primitive that performs only the
// first is atomic but not durable.
//
// # Why the primitive routes through the FS interface
//
// The atomic write could be implemented with direct os calls in
// safefs, but the layering rule (ADR-035) forbids safefs from
// importing os. The FS interface is the only package permitted to
// touch the disk; it handles the boundary join and the Windows
// long-path prefix inside OSFS.resolve. Routing through it keeps
// safefs free of platform-specific concerns and lets OSFS be the
// single point where the real filesystem is reached.
//
// The platform-specific pieces of the primitive — the exclusive
// open with symlink-safe flags, and the directory fsync — are
// implemented in OSFS. See internal/filesystem/exclusive_unix.go
// and internal/filesystem/exclusive_windows.go.

// WriteAtomic writes data to name atomically.
//
// # Semantics
//
// WriteAtomic is fail-closed: if name already exists, it returns
// an error wrapping ErrExists. A caller that wants to overwrite
// uses WriteAtomicOverwrite.
//
// The write is atomic: a reader that opens name sees either the
// old content (if name existed before and the caller used
// WriteAtomicOverwrite) or the new content, never a partial
// write. On POSIX, the atomicity is provided by rename(2); on
// Windows, by MoveFileEx with MOVEFILE_REPLACE_EXISTING.
//
// # Mechanism
//
//  1. Resolve the parent of name under the pipeline. This runs
//     validateName, the deny-list and allow-list checks, and the
//     symlink policy.
//  2. Re-verify the parent's non-symlink status. This is the
//     TOCTOU mitigation: it narrows the window between the
//     pipeline's resolution and the write primitive's open to a
//     single syscall.
//  3. Verify the target does not exist (unless the caller used
//     WriteAtomicOverwrite).
//  4. Generate a random temp-file suffix.
//  5. Create the temp file with the platform's symlink-safe
//     exclusive flags. The FS implementation applies O_NOFOLLOW
//     (Unix) or FILE_FLAG_OPEN_REPARSE_POINT (Windows), so a
//     symlink at the final component is refused, and it fsyncs
//     the file's content before returning.
//  6. Rename the temp file to name. The rename is the atomic
//     step.
//  7. Fsync the parent directory. This makes the rename durable.
//     On Unix it is a real fsync; on Windows it is a no-op.
//
// If any step after step 5 fails, the temp file is removed and
// the error is returned. The target is never left in a
// partially-written state.
//
// # Why the temp file is in the same directory
//
// rename(2) is atomic only within a single filesystem. A temp
// file in /tmp renamed to a target in /home/user/project would
// cross filesystems and fail with EXDEV (or, worse, fall back to
// a non-atomic copy). The temp file is created in the same
// directory as the target so that the rename is within a single
// filesystem.
//
// # Why the temp file name is random
//
// Two concurrent writers to the same target would otherwise
// choose the same temp name and race. The random suffix makes
// collisions vanishingly unlikely. The suffix is generated with
// crypto/rand because the same source is available on every
// platform and the cost is negligible.
//
// # The residual race
//
// The fail-closed check (step 3) and the rename (step 6) are not
// atomic with respect to each other. A concurrent writer can
// create the target between the two, and the rename will replace
// the concurrent writer's file. POSIX's rename(2) does not
// support "rename only if the target does not exist"; the
// closest portable equivalent is not available on all platforms.
//
// The guarantee WriteAtomic provides is: *this* process never
// silently overwrites an existing target. A concurrent writer is
// out of scope; a caller that needs stronger coordination uses
// an external lock.
func (r *Root) WriteAtomic(name string, data []byte, mode fs.FileMode) error {
	return r.writeAtomicImpl(name, data, mode, false)
}

// WriteAtomicOverwrite is WriteAtomic with an explicit overwrite.
//
// # Semantics
//
// If name exists, WriteAtomicOverwrite replaces it atomically.
// The atomicity and durability guarantees are the same as
// WriteAtomic: a reader sees either the old content or the new
// content, and the new content survives a power failure.
//
// # Why a distinct method
//
// The fail-closed default (WriteAtomic returns ErrExists) makes
// the caller's intent explicit. A caller that wants to overwrite
// names the overwrite in the method call, and a reviewer sees
// the intent at the call site. The alternative — a boolean
// parameter on WriteAtomic — would hide the intent in an
// argument value, which is invisible in a diff and easy to get
// wrong.
//
// The pattern mirrors os.OpenFile's O_TRUNC flag vs the absence
// of it, and the stdlib's own split between os.Create (which
// overwrites) and os.OpenFile with O_EXCL (which does not). The
// two named methods make the fail-closed policy visible at the
// type level.
func (r *Root) WriteAtomicOverwrite(name string, data []byte, mode fs.FileMode) error {
	return r.writeAtomicImpl(name, data, mode, true)
}

// writeAtomicImpl is the shared implementation of WriteAtomic and
// WriteAtomicOverwrite.
//
// The overwrite parameter controls whether the primitive checks
// the target's absence before writing. The temp file is created
// with exclusive semantics regardless, so the temp-file creation
// never overwrites.
//
// # Concurrency
//
// The function reads r.fs, r.policy, and r.maxSymlinkChain, all
// immutable after New. It calls the FS interface's methods, whose
// concurrency guarantee is documented per method. It is safe for
// concurrent use by multiple goroutines calling the same Root.
func (r *Root) writeAtomicImpl(name string, data []byte, mode fs.FileMode, overwrite bool) error {
	// Step 1: run the pipeline. This validates the name,
	// applies the deny-list and allow-list checks, and resolves
	// the parent under the symlink policy.
	resolved, err := r.resolveForIO(name)
	if err != nil {
		return err
	}

	// Step 2: re-verify the parent has no symlink components.
	// This is the TOCTOU re-verification: it narrows the window
	// between the pipeline's resolution and the write
	// primitive's open. See symlink.go, verifyWritePath.
	if err := r.verifyWritePath(resolved); err != nil {
		return err
	}

	// Step 3: if the caller did not opt into overwrite, verify
	// that the target does not exist.
	//
	// The check is racy against a concurrent writer. The window
	// is narrow (between Exists and Rename below), and closing
	// it would require a "rename only if destination does not
	// exist" syscall that neither POSIX nor Windows provides
	// portably. The residual race is documented on WriteAtomic.
	if !overwrite {
		exists, err := r.fs.Exists(resolved)
		if err != nil {
			return classify(err, "WriteAtomic", name)
		}
		if exists {
			return pathError(ErrExists, name)
		}
	}

	// Step 4: generate a random temp-file suffix.
	suffix, err := randomSuffix()
	if err != nil {
		return err
	}
	tempName := resolved + ".tmp." + suffix

	// Step 5: create the temp file with the platform's
	// symlink-safe flags. The FS implementation applies
	// O_NOFOLLOW (Unix) or FILE_FLAG_OPEN_REPARSE_POINT
	// (Windows), so a symlink at the temp file's final
	// component is refused, and it fsyncs the file's content
	// before returning.
	if err := r.fs.WriteFileExclusive(tempName, data, mode); err != nil {
		return classify(err, "WriteAtomic", name)
	}

	// Step 6: rename the temp file to the target. On Unix this
	// is rename(2); on Windows it is MoveFileEx. The rename is
	// the atomic step: a concurrent reader of the target sees
	// either the old file or the new file, never a partial
	// write.
	if err := r.fs.Rename(tempName, resolved); err != nil {
		// Cleanup: remove the temp file. The cleanup's error
		// is ignored; the primary error is what the caller
		// needs. A leftover temp file is a diagnostic
		// nuisance, not a correctness problem.
		_ = r.fs.Remove(tempName)
		return classify(err, "WriteAtomic", name)
	}

	// Step 7: fsync the parent directory. This makes the
	// rename's directory entry change durable. On Unix it is a
	// real fsync; on Windows it is a no-op (Windows's
	// durability of a rename is provided by the flag on
	// MoveFileEx, if any).
	if err := r.fs.FsyncDir(resolved); err != nil {
		return classify(err, "WriteAtomic", name)
	}

	return nil
}

// randomSuffix returns a random 16-character hex string.
//
// # Why 16 characters
//
// The suffix is used to make temp-file names unique across
// concurrent writers. Two writers would have to generate the same
// 64-bit value to collide, which is a probability of about 1 in
// 1.8 * 10^19 per pair of writes. That is well below the rate at
// which cosmic rays flip bits in memory.
//
// Sixteen hex characters is the same length as the SHA-256
// prefix used elsewhere in the codebase, which makes the temp
// names greppable alongside the content hashes.
//
// # Why crypto/rand and not math/rand
//
// math/rand is fast but not cryptographically random. A
// sufficiently motivated attacker who can predict the sequence
// could pre-create the temp file and win a race. crypto/rand
// removes that vector. The cost is negligible: one read of 8
// bytes per write.
//
// # Error handling
//
// crypto/rand.Read returns an error on a system where the
// entropy source is unavailable. On Linux, macOS, and Windows,
// this is a system-level failure that the caller cannot
// meaningfully recover from; the error is returned to the caller
// and the write does not proceed.
func randomSuffix() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
