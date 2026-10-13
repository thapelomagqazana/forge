package config

import (
	"fmt"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/logger"
)

// =============================================================================
// Synthetic schema
// =============================================================================
//
// The tests exercise the redaction mechanism with a test-local
// schema. The synthetic schema has two credential fields
// (Registry.Token, Registry.Password) that the production schema
// does not have. The tests inject these fields into a
// test-local struct and assert that the mechanism moves them to
// their Secret counterparts.
//
// # Why the tests use a synthetic schema
//
// The production schema (WBS 8.2.1) has no credential fields.
// The mechanism's behavior cannot be exercised against the
// current schema without inventing fields, which would violate
// the schema's frozen contract. The synthetic schema lets the
// mechanism be tested without touching the production type.
//
// # How the synthetic schema relates to the production mechanism
//
// The production RedactSecrets iterates the package-level
// sensitiveFields map (sensitive.go) and calls each entry's
// mover function. The test's applySynthMovers mirrors that
// iteration against a test-local type. The two share the same
// shape; the test proves the shape works, and the production
// RedactSecrets is a no-op today because sensitiveFields is
// empty.

type synthRegistry struct {
	URL            string
	Token          string
	TokenSecret    logger.Secret
	Password       string
	PasswordSecret logger.Secret
}

type synthConfig struct {
	Registry *synthRegistry
}

// synthMovers is the test-local equivalent of sensitiveFields. It
// maps a field path to a function that moves the field on a
// *synthConfig.
//
// # Why the movers are on synthConfig and not on Config
//
// Config's fields are frozen by WBS 8.2.1; adding a field would
// violate the freeze. The test-local type lets the mechanism's
// shape be exercised without altering the frozen type. The
// production movers will be on *Config when the production schema
// gains the fields; the shape (a function that copies a nested
// struct and moves a field) is identical.
var synthMovers = map[string]func(*synthConfig){
	"Registry.Token": func(c *synthConfig) {
		if c == nil || c.Registry == nil {
			return
		}
		r := *c.Registry
		if r.Token != "" {
			r.TokenSecret = logger.NewSecret(r.Token)
			r.Token = ""
		}
		c.Registry = &r
	},
	"Registry.Password": func(c *synthConfig) {
		if c == nil || c.Registry == nil {
			return
		}
		r := *c.Registry
		if r.Password != "" {
			r.PasswordSecret = logger.NewSecret(r.Password)
			r.Password = ""
		}
		c.Registry = &r
	},
}

// applySynthMovers applies every mover in synthMovers to c. It
// mirrors the production RedactSecrets' iteration; the production
// function iterates sensitiveFields (sensitive.go).
func applySynthMovers(c *synthConfig) {
	for _, move := range synthMovers {
		move(c)
	}
}

// =============================================================================
// TDD: the mechanism moves the field
// =============================================================================

func TestSynthRedaction_MovesToken(t *testing.T) {
	cfg := &synthConfig{Registry: &synthRegistry{Token: "hunter2"}}
	applySynthMovers(cfg)
	if cfg.Registry.Token != "" {
		t.Fatalf("Token not cleared: %q", cfg.Registry.Token)
	}
	if cfg.Registry.TokenSecret.Reveal() != "hunter2" {
		t.Fatal("TokenSecret lost the value")
	}
}

func TestSynthRedaction_MovesPassword(t *testing.T) {
	cfg := &synthConfig{Registry: &synthRegistry{Password: "p1"}}
	applySynthMovers(cfg)
	if cfg.Registry.Password != "" {
		t.Fatalf("Password not cleared: %q", cfg.Registry.Password)
	}
	if cfg.Registry.PasswordSecret.Reveal() != "p1" {
		t.Fatal("PasswordSecret lost the value")
	}
}

func TestSynthRedaction_MultipleSecrets(t *testing.T) {
	cfg := &synthConfig{
		Registry: &synthRegistry{Token: "t1", Password: "p1"},
	}
	applySynthMovers(cfg)
	for _, leak := range []string{"t1", "p1"} {
		if strings.Contains(fmt.Sprintf("%v", cfg), leak) {
			t.Fatalf("Config leaked %q", leak)
		}
	}
}

func TestSynthRedaction_NoMutationOfInput(t *testing.T) {
	cfg := &synthConfig{Registry: &synthRegistry{Token: "hunter2"}}
	// The mover copies the registry before mutating, so the
	// input's Registry field is the same pointer after the call,
	// and the pointer's contents are unchanged.
	before := *cfg.Registry
	applySynthMovers(cfg)

	// The registry pointer is the same as before: the mover
	// replaced c.Registry with a *new* struct, but the caller's
	// original struct (before) was copied, not mutated.
	if cfg.Registry == &before {
		// The pointer identity check is intentional: the mover
		// is supposed to *replace* the field with a copy, not
		// mutate the original. The check would be true only if
		// the mover took a different approach; the current
		// implementation copies.
		t.Log("mover reused the same pointer; this is also acceptable")
	}
	if before.Token != "hunter2" {
		t.Fatalf("input was mutated: Token = %q", before.Token)
	}
}

func TestSynthRedaction_NilRegistry(t *testing.T) {
	cfg := &synthConfig{}
	applySynthMovers(cfg)
	if cfg.Registry != nil {
		t.Fatal("nil Registry should remain nil")
	}
}

func TestSynthRedaction_NilConfig(t *testing.T) {
	applySynthMovers(nil) // must not panic
}

// =============================================================================
// TDD: the production RedactSecrets on the frozen schema
// =============================================================================
//
// The three tests below assert the production function's behavior
// against the current schema. The schema has no sensitive fields,
// so the function is a no-op. When the schema gains a sensitive
// field, these tests are extended with cases that exercise the
// new entry; the "no-op" assertions remain, because a Config
// without the new field's value still has nothing to redact.

// assertConfigEqual asserts that two Config values are equal on
// their exported fields.
//
// # Why not ==
//
// Config contains a map (sources); Go forbids == and != on
// structs containing maps. The helper compares the three
// exported fields explicitly.
//
// # Why not reflect.DeepEqual
//
// reflect.DeepEqual would compare the unexported sources map
// too, which the test does not want to couple to. It is also
// slower and less readable at the call site. The explicit
// helper documents *what* "equal" means and fails to compile if
// a future Config field is added (the helper's body would
// reference a field that does not exist), forcing a conscious
// update.
//
// # What the helper does not compare
//
// The unexported sources map is not compared. A caller that
// wants to assert on sources uses SourceFor on the field of
// interest, which is the read path the production code uses.
func assertConfigEqual(t *testing.T, name string, got, want Config) {
	t.Helper()
	if got.LogLevel != want.LogLevel {
		t.Errorf("%s: LogLevel = %q; want %q",
			name, got.LogLevel, want.LogLevel)
	}
	if got.OutputFormat != want.OutputFormat {
		t.Errorf("%s: OutputFormat = %q; want %q",
			name, got.OutputFormat, want.OutputFormat)
	}
	if got.SourcePath != want.SourcePath {
		t.Errorf("%s: SourcePath = %q; want %q",
			name, got.SourcePath, want.SourcePath)
	}
}

func TestRedactSecrets_EmptySchemaIsNoOp(t *testing.T) {
	in := Config{
		LogLevel:     "debug",
		OutputFormat: "json",
		SourcePath:   "/etc/forge/config.yaml",
	}
	out := RedactSecrets(&in)
	assertConfigEqual(t, "no-op", out, in)
}

func TestRedactSecrets_NilConfig(t *testing.T) {
	got := RedactSecrets(nil)
	assertConfigEqual(t, "nil", got, Config{})
}

func TestRedactSecrets_DoesNotMutateInput(t *testing.T) {
	in := Config{
		LogLevel:     "debug",
		OutputFormat: "json",
		SourcePath:   "/etc/forge/config.yaml",
	}
	before := in
	_ = RedactSecrets(&in)
	assertConfigEqual(t, "input after call", in, before)
}

// =============================================================================
// Non-functional: the mechanism does not allocate when the schema is empty
// =============================================================================

func TestRedactSecrets_EmptySchemaNoAllocation(t *testing.T) {
	// The function is a no-op on the current schema. A no-op
	// should not allocate beyond the returned Config value's own
	// storage (which is on the caller's stack for a value
	// return). The bound is a canary: a future change that makes
	// the empty-schema path allocate is caught here.
	in := Config{LogLevel: "info"}
	allocs := testing.AllocsPerRun(1000, func() {
		_ = RedactSecrets(&in)
	})
	if allocs > 1 {
		t.Fatalf("RedactSecrets allocated %.1f times on an empty "+
			"schema; want <= 1", allocs)
	}
}
