package main

import "github.com/thapelomagqazana/forge/internal/platform"

// platformForTest constructs a platform.Platform for a given
// goos/goarch pair. See the checksums driver's copy for the
// rationale; the helper is duplicated per package because Go does
// not share test helpers across packages without exporting them
// from a non-test file, which would leak test code into the
// production binary.
func platformForTest(goos, goarch string) platform.Platform {
	return platform.Platform{
		Name:   goos + "/" + goarch,
		GOOS:   goos,
		GOARCH: goarch,
		Tier:   platform.Tier1,
	}
}
