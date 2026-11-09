package dom

import (
	"encoding/json"
	"os"
	"testing"
)

// xmlCase is one case of testdata/xml-serialize.json, which
// tools/reference/gen-xml-serialize.mjs writes from Chromium.
type xmlCase struct {
	Name string `json:"name"`
	// SVG is the markup that became the innerHTML of a div.
	SVG string `json:"svg"`
	// Script is JavaScript that changed the tree after the parse.
	Script string `json:"script"`
	// Tree is the first svg element of the div in Chromium.
	Tree xmlTree `json:"tree"`
	// XML is new XMLSerializer().serializeToString(svg) in Chromium.
	XML string `json:"xml"`
}

type xmlTree struct {
	Text    *string   `json:"t"`
	Comment *string   `json:"c"`
	NS      string    `json:"ns"`
	Tag     string    `json:"tag"`
	Attrs   []xmlAttr `json:"attrs"`
	Kids    []xmlTree `json:"kids"`
}

type xmlAttr struct {
	NS     string `json:"ns"`
	Prefix string `json:"prefix"`
	Local  string `json:"local"`
	Value  string `json:"value"`
}

func (t *xmlTree) build(d *Document) *Node {
	switch {
	case t.Text != nil:
		return d.CreateText(*t.Text)
	case t.Comment != nil:
		return d.CreateComment(*t.Comment)
	}
	n := &Node{Type: ElementNode, NS: t.NS, Tag: t.Tag, doc: d}
	for _, a := range t.Attrs {
		n.Attrs = append(n.Attrs, Attr{NS: a.NS, Prefix: a.Prefix, Local: a.Local, Value: a.Value})
	}
	for i := range t.Kids {
		n.AppendChild(t.Kids[i].build(d))
	}
	return n
}

func loadXMLCases(t *testing.T) []xmlCase {
	t.Helper()
	data, err := os.ReadFile("testdata/xml-serialize.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []xmlCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 80 {
		t.Fatalf("only %d cases", len(cases))
	}
	return cases
}

// TestXMLFromTree serializes the tree that Chromium had. It tests the
// serializer alone.
func TestXMLFromTree(t *testing.T) {
	for _, c := range loadXMLCases(t) {
		t.Run(c.Name, func(t *testing.T) {
			got := c.Tree.build(NewDocument()).XML()
			if got != c.XML {
				t.Errorf("XML differs\n got: %s\nwant: %s", got, c.XML)
			}
		})
	}
}

// parserDivergences are the cases where the HTML parser of this package
// builds another tree than Chromium. The serializer is not the cause:
// TestXMLFromTree passes for them.
var parserDivergences = map[string]string{
	"empty/html-elements":    "golang.org/x/net/html drops what follows a template element in a fragment",
	"comment/markup-inside":  "golang.org/x/net/html decodes character references in a comment",
	"nested/html-breaks-out": "golang.org/x/net/html keeps an HTML start tag inside foreign content when it parses a fragment",
}

// TestXMLFromMarkup parses the markup as the innerHTML of a div, as
// mermaid-cli does, and serializes the first svg element.
func TestXMLFromMarkup(t *testing.T) {
	for _, c := range loadXMLCases(t) {
		if c.Script != "" {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			d := NewDocument()
			div := d.Body.Append("div")
			div.HTML(c.SVG)
			svg := div.Select("svg")
			if svg == nil {
				t.Fatal("no svg element")
			}
			got := svg.XML()
			if reason, known := parserDivergences[c.Name]; known {
				if got == c.XML {
					t.Errorf("the parser now agrees with Chromium: remove the case from parserDivergences (%s)", reason)
				}
				return
			}
			if got != c.XML {
				t.Errorf("XML differs\n got: %s\nwant: %s", got, c.XML)
			}
		})
	}
}
