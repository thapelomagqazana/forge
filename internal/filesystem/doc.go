// Package filesystem provides portable, policy-free filesystem
// primitives for Forge.
//
// # Contract (WBS 13.1.1, ADR-035)
//
// This package mirrors os and io/fs but abstracts platform
// differences. It contains NO security policy: no symlink policy, no
// traversal checks, no notion of a project root. Any package that
// needs those MUST use internal/filesystem/safefs instead.
//
// The only type in this package that holds an *os.File is OSFS. It
// is the single point at which Forge touches the real disk.
//
// Import direction (enforced by scripts/deps/verify-fs-layering.sh):
//
//	domain → safefs → filesystem → os / io/fs
//
// Reverse imports are forbidden. This package MUST NOT import
// safefs or any domain package.
package filesystem
