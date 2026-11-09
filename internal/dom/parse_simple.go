package dom

import (
	"strings"
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

var simpleHTMLTags = map[string]simpleHTMLTag{
	"span": simpleOrdinary, "sub": simpleOrdinary, "sup": simpleOrdinary,
	"div": simpleBlock, "p": simpleBlock,
	"b": simpleFormatting, "i": simpleFormatting, "em": simpleFormatting, "strong": simpleFormatting,
	"u": simpleFormatting, "s": simpleFormatting, "small": simpleFormatting, "code": simpleFormatting,
	"a":  simpleFormatting,
	"br": simpleVoid, "img": simpleVoid,
}

// svgBreakout are the start tags that end foreign content (the breakout
// table of the HTML standard, and font, which breaks out with some
// attributes). The parser does not read them inside SVG.
var svgBreakout = map[string]bool{
	"b": true, "big": true, "blockquote": true, "body": true, "br": true, "center": true, "code": true,
	"dd": true, "div": true, "dl": true, "dt": true, "em": true, "embed": true, "h1": true, "h2": true,
	"h3": true, "h4": true, "h5": true, "h6": true, "head": true, "hr": true, "i": true, "img": true,
	"li": true, "listing": true, "menu": true, "meta": true, "nobr": true, "ol": true, "p": true,
	"pre": true, "ruby": true, "s": true, "small": true, "span": true, "strong": true, "strike": true,
	"sub": true, "sup": true, "table": true, "tt": true, "u": true, "ul": true, "var": true, "font": true,
}

// svgTagNames is the table of the HTML standard that gives an SVG tag name
// its case, as x/net/html has it, with the name that fixParserDivergences
// adds.
var svgTagNames = map[string]string{
	"altglyph": "altGlyph", "altglyphdef": "altGlyphDef", "altglyphitem": "altGlyphItem",
	"animatecolor": "animateColor", "animatemotion": "animateMotion", "animatetransform": "animateTransform",
	"clippath": "clipPath", "feblend": "feBlend", "fecolormatrix": "feColorMatrix",
	"fecomponenttransfer": "feComponentTransfer", "fecomposite": "feComposite",
	"feconvolvematrix": "feConvolveMatrix", "fediffuselighting": "feDiffuseLighting",
	"fedisplacementmap": "feDisplacementMap", "fedistantlight": "feDistantLight", "feflood": "feFlood",
	"fefunca": "feFuncA", "fefuncb": "feFuncB", "fefuncg": "feFuncG", "fefuncr": "feFuncR",
	"fegaussianblur": "feGaussianBlur", "feimage": "feImage", "femerge": "feMerge",
	"femergenode": "feMergeNode", "femorphology": "feMorphology", "feoffset": "feOffset",
	"fepointlight": "fePointLight", "fespecularlighting": "feSpecularLighting", "fespotlight": "feSpotLight",
	"fetile": "feTile", "feturbulence": "feTurbulence", "foreignobject": "foreignObject",
	"glyphref": "glyphRef", "lineargradient": "linearGradient", "radialgradient": "radialGradient",
	"textpath":     "textPath",
	"fedropshadow": "feDropShadow",
}

// svgAttrNames is the table of the HTML standard that gives an SVG
// attribute name its case, as x/net/html has it.
var svgAttrNames = map[string]string{
	"attributename": "attributeName", "attributetype": "attributeType", "basefrequency": "baseFrequency",
	"baseprofile": "baseProfile", "calcmode": "calcMode", "clippathunits": "clipPathUnits",
	"diffuseconstant": "diffuseConstant", "edgemode": "edgeMode", "filterunits": "filterUnits",
	"glyphref": "glyphRef", "gradienttransform": "gradientTransform", "gradientunits": "gradientUnits",
	"kernelmatrix": "kernelMatrix", "kernelunitlength": "kernelUnitLength", "keypoints": "keyPoints",
	"keysplines": "keySplines", "keytimes": "keyTimes", "lengthadjust": "lengthAdjust",
	"limitingconeangle": "limitingConeAngle", "markerheight": "markerHeight", "markerunits": "markerUnits",
	"markerwidth": "markerWidth", "maskcontentunits": "maskContentUnits", "maskunits": "maskUnits",
	"numoctaves": "numOctaves", "pathlength": "pathLength", "patterncontentunits": "patternContentUnits",
	"patterntransform": "patternTransform", "patternunits": "patternUnits", "pointsatx": "pointsAtX",
	"pointsaty": "pointsAtY", "pointsatz": "pointsAtZ", "preservealpha": "preserveAlpha",
	"preserveaspectratio": "preserveAspectRatio", "primitiveunits": "primitiveUnits", "refx": "refX",
	"refy": "refY", "repeatcount": "repeatCount", "repeatdur": "repeatDur",
	"requiredextensions": "requiredExtensions", "requiredfeatures": "requiredFeatures",
	"specularconstant": "specularConstant", "specularexponent": "specularExponent",
	"spreadmethod": "spreadMethod", "startoffset": "startOffset", "stddeviation": "stdDeviation",
	"stitchtiles": "stitchTiles", "surfacescale": "surfaceScale", "systemlanguage": "systemLanguage",
	"tablevalues": "tableValues", "targetx": "targetX", "targety": "targetY", "textlength": "textLength",
	"viewbox": "viewBox", "viewtarget": "viewTarget", "xchannelselector": "xChannelSelector",
	"ychannelselector": "yChannelSelector", "zoomandpan": "zoomAndPan",
}

// simpleMaxOpen is the largest number of open elements for which the parser
// reads a start tag. The full parser rejects a document that has more than
// 512 open elements at one time, so it must decide for a document that comes
// near to that number. No diagram comes near to simpleMaxOpen.
const simpleMaxOpen = 256

// simpleParser is the state of one parseSimpleDocument.
type simpleParser struct {
	src string
	pos int
	doc *Document
	// stack has the open elements. The first one is the body.
	stack []*Node
	// nodes and attrs are the storage of the nodes of the document.
	nodes []Node
	attrs []Attr
	// formatting has the formatting elements with two or more attributes
	// that were read, to find two of them whose attributes differ only in
	// the order. See startHTML.
	formatting []*Node
}

func (p *simpleParser) newNode() *Node {
	if len(p.nodes) == 0 {
		p.nodes = make([]Node, 32)
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

// parseSimpleDocument is ParseHTMLDocument for simple markup. See the top
// of this file.
func parseSimpleDocument(markup string) (*HTMLDocument, bool) {
	// The full parser changes these: a carriage return becomes a line feed,
	// and NUL and bytes that are not UTF-8 become other characters.
	if strings.ContainsAny(markup, "\r\x00") || !utf8.ValidString(markup) {
		return nil, false
	}
	d := &Document{}
	p := &simpleParser{src: markup, doc: d}
	root := p.element(NSHTML, "html")
	head := p.element(NSHTML, "head")
	body := p.element(NSHTML, "body")
	p.stack = append(make([]*Node, 0, 16), root)
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
		return nil, false
	}
	for p.pos < len(markup) {
		if markup[p.pos] != '<' {
			if !p.text() {
				return nil, false
			}
			continue
		}
		if p.pos+1 >= len(markup) {
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
			return nil, false
		}
	}
	if len(p.stack) != 1 {
		// An element is still open. The full parser closes it.
		return nil, false
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
			kind := simpleHTMLTags[name]
			if kind == 0 || !p.startHTML(name, kind) {
				return false
			}
			void = kind == simpleVoid
			n = p.element(NSHTML, name)
		}
	} else {
		if svgBreakout[name] {
			return false
		}
		if adjusted, ok := svgTagNames[name]; ok {
			name = adjusted
		}
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
	if n.NS == NSHTML && len(n.Attrs) > 1 && simpleHTMLTags[n.Tag] == simpleFormatting {
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
	// Count the attributes, to take their storage in one piece.
	count := 0
	for i := p.pos; i < len(src) && src[i] != '>'; i++ {
		if src[i] == '"' {
			count++
		}
	}
	count /= 2
	if count > 0 {
		if len(p.attrs) < count {
			p.attrs = make([]Attr, max(count, 64))
		}
		n.Attrs = p.attrs[:0:count]
		p.attrs = p.attrs[count:]
	}
	for {
		if p.pos >= len(src) {
			return false, false
		}
		switch src[p.pos] {
		case '>':
			p.pos++
			return false, true
		case '/':
			if p.pos+1 < len(src) && src[p.pos+1] == '>' {
				p.pos += 2
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
		for i := range n.Attrs {
			if n.Attrs[i].Local == attr.Local && n.Attrs[i].Prefix == attr.Prefix {
				return false, false
			}
		}
		if len(n.Attrs) == cap(n.Attrs) {
			// A quote inside a value made the count too small.
			return false, false
		}
		n.Attrs = append(n.Attrs, attr)
	}
}

// adjustSVGAttr gives the attribute of a foreign element the name and the
// namespace that the full parser gives it: the case of SVG attribute names,
// and the namespaces of the xlink, xml and xmlns attributes.
func adjustSVGAttr(a *Attr) {
	if adjusted, ok := svgAttrNames[a.Local]; ok {
		a.Local = adjusted
		return
	}
	if a.Local == "" || a.Local[0] != 'x' {
		return
	}
	switch a.Local {
	case "xlink:actuate", "xlink:arcrole", "xlink:href", "xlink:role", "xlink:show", "xlink:title", "xlink:type":
		a.NS, a.Prefix, a.Local = NSXLink, "xlink", a.Local[len("xlink:"):]
	case "xml:lang", "xml:space":
		a.NS, a.Prefix, a.Local = NSXML, "xml", a.Local[len("xml:"):]
	case "xmlns:xlink":
		a.NS, a.Prefix, a.Local = NSXMLNS, "xmlns", "xlink"
	case "xmlns":
		a.NS = NSXMLNS
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
