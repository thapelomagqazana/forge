// Command forge-internal-version is an internal build tool for the
// Forge versioning and release-channel scheme. It is not distributed
// to users.
//
// Usage:
//
//	forge-internal-version verify
//	forge-internal-version channel <tag>
//	forge-internal-version tag-table
//	forge-internal-version fmt
//	forge-internal-version notes check <path>
//
// Subcommands:
//
//	verify           Load and validate docs/reference/version-metadata.yaml
//	                 against schemas/version-metadata.schema.json and the
//	                 rules in docs/reference/versioning.md.
//	channel <tag>    Print the release channel a git tag belongs to, or
//	                 exit non-zero if the tag matches no channel.
//	tag-table        Print the tag-to-channel table for a corpus of
//	                 example tags. Used by docs and by CI debugging.
//	fmt              Sort identifiers and channels in
//	                 version-metadata.yaml and rewrite it in place.
//	                 Idempotent.
//	notes check      Validate a filled-in release notes file against
//	                 the template's required headings for its channel
//	                 and bump class.
//
// Exit codes:
//
//	0  success
//	1  validation or I/O failure
//	2  usage error
//
// This tool is invoked by:
//
//	Taskfile.yml targets version:verify, version:tag-table, version:fmt
//	.github/workflows/release.yml  (channel selection, notes check)
//
// It lives in a separate cmd/ directory so it does not link into the
// user-facing cmd/forge binary and does not appear in the release
// artifact. cmd/forge is the process entry point and the exit boundary
// (ADR-020); it must not grow a --format flag for version metadata.
package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

// run dispatches to a subcommand. It is separated from main so it can
// be driven from a test in the same package without spawning a
// subprocess.
func run(args []string) int {
	if len(args) == 0 {
		usage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "verify":
		return cmdVerify(os.Stdout, os.Stderr)
	case "channel":
		return cmdChannel(args[1:], os.Stdout, os.Stderr)
	case "tag-table":
		return cmdTagTable(os.Stdout, os.Stderr)
	case "fmt":
		return cmdFmt(os.Stdout, os.Stderr)
	case "notes":
		return cmdNotes(args[1:], os.Stdout, os.Stderr)
	case "-h", "--help", "help":
		usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "forge-internal-version: unknown subcommand %q\n", args[0])
		usage(os.Stderr)
		return 2
	}
}

func usage(w *os.File) {
	const help = `forge-internal-version — internal build tool for the Forge version scheme

Usage:
  forge-internal-version verify
  forge-internal-version channel <tag>
  forge-internal-version tag-table
  forge-internal-version fmt
  forge-internal-version notes check <path>

Run with -h for this message.`
	fmt.Fprintln(w, help)
}
