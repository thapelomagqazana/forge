// metadata.go defines the structured version-metadata document Forge
// emits from `forge version --format=json`.
//
// The shape is frozen by docs/reference/version-output.md and by
// schemas/version.schema.json. It is a public contract: a user's
// script may parse it. Any change requires a MAJOR version bump.
package version

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Metadata is the structured version document.
//
// Field order in the struct mirrors the JSON field order in the
// schema: version, commit, buildDate, dirty, buildType.
// encoding/json emits fields in struct order, so this order is
// observable and MUST NOT be changed without a schema change.
type Metadata struct {
	// Version is the version string, e.g. "v1.4.2" or "dev".
	Version string `json:"version"`
	// Commit is the short commit SHA, e.g. "abc1234" or "none".
	Commit string `json:"commit"`
	// BuildDate is the UTC RFC 3339 build timestamp, e.g.
	// "2026-06-01T12:00:00Z".
	BuildDate string `json:"buildDate"`
	// Dirty is true if the build was produced from a working tree
	// with uncommitted changes.
	Dirty bool `json:"dirty"`
	// BuildType is one of "release", "ci", "local".
	BuildType string `json:"buildType"`
}

// BuildKind names the environment a binary was built in.
//
// The name is BuildKind, not BuildType, because the linker variable
// in version.go is named BuildType. A type and a variable cannot share
// an identifier in the same package; the variable keeps its name
// because it is the injection target and the name is frozen by
// Taskfile.yml, scripts/build.go, and docs/reference/versioning.md.
type BuildKind string

const (
	// BuildKindRelease is set by the release pipeline.
	BuildKindRelease BuildKind = "release"
	// BuildKindCI is set by non-release CI.
	BuildKindCI BuildKind = "ci"
	// BuildKindLocal is the default for local builds.
	BuildKindLocal BuildKind = "local"
)

// Valid reports whether k is one of the three defined build kinds.
func (k BuildKind) Valid() bool {
	switch k {
	case BuildKindRelease, BuildKindCI, BuildKindLocal:
		return true
	default:
		return false
	}
}

// String returns the build kind as a string.
func (k BuildKind) String() string { return string(k) }

// ParseBuildKind parses a build kind from a string.
//
// An empty string becomes BuildKindLocal: a binary that carries no
// build-kind information was almost certainly built outside CI.
//
// An unrecognized non-empty value returns an error. The caller
// decides whether to treat the error as fatal or to normalize to
// BuildKindLocal; MetadataOf normalizes.
func ParseBuildKind(s string) (BuildKind, error) {
	switch s {
	case "":
		return BuildKindLocal, nil
	case string(BuildKindRelease):
		return BuildKindRelease, nil
	case string(BuildKindCI):
		return BuildKindCI, nil
	case string(BuildKindLocal):
		return BuildKindLocal, nil
	default:
		return "", fmt.Errorf("version: unknown build kind %q", s)
	}
}

// MetadataOf constructs a Metadata from the four linker-injected
// values plus the parsed dirty flag.
//
// The build kind is normalized: an empty or unrecognized value
// becomes BuildKindLocal, so a binary built outside CI always
// reports a valid kind. The version string is used verbatim: it is
// whatever the linker embedded and is not normalized, because a
// dirty tree ("v1.4.2-dirty"), a commit offset
// ("v1.4.2-3-gabc1234"), and the literal "dev" are all meaningful.
//
// The dirty argument is the linker's raw string ("true", "false", or
// ""). Only the exact string "true" yields true; everything else
// yields false. A binary that does not know it is dirty is safer
// reported as clean than as dirty.
func MetadataOf(versionStr, commit, buildDate, dirty, buildKind string) Metadata {
	k, err := ParseBuildKind(buildKind)
	if err != nil {
		k = BuildKindLocal
	}
	return Metadata{
		Version:   versionStr,
		Commit:    commit,
		BuildDate: buildDate,
		Dirty:     parseDirty(dirty),
		BuildType: string(k),
	}
}

// parseDirty converts the linker's string representation of a
// boolean to a Go bool.
func parseDirty(s string) bool {
	return s == "true"
}

// WriteJSON writes m as a single-line JSON object to w.
//
// It uses encoding/json with HTML escaping disabled, so `<`, `>`, and
// `&` in a version string (unusual but permitted) survive verbatim.
// The output ends with a newline, matching the convention used by the
// rest of the CLI's JSON writers.
func WriteJSON(w io.Writer, m Metadata) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("version: encode metadata: %w", err)
	}
	return nil
}

// Validate checks that m satisfies the invariants the schema enforces.
//
// The zero value is not valid: version, commit, and buildDate are
// required. This is the check the release pipeline runs before
// publishing an artifact.
//
// buildDate MUST be an RFC 3339 timestamp in UTC (a trailing Z, not a
// numeric offset). A build from a host in a non-UTC timezone is a
// defect: two builds of the same commit on different hosts would
// produce different embedded dates, which violates the reproducibility
// contract in docs/reference/versioning.md §6.
func (m Metadata) Validate() error {
	if m.Version == "" {
		return fmt.Errorf("version: version is empty")
	}
	if m.Commit == "" {
		return fmt.Errorf("version: commit is empty")
	}
	if m.BuildDate == "" {
		return fmt.Errorf("version: buildDate is empty")
	}

	// Parse as RFC 3339 first, so a malformed timestamp gets a parse
	// error rather than the weaker "not UTC" error.
	t, err := time.Parse(time.RFC3339, m.BuildDate)
	if err != nil {
		return fmt.Errorf("version: buildDate %q is not RFC 3339: %w", m.BuildDate, err)
	}
	// Require UTC. time.Parse returns a time in the offset's location;
	// only a literal "Z" (or a "+00:00"/"-00:00" that the parser maps to
	// UTC) yields time.UTC. A "+02:00" offset yields a fixed-zone
	// location, and the check below catches it.
	//
	// The contract permits the fractional-seconds form
	// ("2026-06-01T12:00:00.000Z"); time.Parse handles it and still
	// returns UTC for a trailing Z.
	_, offset := t.Zone()
	if offset != 0 || t.Location() != time.UTC {
		return fmt.Errorf("version: buildDate %q is not UTC (offset %ds)", m.BuildDate, offset)
	}

	if !BuildKind(m.BuildType).Valid() {
		return fmt.Errorf("version: buildType %q is not release|ci|local", m.BuildType)
	}
	return nil
}
