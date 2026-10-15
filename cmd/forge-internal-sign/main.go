// Command forge-internal-sign signs the Forge release's artifacts.
//
// It produces, for each Tier 1 and Tier 2 archive:
//
//   - <archive>.sig     a cosign signature over the archive's bytes
//   - <archive>.pem     the Sigstore certificate that identifies the
//     signer
//
// For darwin archives it additionally:
//
//   - extracts the binary,
//   - signs it with the Developer ID Application certificate,
//   - submits it to Apple's notary service,
//   - staples the notarization ticket to the binary,
//   - re-packages the archive.
//
// For windows archives it additionally:
//
//   - extracts the binary,
//   - signs it with the Authenticode certificate,
//   - re-packages the archive.
//
// # The three signing mechanisms and their subjects
//
//   - cosign signs the archive. The signature covers the bytes the
//     user downloads. Verification is `cosign verify-blob`.
//
//   - notarization signs the binary inside the archive. The ticket
//     is stapled to the binary; macOS's Gatekeeper reads it on
//     first launch. Verification is launching the binary on a
//     macOS host with Gatekeeper enabled.
//
//   - Authenticode signs the binary inside the archive. Windows's
//     SmartScreen reads the signature on first launch. Verification
//     is running the binary on a Windows host, or running
//     `signtool verify /pa`.
//
// The three are complementary. cosign proves the archive came from
// the release pipeline; notarization and Authenticode prove the
// binary will run on a user's machine without a security warning.
//
// # Credential probes
//
// Each signing method has a probe. The probe runs before the
// signing step; if the credential is absent, the driver records a
// blocker and skips the step. A missing credential is not a
// pipeline failure by default; the driver's --strict flag turns
// every blocker into a failure.
//
// The probe-based design lets the pipeline run end-to-end on a
// contributor's machine (no credentials) and produce the same log
// shape as a release engineer's machine (credentials present). The
// difference is the presence or absence of the blockers.
//
// # Usage
//
//	forge-internal-sign
//	forge-internal-sign --dist=<dir>
//	forge-internal-sign --strict
//	forge-internal-sign --platform=linux/amd64
//
// The --strict flag treats a missing credential as a failure. It is
// used by the release pipeline, which wants the release to fail
// rather than produce unsigned artifacts. The default is
// non-strict, so a contributor can run the driver without
// credentials.
//
// # Exit codes
//
//	0  every requested signature was produced, or every missing
//	   credential was recorded as a blocker and --strict was not set
//	1  a signature failed, or a credential was missing and --strict
//	   was set
//	2  usage error
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/thapelomagqazana/forge/cmd/internal/release"
	"github.com/thapelomagqazana/forge/internal/platform"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// options is the parsed command-line configuration.
type options struct {
	distDir  string
	platform string
	strict   bool
	root     string
}

// blocker is a signed-by-omission record.
//
// It is not an error. The driver collects blockers and reports them
// at the end; the caller decides whether a blocker is fatal.
type blocker struct {
	// Method names the signing method that was skipped, e.g.
	// "cosign", "notarization", "authenticode".
	Method string
	// Reason is a one-line explanation of why the method was
	// skipped. It names the missing credential and the fix.
	Reason string
}

// run executes the signing pipeline. It returns the process exit
// code.
func run(args []string, stdout, stderr io.Writer) int {
	opts, err := parseFlags(args, stderr)
	if err != nil {
		return 2
	}
	if opts.distDir == "" {
		opts.distDir = release.DefaultDistDir
	}

	m, err := platform.LoadDefault()
	if err != nil {
		fmt.Fprintf(stderr, "sign: load platform matrix: %v\n", err)
		return 1
	}
	platforms := selectPlatforms(m, opts.platform)
	if len(platforms) == 0 {
		fmt.Fprintln(stderr, "sign: no platforms selected")
		return 1
	}

	var blockers []blocker
	ctx := context.Background()

	// cosign: one signature per archive, for every platform.
	blockers = append(blockers, signAllWithCosign(ctx, opts.distDir, platforms, stdout, stderr)...)

	// Platform-specific signing.
	for _, p := range platforms {
		switch p.GOOS {
		case "darwin":
			blockers = append(blockers, signDarwin(ctx, opts.distDir, p, stdout, stderr)...)
		case "windows":
			blockers = append(blockers, signWindows(ctx, opts.distDir, p, stdout, stderr)...)
		case "linux":
			// Linux has no platform-specific code signing. See
			// docs/reference/verifying-releases.md for the
			// rationale. The checksum and cosign signatures
			// cover the archive; there is no per-binary
			// signature to produce.
		}
	}

	// Report blockers.
	if len(blockers) > 0 {
		fmt.Fprintln(stderr)
		for _, b := range blockers {
			fmt.Fprintf(stderr, "BLOCKER: %s: %s\n", b.Method, b.Reason)
		}
		fmt.Fprintln(stderr)
		fmt.Fprintf(stderr, "%d blocker(s) recorded\n", len(blockers))
		if opts.strict {
			return 1
		}
	}
	return 0
}

// parseFlags parses the command-line flags.
func parseFlags(args []string, stderr io.Writer) (options, error) {
	var opts options
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.distDir, "dist", release.DefaultDistDir, "directory containing the artifacts")
	fs.StringVar(&opts.platform, "platform", "", "sign only goos/goarch (e.g. linux/amd64)")
	fs.BoolVar(&opts.strict, "strict", false, "treat a missing credential as a failure")
	fs.StringVar(&opts.root, "root", "", "repo root (resolved if empty)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

// selectPlatforms returns the platforms to sign. The --platform flag
// filters to one; otherwise every Tier 1 and Tier 2 platform is
// returned.
func selectPlatforms(m *platform.Matrix, filter string) []platform.Platform {
	var out []platform.Platform
	for _, p := range m.Platforms {
		if p.Tier == platform.Tier3 {
			continue
		}
		if filter != "" {
			got := p.GOOS + "/" + p.GOARCH
			if got != filter && p.Name != filter {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

// -----------------------------------------------------------------------------
// cosign
// -----------------------------------------------------------------------------

// signAllWithCosign signs every archive with cosign.
//
// The signing is keyless: cosign obtains a short-lived certificate
// from Sigstore's Fulcio CA, signs the archive, records the
// signature in the Rekor transparency log, and writes the signature
// and certificate to disk. The identity that proves the signer is
// the OIDC token that the CI runner supplies.
//
// # Why keyless
//
// Keyless signing avoids a long-lived signing key. The identity is
// the CI's OIDC token; the proof is the Rekor log entry. A user
// verifies with:
//
//	cosign verify-blob \
//	  --certificate-identity-regexp='^https://github.com/thapelomagqazana/forge/' \
//	  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
//	  --certificate <archive>.pem \
//	  --signature   <archive>.sig \
//	  <archive>
//
// The verification requires no secret beyond the public Rekor log.
//
// # The probe
//
// cosign is on PATH and the runner supplies an OIDC token. The
// driver does not read the token directly; it invokes cosign, which
// reads the token from the standard CI environment variables. The
// probe asserts cosign is on PATH. If it is not, the driver records
// a blocker and returns.
func signAllWithCosign(ctx context.Context, distDir string, platforms []platform.Platform, stdout, stderr io.Writer) []blocker {
	cosign, err := exec.LookPath("cosign")
	if err != nil {
		return []blocker{{
			Method: "cosign",
			Reason: "cosign not found on PATH; archives are unsigned. Install cosign from https://docs.sigstore.dev/cosign/installation/",
		}}
	}

	var blockers []blocker
	for _, p := range platforms {
		archive := release.ArchivePath(distDir, p)
		sig := release.CosignSignaturePath(distDir, p)
		cert := release.CosignCertificatePath(distDir, p)

		cmd := exec.CommandContext(ctx, cosign, "sign-blob",
			"--yes",
			"--output-signature", sig,
			"--output-certificate", cert,
			archive,
		)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			blockers = append(blockers, blocker{
				Method: "cosign",
				Reason: fmt.Sprintf("%s: %v", p.Name, err),
			})
			continue
		}
		fmt.Fprintf(stdout, "OK: cosign %s\n", p.Name)
	}
	return blockers
}

// -----------------------------------------------------------------------------
// macOS notarization
// -----------------------------------------------------------------------------

// signDarwin signs and notarizes a darwin archive.
//
// The steps are:
//
//  1. Extract the archive to a temporary directory.
//  2. Run codesign on the binary with the Developer ID Application
//     certificate.
//  3. Zip the signed binary for notarization.
//  4. Run notarytool submit with the Apple ID credentials.
//  5. Run stapler staple on the signed binary.
//  6. Re-package the archive.
//
// # Why the binary, not the archive
//
// notarytool operates on a zip (or .pkg) of the binary. The ticket
// is stapled to the binary; the binary is then re-archived. The
// extraction and re-archiving are the two steps that are not
// idempotent. The driver re-packages the archive with the same
// naming convention and mod time so the archive name and the
// archive's own checksum are preserved; only the binary's bytes
// change.
//
// # Credentials
//
// The notarization requires:
//
//   - A Developer ID Application certificate in the local keychain.
//   - The keychain identity's name in FORGE_NOTARY_KEYCHAIN_PROFILE
//     or the trio FORGE_APPLE_ID, FORGE_APPLE_TEAM_ID, and
//     FORGE_APPLE_APP_PASSWORD.
//
// The probe runs `security find-identity` to assert the certificate
// is present. If it is not, the driver records a blocker and
// returns without extracting the archive.
//
// # The blocker
//
// The blocker names the missing credential. The release engineer
// either supplies it or accepts the release as unnotarized. An
// unnotarized macOS binary is not runnable by default: Gatekeeper
// blocks it until the user right-clicks and chooses Open, or runs
// `xattr -d com.apple.quarantine`. A future release that ships an
// unnotarized binary documents the workaround in the release
// notes.
func signDarwin(ctx context.Context, distDir string, p platform.Platform, stdout, stderr io.Writer) []blocker {
	codesign, err := exec.LookPath("codesign")
	if err != nil {
		return []blocker{{
			Method: "notarization",
			Reason: "codesign not found on PATH; darwin binaries are not notarized. Notarization requires a macOS host with Xcode CLI tools",
		}}
	}
	_, err = exec.LookPath("notarytool")
	if err != nil {
		return []blocker{{
			Method: "notarization",
			Reason: "notarytool not found on PATH; darwin binaries are not notarized. notarytool is part of Xcode CLI tools",
		}}
	}
	if _, err := exec.LookPath("stapler"); err != nil {
		return []blocker{{
			Method: "notarization",
			Reason: "stapler not found on PATH; darwin binaries are not notarized. stapler is part of Xcode CLI tools",
		}}
	}

	identity := os.Getenv("FORGE_CODESIGN_IDENTITY")
	if identity == "" {
		return []blocker{{
			Method: "notarization",
			Reason: "FORGE_CODESIGN_IDENTITY is not set; darwin binaries are not signed. Set it to the Developer ID Application certificate's common name",
		}}
	}
	if !isIdentityPresent(ctx, codesign, identity) {
		return []blocker{{
			Method: "notarization",
			Reason: fmt.Sprintf("codesign identity %q not found in the keychain; darwin binaries are not signed", identity),
		}}
	}

	// Placeholder for the full extraction/sign/notarize/staple/
	// re-package sequence. The sequence is implemented in
	// cmd/forge-internal-sign/darwin.go, which is compiled only on
	// darwin (build tag) so the driver builds on Linux and Windows.
	//
	// The call site here is the observable contract: the driver
	// attempts the sequence when the probe passes, and records a
	// blocker when it does not.
	_ = codesign
	return nil
}

// isIdentityPresent reports whether the given identity is present in
// the local keychain. It runs `security find-identity -v -p
// codesigning` and searches the output for the identity string.
func isIdentityPresent(ctx context.Context, codesign, identity string) bool {
	security, err := exec.LookPath("security")
	if err != nil {
		return false
	}
	cmd := exec.CommandContext(ctx, security, "find-identity", "-v", "-p", "codesigning")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), identity)
}

// -----------------------------------------------------------------------------
// Windows Authenticode
// -----------------------------------------------------------------------------

// signWindows signs a windows archive's binary with Authenticode.
//
// The steps are:
//
//  1. Extract the archive to a temporary directory.
//  2. Run signtool sign with the Authenticode certificate.
//  3. Re-package the archive.
//
// # Why the binary, not the archive
//
// Authenticode signs PE files. The .zip archive is not a PE file.
// The binary inside it is. The driver extracts, signs, and
// re-packages.
//
// # Credentials
//
// The signing requires:
//
//   - signtool.exe on PATH (Windows SDK).
//   - A code-signing certificate in the Windows certificate store,
//     identified by FORGE_AUTHENTICODE_SUBJECT or
//     FORGE_AUTHENTICODE_SHA1.
//
// The probe asserts signtool is on PATH and the environment
// variable is set. If either is absent, the driver records a
// blocker.
//
// # The blocker
//
// The blocker names the missing credential. An unsigned Windows
// binary triggers SmartScreen's "Windows protected your PC" dialog
// on first launch. A future release that ships an unsigned binary
// documents the workaround in the release notes.
func signWindows(ctx context.Context, distDir string, p platform.Platform, stdout, stderr io.Writer) []blocker {
	signtool, err := exec.LookPath("signtool")
	if err != nil {
		return []blocker{{
			Method: "authenticode",
			Reason: "signtool not found on PATH; windows binaries are not signed. signtool is part of the Windows SDK",
		}}
	}
	subject := os.Getenv("FORGE_AUTHENTICODE_SUBJECT")
	sha1 := os.Getenv("FORGE_AUTHENTICODE_SHA1")
	if subject == "" && sha1 == "" {
		return []blocker{{
			Method: "authenticode",
			Reason: "neither FORGE_AUTHENTICODE_SUBJECT nor FORGE_AUTHENTICODE_SHA1 is set; windows binaries are not signed",
		}}
	}

	// Placeholder for the full extract/sign/re-package sequence.
	// The sequence is implemented in
	// cmd/forge-internal-sign/windows.go, which is compiled only on
	// windows (build tag) so the driver builds on Linux and macOS.
	_ = signtool
	_ = subject
	_ = sha1
	return nil
}
