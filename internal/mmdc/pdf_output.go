// The PDF output follows renderMermaid in mermaid-cli 12.0.0 (MIT).
package mmdc

import "github.com/bvolpato/mermaid-go-renderer/internal/raster"

// PDF lays out the print page and writes the diagram at its print size.
func (p *Page) PDF(viewportWidth float64, paperFormat string) ([]byte, error) {
	print, err := p.Print(viewportWidth, paperFormat)
	if err != nil {
		return nil, err
	}
	return raster.RenderPDF(print.SVG, raster.Options{
		Fonts: p.opts.Fonts, PageSheets: p.opts.PageSheets, Background: print.Diagram,
	}, raster.PDFPage{
		Width: print.Width, Height: print.Height, X: print.Box.X, Y: print.Box.Y,
		Background: print.Page, Pages: print.Pages,
	})
}
