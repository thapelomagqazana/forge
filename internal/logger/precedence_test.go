package logger

import "testing"

// --- TDD: each source alone ---

func TestResolveLevel_EachSourceAlone(t *testing.T) {
	cases := []struct {
		name       string
		in         Inputs
		wantLevel  Level
		wantSource Source
	}{
		{
			name:       "default when nothing set",
			in:         Inputs{},
			wantLevel:  LevelInfo,
			wantSource: SourceDefault,
		},
		{
			name:       "config alone",
			in:         Inputs{ConfigLevel: "warn"},
			wantLevel:  LevelWarn,
			wantSource: SourceConfig,
		},
		{
			name:       "env alone",
			in:         Inputs{EnvLevel: "debug"},
			wantLevel:  LevelDebug,
			wantSource: SourceEnv,
		},
		{
			name:       "quiet alone",
			in:         Inputs{Quiet: true},
			wantLevel:  LevelError,
			wantSource: SourceFlagQuiet,
		},
		{
			name:       "verbose alone",
			in:         Inputs{Verbose: true},
			wantLevel:  LevelDebug,
			wantSource: SourceFlagVerbose,
		},
		{
			name:       "explicit flag alone",
			in:         Inputs{LogLevel: "warn"},
			wantLevel:  LevelWarn,
			wantSource: SourceFlagExplicit,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveLevel(tc.in)
			if err != nil {
				t.Fatalf("ResolveLevel: %v", err)
			}
			if got.Level != tc.wantLevel {
				t.Fatalf("Level = %v, want %v", got.Level, tc.wantLevel)
			}
			if got.Source != tc.wantSource {
				t.Fatalf("Source = %v, want %v", got.Source, tc.wantSource)
			}
		})
	}
}

// --- TDD: precedence chain ---

func TestResolveLevel_Precedence(t *testing.T) {
	cases := []struct {
		name       string
		in         Inputs
		wantLevel  Level
		wantSource Source
	}{
		{
			name: "explicit beats verbose",
			in: Inputs{
				LogLevel: "error",
				Verbose:  true,
			},
			wantLevel:  LevelError,
			wantSource: SourceFlagExplicit,
		},
		{
			name: "explicit beats quiet",
			in: Inputs{
				LogLevel: "debug",
				Quiet:    true,
			},
			wantLevel:  LevelDebug,
			wantSource: SourceFlagExplicit,
		},
		{
			name: "explicit beats env",
			in: Inputs{
				LogLevel: "warn",
				EnvLevel: "debug",
			},
			wantLevel:  LevelWarn,
			wantSource: SourceFlagExplicit,
		},
		{
			name: "explicit beats config",
			in: Inputs{
				LogLevel:    "info",
				ConfigLevel: "debug",
			},
			wantLevel:  LevelInfo,
			wantSource: SourceFlagExplicit,
		},
		{
			name: "verbose beats quiet",
			in: Inputs{
				Verbose: true,
				Quiet:   true,
			},
			wantLevel:  LevelDebug,
			wantSource: SourceFlagVerbose,
		},
		{
			name: "verbose beats env",
			in: Inputs{
				Verbose:  true,
				EnvLevel: "error",
			},
			wantLevel:  LevelDebug,
			wantSource: SourceFlagVerbose,
		},
		{
			name: "quiet beats env",
			in: Inputs{
				Quiet:    true,
				EnvLevel: "debug",
			},
			wantLevel:  LevelError,
			wantSource: SourceFlagQuiet,
		},
		{
			name: "env beats config",
			in: Inputs{
				EnvLevel:    "warn",
				ConfigLevel: "debug",
			},
			wantLevel:  LevelWarn,
			wantSource: SourceEnv,
		},
		{
			name: "config beats default",
			in: Inputs{
				ConfigLevel: "error",
			},
			wantLevel:  LevelError,
			wantSource: SourceConfig,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveLevel(tc.in)
			if err != nil {
				t.Fatalf("ResolveLevel: %v", err)
			}
			if got.Level != tc.wantLevel {
				t.Fatalf("Level = %v, want %v", got.Level, tc.wantLevel)
			}
			if got.Source != tc.wantSource {
				t.Fatalf("Source = %v, want %v", got.Source, tc.wantSource)
			}
		})
	}
}

// --- BDD: given/when/then for the headline precedence rule ---

func TestResolveLevel_EnvBeatsConfig_BDD(t *testing.T) {
	// Given a config file that sets log.level to debug
	// And FORGE_LOG_LEVEL set to warn
	in := Inputs{
		ConfigLevel: "debug",
		EnvLevel:    "warn",
	}

	// When the effective level is resolved
	got, err := ResolveLevel(in)

	// Then the env var wins, and the level is WARN
	if err != nil {
		t.Fatalf("ResolveLevel: %v", err)
	}
	if got.Level != LevelWarn {
		t.Fatalf("Level = %v, want WARN", got.Level)
	}
	if got.Source != SourceEnv {
		t.Fatalf("Source = %v, want env", got.Source)
	}
}

// --- Negative: invalid value at each source is an error, not a fallthrough ---

func TestResolveLevel_InvalidValuesAreErrors(t *testing.T) {
	cases := []struct {
		name string
		in   Inputs
	}{
		{"invalid log-level flag", Inputs{LogLevel: "trace"}},
		{"invalid env", Inputs{EnvLevel: "loud"}},
		{"invalid config", Inputs{ConfigLevel: "silent"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveLevel(tc.in)
			if err == nil {
				t.Fatal("expected error for invalid value")
			}
		})
	}
}

// --- Negative: invalid higher-precedence source does NOT fall through ---

func TestResolveLevel_InvalidHigherSource_NoFallthrough(t *testing.T) {
	// A valid config exists, but the flag is invalid. The flag must
	// error; the resolver must not silently use the config.
	_, err := ResolveLevel(Inputs{
		LogLevel:    "trace",
		ConfigLevel: "debug",
	})
	if err == nil {
		t.Fatal("invalid flag must error even when config is valid")
	}
}

// --- Edge: empty strings are treated as unset ---

func TestResolveLevel_EmptyStringsAreUnset(t *testing.T) {
	got, err := ResolveLevel(Inputs{
		LogLevel:    "",
		EnvLevel:    "",
		ConfigLevel: "",
	})
	if err != nil {
		t.Fatalf("ResolveLevel: %v", err)
	}
	if got.Source != SourceDefault {
		t.Fatalf("Source = %v, want default", got.Source)
	}
	if got.Level != LevelInfo {
		t.Fatalf("Level = %v, want INFO", got.Level)
	}
}

// --- Corner: alias and case in each source ---

func TestResolveLevel_AliasAndCase(t *testing.T) {
	cases := []struct {
		name      string
		in        Inputs
		wantLevel Level
	}{
		{"flag warning alias", Inputs{LogLevel: "warning"}, LevelWarn},
		{"flag uppercase", Inputs{LogLevel: "DEBUG"}, LevelDebug},
		{"env warning alias", Inputs{EnvLevel: "WARNING"}, LevelWarn},
		{"env padded", Inputs{EnvLevel: "  warn  "}, LevelWarn},
		{"config uppercase", Inputs{ConfigLevel: "ERROR"}, LevelError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveLevel(tc.in)
			if err != nil {
				t.Fatalf("ResolveLevel: %v", err)
			}
			if got.Level != tc.wantLevel {
				t.Fatalf("Level = %v, want %v", got.Level, tc.wantLevel)
			}
		})
	}
}

// --- Source.String is stable ---

func TestSource_String(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range []Source{
		SourceFlagExplicit, SourceFlagVerbose, SourceFlagQuiet,
		SourceEnv, SourceConfig, SourceDefault,
	} {
		str := s.String()
		if str == "" || str == "unknown" {
			t.Errorf("Source(%d).String() = %q, want non-empty non-unknown", s, str)
		}
		if seen[str] {
			t.Errorf("Source(%d).String() = %q duplicates another source", s, str)
		}
		seen[str] = true
	}
}

// --- Non-functional: ResolveLevel is allocation-light ---

func TestResolveLevel_AllocsBound(t *testing.T) {
	in := Inputs{LogLevel: "warn"}
	allocs := testing.AllocsPerRun(1000, func() {
		_, _ = ResolveLevel(in)
	})
	// A small fixed number of allocations is acceptable (string
	// parsing for the flag value). The point is that it does not grow
	// with input size or call count.
	if allocs > 4 {
		t.Fatalf("ResolveLevel allocated %.1f times, want <= 4", allocs)
	}
}
