// Ported from Chromium 156.0.8078.4 (Blink), in part: the boxes of the
// pseudo-elements ::before and ::after. The functions of this file follow
// the functions of Blink that their comments name:
//
//   - PseudoElementLayoutObjectIsNeeded and PseudoElement::AttachLayoutTree:
//     core/dom/pseudo_element.cc (BSD-3-Clause, with the copyright of Google
//     in the file).
//   - Content::ApplyValue: core/css/properties/longhands/longhands_custom.cc
//     (BSD-3-Clause).
//   - ComputedStyleUtils::ValueForLineHeight and ValueForContentData:
//     core/css/properties/computed_style_utils.cc (BSD-3-Clause).
//
// The comments also name Element::CanGeneratePseudoElement of
// core/dom/element.cc and TextContentData::CreateLayoutObject of
// core/style/content_data.cc (both LGPL-2.0-or-later). The paths are in
// third_party/blink/renderer. See NOTICE in this directory.

package browser

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// This file has the generated content of CSS: the pseudo-elements ::before
// and ::after with the property content. The style sheets of Font Awesome
// draw an icon with them: the class of the icon sets a custom property to
// the character of the icon, and a rule for ::before has
// content: var(--fa).
//
// A pseudo-element with a box is a node that the engine makes: an element
// in the namespace pseudoNS whose parent is the originating element, with
// one text node for the strings of the content. The node is not a child of
// the originating element in the DOM: the tree, its serialization and the
// selectors do not see it. The layout takes it as the first child (::before)
// or the last child (::after) of the originating element, as Blink does
// with its PseudoElement and the LayoutTextFragment of the content
// (TextContentData::CreateLayoutObject).
//
// testdata/pagesheets-cases.json has the values of Chromium.

// pseudoKind is a pseudo-element that can have a box of generated content.
type pseudoKind uint8

const (
	pseudoNone pseudoKind = iota
	pseudoBefore
	pseudoAfter
)

// pseudoNS is the namespace of the nodes that stand for pseudo-elements. No
// element of a document has it.
const pseudoNS = "urn:mmdg:pseudo-element"

// pseudoKindOfName returns the kind of a pseudo-element name of a selector.
func pseudoKindOfName(name string) pseudoKind {
	switch name {
	case "before":
		return pseudoBefore
	case "after":
		return pseudoAfter
	}
	return pseudoNone
}

func (k pseudoKind) name() string {
	if k == pseudoAfter {
		return "after"
	}
	return "before"
}

// pseudoKindOf returns the kind of a node that stands for a pseudo-element,
// or pseudoNone for another node.
func pseudoKindOf(n *dom.Node) pseudoKind {
	if n == nil || n.NS != pseudoNS {
		return pseudoNone
	}
	return pseudoKindOfName(n.Tag)
}

// cascadePseudo is the cascade of a pseudo-element: the declarations of the
// rules whose selector has the pseudo-element and matches the originating
// element. The pseudo-element inherits from the originating element.
func (e *Engine) cascadePseudo(p *dom.Node, st *nodeStyle) {
	origin := p.Parent
	doc := p.Document()
	rs := e.collectRules(doc)
	st.rules = rs
	kind := pseudoKindOf(p)
	var ms []matched
	collect := func(set *ruleSet, base int) {
		for _, r := range set.pseudo {
			if r.pseudo != kind || !r.sel.MatchOriginating(origin) {
				continue
			}
			for i := range r.decls {
				d := &r.decls[i]
				ms = append(ms, matched{decl: d, spec: r.sel.Specificity(), order: base + r.order, important: d.Important})
			}
		}
	}
	if page := e.pageRules(doc); page != nil {
		collect(page, 0)
		collect(rs, page.count)
	} else {
		collect(rs, 0)
	}
	e.applyCascade(p, st, ms, nil)
}

// hasPseudoRule reports whether a rule for the pseudo-element matches the
// originating element.
func hasPseudoRule(set *ruleSet, n *dom.Node, kind pseudoKind) bool {
	if set == nil {
		return false
	}
	for _, r := range set.pseudo {
		if r.pseudo == kind && r.sel.MatchOriginating(n) {
			return true
		}
	}
	return false
}

// newPseudoNode makes the node that stands for a pseudo-element of n. It
// has no content yet.
func newPseudoNode(n *dom.Node, kind pseudoKind) *dom.Node {
	p := n.Document().CreateElementNS(pseudoNS, kind.name())
	// The node has a parent and is not in the list of its children.
	p.Parent = n
	return p
}

// contentOf returns the computed value of the property content of an
// element or of a pseudo-element. The property does not inherit.
func (e *Engine) contentOf(n *dom.Node) css.Content {
	v, ok := e.declared(n, "content")
	if !ok {
		return css.Content{Kind: css.ContentNormal}
	}
	c, ok := css.ParseContent(v)
	if !ok {
		return css.Content{Kind: css.ContentNormal}
	}
	// attr() reads the attribute of the element, or of the originating
	// element of a pseudo-element.
	el := n
	if pseudoKindOf(n) != pseudoNone {
		el = n.Parent
	}
	return c.WithAttrs(func(name string) string {
		v, _ := el.GetAttr(name)
		return v
	})
}

// pseudoBoxes returns the nodes of the pseudo-elements ::before and ::after
// of an element that have a box, or nil.
//
// Element::CanGeneratePseudoElement and PseudoElementLayoutObjectIsNeeded:
// a pseudo-element has a box when its display is not none and its content
// is not none. The content normal counts as none for ::before and ::after.
// An SVG element has no box for them: its layout object takes no such
// child. An element that takes no children, such as br and img, has none.
func (e *Engine) pseudoBoxes(n *dom.Node) (before, after *dom.Node) {
	if n.Type != dom.ElementNode || n.NS != dom.NSHTML {
		return nil, nil
	}
	doc := n.Document()
	rs := e.collectRules(doc)
	page := e.pageRules(doc)
	if len(rs.pseudo) == 0 && (page == nil || len(page.pseudo) == 0) {
		// No style sheet has a rule for a pseudo-element.
		return nil, nil
	}
	if n.Tag == "br" || isImage(n) {
		return nil, nil
	}
	st := e.styleOf(n)
	if st.pseudo == nil {
		before := e.makePseudoBox(n, pseudoBefore, rs, page)
		after := e.makePseudoBox(n, pseudoAfter, rs, page)
		if before == nil && after == nil {
			// Most elements have no pseudo-element with a box.
			st.pseudo = &noPseudoBoxes
		} else {
			st.pseudo = &pseudoBoxes{before: before, after: after}
		}
	}
	return st.pseudo.before, st.pseudo.after
}

// pseudoBoxes has the nodes of the pseudo-elements of an element that have
// a box.
type pseudoBoxes struct {
	before, after *dom.Node
}

// noPseudoBoxes is the value of an element with no box of a pseudo-element.
// Nothing writes to it.
var noPseudoBoxes pseudoBoxes

// makePseudoBox returns the node of a pseudo-element of n when it has a
// box, with its text.
func (e *Engine) makePseudoBox(n *dom.Node, kind pseudoKind, rs, page *ruleSet) *dom.Node {
	if !hasPseudoRule(page, n, kind) && !hasPseudoRule(rs, n, kind) {
		// No rule: the content is normal.
		return nil
	}
	p := newPseudoNode(n, kind)
	if e.display(p) == "none" {
		return nil
	}
	content := e.contentOf(p)
	if content.Kind != css.ContentItems {
		return nil
	}
	// PseudoElement::AttachLayoutTree makes one child for each item of the
	// content. Content::ApplyValue joined the strings that follow each
	// other. The engine has the strings only: a counter, a quote and an
	// image make no child here.
	if text, _ := content.Text(); text != "" {
		t := n.Document().CreateText(text)
		t.Parent = p
		p.FirstChild, p.LastChild = t, t
	}
	return p
}

// ComputedPseudoStyle is getComputedStyle(n, pseudo).getPropertyValue(
// property) for the pseudo-elements "::before" and "::after". It returns ""
// for another pseudo-element.
func (e *Engine) ComputedPseudoStyle(n *dom.Node, pseudo, property string) string {
	if n == nil || n.Type != dom.ElementNode {
		return ""
	}
	kind := pseudoKindOfName(strings.TrimLeft(strings.ToLower(pseudo), ":"))
	if kind == pseudoNone {
		return ""
	}
	// The style of a pseudo-element exists also when it has no box.
	p := newPseudoNode(n, kind)
	if before, after := e.pseudoBoxes(n); kind == pseudoBefore && before != nil {
		p = before
	} else if kind == pseudoAfter && after != nil {
		p = after
	}
	return e.ComputedStyle(p, property)
}

// computedExtra is ComputedStyle for the properties that the tests of the
// page sheets read: custom properties, content, line-height, text-align,
// vertical-align, width and height. The second result is false for another
// property.
func (e *Engine) computedExtra(n *dom.Node, property string) (string, bool) {
	if strings.HasPrefix(property, "--") {
		v, _ := e.customProperty(n, property, 0)
		return v, true
	}
	ts := e.textStyleOf(n)
	switch property {
	case "content":
		// ComputedStyleUtils::ValueForContentData: the content normal of a
		// pseudo-element prints as none.
		c := e.contentOf(n)
		if c.Kind == css.ContentNormal && pseudoKindOf(n) != pseudoNone {
			return "none", true
		}
		return c.CSSText(), true
	case "line-height":
		// ComputedStyleUtils::ValueForLineHeight: normal, or the length
		// in px. A number is a percentage of the computed font size.
		switch ts.lhKind {
		case lhNumber:
			return css.FormatNumber(float64(float32(float32(ts.lhValue*100)*float32(ts.size))/100)) + "px", true
		case lhLength:
			return css.FormatNumber(ts.lhValue) + "px", true
		}
		return "normal", true
	case "text-align":
		return ts.textAlign, true
	case "vertical-align":
		v, ok := e.declared(n, "vertical-align")
		if !ok {
			return "baseline", true
		}
		v = strings.ToLower(strings.TrimSpace(v))
		if px, ok := e.cssLength(v, ts, false); ok && !strings.HasSuffix(v, "%") {
			return css.FormatNumber(float64(float32(px))) + "px", true
		}
		return v, true
	case "width", "height":
		// The used size of the content box of a box. An inline box and an
		// element with no box have auto.
		if n.NS != dom.NSHTML || foreignObjectOf(n) == nil || !isBlockLevelDisplay(e.display(n)) {
			// The property does not apply: the computed value is the
			// length of the declaration.
			if v, ok := e.declared(n, property); ok && !strings.HasSuffix(strings.TrimSpace(v), "%") {
				if px, ok := e.cssLength(v, ts, false); ok {
					return css.FormatNumber(float64(float32(px))) + "px", true
				}
			}
			return "auto", true
		}
		_, _, w, h, ok := e.htmlBoxRect(n)
		if !ok {
			return "auto", true
		}
		pad := e.edges(n, "padding", "", ts, 0)
		border := e.borderEdges(n, ts)
		if property == "width" {
			return css.FormatNumber((w - pad.horizontal() - border.horizontal()).px()) + "px", true
		}
		return css.FormatNumber((h - pad.vertical() - border.vertical()).px()) + "px", true
	}
	return "", false
}
