// Ported from mermaid-cli 12.0.0, src/index.js (renderMermaid).
// Checked against 12.2.0. The render steps and metadata are unchanged.

// Package mmdc has what mermaid-cli (mmdc) does with a diagram in its page
// after mermaid.render() returned it. The command mmdg and the function
// StandaloneSVG of the root package use it, so that their output files have
// the content of the files that mmdc writes.
//
// mmdc does not write the string that mermaid.render() returns. It makes
// the string the content of div#container, changes the style of the svg
// element, and then writes the XML serialization of the element, or takes a
// screenshot of it.
package mmdc

import (
	"errors"
	"image/color"
	"math"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// DefaultViewportWidth is the width of the window of mmdc in CSS px. mmdc
// 12 does not set a size, so the window has the default size of headless
// Chromium, 800 by 600. mmdc 11 had the same values as the defaults of its
// options --width and --height.
const DefaultViewportWidth = 800

// Options are the options of mmdc that change the svg element.
type Options struct {
	// Background is the value of --backgroundColor: a CSS color.
	Background string
	// SetBackground is false to keep the style of the svg element as it is.
	// mmdc always sets the background: its default is "white".
	SetBackground bool
	// Size is the value of --size in CSS px, or 0 without the option.
	Size int
	// Fonts is the font library that laid out the diagram. Screenshot and
	// EmbedFonts use it. Nil is the library of the system.
	Fonts *font.Library
	// PageSheets is the text of the style sheets in the head of the page,
	// in their order. A plain page has none. The page of mermaid-cli has
	// the style sheets of internal/webfonts (render.PageSheets).
	PageSheets []string
}

// Page is the page of mmdc after the diagram is in it.
type Page struct {
	// SVG is the svg element of the diagram.
	SVG *dom.Node

	engine *browser.Engine
	opts   Options
}

// Load puts the result of mermaid.render() into the page and runs the steps
// of mmdc that all output formats share.
func Load(svgText string, opts Options) (*Page, error) {
	engine := browser.NewWithFonts(opts.Fonts)
	doc := dom.NewDocument()
	doc.Engine = engine
	doc.PageSheets = opts.PageSheets
	// The page of mmdc: body > div#container.
	container := doc.Body.Append("div").Attr("id", "container")
	// container.innerHTML = svgText
	container.HTML(svgText)
	// const svg = container.getElementsByTagName?.("svg")?.[0]
	svg := container.Select("svg")
	if svg == nil {
		return nil, errors.New("svg not found")
	}
	if opts.SetBackground {
		// svg.style.backgroundColor = backgroundColor
		setBackgroundColor(svg, opts.Background)
	}
	if opts.Size > 0 {
		// If the SVG has a maxWidth, set it to 4K resolution
		size := strconv.Itoa(opts.Size) + "px"
		svg.Style("max-width", size)
		svg.Style("max-height", size)
		// in Mermaid v10.9.1, user journey diagrams have a fixed height
		svg.Attr("height", nil)
	}
	return &Page{SVG: svg, engine: engine, opts: opts}, nil
}

// setBackgroundColor is svg.style.backgroundColor = value.
func setBackgroundColor(svg *dom.Node, value string) {
	if value != "" && strings.TrimSpace(value) == "" {
		// Only the empty string removes the property. White space is a
		// value that is not valid, which changes nothing.
		return
	}
	svg.Style("background-color", quirkyColor(value))
}

// quirkyColor adds the # to a hex color that has none. Blink parses the
// inline style of an element that is not an HTML element in quirks mode
// (Element::EnsureMutableInlineStyle), and quirks mode accepts such a color
// (the hashless hex color quirk): "F0F0F0" is "#F0F0F0".
//
// A text of 3 or 6 hex digits is a color (the fast path of the parser of
// Blink). Other texts are colors when the CSS tokenizer reads them as a
// whole number, or as a whole number with a unit: they get zeros in front,
// up to 6 digits. A number with an exponent, such as 12e3, is not a whole
// number.
func quirkyColor(value string) string {
	s := strings.TrimSpace(value)
	if s == "" || len(s) > 6 {
		return value
	}
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !isDigit(c) && !(c >= 'a' && c <= 'f') && !(c >= 'A' && c <= 'F') {
			return value
		}
	}
	if len(s) == 3 || len(s) == 6 {
		return "#" + s
	}
	number := 0
	for number < len(s) && isDigit(s[number]) {
		number++
	}
	if number == 0 {
		// An identifier of another length.
		return value
	}
	if rest := s[number:]; len(rest) > 1 && (rest[0] == 'e' || rest[0] == 'E') && isDigit(rest[1]) {
		return value
	}
	return "#" + strings.Repeat("0", 6-len(s)) + s
}

// XML is what mmdc writes to an .svg file: the XML serialization of the svg
// element. By default mmdc embeds the fonts of its page into the SVG
// first: call EmbedFonts before XML for that file. Without the call the
// result is the file of mmdc --no-font-embed.
func (p *Page) XML() string {
	return p.SVG.XML()
}

// Title is the accessible title of the diagram: the text of the title
// element when it is the first child of the svg element. It is "" without
// such an element.
func (p *Page) Title() string {
	// if (svg.firstChild instanceof SVGTitleElement)
	if first := p.SVG.FirstChild; isSVG(first, "title") {
		return first.TextContent()
	}
	return ""
}

// Desc is the accessible description of the diagram: the text of the last
// desc element among the children of the svg element. It is "" without such
// an element.
func (p *Page) Desc() string {
	desc := ""
	for c := p.SVG.FirstChild; c != nil; c = c.Next {
		// if (svgNode instanceof SVGDescElement)
		if isSVG(c, "desc") {
			desc = c.TextContent()
		}
	}
	return desc
}

func isSVG(n *dom.Node, tag string) bool {
	return n != nil && n.Type == dom.ElementNode && n.NS == dom.NSSVG && n.Tag == tag
}

// Screenshot is the geometry of the screenshot that mmdc takes for a PNG.
type Screenshot struct {
	// Width and Height are the size of the image in CSS px: the clip of
	// the screenshot. The image has the device scale factor times as many
	// pixels.
	Width, Height int
	// Box is the box of the svg element inside the image, in CSS px.
	Box dom.Rect
	// SVG is the diagram with the size of Box as its width and height, for
	// the rasterizer.
	SVG string
	// Transparent is true when the image has no background.
	Transparent bool
	// Page is the color of the page: what the image shows outside of Box.
	// Diagram is the color below the diagram, inside of Box. They differ
	// for a background that is partly transparent, which the page and the
	// svg element both paint.
	Page, Diagram color.NRGBA
}

// Screenshot computes what mmdc captures for a PNG, for a window that is
// viewportWidth CSS px wide. It changes the page: call it last.
//
// mmdc measures the svg element, and then makes the window as large as the
// right and bottom edges of the element before it takes the screenshot.
// The body has a margin of 8 px on each side, so the block of the diagram
// becomes 8 px narrower than the element was. An svg element with
// width="100%", which is what mermaid makes, shrinks with it: the image
// has the size of the first measurement, and shows the diagram a little
// smaller, with a strip of background at the right and at the bottom.
func (p *Page) Screenshot(viewportWidth float64) (*Screenshot, error) {
	svg := p.SVG
	if p.opts.Size > 0 {
		// const aspectRatio = svg.viewBox.baseVal.width / svg.viewBox.baseVal.height
		// Browsers always use 100% width by default, so we need to
		// decrease the width if the height would be 100%.
		if w, h, ok := viewBoxSize(svg); ok {
			if aspectRatio := w / h; aspectRatio < 1 {
				svg.Style("max-width", js.Str(js.Round(float64(float64(p.opts.Size)*aspectRatio)))+"px")
			}
		}
	}
	if !(viewportWidth > 0) {
		viewportWidth = DefaultViewportWidth
	}
	// const react = svg.getBoundingClientRect()
	p.engine.ViewportWidth = viewportWidth
	rect := p.clientRect()
	clipX, clipY := math.Floor(rect.X), math.Floor(rect.Y)
	clipW, clipH := math.Ceil(rect.Width), math.Ceil(rect.Height)
	if !(clipW > 0) || !(clipH > 0) {
		return nil, errors.New("the svg element has no size")
	}
	// page.setViewport({ ...viewport, width: clip.x + clip.width, height: clip.y + clip.height })
	p.engine.ViewportWidth = clipX + clipW
	rect = p.clientRect()

	shot := &Screenshot{
		Width:  int(clipW),
		Height: int(clipH),
		Box:    dom.Rect{X: rect.X - clipX, Y: rect.Y - clipY, Width: rect.Width, Height: rect.Height},
	}
	shot.Transparent, shot.Page, shot.Diagram = backgroundColors(p.opts)

	// The rasterizer paints an svg element at its natural size. Give the
	// element the size of its box as that size.
	svg.Attr("width", js.Str(rect.Width))
	svg.Attr("height", js.Str(rect.Height))
	svg.Style("max-width", nil)
	svg.Style("max-height", nil)
	svg.Style("background-color", nil)
	shot.SVG = svg.OuterHTML()
	return shot, nil
}

// clientRect is svg.getBoundingClientRect(). The layout engine does not
// read max-height, which --size sets: the limit is applied here.
func (p *Page) clientRect() dom.Rect {
	rect := p.SVG.BoundingClientRect()
	if limit, ok := pxValue(p.SVG.StyleValue("max-height")); ok && rect.Height > limit {
		rect.Height = limit
	}
	return rect
}

// viewBoxSize is the width and the height of svg.viewBox.baseVal. An
// SVGRect holds single-precision numbers.
func viewBoxSize(svg *dom.Node) (w, h float64, ok bool) {
	fields := strings.FieldsFunc(svg.AttrOr("viewBox", ""), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	})
	if len(fields) != 4 {
		return 0, 0, false
	}
	w32, err1 := strconv.ParseFloat(fields[2], 32)
	h32, err2 := strconv.ParseFloat(fields[3], 32)
	if err1 != nil || err2 != nil || !(w32 >= 0) || !(h32 > 0) {
		return 0, 0, false
	}
	return w32, h32, true
}

func pxValue(v string) (float64, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasSuffix(v, "px") {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
	return f, err == nil
}

// backgroundColors resolves --backgroundColor for a screenshot. mmdc sets
// the color as the background of the body and of the svg element. It asks
// for an image without a background only for the exact value "transparent".
// The page of a browser is white below the body.
func backgroundColors(opts Options) (transparent bool, page, diagram color.NRGBA) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if !opts.SetBackground {
		return false, white, white
	}
	if opts.Background == "transparent" {
		return true, color.NRGBA{}, color.NRGBA{}
	}
	// body.style.background = backgroundColor. The body is an HTML element
	// of a document in standards mode: it takes no color without a #.
	page = white
	if c, ok := css.ParseColor(opts.Background); ok {
		page = over(white, c)
	}
	diagram = page
	if c, ok := css.ParseColor(quirkyColor(opts.Background)); ok {
		diagram = over(page, c)
	}
	return false, page, diagram
}

// over paints a color with straight alpha over an opaque color.
func over(below color.NRGBA, c css.Color) color.NRGBA {
	a := c.A
	if a >= 1 {
		return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
	}
	if a < 0 {
		a = 0
	}
	mix := func(top, bottom uint8) uint8 {
		return uint8(math.Round(float64(float64(top)*a) + float64(float64(bottom)*(1-a))))
	}
	return color.NRGBA{R: mix(c.R, below.R), G: mix(c.G, below.G), B: mix(c.B, below.B), A: 255}
}
