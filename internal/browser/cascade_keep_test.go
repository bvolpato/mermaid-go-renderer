package browser

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// cascadeSheets are style sheets with the kinds of selectors that the
// cascade keeps results for in different ways: only the ancestors, the
// earlier siblings, all siblings, and the rest of the tree.
var cascadeSheets = []string{
	// Only the element and its ancestors.
	`.a { fill: red; font-size: 10px } .a .b { fill: blue } .a > .c rect { stroke: green; font-weight: bold }
	 g.b text { font-size: 20px } #x { fill: #111 } #x .c { color: #222 } [data-k="1"] { opacity: 0.5 }
	 [data-k="1"] .a { stroke-width: 2 } :root { --v: 7px } .c { letter-spacing: var(--v) }
	 .a { --w: 3px } .b { word-spacing: var(--w, 1px); font-style: italic } rect { visibility: hidden }
	 .d :not(.a) { text-anchor: end } :is(.a, .b) > path { display: none } .b .a .c { fill: yellow !important }
	 [style*="red"] { opacity: 0.1 } .a [style] { stroke: orange }`,
	// The position among the earlier siblings.
	`.a { fill: red } .a path:first-of-type { stroke: blue } .b rect:nth-of-type(2) { fill: green }
	 g:nth-child(odd) { font-size: 11px } .c:first-child text { font-weight: bold } :nth-child(3) { opacity: 0.3 }
	 .a :not(:first-child) { color: #123 }`,
	// The siblings behind an element too.
	`.a { fill: red } .a path:last-of-type { stroke: blue } g:last-child { font-size: 12px }
	 .b :only-child { fill: green } rect:nth-last-child(2) { opacity: 0.2 } .c:only-of-type { color: #321 }`,
	// The attributes of siblings and the children.
	`.a { fill: red } .a + .b { stroke: blue } .c ~ g { font-size: 13px } g:empty { opacity: 0.4 }
	 .b:empty + rect { fill: green } .a > .b + .c text { color: #456 }`,
}

var cascadeProps = []string{
	"fill", "stroke", "font-size", "font-weight", "font-style", "opacity", "color", "letter-spacing", "word-spacing",
	"visibility", "text-anchor", "display", "stroke-width", "font-family",
}

// cascadeSnapshot prints the computed style of every element under root,
// with the engine e.
func cascadeSnapshot(e *Engine, root *dom.Node) string {
	var sb strings.Builder
	var walk func(n *dom.Node, path string)
	walk = func(n *dom.Node, path string) {
		i := 0
		for c := n.FirstChild; c != nil; c = c.Next {
			if c.Type != dom.ElementNode {
				continue
			}
			p := fmt.Sprintf("%s/%s[%d]", path, c.Tag, i)
			i++
			sb.WriteString(p)
			for _, prop := range cascadeProps {
				sb.WriteString(" " + prop + "=" + e.ComputedStyle(c, prop))
			}
			sb.WriteByte('\n')
			walk(c, p)
		}
	}
	walk(root, "")
	return sb.String()
}

// TestKeptCascadeAgainstNewEngine changes a document in many ways and asks
// for computed styles between the changes, with one engine that keeps its
// cascades as long as it can. After each change the styles of all elements
// must be the styles that a new engine computes for a copy of the document:
// a copy has no kept result.
func TestKeptCascadeAgainstNewEngine(t *testing.T) {
	fonts := font.Embedded()
	tags := []string{"g", "rect", "path", "text", "g", "g"}
	classes := []string{"a", "b", "c", "d", "a b", "b c", ""}
	for sheetIndex, sheet := range cascadeSheets {
		for seed := int64(1); seed <= 12; seed++ {
			rng := rand.New(rand.NewSource(seed*100 + int64(sheetIndex)))
			doc := NewDocumentWithFonts(fonts)
			e := doc.Engine.(*Engine)
			svg := doc.Body.Append("svg")
			style := svg.Append("style")
			style.Text(sheet)
			root := svg.Append("g")
			// elements returns the elements under the svg, in order.
			elements := func() []*dom.Node {
				var list []*dom.Node
				var walk func(n *dom.Node)
				walk = func(n *dom.Node) {
					for c := n.FirstChild; c != nil; c = c.Next {
						if c.Type == dom.ElementNode && c.Tag != "style" {
							list = append(list, c)
							walk(c)
						}
					}
				}
				walk(svg)
				return list
			}
			pick := func() *dom.Node {
				list := elements()
				return list[rng.Intn(len(list))]
			}
			// prepared is a style element with its text that is not in the
			// document yet.
			var prepared *dom.Node
			for step := 0; step < 120; step++ {
				n := pick()
				what := fmt.Sprintf("sheet %d, seed %d, step %d", sheetIndex, seed, step)
				switch op := rng.Intn(27); {
				case op >= 24 && op < 26:
					// A style element that was made outside of the document
					// goes in, in a later step than its text.
					if prepared == nil {
						prepared = doc.CreateElementNS(dom.NSSVG, "style")
						prepared.AppendChild(doc.CreateText(`.b { fill: brown } .a .c { stroke: teal }`))
					} else {
						holder := doc.CreateElementNS(dom.NSSVG, "defs")
						holder.AppendChild(prepared)
						svg.AppendChild(holder)
						prepared = nil
					}
				case op == 26:
					// More text for the first style element, without a
					// removal of the old text.
					style.AppendChild(doc.CreateText(` .d { fill: gold } `))
				case op < 6:
					c := n.Append(tags[rng.Intn(len(tags))])
					if cl := classes[rng.Intn(len(classes))]; cl != "" {
						c.Attr("class", cl)
					}
				case op < 8:
					n.Attr("class", classes[rng.Intn(len(classes))])
				case op < 9:
					n.Attr("class", nil)
				case op < 10:
					n.Attr("id", []string{"x", "y"}[rng.Intn(2)])
				case op < 11:
					n.Attr("data-k", []any{"1", "2", nil}[rng.Intn(3)])
				case op < 13:
					n.Style([]string{"fill", "stroke", "font-size", "--v", "--w"}[rng.Intn(5)], []any{"red", "12px", "4px", nil}[rng.Intn(4)])
				case op < 14:
					n.Attr("fill", []any{"#abc", "var(--v)", nil}[rng.Intn(3)])
				case op < 15:
					n.Attr("font-size", []any{"15px", "2em", nil}[rng.Intn(3)])
				case op < 17:
					if n != root && n.Parent != nil {
						n.Remove()
					}
				case op < 19:
					// Insert in front of a sibling.
					if n.Parent != nil && n.Parent != svg {
						c := doc.CreateElementNS(dom.NSSVG, tags[rng.Intn(len(tags))])
						c.Attr("class", classes[rng.Intn(len(classes))])
						n.Parent.InsertBefore(c, n)
					}
				case op < 20:
					// Move an element to another parent, when that makes
					// no cycle.
					to := pick()
					inside := false
					for p := to; p != nil; p = p.Parent {
						if p == n {
							inside = true
						}
					}
					if !inside && n != root {
						to.AppendChild(n)
					}
				case op < 21:
					n.Raise()
				case op < 22:
					n.Text([]string{"", "text"}[rng.Intn(2)])
				case op < 23:
					// The style sheet changes.
					style.Text(cascadeSheets[rng.Intn(len(cascadeSheets))])
				default:
					// A second style element comes and goes.
					if extra := svg.Select("style.extra"); extra != nil {
						extra.Remove()
					} else {
						svg.Append("style").Attr("class", "extra").Text(`.a { fill: purple } g { stroke: pink }`)
					}
				}
				// Ask for some styles now, so that kept results are of
				// many different generations.
				for k := 0; k < 3; k++ {
					q := pick()
					e.ComputedStyle(q, cascadeProps[rng.Intn(len(cascadeProps))])
				}
				if step%4 != 3 {
					continue
				}
				got := cascadeSnapshot(e, svg)
				clone := NewDocumentWithFonts(fonts)
				copyOf := svg.CloneNode(true)
				clone.Body.AppendChild(adopt(clone, copyOf))
				want := cascadeSnapshot(clone.Engine.(*Engine), clone.Body.FirstChild)
				if got != want {
					gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
					for i := range wl {
						if i >= len(gl) || gl[i] != wl[i] {
							t.Fatalf("%s: the kept styles differ from a new engine:\n got  %s\n want %s", what, gl[min(i, len(gl)-1)], wl[i])
						}
					}
					t.Fatalf("%s: the kept styles differ from a new engine", what)
				}
			}
		}
	}
}

// adopt builds a copy of a tree in another document, with the same
// attributes and inline styles.
func adopt(doc *dom.Document, n *dom.Node) *dom.Node {
	switch n.Type {
	case dom.TextNode:
		return doc.CreateText(n.Data)
	case dom.CommentNode:
		return doc.CreateComment(n.Data)
	}
	c := doc.CreateElementNS(n.NS, n.Tag)
	for _, a := range n.Attrs {
		c.SetAttributeRaw(a.NS, a.Prefix, a.Local, a.Value)
	}
	for child := n.FirstChild; child != nil; child = child.Next {
		c.AppendChild(adopt(doc, child))
	}
	return c
}
