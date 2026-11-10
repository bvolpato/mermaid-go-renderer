// Ported from mermaid-cli 12.0.0 src/index.js (MIT), Puppeteer 25.16.0
// common/PDFOptions.ts (Apache-2.0), and Chromium 156 printing:
// components/printing/browser/print_to_pdf/pdf_print_utils.cc (BSD-3-Clause)
// and Blink core/frame/local_frame.cc (LGPL-2.0-or-later).
// mermaid-cli 12.2.0 uses the same print options and Puppeteer version.
package mmdc

import (
	"errors"
	"image/color"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// PrintPage is the geometry of the page that mmdc prints for a PDF.
type PrintPage struct {
	SVG string
	// Width and Height are the final PDF MediaBox dimensions in CSS px.
	// Print layout uses whole points. The PDF paper also rounds at 300 dpi.
	Width, Height float64
	// Pages includes blank pages caused by inline SVG fragmentation.
	Pages int
	// Box is the diagram in the continuous sequence of pages, in CSS px.
	// A diagram that starts on the second page has Y equal to Height.
	Box dom.Rect
	// Page and Diagram are the backgrounds outside and inside Box.
	Page, Diagram color.NRGBA
}

// Print computes the page that mmdc prints. An empty paperFormat fits the
// paper to the screen measurement and prints only the first page. A named
// format uses Puppeteer's paper dimensions and allows more pages. It changes
// the page: call it last.
//
// Print Pages sheets. Move the diagram up by Height for each subsequent
// sheet. Pages includes the descent of the SVG's inline line box.
// Dimensions above 1,048,576 CSS px or more than 1,000 pages return an error.
func (p *Page) Print(viewportWidth float64, paperFormat string) (*PrintPage, error) {
	if p == nil || p.SVG == nil || p.engine == nil {
		return nil, errors.New("svg not found")
	}
	if math.IsNaN(viewportWidth) || math.IsInf(viewportWidth, 0) || math.Abs(viewportWidth) > maxPrintDimension || p.opts.Size < 0 || p.opts.Size > maxPrintDimension {
		return nil, errors.New("print viewport or size exceeds the supported range")
	}
	var width, height float64
	if paperFormat != "" {
		var ok bool
		width, height, ok = paperSize(paperFormat)
		if !ok {
			return nil, errors.New("Unknown paper format: " + paperFormat)
		}
	}
	if !(viewportWidth > 0) {
		viewportWidth = DefaultViewportWidth
	}
	p.engine.ViewportWidth = viewportWidth
	rect := p.printClientRect()
	if !validPrintRect(rect) {
		return nil, errors.New("the svg element has no size or exceeds the supported print range")
	}
	if paperFormat == "" {
		// mmdc passes these strings to page.pdf, which converts px to inches.
		width = (math.Ceil(rect.Width) + float64(rect.X*2)) / 96
		height = (math.Ceil(rect.Height) + float64(rect.Y*2)) / 96
	}
	// GetPrintPagesParams converts inches to a gfx::SizeF in points and
	// rounds each dimension up before Blink enters print layout.
	width = math.Ceil(float64(float32(width*72))) * 96 / 72
	height = math.Ceil(float64(float32(height*72))) * 96 / 72
	// PhysicalSize::FromSizeFRound rounds to the nearest LayoutUnit.
	layoutWidth := printLayoutUnit(float32(width))
	layoutHeight := printLayoutUnit(float32(height))
	if width > maxPrintDimension || height > maxPrintDimension {
		return nil, errors.New("print paper exceeds the supported range")
	}
	// The frame viewport rounds the paper width up to whole CSS pixels.
	// Fragmentainers retain their fractional LayoutUnit dimensions.
	p.engine.ViewportWidth = math.Ceil(width)
	rect = p.printClientRect()
	if !validPrintRect(rect) {
		return nil, errors.New("the svg element has no size or exceeds the supported print range")
	}
	scale := float32(1)
	if paperFormat != "" && rect.X+rect.Width > layoutWidth {
		// StartPrinting expands the layout viewport for horizontal
		// overflow, up to PrintingMaximumShrinkFactor (1.5).
		shrink := min(float32(rect.X+rect.Width)/float32(layoutWidth), float32(1.5))
		scale = 1 / shrink
		p.engine.ViewportWidth = math.Ceil(float64(float32(width) * shrink))
		layoutHeight = printLayoutUnit(float32(layoutHeight) * shrink)
		rect = p.printClientRect()
		if !validPrintRect(rect) {
			return nil, errors.New("the svg element exceeds the supported print range")
		}
	}

	page := &PrintPage{Width: printMediaDimension(width), Height: printMediaDimension(height), Pages: 1, Box: dom.Rect{
		X: float64(float32(rect.X) * scale), Y: float64(float32(rect.Y) * scale),
		Width: float64(float32(rect.Width) * scale), Height: float64(float32(rect.Height) * scale),
	}}
	_, page.Page, page.Diagram = backgroundColors(p.opts)
	if paperFormat != "" {
		descent := p.printLineDescent()
		start := rect.Y
		if rect.Y+rect.Height+descent > layoutHeight {
			// Blink moves the inline replaced element and its line box to
			// the next fragmentainer. The next page has no top margin.
			page.Box.Y = page.Height
			start = layoutHeight
		}
		count := math.Ceil((start + rect.Height + descent) / layoutHeight)
		if !(count >= 1) || count > maxPrintPages {
			return nil, errors.New("print page count exceeds the supported range")
		}
		page.Pages = int(count)
	}

	// The rasterizer receives the used size, with backgrounds painted by
	// the PDF page. Unlike Screenshot, PDF does not adjust --size by ratio.
	svg := p.SVG
	svg.Attr("width", js.Str(page.Box.Width))
	svg.Attr("height", js.Str(page.Box.Height))
	svg.Style("max-width", nil)
	svg.Style("max-height", nil)
	svg.Style("background-color", nil)
	page.SVG = svg.OuterHTML()
	return page, nil
}

// printMediaDimension keeps print layout separate from the PDF paper.
// Chromium converts whole points to 300 dpi device pixels and rounds them.
// Skia converts those pixels back to points with float32 arithmetic.
func printMediaDimension(cssPixels float64) float64 {
	points := float32(cssPixels * 72 / 96)
	devicePixels := float32(math.Floor(float64(points*float32(300.0/72.0)) + 0.5))
	return float64(devicePixels*float32(72.0/300.0)) * 96 / 72
}

func printLayoutUnit(pixels float32) float64 {
	return math.Floor(float64(float64(pixels)*64)+0.5) / 64
}

// The bounds keep downstream PDF dimensions and page loops finite.
const (
	maxPrintDimension = 1 << 20
	maxPrintPages     = 1000
)

func validPrintRect(rect dom.Rect) bool {
	for _, v := range []float64{rect.X, rect.Y, rect.Width, rect.Height} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > maxPrintDimension {
			return false
		}
	}
	return rect.Width > 0 && rect.Height > 0
}

// printLineDescent follows lineExtent for the default mmdc container font.
func (p *Page) printLineDescent() float64 {
	m, _, ok := p.engine.RasterPrimaryMetrics(p.SVG.Parent)
	if !ok {
		return 0
	}
	leading := m.LineSpacing - (m.Ascent + m.Descent)
	return m.Descent + (leading - math.Floor(leading/2))
}

// printClientRect also lays out the baseline when max-height limits the SVG.
func (p *Page) printClientRect() dom.Rect {
	rect := p.SVG.BoundingClientRect()
	if limit, ok := pxValue(p.SVG.StyleValue("max-height")); ok && rect.Height > limit {
		old, set := p.SVG.GetAttr("height")
		p.SVG.Attr("height", js.Str(limit))
		rect = p.SVG.BoundingClientRect()
		if set {
			p.SVG.Attr("height", old)
		} else {
			p.SVG.Attr("height", nil)
		}
	}
	return rect
}

// paperSize is Puppeteer's paperFormats table, in inches. The decimal
// values are upstream's rounded values, not conversions from millimeters.
func paperSize(format string) (width, height float64, ok bool) {
	switch strings.ToLower(format) {
	case "letter":
		return 8.5, 11, true
	case "legal":
		return 8.5, 14, true
	case "tabloid":
		return 11, 17, true
	case "ledger":
		return 17, 11, true
	case "a0":
		return 33.1102, 46.811, true
	case "a1":
		return 23.3858, 33.1102, true
	case "a2":
		return 16.5354, 23.3858, true
	case "a3":
		return 11.6929, 16.5354, true
	case "a4":
		return 8.2677, 11.6929, true
	case "a5":
		return 5.8268, 8.2677, true
	case "a6":
		return 4.1339, 5.8268, true
	}
	return 0, 0, false
}
