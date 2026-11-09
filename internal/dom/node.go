// Ported from d3-selection 3.0.0, in part: d3-selection/src/namespaces.js,
// namespace.js and creator.js, and the methods of d3-selection/src/selection
// that the comments of this file name (ISC). The DOM tree itself ports no
// upstream file. One function, syncStyle, follows
// Element::SynchronizeStyleAttribute of Blink
// (third_party/blink/renderer/core/dom/element.cc of Chromium 156.0.8078.4,
// LGPL-2.0-or-later). See NOTICE in this directory.

// Package dom is a small DOM for the SVG and HTML that mermaid builds.
//
// The upstream renderers create elements with d3-selection, measure them with
// browser APIs, and read the result back with innerHTML. This package gives
// the ported renderers the same operations with the same observable behavior:
// namespace rules of d3 for element and attribute names, attribute order,
// CSSOM serialization of the style attribute, and the HTML serialization
// algorithm. Measurement (getBBox and friends) is delegated to an Engine.
package dom

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Namespace URIs.
const (
	NSHTML   = "http://www.w3.org/1999/xhtml"
	NSSVG    = "http://www.w3.org/2000/svg"
	NSMathML = "http://www.w3.org/1998/Math/MathML"
	NSXLink  = "http://www.w3.org/1999/xlink"
	NSXML    = "http://www.w3.org/XML/1998/namespace"
	NSXMLNS  = "http://www.w3.org/2000/xmlns/"
)

// NodeType is the kind of a Node.
type NodeType uint8

const (
	ElementNode NodeType = iota + 1
	TextNode
	CommentNode
	DocumentNode
)

// Attr is one attribute of an element.
type Attr struct {
	NS     string // namespace URI, "" for none
	Prefix string // namespace prefix, "" for none
	Local  string // local name
	Value  string
}

// QName is the qualified name: "prefix:local", or the local name.
func (a *Attr) QName() string {
	if a.Prefix == "" {
		return a.Local
	}
	return a.Prefix + ":" + a.Local
}

// Rect is a rectangle, as DOMRect and SVGRect are.
type Rect struct {
	X, Y, Width, Height float64
}

// Point is a 2D point.
type Point struct {
	X, Y float64
}

// Engine provides the measurements that need fonts and layout. The browser
// package implements it. A Document without an Engine returns zero values.
type Engine interface {
	// BBox is SVGGraphicsElement.getBBox().
	BBox(n *Node) Rect
	// ClientRect is Element.getBoundingClientRect().
	ClientRect(n *Node) Rect
	// ComputedTextLength is SVGTextContentElement.getComputedTextLength().
	ComputedTextLength(n *Node) float64
	// TotalLength is SVGGeometryElement.getTotalLength().
	TotalLength(n *Node) float64
	// PointAtLength is SVGGeometryElement.getPointAtLength().
	PointAtLength(n *Node, distance float64) Point
	// ComputedStyle is getComputedStyle(n).getPropertyValue(property).
	ComputedStyle(n *Node, property string) string
}

// Document owns a tree of nodes.
type Document struct {
	// Body is the <body> element. Renderers append their work under it.
	Body   *Node
	Engine Engine

	// BeforeMeasure, when it is set, runs before each measurement. The
	// renderer uses it to stop a render whose context is canceled.
	BeforeMeasure func()

	// PageSheets has the text of the style sheets of the page that are not
	// style elements of the tree: the link elements in the head of the
	// page, in document order. They come before every style element of the
	// body in the cascade. Set it before the first measurement, and do not
	// change it after that.
	PageSheets []string

	// gen counts mutations. Caches in the Engine compare it to know when
	// their content is stale.
	gen uint64
	// styleGen is the value of gen at the last change that can change the
	// text of a style element of the document. See StyleGeneration.
	styleGen uint64
}

// NewDocument returns an HTML document that has an empty body.
func NewDocument() *Document {
	d := &Document{}
	d.Body = d.CreateElementNS(NSHTML, "body")
	return d
}

// Generation changes every time the tree, an attribute or text changes.
func (d *Document) Generation() uint64 { return d.gen }

// StyleGeneration is the value of Generation at the last change that can
// change the style sheets of the document: a change of the children of a
// style element, and the insertion or the removal of a style element or of
// an element that can hold one. While it stays the same, the style elements
// of the document and their text are the same.
func (d *Document) StyleGeneration() uint64 { return d.styleGen }

// Node is an element, a text node or a comment.
type Node struct {
	Type NodeType
	NS   string // namespace of an element
	Tag  string // local name of an element, in the case it was created with
	Data string // content of a text node or comment

	Attrs []Attr

	Parent, FirstChild, LastChild, Prev, Next *Node

	doc *Document

	// Cache is scratch space for the Engine (computed style, layout). The
	// Engine is responsible for its validity.
	Cache any

	// styleCanon is the style attribute text that the last Style call wrote.
	styleCanon string
	// styleBlock is the parsed inline style of the last Style call. It is
	// valid while the style text equals styleCanon. It keeps what the text
	// loses: the full digits of numbers (css.ExactValue).
	styleBlock []css.Decl

	// pendingStyle is an inline style that the CSSOM holds and that is not
	// an attribute yet. Blink creates the style attribute lazily: a style
	// that is set with element.style on an element without the attribute
	// becomes an attribute only when something reads the attributes (for
	// example serialization or getAttribute("style")). Attributes that are
	// set in between come first in the attribute order.
	pendingStyle    string
	hasPendingStyle bool

	// The stamps below are values of Document.gen. They say when the node
	// last changed in a way that a selector can see, so that the style
	// cascade of the Engine can keep a result while its inputs are the
	// same. See the methods of the same names.
	selfGen      uint64
	kidsGen      uint64
	kidsOrderGen uint64
}

// SelfGeneration is the value of Document.Generation at the last change of
// the node itself: of an attribute or of the inline style, and of its place
// in the tree (the node was inserted, moved or removed).
func (n *Node) SelfGeneration() uint64 { return n.selfGen }

// ChildrenGeneration is the value of Document.Generation at the last change
// of the list of children of the node.
func (n *Node) ChildrenGeneration() uint64 { return n.kidsGen }

// ChildOrderGeneration is the value of Document.Generation at the last
// change of the list of children that can change the position of a child
// that stays: a child was removed, or a child was inserted in front of
// another one. An append to the end does not change it.
func (n *Node) ChildOrderGeneration() uint64 { return n.kidsOrderGen }

// syncStyle is Element::SynchronizeStyleAttribute: it turns a pending
// inline style into the style attribute.
func (n *Node) syncStyle() {
	if !n.hasPendingStyle {
		return
	}
	n.hasPendingStyle = false
	n.Attrs = append(n.Attrs, Attr{Local: "style", Value: n.pendingStyle})
	n.pendingStyle = ""
}

// InlineStyleExact is InlineStyle for layout: a number that element.style
// received keeps all its digits, where the attribute text has 6 significant
// digits. A browser lays out from the parsed style, not from the text.
func (n *Node) InlineStyleExact() (string, bool) {
	text, ok := n.InlineStyle()
	if ok && n.styleBlock != nil && text == n.styleCanon {
		return css.SerializeInlineStyleExact(n.styleBlock), true
	}
	return text, ok
}

// InlineStyle returns the text of the inline style, from the attribute or
// from the pending CSSOM state. It does not create the attribute. The
// layout engine reads the style with it, as the style engine of a browser
// reads the style object and not the attribute.
func (n *Node) InlineStyle() (string, bool) {
	if n == nil || n.Type != ElementNode {
		return "", false
	}
	if n.hasPendingStyle {
		return n.pendingStyle, true
	}
	for i := range n.Attrs {
		if n.Attrs[i].NS == "" && n.Attrs[i].Prefix == "" && n.Attrs[i].Local == "style" {
			return n.Attrs[i].Value, true
		}
	}
	return "", false
}

// Document returns the owner document.
func (n *Node) Document() *Document { return n.doc }

// touch records a change of an attribute or of the inline style of n.
func (n *Node) touch() {
	if n.doc != nil {
		n.doc.gen++
		n.selfGen = n.doc.gen
	}
}

// canHoldStyle reports whether the subtree of n can have a style element:
// n is one, or n has children. The test is cheap and errs on the safe side.
func (n *Node) canHoldStyle() bool {
	return n.Type == ElementNode && (n.Tag == "style" || n.FirstChild != nil)
}

// inStyle reports whether n or an ancestor of n is a style element.
func (n *Node) inStyle() bool {
	for p := n; p != nil; p = p.Parent {
		if p.Type == ElementNode && p.Tag == "style" {
			return true
		}
	}
	return false
}

// CreateElementNS is document.createElementNS. The name can have a prefix,
// which is dropped: serialization of elements uses only the local name.
func (d *Document) CreateElementNS(ns, qname string) *Node {
	local := qname
	if i := strings.IndexByte(qname, ':'); i >= 0 {
		local = qname[i+1:]
	}
	return &Node{Type: ElementNode, NS: ns, Tag: local, doc: d}
}

// CreateElement is document.createElement in an HTML document: the element is
// in the HTML namespace and the name is lowercased.
func (d *Document) CreateElement(name string) *Node {
	return &Node{Type: ElementNode, NS: NSHTML, Tag: strings.ToLower(name), doc: d}
}

// CreateText is document.createTextNode.
func (d *Document) CreateText(data string) *Node {
	return &Node{Type: TextNode, Data: data, doc: d}
}

// CreateComment is document.createComment.
func (d *Document) CreateComment(data string) *Node {
	return &Node{Type: CommentNode, Data: data, doc: d}
}

// d3Namespaces is the prefix table of d3-selection (namespaces.js).
var d3Namespaces = map[string]string{
	"svg":   NSSVG,
	"xhtml": NSHTML,
	"xlink": NSXLink,
	"xml":   NSXML,
	"xmlns": NSXMLNS,
}

// d3Namespace is namespace() of d3-selection. It returns the namespace URI
// and the local name. The URI is "" when the name has no known prefix. An
// unknown prefix is dropped, as d3 does.
func d3Namespace(name string) (space, local string) {
	prefix := name
	i := strings.IndexByte(name, ':')
	if i >= 0 {
		prefix = name[:i]
		if prefix != "xmlns" {
			name = name[i+1:]
		}
	}
	if uri, ok := d3Namespaces[prefix]; ok {
		return uri, name
	}
	return "", name
}

// create is creator() of d3-selection, called with n as the parent.
func (n *Node) create(name string) *Node {
	space, local := d3Namespace(name)
	if space != "" {
		return n.doc.CreateElementNS(space, local)
	}
	if n.NS == NSHTML {
		return n.doc.CreateElement(local)
	}
	return n.doc.CreateElementNS(n.NS, local)
}

// IsElement reports whether n is an element with the given local name.
func (n *Node) IsElement(tag string) bool {
	return n != nil && n.Type == ElementNode && n.Tag == tag
}

// ---- tree mutation ----

func (n *Node) unlink() {
	p := n.Parent
	if p == nil {
		return
	}
	if n.Prev != nil {
		n.Prev.Next = n.Next
	} else {
		p.FirstChild = n.Next
	}
	if n.Next != nil {
		n.Next.Prev = n.Prev
	} else {
		p.LastChild = n.Prev
	}
	n.Parent, n.Prev, n.Next = nil, nil, nil
}

// AppendChild is Node.appendChild. A child that has a parent is moved.
func (n *Node) AppendChild(c *Node) *Node {
	return n.InsertBefore(c, nil)
}

// InsertBefore is Node.insertBefore. A nil ref appends.
func (n *Node) InsertBefore(c, ref *Node) *Node {
	if c == ref {
		return c
	}
	from := c.Parent
	c.unlink()
	c.Parent = n
	if ref == nil {
		c.Prev = n.LastChild
		if n.LastChild != nil {
			n.LastChild.Next = c
		} else {
			n.FirstChild = c
		}
		n.LastChild = c
	} else {
		c.Next = ref
		c.Prev = ref.Prev
		if ref.Prev != nil {
			ref.Prev.Next = c
		} else {
			n.FirstChild = c
		}
		ref.Prev = c
	}
	if d := n.doc; d != nil {
		d.gen++
		// The child has a new place. Its parent has another list of
		// children, and so has the parent that it came from.
		c.selfGen, n.kidsGen = d.gen, d.gen
		if ref != nil {
			n.kidsOrderGen = d.gen
		}
		if from != nil {
			from.kidsGen, from.kidsOrderGen = d.gen, d.gen
		}
		if c.canHoldStyle() || n.inStyle() || (from != nil && from.inStyle()) {
			d.styleGen = d.gen
		}
	}
	return c
}

// RemoveChildren removes every child.
func (n *Node) RemoveChildren() {
	d := n.doc
	if d != nil {
		d.gen++
		n.kidsGen, n.kidsOrderGen = d.gen, d.gen
		if n.inStyle() {
			d.styleGen = d.gen
		}
	}
	for c := n.FirstChild; c != nil; {
		next := c.Next
		c.Parent, c.Prev, c.Next = nil, nil, nil
		if d != nil {
			c.selfGen = d.gen
			if c.canHoldStyle() {
				d.styleGen = d.gen
			}
		}
		c = next
	}
	n.FirstChild, n.LastChild = nil, nil
}

// Remove is selection.remove(): it detaches the node from its parent.
func (n *Node) Remove() *Node {
	if n == nil {
		return nil
	}
	from := n.Parent
	n.unlink()
	if d := n.doc; d != nil {
		d.gen++
		n.selfGen = d.gen
		if from != nil {
			from.kidsGen, from.kidsOrderGen = d.gen, d.gen
		}
		if n.canHoldStyle() || (from != nil && from.inStyle()) {
			d.styleGen = d.gen
		}
	}
	return n
}

// Append is selection.append(name). The new element inherits the namespace
// of n unless the name has a prefix that d3 knows ("svg:", "xhtml:").
func (n *Node) Append(name string) *Node {
	if n == nil {
		return nil
	}
	return n.AppendChild(n.create(name))
}

// Insert is selection.insert(name, before). The before selector is evaluated
// with querySelector on n. An empty selector, or no match, appends.
func (n *Node) Insert(name string, before ...string) *Node {
	if n == nil {
		return nil
	}
	var ref *Node
	if len(before) > 0 && before[0] != "" {
		ref = n.Select(before[0])
	}
	return n.InsertBefore(n.create(name), ref)
}

// Lower is selection.lower(): the node becomes the first child of its parent.
func (n *Node) Lower() *Node {
	if n != nil && n.Prev != nil {
		p := n.Parent
		p.InsertBefore(n, p.FirstChild)
	}
	return n
}

// Raise is selection.raise(): the node becomes the last child of its parent.
func (n *Node) Raise() *Node {
	if n != nil && n.Next != nil {
		n.Parent.AppendChild(n)
	}
	return n
}

// ---- attributes ----

func (n *Node) findAttr(space, local string) int {
	if space != "" {
		for i := range n.Attrs {
			if n.Attrs[i].NS == space && n.Attrs[i].Local == local {
				return i
			}
		}
		return -1
	}
	// getAttribute matches the qualified name. In an HTML document the name
	// is lowercased first for an HTML element.
	if n.NS == NSHTML {
		local = strings.ToLower(local)
	}
	for i := range n.Attrs {
		a := &n.Attrs[i]
		if a.Prefix == "" {
			if a.Local == local {
				return i
			}
		} else if a.QName() == local {
			return i
		}
	}
	return -1
}

// GetAttr is selection.attr(name) as a getter. The second result is false
// when the attribute does not exist (d3 returns null).
func (n *Node) GetAttr(name string) (string, bool) {
	if n == nil || n.Type != ElementNode {
		return "", false
	}
	space, local := d3Namespace(name)
	if space == "" && local == "style" {
		n.syncStyle()
	}
	if i := n.findAttr(space, local); i >= 0 {
		return n.Attrs[i].Value, true
	}
	return "", false
}

// AttrOr returns the attribute value, or def when the attribute is absent.
func (n *Node) AttrOr(name, def string) string {
	if v, ok := n.GetAttr(name); ok {
		return v
	}
	return def
}

// HasAttr reports whether the attribute exists.
func (n *Node) HasAttr(name string) bool {
	_, ok := n.GetAttr(name)
	return ok
}

// Attr is selection.attr(name, value). The value is converted with the
// JavaScript String() rules: a float64 prints as a JS number. A nil value
// removes the attribute, as null does in d3. The receiver is returned so that
// calls can be chained. A nil receiver is an empty selection: nothing happens.
func (n *Node) Attr(name string, value any) *Node {
	if n == nil || n.Type != ElementNode {
		return n
	}
	space, local := d3Namespace(name)
	if space == "" && local == "style" {
		if n.hasPendingStyle && value == nil {
			// removeAttribute("style") on a style that the CSSOM holds
			// leaves an empty style attribute in Chromium. The element
			// has no style attribute to remove, so Blink only empties
			// the inline style. The empty style stays in the CSSOM, and
			// becomes an attribute when the attributes are next read.
			n.pendingStyle = ""
			n.styleCanon = ""
			n.styleBlock = nil
			n.touch()
			return n
		}
		n.syncStyle()
		// The attribute is set from text: the parsed style is replaced.
		n.styleBlock = nil
	}
	if value == nil {
		if i := n.findAttr(space, local); i >= 0 {
			n.Attrs = append(n.Attrs[:i], n.Attrs[i+1:]...)
			n.touch()
		}
		return n
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
	n.setAttrNS(space, local, s)
	return n
}

// setAttrNS is setAttributeNS(space, qname, value), or setAttribute(qname,
// value) when space is "".
func (n *Node) setAttrNS(space, qname, value string) {
	if i := n.findAttr(space, attrLocal(space, qname)); i >= 0 {
		n.Attrs[i].Value = value
		n.touch()
		return
	}
	a := Attr{NS: space, Local: qname, Value: value}
	if space != "" {
		if i := strings.IndexByte(qname, ':'); i >= 0 {
			a.Prefix, a.Local = qname[:i], qname[i+1:]
		}
	} else if n.NS == NSHTML {
		a.Local = strings.ToLower(qname)
	}
	n.Attrs = append(n.Attrs, a)
	n.touch()
}

func attrLocal(space, qname string) string {
	if space != "" {
		if i := strings.IndexByte(qname, ':'); i >= 0 {
			return qname[i+1:]
		}
	}
	return qname
}

// SetAttributeRaw sets an attribute with an explicit namespace, prefix and
// local name. Parsers use it to keep the exact attribute identity.
func (n *Node) SetAttributeRaw(ns, prefix, local, value string) {
	for i := range n.Attrs {
		a := &n.Attrs[i]
		if a.NS == ns && a.Local == local && a.Prefix == prefix {
			a.Value = value
			n.touch()
			return
		}
	}
	n.Attrs = append(n.Attrs, Attr{NS: ns, Prefix: prefix, Local: local, Value: value})
	n.touch()
}

// ID returns the id attribute.
func (n *Node) ID() string {
	v, _ := n.plainAttr("id")
	return v
}

// plainAttr is GetAttr for a name that has no prefix and is not style: the
// attribute that has the name and no prefix. The style cascade asks for the
// id and the class of an element for every rule that it tries, so these two
// do not take the way of GetAttr through the name rules of d3.
func (n *Node) plainAttr(local string) (string, bool) {
	if n == nil || n.Type != ElementNode {
		return "", false
	}
	for i := range n.Attrs {
		if a := &n.Attrs[i]; a.Prefix == "" && a.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// ClassList returns the class names, split as DOMTokenList does.
func (n *Node) ClassList() []string {
	v, ok := n.plainAttr("class")
	if !ok {
		return nil
	}
	return strings.FieldsFunc(v, isASCIISpace)
}

// AppendClasses appends the class names of the element to list.
func (n *Node) AppendClasses(list []string) []string {
	v, _ := n.plainAttr("class")
	for i := 0; i < len(v); {
		for i < len(v) && isASCIISpaceByte(v[i]) {
			i++
		}
		start := i
		for i < len(v) && !isASCIISpaceByte(v[i]) {
			i++
		}
		if i > start {
			list = append(list, v[start:i])
		}
	}
	return list
}

func isASCIISpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

func isASCIISpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\f' || r == '\r'
}

// HasClass reports whether the element has the class.
func (n *Node) HasClass(name string) bool {
	v, ok := n.plainAttr("class")
	if !ok {
		return false
	}
	// The names between runs of white space. After white space at the end
	// the name is empty, as it is for the loop that trims and then cuts.
	for i := 0; i < len(v); {
		for i < len(v) && isASCIISpaceByte(v[i]) {
			i++
		}
		start := i
		for i < len(v) && !isASCIISpaceByte(v[i]) {
			i++
		}
		if v[start:i] == name {
			return true
		}
	}
	return false
}

// Classed is selection.classed(names, value). d3 rewrites the class
// attribute as the names joined with single spaces.
func (n *Node) Classed(names string, on bool) *Node {
	if n == nil || n.Type != ElementNode {
		return n
	}
	// d3 splits on white space after trim: classArray().
	current := strings.Fields(strings.TrimSpace(n.AttrOr("class", "")))
	if strings.TrimSpace(n.AttrOr("class", "")) == "" {
		current = nil
	}
	for _, name := range strings.Fields(strings.TrimSpace(names)) {
		idx := -1
		for i, c := range current {
			if c == name {
				idx = i
				break
			}
		}
		switch {
		case on && idx < 0:
			current = append(current, name)
			n.setAttrNS("", "class", strings.Join(current, " "))
		case !on && idx >= 0:
			current = append(current[:idx], current[idx+1:]...)
			n.setAttrNS("", "class", strings.Join(current, " "))
		}
	}
	return n
}

// ---- content ----

// Text is selection.text(value): it replaces the children with one text
// node. An empty string leaves the element with no children.
func (n *Node) Text(value string) *Node {
	if n == nil {
		return nil
	}
	n.RemoveChildren()
	if value != "" {
		n.AppendChild(n.doc.CreateText(value))
	}
	return n
}

// TextContent is Node.textContent.
func (n *Node) TextContent() string {
	if n == nil {
		return ""
	}
	switch n.Type {
	case TextNode:
		return n.Data
	case CommentNode:
		return ""
	}
	if n.FirstChild != nil && n.FirstChild == n.LastChild && n.FirstChild.Type == TextNode {
		return n.FirstChild.Data
	}
	var sb strings.Builder
	var walk func(*Node)
	walk = func(p *Node) {
		for c := p.FirstChild; c != nil; c = c.Next {
			switch c.Type {
			case TextNode:
				sb.WriteString(c.Data)
			case ElementNode:
				walk(c)
			}
		}
	}
	walk(n)
	return sb.String()
}

// Children returns the element children (Element.children).
func (n *Node) Children() []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == ElementNode {
			out = append(out, c)
		}
	}
	return out
}

// FirstElementChild is Element.firstElementChild.
func (n *Node) FirstElementChild() *Node {
	if n == nil {
		return nil
	}
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == ElementNode {
			return c
		}
	}
	return nil
}

// ---- measurement, delegated to the Engine ----

// BBox is getBBox().
func (n *Node) BBox() Rect {
	if n == nil || n.doc == nil || n.doc.Engine == nil {
		return Rect{}
	}
	if n.doc.BeforeMeasure != nil {
		n.doc.BeforeMeasure()
	}
	return n.doc.Engine.BBox(n)
}

// BoundingClientRect is getBoundingClientRect().
func (n *Node) BoundingClientRect() Rect {
	if n == nil || n.doc == nil || n.doc.Engine == nil {
		return Rect{}
	}
	if n.doc.BeforeMeasure != nil {
		n.doc.BeforeMeasure()
	}
	return n.doc.Engine.ClientRect(n)
}

// ComputedTextLength is getComputedTextLength().
func (n *Node) ComputedTextLength() float64 {
	if n == nil || n.doc == nil || n.doc.Engine == nil {
		return 0
	}
	if n.doc.BeforeMeasure != nil {
		n.doc.BeforeMeasure()
	}
	return n.doc.Engine.ComputedTextLength(n)
}

// TotalLength is getTotalLength().
func (n *Node) TotalLength() float64 {
	if n == nil || n.doc == nil || n.doc.Engine == nil {
		return 0
	}
	return n.doc.Engine.TotalLength(n)
}

// PointAtLength is getPointAtLength().
func (n *Node) PointAtLength(distance float64) Point {
	if n == nil || n.doc == nil || n.doc.Engine == nil {
		return Point{}
	}
	return n.doc.Engine.PointAtLength(n, distance)
}

// ComputedStyle is getComputedStyle(n).getPropertyValue(property).
func (n *Node) ComputedStyle(property string) string {
	if n == nil || n.doc == nil || n.doc.Engine == nil {
		return ""
	}
	return n.doc.Engine.ComputedStyle(n, property)
}

// InsertNode is selection.insert(() => node, before): it inserts an
// existing node, where Insert creates one. The before selector is evaluated
// with querySelector on n. An empty selector, or no match, appends.
func (n *Node) InsertNode(c *Node, before ...string) *Node {
	if n == nil || c == nil {
		return nil
	}
	var ref *Node
	if len(before) > 0 && before[0] != "" {
		ref = n.Select(before[0])
	}
	return n.InsertBefore(c, ref)
}

// AppendNode is selection.append(() => node): it appends an existing node.
func (n *Node) AppendNode(c *Node) *Node {
	if n == nil || c == nil {
		return nil
	}
	return n.AppendChild(c)
}
