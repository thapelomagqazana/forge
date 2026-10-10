// Package cli contains white-box tests for the CLI package.
//
// This file pins the command metadata contract (WBS 7.2.1). The
// contract maps each required help element to a Cobra field and
// asserts the field's shape; each test iterates every command in
// the registry and checks one rule. A future command that violates
// the contract fails the test that names the rule.
//
// # Why a separate file from help_content_test.go
//
// help_content_test.go asserts the shape of the *rendered help
// output*: global flags present, no ANSI escapes. This file
// asserts the shape of the *metadata fields*: Short, Long, Use,
// Args, Example, Aliases. The two kinds of assertion read different
// sources of truth (the struct vs. stdout), so they live in
// different files.
//
// The two files overlap on three rules — Short length, Long length,
// Long terminal punctuation — because those rules can be enforced
// at either layer. The metadata test gives the precise diagnosis
// ("Short does not start with an uppercase letter"); the help
// golden test catches changes the metadata test does not ("the
// Examples block reordered the rows"). Both are kept.
//
// # How the tests iterate
//
// Each test iterates allRegisteredCommands() (defined in
// help_content.go), which returns one *cobra.Command per entry in
// the registry. The iteration includes hidden commands; the
// contract applies to them the same way it applies to visible ones.
package cli

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// =============================================================================
// Use field
// =============================================================================

// TestMetadata_UseMatchesPattern verifies that every command's Use
// field matches usePattern.
//
// The pattern is documented on usePattern in metadata.go. The
// common mistakes it catches are:
//
//   - A command name that starts with an uppercase letter or a
//     digit. ("Version", "1config")
//   - A command name that contains a space where a hyphen was
//     intended. ("my command")
//   - A command name that contains punctuation that is not a
//     hyphen. ("ver_sion", "ver.sion")
//
// # What the test catches
//
// A future command whose Use field does not match. The failure
// quotes the Use field and the pattern, so the contributor sees
// exactly what the pattern requires.
func TestMetadata_UseMatchesPattern(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if !commandMatchesUsePattern(cmd) {
				t.Errorf("Use does not match pattern\n"+
					"Use:     %q\n"+
					"Pattern: %s",
					cmd.Use, usePattern().String())
			}
		})
	}
}

// =============================================================================
// Short field
// =============================================================================

// TestMetadata_ShortStartsWithUppercase verifies that every
// command's Short field starts with an uppercase letter.
//
// The rule is stylistic: the Short is a sentence fragment in
// imperative mood ("Print Forge version information"), and
// imperative sentences in English start with a capitalised verb.
//
// # What the test catches
//
// A future command whose Short starts with a lowercase letter or
// with punctuation. The failure quotes the Short.
func TestMetadata_ShortStartsWithUppercase(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Short == "" {
				t.Fatalf("Short is empty")
			}
			if !shortStartsWithUppercase(cmd) {
				r, _ := utf8.DecodeRuneInString(cmd.Short)
				t.Errorf("Short does not start with an uppercase letter\n"+
					"Short:        %q\n"+
					"First rune:   %q",
					cmd.Short, r)
			}
		})
	}
}

// TestMetadata_ShortEndsWithoutPeriod verifies that every command's
// Short field does not end with a period.
//
// The rule is stylistic: the Short is a fragment, not a sentence,
// and fragments do not end with a period.
//
// # What the test catches
//
// A future command whose Short ends with a period. The failure
// quotes the Short.
func TestMetadata_ShortEndsWithoutPeriod(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if !shortEndsWithoutPeriod(cmd) {
				t.Errorf("Short ends with a period\nShort: %q", cmd.Short)
			}
		})
	}
}

// TestMetadata_ShortWithinLimit verifies that every command's Short
// field is at most maxShortLength runes.
//
// The test was previously in help_content_test.go; it lives here
// now because the rule is a metadata rule, not a help-output rule.
//
// See maxShortLength's docstring for the rationale.
func TestMetadata_ShortWithinLimit(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Short == "" {
				t.Fatalf("Short is empty")
			}
			n := utf8.RuneCountInString(cmd.Short)
			if n > maxShortLength {
				t.Errorf("Short is %d runes; limit is %d\nShort: %q",
					n, maxShortLength, cmd.Short)
			}
		})
	}
}

// =============================================================================
// Long field
// =============================================================================

// TestMetadata_LongEndsWithPeriod verifies that every command whose
// Long field is non-empty has a Long that ends with a period.
//
// The rule applies only when Long is non-empty. A command with no
// Long is exempt.
//
// The test was previously in help_content_test.go; it lives here
// now because the rule is a metadata rule.
//
// # What the test catches
//
// A future command whose Long ends with a colon, a code block, a
// table, or any other non-sentence-terminator. The failure quotes
// the Long.
func TestMetadata_LongEndsWithPeriod(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if !longEndsWithPeriod(cmd) {
				t.Errorf("Long does not end with a period\nLong: %q", cmd.Long)
			}
		})
	}
}

// TestMetadata_LongWithinLimit verifies that every command whose
// Long field is non-empty has a Long at most maxLongLength runes.
//
// The test was previously in help_content_test.go; it lives here
// now because the rule is a metadata rule.
//
// See maxLongLength's docstring for the rationale.
func TestMetadata_LongWithinLimit(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Long == "" {
				return
			}
			n := utf8.RuneCountInString(cmd.Long)
			if n > maxLongLength {
				t.Errorf("Long is %d runes; limit is %d",
					n, maxLongLength)
			}
		})
	}
}

// =============================================================================
// Args validator
// =============================================================================

// TestMetadata_ArgsIsSet verifies that every command has an Args
// validator set.
//
// The rule is documented on the test. The specific function is the
// command's choice; the contract requires a non-nil validator.
//
// The test was previously in help_content_test.go; it lives here
// now because the rule is a metadata rule.
//
// # What the test catches
//
// A future command whose author forgot to set Args. The failure
// names the command.
func TestMetadata_ArgsIsSet(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if cmd.Args == nil {
				t.Errorf("Args is nil; every command must set a validator " +
					"(cobra.NoArgs, cobra.ExactArgs(n), or custom)")
			}
		})
	}
}

// =============================================================================
// Examples
// =============================================================================

// TestMetadata_ExamplePresentWhenRequired verifies that every
// command that requires an Example declares one.
//
// The rule is documented on requiresExample in metadata.go.
//
// The test was previously in help_content_test.go; it lives here
// now because the rule is a metadata rule.
//
// # What the test catches
//
// A future command that takes arguments or has command-specific
// flags but omits the Example field. The failure quotes the Use
// and Short fields.
func TestMetadata_ExamplePresentWhenRequired(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if !requiresExample(cmd) {
				return
			}
			if strings.TrimSpace(cmd.Example) == "" {
				t.Errorf("command requires an Example but Example is empty\n"+
					"Use: %q\nShort: %q",
					cmd.Use, cmd.Short)
			}
		})
	}
}

// =============================================================================
// Aliases
// =============================================================================

// TestMetadata_NoAliases verifies that no command declares aliases.
//
// Aliases are prohibited in Phase 2 (WBS 7.2.1). A command that
// needs an alias must add it with an ADR; this test is amended at
// that point to recognise the specific alias.
//
// # What the test catches
//
// A future command whose author added an alias without an ADR. The
// failure lists the aliases.
func TestMetadata_NoAliases(t *testing.T) {
	t.Parallel()

	for _, cmd := range allRegisteredCommands() {
		cmd := cmd
		name := commandName(cmd.Use)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if commandHasAliases(cmd) {
				t.Errorf("command declares aliases; aliases are prohibited in Phase 2\n"+
					"Aliases: %v", cmd.Aliases)
			}
		})
	}
}
