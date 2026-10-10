// Package cli defines the Forge command-line interface.
//
// This file holds the ASCII-only output policy (WBS 7.4.2). The
// policy freezes two Phase 2 decisions:
//
//   - No colour output. No ANSI escape sequences are emitted.
//   - No non-ASCII characters in output. Every byte is in the
//     ASCII printable range or is a permitted control character
//     (newline, tab).
//
// The policy is documented in docs/cli-ux-spec.md "Colour and
// Terminal Policy" and enforced by the tests in ascii_policy_test.go.
//
// # Why the policy exists
//
// Colour and non-ASCII output interact with shell scripting. A
// script that pipes a command's output to grep expects plain text;
// a terminal that does not render a Unicode glyph expects ASCII.
// The policy defers colour and Unicode to Phase 6+ so that Phase 2's
// output is universally parseable.
//
// # What the policy does not do
//
// The policy does not add TTY detection. Without colour, TTY
// detection has nothing to detect. The policy defers TTY detection
// to Phase 6 as well.
//
// The policy does not add a --no-color flag. The flag would be
// dead code in Phase 2; the policy documents that colour is off,
// and the flag is reserved for Phase 6.
//
// The policy does not read NO_COLOR or CLICOLOR. The environment
// variables are ignored in Phase 2 because no colour is emitted.
// The policy documents the variables so that a future colour
// implementation honours them.
//
// # Future-proofing
//
// When colour is introduced (Phase 6+), the following rules apply:
//
//   - Colour is disabled when NO_COLOR is set, per no-color.org.
//   - Colour is disabled when stdout is not a TTY.
//   - Colour is disabled when --no-color is passed.
//   - Colour never conveys semantic information alone; it is
//     always paired with a symbol or text.
//
// The rules are documented in docs/cli-ux-spec.md "Colour and
// Terminal Policy". They are not tested in Phase 2; a future WBS
// item that introduces colour adds the tests.
//
// # No package-level mutable state
//
// The CLI package's contract test forbids package-level
// variables. This file declares only constants and functions.
package cli

// asciiPrintableMin is the lowest byte value in the ASCII printable
// range.
//
// The range is 0x20 (space) through 0x7e (tilde). Bytes below 0x20
// are control characters; bytes above 0x7e are non-ASCII.
const asciiPrintableMin = 0x20

// asciiPrintableMax is the highest byte value in the ASCII
// printable range.
//
// See asciiPrintableMin for the range's definition.
const asciiPrintableMax = 0x7e

// permittedControlBytes is the set of control bytes that the
// policy allows.
//
// # Why newline and tab are permitted
//
// Newline (0x0a) separates lines; every output's last line ends
// with one. Tab (0x09) is the standard indentation for Cobra's
// help output and for Go source. Both are in the ASCII range and
// are universally rendered.
//
// # Why carriage return is not permitted
//
// Carriage return (0x0d) is a Windows line-ending byte. The CLI's
// output uses Unix line endings (0x0a). A carriage return in the
// output would indicate a line-ending conversion that the policy
// forbids. The absence of carriage returns is enforced by the
// output stream boundary tests (WBS 7.4.1) and by the ASCII test
// (this file).
//
// # Why form feed and backspace are not permitted
//
// Form feed (0x0c) and backspace (0x08) are terminal control
// characters. They are not used by the CLI's output and are not
// permitted by the policy.
//
// # Why the set is a function, not a variable
//
// The CLI package's contract test forbids package-level
// variables. A function that returns the set on each call has no
// package-level state. The cost is one map allocation per call;
// the test calls the function a handful of times per run.
func permittedControlBytes() map[byte]bool {
	return map[byte]bool{
		'\n': true,
		'\t': true,
	}
}

// isASCIIOnly reports whether every byte in s is either in the
// ASCII printable range or is a permitted control byte.
//
// # What the predicate does
//
// The function iterates s's bytes. For each byte b:
//
//   - If b is in [0x20, 0x7e], the byte is printable ASCII; it
//     passes.
//   - If b is in the permitted control set ('\n' or '\t'), the
//     byte passes.
//   - Otherwise, the byte fails.
//
// The function returns true if every byte passed, false otherwise.
// The function does not report the offending byte; a caller that
// wants the byte iterates separately.
//
// # Why byte-level, not rune-level
//
// The policy is byte-level: every byte in the output must be
// ASCII. A rune-level check would accept a multi-byte UTF-8
// sequence as a single character and would miss the fact that the
// sequence contains bytes outside the ASCII range. The byte-level
// check is stricter and matches the policy's intent.
//
// # Boundary cases
//
//   - The empty string is ASCII-only (trivially).
//   - A string of spaces is ASCII-only.
//   - A string containing a tab is ASCII-only.
//   - A string containing a newline is ASCII-only.
//   - A string containing a carriage return is not ASCII-only.
//   - A string containing a byte above 0x7e (for example, part of
//     a UTF-8 multi-byte sequence) is not ASCII-only.
func isASCIIOnly(s string) bool {
	permitted := permittedControlBytes()
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= asciiPrintableMin && b <= asciiPrintableMax {
			continue
		}
		if permitted[b] {
			continue
		}
		return false
	}
	return true
}

// firstNonASCIIByte returns the index of the first byte in s that
// is not ASCII-only, or -1 if no such byte exists.
//
// # Why a separate function
//
// isASCIIOnly returns a boolean; a caller that wants to report the
// offending byte's index calls this function. The two functions
// share the same predicate but serve different reporting needs.
//
// # Boundary cases
//
// The function returns -1 for an empty string, for a string of
// permitted bytes, and for a string of ASCII printable bytes.
func firstNonASCIIByte(s string) int {
	permitted := permittedControlBytes()
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= asciiPrintableMin && b <= asciiPrintableMax {
			continue
		}
		if permitted[b] {
			continue
		}
		return i
	}
	return -1
}

// firstAnsiEscape returns the index of the first ANSI escape byte
// (0x1b, ESC) in s, or -1 if no such byte exists.
//
// # Why ANSI escapes get their own function
//
// The ANSI escape byte (0x1b) is a control byte below the ASCII
// printable range. isASCIIOnly already returns false for a string
// that contains it, but the test's failure message is more
// informative if it names the byte specifically. The function
// exists so the test can distinguish "the output contains an ANSI
// escape" from "the output contains a non-ASCII byte".
func firstAnsiEscape(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			return i
		}
	}
	return -1
}
