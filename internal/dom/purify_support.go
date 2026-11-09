// This file ports no upstream file. It has the DOM operations that the
// DOMPurify port needs, as the DOM standard and the HTML standard define
// them. Its comments name parts of Blink, at Chromium 156.0.8078.4:
//
//   - Element::SynchronizeAttribute and Element::CloneAttributesFrom:
//     third_party/blink/renderer/core/dom/element.cc (LGPL-2.0-or-later).
//     Two functions of this file synchronize the style attribute at the
//     same place as these functions.
//   - The node iterator follows the DOM standard. The class of Blink for it
//     is NodeIterator in third_party/blink/renderer/core/dom/node_iterator.cc
//     (LGPL-2.0-or-later). This file does not say that it follows that
//     Blink file.
//
// See NOTICE in this directory.

package dom

import (
	"errors"
	"slices"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"golang.org/x/net/html"
)

// This file adds the DOM operations that the DOMPurify port (internal/purify)
// needs: the attribute methods of Element with DOM semantics, cloneNode, a
// NodeIterator, the parse that DOMParser does, and the serialization of a
// document that has no browsing context.
//
// Template contents: a browser moves the children that the parser gives to an
// HTML template element into a separate DocumentFragment (template.content).
// This package keeps them as children of the template element. The functions
// in this file that follow DOM tree order treat those children as a separate
// tree, as a browser does.

// IsTemplate reports whether n is an HTML template element. Its children are
// the template contents.
func (n *Node) IsTemplate() bool {
	return n != nil && n.Type == ElementNode && n.NS == NSHTML && n.Tag == "template"
}

// ---- attributes with DOM semantics ----
//
// GetAttr and Attr resolve a prefix as d3-selection does. The methods below
// are the plain DOM methods: they match the qualified name of the attribute.

// ErrInvalidCharacter is the DOM InvalidCharacterError.
var ErrInvalidCharacter = errors.New("dom: InvalidCharacterError")

// ErrNamespace is the DOM NamespaceError.
var ErrNamespace = errors.New("dom: NamespaceError")

// GetAttribute is Element.getAttribute(qualifiedName). The second result is
// false when the attribute does not exist (the DOM returns null).
func (n *Node) GetAttribute(name string) (string, bool) {
	if n == nil || n.Type != ElementNode {
		return "", false
	}
	if name == "style" {
		// Element::SynchronizeAttribute: a read of the style attribute
		// makes the attribute from the inline style that the CSSOM holds.
		n.syncStyle()
	}
	if i := n.findAttr("", name); i >= 0 {
		return n.Attrs[i].Value, true
	}
	return "", false
}

// HasAttribute is Element.hasAttribute(qualifiedName).
func (n *Node) HasAttribute(name string) bool {
	_, ok := n.GetAttribute(name)
	return ok
}

// validAttributeLocalName is the "valid attribute local name" check of the DOM
// standard, which Chromium applies in setAttribute: the name is not empty
// and has no ASCII white space, U+0000, "/", "=" or ">".
func validAttributeLocalName(name string) bool {
	if name == "" {
		return false
	}
	return !strings.ContainsAny(name, "\t\n\f\r \x00/=>")
}

// validNamespacePrefix is the "valid namespace prefix" check of the DOM
// standard.
func validNamespacePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	return !strings.ContainsAny(prefix, "\t\n\f\r \x00/>")
}

// SetAttribute is Element.setAttribute(qualifiedName, value). An existing
// attribute keeps its position. A new attribute goes to the end.
func (n *Node) SetAttribute(name, value string) error {
	if n == nil || n.Type != ElementNode {
		return nil
	}
	if !validAttributeLocalName(name) {
		return ErrInvalidCharacter
	}
	n.setAttrNS("", name, value)
	return nil
}

// SetAttributeNS is Element.setAttributeNS(namespace, qualifiedName, value).
// An empty namespace is the null namespace.
func (n *Node) SetAttributeNS(ns, qname, value string) error {
	if n == nil || n.Type != ElementNode {
		return nil
	}
	// Validate and extract (https://dom.spec.whatwg.org/#validate-and-extract).
	prefix, local := "", qname
	hasPrefix := false
	if i := strings.IndexByte(qname, ':'); i >= 0 {
		prefix, local = qname[:i], qname[i+1:]
		hasPrefix = true
		if !validNamespacePrefix(prefix) {
			return ErrInvalidCharacter
		}
	}
	if !validAttributeLocalName(local) {
		return ErrInvalidCharacter
	}
	switch {
	case hasPrefix && ns == "":
		return ErrNamespace
	case prefix == "xml" && ns != NSXML:
		return ErrNamespace
	case (qname == "xmlns" || prefix == "xmlns") && ns != NSXMLNS:
		return ErrNamespace
	case ns == NSXMLNS && qname != "xmlns" && prefix != "xmlns":
		return ErrNamespace
	}
	if ns == "" {
		// The null namespace matches by local name, with no lowercasing.
		for i := range n.Attrs {
			a := &n.Attrs[i]
			if a.NS == "" && a.Local == local {
				a.Value = value
				n.touch()
				return nil
			}
		}
		n.Attrs = append(n.Attrs, Attr{Local: local, Value: value})
		n.touch()
		return nil
	}
	n.setAttrNS(ns, qname, value)
	return nil
}

// RemoveAttribute is Element.removeAttribute(qualifiedName). It removes the
// first attribute that has the qualified name.
func (n *Node) RemoveAttribute(name string) {
	if n == nil || n.Type != ElementNode {
		return
	}
	if name == "style" {
		// The inline style goes away with the attribute: Attr has the
		// rules for a style that the CSSOM holds.
		n.Attr("style", nil)
		return
	}
	if i := n.findAttr("", name); i >= 0 {
		n.Attrs = append(n.Attrs[:i], n.Attrs[i+1:]...)
		n.touch()
	}
}

// ---- cloning ----

// CloneNode is Node.cloneNode(deep). The clone has no parent. A deep clone of
// a template includes the template contents, as the cloning steps of the
// template element do.
//
// Blink synchronizes the style attribute of the source before it copies the
// attributes (Element::CloneAttributesFrom). So an inline style that the
// CSSOM holds becomes an attribute of the source here, and the clone has it.
func (n *Node) CloneNode(deep bool) *Node {
	if n == nil {
		return nil
	}
	n.syncStyle()
	c := &Node{Type: n.Type, NS: n.NS, Tag: n.Tag, Data: n.Data, doc: n.doc, styleCanon: n.styleCanon}
	if n.styleBlock != nil {
		// The clone gets a copy of the parsed inline style.
		c.styleBlock = append([]css.Decl(nil), n.styleBlock...)
	}
	if len(n.Attrs) > 0 {
		c.Attrs = make([]Attr, len(n.Attrs))
		copy(c.Attrs, n.Attrs)
	}
	if deep {
		for child := n.FirstChild; child != nil; child = child.Next {
			cc := child.CloneNode(true)
			cc.Parent = c
			cc.Prev = c.LastChild
			if c.LastChild != nil {
				c.LastChild.Next = cc
			} else {
				c.FirstChild = cc
			}
			c.LastChild = cc
		}
	}
	return c
}

// ---- NodeIterator ----

// Bits of NodeFilter for the whatToShow argument of a NodeIterator.
const (
	ShowElement uint32 = 0x1
	ShowText    uint32 = 0x4
	ShowComment uint32 = 0x80
	ShowAll     uint32 = 0xFFFFFFFF
)

// NodeIterator is the DOM NodeIterator
// (https://dom.spec.whatwg.org/#interface-nodeiterator). A browser keeps
// every iterator of a document up to date when a node is removed. This
// package has no such registry: the code that removes a node while an
// iterator is active must call PreRemove first.
type NodeIterator struct {
	root *Node
	// fragment is set when the root is a template element that stands for its
	// content fragment. The root is then not a node of the collection that
	// whatToShow can accept, and the children of the root are visited.
	fragment bool
	ref      *Node
	before   bool
	show     uint32
}

// NewNodeIterator is document.createNodeIterator(root, whatToShow, null).
func NewNodeIterator(root *Node, whatToShow uint32) *NodeIterator {
	return &NodeIterator{root: root, ref: root, before: true, show: whatToShow}
}

// NewContentIterator is document.createNodeIterator(template.content,
// whatToShow, null): it visits the contents of an HTML template element.
func NewContentIterator(template *Node, whatToShow uint32) *NodeIterator {
	return &NodeIterator{root: template, fragment: true, ref: template, before: true, show: whatToShow}
}

// Root is the root of the iterator.
func (it *NodeIterator) Root() *Node { return it.root }

// firstChild is the first child of n in the tree of the iterator. Template
// contents are not part of the tree, except under a fragment root.
func (it *NodeIterator) firstChild(n *Node) *Node {
	if n.IsTemplate() && !(it.fragment && n == it.root) {
		return nil
	}
	return n.FirstChild
}

func (it *NodeIterator) lastChild(n *Node) *Node {
	if n.IsTemplate() && !(it.fragment && n == it.root) {
		return nil
	}
	return n.LastChild
}

// following is the first node that follows n in tree order and is an
// inclusive descendant of the root.
func (it *NodeIterator) following(n *Node) *Node {
	if c := it.firstChild(n); c != nil {
		return c
	}
	return it.followingSkip(n)
}

// followingSkip is following, without the descendants of n.
func (it *NodeIterator) followingSkip(n *Node) *Node {
	for n != nil && n != it.root {
		if n.Next != nil {
			return n.Next
		}
		n = n.Parent
	}
	return nil
}

// inTree reports whether n is an inclusive descendant of the root.
func (it *NodeIterator) inTree(n *Node) bool {
	for ; n != nil; n = n.Parent {
		if n == it.root {
			return true
		}
	}
	return false
}

func (it *NodeIterator) accept(n *Node) bool {
	if it.fragment && n == it.root {
		// A DocumentFragment. No caller asks for SHOW_DOCUMENT_FRAGMENT.
		return false
	}
	switch n.Type {
	case ElementNode:
		return it.show&ShowElement != 0
	case TextNode:
		return it.show&ShowText != 0
	case CommentNode:
		return it.show&ShowComment != 0
	}
	return false
}

// NextNode is NodeIterator.nextNode(). It returns nil at the end.
func (it *NodeIterator) NextNode() *Node {
	node, before := it.ref, it.before
	// A caller that removed the reference node without PreRemove leaves the
	// iterator outside of its tree. A browser cannot get into this state.
	if node == nil || !it.inTree(node) {
		return nil
	}
	for {
		if !before {
			node = it.following(node)
			if node == nil {
				return nil
			}
		} else {
			before = false
		}
		if it.accept(node) {
			break
		}
	}
	it.ref, it.before = node, before
	return node
}

// PreRemove runs the NodeIterator pre-removing steps
// (https://dom.spec.whatwg.org/#nodeiterator-pre-removing-steps). Call it
// before toBeRemoved is detached from its parent.
func (it *NodeIterator) PreRemove(toBeRemoved *Node) {
	if toBeRemoved == nil || toBeRemoved == it.root {
		return
	}
	ancestor := false
	for x := it.ref; x != nil; x = x.Parent {
		if x == toBeRemoved {
			ancestor = true
			break
		}
		if x == it.root {
			break
		}
	}
	if !ancestor {
		return
	}
	if it.before {
		if next := it.followingSkip(toBeRemoved); next != nil {
			it.ref = next
			return
		}
		it.before = false
	}
	if toBeRemoved.Prev == nil {
		it.ref = toBeRemoved.Parent
		return
	}
	n := toBeRemoved.Prev
	for c := it.lastChild(n); c != nil; c = it.lastChild(n) {
		n = c
	}
	it.ref = n
}

// ---- DOMParser ----

// HTMLDocument is the result of ParseHTMLDocument.
type HTMLDocument struct {
	// Document owns the nodes. Document.Body is the body element when the
	// markup produced one.
	Document *Document
	// Root is the html element (document.documentElement).
	Root *Node
	// Body is the first body element of the document, or nil for a frameset
	// document (document.getElementsByTagName("body")[0]).
	Body *Node
	// Frameset is the frameset element of a frameset document, or nil.
	Frameset *Node
	// DoctypeName is the name of the doctype. HasDoctype is false when the
	// markup has no doctype.
	DoctypeName string
	HasDoctype  bool
}

// ParseHTMLDocument parses markup as new DOMParser().parseFromString(markup,
// "text/html") does. The resulting document has no browsing context, so the
// scripting flag of the parser is off: the content of a noscript element is
// parsed as markup. ParseBody parses with the scripting flag on.
//
// The parser of golang.org/x/net/html rejects markup that nests more than 512
// elements. Chromium accepts it and flattens the deeper levels.
func ParseHTMLDocument(markup string) (*HTMLDocument, error) {
	// The markup that this module writes itself needs no error recovery:
	// see parse_simple.go.
	if doc, ok := parseSimpleDocument(markup); ok {
		if checkSimpleParser {
			checkSimpleDocument(markup, doc)
		}
		return doc, nil
	}
	if checkSimpleParser {
		simpleParserFull.Add(1)
	}
	return parseHTMLDocumentFull(markup)
}

// parseHTMLDocumentFull is ParseHTMLDocument by the parser of
// golang.org/x/net/html.
func parseHTMLDocumentFull(markup string) (*HTMLDocument, error) {
	root, err := html.ParseWithOptions(strings.NewReader(markup), html.ParseOptionEnableScripting(false))
	if err != nil {
		return nil, err
	}
	fixParserDivergences(markup, root)
	d := &Document{}
	out := &HTMLDocument{Document: d}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.DoctypeNode:
			if !out.HasDoctype {
				out.HasDoctype = true
				out.DoctypeName = c.Data
			}
		case html.ElementNode:
			if out.Root == nil {
				out.Root = d.fromHTMLNode(c)
			}
		}
	}
	if out.Root == nil {
		out.Root = d.CreateElementNS(NSHTML, "html")
	}
	for c := out.Root.FirstChild; c != nil; c = c.Next {
		if c.Type != ElementNode || c.NS != NSHTML {
			continue
		}
		if c.Tag == "body" && out.Body == nil {
			out.Body = c
		}
		if c.Tag == "frameset" && out.Frameset == nil && out.Body == nil {
			out.Frameset = c
		}
	}
	if out.Body != nil {
		d.Body = out.Body
	} else {
		d.Body = d.CreateElementNS(NSHTML, "body")
	}
	return out, nil
}

// ---- repairs of the tree of golang.org/x/net/html ----

// formattingElements are the elements that the HTML parser keeps in the list
// of active formatting elements.
var formattingElements = map[string]bool{
	"a": true, "b": true, "big": true, "code": true, "em": true, "font": true, "i": true,
	"nobr": true, "s": true, "small": true, "strike": true, "strong": true, "tt": true, "u": true,
}

// chromiumSVGTagNames maps the name that golang.org/x/net/html gives to an
// SVG element to the name that Chromium gives it, where the two differ.
// Chromium adjusts the case of the SVG tag names that Blink implements. The
// table of x/net/html is an older copy of the table of the HTML standard.
var chromiumSVGTagNames = map[string]string{
	"fedropshadow": "feDropShadow",
}

// fixParserDivergences changes the tree that golang.org/x/net/html built into
// the tree that Chromium builds, for the differences that can be repaired
// after the parse:
//
//   - x/net/html sorts the attributes of a formatting element (a, b, i, em,
//     strong, font and the others of formattingElements) by name. A browser
//     keeps the order of the source.
//   - x/net/html keeps U+0000 in an attribute name or value. The tokenizer of
//     the HTML standard replaces it with U+FFFD.
//   - x/net/html does not know every SVG tag name whose case Chromium adjusts.
//   - x/net/html gives the attribute xml:base of a foreign element the XML
//     namespace. Chromium gives it no namespace.
//
// ParseBody and Node.HTML make these repairs too.
func fixParserDivergences(markup string, root *html.Node) {
	var formatting []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			for i := range n.Attr {
				a := &n.Attr[i]
				if strings.IndexByte(a.Val, 0) >= 0 {
					a.Val = strings.ReplaceAll(a.Val, "\x00", "\uFFFD")
				}
				if strings.IndexByte(a.Key, 0) >= 0 {
					a.Key = strings.ReplaceAll(a.Key, "\x00", "\uFFFD")
				}
				// x/net/html moves xml:base to the XML namespace, as an older
				// HTML standard did. Chromium keeps it in no namespace.
				if a.Namespace == "xml" && a.Key == "base" {
					a.Namespace, a.Key = "", "xml:base"
				}
			}
			switch n.Namespace {
			case "":
				if len(n.Attr) > 1 && formattingElements[n.Data] {
					formatting = append(formatting, n)
				}
			case "svg":
				if name, ok := chromiumSVGTagNames[n.Data]; ok {
					n.Data = name
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if len(formatting) > 0 {
		restoreAttrOrder(markup, formatting)
	}
}

// attrSignature identifies a start tag by its name and its set of
// attributes, in any order.
func attrSignature(tag string, attrs []html.Attribute) string {
	sorted := slices.Clone(attrs)
	slices.SortFunc(sorted, func(a, b html.Attribute) int {
		if c := strings.Compare(a.Key, b.Key); c != 0 {
			return c
		}
		return strings.Compare(a.Val, b.Val)
	})
	var sb strings.Builder
	sb.WriteString(tag)
	for _, a := range sorted {
		sb.WriteByte(0)
		sb.WriteString(a.Key)
		sb.WriteByte(1)
		sb.WriteString(a.Val)
	}
	return sb.String()
}

// restoreAttrOrder gives the attributes of the formatting elements the order
// of the source again. The parser does not say which start tag made which
// element, so the start tags are read again with the tokenizer and matched
// by tag name and attribute set. Every element of a formatting element,
// including the copies that the parser makes when it reconstructs the active
// formatting elements, has the attributes of one start tag.
//
// The tokenizer modes of the parser depend on the tree (raw text and CDATA
// are different in foreign content), so the markup is read in each
// combination of modes and the start tags of all passes are used. When two
// start tags have the same name and attribute set in a different order, the
// elements take the orders in sequence. That case is a guess.
func restoreAttrOrder(markup string, nodes []*html.Node) {
	orders := map[string][][]html.Attribute{}
	seen := map[string]int{}
	for pass := 0; pass < 4; pass++ {
		rawText, cdata := pass&1 == 0, pass&2 != 0
		counts := map[string]int{}
		z := html.NewTokenizer(strings.NewReader(markup))
		z.AllowCDATA(cdata)
		for {
			tt := z.Next()
			if tt == html.ErrorToken {
				break
			}
			if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
				continue
			}
			tok := z.Token()
			// The parser reads the content of noscript as markup when
			// scripting is off.
			if !rawText || tok.Data == "noscript" {
				z.NextIsNotRawText()
			}
			if len(tok.Attr) < 2 || !formattingElements[tok.Data] {
				continue
			}
			for i := range tok.Attr {
				a := &tok.Attr[i]
				a.Val = strings.ReplaceAll(a.Val, "\x00", "\uFFFD")
				a.Key = strings.ReplaceAll(a.Key, "\x00", "\uFFFD")
			}
			sig := attrSignature(tok.Data, tok.Attr)
			counts[sig]++
			// A later pass adds only the start tags that the earlier passes
			// did not see as often.
			if counts[sig] > seen[sig] {
				orders[sig] = append(orders[sig], tok.Attr)
				seen[sig] = counts[sig]
			}
		}
	}
	used := map[string]int{}
	for _, n := range nodes {
		sig := attrSignature(n.Data, n.Attr)
		list := orders[sig]
		if len(list) == 0 {
			continue
		}
		i := used[sig]
		if i >= len(list) {
			i = len(list) - 1
		}
		used[sig]++
		n.Attr = slices.Clone(list[i])
	}
}

// ---- serialization of an inert document ----

// InnerHTMLInert is Element.innerHTML for a node of a document that has no
// browsing context (a DOMParser document, template contents). Scripting is
// off in such a document, so Chromium escapes the text of a noscript element.
// In all other respects it is InnerHTML.
func (n *Node) InnerHTMLInert() string {
	w := getHTMLWriter()
	for c := n.FirstChild; c != nil; c = c.Next {
		c.serializeInert(w)
	}
	return w.finish()
}

// OuterHTMLInert is Element.outerHTML for a node of a document that has no
// browsing context. See InnerHTMLInert.
func (n *Node) OuterHTMLInert() string {
	w := getHTMLWriter()
	n.serializeInert(w)
	return w.finish()
}

func (n *Node) serializeInert(sb *htmlWriter) {
	switch n.Type {
	case TextNode:
		p := n.Parent
		if p != nil && p.Type == ElementNode && p.NS == NSHTML && p.Tag != "noscript" && rawTextElements[p.Tag] {
			sb.WriteString(n.Data)
		} else {
			escapeText(sb, n.Data)
		}
	case ElementNode:
		sb.WriteByte('<')
		sb.WriteString(n.Tag)
		for i := range n.Attrs {
			a := &n.Attrs[i]
			sb.WriteByte(' ')
			sb.WriteString(a.serializedName())
			sb.WriteString(`="`)
			escapeAttr(sb, a.Value)
			sb.WriteByte('"')
		}
		sb.WriteByte('>')
		if n.NS == NSHTML && voidElements[n.Tag] {
			return
		}
		for c := n.FirstChild; c != nil; c = c.Next {
			c.serializeInert(sb)
		}
		sb.WriteString("</")
		sb.WriteString(n.Tag)
		sb.WriteByte('>')
	default:
		n.serialize(sb)
	}
}

// ---- tree dump for the parser goldens ----

// DumpTree prints the tree under n in the format of the parser goldens
// (testdata/purify_parse.json.gz, written by tools/reference/gen-purify.mjs).
// The format shows what the serialization hides: the namespace of every
// element and attribute, the template contents, and the split of text nodes.
//
//	element    <N:name attr="value" attr{A}="value">children</>
//	template   <h:template>{contents}</>
//	text       "data"
//	comment    <!--"data"-->
//
// N is h, s or m for the HTML, SVG and MathML namespaces. A is x, X or n for
// the XLink, XML and XMLNS namespaces of an attribute. A backslash, a double
// quote and a control character below U+0020 are escaped in a value.
func (n *Node) DumpTree() string {
	var sb strings.Builder
	n.dumpTree(&sb)
	return sb.String()
}

func dumpEscape(sb *strings.Builder, s string) {
	const hex = "0123456789abcdef"
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\':
			sb.WriteString(`\\`)
		case c == '"':
			sb.WriteString(`\"`)
		case c < 0x20:
			sb.WriteString(`\u00`)
			sb.WriteByte(hex[c>>4])
			sb.WriteByte(hex[c&0xf])
		default:
			sb.WriteByte(c)
		}
	}
}

func (n *Node) dumpTree(sb *strings.Builder) {
	switch n.Type {
	case ElementNode:
		sb.WriteByte('<')
		switch n.NS {
		case NSHTML:
			sb.WriteByte('h')
		case NSSVG:
			sb.WriteByte('s')
		case NSMathML:
			sb.WriteByte('m')
		default:
			sb.WriteByte('?')
		}
		sb.WriteByte(':')
		sb.WriteString(n.Tag)
		for i := range n.Attrs {
			a := &n.Attrs[i]
			sb.WriteByte(' ')
			sb.WriteString(a.QName())
			switch a.NS {
			case "":
			case NSXLink:
				sb.WriteString("{x}")
			case NSXML:
				sb.WriteString("{X}")
			case NSXMLNS:
				sb.WriteString("{n}")
			default:
				sb.WriteString("{?}")
			}
			sb.WriteString(`="`)
			dumpEscape(sb, a.Value)
			sb.WriteByte('"')
		}
		sb.WriteByte('>')
		if n.IsTemplate() {
			sb.WriteByte('{')
		}
		for c := n.FirstChild; c != nil; c = c.Next {
			c.dumpTree(sb)
		}
		if n.IsTemplate() {
			sb.WriteByte('}')
		}
		sb.WriteString("</>")
	case TextNode:
		sb.WriteByte('"')
		dumpEscape(sb, n.Data)
		sb.WriteByte('"')
	case CommentNode:
		sb.WriteString(`<!--"`)
		dumpEscape(sb, n.Data)
		sb.WriteString(`"-->`)
	}
}
