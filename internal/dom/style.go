// Ported from d3-selection 3.0.0, in part: the method selection.style of
// d3-selection/src/selection/style.js (ISC). The work of
// element.style.setProperty in the browser ports no upstream file.

package dom

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Style is selection.style(name, value): element.style.setProperty. The
// browser parses the whole style attribute into a declaration block, changes
// one property and writes the block back in CSSOM form. An unknown property
// or an invalid value changes nothing. A nil value removes the property.
func (n *Node) Style(property string, value any) *Node {
	return n.StylePriority(property, value, false)
}

// StylePriority is selection.style(name, value, "important").
func (n *Node) StylePriority(property string, value any, important bool) *Node {
	if n == nil || n.Type != ElementNode {
		return n
	}
	// Text that the CSSOM wrote is already canonical. The SVG rules for an
	// authored attribute (unitless lengths, !important last) apply only to
	// the first parse.
	svg := n.NS == NSSVG
	var block []css.Decl
	var has bool
	if n.hasPendingStyle && n.pendingStyleDirty {
		has = true
		block = n.styleBlock
	} else {
		var current string
		current, has = n.InlineStyle()
		if has && current == n.styleCanon && n.styleBlock != nil {
			// The CSSOM changes its parsed style. It does not read the text
			// again, so the values keep their digits.
			block = n.styleBlock
		} else {
			block = css.ParseInlineStyle(current, svg && !(has && current == n.styleCanon))
		}
	}
	if block == nil && value != nil && n.doc != nil && n.doc.arena != nil {
		block = n.doc.arena.allocStyleDecls(6)
	}
	var changed bool
	if value == nil {
		block, changed = css.RemoveProperty(block, property)
	} else {
		var s string
		switch v := value.(type) {
		case string:
			s = v
		case float64:
			s = js.Str(v)
		case int:
			s = js.Str(float64(v))
		default:
			s = js.ToString(v)
		}
		block, changed = css.SetProperty(block, property, s, important, svg)
	}
	if !changed {
		return n
	}
	// Blink writes the attribute even when the block becomes empty, if the
	// attribute existed. It creates the attribute when a property is set.
	if len(block) == 0 && !has {
		return n
	}
	n.styleBlock = block
	if n.hasPendingStyle || n.findAttr("", "style") < 0 {
		// No style attribute yet: defer serialization until the attribute or
		// InlineStyle() text is actually read.
		n.hasPendingStyle = true
		n.pendingStyleDirty = true
		n.touch()
		return n
	}
	text := css.SerializeInlineStyle(block)
	n.styleCanon = text
	n.setAttrNS("", "style", text)
	return n
}

// StyleValue is selection.style(name) as a getter on the inline style only.
func (n *Node) StyleValue(property string) string {
	if n == nil {
		return ""
	}
	if n.hasPendingStyle && n.pendingStyleDirty {
		for _, d := range n.styleBlock {
			if d.Name == property {
				return d.Value
			}
		}
		return ""
	}
	current, _ := n.InlineStyle()
	for _, d := range css.ParseInlineStyle(current, n.NS == NSSVG) {
		if d.Name == property {
			return d.Value
		}
	}
	return ""
}
