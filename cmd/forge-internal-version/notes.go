package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/thapelomagqazana/forge/internal/version"
)

// cmdNotes validates a filled-in release notes file against the
// required headings for its channel and bump class.
//
// Usage:
//
//	forge-internal-version notes check <path>
//
// The file's channel is taken from the first line's "Channel:" field.
// Its bump class is inferred from the title's "vX.Y.Z" against the
// previous release's tag, which the caller must supply. If no
// previous tag is supplied, the check is permissive: it requires the
// sections the template marks as unconditionally required and treats
// "Breaking changes" as optional.
func cmdNotes(args []string, stdout, stderr *os.File) int {
	if len(args) < 2 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: forge-internal-version notes check <path> [--previous <tag>]")
		return 2
	}
	path := args[1]
	var previous string
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--previous":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "forge-internal-version: --previous requires a tag")
				return 2
			}
			previous = args[i+1]
			i++
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: read %s: %v\n", path, err)
		return 1
	}
	body := string(data)

	// 1. No template comments may remain.
	if strings.Contains(body, "<!--") {
		fmt.Fprintf(stderr, "forge-internal-version: %s still contains template comments\n", path)
		return 1
	}

	// 2. Title.
	titleRe := regexp.MustCompile(`(?m)^# Forge (v\d+\.\d+\.\d+(?:-[A-Za-z0-9.]+)?)\s*$`)
	titleMatch := titleRe.FindStringSubmatch(body)
	if titleMatch == nil {
		fmt.Fprintf(stderr, "forge-internal-version: %s has no '# Forge vX.Y.Z' title\n", path)
		return 1
	}
	thisTag := titleMatch[1]

	// 3. Determine the bump class.
	bump := bumpClass(previous, thisTag)

	// 4. Channel from the tag.
	channel, err := version.ChannelOf(thisTag)
	if err != nil {
		fmt.Fprintf(stderr, "forge-internal-version: %s: %v\n", path, err)
		return 1
	}

	// 5. Required sections.
	required := []string{"Summary", "Distribution", "Artifacts", "Full changelog"}
	switch {
	case bump == "MAJOR":
		required = append([]string{"Summary", "Breaking changes"}, required[1:]...)
	case bump == "MINOR" && channel == version.ChannelStable && strings.HasPrefix(thisTag, "v0."):
		// v0.x.0 MAY break; the notes MUST contain the heading only if
		// a break occurred. The check is permissive: if "Breaking
		// changes" is present it must be non-empty; if absent, that
		// is acceptable for a v0.x.0 that added only features.
	case channel == version.ChannelPrerelease || channel == version.ChannelNightly:
		// Prerelease and nightly notes use the same template but the
		// "Distribution" table is shorter. The required set is the
		// same.
	}

	for _, heading := range required {
		if !hasNonEmptySection(body, heading) {
			fmt.Fprintf(stderr, "forge-internal-version: %s is missing a non-empty '%s' section\n", path, heading)
			return 1
		}
	}

	// 6. If "Breaking changes" is present, it must be non-empty.
	if strings.Contains(body, "## Breaking changes") && !hasNonEmptySection(body, "Breaking changes") {
		fmt.Fprintf(stderr, "forge-internal-version: %s has an empty 'Breaking changes' section\n", path)
		return 1
	}

	fmt.Fprintf(stdout, "OK: %s (%s, %s)\n", path, channel, bump)
	return 0
}

// bumpClass returns "MAJOR", "MINOR", "PATCH", or "" if unknown.
// It compares two SemVer tags. It does not attempt full SemVer
// precedence; it compares the numeric MAJOR.MINOR.PATCH triples and
// returns the largest differing segment.
func bumpClass(previous, current string) string {
	if previous == "" {
		return ""
	}
	pv, ok := parseSimpleSemver(previous)
	if !ok {
		return ""
	}
	cv, ok := parseSimpleSemver(current)
	if !ok {
		return ""
	}
	switch {
	case cv[0] != pv[0]:
		return "MAJOR"
	case cv[1] != pv[1]:
		return "MINOR"
	case cv[2] != pv[2]:
		return "PATCH"
	default:
		return ""
	}
}

var simpleSemverRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)`)

func parseSimpleSemver(tag string) ([3]int, bool) {
	m := simpleSemverRe.FindStringSubmatch(tag)
	if m == nil {
		return [3]int{}, false
	}
	var v [3]int
	for i := 0; i < 3; i++ {
		fmt.Sscanf(m[i+1], "%d", &v[i])
	}
	return v, true
}

// hasNonEmptySection reports whether body contains the heading
// "## <name>" followed by at least one non-blank, non-heading line
// before the next "## " heading.
func hasNonEmptySection(body, name string) bool {
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inSection := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "## ") {
			if inSection {
				return false
			}
			if strings.TrimSpace(strings.TrimPrefix(line, "## ")) == name {
				inSection = true
			}
			continue
		}
		if inSection && strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}
