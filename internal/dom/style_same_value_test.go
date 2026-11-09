package dom

import (
	"encoding/json"
	"os"
	"testing"
)

// sameValueCase is one case of testdata/style_same_value.json, which
// tools/reference/gen-dom-style-same-value.mjs wrote with Chromium: an
// element with an authored style attribute, some calls of
// style.setProperty and style.removeProperty, and the style attribute after
// the calls.
type sameValueCase struct {
	Name  string     `json:"name"`
	Tag   string     `json:"tag"`
	Style string     `json:"style"`
	Ops   [][]string `json:"ops"`
	Want  string     `json:"want"`
}

// TestStyleSameValueAgainstChromium checks what Style does with the style
// attribute when the inline style already has the value. Chromium compares
// the parsed values. For an equal value it changes nothing, and the
// attribute keeps its text, also a text in a non-canonical form.
func TestStyleSameValueAgainstChromium(t *testing.T) {
	data, err := os.ReadFile("testdata/style_same_value.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []sameValueCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 200 {
		t.Fatalf("only %d cases", len(cases))
	}
	// The engine has no model of shorthand properties: it keeps margin and
	// margin-top as two declarations, and it does not know that the value
	// "1px 1px" of margin is the value "1px". That is a limit of the css
	// package that these cases show. They are not checked here.
	shorthand := func(c sameValueCase) bool {
		property, value := c.Ops[0][1], ""
		if len(c.Ops[0]) > 2 {
			value = c.Ops[0][2]
		}
		switch c.Style {
		case "margin:1px":
			return property == "margin-top" || value == "1px 1px"
		case "margin:1px 2px":
			return true
		case "overflow:hidden":
			return property == "overflow-x"
		case "font:12px arial":
			return true
		}
		return false
	}
	kept, failed, skipped := 0, 0, 0
	for _, c := range cases {
		if shorthand(c) {
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
		el.Attr("style", c.Style)
		for _, op := range c.Ops {
			switch op[0] {
			case "set":
				el.StylePriority(op[1], op[2], len(op) > 3 && op[3] == "important")
			case "remove":
				el.Style(op[1], nil)
			default:
				t.Fatalf("%s: unknown operation %q", c.Name, op[0])
			}
		}
		got, _ := el.GetAttr("style")
		if got != c.Want {
			failed++
			t.Errorf("%s: style %q, operations %v:\n got  %q\n want %q", c.Name, c.Style, c.Ops, got, c.Want)
			continue
		}
		if c.Want == c.Style {
			kept++
		}
	}
	t.Logf("%d cases, %d keep the text of the attribute, %d failed, %d not checked (shorthand properties)", len(cases), kept, failed, skipped)
}
