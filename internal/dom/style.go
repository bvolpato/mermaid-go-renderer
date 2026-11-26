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

// StyleStr is Style for a string value without boxing into any.
func (n *Node) StyleStr(property, value string) *Node {
	return n.styleStrPriority(property, value, false)
}

// StylePriority is selection.style(name, value, "important").
func (n *Node) StylePriority(property string, value any, important bool) *Node {
	if n == nil || n.Type != ElementNode {
		return n
	}
	if value == nil {
		return n.removeStyleProperty(property)
	}
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
	return n.styleStrPriority(property, s, important)
}

func (n *Node) loadStyleBlock(svg bool, allocForWrite bool) (block []css.Decl, has bool) {
	if n.hasPendingStyle && n.pendingStyleDirty {
		return n.styleBlock, true
	}
	var current string
	current, has = n.InlineStyle()
	if has && current == n.styleCanon && n.styleBlock != nil {
		// The CSSOM changes its parsed style. It does not read the text
		// again, so the values keep their digits.
		return n.styleBlock, true
	}
	var dst []css.Decl
	if allocForWrite && n.doc != nil && n.doc.arena != nil {
		dst = n.doc.arena.allocStyleDecls(8)
	}
	block = css.ParseInlineStyleInto(dst, current, svg && !(has && current == n.styleCanon))
	return block, has
}

func (n *Node) finishStyleChange(block []css.Decl, has bool) *Node {
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

func (n *Node) removeStyleProperty(property string) *Node {
	svg := n.NS == NSSVG
	block, has := n.loadStyleBlock(svg, false)
	var changed bool
	block, changed = css.RemoveProperty(block, property)
	if !changed {
		return n
	}
	return n.finishStyleChange(block, has)
}

func (n *Node) styleStrPriority(property, value string, important bool) *Node {
	if n == nil || n.Type != ElementNode {
		return n
	}
	svg := n.NS == NSSVG
	block, has := n.loadStyleBlock(svg, true)
	var changed bool
	block, changed = css.SetProperty(block, property, value, important, svg)
	if !changed {
		return n
	}
	return n.finishStyleChange(block, has)
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
