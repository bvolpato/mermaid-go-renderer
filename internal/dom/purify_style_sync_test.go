package dom

import "testing"

// TestDOMMethodsSyncStyle checks that the DOM-style attribute methods see an
// inline style that the CSSOM holds and that is not an attribute yet. Blink
// makes the style attribute when something reads it
// (Element::SynchronizeAttribute), so getAttribute and hasAttribute find
// it. removeAttribute follows Attr("style", nil).
func TestDOMMethodsSyncStyle(t *testing.T) {
	doc := NewDocument()
	el := doc.Body.Append("div")
	el.Style("color", "red")
	if !el.HasAttribute("style") {
		t.Errorf("HasAttribute(style) is false after Style")
	}
	if v, ok := el.GetAttribute("style"); !ok || v != "color: red;" {
		t.Errorf("GetAttribute(style) = %q, %v; want %q", v, ok, "color: red;")
	}
	// The attribute exists now, so removeAttribute removes it.
	el.RemoveAttribute("style")
	if v, ok := el.GetAttr("style"); ok {
		t.Errorf("the style attribute is %q after RemoveAttribute", v)
	}

	// A style that is still only in the CSSOM: the same result as Attr.
	a := doc.Body.Append("div")
	a.Style("color", "red")
	a.RemoveAttribute("style")
	b := doc.Body.Append("div")
	b.Style("color", "red")
	b.Attr("style", nil)
	if a.OuterHTML() != b.OuterHTML() {
		t.Errorf("RemoveAttribute(style) gives %s, Attr(style, nil) gives %s", a.OuterHTML(), b.OuterHTML())
	}
}
