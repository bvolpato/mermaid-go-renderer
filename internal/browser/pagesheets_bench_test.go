package browser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// benchFlowchart is the markup of a flowchart with n nodes whose labels
// have no icon, with rules of the style sheet of a flowchart.
func benchFlowchart(n int) string {
	var sb strings.Builder
	sb.WriteString(`<svg id="my-svg" width="100%" xmlns="http://www.w3.org/2000/svg" class="flowchart"><style>` +
		`#my-svg{font-family:arial,sans-serif;font-size:14px;fill:#28253D;}#my-svg p{margin:0;}` +
		`#my-svg .label{font-family:arial,sans-serif;color:#28253D;}#my-svg .label text,#my-svg span{fill:#28253D;color:#28253D;}` +
		`#my-svg .node rect,#my-svg .node circle,#my-svg .node path{fill:#ffffff;stroke:#28253D;stroke-width:2px;}` +
		`#my-svg .node .label{text-align:center;}#my-svg .edgeLabel{background-color:#ccc;text-align:center;}` +
		`#my-svg .flowchart-link{stroke:#000000;fill:none;}#my-svg .edge-thickness-normal{stroke-width:2px;}` +
		`</style><g class="root"><g class="nodes">`)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, `<g class="node default" id="my-svg-flowchart-N%d-%d" transform="translate(%d, 34.5)">`+
			`<rect class="basic label-container" x="-76" y="-22.5" width="152" height="45"></rect>`+
			`<g class="label" transform="translate(-60, -10.5)"><rect></rect><foreignObject width="120" height="21">`+
			`<div xmlns="http://www.w3.org/1999/xhtml" style="display: table; white-space: nowrap; line-height: 1.5; max-width: 120px; text-align: center; width: 120px;">`+
			`<span class="nodeLabel"><p>Node %d</p></span></div></foreignObject></g></g>`, i, i, 88+160*i, i)
	}
	sb.WriteString(`</g></g></svg>`)
	return sb.String()
}

// BenchmarkCascadePageSheets measures the cascade of every element of a
// flowchart without an icon, in a plain page and in the page of
// mermaid-cli. The two must stay close: an element without a class of the
// page sheets tries none of their rules.
func BenchmarkCascadePageSheets(b *testing.B) {
	sheets := pageSheetsOfMermaidCLI(b)
	html := benchFlowchart(100)
	for _, page := range []struct {
		name   string
		sheets []string
	}{{"plain", nil}, {"mermaid-cli", sheets}} {
		b.Run(page.name, func(b *testing.B) {
			doc := NewDocumentWithFonts(font.Embedded())
			doc.PageSheets = page.sheets
			container := doc.Body.Append("div").Attr("id", "container")
			container.HTML(html)
			engine := doc.Engine.(*Engine)
			var elements []*dom.Node
			var walk func(n *dom.Node)
			walk = func(n *dom.Node) {
				if n.Type == dom.ElementNode {
					elements = append(elements, n)
				}
				for c := n.FirstChild; c != nil; c = c.Next {
					walk(c)
				}
			}
			walk(doc.Body)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for _, n := range elements {
					engine.cascade(n, &nodeStyle{})
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(len(elements)), "ns/element")
		})
	}
}

// BenchmarkLabelLayoutPageSheets measures the layout of the labels of a
// flowchart without an icon in the two pages: the rectangle of every label,
// from a new document.
func BenchmarkLabelLayoutPageSheets(b *testing.B) {
	sheets := pageSheetsOfMermaidCLI(b)
	html := benchFlowchart(100)
	for _, page := range []struct {
		name   string
		sheets []string
	}{{"plain", nil}, {"mermaid-cli", sheets}} {
		b.Run(page.name, func(b *testing.B) {
			lib := font.Embedded()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				doc := NewDocumentWithFonts(lib)
				doc.PageSheets = page.sheets
				container := doc.Body.Append("div").Attr("id", "container")
				container.HTML(html)
				for _, div := range container.SelectAll("foreignObject > div") {
					div.BoundingClientRect()
				}
			}
		})
	}
}
