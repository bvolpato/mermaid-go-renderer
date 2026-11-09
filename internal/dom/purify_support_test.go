package dom

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

func mustParse(t *testing.T, markup string) *HTMLDocument {
	t.Helper()
	doc, err := ParseHTMLDocument(markup)
	if err != nil {
		t.Fatalf("ParseHTMLDocument(%q): %v", markup, err)
	}
	return doc
}

func TestDOMAttributeMethods(t *testing.T) {
	doc := mustParse(t, `<div Class="a" xlink:href="h"></div><svg viewBox="0 0 1 1" xlink:href="s" xmlns="http://www.w3.org/2000/svg"></svg>`)
	div := doc.Body.FirstChild
	svg := div.Next

	// An HTML element matches the qualified name in lowercase.
	if v, ok := div.GetAttribute("CLASS"); !ok || v != "a" {
		t.Errorf(`div.getAttribute("CLASS") = %q, %v`, v, ok)
	}
	// On an HTML element xlink:href has no namespace. GetAttr resolves the
	// prefix as d3 does and misses it. GetAttribute matches the name.
	if v, ok := div.GetAttribute("xlink:href"); !ok || v != "h" {
		t.Errorf(`div.getAttribute("xlink:href") = %q, %v`, v, ok)
	}
	// An SVG element matches the exact case.
	if svg.HasAttribute("viewbox") || !svg.HasAttribute("viewBox") {
		t.Errorf("svg viewBox lookup is not case-sensitive")
	}
	if v, ok := svg.GetAttribute("xlink:href"); !ok || v != "s" {
		t.Errorf(`svg.getAttribute("xlink:href") = %q, %v`, v, ok)
	}

	// setAttribute keeps the position of an existing attribute.
	if err := div.SetAttribute("CLASS", "b"); err != nil {
		t.Fatal(err)
	}
	if err := div.SetAttribute("Title", "t"); err != nil {
		t.Fatal(err)
	}
	if got := div.OuterHTML(); got != `<div class="b" xlink:href="h" title="t"></div>` {
		t.Errorf("after setAttribute: %s", got)
	}
	// removeAttribute and a new set move the attribute to the end.
	div.RemoveAttribute("class")
	_ = div.SetAttribute("class", "c")
	if got := div.OuterHTML(); got != `<div xlink:href="h" title="t" class="c"></div>` {
		t.Errorf("after remove and set: %s", got)
	}
	// setAttributeNS changes the attribute of the namespace in place.
	if err := svg.SetAttributeNS(NSXLink, "xlink:href", "s2"); err != nil {
		t.Fatal(err)
	}
	if err := svg.SetAttributeNS(NSXMLNS, "xmlns", "u"); err != nil {
		t.Fatal(err)
	}
	if got := svg.OuterHTML(); got != `<svg viewBox="0 0 1 1" xlink:href="s2" xmlns="u"></svg>` {
		t.Errorf("after setAttributeNS: %s", got)
	}

	for _, name := range []string{"", "a b", "a/b", "a=b", "a>b", "a\x00b", "a\tb"} {
		if err := div.SetAttribute(name, ""); !errors.Is(err, ErrInvalidCharacter) {
			t.Errorf("setAttribute(%q) = %v, want InvalidCharacterError", name, err)
		}
	}
	for _, name := range []string{"1a", "a:b", "<a", `a"b`, "data-\u00d7"} {
		if err := div.SetAttribute(name, ""); err != nil {
			t.Errorf("setAttribute(%q) = %v", name, err)
		}
	}
	nsCases := []struct {
		ns, qname string
		want      error
	}{
		{"", "a:b", ErrNamespace},
		{NSSVG, "xml:a", ErrNamespace},
		{NSSVG, "xmlns", ErrNamespace},
		{NSSVG, "xmlns:a", ErrNamespace},
		{NSXMLNS, "a", ErrNamespace},
		{NSXMLNS, "xmlns:a", nil},
		{NSXML, "xml:lang", nil},
		{NSXLink, "a b", ErrInvalidCharacter},
		{NSXLink, ":b", ErrInvalidCharacter},
	}
	for _, c := range nsCases {
		if err := svg.SetAttributeNS(c.ns, c.qname, ""); !errors.Is(err, c.want) {
			t.Errorf("setAttributeNS(%q, %q) = %v, want %v", c.ns, c.qname, err, c.want)
		}
	}
}

func TestCloneNode(t *testing.T) {
	doc := mustParse(t, `<div id="a" class="b">x<span>y</span><template><i>z</i></template><!--c--></div>`)
	div := doc.Body.FirstChild
	shallow := div.CloneNode(false)
	if got := shallow.OuterHTML(); got != `<div id="a" class="b"></div>` {
		t.Errorf("shallow clone: %s", got)
	}
	deep := div.CloneNode(true)
	if got, want := deep.OuterHTML(), div.OuterHTML(); got != want {
		t.Errorf("deep clone: %s, want %s", got, want)
	}
	if deep.Parent != nil || deep.Document() != div.Document() {
		t.Errorf("the clone must have no parent and the same document")
	}
	// The clone is independent.
	_ = deep.SetAttribute("id", "changed")
	deep.FirstChild.Data = "changed"
	if got := div.OuterHTML(); !strings.HasPrefix(got, `<div id="a" class="b">x<span>`) {
		t.Errorf("the original changed: %s", got)
	}
	for c := deep.FirstChild; c != nil; c = c.Next {
		if c.Parent != deep {
			t.Errorf("child %s has a wrong parent", c.OuterHTML())
		}
	}
}

// names lists the nodes that an iterator returns.
func names(it *NodeIterator) string {
	var out []string
	for n := it.NextNode(); n != nil; n = it.NextNode() {
		switch n.Type {
		case ElementNode:
			out = append(out, n.Tag)
		case TextNode:
			out = append(out, "#"+n.Data)
		case CommentNode:
			out = append(out, "!"+n.Data)
		}
	}
	return strings.Join(out, " ")
}

func TestNodeIterator(t *testing.T) {
	const markup = `<a>1<b>2<c></c></b><!--k--><template><d>3</d><template><e></e></template></template></a><f></f>`
	doc := mustParse(t, markup)
	all := ShowElement | ShowText | ShowComment
	// The root is the first node. The contents of a template are not part of
	// the tree of the document.
	if got, want := names(NewNodeIterator(doc.Body, all)), "body a #1 b #2 c !k template f"; got != want {
		t.Errorf("all: %q, want %q", got, want)
	}
	if got, want := names(NewNodeIterator(doc.Body, ShowElement)), "body a b c template f"; got != want {
		t.Errorf("elements: %q, want %q", got, want)
	}
	if got, want := names(NewNodeIterator(doc.Body, ShowText|ShowComment)), "#1 #2 !k"; got != want {
		t.Errorf("text and comments: %q, want %q", got, want)
	}
	// An iterator over the contents of a template does not return the
	// fragment, and stops at a nested template.
	template := doc.Body.FirstChild.LastChild
	if got, want := names(NewContentIterator(template, all)), "d #3 template"; got != want {
		t.Errorf("contents: %q, want %q", got, want)
	}
}

// TestNodeIteratorPreRemove checks the pre-removing steps with the sequence
// that DOMPurify uses: the children of the current node are inserted after
// it, and then the current node is removed.
func TestNodeIteratorPreRemove(t *testing.T) {
	doc := mustParse(t, `<a><x><b>1</b><c></c></x><d></d></a>`)
	it := NewNodeIterator(doc.Body, ShowElement|ShowText)
	var seen []string
	for n := it.NextNode(); n != nil; n = it.NextNode() {
		if n.Type == TextNode {
			seen = append(seen, "#"+n.Data)
			continue
		}
		seen = append(seen, n.Tag)
		if n.Tag == "x" {
			parent := n.Parent
			var children []*Node
			for c := n.FirstChild; c != nil; c = c.Next {
				children = append(children, c)
			}
			for i := len(children) - 1; i >= 0; i-- {
				parent.InsertBefore(children[i].CloneNode(true), n.Next)
			}
			it.PreRemove(n)
			n.Remove()
		}
	}
	if got, want := strings.Join(seen, " "), "body a x b #1 c d"; got != want {
		t.Errorf("visited %q, want %q", got, want)
	}
	if got, want := doc.Body.InnerHTML(), `<a><b>1</b><c></c><d></d></a>`; got != want {
		t.Errorf("tree %q, want %q", got, want)
	}

	// The removed node has a previous sibling: the reference becomes the
	// last descendant of that sibling.
	doc = mustParse(t, `<p><q><r></r></q><x><s></s></x><u></u></p>`)
	it = NewNodeIterator(doc.Body, ShowElement)
	seen = seen[:0]
	for n := it.NextNode(); n != nil; n = it.NextNode() {
		seen = append(seen, n.Tag)
		if n.Tag == "x" {
			it.PreRemove(n)
			n.Remove()
		}
	}
	if got, want := strings.Join(seen, " "), "body p q r x u"; got != want {
		t.Errorf("visited %q, want %q", got, want)
	}

	// Removing an ancestor of the reference moves the reference out of the
	// removed subtree.
	doc = mustParse(t, `<p><q><r></r></q><u></u></p>`)
	it = NewNodeIterator(doc.Body, ShowElement)
	seen = seen[:0]
	for n := it.NextNode(); n != nil; n = it.NextNode() {
		seen = append(seen, n.Tag)
		if n.Tag == "r" {
			q := n.Parent
			it.PreRemove(q)
			q.Remove()
		}
	}
	if got, want := strings.Join(seen, " "), "body p q r u"; got != want {
		t.Errorf("visited %q, want %q", got, want)
	}

	// The root is never left, also when it is removed from its parent.
	doc = mustParse(t, `<p></p><q></q>`)
	it = NewNodeIterator(doc.Body, ShowElement)
	it.NextNode()
	it.PreRemove(doc.Body)
	doc.Body.Remove()
	if got, want := names(it), "p q"; got != want {
		t.Errorf("after the removal of the root: %q, want %q", got, want)
	}
}

func TestParseHTMLDocument(t *testing.T) {
	cases := []struct{ markup, body string }{
		{`<b>x</b>`, `<b>x</b>`},
		{`  <b>x</b>`, `<b>x</b>`},
		{`<title>t</title><p class="a">x`, `<p class="a">x</p>`},
		// Scripting is off: the content of noscript is markup.
		{`a<noscript><b>x</b></noscript>`, `a<noscript><b>x</b></noscript>`},
		{`a<noscript>&lt;b</noscript>`, `a<noscript>&lt;b</noscript>`},
		// The attributes of the body element are kept.
		{`<body class="c"><i>x</i>`, `<i>x</i>`},
	}
	for _, c := range cases {
		doc := mustParse(t, c.markup)
		if doc.Body == nil {
			t.Errorf("%q: no body", c.markup)
			continue
		}
		if got := doc.Body.InnerHTMLInert(); got != c.body {
			t.Errorf("%q: body %q, want %q", c.markup, got, c.body)
		}
		if doc.Root == nil || doc.Root.Tag != "html" || doc.Body.Parent != doc.Root {
			t.Errorf("%q: the body is not a child of the html element", c.markup)
		}
	}
	doc := mustParse(t, `<body class="c" id="i">x`)
	if got := doc.Body.OuterHTML(); got != `<body class="c" id="i">x</body>` {
		t.Errorf("body attributes: %s", got)
	}
	doc = mustParse(t, `<!DOCTYPE html><frameset><frame></frameset>`)
	if doc.Body != nil || doc.Frameset == nil || !doc.HasDoctype || doc.DoctypeName != "html" {
		t.Errorf("frameset document: body %v, frameset %v, doctype %q", doc.Body, doc.Frameset, doc.DoctypeName)
	}
	if doc.Document.Body == nil {
		t.Errorf("Document.Body must not be nil")
	}
	// Scripting is on in ParseBody: the content of noscript is text.
	live, err := ParseBody(`a<noscript><b>x</b></noscript>`)
	if err != nil {
		t.Fatal(err)
	}
	if n := live.Body.FirstChild.Next; n.FirstChild == nil || n.FirstChild.Type != TextNode {
		t.Errorf("ParseBody must parse noscript as text")
	}
}

// TestParserRepairs checks the repairs that ParseHTMLDocument makes to the
// tree of golang.org/x/net/html. Each case is a difference from Chromium
// that ParseBody and Node.HTML still have.
func TestParserRepairs(t *testing.T) {
	cases := []struct{ name, markup, want string }{
		{"attribute order of a formatting element", `<a href="h" class="c" target="t">x</a>`, `<a href="h" class="c" target="t">x</a>`},
		{"attribute order, every formatting element", `<b z="1" a="2"></b><i z="1" a="2"></i><em z="1" a="2"></em><strong z="1" a="2"></strong><font size="1" color="c"></font><code z="1" a="2"></code><u z="1" a="2"></u><s z="1" a="2"></s><small z="1" a="2"></small><big z="1" a="2"></big><tt z="1" a="2"></tt><strike z="1" a="2"></strike><nobr z="1" a="2"></nobr>`, ""},
		{"attribute order of a reconstructed element", `<p><b z="1" a="2">x</p>y`, `<p><b z="1" a="2">x</b></p><b z="1" a="2">y</b>`},
		{"attribute order with a character reference", `<a title="a&amp;b" href="x">l</a>`, `<a title="a&amp;b" href="x">l</a>`},
		{"attribute order in foreign content", `<svg><foreignObject><a z="1" a="2">x</a></foreignObject></svg>`, `<svg><foreignObject><a z="1" a="2">x</a></foreignObject></svg>`},
		{"attribute order after raw text", `<style>a<b</style><b z="1" a="2">x</b>`, ""},
		{"same attributes in two orders", `<b z="1" a="2">x</b><b a="2" z="1">y</b>`, `<b z="1" a="2">x</b><b a="2" z="1">y</b>`},
		{"U+0000 in an attribute value", "<p title=\"a\x00b\">x</p>", "<p title=\"a\uFFFDb\">x</p>"},
		{"feDropShadow", `<svg><filter><fedropshadow dx="1"></fedropshadow></filter></svg>`, `<svg><filter><feDropShadow dx="1"></feDropShadow></filter></svg>`},
	}
	for _, c := range cases {
		doc := mustParse(t, c.markup)
		want := c.want
		if want == "" {
			want = c.markup
			if i := strings.Index(want, "</style>"); i >= 0 {
				// The style element goes to the head.
				want = want[i+len("</style>"):]
			}
		}
		if got := doc.Body.InnerHTMLInert(); got != want {
			t.Errorf("%s:\n  got  %s\n  want %s", c.name, got, want)
		}
	}
}

func TestInnerHTMLInert(t *testing.T) {
	d := NewDocument()
	noscript := d.Body.Append("noscript")
	noscript.AppendChild(d.CreateText("<b>&"))
	style := d.Body.Append("style")
	style.AppendChild(d.CreateText("a>b{}"))
	// A document with a browsing context writes the text of noscript raw.
	if got, want := d.Body.InnerHTML(), `<noscript><b>&</noscript><style>a>b{}</style>`; got != want {
		t.Errorf("InnerHTML: %s, want %s", got, want)
	}
	// A document without one escapes it. Other raw text elements stay raw.
	if got, want := d.Body.InnerHTMLInert(), `<noscript>&lt;b&gt;&amp;</noscript><style>a>b{}</style>`; got != want {
		t.Errorf("InnerHTMLInert: %s, want %s", got, want)
	}
	if got, want := noscript.OuterHTMLInert(), `<noscript>&lt;b&gt;&amp;</noscript>`; got != want {
		t.Errorf("OuterHTMLInert: %s, want %s", got, want)
	}
}

// The tree of golang.org/x/net/html differs from the tree of Chromium for
// some inputs, and no repair after the parse is possible. Each class below
// needs a change of the parser. TestGoldenParse assigns every differing
// input of the golden to one class, and fails for an input that fits none.

var (
	dumpComment = regexp.MustCompile(`<!--"(?:[^"\\]|\\.)*"-->`)
	dumpPI      = regexp.MustCompile(`<\?"(?:[^"\\]|\\.)*" "(?:[^"\\]|\\.)*">`)
	// dumpTextJoin is the place where two text nodes meet in a dump. The
	// quotes of an empty attribute value follow an equals sign.
	dumpTextJoin = regexp.MustCompile(`([^=])""`)
	// inputPI is the start of a processing instruction in an input.
	inputPI = regexp.MustCompile(`<\?[a-zA-Z]`)
	// dumpSpaceText is a text node of white space only.
	dumpSpaceText = regexp.MustCompile(`"(?: |\\u0009|\\u000a|\\u000c)+"`)
)

// withoutComments removes the comments and the processing instructions from
// a dump, and joins the text nodes that then are adjacent.
func withoutComments(dump string) string {
	dump = dumpPI.ReplaceAllString(dumpComment.ReplaceAllString(dump, ""), "")
	return dumpTextJoin.ReplaceAllString(dump, "$1")
}

// parserDivergence is one class of difference.
type parserDivergence struct {
	name string
	// match reports whether the input, the tree of the port and the tree of
	// Chromium belong to the class.
	match func(input, got, want string) bool
}

func inputMatches(pattern string) func(input, got, want string) bool {
	re := regexp.MustCompile(pattern)
	return func(input, _, _ string) bool { return re.MatchString(input) }
}

// knownParserDivergences lists the classes. The first class that matches
// counts.
var knownParserDivergences = []parserDivergence{
	// x/net/html rejects markup that nests more than 512 elements. Chromium
	// keeps the content and adds the deeper elements as siblings.
	{"nesting deeper than 512 elements", func(_, got, _ string) bool { return got == "error" }},
	// Chromium 156 makes a ProcessingInstruction node for `<?name data?>`,
	// and nothing when the input ends inside it. x/net/html makes a comment,
	// as the HTML standard said before.
	{"processing instruction", func(input, got, want string) bool {
		return (dumpPI.MatchString(want) || inputPI.MatchString(input)) && withoutComments(got) == withoutComments(want)
	}},
	// x/net/html makes an empty comment for `</>`. The standard and Chromium
	// drop it.
	{"empty end tag", func(input, got, want string) bool {
		return strings.Contains(input, "</>") && withoutComments(got) == withoutComments(want)
	}},
	// x/net/html decodes character references in the data of a comment. The
	// standard and Chromium keep the text as it is.
	{"character reference in a comment", func(input, got, want string) bool {
		return strings.Contains(input, "&") && withoutComments(got) == withoutComments(want)
	}},
	// Chromium implements the parser change for the customizable select
	// element (HTML standard, 2025): the "in select" insertion modes are
	// gone, and a select keeps most elements as children. x/net/html drops
	// every tag inside a select except option, optgroup and hr.
	{"select content model", inputMatches(`(?i)<select|<option|<optgroup`)},
	// Chromium nests a button start tag inside an open button.
	{"nested button", inputMatches(`(?is)<button.*<button`)},
	// x/net/html decodes character references inside a CDATA section of
	// foreign content. The standard and Chromium keep the text as it is.
	{"character reference in CDATA", inputMatches(`<!\[CDATA\[[^\]]*&`)},
	// Chromium does not put a formatting element that has an is attribute in
	// the list of active formatting elements: the element is not
	// reconstructed and the adoption agency algorithm does not run for it.
	{"formatting element with an is attribute", inputMatches(`(?is)<(a|b|big|code|em|font|i|nobr|s|small|strike|strong|tt|u)\s[^>]*\bis\b`)},
	// x/net/html drops a line feed that is the first text of a pre or listing
	// element also when another token came between the start tag and the
	// text. The standard drops it only when it is the next token.
	{"line feed after pre or listing", inputMatches(`(?is)<(pre|listing)\b`)},
	// The tokenizer of the standard replaces U+0000 in a tag name with
	// U+FFFD. x/net/html ends the tag name there.
	{"U+0000 in a tag name", inputMatches(`<[a-zA-Z][^\s>/]*\x00`)},
	// x/net/html puts white space after a noscript element of the head. The
	// standard puts it into the element.
	{"white space in a noscript element of the head", inputMatches(`(?is)<noscript`)},
	// x/net/html loses the contents of a template that is open at the end of
	// the input inside an HTML integration point of SVG.
	{"template in foreign content", inputMatches(`(?is)<svg.*<template`)},
	// The white space of a frameset document goes to other places. Such a
	// document has no body, so the sanitizer returns "" for it.
	{"white space in a frameset document", func(input, got, want string) bool {
		return strings.Contains(strings.ToLower(input), "<frameset") &&
			dumpSpaceText.ReplaceAllString(got, "") == dumpSpaceText.ReplaceAllString(want, "")
	}},
}

// minEqualTrees is the share of the inputs of the parser golden whose tree
// must equal the tree of Chromium. The measured share is 5233 of 5367.
const minEqualTrees = 0.975

// TestGoldenParse compares the tree of ParseHTMLDocument with the tree that
// DOMParser builds in Chromium (tools/reference/gen-purify.mjs), for the
// inputs of the DOMPurify goldens.
func TestGoldenParse(t *testing.T) {
	f, err := os.Open("testdata/purify_parse.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Inputs []string `json:"inputs"`
		Trees  []string `json:"trees"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	counts := make([]int, len(knownParserDivergences))
	passed, unknown := 0, 0
	for i, input := range golden.Inputs {
		got := "error"
		doc, err := ParseHTMLDocument(input)
		if err == nil {
			got = doc.Root.DumpTree()
		}
		if got == golden.Trees[i] {
			passed++
			continue
		}
		class := -1
		for k, d := range knownParserDivergences {
			if d.match(input, got, golden.Trees[i]) {
				class = k
				break
			}
		}
		if class >= 0 {
			counts[class]++
			continue
		}
		unknown++
		if unknown <= 10 {
			t.Errorf("tree differs from Chromium\n  input %q\n  got   %s\n  want  %s", input, got, golden.Trees[i])
		}
	}
	t.Logf("parse: %d of %d trees equal Chromium", passed, len(golden.Inputs))
	// A lower count means that a repair of ParseHTMLDocument broke.
	if rate := float64(passed) / float64(len(golden.Inputs)); rate < minEqualTrees {
		t.Errorf("parse: %.4f of the trees equal Chromium, want at least %.4f", rate, minEqualTrees)
	}
	for k, d := range knownParserDivergences {
		t.Logf("parse: %d trees differ, class %q", counts[k], d.name)
	}
	if unknown > 0 {
		t.Errorf("parse: %d trees differ for an unknown reason", unknown)
	}
}
