package dom

import (
	"strings"
	"sync"
	"unicode/utf8"
)

// This file has a parser for the markup that this module writes itself: the
// serialized SVG of a diagram, and the simple HTML of a label. DOMPurify
// parses every text that it sanitizes, so each render parses its whole SVG
// one time more, and each label with markup too.
//
// parseSimpleDocument builds the tree that ParseHTMLDocument builds with the
// parser of golang.org/x/net/html, for the markup whose tree does not depend
// on the error recovery of the HTML standard: start tags with quoted
// attributes, end tags that match, text, and a small set of HTML elements
// with nesting that the standard does not rearrange. For any other markup
// it reports false, and the caller uses the full parser. So the full parser
// still defines every tree: this parser must give the same tree where it
// gives one. TestSimpleParserAgainstFullParser compares the two, and the
// build tag domcheck makes ParseHTMLDocument compare them on every call.
//
// The rules below follow the parser of x/net/html (parse.go, foreign.go) and
// the repairs of fixParserDivergences.

// simpleHTMLTag classifies the HTML elements that the parser reads.
type simpleHTMLTag uint8

const (
	// simpleOrdinary is an element that "any other start tag" inserts.
	simpleOrdinary simpleHTMLTag = iota + 1
	// simpleBlock closes an open p element first: the parser reads it only
	// when no p element is open.
	simpleBlock
	// simpleFormatting is an element of the list of active formatting
	// elements.
	simpleFormatting
	// simpleVoid has no content and no end tag.
	simpleVoid
)

func simpleHTMLTagKind(name string) simpleHTMLTag {
	switch name {
	case "span", "sub", "sup":
		return simpleOrdinary
	case "div", "p":
		return simpleBlock
	case "b", "i", "em", "strong", "u", "s", "small", "code", "a":
		return simpleFormatting
	case "br", "img":
		return simpleVoid
	}
	return 0
}

// isSVGBreakout reports whether name is a start tag that ends foreign content
// (the breakout table of the HTML standard, and font, which breaks out with
// some attributes). The parser does not read them inside SVG.
func isSVGBreakout(name string) bool {
	switch name {
	case "b", "big", "blockquote", "body", "br", "center", "code",
		"dd", "div", "dl", "dt", "em", "embed", "h1", "h2", "h3", "h4", "h5", "h6",
		"head", "hr", "i", "img", "li", "listing", "menu", "meta", "nobr", "ol", "p",
		"pre", "ruby", "s", "small", "span", "strong", "strike", "sub", "sup",
		"table", "tt", "u", "ul", "var", "font":
		return true
	}
	return false
}

// adjustSVGTag gives an SVG tag name its case per the HTML standard (and
// fixParserDivergences).
func adjustSVGTag(name string) string {
	if len(name) < 6 {
		return name
	}
	switch name {
	case "altglyph":
		return "altGlyph"
	case "altglyphdef":
		return "altGlyphDef"
	case "altglyphitem":
		return "altGlyphItem"
	case "animatecolor":
		return "animateColor"
	case "animatemotion":
		return "animateMotion"
	case "animatetransform":
		return "animateTransform"
	case "clippath":
		return "clipPath"
	case "feblend":
		return "feBlend"
	case "fecolormatrix":
		return "feColorMatrix"
	case "fecomponenttransfer":
		return "feComponentTransfer"
	case "fecomposite":
		return "feComposite"
	case "feconvolvematrix":
		return "feConvolveMatrix"
	case "fediffuselighting":
		return "feDiffuseLighting"
	case "fedisplacementmap":
		return "feDisplacementMap"
	case "fedistantlight":
		return "feDistantLight"
	case "fedropshadow":
		return "feDropShadow"
	case "feflood":
		return "feFlood"
	case "fefunca":
		return "feFuncA"
	case "fefuncb":
		return "feFuncB"
	case "fefuncg":
		return "feFuncG"
	case "fefuncr":
		return "feFuncR"
	case "fegaussianblur":
		return "feGaussianBlur"
	case "feimage":
		return "feImage"
	case "femerge":
		return "feMerge"
	case "femergenode":
		return "feMergeNode"
	case "femorphology":
		return "feMorphology"
	case "feoffset":
		return "feOffset"
	case "fepointlight":
		return "fePointLight"
	case "fespecularlighting":
		return "feSpecularLighting"
	case "fespotlight":
		return "feSpotLight"
	case "fetile":
		return "feTile"
	case "feturbulence":
		return "feTurbulence"
	case "foreignobject":
		return "foreignObject"
	case "glyphref":
		return "glyphRef"
	case "lineargradient":
		return "linearGradient"
	case "radialgradient":
		return "radialGradient"
	case "textpath":
		return "textPath"
	}
	return name
}

// simpleMaxOpen is the largest number of open elements for which the parser
// reads a start tag. The full parser rejects a document that has more than
// 512 open elements at one time, so it must decide for a document that comes
// near to that number. No diagram comes near to simpleMaxOpen.
const simpleMaxOpen = 256

// isFastCanonicalSVGTag reports whether tag is a common canonical SVG tag name
// (already matching adjustSVGTag(lowerASCII(tag)) and not an SVG breakout tag).
func isFastCanonicalSVGTag(tag string) bool {
	switch tag {
	case "g", "path", "rect", "text", "tspan", "line", "circle", "polygon",
		"polyline", "ellipse", "svg", "defs", "marker", "foreignObject",
		"clipPath", "linearGradient", "radialGradient", "stop", "filter",
		"feDropShadow", "feGaussianBlur", "feOffset", "feComposite", "feMerge",
		"feMergeNode", "feFlood", "title", "desc", "a", "image", "mask",
		"pattern", "symbol", "textPath", "switch", "metadata":
		return true
	}
	return false
}

// isFastCanonicalSVGAttr reports whether a is a common canonical SVG attribute
// with empty NS and Prefix that already matches adjustSVGAttr(lowerASCII(a.Local)).
func isFastCanonicalSVGAttr(a *Attr) bool {
	if a.NS != "" || a.Prefix != "" {
		return false
	}
	switch a.Local {
	case "d", "class", "id", "style", "transform", "x", "y", "width", "height",
		"rx", "ry", "fill", "stroke", "cx", "cy", "r", "x1", "y1", "x2", "y2",
		"points", "opacity", "text-anchor", "dominant-baseline", "alignment-baseline",
		"dy", "dx", "marker-end", "marker-start", "marker-mid", "stroke-width",
		"stroke-dasharray", "stroke-linecap", "stroke-linejoin", "stroke-opacity",
		"fill-opacity", "fill-rule", "font-size", "font-family", "font-weight",
		"font-style", "role", "clip-path", "filter", "visibility", "offset",
		"stop-color", "stop-opacity", "orient", "viewBox", "refX", "refY",
		"markerWidth", "markerHeight", "markerUnits", "preserveAspectRatio",
		"aria-roledescription", "data-look", "data-et", "data-id", "data-node",
		"data-points", "data-edge", "data-class", "data-state":
		return true
	}
	return false
}

// hasDirectSanitizeBadBytes reports whether s contains \r, NUL, or mermaid's
// entity placeholder sequences ("ﬂ°" = ef ac 82 c2 b0, "¶ß" = c2 b6 c3 9f).
func hasDirectSanitizeBadBytes(s string) bool {
	const (
		highBits = 0x8080808080808080
		lowMask  = 0x2020202020202020
	)
	i := 0
	for i+8 <= len(s) {
		w := uint64(s[i]) | uint64(s[i+1])<<8 | uint64(s[i+2])<<16 | uint64(s[i+3])<<24 |
			uint64(s[i+4])<<32 | uint64(s[i+5])<<40 | uint64(s[i+6])<<48 | uint64(s[i+7])<<56
		if ((w | (w - lowMask)) & highBits) == 0 {
			i += 8
			continue
		}
		break
	}
	for ; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c < 0x80 {
			continue
		}
		if c == '\r' || c == 0 {
			return true
		}
		if c == 0xef && i+4 < len(s) && s[i+1] == 0xac && s[i+2] == 0x82 && s[i+3] == 0xc2 && s[i+4] == 0xb0 {
			return true
		}
		if c == 0xc2 && i+3 < len(s) && s[i+1] == 0xb6 && s[i+2] == 0xc3 && s[i+3] == 0x9f {
			return true
		}
	}
	return false
}

// PrepareDirectSanitize reports whether root (an HTML container such as
// mermaid's enclosingDiv) already has the exact DOM tree that serializing
// root.InnerHTML(), running cleanUpSvgCode, and reparsing with
// ParseHTMLDocument would produce. When it returns true, it applies the
// minimal namespace normalizations that reparsing would perform (SVG-nested
// HTML <style> moved to NSSVG, HTML xmlns attribute NS cleared) so the caller
// can sanitize root in place without serializing and reparsing the full SVG.
func PrepareDirectSanitize(root *Node, checkMarkerEnd bool) bool {
	if root == nil || root.FirstChild == nil {
		return false
	}
	var styleBuf [4]*Node
	styleNodes := styleBuf[:0]
	type attrFix struct {
		node *Node
		idx  int
		ns   string
	}
	var fixBuf [16]attrFix
	attrFixes := fixBuf[:0]

	checkSVGAttrs := func(c *Node) bool {
		for i := range c.Attrs {
			a := &c.Attrs[i]
			if hasDirectSanitizeBadBytes(a.Value) {
				return false
			}
			if checkMarkerEnd && a.Local == "marker-end" &&
				strings.HasPrefix(a.Value, "url(") && !strings.HasPrefix(a.Value, "url(#") {
				return false
			}
			if !isFastCanonicalSVGAttr(a) {
				sn := a.serializedName()
				if sn == "" {
					return false
				}
				for k := 0; k < len(sn); k++ {
					if !isMarkupNameChar(sn[k]) {
						return false
					}
				}
				rep := Attr{Local: lowerASCII(sn)}
				adjustSVGAttr(&rep)
				if rep.Prefix != a.Prefix || rep.Local != a.Local {
					return false
				}
				if rep.NS != a.NS {
					if rep.NS == NSXMLNS && a.NS == "" && a.Local == "xmlns" && a.Prefix == "" {
						attrFixes = append(attrFixes, attrFix{node: c, idx: i, ns: NSXMLNS})
					} else {
						return false
					}
				}
			}
			for j := 0; j < i; j++ {
				if c.Attrs[j].Local == a.Local && c.Attrs[j].Prefix == a.Prefix {
					return false
				}
			}
		}
		return true
	}

	var walk func(n *Node, inSVG, inP, inA bool, depth int) bool
	walk = func(n *Node, inSVG, inP, inA bool, depth int) bool {
		if depth >= simpleMaxOpen {
			return false
		}
		for c := n.FirstChild; c != nil; c = c.Next {
			switch c.Type {
			case TextNode:
				if hasDirectSanitizeBadBytes(c.Data) {
					return false
				}
			case ElementNode:
				c.syncStyle()
				if inSVG {
					if c.NS == NSHTML && c.Tag == "style" {
						// Mermaid creates <style> via document.createElement("style")
						// inside <svg>. When serialized and reparsed, the HTML5 parser
						// puts it in NSSVG and parses its text as foreign content.
						if len(c.Attrs) > 0 {
							return false
						}
						for sc := c.FirstChild; sc != nil; sc = sc.Next {
							if sc.Type != TextNode || hasDirectSanitizeBadBytes(sc.Data) ||
								strings.IndexByte(sc.Data, '<') >= 0 ||
								strings.IndexByte(sc.Data, '&') >= 0 {
								return false
							}
						}
						styleNodes = append(styleNodes, c)
						continue
					}
					if c.NS != NSSVG || c.Tag == "" {
						return false
					}
					if !isFastCanonicalSVGTag(c.Tag) {
						for i := 0; i < len(c.Tag); i++ {
							if !isMarkupNameChar(c.Tag[i]) {
								return false
							}
						}
						lcTag := lowerASCII(c.Tag)
						if isSVGBreakout(lcTag) || adjustSVGTag(lcTag) != c.Tag {
							return false
						}
					}
					if !checkSVGAttrs(c) {
						return false
					}
					if c.Tag == "foreignObject" || c.Tag == "desc" || c.Tag == "title" {
						if !walk(c, false, false, false, depth+1) {
							return false
						}
					} else {
						if !walk(c, true, false, false, depth+1) {
							return false
						}
					}
				} else {
					if depth == 0 && c.NS == NSSVG && c.Tag == "svg" {
						if !checkSVGAttrs(c) {
							return false
						}
						if !walk(c, true, false, false, depth+1) {
							return false
						}
						continue
					}
					if c.NS != NSHTML {
						return false
					}
					kind := simpleHTMLTagKind(c.Tag)
					if kind == 0 {
						return false
					}
					if kind == simpleVoid && c.FirstChild != nil {
						return false
					}
					if kind == simpleBlock && inP {
						return false
					}
					if c.Tag == "a" && inA {
						return false
					}
					for i := range c.Attrs {
						a := &c.Attrs[i]
						if hasDirectSanitizeBadBytes(a.Value) {
							return false
						}
						if a.Prefix != "" || a.Local == "" {
							return false
						}
						if a.NS == "" {
							switch a.Local {
							case "class", "id", "style", "xmlns", "href", "target", "rel", "src", "alt", "title", "width", "height":
							default:
								for k := 0; k < len(a.Local); k++ {
									ch := a.Local[k]
									if !isMarkupNameChar(ch) || (ch >= 'A' && ch <= 'Z') {
										return false
									}
								}
							}
						} else if a.NS == NSXMLNS && a.Local == "xmlns" {
							attrFixes = append(attrFixes, attrFix{node: c, idx: i, ns: ""})
						} else {
							return false
						}
						for j := 0; j < i; j++ {
							if c.Attrs[j].Local == a.Local && c.Attrs[j].Prefix == a.Prefix {
								return false
							}
						}
					}
					if !walk(c, false, inP || c.Tag == "p", inA || c.Tag == "a", depth+1) {
						return false
					}
				}
			default:
				return false
			}
		}
		return true
	}

	if !walk(root, false, false, false, 0) {
		return false
	}
	for _, s := range styleNodes {
		s.NS = NSSVG
	}
	for _, fix := range attrFixes {
		fix.node.Attrs[fix.idx].NS = fix.ns
	}
	return true
}

// ParseArena holds reusable Node and Attr slabs for temporary documents
// parsed by ParseHTMLDocumentPooled (such as inside DOMPurify when ReturnDOM
// is false). Call Release when done with the document.
type ParseArena struct {
	nodeChunks [][]Node
	attrChunks [][]Attr
	nodeIdx    int
	attrIdx    int
	nodeOff    int
	attrOff    int
	stack      []*Node
	formatting []*Node
}

var parseArenaPool = sync.Pool{New: func() any { return &ParseArena{} }}

const maxPooledArenaChunks = 64

func acquireParseArena() *ParseArena {
	a := parseArenaPool.Get().(*ParseArena)
	a.nodeIdx, a.attrIdx = 0, 0
	a.nodeOff, a.attrOff = 0, 0
	a.stack = a.stack[:0]
	a.formatting = a.formatting[:0]
	return a
}

// Release clears all node/attr references in the arena and returns it to the
// pool. The HTMLDocument parsed with this arena must not be used after Release.
func (a *ParseArena) Release() {
	if a == nil {
		return
	}
	for i := 0; i <= a.nodeIdx && i < len(a.nodeChunks); i++ {
		end := len(a.nodeChunks[i])
		if i == a.nodeIdx {
			end = a.nodeOff
		}
		clear(a.nodeChunks[i][:end])
	}
	for i := 0; i <= a.attrIdx && i < len(a.attrChunks); i++ {
		end := len(a.attrChunks[i])
		if i == a.attrIdx {
			end = a.attrOff
		}
		clear(a.attrChunks[i][:end])
	}
	clear(a.stack)
	clear(a.formatting)
	if len(a.nodeChunks) <= maxPooledArenaChunks && len(a.attrChunks) <= maxPooledArenaChunks {
		parseArenaPool.Put(a)
	}
}

func (a *ParseArena) allocNode(doc *Document) *Node {
	if a.nodeIdx >= len(a.nodeChunks) {
		sz := 16
		if len(a.nodeChunks) > 0 {
			sz = min(len(a.nodeChunks[len(a.nodeChunks)-1])*2, 256)
		}
		a.nodeChunks = append(a.nodeChunks, make([]Node, sz))
		a.nodeOff = 0
	}
	chunk := a.nodeChunks[a.nodeIdx]
	if a.nodeOff >= len(chunk) {
		a.nodeIdx++
		a.nodeOff = 0
		return a.allocNode(doc)
	}
	n := &chunk[a.nodeOff]
	a.nodeOff++
	n.doc = doc
	return n
}

func (a *ParseArena) allocAttrs(count int) []Attr {
	for {
		if a.attrIdx >= len(a.attrChunks) {
			sz := max(count, 32)
			if len(a.attrChunks) > 0 {
				sz = max(count, min(len(a.attrChunks[len(a.attrChunks)-1])*2, 512))
			}
			a.attrChunks = append(a.attrChunks, make([]Attr, sz))
			a.attrOff = 0
		}
		chunk := a.attrChunks[a.attrIdx]
		if a.attrOff+count <= len(chunk) {
			s := chunk[a.attrOff : a.attrOff : a.attrOff+count]
			a.attrOff += count
			return s
		}
		a.attrIdx++
		a.attrOff = 0
	}
}

// simpleParser is the state of one parseSimpleDocument.
type simpleParser struct {
	src string
	pos int
	doc *Document
	// arena is non-nil when parsing into a pooled ParseArena.
	arena *ParseArena
	// stack has the open elements. The first one is the body.
	stack []*Node
	// nodes and attrs are the storage of the nodes of the document.
	nodes    []Node
	attrs    []Attr
	nodeSlab int
	attrSlab int
	// formatting has the formatting elements with two or more attributes
	// that were read, to find two of them whose attributes differ only in
	// the order. See startHTML.
	formatting []*Node
}

func (p *simpleParser) newNode() *Node {
	if p.arena != nil {
		return p.arena.allocNode(p.doc)
	}
	if len(p.nodes) == 0 {
		sz := p.nodeSlab
		if sz == 0 {
			if len(p.src) > 1024 {
				sz = 64
			} else {
				sz = 8
			}
		} else if sz < 64 {
			sz *= 2
		}
		p.nodeSlab = sz
		p.nodes = make([]Node, sz)
	}
	n := &p.nodes[0]
	p.nodes = p.nodes[1:]
	n.doc = p.doc
	return n
}

func (p *simpleParser) element(ns, tag string) *Node {
	n := p.newNode()
	n.Type, n.NS, n.Tag = ElementNode, ns, tag
	return n
}

// add appends a node to the current element.
func (p *simpleParser) add(n *Node) {
	parent := p.stack[len(p.stack)-1]
	n.Parent = parent
	n.Prev = parent.LastChild
	if parent.LastChild != nil {
		parent.LastChild.Next = n
	} else {
		parent.FirstChild = n
	}
	parent.LastChild = n
}

// parseSimpleFragment parses an HTML fragment directly into parent for ordinary
// HTML containers (labels, spans, divs, paragraphs). It returns false for any
// markup that requires full HTML5 error recovery, leaving parent empty so the
// caller falls back to html.ParseFragment.
func parseSimpleFragment(parent *Node, markup string) bool {
	if parent.NS != NSHTML {
		return false
	}
	switch parent.Tag {
	case "span", "div", "p", "b", "i", "em", "strong", "u", "s", "small", "code", "sub", "sup", "a":
	default:
		return false
	}
	if strings.ContainsAny(markup, "\r\x00") || !utf8.ValidString(markup) {
		return false
	}
	var stackBuf [16]*Node
	p := simpleParser{
		src:   markup,
		doc:   parent.doc,
		arena: parent.doc.arena,
		stack: append(stackBuf[:0], parent),
	}
	for p.pos < len(markup) {
		if markup[p.pos] != '<' {
			if !p.text() {
				parent.RemoveChildren()
				return false
			}
			continue
		}
		if p.pos+1 >= len(markup) {
			parent.RemoveChildren()
			return false
		}
		ok := false
		switch c := markup[p.pos+1]; {
		case c == '/':
			ok = p.endTag()
		case isASCIILetter(c):
			ok = p.startTag()
		}
		if !ok {
			parent.RemoveChildren()
			return false
		}
	}
	if len(p.stack) != 1 {
		parent.RemoveChildren()
		return false
	}
	return true
}

// parseSimpleDocument is ParseHTMLDocument for simple markup. See the top
// of this file.
func parseSimpleDocument(markup string) (*HTMLDocument, bool) {
	return parseSimpleDocumentWithArena(markup, nil)
}

func parseSimpleDocumentWithArena(markup string, arena *ParseArena) (*HTMLDocument, bool) {
	// The full parser changes these: a carriage return becomes a line feed,
	// and NUL and bytes that are not UTF-8 become other characters.
	if strings.ContainsAny(markup, "\r\x00") || !utf8.ValidString(markup) {
		return nil, false
	}
	d := &Document{}
	p := &simpleParser{src: markup, doc: d, arena: arena}
	root := p.element(NSHTML, "html")
	head := p.element(NSHTML, "head")
	body := p.element(NSHTML, "body")
	if arena != nil {
		p.stack = append(arena.stack[:0], root)
		p.formatting = arena.formatting[:0]
	} else {
		p.stack = append(make([]*Node, 0, 16), root)
	}
	p.add(head)
	p.add(body)
	p.stack[0] = body

	// The insertion modes before the body drop white space.
	for p.pos < len(markup) {
		if c := markup[p.pos]; c != ' ' && c != '\t' && c != '\n' && c != '\f' {
			break
		}
		p.pos++
	}
	if p.pos == len(markup) {
		// No content: the full parser decides what the document is.
		if arena != nil {
			arena.stack = p.stack[:0]
			arena.formatting = p.formatting[:0]
		}
		return nil, false
	}
	for p.pos < len(markup) {
		if markup[p.pos] != '<' {
			if !p.text() {
				if arena != nil {
					arena.stack = p.stack[:0]
					arena.formatting = p.formatting[:0]
				}
				return nil, false
			}
			continue
		}
		if p.pos+1 >= len(markup) {
			if arena != nil {
				arena.stack = p.stack[:0]
				arena.formatting = p.formatting[:0]
			}
			return nil, false
		}
		ok := false
		switch c := markup[p.pos+1]; {
		case c == '/':
			ok = p.endTag()
		case isASCIILetter(c):
			ok = p.startTag()
		}
		if !ok {
			if arena != nil {
				arena.stack = p.stack[:0]
				arena.formatting = p.formatting[:0]
			}
			return nil, false
		}
	}
	if len(p.stack) != 1 {
		// An element is still open. The full parser closes it.
		if arena != nil {
			arena.stack = p.stack[:0]
			arena.formatting = p.formatting[:0]
		}
		return nil, false
	}
	if arena != nil {
		arena.stack = p.stack
		arena.formatting = p.formatting
	}
	d.Body = body
	return &HTMLDocument{Document: d, Root: root, Body: body}, true
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// isMarkupNameChar reports whether c can be in a tag name or an attribute name
// that the parser reads.
func isMarkupNameChar(c byte) bool {
	return isASCIILetter(c) || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == ':' || c == '.'
}

// lowerASCII is the lower case that the tokenizer gives a name.
func lowerASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			return strings.ToLower(s)
		}
	}
	return s
}

// inHTML reports whether the HTML rules apply to a start tag and to text:
// the current element is an HTML element, or an SVG element whose content
// is HTML.
func (p *simpleParser) inHTML() bool {
	top := p.stack[len(p.stack)-1]
	return top.NS == NSHTML || isSVGIntegrationPoint(top)
}

func isSVGIntegrationPoint(n *Node) bool {
	return n.NS == NSSVG && (n.Tag == "foreignObject" || n.Tag == "desc" || n.Tag == "title")
}

// text reads the characters up to the next tag.
func (p *simpleParser) text() bool {
	end := strings.IndexByte(p.src[p.pos:], '<')
	if end < 0 {
		end = len(p.src)
	} else {
		end += p.pos
	}
	data, ok := decodeSimple(p.src[p.pos:end])
	if !ok {
		return false
	}
	p.pos = end
	n := p.newNode()
	n.Type, n.Data = TextNode, data
	p.add(n)
	return true
}

// decodeSimple decodes the character references that the serializer of this
// module writes. The second result is false for a text with another
// reference: the full parser has the table of names and the rules for a
// reference without a semicolon.
func decodeSimple(s string) (string, bool) {
	i := strings.IndexByte(s, '&')
	if i < 0 {
		return s, true
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for {
		sb.WriteString(s[:i])
		s = s[i:]
		switch {
		case strings.HasPrefix(s, "&amp;"):
			sb.WriteByte('&')
			s = s[5:]
		case strings.HasPrefix(s, "&lt;"):
			sb.WriteByte('<')
			s = s[4:]
		case strings.HasPrefix(s, "&gt;"):
			sb.WriteByte('>')
			s = s[4:]
		case strings.HasPrefix(s, "&quot;"):
			sb.WriteByte('"')
			s = s[6:]
		case strings.HasPrefix(s, "&nbsp;"):
			sb.WriteString(" ")
			s = s[6:]
		default:
			// An ampersand that starts no reference stays as it is.
			if len(s) > 1 {
				if c := s[1]; c == '#' || isASCIILetter(c) || (c >= '0' && c <= '9') {
					return "", false
				}
			}
			sb.WriteByte('&')
			s = s[1:]
		}
		if i = strings.IndexByte(s, '&'); i < 0 {
			sb.WriteString(s)
			return sb.String(), true
		}
	}
}

// endTag reads an end tag, which must be the end tag of the current
// element.
func (p *simpleParser) endTag() bool {
	start := p.pos + 2
	end := start
	for end < len(p.src) && isMarkupNameChar(p.src[end]) {
		end++
	}
	if end == start || end >= len(p.src) || p.src[end] != '>' || !isASCIILetter(p.src[start]) {
		return false
	}
	if len(p.stack) == 1 {
		return false
	}
	name := lowerASCII(p.src[start:end])
	top := p.stack[len(p.stack)-1]
	// An HTML element has its name in lower case. The end tag of an SVG
	// element matches without regard to case.
	if top.NS == NSHTML {
		if top.Tag != name {
			return false
		}
	} else if !strings.EqualFold(top.Tag, name) {
		return false
	}
	p.stack = p.stack[:len(p.stack)-1]
	p.pos = end + 1
	return true
}

// startTag reads a start tag and its attributes.
func (p *simpleParser) startTag() bool {
	start := p.pos + 1
	end := start
	for end < len(p.src) && isMarkupNameChar(p.src[end]) {
		end++
	}
	name := lowerASCII(p.src[start:end])
	p.pos = end
	if len(p.stack) > simpleMaxOpen {
		return false
	}

	// The element. The attributes follow below, because their names depend
	// on the namespace.
	var n *Node
	void := false
	html := p.inHTML()
	if html {
		top := p.stack[len(p.stack)-1]
		if top.NS != NSHTML && top.Tag != "foreignObject" {
			// The content of title and desc is text.
			return false
		}
		if name == "svg" {
			n = p.element(NSSVG, "svg")
		} else {
			kind := simpleHTMLTagKind(name)
			if kind == 0 || !p.startHTML(name, kind) {
				return false
			}
			void = kind == simpleVoid
			n = p.element(NSHTML, name)
		}
	} else {
		if isSVGBreakout(name) {
			return false
		}
		name = adjustSVGTag(name)
		n = p.element(NSSVG, name)
	}
	selfClosing, ok := p.attributes(n)
	if !ok {
		return false
	}
	// A self-closing tag is read for a void HTML element only, where the
	// slash means nothing.
	if selfClosing && !void {
		return false
	}
	if n.NS == NSHTML && len(n.Attrs) > 1 && simpleHTMLTagKind(n.Tag) == simpleFormatting {
		// The full parser finds the attribute order of a formatting element
		// by its set of attributes. Two elements with the same set in
		// another order are a case that only it decides.
		for _, other := range p.formatting {
			if other.Tag == n.Tag && sameAttrSet(other.Attrs, n.Attrs) && !sameAttrs(other.Attrs, n.Attrs) {
				return false
			}
		}
		p.formatting = append(p.formatting, n)
	}
	p.add(n)
	if !void {
		p.stack = append(p.stack, n)
	}
	return true
}

// startHTML reports whether the HTML standard inserts the element at the
// current place without a change of the open elements.
func (p *simpleParser) startHTML(name string, kind simpleHTMLTag) bool {
	switch kind {
	case simpleBlock:
		// A block closes a p element that is open in button scope. The
		// scope ends at an SVG element whose content is HTML.
		for i := len(p.stack) - 1; i >= 0; i-- {
			n := p.stack[i]
			if n.NS != NSHTML {
				break
			}
			if n.Tag == "p" {
				return false
			}
		}
	case simpleFormatting:
		// An a element inside an a element, and the fourth of the same
		// formatting element, change the list of active formatting
		// elements.
		same := 0
		for _, n := range p.stack {
			if n.NS == NSHTML && n.Tag == name {
				same++
			}
		}
		if same >= 3 || (name == "a" && same > 0) {
			return false
		}
	}
	return true
}

// attributes reads the attributes of a start tag up to its end. Each one is
// a space, a name, an equals sign and a value in double quotes, as the
// serializer writes it.
func (p *simpleParser) attributes(n *Node) (selfClosing, ok bool) {
	src := p.src
	var stackBuf [16]Attr
	attrs := stackBuf[:0]
	for {
		if p.pos >= len(src) {
			return false, false
		}
		switch src[p.pos] {
		case '>':
			p.pos++
			if len(attrs) > 0 {
				n.Attrs = p.copyAttrs(attrs)
			}
			return false, true
		case '/':
			if p.pos+1 < len(src) && src[p.pos+1] == '>' {
				p.pos += 2
				if len(attrs) > 0 {
					n.Attrs = p.copyAttrs(attrs)
				}
				return true, true
			}
			return false, false
		case ' ':
		default:
			return false, false
		}
		start := p.pos + 1
		end := start
		for end < len(src) && isMarkupNameChar(src[end]) {
			end++
		}
		if end == start || end+1 >= len(src) || src[end] != '=' || src[end+1] != '"' {
			return false, false
		}
		close := strings.IndexByte(src[end+2:], '"')
		if close < 0 {
			return false, false
		}
		value, ok := decodeSimple(src[end+2 : end+2+close])
		if !ok {
			return false, false
		}
		p.pos = end + 2 + close + 1
		attr := Attr{Local: lowerASCII(src[start:end]), Value: value}
		if n.NS != NSHTML {
			adjustSVGAttr(&attr)
		}
		// The tokenizer drops an attribute whose name the tag has already.
		// Names with a namespace are compared as the tokenizer sees them.
		for i := range attrs {
			if attrs[i].Local == attr.Local && attrs[i].Prefix == attr.Prefix {
				return false, false
			}
		}
		attrs = append(attrs, attr)
	}
}

func (p *simpleParser) copyAttrs(attrs []Attr) []Attr {
	count := len(attrs)
	var dst []Attr
	if p.arena != nil {
		dst = p.arena.allocAttrs(count)
	} else {
		if len(p.attrs) < count {
			sz := p.attrSlab
			if sz == 0 {
				if len(p.src) > 1024 {
					sz = 128
				} else {
					sz = 8
				}
			} else if sz < 128 {
				sz *= 2
			}
			p.attrSlab = sz
			p.attrs = make([]Attr, max(count, sz))
		}
		dst = p.attrs[:0:count]
		p.attrs = p.attrs[count:]
	}
	return append(dst, attrs...)
}

// adjustSVGAttr gives the attribute of a foreign element the name and the
// namespace that the full parser gives it: the case of SVG attribute names,
// and the namespaces of the xlink, xml and xmlns attributes.
func adjustSVGAttr(a *Attr) {
	local := a.Local
	if len(local) < 4 {
		return
	}
	if local[0] == 'x' {
		switch local {
		case "xlink:actuate", "xlink:arcrole", "xlink:href", "xlink:role", "xlink:show", "xlink:title", "xlink:type":
			a.NS, a.Prefix, a.Local = NSXLink, "xlink", local[len("xlink:"):]
			return
		case "xml:lang", "xml:space":
			a.NS, a.Prefix, a.Local = NSXML, "xml", local[len("xml:"):]
			return
		case "xmlns:xlink":
			a.NS, a.Prefix, a.Local = NSXMLNS, "xmlns", "xlink"
			return
		case "xmlns":
			a.NS = NSXMLNS
			return
		case "xchannelselector":
			a.Local = "xChannelSelector"
			return
		}
		return
	}
	switch local {
	case "attributename":
		a.Local = "attributeName"
	case "attributetype":
		a.Local = "attributeType"
	case "basefrequency":
		a.Local = "baseFrequency"
	case "baseprofile":
		a.Local = "baseProfile"
	case "calcmode":
		a.Local = "calcMode"
	case "clippathunits":
		a.Local = "clipPathUnits"
	case "diffuseconstant":
		a.Local = "diffuseConstant"
	case "edgemode":
		a.Local = "edgeMode"
	case "filterunits":
		a.Local = "filterUnits"
	case "glyphref":
		a.Local = "glyphRef"
	case "gradienttransform":
		a.Local = "gradientTransform"
	case "gradientunits":
		a.Local = "gradientUnits"
	case "kernelmatrix":
		a.Local = "kernelMatrix"
	case "kernelunitlength":
		a.Local = "kernelUnitLength"
	case "keypoints":
		a.Local = "keyPoints"
	case "keysplines":
		a.Local = "keySplines"
	case "keytimes":
		a.Local = "keyTimes"
	case "lengthadjust":
		a.Local = "lengthAdjust"
	case "limitingconeangle":
		a.Local = "limitingConeAngle"
	case "markerheight":
		a.Local = "markerHeight"
	case "markerunits":
		a.Local = "markerUnits"
	case "markerwidth":
		a.Local = "markerWidth"
	case "maskcontentunits":
		a.Local = "maskContentUnits"
	case "maskunits":
		a.Local = "maskUnits"
	case "numoctaves":
		a.Local = "numOctaves"
	case "pathlength":
		a.Local = "pathLength"
	case "patterncontentunits":
		a.Local = "patternContentUnits"
	case "patterntransform":
		a.Local = "patternTransform"
	case "patternunits":
		a.Local = "patternUnits"
	case "pointsatx":
		a.Local = "pointsAtX"
	case "pointsaty":
		a.Local = "pointsAtY"
	case "pointsatz":
		a.Local = "pointsAtZ"
	case "preservealpha":
		a.Local = "preserveAlpha"
	case "preserveaspectratio":
		a.Local = "preserveAspectRatio"
	case "primitiveunits":
		a.Local = "primitiveUnits"
	case "refx":
		a.Local = "refX"
	case "refy":
		a.Local = "refY"
	case "repeatcount":
		a.Local = "repeatCount"
	case "repeatdur":
		a.Local = "repeatDur"
	case "requiredextensions":
		a.Local = "requiredExtensions"
	case "requiredfeatures":
		a.Local = "requiredFeatures"
	case "specularconstant":
		a.Local = "specularConstant"
	case "specularexponent":
		a.Local = "specularExponent"
	case "spreadmethod":
		a.Local = "spreadMethod"
	case "startoffset":
		a.Local = "startOffset"
	case "stddeviation":
		a.Local = "stdDeviation"
	case "stitchtiles":
		a.Local = "stitchTiles"
	case "surfacescale":
		a.Local = "surfaceScale"
	case "systemlanguage":
		a.Local = "systemLanguage"
	case "tablevalues":
		a.Local = "tableValues"
	case "targetx":
		a.Local = "targetX"
	case "targety":
		a.Local = "targetY"
	case "textlength":
		a.Local = "textLength"
	case "viewbox":
		a.Local = "viewBox"
	case "viewtarget":
		a.Local = "viewTarget"
	case "ychannelselector":
		a.Local = "yChannelSelector"
	case "zoomandpan":
		a.Local = "zoomAndPan"
	}
}

func sameAttrs(a, b []Attr) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameAttrSet reports whether two lists have the same attributes in any
// order.
func sameAttrSet(a, b []Attr) bool {
	if len(a) != len(b) {
		return false
	}
	for _, x := range a {
		found := false
		for _, y := range b {
			if x == y {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
