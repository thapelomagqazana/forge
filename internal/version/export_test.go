package version

import "runtime/debug"

// ProvenanceForTest exposes provenanceFromBuildInfo to the external
// test package. It is compiled only for tests.
//
// The name has a ForTest suffix so it does not collide with any
// production identifier and so a reader knows immediately that the
// symbol is a test hook.
func ProvenanceForTest(info *debug.BuildInfo) Provenance {
	return provenanceFromBuildInfo(info)
}
