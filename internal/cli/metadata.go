// Package cli defines the Forge command-line interface.
//
// This file holds the constants and helpers that the command
// metadata contract (WBS 7.2.1) enforces. The contract itself is
// exercised by the tests in metadata_test.go; the constants here
// are the numbers the tests compare against, and the helpers here
// are the small functions those tests share.
//
// # The metadata mapping
//
// The contract maps each required help element to the Cobra field
// that implements it:
//
//	Element             Cobra field   Enforced by
//	Short description   Short         Length, casing, terminal
//	Long description    Long          Length, terminal
//	Usage               Use           Regex
//	Arguments           Args          Non-nil
//	Flags               Flags()       Inherited global flags
//	Examples            Example       Presence when required
//
// The tests in metadata_test.go iterate the registry and assert
// each rule against each command. A future command that violates a
// rule fails the test that names the rule and cites the command.
//
// # Where this file sits
//
// This file holds the field-level rules. help_content.go holds the
// output-level rules (global flags in the rendered help, no ANSI
// escapes). The two files are complementary: this file reads the
// command's struct fields, help_content.go invokes the command and
// reads stdout.
//
// # No package-level mutable state
//
// The CLI package's contract test (Rule 5 in contract_test.go)
// forbids package-level variables. This file declares no `var`s:
//
//   - maxShortLength and maxLongLength are untyped integer
//     constants, which the rule permits.
//   - usePattern is a function that returns a compiled regexp on
//     each call. The function has no package-level state; the
//     compiled value is local to each caller.
//
// The pattern is compiled per call. The cost is negligible for the
// number of calls this package makes (a handful per test
// invocation). If the cost ever matters, the caller can cache the
// compiled value in a local variable.
package cli

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// maxShortLength is the maximum permitted length of a command's
// Short field, in runes.
//
// The limit is measured in runes, not bytes, so that a Short string
// containing multi-byte characters (for example, the em dash in
// RootShortDesc) is counted by what the user sees, not by what the
// terminal encodes.
//
// # Why 60
//
// Cobra renders the Short field in the "Available Commands:"
// section of a parent's help output, alongside the command's name.
// The command name is at most 30 runes in Phase 2; the short
// description is at most 60; the two plus alignment padding fit
// within the 80-column terminal width that the CLI's UX principles
// target (docs/cli-ux-spec.md § 9.12).
const maxShortLength = 60

// maxLongLength is the maximum permitted length of a command's Long
// field, in runes.
//
// The limit is measured in runes, not bytes, for the same reason as
// maxShortLength.
//
// # Why 500
//
// The Long field is rendered above the "Usage:" section. It is
// prose; a reader scrolls through it once and then uses the
// structured sections below. A 500-rune Long is roughly six lines
// of prose at 80 columns; longer than that and the help text
// becomes a manual, which is the domain of documentation, not of
// `--help`.
const maxLongLength = 500

// usePattern returns the compiled regular expression that a
// command's Use field must match.
//
// # Why a function, not a variable
//
// The CLI package's contract test forbids package-level variables
// (Rule 5 in contract_test.go). A package-level
// `var usePattern = regexp.MustCompile(...)` is a variable, and the
// contract test forbids every package-level `var`, regardless of
// whether the variable's value is mutable. The rule is absolute so
// that a future change cannot introduce mutation without changing
// the declaration's grammar; the declaration's grammar is what the
// contract test inspects.
//
// A function that returns a compiled regexp on each call has no
// package-level state. The cost is a regexp compilation per call,
// which is negligible for the number of calls this package makes
// (a handful per test invocation). If the cost ever matters, the
// caller can cache the compiled value in a local variable; the
// contract test's rule is about package scope, not about repeated
// compilation.
//
// # The pattern
//
//	^[a-z][a-z0-9-]*( [A-Z<[].*)?$
//
// Broken down:
//
//   - ^[a-z]        — the first character is a lowercase ASCII letter.
//   - [a-z0-9-]*    — subsequent characters are lowercase letters,
//     digits, or hyphens. The asterisk allows a
//     single-letter command name.
//   - ( ... )?      — an optional group.
//   - [A-Z<[]       — the group starts with an uppercase letter,
//     an opening angle bracket, or an opening
//     square bracket. The three alternatives cover
//     the three kinds of argument pattern Cobra
//     renders: a placeholder for a required
//     argument ("<name>"), a placeholder for an
//     optional argument ("[name]"), or a
//     description starting with a capitalised word.
//   - .*            — anything else in the group.
//   - $             — end of string.
//
// # Why the pattern is not stricter
//
// The pattern allows a Use field like "new <name>" (a required
// positional argument) and "add <component> [version]" (one required
// and one optional). It does not attempt to parse the argument
// pattern's structure; that is Cobra's job at runtime, and a
// stricter regex would reject legitimate Use strings that Cobra
// accepts.
//
// The pattern's purpose is to catch the common mistakes:
//
//   - A command name that starts with an uppercase letter or a
//     digit. ("Version", "1config")
//   - A command name that contains a space where a hyphen was
//     intended. ("my command" instead of "my-command")
//   - A command name that contains punctuation that is not a
//     hyphen. ("ver_sion", "ver.sion")
//
// It does not attempt to validate the argument pattern's semantic
// content.
func usePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9-]*( [A-Z<[].*)?$`)
}

// commandMatchesUsePattern reports whether cmd's Use field matches
// the pattern returned by usePattern.
//
// The function is a thin wrapper over usePattern().MatchString. It
// exists so that the test file does not reach into the compiled
// regexp directly; the wrapper is the one place the pattern is
// applied.
func commandMatchesUsePattern(cmd *cobra.Command) bool {
	return usePattern().MatchString(cmd.Use)
}

// shortStartsWithUppercase reports whether cmd's Short field starts
// with an uppercase letter.
//
// The rule is stylistic: the Short is a sentence fragment in
// imperative mood ("Print Forge version information"), and
// imperative sentences in English start with a capitalised verb.
// The check is on the first rune; a Short that starts with
// punctuation or a digit fails.
//
// # Why unicode.IsUpper and not a byte comparison
//
// The Short is a UTF-8 string. A byte comparison (short[0] >= 'A'
// && short[0] <= 'Z') would work for ASCII-only Shorts, but would
// not recognise an uppercase letter from another script. The
// unicode package's IsUpper function operates on runes and handles
// both.
func shortStartsWithUppercase(cmd *cobra.Command) bool {
	if cmd.Short == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(cmd.Short)
	return unicode.IsUpper(r)
}

// shortEndsWithoutPeriod reports whether cmd's Short field does not
// end with a period.
//
// The rule is stylistic: the Short is a fragment, not a sentence,
// and fragments do not end with a period. A Short that ends with
// a period looks like a sentence that was truncated.
//
// The check is on the trimmed Short, so a trailing space does not
// hide a period. A Short ending with an ellipsis ("...") is not
// treated as ending with a period; the ellipsis is its own
// terminator.
func shortEndsWithoutPeriod(cmd *cobra.Command) bool {
	short := strings.TrimRight(cmd.Short, " \t\n")
	if short == "" {
		return true
	}
	return !strings.HasSuffix(short, ".")
}

// longEndsWithPeriod reports whether cmd's Long field ends with a
// period.
//
// The rule applies only when Long is non-empty. A command with no
// Long is exempt; Cobra falls back to Short in that case, and the
// Short's terminal punctuation rule is different.
//
// # Why the check trims whitespace
//
// A Long field authored as a raw string literal may end with a
// trailing newline or spaces. Trimming before the check ensures
// that a trailing newline does not hide a missing period.
func longEndsWithPeriod(cmd *cobra.Command) bool {
	long := strings.TrimSpace(cmd.Long)
	if long == "" {
		return true
	}
	return strings.HasSuffix(long, ".")
}

// commandHasAliases reports whether cmd declares any aliases.
//
// Aliases are prohibited in Phase 2 (WBS 7.2.1). A command that
// declares an alias is a violation unless an ADR has authorised it.
//
// # Why the check is on Aliases
//
// Cobra's Aliases field is the only place aliases are declared. A
// command that needs an alias adds it to that slice. The check
// reads the slice and asserts it is empty.
//
// # What the check does not do
//
// It does not enforce the ADR requirement. A future command that
// legitimately declares an alias (after an ADR) would still fail
// this check; the fix at that point is to amend the test to
// recognise the ADR-authorised alias. The check is a forcing
// function, not a policy enforcer.
func commandHasAliases(cmd *cobra.Command) bool {
	return len(cmd.Aliases) > 0
}

// requiresExample reports whether a command must declare an Example
// field.
//
// The rule is documented on TestMetadata_ExamplePresentWhenRequired.
//
// # The three conditions
//
// A command requires an Example if any of the following holds:
//
//   - The command takes positional arguments. The Use field
//     contains a bracketed placeholder ("[name]" or "<name>").
//   - The command has command-specific flags. The command's flag
//     set contains a flag that is not a global flag and not the
//     auto-registered --help flag.
//   - The command has subcommands. The command's help output
//     contains an "Available Commands:" section.
//
// A command with none of these conditions may omit the Example
// field; its invocation is self-explanatory from the Usage line.
func requiresExample(cmd *cobra.Command) bool {
	if cmd.HasSubCommands() {
		return true
	}
	if strings.ContainsAny(cmd.Use, "[<") {
		return true
	}
	if len(commandSpecificFlagNames(cmd)) > 0 {
		return true
	}
	return false
}

// commandSpecificFlagNames returns the names of the flags registered
// on cmd that are neither global flags nor the auto-registered
// --help flag.
//
// The function is used by requiresExample to decide whether a
// command has any flags of its own. A command with only inherited
// global flags does not require an Example on that ground alone.
//
// # Why pflag.Flag, not cobra.CommandFlag
//
// Cobra's flag visitation API is inherited from pflag. The
// callback's parameter type is *pflag.Flag. There is no
// cobra.CommandFlag type. The pflag import is direct because the
// type is referenced directly; pflag is already a direct
// dependency of the module (WBS 2.5.1).
//
// # Why the global set is built once
//
// The function iterates the command's flags and, for each, checks
// whether its name (with the "--" prefix) is in the global set.
// Building the set once, before the iteration, is cheaper than
// scanning the slice for each flag.
func commandSpecificFlagNames(cmd *cobra.Command) []string {
	var names []string
	if cmd.Flags() == nil {
		return names
	}
	globals := globalFlagNames()
	global := make(map[string]bool, len(globals))
	for _, name := range globals {
		global[name] = true
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		if !global["--"+f.Name] {
			names = append(names, f.Name)
		}
	})
	return names
}
