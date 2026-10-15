// Command forge-internal-distribute updates one distribution channel
// for a release.
//
// It is invoked by the release workflow's distribute job, once per
// channel. The job reads the channel's repository, applies the
// manifests, commits the result, and (for winget) opens a pull
// request.
//
// # Why one binary per channel would be worse
//
// Six binaries would be six build targets, six entries in the
// workflow, and six places a channel's config lives. One binary with
// a --channel flag is one build target, one workflow step, and one
// place the config lives. The flag selects the adapter; the adapter
// renders the manifests; the binary writes them.
//
// # What the binary does not do
//
// The binary does not commit, does not push, and does not open a
// pull request. It writes the manifests to a directory. The
// workflow's subsequent steps do the git operations. The split keeps
// the binary pure: a test runs it in a temp directory and inspects
// the output, without a git remote.
//
// # Usage
//
//	forge-internal-distribute --channel=homebrew --version=v1.4.2 --dist=dist --out=out
//	forge-internal-distribute --channel=scoop    --version=v1.4.2 --dist=dist --out=out
//	forge-internal-distribute --channel=winget   --version=v1.4.2 --dist=dist --out=out
//	forge-internal-distribute --channel=apt      --version=v1.4.2 --dist=dist --out=out
//	forge-internal-distribute --channel=docker   --version=v1.4.2 --dist=dist --out=out
//	forge-internal-distribute --channel=goinstall --version=v1.4.2 --dist=dist --out=out
//
// # Exit codes
//
//	0  the manifests were written, or the channel has no adapter
//	1  the adapter failed
//	2  usage error
//	3  the channel is not applicable to this release (a nightly to
//	   Homebrew, a prerelease to winget, a release missing a
//	   platform the channel needs)
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/thapelomagqazana/forge/internal/distribution"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command-line configuration.
type options struct {
	channel        string
	version        string
	releaseChannel string
	distDir        string
	outDir         string
	gpgFingerprint string
}

// run executes the driver. It returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}
	if !distribution.IsValidChannel(distribution.Channel(opts.channel)) {
		fmt.Fprintf(stderr, "distribute: unknown channel %q\n", opts.channel)
		return 2
	}

	rel, err := distribution.ParseRelease(
		opts.distDir, opts.version, opts.releaseChannel, opts.gpgFingerprint,
	)
	if err != nil {
		fmt.Fprintf(stderr, "distribute: parse release: %v\n", err)
		return 1
	}

	adapter := distribution.For(distribution.Channel(opts.channel))
	if adapter == nil {
		// ChannelDirect has no adapter. The release pipeline's
		// publish job produces the GitHub Release; the channel
		// is listed so the installation guide can name it, but
		// there is nothing for the driver to render.
		//
		// A future channel that also has no adapter would reach
		// this branch. The branch's message names the channel,
		// not the reason; a reader who needs the reason reads
		// the channel's documentation.
		fmt.Fprintf(stdout, "OK: channel %q has no adapter; nothing to render\n", opts.channel)
		return 0
	}

	manifests, err := adapter.Manifests(rel)
	if err != nil {
		if isNotApplicable(err) {
			fmt.Fprintf(stderr, "SKIP: %v\n", err)
			return 3
		}
		fmt.Fprintf(stderr, "distribute: %v\n", err)
		return 1
	}

	for _, m := range manifests {
		path := filepath.Join(opts.outDir, m.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(stderr, "distribute: mkdir %s: %v\n", filepath.Dir(path), err)
			return 1
		}
		if err := os.WriteFile(path, m.Content, os.FileMode(m.Mode)); err != nil {
			fmt.Fprintf(stderr, "distribute: write %s: %v\n", path, err)
			return 1
		}
		fmt.Fprintf(stdout, "OK: %s\n", path)
	}
	return 0
}

// parseFlags parses the command-line flags.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("distribute", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.channel, "channel", "", "the distribution channel to update")
	fs.StringVar(&opts.version, "version", "", "the release version (e.g. v1.4.2)")
	fs.StringVar(&opts.releaseChannel, "release-channel", "stable", "the release channel (stable, prerelease, nightly)")
	fs.StringVar(&opts.distDir, "dist", "dist", "the directory containing the release's artifacts")
	fs.StringVar(&opts.outDir, "out", "out", "the directory to write the channel's manifests to")
	fs.StringVar(&opts.gpgFingerprint, "gpg-fingerprint", "", "the release key's fingerprint (for apt)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if opts.channel == "" || opts.version == "" {
		return options{}, fmt.Errorf("--channel and --version are required")
	}
	return opts, nil
}

// isNotApplicable reports whether err wraps ErrNotApplicable or
// ErrUnsupportedPlatform. Both are skips, not failures: the channel
// is recorded as a blocker and the release continues.
//
// The function does not include ErrModulePathChanged. A wrong module
// path is a defect in the release, not a legitimate reason to omit
// the channel. The driver returns 1 for it, not 3.
func isNotApplicable(err error) bool {
	return errors.Is(err, distribution.ErrNotApplicable) ||
		errors.Is(err, distribution.ErrUnsupportedPlatform)
}
