// This file ports no upstream file. It has the HTML fragment serialization
// algorithm of the HTML standard.

package dom

import "sync"

// This file implements the HTML fragment serialization algorithm
// (https://html.spec.whatwg.org/#serialising-html-fragments), which is what
// Element.innerHTML returns in a browser.

func isVoidElement(tag string) bool {
	switch tag {
	case "area", "base", "basefont", "bgsound", "br", "col",
		"embed", "frame", "hr", "img", "input", "keygen",
		"link", "meta", "param", "source", "track", "wbr":
		return true
	}
	return false
}

// isRawTextElement reports whether tag is an HTML element whose text children
// are not escaped.
func isRawTextElement(tag string) bool {
	switch tag {
	case "style", "script", "xmp", "iframe", "noembed",
		"noframes", "plaintext", "noscript":
		return true
	}
	return false
}

// htmlWriter collects the text of a serialization. A serialization writes
// to a buffer that is used again, and the result is one string of the exact
// size: a buffer that grows with the text would allocate the text two to
// three times.
type htmlWriter struct{ buf []byte }

func (w *htmlWriter) WriteString(s string) { w.buf = append(w.buf, s...) }

func (w *htmlWriter) WriteByte(c byte) error {
	w.buf = append(w.buf, c)
	return nil
}

var htmlWriters = sync.Pool{New: func() any { return &htmlWriter{buf: make([]byte, 0, 1024)} }}

// largeHTMLCache retains up to 2 large SVG serialization buffers across GC
// cycles so hot renders avoid re-growing 64KB..2MB buffers after every GC.
var largeHTMLCache = make(chan *htmlWriter, 2)

var largeHTMLWriters = sync.Pool{New: func() any { return &htmlWriter{buf: make([]byte, 0, 65536)} }}

// maxPooledHTML is the largest buffer that stays in the pool.
const (
	maxPooledHTML = 4 << 20
	maxCachedHTML = 2 << 20
)

func getHTMLWriter() *htmlWriter {
	w := htmlWriters.Get().(*htmlWriter)
	w.buf = w.buf[:0]
	return w
}

func getHTMLWriterFor(n *Node) *htmlWriter {
	if n != nil && (n.Tag == "body" || n.Tag == "svg" || (n.FirstChild != nil && n.FirstChild.Tag == "svg")) {
		select {
		case w := <-largeHTMLCache:
			w.buf = w.buf[:0]
			return w
		default:
		}
		w := largeHTMLWriters.Get().(*htmlWriter)
		w.buf = w.buf[:0]
		return w
	}
	return getHTMLWriter()
}

// finish returns the text and gives the buffer back.
func (w *htmlWriter) finish() string {
	s := string(w.buf)
	if cap(w.buf) <= maxPooledHTML {
		if cap(w.buf) >= 16384 {
			if cap(w.buf) <= maxCachedHTML {
				select {
				case largeHTMLCache <- w:
					return s
				default:
				}
			}
			largeHTMLWriters.Put(w)
		} else {
			htmlWriters.Put(w)
		}
	}
	return s
}

// InnerHTML is Element.innerHTML as a getter.
func (n *Node) InnerHTML() string {
	w := getHTMLWriterFor(n)
	n.serializeChildren(w)
	return w.finish()
}

// OuterHTML is Element.outerHTML as a getter.
func (n *Node) OuterHTML() string {
	w := getHTMLWriterFor(n)
	n.serialize(w)
	return w.finish()
}

func (n *Node) serializeChildren(sb *htmlWriter) {
	for c := n.FirstChild; c != nil; c = c.Next {
		c.serialize(sb)
	}
}

func (n *Node) serialize(sb *htmlWriter) {
	switch n.Type {
	case TextNode:
		p := n.Parent
		if p != nil && p.Type == ElementNode && p.NS == NSHTML && isRawTextElement(p.Tag) {
			sb.WriteString(n.Data)
		} else {
			escapeText(sb, n.Data)
		}
	case CommentNode:
		sb.WriteString("<!--")
		sb.WriteString(n.Data)
		sb.WriteString("-->")
	case ElementNode:
		n.syncStyle()
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
		if n.NS == NSHTML && isVoidElement(n.Tag) {
			return
		}
		n.serializeChildren(sb)
		sb.WriteString("</")
		sb.WriteString(n.Tag)
		sb.WriteByte('>')
	case DocumentNode:
		n.serializeChildren(sb)
	}
}

// serializedName is the attribute name that the HTML serializer writes.
func (a *Attr) serializedName() string {
	switch a.NS {
	case "":
		return a.Local
	case NSXML:
		return "xml:" + a.Local
	case NSXMLNS:
		if a.Local == "xmlns" {
			return "xmlns"
		}
		return "xmlns:" + a.Local
	case NSXLink:
		return "xlink:" + a.Local
	}
	return a.QName()
}

var textEscapeTable = [256]uint8{
	'&':  1,
	'<':  2,
	'>':  3,
	0xc2: 4,
}

var attrEscapeTable = [256]uint8{
	'&':  1,
	'"':  2,
	'<':  3,
	'>':  4,
	0xc2: 5,
}

const (
	swarLo8   = 0x0101010101010101
	swarHi8   = 0x8080808080808080
	swarAmp8  = 0x2626262626262626
	swarLt8   = 0x3c3c3c3c3c3c3c3c
	swarGt8   = 0x3e3e3e3e3e3e3e3e
	swarQuot8 = 0x2222222222222222
)

func escapeText(sb *htmlWriter, s string) {
	last := 0
	n := len(s)
	i := 0
	for i < n {
		for i+8 <= n {
			chunk := s[i : i+8]
			_ = chunk[7]
			w := uint64(chunk[0]) | uint64(chunk[1])<<8 | uint64(chunk[2])<<16 | uint64(chunk[3])<<24 |
				uint64(chunk[4])<<32 | uint64(chunk[5])<<40 | uint64(chunk[6])<<48 | uint64(chunk[7])<<56
			a := w ^ swarAmp8
			b := w ^ swarLt8
			c := w ^ swarGt8
			if (w|((a-swarLo8)&^a)|((b-swarLo8)&^b)|((c-swarLo8)&^c))&swarHi8 == 0 {
				i += 8
				continue
			}
			break
		}
		if i >= n {
			break
		}
		kind := textEscapeTable[s[i]]
		if kind == 0 {
			i++
			continue
		}
		var rep string
		switch kind {
		case 1:
			rep = "&amp;"
		case 2:
			rep = "&lt;"
		case 3:
			rep = "&gt;"
		case 4:
			if i+1 < n && s[i+1] == 0xa0 {
				sb.WriteString(s[last:i])
				sb.WriteString("&nbsp;")
				i += 2
				last = i
				continue
			}
			i++
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		i++
		last = i
	}
	sb.WriteString(s[last:])
}

func escapeAttr(sb *htmlWriter, s string) {
	last := 0
	n := len(s)
	i := 0
	for i < n {
		for i+8 <= n {
			chunk := s[i : i+8]
			_ = chunk[7]
			w := uint64(chunk[0]) | uint64(chunk[1])<<8 | uint64(chunk[2])<<16 | uint64(chunk[3])<<24 |
				uint64(chunk[4])<<32 | uint64(chunk[5])<<40 | uint64(chunk[6])<<48 | uint64(chunk[7])<<56
			a := w ^ swarAmp8
			b := w ^ swarQuot8
			c := w ^ swarLt8
			d := w ^ swarGt8
			if (w|((a-swarLo8)&^a)|((b-swarLo8)&^b)|((c-swarLo8)&^c)|((d-swarLo8)&^d))&swarHi8 == 0 {
				i += 8
				continue
			}
			break
		}
		if i >= n {
			break
		}
		kind := attrEscapeTable[s[i]]
		if kind == 0 {
			i++
			continue
		}
		var rep string
		switch kind {
		case 1:
			rep = "&amp;"
		case 2:
			rep = "&quot;"
		case 3:
			rep = "&lt;"
		case 4:
			rep = "&gt;"
		case 5:
			if i+1 < n && s[i+1] == 0xa0 {
				sb.WriteString(s[last:i])
				sb.WriteString("&nbsp;")
				i += 2
				last = i
				continue
			}
			i++
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		i++
		last = i
	}
	sb.WriteString(s[last:])
}
