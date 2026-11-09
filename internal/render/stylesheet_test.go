package render

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// knownSheetGaps lists the probe cases that styleSheetRules does not
// reproduce, each with its cause.
var knownSheetGaps = map[string]string{
	"* { margin: 0 }": "css: a unitless zero of a box property does not get the unit px",
	".a { margin: 1px 2px 3px 4px; padding: 0 0 0 0 }":                    "css: equal sides of a shorthand are not collapsed",
	".a { font: bold 12px/14px Arial, sans-serif }":                       "css: the font shorthand changes the case of the family",
	"@font-face { font-family: X; src: url(x.woff) }":                     "css: the descriptor src is not known",
	".a { & .b { fill: red } }":                                           "nested style rules are not handled",
	".a { fill: red; .b { fill: blue } }":                                 "nested style rules are not handled",
	".a { width: calc( 1px + 2px ) ; height: CALC(100% - 10px) }":         "css: calc() is not simplified",
	".a::after { content: '\\201C' }":                                     "css: escapes in strings are not resolved",
	".a { transition: all .3s ease-in-out }":                              "css: the transition shorthand is not expanded",
	"@namespace svg url(http://www.w3.org/2000/svg); svg|a { fill: red }": "namespace prefixes in selectors are not handled",
}

// TestStyleSheetRulesAgainstChromium replays sheets that Chromium parsed
// with CSSStyleSheet.replaceSync and compares the cssText of the rules.
func TestStyleSheetRulesAgainstChromium(t *testing.T) {
	raw, err := os.ReadFile("testdata/stylesheet_chromium.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Sheets []struct {
			Text  string   `json:"text"`
			Rules []string `json:"rules"`
		} `json:"sheets"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	for _, c := range golden.Sheets {
		got := strings.Join(styleSheetRules(c.Text), "\n")
		want := strings.Join(c.Rules, "\n")
		if _, gap := knownSheetGaps[c.Text]; gap {
			if got == want {
				t.Errorf("%q is listed as a gap but matches", c.Text)
			}
			continue
		}
		if got != want {
			t.Errorf("sheet %q\n got: %q\nwant: %q", c.Text, got, want)
		}
	}
}
