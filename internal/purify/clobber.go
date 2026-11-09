// Ported from dompurify 3.4.12, src/purify.ts: _isClobbered and the DOM
// clobbering check of _isValidAttribute (MPL-2.0 OR Apache-2.0).

package purify

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

// clobberProbe lists the properties that _isClobbered reads from a form. An
// HTMLFormElement returns a form control for a property that has the name or
// the id of the control, in place of the DOM property.
var clobberProbe = map[string]bool{
	"nodeName": true, "textContent": true, "removeChild": true, "attributes": true,
	"removeAttribute": true, "setAttribute": true, "namespaceURI": true, "insertBefore": true,
	"hasChildNodes": true, "nodeType": true, "childNodes": true,
}

// isHTMLForm reports whether n is an HTMLFormElement.
func isHTMLForm(n *dom.Node) bool {
	return n.Type == dom.ElementNode && n.NS == htmlNamespace && n.Tag == "form"
}

// isClobbered is _isClobbered. Only an element whose node name is form is
// checked. An element named form in another namespace has no named
// properties, so it is never clobbered.
func (p *Purifier) isClobbered(element *dom.Node) bool {
	if !isHTMLForm(element) {
		return false
	}
	return formNamedCount(element, func(name string) bool { return clobberProbe[name] }, 1) > 0
}

// isListed reports whether n is a listed element: a form control that is in
// form.elements.
func isListed(n *dom.Node) bool {
	if n.Type != dom.ElementNode || n.NS != htmlNamespace {
		return false
	}
	switch n.Tag {
	case "button", "fieldset", "input", "object", "output", "select", "textarea":
		return true
	}
	return false
}

// namesProperty reports whether n gives its form a named property that
// match accepts. The named properties of a form come from the listed
// elements, except an input in the Image Button state, and from img
// elements. The name is the value of the id or of the name attribute.
func namesProperty(n *dom.Node, match func(string) bool) bool {
	if n.Tag == "input" {
		if t, ok := n.GetAttribute("type"); ok && strings.EqualFold(t, "image") {
			return false
		}
	}
	if v, ok := n.GetAttribute("id"); ok && v != "" && match(v) {
		return true
	}
	if v, ok := n.GetAttribute("name"); ok && v != "" && match(v) {
		return true
	}
	return false
}

// treeRoot is the root of the tree of n. The contents of a template are a
// separate tree.
func treeRoot(n *dom.Node) *dom.Node {
	for n.Parent != nil && !n.Parent.IsTemplate() {
		n = n.Parent
	}
	return n
}

// isConnected reports whether n is in the document, and not in a detached
// subtree or in the contents of a template.
func isConnected(n *dom.Node) bool {
	root := treeRoot(n)
	return root.Parent == nil && root.Type == dom.ElementNode && root.NS == htmlNamespace && root.Tag == "html"
}

// elementByID is getElementById on the tree of root: the first element in
// tree order that has the id.
func elementByID(root *dom.Node, id string) *dom.Node {
	if id == "" {
		return nil
	}
	var found *dom.Node
	var walk func(n *dom.Node) bool
	walk = func(n *dom.Node) bool {
		if n.Type == dom.ElementNode {
			if v, ok := n.GetAttribute("id"); ok && v == id {
				found = n
				return true
			}
		}
		for c := firstChild(n); c != nil; c = c.Next {
			if c.Type == dom.ElementNode && walk(c) {
				return true
			}
		}
		return false
	}
	walk(root)
	return found
}

// formNamedCount counts the controls that give a form a named property whose
// name match accepts. It stops at limit. A form returns the control for such
// a property when there is one control, and a RadioNodeList when there are
// more.
//
// The form owner of a control is computed from the tree, as the "reset the
// form owner" algorithm of HTML does: a connected listed element that has a
// form attribute belongs to the first element that has that id, when it is a
// form. Every other control, and every img, belongs to its nearest form
// ancestor.
//
// Known divergence: the HTML parser can also associate a control with a form
// that is not an ancestor (the form element pointer, for example in
// `<div><form></div><input name=attributes>`). That association is not
// visible in the tree, and is not reproduced here.
func formNamedCount(form *dom.Node, match func(string) bool, limit int) int {
	count := 0
	connected := isConnected(form)
	root := treeRoot(form)
	// ownedByAttr reports whether the form attribute of n selects the form.
	// The second result is false when the attribute does not apply.
	ownedByAttr := func(n *dom.Node) (owned, applies bool) {
		if !connected || !isListed(n) {
			return false, false
		}
		id, ok := n.GetAttribute("form")
		if !ok {
			return false, false
		}
		return elementByID(root, id) == form, true
	}
	// Controls under the form, up to a nested form.
	var walk func(n *dom.Node) bool
	walk = func(n *dom.Node) bool {
		for c := firstChild(n); c != nil; c = c.Next {
			if c.Type != dom.ElementNode {
				continue
			}
			if c.NS == htmlNamespace && (isListed(c) || c.Tag == "img") {
				owned, applies := ownedByAttr(c)
				if (!applies || owned) && namesProperty(c, match) {
					count++
					if count >= limit {
						return true
					}
				}
			}
			if isHTMLForm(c) {
				continue
			}
			if walk(c) {
				return true
			}
		}
		return false
	}
	if walk(form) {
		return count
	}
	// Controls outside of the form that name the form in a form attribute.
	if !connected {
		return count
	}
	id, ok := form.GetAttribute("id")
	if !ok || id == "" || elementByID(root, id) != form {
		return count
	}
	inForm := func(n *dom.Node) bool {
		for a := n.Parent; a != nil; a = a.Parent {
			if isHTMLForm(a) {
				return a == form
			}
		}
		return false
	}
	var scan func(n *dom.Node) bool
	scan = func(n *dom.Node) bool {
		for c := firstChild(n); c != nil; c = c.Next {
			if c.Type != dom.ElementNode {
				continue
			}
			// The first walk counted the controls whose nearest form
			// ancestor is the form.
			if isListed(c) && !inForm(c) {
				if v, has := c.GetAttribute("form"); has && v == id && namesProperty(c, match) {
					count++
					if count >= limit {
						return true
					}
				}
			}
			if scan(c) {
				return true
			}
		}
		return false
	}
	scan(root)
	return count
}

// formNamed is formNamedCount for one property name.
func formNamed(form *dom.Node, name string, limit int) int {
	return formNamedCount(form, func(v string) bool { return v == name }, limit)
}
