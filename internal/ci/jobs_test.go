package ci_test

// jobs_test.go — a shared extractor for the `jobs:` block of a
// GitHub Actions workflow file.
//
// Both security_contract_test.go and flake_contract_test.go need to
// enumerate the top-level keys under `jobs:` and, for the timeout
// test, the body of each job. A single implementation keeps the two
// tests consistent and prevents the two from drifting into different
// — and differently wrong — extraction rules.
//
// The helpers are in package ci_test so every external test file in
// the package can use them. They do not read any file on their own;
// they take the file's content as a string, which makes them
// straightforward to unit-test.

import (
	"strings"
	"testing"
)

// jobKeysInJobsBlock returns the top-level keys under `jobs:` in a
// GitHub Actions workflow file, in file order.
//
// The extractor is stateful: it ignores everything before the
// top-level `jobs:` line, collects the keys indented exactly two
// spaces under it, and stops at the next top-level key (any line at
// column zero). A job key is a lowercase identifier ending in `:`
// with nothing else on the line.
//
// # Why this is not a single regexp
//
// A regexp anchored on the two-space indentation alone (`^  key:`)
// cannot tell a job key from a trigger key: the `on:` block uses the
// same shape. A regexp that swept the whole file would return
// `push`, `schedule`, and `workflow_dispatch` alongside the real
// job keys, and a test that compared that set against the model
// would report the trigger names as undeclared jobs.
func jobKeysInJobsBlock(t *testing.T, path, content string) []string {
	t.Helper()
	lines := strings.Split(content, "\n")

	inJobs := false
	var keys []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if strings.TrimSpace(line) == "jobs:" {
				inJobs = true
				continue
			}
			if inJobs {
				break
			}
			continue
		}
		if !inJobs {
			continue
		}
		indent := 0
		for indent < len(line) && line[indent] == ' ' {
			indent++
		}
		if indent != 2 {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if !strings.HasSuffix(trimmed, ":") {
			continue
		}
		name := strings.TrimSuffix(trimmed, ":")
		if !isLowercaseJobKey(name) {
			continue
		}
		keys = append(keys, name)
	}
	return keys
}

// isLowercaseJobKey reports whether s matches the job-key shape the
// tests rely on: a lowercase letter, followed by lowercase letters,
// digits, or hyphens.
func isLowercaseJobKey(s string) bool {
	if s == "" {
		return false
	}
	if s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '-':
		default:
			return false
		}
	}
	return true
}
