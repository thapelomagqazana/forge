// Package output implements the product surface of the forge CLI.
//
// # Purpose
//
// Output is the *result* of a command — what the user (human or machine)
// asked for. It is the CLI's contract with the outside world. Output is
// written to stdout only, is schema-versioned, and is golden-tested.
//
// # Layer contract
//
//	Output  → stdout  (product result)
//	Logger  → stderr  (operational diagnostics)
//	Errors  → stderr  (rendered by internal/forgeerr)
//
// This package MUST NEVER write to stderr. Diagnostics belong to
// internal/logger. See ADR-022 and docs/architecture/output-vs-logging.md.
//
// # Formats
//
// Two formats are supported, selected by the global --format flag:
//
//	text  human-readable, stable but not machine-parsed
//	json  machine-readable, exactly one JSON envelope on stdout
//
// When format is JSON, stdout contains ONLY the envelope. Log lines
// continue on stderr unchanged and are never injected into stdout.
//
// # Usage
//
//	// Simple value output:
//	output.Print(ctx, "hello")
//
//	// Structured JSON envelope:
//	output.PrintJSON(ctx, versionInfo)
//
// # Testing
//
// Inject a Writer via output.NewWriter to capture output in tests.
// The default Writer is bound to os.Stdout.
//
// # Anti-patterns
//
// Do NOT use fmt.Println, fmt.Printf, or os.Stdout.Write in domain code.
// Do NOT write diagnostics here — use internal/logger.
package output
