package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/config"
)

// TestZZDiagnostic_IsolateBreak reports whether the injection loss
// is in the helper or in the hook. Delete this file once the seam
// works.
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
		opts := options{
			args:   []string{"config", "--path"},
			env:    func(string) string { return "" },
			config: cfg, // set directly
		}
		// Replace stdout/stderr with buffers so we can inspect
		// the result.
		var stdout, stderr strings.Builder
		opts.stdout = &stdout
		opts.stderr = &stderr
		opts.stdin = strings.NewReader("")

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
