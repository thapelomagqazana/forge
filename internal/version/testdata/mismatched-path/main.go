// Command mismatched-path demonstrates the silent-failure trap
// documented in WBS 6.1.2 and docs/development.md.
//
// The module path is "example.com/wrong", but the linker flag passed
// by the test points at "example.com/right/internal/version". The -X
// flag silently does nothing: no error, no warning, empty values.
//
// Do not add this module to the repository's go.work. It is a
// fixture, not a real package.
package main

import (
	"fmt"

	"example.com/wrong/internal/version"
)

func main() {
	v, c, b, d := version.Get()
	fmt.Printf("%q %q %q %q\n", v, c, b, d)
}
