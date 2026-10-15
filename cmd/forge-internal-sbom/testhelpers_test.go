package main

import "github.com/thapelomagqazana/forge/internal/platform"

// platformForTest constructs a platform.Platform for a given
// goos/goarch pair. See the checksums driver's copy for the
// rationale.
func platformForTest(goos, goarch string) platform.Platform {
	return platform.Platform{
		Name:   goos + "/" + goarch,
		GOOS:   goos,
		GOARCH: goarch,
		Tier:   platform.Tier1,
	}
}
