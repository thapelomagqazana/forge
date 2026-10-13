// Package logger implements the operational surface of the forge CLI.
//
// # Purpose
//
// The logger carries *diagnostics* — information about how a command ran,
// not what it produced. Logs, traces, timing, debug context. They are
// written to stderr only, are level-controlled, and are never injected
// into stdout.
//
// # Layer contract
//
//	Output  → stdout  (product result)
//	Logger  → stderr  (operational diagnostics)
//	Errors  → stderr  (rendered by internal/forgeerr)
//
// This package MUST NEVER write to stdout. Product output belongs to
// internal/output. See ADR-022 and docs/architecture/output-vs-logging.md.
//
// # Levels
//
// Four levels are supported, in increasing severity:
//
//	debug  fine-grained, disabled by default
//	info   normal operational events
//	warn   recoverable problems
//	error  failures (note: the error *value* goes through forgeerr;
//	       the logger only records that an error occurred)
//
// Level is selected by --log-level; format by --log-format.
//
// # Usage
//
//	logger.Info(ctx, "resolved version", "version", v)
//	logger.Debug(ctx, "cache hit", "key", k)
//
// # Testing
//
// Inject a Sink via logger.SetSink to capture records in tests. The
// default Sink writes to os.Stderr.
//
// # Anti-patterns
//
// Do NOT use fmt.Println or os.Stdout.Write here.
// Do NOT log the product result — that is output.
// Do NOT log secrets; redaction is handled by internal/forgeerr.
package logger
