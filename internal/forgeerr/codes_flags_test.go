// Package forgeerr — test flag registration.
//
// The -update flag is used by the golden-file tests in
// codes_test.go. It is registered here, once, so that the
// registry test file does not depend on flag-registration order.
//
// The flag defaults to false. `go test -run TestAC4 -update
// ./internal/forgeerr/` sets it to true and regenerates the
// golden file.

package forgeerr

import "flag"

func init() {
	update = flag.Bool("update", false, "regenerate golden files")
}
