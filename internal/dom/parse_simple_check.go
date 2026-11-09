package dom

import (
	"fmt"
	"sync/atomic"
)

// simpleParserSimple and simpleParserFull count the documents that each
// parser read, for a build with the tag domcheck.
var simpleParserSimple, simpleParserFull atomic.Int64

// SimpleParserCounts returns the number of documents that the simple parser
// read and the number that it left to the full parser. The counts are kept
// only in a build with the tag domcheck.
func SimpleParserCounts() (simple, full int64) {
	return simpleParserSimple.Load(), simpleParserFull.Load()
}

// checkSimpleDocument compares a document of the simple parser with the
// document of the full parser, and panics when they differ. A build with
// the tag domcheck calls it for every document.
func checkSimpleDocument(markup string, got *HTMLDocument) {
	simpleParserSimple.Add(1)
	want, err := parseHTMLDocumentFull(markup)
	if err != nil {
		panic(fmt.Sprintf("dom: the simple parser read a document that the full parser rejects: %v\n%s", err, markup))
	}
	if d := diffDocuments(got, want); d != "" {
		panic(fmt.Sprintf("dom: the simple parser differs from the full parser: %s\n%s", d, markup))
	}
}

// diffDocuments describes the first difference between two documents, or
// returns "".
func diffDocuments(got, want *HTMLDocument) string {
	if got.HasDoctype != want.HasDoctype || got.DoctypeName != want.DoctypeName {
		return "the doctype"
	}
	if (got.Frameset == nil) != (want.Frameset == nil) {
		return "the frameset"
	}
	if (got.Body == nil) != (want.Body == nil) || (got.Document.Body == nil) != (want.Document.Body == nil) {
		return "the body"
	}
	if d := diffNodes("html", got.Root, want.Root, got.Document, want.Document); d != "" {
		return d
	}
	// The body of each result is the body of its tree.
	if got.Body != got.Root.LastChild || want.Body != want.Root.LastChild || got.Document.Body != got.Body {
		return "the place of the body"
	}
	return ""
}

func diffNodes(path string, got, want *Node, gotDoc, wantDoc *Document) string {
	if got.Type != want.Type || got.NS != want.NS || got.Tag != want.Tag || got.Data != want.Data {
		return fmt.Sprintf("%s: node %d %q %q %q, want %d %q %q %q", path, got.Type, got.NS, got.Tag, got.Data, want.Type, want.NS, want.Tag, want.Data)
	}
	if got.doc != gotDoc || want.doc != wantDoc {
		return path + ": the document of the node"
	}
	if len(got.Attrs) != len(want.Attrs) {
		return fmt.Sprintf("%s: %d attributes, want %d", path, len(got.Attrs), len(want.Attrs))
	}
	for i := range want.Attrs {
		if got.Attrs[i] != want.Attrs[i] {
			return fmt.Sprintf("%s: attribute %d is %+v, want %+v", path, i, got.Attrs[i], want.Attrs[i])
		}
	}
	g, w := got.FirstChild, want.FirstChild
	var prev *Node
	for i := 0; g != nil || w != nil; i++ {
		if g == nil || w == nil {
			return fmt.Sprintf("%s: the number of children", path)
		}
		if g.Parent != got || g.Prev != prev {
			return fmt.Sprintf("%s: the links of child %d", path, i)
		}
		if d := diffNodes(fmt.Sprintf("%s > %s[%d]", path, w.Tag, i), g, w, gotDoc, wantDoc); d != "" {
			return d
		}
		prev = g
		g, w = g.Next, w.Next
	}
	if got.LastChild != prev {
		return path + ": the last child"
	}
	return ""
}
