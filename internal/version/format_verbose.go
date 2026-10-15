// format_verbose.go implements the human-readable verbose output for
// `forge version --verbose`.
//
// The verbose format is not a machine contract; it is for humans. It
// prints the five linker fields, then a provenance section listing
// the Go toolchain, the module, and every dependency the binary links
// against. It ends with the build settings from ReadBuildInfo.
//
// The format is stable enough to grep: every line starts with a
// label, a colon, and a single space. A future addition appends
// lines; it does not reformat existing ones.
//
// # Why human-readable and not JSON
//
// `forge version --format json --verbose` is the machine-readable
// form (see format_full_json.go). This file is the human form. A
// user who runs `forge version --verbose` from a terminal wants to
// read the output; a user who runs it from a script wants
// `--format json`. Keeping the two separate means neither is
// compromised by the other's requirements.
//
// # Determinism
//
// WriteVerbose is deterministic: two calls with the same Metadata
// and Provenance produce byte-identical output. Dependencies and
// Settings are sorted before printing, so the runtime's ordering
// does not leak into the output. This matters because the output is
// compared in tests and may be captured in bug reports.
//
// # No global state
//
// WriteVerbose reads its two arguments and writes to w. It does not
// touch the filesystem, the network, the environment, or any
// package-level variable. It is safe for concurrent use with the
// same arguments.
//
// See docs/reference/version-output.md §"Verbose format".
package version

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// WriteVerbose writes the verbose human-readable version output to w.
//
// The output has up to four sections separated by blank lines:
//
//  1. The five linker fields: version, commit, buildDate, dirty,
//     buildType.
//  2. The provenance summary: go, module, module version (when it is
//     not the sentinel "(devel)"), vcs revision, vcs time (when
//     present), vcs dirty.
//  3. The dependencies, one per line, sorted by module path.
//  4. The build settings, one per line, sorted by key.
//
// Sections 3 and 4 are omitted when their lists are empty. Section 2
// is always present: the go and module lines are printed even when
// the values are empty, rendered as "-".
//
// # Errors
//
// Errors from w are returned. A writer that fails midway leaves
// partial output; the caller decides whether to print a further
// message. WriteVerbose does not retry.
//
// # Format
//
// Each line is "<label>:<padding> <value>\n". The label column is
// right-padded to a fixed width so values align. The dependency and
// setting sections are indented by two spaces so they are visually
// distinct from the top-level fields.
func WriteVerbose(w io.Writer, m Metadata, p Provenance) error {
	var b strings.Builder

	writeMetadataSection(&b, m)
	b.WriteByte('\n')
	writeProvenanceSection(&b, p)
	if len(p.Dependencies) > 0 {
		b.WriteByte('\n')
		writeDependenciesSection(&b, p.Dependencies)
	}
	if len(p.Settings) > 0 {
		b.WriteByte('\n')
		writeSettingsSection(&b, p.Settings)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// labelWidth is the width of the label column in the top-level
// fields. It is wide enough for the longest label ("buildDate:") plus
// one space. A longer label in a future field widens this constant.
const labelWidth = 11

// writeMetadataSection writes the five linker fields.
//
// The fields are printed in the same order as Metadata's struct
// fields and as the JSON schema: version, commit, buildDate, dirty,
// buildType. The order is part of the human format's stability: a
// user who greps for "version:" finds it on the first line.
func writeMetadataSection(b *strings.Builder, m Metadata) {
	writeField(b, "version", m.Version)
	writeField(b, "commit", m.Commit)
	writeField(b, "buildDate", m.BuildDate)
	writeField(b, "dirty", fmt.Sprintf("%t", m.Dirty))
	writeField(b, "buildType", m.BuildType)
}

// writeProvenanceSection writes the provenance summary.
//
// The module version is printed only when it is meaningful. The Go
// toolchain writes "(devel)" for a binary built from a working tree;
// the value carries no information a reader does not already have
// from the version field. Printing it anyway would add noise to
// every local build's output.
//
// The vcsTime line is printed only when the time is non-zero. A
// binary built without -buildvcs=true has no time; printing
// "vcsTime: -" would suggest the setting was present but empty.
func writeProvenanceSection(b *strings.Builder, p Provenance) {
	writeField(b, "go", orDash(p.GoVersion))
	writeField(b, "module", orDash(p.Module))
	if p.ModuleVersion != "" && p.ModuleVersion != "(devel)" {
		writeField(b, "moduleVersion", p.ModuleVersion)
	}
	writeField(b, "vcs", orDash(p.VCSRevision))
	if !p.VCSTime.IsZero() {
		writeField(b, "vcsTime", p.VCSTime.UTC().Format("2006-01-02T15:04:05Z"))
	}
	writeField(b, "vcsDirty", fmt.Sprintf("%t", p.VCSModified))
}

// writeDependenciesSection writes the dependency list.
//
// The list is sorted by module path. The runtime returns
// dependencies in a deterministic but unspecified order; sorting
// makes the output stable across calls and across Go versions.
//
// Each line is "  <path> <version>[ => <replace>][ (indirect)]". The
// two-space indent distinguishes the section's entries from the
// top-level fields.
func writeDependenciesSection(b *strings.Builder, deps []Dependency) {
	b.WriteString("dependencies:\n")
	sorted := append([]Dependency(nil), deps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, d := range sorted {
		b.WriteString("  ")
		b.WriteString(d.Path)
		if d.Version != "" {
			b.WriteByte(' ')
			b.WriteString(d.Version)
		}
		if d.Replace != "" {
			b.WriteString(" => ")
			b.WriteString(d.Replace)
		}
		if d.Indirect {
			b.WriteString(" (indirect)")
		}
		b.WriteByte('\n')
	}
}

// writeSettingsSection writes the build settings.
//
// The list is sorted by key. The runtime returns settings in a
// deterministic order, but the order is not documented; sorting
// makes the output stable.
//
// Each line is "  <key>=<value>". The format matches `go version -m`
// so a user who is used to one recognizes the other.
func writeSettingsSection(b *strings.Builder, settings []Setting) {
	b.WriteString("settings:\n")
	sorted := append([]Setting(nil), settings...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
	for _, s := range sorted {
		b.WriteString("  ")
		b.WriteString(s.Key)
		b.WriteByte('=')
		b.WriteString(s.Value)
		b.WriteByte('\n')
	}
}

// writeField writes one top-level field line: "<label>: <value>\n".
//
// The label is right-padded to labelWidth so the values align. The
// padding is spaces, not tabs: tabs render differently in different
// terminals and would break the alignment in a captured log.
func writeField(b *strings.Builder, label, value string) {
	b.WriteString(label)
	b.WriteByte(':')
	for i := len(label) + 1; i < labelWidth; i++ {
		b.WriteByte(' ')
	}
	b.WriteString(value)
	b.WriteByte('\n')
}

// orDash returns s, or "-" if s is empty. It is used so a blank line
// in the verbose output is unambiguous: a blank value means "not
// present", not "the tool refused to print".
//
// The dash is the same convention used by `git status -s` for an
// absent value and by many Unix tools for a missing field. It is
// short, unambiguous, and does not collide with any value a user
// would supply.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
