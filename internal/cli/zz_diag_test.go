package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/config"
	"github.com/thapelomagqazana/forge/internal/streams"
)

// TestZZDiagnostic_IsolateBreak reports whether the injection loss
// is in the helper or in the hook. Delete this file once the seam
// works.
//
// # Why the file is named zz_*
//
// The zz_ prefix sorts the file last in the package's test-file
// listing, so a reader who scans the test directory sees it after
// the substantive tests. The name is a convention the maintainers
// use for diagnostic-only tests that are expected to be deleted
// after they have served their purpose.
//
// # Why the file uses explicit streams
//
// The diagnostic constructs an options value by hand to bypass the
// runCLI helper and isolate the suspected break. The options struct
// carries the three standard streams in a single streams.Streams
// value (WBS 12.4.1), so the diagnostic builds one and assigns it
// to the field.
//
// The two strings.Builder values are captured by the diagnostic
// and read after runWithOptions returns. A strings.Builder is used
// instead of a bytes.Buffer because the diagnostic only reads the
// contents, and strings.Builder has a cheaper Write path for that
// use. The choice is not load-bearing; either works.
func TestZZDiagnostic_IsolateBreak(t *testing.T) {
	const marker = "/ISOLATE-MARKER.yaml"
	cfg := &config.Config{SourcePath: marker}

	// Path 1: does the helper pass cfg to opts.config?
	//
	// We check this by calling runCLIWithConfig and observing the
	// result. If the result contains the marker, the helper
	// passes; if not, the loss is before the dispatcher.
	t.Run("helper", func(t *testing.T) {
		got := runCLIWithConfig(t, []string{"config", "--path"}, nil, cfg)
		t.Logf("runCLIWithConfig stdout = %q", got.stdout)
		if got.stdout == marker+"\n" {
			t.Logf("HELPER OK: runCLIWithConfig passes cfg to opts.config")
		} else {
			t.Logf("HELPER SUSPECT: runCLIWithConfig produced %q, "+
				"not %q. The helper's opts literal may not "+
				"contain `config: cfg`.", got.stdout, marker+"\n")
		}
	})

	// Path 2: bypass runCLIWithConfig and construct options
	// manually, then call runWithOptions. This isolates the
	// hook from the helper.
	t.Run("manual", func(t *testing.T) {
		var stdout, stderr strings.Builder
		opts := options{
			args: []string{"config", "--path"},
			streams: streams.Streams{
				In:  strings.NewReader(""),
				Out: &stdout,
				Err: &stderr,
			},
			env:    func(string) string { return "" },
			config: cfg, // set directly
		}

		code, _ := runWithOptions(context.Background(), opts)
		t.Logf("runWithOptions: exit=%d stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())

		if stdout.String() == marker+"\n" {
			t.Logf("MANUAL OK: the injection works when options " +
				"is constructed manually. The break is in " +
				"runCLIWithConfig.")
		} else {
			t.Logf("MANUAL FAILED: the injection is lost even " +
				"with options constructed manually. The break " +
				"is in runWithOptions or the command tree " +
				"(most likely the PersistentPreRunE hook).")
		}
	})
}
