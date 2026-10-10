// Package cli implements the Forge command-line interface.
//
// This file defines validateArgs, the pre-parse argument validator.
// It rejects four malformed invocations that Cobra's flag parser
// and dispatcher would otherwise accept silently. See the function's
// docstring for the four cases and for the rationale of placing the
// check before Cobra parses the arguments.
package cli

import (
	"fmt"
	"strings"
)

// validateArgs inspects the raw argument list and returns an error
// for invocations that Cobra's parser and dispatcher would accept
// but that Forge's contracts reject.
//
// # What the function rejects
//
// Four malformed invocations, one per contract section:
//
//  1. `forge --help <cmd>` — see validateHelpFlagWithArgs.
//     Contract: docs/cli-ux-spec.md § 4.10, rule 6.
//
//  2. `forge --version <arg>` — see validateVersionFlagWithArgs.
//     Contract: docs/cli-ux-spec.md § 4.11, rule 5.
//
//  3. `forge --config` with no value — see
//     validateConfigFlagWithArgs. Contract: docs/cli-ux-spec.md
//     § 4.12, rule 5.
//
//  4. `forge help <unknown>` — see validateHelpTopic.
//     Contract: docs/cli-ux-spec.md § 4.10, rule 8.
//
// # Why the checks are here and not in Cobra hooks
//
// For the help flag and the version flag, Cobra's interception runs
// before any hook: PersistentPreRunE, PreRunE, and RunE are all
// skipped when --help or --version is set. The only place to observe
// the malformed invocation is before Cobra parses the arguments —
// i.e., here, in the CLI's pipeline, before Execute is called on the
// root command.
//
// For the help-topic case, Cobra's built-in help command does run
// the hook chain, so a PersistentPreRunE check would work. For the
// config case, Cobra's own parser rejects the missing value but
// with its own error message; the pre-parse check gives the
// rejection the same shape as the other three. Splitting the four
// cases across two or three mechanisms is more complex than
// handling all of them here. The four checks share the same
// structure — "does this raw argument list match a forbidden
// pattern?" — and belong together. A single function that rejects
// all four is easier to read, easier to test, and easier to extend.
//
// # What the function does not do
//
// It does not parse flags. It does not resolve commands. It does not
// know about the constructed command tree. It reads the raw argument
// list as a sequence of strings and matches four specific patterns:
//
//   - The string "--help" or "-h" appears before any positional
//     argument, and at least one positional argument follows.
//
//   - The string "--version" or "-v" appears before any positional
//     argument, and at least one positional argument follows.
//
//   - The string "--config" appears as the last argument or is
//     followed by another flag.
//
//   - The first positional argument is "help", and the second
//     positional argument (the topic) is not a known command name.
//
// The fourth pattern requires the function to know the set of known
// command names. It reads them from the registry (registry.go), not
// from the constructed command tree, because the tree is not
// available at this point in the pipeline.
//
// # Why the registry and not the command tree
//
// The command tree is constructed by newRootCmd, which is called
// after validateArgs in executeWithOptions. The registry is a
// package-level slice that is available before the tree is built.
// Using the registry keeps the dependency order correct: validate
// before construct.
//
// # What "a known command name" means
//
// The set of known names is:
//
//   - Every visible command in the registry (version, and any
//     future commands added by later WBS items).
//   - Every hidden command in the registry (config, in Phase 2).
//   - Cobra's auto-generated "help" and "completion" commands.
//
// The hidden commands and the auto-generated commands are valid
// help topics: `forge help config` and `forge help completion` are
// legitimate invocations. The function accepts them by name.
//
// # The scans are deliberately simple
//
// The scans do not handle flag values that look like flags
// (`--format --json`), quoted arguments, or the `--` separator.
// Forge does not use any of those in Phase 2, and the simple scans
// are sufficient for the invocations the contracts reject. If a
// future WBS item adds a flag that takes a value, this file must be
// revisited.
//
// The `--config=path` form is handled by the check for the exact
// string `--config`. Because the check is an equality comparison,
// `--config=path` does not match it and is treated as well-formed.
// That is correct: the form carries its own value and does not
// require the next argument.
//
// # Purity
//
// The function is pure with respect to its inputs: it reads args and
// registry, both of which are read-only during its execution, and
// returns an error or nil. It does not write to any stream, does not
// touch the filesystem, and does not mutate any state.
//
// # Concurrency
//
// The registry is read-only after package initialization. The args
// slice is owned by the caller. Concurrent calls to validateArgs
// with different args slices are safe.
func validateArgs(args []string) error {
	if err := validateHelpFlagWithArgs(args); err != nil {
		return err
	}
	if err := validateVersionFlagWithArgs(args); err != nil {
		return err
	}
	if err := validateConfigFlagWithArgs(args); err != nil {
		return err
	}
	if err := validateHelpTopic(args); err != nil {
		return err
	}
	return nil
}

// validateHelpFlagWithArgs rejects `forge --help <cmd>` and its
// short form `forge -h <cmd>`.
//
// # The malformed shape
//
// The invocation contains a help flag (--help or -h) followed by
// one or more positional arguments. Cobra's flag parser accepts
// this: --help is a bool flag, so it does not consume the next
// token; the token is treated as a positional argument. Cobra's
// dispatcher then intercepts for help and ignores the positional
// argument.
//
// # What the function returns
//
// For a malformed invocation, the function returns an error whose
// message names the offending positional argument and suggests the
// correct form (`forge help <cmd>`). For a well-formed
// invocation — one with no help flag, or with a help flag but no
// following positional argument — the function returns nil.
//
// # The scan
//
// The function scans args from left to right. It tracks whether a
// help flag has been seen. When it reaches the first positional
// argument (the first element not starting with "-"), it checks
// whether a help flag was seen earlier. If so, the invocation is
// malformed.
//
// The scan does not recurse or backtrack. It reads args once, in
// order. The simplicity is intentional: the forbidden pattern is a
// simple prefix, and the scan that detects it should be equally
// simple.
func validateHelpFlagWithArgs(args []string) error {
	seenHelpFlag := false
	seenHelpFlagName := ""

	for _, arg := range args {
		// A positional argument ends the scan. If a help flag
		// appeared before it, the invocation is malformed.
		if !strings.HasPrefix(arg, "-") {
			if seenHelpFlag {
				return fmt.Errorf(
					"%s takes no arguments; got %q. "+
						"Use `forge help %s` instead",
					seenHelpFlagName, arg, arg)
			}
			return nil
		}

		// Track the help flag. Both forms are equivalent; the
		// first one seen is the one named in the error message.
		if arg == "--help" && !seenHelpFlag {
			seenHelpFlag = true
			seenHelpFlagName = "--help"
		}
		if arg == "-h" && !seenHelpFlag {
			seenHelpFlag = true
			seenHelpFlagName = "-h"
		}
	}

	// The loop consumed every argument without finding a
	// positional one. The invocation has no positional arguments;
	// it is well-formed.
	return nil
}

// validateVersionFlagWithArgs rejects `forge --version <arg>` and
// its short form `forge -v <arg>`.
//
// # The malformed shape
//
// The invocation contains a version flag (--version or -v)
// followed by one or more positional arguments. Cobra's flag
// parser accepts this: --version and -v are bool flags, so they do
// not consume the next token; the token is treated as a positional
// argument. Cobra's dispatcher then intercepts for version and
// ignores the positional argument.
//
// # Why the rejection is correct
//
// The version flag takes no argument. Its purpose is to print the
// version block and exit. An extra positional argument after it is
// not part of the invocation's grammar. Silently ignoring it would
// suggest to the user that their input was accepted; rejecting it
// with a diagnostic tells them the invocation was malformed and
// suggests the correct form.
//
// # Relationship to the help-flag validator
//
// The two validators have the same shape and the same rationale.
// They are kept separate because they reject different flags and
// their error messages name the offending flag. A future refactor
// that unified them under a single "flag with args" validator
// would have to parameterize the flag name, the accepted aliases,
// and the error template; the current form is simpler to read and
// to extend.
//
// # What the function returns
//
// For a malformed invocation, the function returns an error whose
// message names the offending positional argument and the flag
// that was misused. For a well-formed invocation — one with no
// version flag, or with a version flag but no following positional
// argument — the function returns nil.
func validateVersionFlagWithArgs(args []string) error {
	seenVersionFlag := false
	seenVersionFlagName := ""

	for _, arg := range args {
		// A positional argument ends the scan. If a version flag
		// appeared before it, the invocation is malformed.
		if !strings.HasPrefix(arg, "-") {
			if seenVersionFlag {
				return fmt.Errorf(
					"%s takes no arguments; got %q. "+
						"Use `forge version` for the version block",
					seenVersionFlagName, arg)
			}
			return nil
		}

		// Track the version flag. Both forms are equivalent; the
		// first one seen is the one named in the error message.
		if arg == "--version" && !seenVersionFlag {
			seenVersionFlag = true
			seenVersionFlagName = "--version"
		}
		if arg == "-v" && !seenVersionFlag {
			seenVersionFlag = true
			seenVersionFlagName = "-v"
		}
	}

	// The loop consumed every argument without finding a
	// positional one. The invocation has no positional arguments;
	// it is well-formed.
	return nil
}

// validateConfigFlagWithArgs rejects `forge --config` with no
// following value.
//
// # The malformed shape
//
// The invocation contains the string flag --config as the last
// argument, or followed immediately by another flag. In either
// case, the flag is present but its value is not.
//
// # Why the rejection is correct
//
// `--config` takes a value: the path to a configuration file.
// Without a value, the flag is incomplete. Cobra's parser would
// reject the invocation on its own, but with Cobra's own error
// message and via Cobra's own error path. The pre-parse check
// gives the rejection the same shape as the other three:
// a diagnostic on stderr and the exit code that
// exitCodeFromError produces for a plain error (ExitUsage).
//
// # The `--config=path` form
//
// The check compares each argument to the exact string "--config"
// (and its short form "-config", which Cobra accepts by default).
// The `--config=path` form does not match the comparison because
// the argument is `--config=path`, not `--config`. That form
// carries its own value and does not require the next argument.
// It is treated as well-formed.
//
// # The scan
//
// The function scans args from left to right. When it finds an
// argument equal to "--config", it checks whether the next argument
// exists and is not itself a flag. If the next argument is missing
// or is a flag, the invocation is malformed.
//
// The scan is deliberately simple. It does not attempt to handle
// the case where the value looks like a flag (for example,
// `--config --verbose`, where the user intended the value to be
// the literal string "--verbose"). Such a value would require the
// `--config=--verbose` form; the simple scan rejects the unquoted
// form, which is the correct behaviour.
//
// # What the function returns
//
// For a malformed invocation, the function returns an error whose
// message names the flag and explains what is expected. For a
// well-formed invocation — one with no --config, or with --config
// followed by a value — the function returns nil.
func validateConfigFlagWithArgs(args []string) error {
	for i, arg := range args {
		// Match the exact string "--config" or "-config". The
		// "--config=path" form does not match; see the docstring.
		if arg != "--config" && arg != "-config" {
			continue
		}

		// The flag is present. Check for a following value.
		if i+1 >= len(args) {
			return fmt.Errorf(
				"--config requires a value; got the flag with no value. " +
					"Use `--config <path>` to specify a configuration file")
		}
		next := args[i+1]
		if strings.HasPrefix(next, "-") {
			return fmt.Errorf(
				"--config requires a value; got %q. "+
					"Use `--config <path>` to specify a configuration file",
				next)
		}

		// The flag has a value. The scan continues: a second
		// occurrence of --config in the same invocation would be
		// unusual, but it is not a shape this validator rejects.
		// Cobra's parser would use the last occurrence's value;
		// the validator does not need to duplicate that policy.
	}
	return nil
}

// validateHelpTopic rejects `forge help <unknown>`, where the
// topic does not match any known command name.
//
// # The malformed shape
//
// The invocation is `forge help <topic>`, where <topic> is not the
// name of any registered command, and not one of the auto-generated
// commands Cobra registers (help, completion).
//
// # What the function returns
//
// For an unknown topic, the function returns an error whose
// message names the topic and suggests running `forge --help` for
// a list of valid commands. For a well-formed invocation — one
// that is not a `help <topic>` invocation, or whose topic is
// known — the function returns nil.
//
// # The scan
//
// The function finds the first positional argument. If it is not
// "help", the invocation is not a `help` invocation and the
// function returns nil. If it is "help", the function finds the
// next positional argument (the topic). If there is none, the
// invocation is `forge help`, which is well-formed; the function
// returns nil. If the topic is known, the function returns nil;
// otherwise, it returns an error.
func validateHelpTopic(args []string) error {
	// Find the first positional argument.
	firstPositional := -1
	for i, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			firstPositional = i
			break
		}
	}

	// The invocation is not a `help` invocation.
	if firstPositional < 0 || args[firstPositional] != "help" {
		return nil
	}

	// Find the topic: the next positional argument after "help".
	topic := ""
	for i := firstPositional + 1; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			topic = args[i]
			break
		}
	}

	// `forge help` with no topic is well-formed: it prints root
	// help and exits 0. The no-topic case is covered by the
	// no-args/help contract.
	if topic == "" {
		return nil
	}

	// The topic must be a known command name.
	if isKnownCommandName(topic) {
		return nil
	}

	return fmt.Errorf(
		"unknown help topic %q. "+
			"Use `forge --help` for a list of commands",
		topic)
}

// isKnownCommandName reports whether name is a valid help topic.
//
// # The set of known names
//
// The function checks three sources:
//
//  1. The registry. Every constructor in the registry produces a
//     command whose name is a valid help topic. The check iterates
//     the registry and constructs each command to read its Use
//     field. The construction is cheap and side-effect-free; the
//     resulting command is discarded.
//
//  2. Cobra's auto-generated commands. The names "help" and
//     "completion" are reserved by Cobra and are always valid
//     help topics, even though they do not appear in the registry.
//
//  3. The empty string. The empty string is not a valid topic and
//     the function returns false for it, but the caller
//     (validateHelpTopic) already filters the empty string before
//     calling this function. The check is defensive.
//
// # Why the registry and not the command tree
//
// The command tree is constructed after validateArgs runs. The
// registry is available before. Using the registry keeps the
// dependency order correct.
func isKnownCommandName(name string) bool {
	if name == "" {
		return false
	}

	// Cobra's auto-generated commands. They are valid help
	// topics; they are not in the registry.
	if name == "help" || name == "completion" {
		return true
	}

	// The registry's commands. The check constructs each command
	// to read its name. The construction requires a Dependencies
	// value; the value is used only to satisfy the constructor's
	// signature and is discarded with the command.
	for _, ctor := range registry {
		cmd := ctor(validationDependencies())
		if commandName(cmd.Use) == name {
			return true
		}
	}

	return false
}

// commandName returns the first whitespace-separated word of a
// command's Use string. It is the command's canonical name, as
// Cobra's dispatcher sees it.
//
// # Why this helper exists separately from firstWord
//
// firstWord is defined in registry_test.go. Test files are not
// part of the production build, so production code cannot call
// firstWord. Rather than move firstWord into a production file
// (which would couple production code to a testing utility),
// validate.go defines its own small helper for the narrow purpose
// of extracting a command's name from its Use field.
//
// # Why the helper is not exported
//
// The helper is an implementation detail of validate.go. No other
// file in the package needs it. Exported names are part of the
// package's surface; an unexported name keeps the surface small.
//
// # Behaviour for edge cases
//
//   - An empty Use string returns "".
//   - A Use string containing only whitespace returns "".
//   - A Use string like "version --help" returns "version".
//   - A Use string with a leading space returns the first word
//     after the space (Go's strings.Fields discards leading
//     whitespace).
func commandName(use string) string {
	fields := strings.Fields(use)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// validationDependencies returns a Dependencies value suitable for
// constructing a command during argument validation.
//
// The value is used only to call a command constructor and read
// its Use field; the constructed command is never executed. The
// value therefore does not need a real logger, filesystem, or
// config. A value with a no-op logger and nil writers is
// sufficient.
//
// # Why a separate helper and not testDependencies
//
// testDependencies lives in testhelper_test.go and is not
// available in production code. This function is the production
// equivalent: it returns a value that is safe to use for the
// narrow purpose of constructing a command and reading its name.
// It does not need to be as complete as the test helper, because
// the constructed command is never executed and its fields are
// only inspected, not used.
//
// # Why the value has a no-op logger
//
// Every command constructor that reads deps.Logger (none in Phase
// 2, but possible in a future phase) receives a logger that
// discards its messages. The no-op logger is the correct choice
// for a value that exists only to satisfy a constructor's
// signature.
//
// # Why the writers are nil
//
// The writers are not used during construction. A command's RunE
// closure captures deps.Stdout and deps.Stderr, but the closure is
// not called; only the command's metadata (Use, Short, Long) is
// read. Nil writers are therefore safe.
func validationDependencies() Dependencies {
	return Dependencies{
		Logger: newNoopLogger(),
	}
}
