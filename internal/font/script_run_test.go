package font

import (
	"strings"
	"testing"
)

// TestScriptRuns checks the limits of the runs. The widths that follow from
// the runs are compared with Chromium in
// internal/browser/scriptrun_probe_test.go.
func TestScriptRuns(t *testing.T) {
	cases := []struct {
		text string
		want string // the runs, joined with "|"
	}{
		{"", ""},
		{"Alice", "Alice"},
		{"1234 %", "1234 %"},
		// A character of the script Common goes with the run before it.
		{"Привет Alice", "Привет |Alice"},
		{"Alice, Привет!", "Alice, |Привет!"},
		{"Bob 100% 中文 A I a", "Bob 100% |中文 |A I a"},
		{"日本語かな", "日本語|かな"},
		// Common characters at the start go with the first run.
		{"100% Привет", "100% Привет"},
		// A closing bracket goes with the run of its opening bracket.
		{"Привет (Alice) мир", "Привет (|Alice|) мир"},
		{"(Привет) Alice", "(Привет) |Alice"},
		{"Alice (Привет Tom) Yes", "Alice (|Привет |Tom) Yes"},
		{"Привет (A [Tom] Y) W", "Привет (|A [Tom] Y|) |W"},
		// A closing bracket without an opening bracket is a Common character.
		{"Привет) Alice", "Привет) |Alice"},
	}
	for _, c := range cases {
		text := []rune(c.text)
		var parts []string
		for _, r := range scriptRuns(text) {
			parts = append(parts, string(text[r.start:r.end]))
		}
		if got := strings.Join(parts, "|"); got != c.want {
			t.Errorf("scriptRuns(%q): got %q, want %q", c.text, got, c.want)
		}
	}
}
