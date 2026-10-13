package output

import "testing"

func TestParseFormat(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Format
		wantErr bool
	}{
		// Positive
		{"empty defaults to text", "", FormatText, false},
		{"text", "text", FormatText, false},
		{"json", "json", FormatJSON, false},
		{"TEXT uppercase", "TEXT", FormatText, false},
		{"JSON uppercase", "JSON", FormatJSON, false},
		{"padded", "  json  ", FormatJSON, false},

		// Negative
		{"unknown", "yaml", FormatText, true},
		{"garbage", "!!!", FormatText, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseFormat(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFormat_String(t *testing.T) {
	if got := FormatText.String(); got != "text" {
		t.Fatalf("got %q", got)
	}
	if got := FormatJSON.String(); got != "json" {
		t.Fatalf("got %q", got)
	}
	if got := Format(99).String(); got == "text" || got == "json" {
		t.Fatalf("unknown format should not masquerade: %q", got)
	}
}

func TestFormat_IsJSON(t *testing.T) {
	if FormatText.IsJSON() {
		t.Fatal("text should not be JSON")
	}
	if !FormatJSON.IsJSON() {
		t.Fatal("json should be JSON")
	}
}
