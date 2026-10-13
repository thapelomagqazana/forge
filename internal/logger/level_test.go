package logger

import "testing"

// --- TDD: ParseLevel ---

func TestParseLevel(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Level
		wantErr bool
	}{
		// Positive: canonical names
		{"empty defaults to info", "", LevelInfo, false},
		{"debug", "debug", LevelDebug, false},
		{"info", "info", LevelInfo, false},
		{"warn", "warn", LevelWarn, false},
		{"error", "error", LevelError, false},

		// Positive: case and whitespace
		{"DEBUG uppercase", "DEBUG", LevelDebug, false},
		{"Info mixed", "Info", LevelInfo, false},
		{"  warn  padded", "  warn  ", LevelWarn, false},

		// Corner: alias
		{"warning alias", "warning", LevelWarn, false},
		{"WARNING alias uppercase", "WARNING", LevelWarn, false},

		// Negative: unknown values
		{"unknown trace", "trace", LevelInfo, true},
		{"unknown fatal", "fatal", LevelInfo, true},
		{"unknown numeric", "3", LevelInfo, true},
		{"unknown garbage", "!!!", LevelInfo, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLevel(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseLevel(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLevel(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ParseLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// --- TDD: Level.Enabled boundary ---

func TestLevel_Enabled(t *testing.T) {
	// The ordering rule: a threshold enables itself and everything
	// more severe.
	cases := []struct {
		threshold Level
		record    Level
		want      bool
	}{
		// Debug threshold enables everything.
		{LevelDebug, LevelDebug, true},
		{LevelDebug, LevelInfo, true},
		{LevelDebug, LevelWarn, true},
		{LevelDebug, LevelError, true},

		// Info threshold enables Info and above.
		{LevelInfo, LevelDebug, false},
		{LevelInfo, LevelInfo, true},
		{LevelInfo, LevelWarn, true},
		{LevelInfo, LevelError, true},

		// Warn threshold enables Warn and above.
		{LevelWarn, LevelDebug, false},
		{LevelWarn, LevelInfo, false},
		{LevelWarn, LevelWarn, true},
		{LevelWarn, LevelError, true},

		// Error threshold enables only Error.
		{LevelError, LevelDebug, false},
		{LevelError, LevelInfo, false},
		{LevelError, LevelWarn, false},
		{LevelError, LevelError, true},
	}
	for _, tc := range cases {
		got := tc.threshold.Enabled(tc.record)
		if got != tc.want {
			t.Errorf("%v.Enabled(%v) = %v, want %v",
				tc.threshold, tc.record, got, tc.want)
		}
	}
}

// --- TDD: String and Describe ---

func TestLevel_String(t *testing.T) {
	want := map[Level]string{
		LevelDebug: "debug",
		LevelInfo:  "info",
		LevelWarn:  "warn",
		LevelError: "error",
	}
	for l, w := range want {
		if got := l.String(); got != w {
			t.Errorf("%d.String() = %q, want %q", l, got, w)
		}
	}
	// Unknown level must not masquerade as a canonical name.
	if got := Level(99).String(); got == "debug" || got == "info" ||
		got == "warn" || got == "error" {
		t.Errorf("unknown level String() = %q, want non-canonical", got)
	}
}

func TestLevel_Describe(t *testing.T) {
	// Every canonical level must have a non-empty, distinct description.
	seen := map[string]bool{}
	for _, l := range AllLevels() {
		d := l.Describe()
		if d == "" {
			t.Errorf("%v.Describe() is empty", l)
		}
		if seen[d] {
			t.Errorf("%v.Describe() = %q duplicates another level", l, d)
		}
		seen[d] = true
	}
}

// --- TDD: AllLevels ---

func TestAllLevels(t *testing.T) {
	got := AllLevels()
	want := []Level{LevelDebug, LevelInfo, LevelWarn, LevelError}
	if len(got) != len(want) {
		t.Fatalf("AllLevels() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllLevels()[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// AllLevels must return a fresh copy: mutating it must not affect
	// the next call.
	got[0] = LevelError
	again := AllLevels()
	if again[0] != LevelDebug {
		t.Fatal("AllLevels() returned a shared slice")
	}
}

// --- Boundary: ordering is total ---

func TestLevel_Ordering(t *testing.T) {
	// The iota ordering is part of the contract; a future reordering
	// would break Enabled. Assert it explicitly.
	if !(LevelDebug < LevelInfo && LevelInfo < LevelWarn && LevelWarn < LevelError) {
		t.Fatal("level ordering violated: want Debug < Info < Warn < Error")
	}
}

// --- Non-functional: no allocation on hot path ---

func TestLevel_Enabled_NoAllocation(t *testing.T) {
	// Enabled is called on every log call; it must not allocate.
	allocs := testing.AllocsPerRun(1000, func() {
		_ = LevelInfo.Enabled(LevelWarn)
	})
	if allocs != 0 {
		t.Fatalf("Enabled allocated %.1f times, want 0", allocs)
	}
}
