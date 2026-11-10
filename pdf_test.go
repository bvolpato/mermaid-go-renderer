// These tests exercise the public PDF rendering and conversion functions.
package mermaid

import (
	"bytes"
	"testing"
)

func TestRenderPDF(t *testing.T) {
	const src = "flowchart LR\n  A[Read] --> B[Write]"
	opts := []Option{WithPage(PageMermaidCLI), WithBackgroundColor("transparent")}
	svg, err := Render(src, opts...)
	if err != nil {
		t.Fatal(err)
	}
	fit, err := SVGToPDF(svg, opts...)
	if err != nil {
		t.Fatal(err)
	}
	letter := append(opts, WithPDFPaperFormat("Letter"))
	want, err := SVGToPDF(svg, letter...)
	if err != nil {
		t.Fatal(err)
	}
	got, err := RenderPDF(src, letter...)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, []byte("%PDF-")) || !bytes.Equal(got, want) || bytes.Equal(got, fit) {
		t.Fatal("RenderPDF and SVGToPDF must produce the same PDF for the same page options and honor the paper size")
	}
	if _, err := SVGToPDF(svg, WithPDFPaperFormat("unknown")); err == nil {
		t.Error("expected an error for an unknown paper format")
	}
	if _, err := SVGToPDF("no SVG"); err == nil {
		t.Error("expected an error for input without an SVG")
	}
}
