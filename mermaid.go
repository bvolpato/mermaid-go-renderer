// Package mermaid renders Mermaid diagrams to SVG in pure Go.
//
// It is a port of mermaid 12.2.0. The goal is the same SVG that
// mermaid.render() returns in a browser, without a browser: the same
// parser, layout and markup, and text measurement that reproduces Chromium.
//
//	svg, err := mermaid.Render("flowchart LR\n  A --> B")
//
// Config works as in mermaid: front matter and directives in the diagram
// text, and a site config that corresponds to mermaid.initialize:
//
//	svg, err := mermaid.Render(src, mermaid.WithConfigJSON([]byte(`{"theme":"dark"}`)))
package mermaid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"math"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/css"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams" // registers the diagram types
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/raster"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// MermaidVersion is the version of mermaid that this package is a port of.
const MermaidVersion = render.Version

// UnsupportedDiagramError reports a diagram type that mermaid supports and
// that this package does not render yet.
type UnsupportedDiagramError struct {
	// Type is the diagram type id of mermaid, for example "architecture".
	Type string
}

func (e *UnsupportedDiagramError) Error() string {
	return fmt.Sprintf("mermaid: diagram type %q is not supported yet", e.Type)
}

type options struct {
	render render.Options
	raster raster.Options
	// systemFonts and fontDirs select the font library. See fontLibrary.
	systemFonts bool
	fontDirs    []string
	// background is the CSS color of WithBackgroundColor.
	background    string
	hasBackground bool
	// page is the page of WithPage, and fontEmbed is WithFontEmbed.
	page      Page
	fontEmbed bool
	err       error
}

// Page is the kind of web page that a render reproduces. Mermaid measures
// text in its page, so the fonts and the style sheets of the page change
// the layout of a diagram.
type Page int

const (
	// PagePlain is a page that has only mermaid. It is the default. A
	// font that a diagram names and the machine has not, such as
	// "Recursive Variable" in the default theme of mermaid 12, falls back
	// to the next font of the diagram.
	PagePlain Page = iota
	// PageMermaidCLI is the page of mermaid-cli 12 (mmdc). It has the web
	// fonts Recursive Variable, Open Sans and Font Awesome, and the style
	// sheets of Font Awesome, so that an icon such as fa:fa-car shows.
	// The fonts are part of this module.
	PageMermaidCLI
)

// WithPage selects the kind of web page that the render reproduces. Give
// the same page to Render and to the functions that read its result
// (StandaloneSVG, SVGToPNG).
func WithPage(p Page) Option {
	return func(o *options) { o.page = p }
}

// WithFontEmbed makes StandaloneSVG add a style element with the web fonts
// that the diagram uses, as mermaid-cli does by default, so that the file
// shows the same text on a machine that has not the fonts. It applies to
// the page PageMermaidCLI.
func WithFontEmbed(on bool) Option {
	return func(o *options) { o.fontEmbed = on }
}

var (
	fontLibrariesMu sync.Mutex
	// fontLibraries has one library for each list of font directories.
	fontLibraries = map[string]*font.Library{}
)

// fontLibrary returns the font library of the options. Libraries are
// shared: a library reads each font file one time.
func (o *options) fontLibrary() *font.Library {
	lib := o.machineFonts()
	if o.page == PageMermaidCLI {
		// The web fonts of the page come before the fonts of the machine.
		return lib.WithWebFonts()
	}
	return lib
}

// machineFonts returns the library of the fonts that stand for the fonts of
// the machine of the browser.
func (o *options) machineFonts() *font.Library {
	if o.systemFonts {
		return font.System()
	}
	if len(o.fontDirs) == 0 {
		return font.Embedded()
	}
	key := strings.Join(o.fontDirs, "\x00")
	fontLibrariesMu.Lock()
	defer fontLibrariesMu.Unlock()
	lib := fontLibraries[key]
	if lib == nil {
		lib = font.EmbeddedWith(o.fontDirs...)
		fontLibraries[key] = lib
	}
	return lib
}

// Option changes how a diagram is rendered.
type Option func(*options)

// WithConfigJSON sets the site config from JSON: the argument of
// mermaid.initialize, for example {"theme":"forest","look":"classic"}.
func WithConfigJSON(data []byte) Option {
	return func(o *options) {
		v, err := js.ParseJSON(data)
		if err != nil {
			o.err = fmt.Errorf("mermaid: invalid config JSON: %w", err)
			return
		}
		obj := js.AsObj(v)
		if obj == nil {
			o.err = errors.New("mermaid: the config must be a JSON object")
			return
		}
		o.render.Config = mergeConfig(o.render.Config, obj)
	}
}

// WithConfig sets the site config from a map: the argument of
// mermaid.initialize. Values are JSON-like: maps, slices, strings, numbers
// and booleans.
func WithConfig(cfg map[string]any) Option {
	return func(o *options) {
		o.render.Config = mergeConfig(o.render.Config, toObj(cfg))
	}
}

// WithTheme sets the theme: "default", "dark", "forest", "neutral", "base"
// and the other themes of mermaid.
func WithTheme(name string) Option {
	return WithConfig(map[string]any{"theme": name})
}

// WithID sets the id of the svg element. The default is "my-svg", as in
// mermaid-cli. The id appears in the style rules and in marker ids.
func WithID(id string) Option {
	return func(o *options) { o.render.ID = id }
}

// WithViewportWidth sets the width of the browser window that the layout
// assumes, in px. The default is 800, as in mermaid-cli. A few diagram
// types, such as gantt, size themselves to it.
func WithViewportWidth(px float64) Option {
	return func(o *options) { o.render.ViewportWidth = px }
}

// WithTime sets the current time and the local time zone, for diagrams
// that show today. The default is a fixed time in UTC, so that output is
// reproducible.
func WithTime(now time.Time, loc *time.Location) Option {
	return func(o *options) {
		o.render.Now = now
		o.render.Location = loc
	}
}

// WithRandomSeed sets the seed of the random numbers that mermaid draws,
// for example for generated ids and for the hand-drawn look. The default
// is a fixed seed, so that output is reproducible.
func WithRandomSeed(seed uint32) Option {
	return func(o *options) { o.render.RandomSeed = seed }
}

func mergeConfig(base, add *js.Obj[any]) *js.Obj[any] {
	if base == nil {
		return add
	}
	merged, _ := config.AssignWithDepth(base, add, 2).(*js.Obj[any])
	if merged == nil {
		return add
	}
	return merged
}

// toObj converts JSON-like Go values to the value model of the renderer.
// Map keys are sorted, so that the result does not depend on map order.
func toObj(m map[string]any) *js.Obj[any] {
	obj := js.NewObj[any]()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		obj.Set(k, toValue(m[k]))
	}
	return obj
}

func toValue(v any) any {
	switch x := v.(type) {
	case nil:
		return js.Null
	case map[string]any:
		return toObj(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = toValue(e)
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = e
		}
		return out
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	case bool, float64, string:
		return x
	}
	return fmt.Sprint(v)
}

// Result is a rendered diagram.
type Result struct {
	// SVG is the markup that mermaid.render() returns.
	SVG string
	// Type is the diagram type id of mermaid, for example "flowchart-v2".
	Type string
}

// InternalError reports a defect of this package: the renderer stopped
// with a panic. Render returns it as an error, so that a diagram cannot
// stop the program that renders it.
type InternalError struct {
	// Value is the value of the panic.
	Value any
	// Stack is the stack of the goroutine at the panic.
	Stack []byte
}

func (e *InternalError) Error() string {
	return fmt.Sprintf("mermaid: internal error: %v", e.Value)
}

// Render renders a diagram to SVG.
//
// Render is safe for concurrent use. The result depends only on the source
// and the options: the clock and the random numbers are fixed by default.
func Render(source string, opts ...Option) (string, error) {
	res, err := RenderContext(context.Background(), source, opts...)
	if err != nil {
		return "", err
	}
	return res.SVG, nil
}

// RenderResult renders a diagram and also reports its type.
func RenderResult(source string, opts ...Option) (*Result, error) {
	return RenderContext(context.Background(), source, opts...)
}

// RenderContext renders a diagram and stops when ctx is done: it then
// returns the error of ctx. Use it with a deadline for input that is not
// trusted, because a large diagram can take a long time to lay out.
func RenderContext(ctx context.Context, source string, opts ...Option) (res *Result, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.err != nil {
		return nil, o.err
	}
	if ctx != nil && ctx != context.Background() {
		o.render.Context = ctx
	}
	o.render.Fonts = o.fontLibrary()
	o.render.Page = render.Page(o.page)
	defer func() {
		if p := recover(); p != nil {
			res, err = nil, &InternalError{Value: p, Stack: debug.Stack()}
		}
	}()
	out, err := render.Render(source, o.render)
	if err != nil {
		var unsupported *render.UnsupportedError
		if errors.As(err, &unsupported) {
			return nil, &UnsupportedDiagramError{Type: unsupported.Type}
		}
		return nil, err
	}
	return &Result{SVG: out.SVG, Type: out.Type}, nil
}

// WithSystemFonts measures text with the fonts that are installed on this
// machine, by the rules of Chromium on this operating system. The result
// then matches mermaid in a browser on the same machine, and it differs
// between machines that have other fonts.
//
// Without this option the renderer uses fonts that are part of this module
// (Liberation Sans, Serif and Mono, which have the metrics of Arial, Times
// New Roman and Courier New), by the rules of Chromium on Linux. It reads no
// font file, and it gives the same result on every machine: the result of
// mermaid in Chromium on Linux with only the Liberation fonts installed.
func WithSystemFonts() Option {
	return func(o *options) { o.systemFonts = true }
}

// WithFontDirs adds the font files of directories to the fonts of this
// module, for text that the Liberation fonts have no glyphs for (Chinese,
// Japanese, Korean, emoji) and for diagrams that name other font families.
// A family in a directory comes before the family of the same name in the
// module. The rules are those of Chromium on Linux. WithSystemFonts
// replaces this option.
func WithFontDirs(dirs ...string) Option {
	return func(o *options) { o.fontDirs = append(o.fontDirs, dirs...) }
}

// WithImageSize gives the renderer the natural size of the images that a
// diagram names by URL, in img elements of labels and in image shapes. The
// renderer never reads from the network. Without this option, or when size
// reports false, an image that is not a data URL counts as an image that
// failed to load, as in a browser with no network: an img in a label is 16
// px high, and a diagram with an image shape fails. An image in a data URL
// needs no size: the renderer decodes it.
func WithImageSize(size func(src string) (width, height float64, ok bool)) Option {
	return func(o *options) { o.render.ImageSize = size }
}

// WithScale sets the number of bitmap pixels per CSS pixel of PNG output.
// The default is 1. Use 2 for a sharp image on a high-density screen.
func WithScale(scale float64) Option {
	return func(o *options) { o.raster.Scale = scale }
}

// WithBackground sets the color below the diagram in PNG output. The
// default is opaque white. A color with zero alpha, such as
// color.Transparent, gives a transparent image.
func WithBackground(c color.Color) Option {
	return func(o *options) { o.raster.Background = c }
}

// RenderPNG renders a diagram to a PNG image, at the natural size of the
// SVG. The rasterizer is part of this package: no browser is used.
func RenderPNG(source string, opts ...Option) ([]byte, error) {
	res, err := RenderContext(context.Background(), source, opts...)
	if err != nil {
		return nil, err
	}
	return SVGToPNG(res.SVG, opts...)
}

// SVGToPNG paints an SVG that Render returned into a PNG image. These
// options apply: WithScale, WithBackground, and the font options, which
// must be the font options of the Render call.
func SVGToPNG(svg string, opts ...Option) (png []byte, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.err != nil {
		return nil, o.err
	}
	defer func() {
		if p := recover(); p != nil {
			png, err = nil, &InternalError{Value: p, Stack: debug.Stack()}
		}
	}()
	// The rasterizer paints with the fonts that laid out the diagram.
	o.raster.Fonts = o.fontLibrary()
	o.raster.PageSheets = render.PageSheets(render.Page(o.page))
	img, err := raster.Render(svg, o.raster)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := raster.EncodePNG(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WithBackgroundColor sets the background from a CSS color, such as
// "white", "transparent" or "#F0F0F0", as the option --backgroundColor of
// mermaid-cli does. StandaloneSVG writes it as the background-color of the
// svg element. PNG output paints it below the diagram. A value that is not
// a color changes nothing, as in mermaid-cli.
func WithBackgroundColor(value string) Option {
	return func(o *options) {
		o.background, o.hasBackground = value, true
		if c, ok := css.ParseColor(value); ok {
			o.raster.Background = color.NRGBA{R: c.R, G: c.G, B: c.B, A: uint8(math.Round(float64(c.A * 255)))}
		}
	}
}

// StandaloneSVG turns the SVG that Render returns into the content of an
// .svg file, as mermaid-cli writes it.
//
// Render returns what mermaid.render() returns: markup for a web page, in
// the HTML syntax. In a file of its own an SVG must be XML, and the HTML
// syntax is not always XML: it writes <br> without an end, and a
// non-breaking space as &nbsp;. StandaloneSVG parses the markup as a browser
// does and writes the svg element as XML, with the rules of XMLSerializer in
// Chromium. The result has the same content as the file of mermaid-cli with
// its option --no-font-embed.
//
// One option applies: WithBackgroundColor. mermaid-cli sets the background
// "white" by default. Without the option the svg element keeps its style.
func StandaloneSVG(svg string, opts ...Option) (out string, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.err != nil {
		return "", o.err
	}
	defer func() {
		if p := recover(); p != nil {
			out, err = "", &InternalError{Value: p, Stack: debug.Stack()}
		}
	}()
	page, err := mmdc.Load(svg, mmdc.Options{Background: o.background, SetBackground: o.hasBackground})
	if err != nil {
		return "", fmt.Errorf("mermaid: %w", err)
	}
	return page.XML(), nil
}

// DetectType returns the diagram type id of a diagram source, for example
// "sequence", without rendering it.
func DetectType(source string, opts ...Option) (string, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.err != nil {
		return "", o.err
	}
	resolved, err := config.Resolve(source, o.render.Config)
	if err != nil {
		return "", err
	}
	if resolved.DiagramType == "" {
		return "", errors.New("mermaid: no diagram type detected")
	}
	return resolved.DiagramType, nil
}

// SupportedTypes returns the diagram type ids that this package renders.
func SupportedTypes() []string {
	return render.Types()
}
