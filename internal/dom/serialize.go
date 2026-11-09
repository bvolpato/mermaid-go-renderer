// This file ports no upstream file. It has the HTML fragment serialization
// algorithm of the HTML standard.

package dom

import "sync"

// This file implements the HTML fragment serialization algorithm
// (https://html.spec.whatwg.org/#serialising-html-fragments), which is what
// Element.innerHTML returns in a browser.

var voidElements = map[string]bool{
	"area": true, "base": true, "basefont": true, "bgsound": true, "br": true, "col": true,
	"embed": true, "frame": true, "hr": true, "img": true, "input": true, "keygen": true,
	"link": true, "meta": true, "param": true, "source": true, "track": true, "wbr": true,
}

// rawTextElements are the HTML elements whose text children are not escaped.
var rawTextElements = map[string]bool{
	"style": true, "script": true, "xmp": true, "iframe": true, "noembed": true,
	"noframes": true, "plaintext": true, "noscript": true,
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

var htmlWriters = sync.Pool{New: func() any { return new(htmlWriter) }}

// maxPooledHTML is the largest buffer that stays in the pool.
const maxPooledHTML = 4 << 20

func getHTMLWriter() *htmlWriter {
	w := htmlWriters.Get().(*htmlWriter)
	w.buf = w.buf[:0]
	return w
}

// finish returns the text and gives the buffer back.
func (w *htmlWriter) finish() string {
	s := string(w.buf)
	if cap(w.buf) <= maxPooledHTML {
		htmlWriters.Put(w)
	}
	return s
}

// InnerHTML is Element.innerHTML as a getter.
func (n *Node) InnerHTML() string {
	w := getHTMLWriter()
	n.serializeChildren(w)
	return w.finish()
}

// OuterHTML is Element.outerHTML as a getter.
func (n *Node) OuterHTML() string {
	w := getHTMLWriter()
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
		if p != nil && p.Type == ElementNode && p.NS == NSHTML && rawTextElements[p.Tag] {
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
		if n.NS == NSHTML && voidElements[n.Tag] {
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

func escapeText(sb *htmlWriter, s string) {
	last := 0
	for i := 0; i < len(s); i++ {
		var rep string
		switch s[i] {
		case '&':
			rep = "&amp;"
		case '<':
			rep = "&lt;"
		case '>':
			rep = "&gt;"
		case 0xc2:
			if i+1 < len(s) && s[i+1] == 0xa0 {
				sb.WriteString(s[last:i])
				sb.WriteString("&nbsp;")
				i++
				last = i + 1
			}
			continue
		default:
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		last = i + 1
	}
	sb.WriteString(s[last:])
}

func escapeAttr(sb *htmlWriter, s string) {
	last := 0
	for i := 0; i < len(s); i++ {
		var rep string
		switch s[i] {
		case '&':
			rep = "&amp;"
		case '"':
			rep = "&quot;"
		case '<':
			rep = "&lt;"
		case '>':
			rep = "&gt;"
		case 0xc2:
			if i+1 < len(s) && s[i+1] == 0xa0 {
				sb.WriteString(s[last:i])
				sb.WriteString("&nbsp;")
				i++
				last = i + 1
			}
			continue
		default:
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		last = i + 1
	}
	sb.WriteString(s[last:])
}
