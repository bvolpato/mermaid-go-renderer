// This file ports no upstream file. It connects the HTML parser of
// golang.org/x/net/html to the DOM, and repairs the places where the tree
// of that parser differs from the tree of Chromium.

package dom

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// This file connects the HTML parser of golang.org/x/net/html to the DOM.
// The browser parses markup in two places that matter for parity: the
// innerHTML setter (labels), and the re-parse that DOMPurify does on the
// finished SVG.

// HTML is selection.html(value): Element.innerHTML as a setter. The markup
// is parsed with the HTML fragment parsing algorithm, with n as the context.
func (n *Node) HTML(markup string) *Node {
	if n == nil || n.Type != ElementNode {
		return n
	}
	n.RemoveChildren()
	// Text with no markup and no character reference needs no parser.
	if !strings.ContainsAny(markup, "<&\r\x00") {
		if markup != "" {
			n.AppendChild(n.doc.CreateText(markup))
		}
		return n
	}
	ctx := &html.Node{Type: html.ElementNode, Data: n.Tag, DataAtom: atom.Lookup([]byte(n.Tag))}
	switch n.NS {
	case NSSVG:
		ctx.Namespace = "svg"
	case NSMathML:
		ctx.Namespace = "math"
	}
	nodes, err := html.ParseFragment(strings.NewReader(markup), ctx)
	if err != nil {
		return n
	}
	// Repair the places where the parser differs from Chromium.
	for _, hn := range nodes {
		fixParserDivergences(markup, hn)
	}
	for _, hn := range nodes {
		if c := n.doc.fromHTMLNode(hn); c != nil {
			n.AppendChild(c)
		}
	}
	return n
}

// ParseBody parses markup as a browser parses a whole text/html document,
// and returns the children of the resulting body in a new Document.
func ParseBody(markup string) (*Document, error) {
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return nil, err
	}
	fixParserDivergences(markup, root)
	d := NewDocument()
	var body *html.Node
	var find func(*html.Node)
	find = func(hn *html.Node) {
		for c := hn.FirstChild; c != nil && body == nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Namespace == "" && c.Data == "body" {
				body = c
				return
			}
			find(c)
		}
	}
	find(root)
	if body == nil {
		return d, nil
	}
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if n := d.fromHTMLNode(c); n != nil {
			d.Body.AppendChild(n)
		}
	}
	return d, nil
}

func (d *Document) fromHTMLNode(hn *html.Node) *Node {
	switch hn.Type {
	case html.TextNode:
		return d.CreateText(hn.Data)
	case html.CommentNode:
		return d.CreateComment(hn.Data)
	case html.ElementNode:
		n := &Node{Type: ElementNode, Tag: hn.Data, doc: d}
		switch hn.Namespace {
		case "":
			n.NS = NSHTML
		case "svg":
			n.NS = NSSVG
		case "math":
			n.NS = NSMathML
		default:
			n.NS = hn.Namespace
		}
		if len(hn.Attr) > 0 {
			n.Attrs = make([]Attr, 0, len(hn.Attr))
			for _, a := range hn.Attr {
				attr := Attr{Local: a.Key, Value: a.Val}
				switch a.Namespace {
				case "xlink":
					attr.NS, attr.Prefix = NSXLink, "xlink"
				case "xml":
					attr.NS, attr.Prefix = NSXML, "xml"
				case "xmlns":
					attr.NS, attr.Prefix = NSXMLNS, "xmlns"
				case "":
					if a.Key == "xmlns" && n.NS != NSHTML {
						attr.NS = NSXMLNS
					}
				}
				n.Attrs = append(n.Attrs, attr)
			}
		}
		for c := hn.FirstChild; c != nil; c = c.NextSibling {
			if child := d.fromHTMLNode(c); child != nil {
				child.Parent = n
				child.Prev = n.LastChild
				if n.LastChild != nil {
					n.LastChild.Next = child
				} else {
					n.FirstChild = child
				}
				n.LastChild = child
			}
		}
		return n
	}
	return nil
}
