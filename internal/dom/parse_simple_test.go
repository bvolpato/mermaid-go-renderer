package dom

import (
	"math/rand"
	"strings"
	"testing"
)

// simpleSeeds are documents of the kinds that the sanitizer parses: the
// SVG of a diagram and the HTML of a label.
var simpleSeeds = []string{
	`<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" class="flowchart" style="max-width: 100px;" viewBox="0 0 100 50" role="graphics-document document" aria-roledescription="flowchart-v2"><style>#my-svg{font-family:"trebuchet ms",verdana;fill:#333;}#my-svg .a>b{stroke:red;}</style><g><marker id="m" class="marker" viewBox="0 0 10 10" refX="5" refY="5" markerUnits="userSpaceOnUse" markerWidth="8" markerHeight="8" orient="auto"><path d="M 0 0 L 10 5 L 0 10 z" class="arrowMarkerPath" style="stroke-width: 1; stroke-dasharray: 1, 0;"></path></marker><g class="root"><g class="nodes"><g class="node default" id="flowchart-A-0" transform="translate(10, 20)"><rect class="basic label-container" style="" x="-10" y="-5" width="20" height="10"></rect><g class="label" style="" transform="translate(-5, -5)"><rect></rect><foreignObject width="10" height="10"><div xmlns="http://www.w3.org/1999/xhtml" style="display: table-cell; white-space: nowrap;"><span class="nodeLabel"><p>A &amp; B &lt; C</p></span></div></foreignObject></g></g></g></g></g></svg>`,
	`<svg id="s" xmlns="http://www.w3.org/2000/svg"><title id="chart-title-s">A title &amp; more</title><desc id="chart-desc-s">A description</desc><defs><linearGradient id="g" gradientUnits="objectBoundingBox" x1="0%" y1="0%"><stop offset="0%" stop-color="red"></stop></linearGradient><filter id="f"><feDropShadow dx="1" dy="1" stdDeviation="2"></feDropShadow><feGaussianBlur stdDeviation="1"></feGaussianBlur></filter><clipPath id="c"><rect width="1" height="1"></rect></clipPath></defs><text x="1" y="2" dominant-baseline="middle" text-anchor="middle" xml:space="preserve"><tspan dy="1em" x="0">line 1</tspan><tspan>line&nbsp;2 "q"</tspan></text><a xlink:href="#top" target="_blank"><path d="M0,0L1,1" marker-end="url(#m)"></path></a><image xlink:href="data:image/png;base64,AA" width="1" height="1" preserveAspectRatio="none"></image><textPath startOffset="50%" href="#p">t</textPath></svg>`,
	`<svg xmlns="http://www.w3.org/2000/svg"><g><foreignObject width="1" height="1"><div xmlns="http://www.w3.org/1999/xhtml"><span class="edgeLabel"><p>one<br>two<br/>three <b>bold <i>both</i></b> <em>em</em> <strong>strong</strong> <code>code</code> <a href="https://example.com" target="_blank" class="x">link</a> <img src="x.png" width="1" alt="i"> H<sub>2</sub>O x<sup>2</sup></p></span></div></foreignObject></g></svg>`,
	`<span class="nodeLabel"><p>Decision</p></span>`,
	`Line 1<br/>Line <b class="x">2</b>`,
	`text with &amp; and &lt;tags&gt; and "quotes"`,
	`<p>a</p><p>b <span>c</span></p> tail`,
	`<div><div><p>x</p><div>y</div></div></div>`,
	`<i class="fa fa-car"></i> Car <u>u</u> <s>s</s> <small>small</small>`,
	`<svg width="16" height="16" viewBox="0 0 16 16"><path d="M0 0h16v16H0z"></path></svg> icon`,
}

// simpleMutations are pieces of markup that a mutation puts into a seed.
// They reach the rules that make the simple parser give up: tags that the
// HTML standard moves or closes, references, other attribute forms, the
// case of names.
var simpleMutations = []string{
	"<p>", "</p>", "<div>", "</div>", "<span>", "</span>", "<b>", "</b>", "<i>", "</i>", "<a href=\"x\">", "</a>", "<br>",
	"<br/>", "</br>", "<img src=\"x\">", "<svg>", "</svg>", "<g>", "</g>", "<style>", "</style>", "<title>", "</title>",
	"<desc>", "</desc>", "<foreignObject>", "</foreignObject>", "<FOREIGNOBJECT>", "<table>", "<tr>", "<td>", "<li>", "<ul>",
	"<h1>", "<pre>\n", "<textarea>", "<script>", "</script>", "<font color=\"x\">", "<font>", "<math>", "<nobr>", "<select>",
	"<button>", "<form>", "<input>", "<hr>", "<path d=\"M0 0\"/>", "<rect/>", "<circle></circle>", "<clipPath>", "<clippath>",
	"<lineargradient>", "<feDropShadow>", "<textPath>", "<P>", "<DIV>", "<Span>", "<B>", "</B>", "<em>", "</em>", "<strong>",
	"<code>", "<u>", "<s>", "<small>", "<sub>", "<sup>", "<big>", "<label>", "<x-y>", "<a>", "<b><b><b><b>",
	"&amp;", "&lt;", "&gt;", "&quot;", "&nbsp;", "&", "& ", "&#65;", "&#x41;", "&copy;", "&copy", "&amp", "&ampx;", "&lt", "&;", "&#;",
	"<", ">", "/", "=", "\"", "'", " ", "\n", "\t", "\f", "\r", "\x00", "<!--c-->", "<!DOCTYPE html>", "<?x?>", "<![CDATA[x]]>", "</", "<1>",
	" viewBox=\"0 0 1 1\"", " VIEWBOX=\"1\"", " xlink:href=\"#a\"", " xml:base=\"b\"", " xml:lang=\"en\"", " xmlns=\"http://www.w3.org/2000/svg\"",
	" xmlns:xlink=\"x\"", " class=\"a\"", " class=\"b\"", " id='s'", " hidden", " a=b", " data-x=\"1\"", " A=\"1\"", " a.b=\"1\"", " xlink:foo=\"1\"",
	" refX=\"1\"", " refx=\"2\"", " style=\"a:b\"", " title=\"a &amp; b\"", " alt=\"a>b\"", " alt=\"a<b\"", "é", " ", "\xff", "中",
	"<a href=\"1\" class=\"2\">", "<a class=\"2\" href=\"1\">", "<b id=\"1\" class=\"2\">", "<b class=\"2\" id=\"1\">",
	// Whole elements: they keep the document well formed, so the simple
	// parser must decide by the rule for the element in its new place.
	"<p>x</p>", "<div>d</div>", "<b>bold</b>", "<span>s</span>", "<i></i>", "<em>e</em>", "<table></table>",
	"<a href=\"y\">in</a>", "<b><b><b><b>x</b></b></b></b>", "<b><b><b>x</b></b></b>", "<g><p>x</p></g>", "<svg><b>x</b></svg>",
	"<font>f</font>", "<font color=\"red\">f</font>", "<h1>h</h1>", "<li>l</li>", "<ul><li>l</li></ul>", "<code>c</code>",
	"<sub>s</sub>", "<title>t</title>", "<style>s{}</style>", "<script>s</script>", "<textarea>t</textarea>",
	"<svg><title>t</title></svg>", "<foreignObject><p>x</p></foreignObject>", "<desc><b>x</b></desc>", "<desc>d</desc>",
	"<svg><g><rect></rect></g></svg>", "<img src=\"x\">text", "<br>text", "<nobr>n</nobr>", "<label>l</label>", "<u>u</u>",
	"<a class=\"2\" href=\"1\">x</a>", "<a href=\"1\" class=\"2\">x</a>", "<strong>s</strong>", "<small>s</small>",
	"<math><mi>x</mi></math>", "<svg><foreignObject><div>d</div></foreignObject></svg>", "<pre>\nx</pre>", "<button>b</button>",
}

func mutateMarkup(rng *rand.Rand, s string) string {
	// One change in most documents: a document with many changes is rarely
	// simple.
	count := 1
	if r := rng.Intn(20); r >= 19 {
		count = 3
	} else if r >= 14 {
		count = 2
	}
	for n := count; n > 0; n-- {
		at := rng.Intn(len(s) + 1)
		switch rng.Intn(6) {
		case 0, 1, 2:
			s = s[:at] + simpleMutations[rng.Intn(len(simpleMutations))] + s[at:]
		case 3:
			// Delete a span.
			end := min(len(s), at+rng.Intn(12))
			s = s[:at] + s[end:]
		case 4:
			// Copy a span to another place.
			end := min(len(s), at+rng.Intn(40))
			to := rng.Intn(len(s) + 1)
			s = s[:to] + s[at:end] + s[to:]
		case 5:
			// Put the mutation at a tag.
			tags := strings.Count(s, "<")
			if tags == 0 {
				continue
			}
			i := 0
			for k := rng.Intn(tags); ; k-- {
				i += strings.IndexByte(s[i:], '<')
				if k == 0 {
					break
				}
				i++
			}
			if rng.Intn(2) == 0 {
				// After the tag name.
				j := i + 1
				for j < len(s) && isMarkupNameChar(s[j]) {
					j++
				}
				i = j
			}
			s = s[:i] + simpleMutations[rng.Intn(len(simpleMutations))] + s[i:]
		}
	}
	return s
}

// TestSimpleParserAgainstFullParser parses documents and many mutations of
// them. Where the simple parser gives a tree, it must be the tree of the
// full parser.
func TestSimpleParserAgainstFullParser(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	simple, total := 0, 0
	check := func(markup string) {
		total++
		got, ok := parseSimpleDocument(markup)
		if !ok {
			return
		}
		simple++
		want, err := parseHTMLDocumentFull(markup)
		if err != nil {
			t.Fatalf("the full parser rejects %q: %v", markup, err)
		}
		if d := diffDocuments(got, want); d != "" {
			t.Fatalf("the simple parser differs for\n%s\n%s", markup, d)
		}
	}
	for _, seed := range simpleSeeds {
		check(seed)
		if _, ok := parseSimpleDocument(seed); !ok {
			t.Errorf("the simple parser does not read the seed %q", seed)
		}
	}
	iterations := 150000
	if testing.Short() {
		iterations = 20000
	}
	for i := 0; i < iterations; i++ {
		check(mutateMarkup(rng, simpleSeeds[rng.Intn(len(simpleSeeds))]))
	}
	// Many mutations must still be simple, or the test does not reach the
	// rules of the simple parser.
	if simple < total/10 {
		t.Errorf("the simple parser read %d of %d documents", simple, total)
	}
	t.Logf("the simple parser read %d of %d documents", simple, total)
}

// TestSimpleParserDepth parses documents with elements inside elements, up
// to more levels than the full parser accepts. ParseHTMLDocument must give
// what the full parser gives at every depth: the same tree, or an error.
func TestSimpleParserDepth(t *testing.T) {
	simple, rejected := 0, 0
	for _, depth := range []int{1, 100, simpleMaxOpen - 2, simpleMaxOpen - 1, simpleMaxOpen, simpleMaxOpen + 1, simpleMaxOpen + 2, 400, 505, 506, 507, 508, 509, 510, 511, 512, 513, 514, 600, 5000} {
		for _, markup := range []string{
			`<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat("<g>", depth) + "<rect></rect>" + strings.Repeat("</g>", depth) + "</svg>",
			strings.Repeat("<span>", depth) + "x<br>" + strings.Repeat("</span>", depth),
			strings.Repeat("<div>", depth) + `<img src="x">` + strings.Repeat("</div>", depth),
		} {
			want, wantErr := parseHTMLDocumentFull(markup)
			got, err := ParseHTMLDocument(markup)
			if (err != nil) != (wantErr != nil) {
				t.Fatalf("depth %d: ParseHTMLDocument gives the error %v, the full parser gives %v", depth, err, wantErr)
			}
			if _, ok := parseSimpleDocument(markup); ok {
				simple++
				if wantErr != nil {
					t.Fatalf("depth %d: the simple parser reads a document that the full parser rejects", depth)
				}
			}
			if wantErr != nil {
				rejected++
				continue
			}
			if d := diffDocuments(got, want); d != "" {
				t.Fatalf("depth %d: ParseHTMLDocument differs from the full parser: %s", depth, d)
			}
		}
	}
	// The test must reach both sides of the limit.
	if simple == 0 || rejected == 0 {
		t.Fatalf("the simple parser read %d documents, the full parser rejected %d", simple, rejected)
	}
}

// TestSimpleParserGivesUp lists markup that the simple parser must leave to
// the full parser, because the HTML standard changes its tree.
func TestSimpleParserGivesUp(t *testing.T) {
	for _, markup := range []string{
		"",
		"  \n",
		"<p>a<p>b</p></p>",
		"<p><div>x</div></p>",
		"<a href=\"x\"><a href=\"y\">z</a></a>",
		"<b><i>x</b></i>",
		"<div>unclosed",
		"</div>",
		"<svg><p>x</p></svg>",
		"<svg><g/></svg>",
		"<svg><title><b>x</b></title></svg>",
		"<span title='x'>y</span>",
		"<span hidden>y</span>",
		"<span a=\"1\" a=\"2\">y</span>",
		"a &copy; b",
		"a &#65; b",
		"a &amp b",
		"<table><tr><td>x</td></tr></table>",
		"<style>p{}</style>",
		"<!-- c -->x",
		"a\r\nb<br>",
		"<span>a\x00</span>",
		"<span>\xff</span>",
		"<br></br>",
		"<a href=\"1\" class=\"2\">x</a><a class=\"2\" href=\"1\">y</a>",
	} {
		if _, ok := parseSimpleDocument(markup); ok {
			t.Errorf("the simple parser reads %q", markup)
		}
	}
}
