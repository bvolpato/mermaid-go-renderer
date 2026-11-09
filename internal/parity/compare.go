// Package parity compares SVG output of this renderer with the output of
// upstream mermaid in Chromium.
package parity

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// Verdict is the result of a comparison.
type Verdict int

const (
	// Exact means that the two strings are identical.
	Exact Verdict = iota
	// Equivalent means the same elements, attributes and text, where
	// numbers can differ by at most the tolerance.
	Equivalent
	// Different means a structural difference or a number out of tolerance.
	Different
)

func (v Verdict) String() string {
	return [...]string{"exact", "equivalent", "different"}[v]
}

// Diff describes the first difference between two documents.
type Diff struct {
	Path string // location in the tree
	Got  string
	Want string
	// MaxDelta is the largest numeric difference that was seen.
	MaxDelta float64
}

func (d *Diff) String() string {
	return fmt.Sprintf("%s\n    got:  %s\n    want: %s", d.Path, clip(d.Got), clip(d.Want))
}

func clip(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// CompareSVG compares two SVG strings. tol is the absolute tolerance for
// numbers, in user units (px).
func CompareSVG(got, want string, tol float64) (Verdict, *Diff) {
	if got == want {
		return Exact, nil
	}
	gd, err1 := dom.ParseBody(got)
	wd, err2 := dom.ParseBody(want)
	if err1 != nil || err2 != nil {
		return Different, &Diff{Path: "parse", Got: fmt.Sprint(err1), Want: fmt.Sprint(err2)}
	}
	c := &comparer{tol: tol}
	if d := c.nodes(gd.Body, wd.Body, "body"); d != nil {
		d.MaxDelta = c.maxDelta
		return Different, d
	}
	return Equivalent, &Diff{MaxDelta: c.maxDelta}
}

type comparer struct {
	tol      float64
	maxDelta float64
}

func describe(n *dom.Node) string {
	if n == nil {
		return "(none)"
	}
	switch n.Type {
	case dom.TextNode:
		return "#text " + strconv.Quote(n.Data)
	case dom.CommentNode:
		return "#comment"
	}
	s := "<" + n.Tag
	if id := n.ID(); id != "" {
		s += " id=" + id
	}
	if cl := n.AttrOr("class", ""); cl != "" {
		s += " class=" + strconv.Quote(cl)
	}
	return s + ">"
}

func (c *comparer) nodes(g, w *dom.Node, path string) *Diff {
	gc, wc := g.FirstChild, w.FirstChild
	idx := 0
	for gc != nil || wc != nil {
		p := fmt.Sprintf("%s > %s[%d]", path, tagOf(wc, gc), idx)
		if gc == nil || wc == nil {
			return &Diff{Path: p + ": child count", Got: describe(gc), Want: describe(wc)}
		}
		if gc.Type != wc.Type {
			return &Diff{Path: p + ": node type", Got: describe(gc), Want: describe(wc)}
		}
		switch gc.Type {
		case dom.TextNode:
			if gc.Data != wc.Data {
				// The text of a style element must be identical.
				if g.Type == dom.ElementNode && g.Tag == "style" {
					return &Diff{Path: p + ": style text", Got: firstDiff(gc.Data, wc.Data), Want: firstDiff(wc.Data, gc.Data)}
				}
				if !c.values(gc.Data, wc.Data) {
					return &Diff{Path: p + ": text", Got: gc.Data, Want: wc.Data}
				}
			}
		case dom.ElementNode:
			if gc.Tag != wc.Tag || gc.NS != wc.NS {
				return &Diff{Path: p + ": element", Got: describe(gc), Want: describe(wc)}
			}
			if d := c.attrs(gc, wc, p); d != nil {
				return d
			}
			if d := c.nodes(gc, wc, p); d != nil {
				return d
			}
		}
		gc, wc = gc.Next, wc.Next
		idx++
	}
	return nil
}

func tagOf(a, b *dom.Node) string {
	for _, n := range []*dom.Node{a, b} {
		if n != nil {
			if n.Type == dom.ElementNode {
				s := n.Tag
				if id := n.ID(); id != "" {
					s += "#" + id
				} else if cl := n.AttrOr("class", ""); cl != "" {
					s += "." + strings.ReplaceAll(cl, " ", ".")
				}
				return s
			}
			return "#text"
		}
	}
	return "?"
}

func (c *comparer) attrs(g, w *dom.Node, path string) *Diff {
	if len(g.Attrs) != len(w.Attrs) {
		return &Diff{Path: path + ": attribute names", Got: attrNames(g), Want: attrNames(w)}
	}
	for i := range g.Attrs {
		ga, wa := &g.Attrs[i], &w.Attrs[i]
		if ga.QName() != wa.QName() || ga.NS != wa.NS {
			return &Diff{Path: path + ": attribute names", Got: attrNames(g), Want: attrNames(w)}
		}
		if ga.Value == wa.Value {
			continue
		}
		name := ga.QName()
		if name == "data-points" {
			gd, err1 := base64.StdEncoding.DecodeString(ga.Value)
			wd, err2 := base64.StdEncoding.DecodeString(wa.Value)
			if err1 == nil && err2 == nil && c.values(string(gd), string(wd)) {
				continue
			}
			return &Diff{Path: path + " @" + name + " (decoded)", Got: string(gd), Want: string(wd)}
		}
		if !c.values(ga.Value, wa.Value) {
			return &Diff{Path: path + " @" + name, Got: ga.Value, Want: wa.Value}
		}
	}
	return nil
}

func attrNames(n *dom.Node) string {
	names := make([]string, len(n.Attrs))
	for i := range n.Attrs {
		names[i] = n.Attrs[i].QName()
	}
	return strings.Join(names, " ")
}

// values compares two strings token by token: text must be equal, numbers
// can differ by the tolerance.
func (c *comparer) values(a, b string) bool {
	for {
		an, arest, atext := nextNumber(a)
		bn, brest, btext := nextNumber(b)
		if atext != btext {
			return false
		}
		if an == nil || bn == nil {
			return an == nil && bn == nil
		}
		delta := math.Abs(*an - *bn)
		if delta > c.maxDelta {
			c.maxDelta = delta
		}
		if delta > c.tol && delta > 1e-9*math.Max(math.Abs(*an), math.Abs(*bn)) {
			return false
		}
		a, b = arest, brest
	}
}

// nextNumber splits s at its first number: the text before it, the number,
// and the rest. The number is nil when s has none.
func nextNumber(s string) (num *float64, rest, text string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9') && !(c == '.' && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9') {
			continue
		}
		start := i
		// A sign belongs to the number when it is not part of a word.
		if start > 0 && s[start-1] == '-' && (start == 1 || !isWordByte(s[start-2])) {
			start--
		}
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && s[j] == '.' {
			k := j + 1
			for k < len(s) && s[k] >= '0' && s[k] <= '9' {
				k++
			}
			if k > j+1 {
				j = k
			}
		}
		if j < len(s) && (s[j] == 'e' || s[j] == 'E') {
			k := j + 1
			if k < len(s) && (s[k] == '+' || s[k] == '-') {
				k++
			}
			if k < len(s) && s[k] >= '0' && s[k] <= '9' {
				for k < len(s) && s[k] >= '0' && s[k] <= '9' {
					k++
				}
				j = k
			}
		}
		f, err := strconv.ParseFloat(s[start:j], 64)
		if err != nil {
			continue
		}
		return &f, s[j:], s[:start]
	}
	return nil, "", s
}

func isWordByte(c byte) bool {
	return c == '_' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// firstDiff returns a window of a around the first byte where a and b differ.
func firstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(0, i-60)
	hi := min(len(a), i+120)
	return fmt.Sprintf("@%d …%s…", i, a[lo:hi])
}
