package logger

// Nop returns a Logger whose methods do nothing.
//
// # When to use Nop
//
//   - In tests that do not assert on logs, to avoid writing to
//     stderr and cluttering the test output.
//   - In code paths where logging is disabled by configuration.
//   - As the default value of an optional Logger field.
//
// # Guarantees
//
//   - No method allocates.
//   - No method writes to any stream.
//   - No method panics for any input, including nil contexts and
//     odd-length args.
//   - With returns the same Nop value, so the chain length is
//     constant regardless of how many With calls are made.
//
// # Why a single value
//
// The Nop logger is a value type with no state. A single shared
// instance is returned on every call to Nop; there is no reason
// to allocate a fresh one. The value is safe for concurrent use
// (it has no mutable state).
func Nop() Logger {
	return nopLogger{}
}

// nopLogger is the no-op implementation of Logger.
//
// The type is a value (not a pointer): copying it is free, and
// comparing two nopLoggers for equality compares nothing. It is
// safe for concurrent use by construction.
type nopLogger struct{}

// Debug implements Logger. It does nothing.
func (nopLogger) Debug(string, ...any) {}

// Info implements Logger. It does nothing.
func (nopLogger) Info(string, ...any) {}

// Warn implements Logger. It does nothing.
func (nopLogger) Warn(string, ...any) {}

// Error implements Logger. It does nothing.
func (nopLogger) Error(string, ...any) {}

// With implements Logger. It returns the same Nop logger.
//
// The return value is the same interface value as the receiver,
// so a chain of With calls returns a single Nop value with no
// allocation per call.
func (n nopLogger) With(...any) Logger { return n }
