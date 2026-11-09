package markdown

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The labels of the benchmarks: a typical short label, a name with an
// underscore, a label with a br tag, a label with markdown, and a label
// with line breaks.
var benchLabels = []struct{ name, text string }{
	{"plain", "Process the order"},
	{"underscore", "customer_id"},
	{"br", "Start here<br/>then go on"},
	{"emphasis", "This is **bold** and *italic* text"},
	{"lines", "First line\nsecond line with `code`\nthird <br/> line"},
}

func BenchmarkMarkdownToHTML(b *testing.B) {
	config := js.NewObj[any]()
	config.Set("markdownAutoWrap", true)
	for _, label := range benchLabels {
		b.Run(label.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				MarkdownToHTML(label.text, config)
			}
		})
	}
}

func BenchmarkMarkdownToLines(b *testing.B) {
	for _, label := range benchLabels {
		b.Run(label.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				MarkdownToLines(label.text, nil)
			}
		})
	}
}

func BenchmarkNonMarkdownToHTML(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		NonMarkdownToHTML(benchLabels[0].text)
	}
}
