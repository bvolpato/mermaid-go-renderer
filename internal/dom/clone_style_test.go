package dom

import (
	"encoding/json"
	"os"
	"testing"
)

// styleSyncCase is one case of testdata/style_sync.json, which
// tools/reference/gen-dom-style-sync.mjs wrote with Chromium: operations on
// one element, and its outerHTML after the last operation.
type styleSyncCase struct {
	Name  string     `json:"name"`
	Tag   string     `json:"tag"`
	Ops   [][]string `json:"ops"`
	Outer string     `json:"outer"`
}

// TestCloneNodeSyncsStyleAgainstChromium checks where the style attribute
// of an element goes when the element is cloned. Blink synchronizes the
// style attribute of the source before it copies the attributes
// (Element::CloneAttributesFrom), so an inline style that was written
// through the CSSOM becomes an attribute of the source at that moment, and
// the clone has it. The test replays the cases of the golden file that
// clone the element and that use only the attribute, style, text and
// serialization operations.
func TestCloneNodeSyncsStyleAgainstChromium(t *testing.T) {
	data, err := os.ReadFile("testdata/style_sync.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []styleSyncCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	replayable := map[string]bool{"attr": true, "remove": true, "get": true, "has": true, "style": true, "unstyle": true,
		"outer": true, "inner": true, "clone": true, "text": true, "child": true}
	ran := 0
	for _, c := range cases {
		hasClone, ok := false, true
		for _, op := range c.Ops {
			hasClone = hasClone || op[0] == "clone"
			ok = ok && replayable[op[0]]
		}
		if !hasClone || !ok {
			continue
		}
		ran++
		doc := NewDocument()
		var parent, el *Node
		if c.Tag == "div" {
			parent = doc.Body.Append("section")
			el = parent.Append("div")
		} else {
			parent = doc.Body.Append("svg:svg").Append("g")
			el = parent.Append(c.Tag)
		}
		for _, op := range c.Ops {
			switch op[0] {
			case "attr":
				el.Attr(op[1], op[2])
			case "remove":
				el.Attr(op[1], nil)
			case "get":
				el.GetAttr(op[1])
			case "has":
				el.HasAttr(op[1])
			case "style":
				el.Style(op[1], op[2])
			case "unstyle":
				el.Style(op[1], nil)
			case "outer":
				_ = el.OuterHTML()
			case "inner":
				_ = parent.InnerHTML()
			case "clone":
				// The clone has the inline style that the source has.
				style, has := el.InlineStyle()
				clone := el.CloneNode(true)
				if got, cloneHas := clone.GetAttr("style"); cloneHas != has || got != style {
					t.Errorf("%s: style of the clone: got %q (%v), want %q (%v)", c.Name, got, cloneHas, style, has)
				}
			case "text":
				el.Text(op[1])
			case "child":
				el.Append(op[1])
			}
		}
		if got := el.OuterHTML(); got != c.Outer {
			t.Errorf("%s: %v\n got  %s\n want %s", c.Name, c.Ops, got, c.Outer)
		}
	}
	if ran < 10 {
		t.Fatalf("only %d cases with a clone", ran)
	}
}
