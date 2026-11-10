// The PDF page follows renderMermaid in mermaid-cli 12.0.0 (MIT).
package mermaid

import (
	"fmt"
	"runtime/debug"

	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// WithPDFPaperFormat selects a paper size for PDF output. The names are
// Letter, Legal, Tabloid, Ledger, and A0 through A6. The default, "", fits
// the page to the diagram.
func WithPDFPaperFormat(format string) Option {
	return func(o *options) { o.pdfPaperFormat = format }
}

// RenderPDF renders a Mermaid diagram to PDF. WithPDFPaperFormat sets the
// paper size. WithBackgroundColor sets the page and diagram background.
func RenderPDF(source string, opts ...Option) ([]byte, error) {
	svg, err := Render(source, opts...)
	if err != nil {
		return nil, err
	}
	return SVGToPDF(svg, opts...)
}

// SVGToPDF prints an SVG returned by Render. Use the same page, fonts and
// viewport width as the render call so the text and diagram keep their size.
func SVGToPDF(svg string, opts ...Option) (out []byte, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.err != nil {
		return nil, o.err
	}
	defer func() {
		if p := recover(); p != nil {
			out, err = nil, &InternalError{Value: p, Stack: debug.Stack()}
		}
	}()
	page, err := mmdc.Load(svg, mmdc.Options{
		Background: o.background, SetBackground: o.hasBackground,
		Fonts: o.fontLibrary(), PageSheets: render.PageSheets(render.Page(o.page)),
	})
	if err != nil {
		return nil, fmt.Errorf("mermaid: %w", err)
	}
	return page.PDF(o.render.ViewportWidth, o.pdfPaperFormat)
}
