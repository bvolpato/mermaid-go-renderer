package dom

import (
	"encoding/json"
	"os"
	"testing"
)

// TestStyleAttributePlacementAgainstChromium replays the cases of
// testdata/style_sync.json, which tools/reference/gen-dom-style-sync.mjs
// writes: operations on one element, and its outerHTML in Chromium after
// the last one.
//
// Blink keeps an inline style that the CSSOM wrote (element.style
// .setProperty, as d3 selection.style does) in the element, and adds the
// style attribute to the attribute list only when something reads the
// attributes: serialization, getAttribute('style'), a [style] selector,
// cloneNode. The attribute then goes to the end of the list.
func TestStyleAttributePlacementAgainstChromium(t *testing.T) {
	data, err := os.ReadFile("testdata/style_sync.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string     `json:"name"`
		Tag   string     `json:"tag"`
		Ops   [][]string `json:"ops"`
		Outer string     `json:"outer"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 300 {
		t.Fatalf("%d cases", len(cases))
	}
	pass := 0
	for _, c := range cases {
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		var parent, el *Node
		if c.Tag == "div" {
			parent = container.Append("section")
			el = parent.Append("div")
		} else {
			parent = container.Append("svg:svg").Append("g")
			el = parent.Append(c.Tag)
		}
		arg := func(op []string, i int) string {
			if i < len(op) {
				return op[i]
			}
			return ""
		}
		for _, op := range c.Ops {
			a, b := arg(op, 1), arg(op, 2)
			switch op[0] {
			case "attr":
				el.Attr(a, b)
			case "remove":
				el.Attr(a, nil)
			case "get":
				el.GetAttr(a)
			case "has":
				el.HasAttr(a)
			case "style":
				el.Style(a, b)
			case "unstyle":
				el.Style(a, nil)
			case "outer":
				_ = el.OuterHTML()
			case "inner":
				_ = parent.InnerHTML()
			case "query":
				parent.Select(a)
			case "matches":
				el.Matches(a)
			case "clone":
				el.CloneNode(true)
			case "bbox":
				el.BBox()
			case "computed":
				el.ComputedStyle(a)
			case "text":
				el.Text(a)
			case "child":
				el.Append(a)
			}
		}
		if got := el.OuterHTML(); got != c.Outer {
			t.Errorf("%s: %v\n got  %s\n want %s", c.Name, c.Ops, got, c.Outer)
			continue
		}
		pass++
	}
	t.Logf("%d cases, %d pass", len(cases), pass)
}
