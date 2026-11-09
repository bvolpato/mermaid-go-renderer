package dom

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// rangeCase is one case of testdata/style_range.json, which
// tools/reference/gen-dom-style-range.mjs wrote with Chromium: an element
// with no style attribute, one call of style.setProperty with a numeric
// value, and the style attribute after the call. Want is nil when Chromium
// rejects the value and the element gets no attribute.
type rangeCase struct {
	Name  string     `json:"name"`
	Tag   string     `json:"tag"`
	Style string     `json:"style"`
	Ops   [][]string `json:"ops"`
	Want  *string    `json:"want"`
}

// TestStyleRangeAgainstChromium checks which numeric values Style accepts.
// Chromium rejects a negative value for the properties whose range has no
// negative values, such as width, max-width and padding. The call then
// changes nothing.
func TestStyleRangeAgainstChromium(t *testing.T) {
	data, err := os.ReadFile("testdata/style_range.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []rangeCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 1000 {
		t.Fatalf("only %d cases", len(cases))
	}
	show := func(s *string, ok bool) string {
		if s == nil || !ok {
			return "no attribute"
		}
		return `"` + *s + `"`
	}
	// The cases also show three limits of the css package that are not
	// about the range of a value. They are not checked here:
	//   - a unitless zero of x, y, cx, cy, r, rx or ry: Chromium writes 0px;
	//   - a percentage of outline-offset or tab-size: Chromium rejects it;
	//   - a unitless number of row-gap, column-gap, outline-offset or
	//     flex-basis on an SVG element: Chromium rejects it.
	otherLimit := func(c rangeCase) bool {
		property, value := c.Ops[0][1], c.Ops[0][2]
		unitless := value == "-10" || value == "10" || value == "1.5"
		switch property {
		case "x", "y", "cx", "cy", "r", "rx", "ry":
			return value == "0" || value == "-0"
		case "outline-offset":
			return strings.HasSuffix(value, "%") || (c.Tag != "div" && unitless)
		case "tab-size":
			return strings.HasSuffix(value, "%")
		case "row-gap", "column-gap", "flex-basis":
			return c.Tag != "div" && unitless
		}
		return false
	}
	rejected, failed, skipped := 0, 0, 0
	for _, c := range cases {
		if otherLimit(c) {
			skipped++
			continue
		}
		doc := NewDocument()
		var el *Node
		if c.Tag == "div" {
			el = doc.Body.Append("section").Append("div")
		} else {
			el = doc.Body.Append("svg:svg").Append("g").Append(c.Tag)
		}
		for _, op := range c.Ops {
			el.Style(op[1], op[2])
		}
		got, ok := el.GetAttr("style")
		if c.Want == nil {
			rejected++
		}
		if ok != (c.Want != nil) || (ok && got != *c.Want) {
			failed++
			t.Errorf("%s:\n got  %s\n want %s", c.Name, show(&got, ok), show(c.Want, c.Want != nil))
		}
	}
	t.Logf("%d cases, Chromium rejects %d of the checked ones, %d failed, %d not checked (other limits)", len(cases), rejected, failed, skipped)
}
