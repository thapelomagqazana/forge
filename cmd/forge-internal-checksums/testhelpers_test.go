package main

import (
	"testing"

	"github.com/thapelomagqazana/forge/internal/platform"
)

// platformForTest constructs a platform.Platform for a given
// goos/goarch pair. It is used by tests that need a Platform value
// without loading the canonical matrix.
//
// The Platform struct's fields are populated from the matrix at
// load time; a test that constructs one by hand must set every
// field the code under test reads. This helper sets the fields the
// checksums driver reads: Name, GOOS, GOARCH, and Tier. A future
// field that the driver reads must be added here or the test will
// compile and then fail at run time with a zero value.
func platformForTest(goos, goarch string) platform.Platform {
	return platform.Platform{
		Name:   goos + "/" + goarch,
		GOOS:   goos,
		GOARCH: goarch,
		Tier:   platform.Tier1,
	}
}

// guard against the helper being unused after a refactor.
var _ = testing.Short
