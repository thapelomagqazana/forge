// format_json.go writes the JSON form of the version output.
//
// Two shapes:
//
//	WriteJSON      the five linker fields
//	WriteFullJSON  the five linker fields plus provenance
//
// WriteJSON is the shape emitted by `forge version --format json`.
// WriteFullJSON is the shape emitted by
// `forge version --format json --verbose`.
//
// The schema in schemas/version.schema.json validates WriteJSON's
// output. schemas/version-full.schema.json validates WriteFullJSON.
// Both are exercised by `task version:verify`.
//
// # Why two shapes and not one
//
// The five linker fields are a stable contract with a small surface;
// a user's script that parses them relies on nothing else. The
// provenance adds the Go toolchain version, the module graph, and
// the VCS revision; its shape is larger and may grow. Keeping them
// separate means a change to the provenance does not invalidate the
// metadata schema, and a caller that only needs the five fields does
// not pay for the rest.
//
// # Why not MarshalJSON on Metadata
//
// Implementing MarshalJSON on Metadata would couple the type to its
// JSON representation. The type would then need to know about the
// encoder, the encoder's version, and the schema. Keeping the writer
// a free function (WriteJSON) keeps Metadata a pure value type.
//
// # HTML escaping
//
// Both writers disable encoding/json's default HTML escaping, so
// `<`, `>`, and `&` in a version string survive verbatim. The
// version string is quoted by the encoder; the three characters are
// not escaped because they have no special meaning in a JSON string.
// A consumer that renders the string in HTML is responsible for its
// own escaping, which it must do anyway.
//
// See docs/reference/version-output.md §"JSON format" and §"JSON
// verbose format".
package version

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// FullMetadata is the JSON shape emitted by
// `forge version --format json --verbose`.
//
// It embeds Metadata so the five linker fields appear at the top
// level, then adds a "provenance" field. The embedding makes
// FullMetadata a superset of Metadata from a consumer's perspective:
// a script that reads the five fields from the simple shape reads
// the same five fields from the full shape.
//
// # Field order
//
// encoding/json emits embedded struct fields first, in the order the
// embedded type declares them, then the outer struct's own fields.
// The JSON field order is therefore:
//
//	version, commit, buildDate, dirty, buildType, provenance
//
// This order is observable and is frozen by
// schemas/version-full.schema.json. Changing it requires a new
// schema.
type FullMetadata struct {
	Metadata
	// Provenance carries the Go toolchain version, the module graph,
	// and the VCS revision. It is always present in the full shape,
	// even when every field is empty: a consumer that expects the
	// key finds it.
	Provenance ProvenanceJSON `json:"provenance"`
}

// ProvenanceJSON is the JSON representation of a Provenance.
//
// # Differences from Provenance
//
//   - VCSTime is a string, not a time.Time. The schema validates it
//     as an RFC 3339 date-time. A zero time is serialized as the
//     empty string, which omits the field via omitempty.
//
//   - Dependencies and Settings are the same structs as in
//     Provenance, because they are already JSON-serializable.
//
// # omitempty
//
// ModuleVersion, VCSRevision, VCSTime, Dependencies, and Settings
// use omitempty. The fields are absent rather than empty when they
// carry no information. A consumer that checks for the key's
// presence sees the difference; a consumer that reads the field
// with a default value sees the same zero value either way.
//
// GoVersion, Module, and VCSModified do not use omitempty. GoVersion
// and Module are always populated for a binary built with module
// support; VCSModified is a boolean whose "false" is meaningful
// (the tree was clean) and must not be conflated with "absent".
type ProvenanceJSON struct {
	GoVersion     string       `json:"goVersion"`
	Module        string       `json:"module"`
	ModuleVersion string       `json:"moduleVersion,omitempty"`
	VCSRevision   string       `json:"vcsRevision,omitempty"`
	VCSTime       string       `json:"vcsTime,omitempty"`
	VCSModified   bool         `json:"vcsModified"`
	Dependencies  []Dependency `json:"dependencies,omitempty"`
	Settings      []Setting    `json:"settings,omitempty"`
}

// ProvenanceToJSON converts a Provenance to its JSON representation.
//
// The conversion is pure: it allocates a new value and does not
// mutate the input. The Dependencies and Settings slices are copied
// so a caller that retains the result does not share state with the
// input. The copy also ensures the JSON output cannot be corrupted
// by a subsequent mutation of the input slices.
//
// A zero VCSTime becomes the empty string, which the struct's
// omitempty tag turns into an absent field.
func ProvenanceToJSON(p Provenance) ProvenanceJSON {
	out := ProvenanceJSON{
		GoVersion:     p.GoVersion,
		Module:        p.Module,
		ModuleVersion: p.ModuleVersion,
		VCSRevision:   p.VCSRevision,
		VCSModified:   p.VCSModified,
	}
	if !p.VCSTime.IsZero() {
		out.VCSTime = p.VCSTime.UTC().Format(time.RFC3339)
	}
	if len(p.Dependencies) > 0 {
		out.Dependencies = make([]Dependency, len(p.Dependencies))
		copy(out.Dependencies, p.Dependencies)
	}
	if len(p.Settings) > 0 {
		out.Settings = make([]Setting, len(p.Settings))
		copy(out.Settings, p.Settings)
	}
	return out
}

// WriteFullJSON writes m and p as a single-line JSON object to w.
//
// The output ends with a newline. HTML escaping is disabled, matching
// WriteJSON.
//
// # Errors
//
// Same policy as WriteJSON: an encoding error is wrapped with a
// `version: encode full metadata:` prefix; a write error is returned
// by the encoder as-is.
//
// # Idempotence
//
// Two calls with the same m and p produce byte-identical output. The
// function does not sort the Dependencies or Settings slices; the
// order is whatever the caller supplied. The provenance reader
// returns a deterministic order for a given binary, so the output is
// stable in practice. A caller that wants a canonical order sorts
// the slices before calling.
func WriteFullJSON(w io.Writer, m Metadata, p Provenance) error {
	full := FullMetadata{
		Metadata:   m,
		Provenance: ProvenanceToJSON(p),
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(full); err != nil {
		return fmt.Errorf("version: encode full metadata: %w", err)
	}
	return nil
}
