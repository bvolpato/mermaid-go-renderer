// Ported from mermaid 12.2.0, mermaidAPI.ts (render, renderDiagram) and
// Diagram.ts (MIT).

package render

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/purify"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
	"github.com/bvolpato/mermaid-go-renderer/internal/styles"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// randomDrawsBeforeRender is the number of Math.random calls that the
// mermaid bundle makes in the oracle page before the render starts (the
// randomBefore field of the reference metadata).
const randomDrawsBeforeRender = 1

const (
	maxTextLength            = 50_000
	maxTextLengthExceededMsg = "graph TB;a[Maximum text size in diagram exceeded];style a fill:#faa"
	xmlnsSVG                 = "http://www.w3.org/2000/svg"
	xmlnsXLink               = "http://www.w3.org/1999/xlink"
	xmlnsXHTML               = "http://www.w3.org/1999/xhtml"
)

// Options controls one render.
type Options struct {
	// ID is the id of the svg element. The default is "my-svg".
	ID string
	// Config is the site config: the argument of mermaid.initialize.
	Config *js.Obj[any]
	// Now is the current time. The default is the fixed time of the oracle.
	Now time.Time
	// Location is the local time zone. The default is UTC.
	Location *time.Location
	// RandomSeed seeds Math.random. The default is the seed of the oracle.
	RandomSeed uint32
	// ViewportWidth is the width of the browser window in px. The default
	// is 800, as in mermaid-cli.
	ViewportWidth float64
	// Context cancels the render. The default is context.Background().
	Context context.Context
	// Fonts is the font library that measures text. Its platform selects
	// the rules of Chromium. The default is the system library
	// (font.System). font.Embedded gives the same result on every
	// machine.
	Fonts *font.Library
	// ImageSize gives the natural size of an image by its URL: the src of
	// an img element in a label, or the image of an image shape. The last
	// result is false for an image that cannot load. A render never reads
	// from the network: without ImageSize every image that is not a data
	// URL fails, as in a browser that cannot fetch it.
	ImageSize func(src string) (width, height float64, ok bool)
	// Page is the kind of web page that the render reproduces. The default
	// is a page that has only mermaid. The caller gives the fonts of the
	// page in Fonts: for PageMermaidCLI a library with the web fonts.
	Page Page
}

// Page is the kind of web page that a render reproduces.
type Page int

const (
	// PagePlain is a page that has only mermaid.
	PagePlain Page = iota
	// PageMermaidCLI is the page of mermaid-cli 12. It has style sheets
	// in its head that a plain page has not (internal/webfonts): those of
	// Font Awesome, and the @font-face rules of Recursive Variable and
	// Open Sans.
	PageMermaidCLI
)

var (
	pageSheetsOnce sync.Once
	pageSheets     []string
)

// PageSheets returns the text of the style sheets in the head of a page,
// in document order. A plain page has none.
func PageSheets(page Page) []string {
	if page != PageMermaidCLI {
		return nil
	}
	pageSheetsOnce.Do(func() {
		for _, sheet := range webfonts.Sheets {
			data, err := webfonts.FS.ReadFile(sheet.Path)
			if err != nil {
				panic("render: " + err.Error())
			}
			pageSheets = append(pageSheets, string(data))
		}
	})
	return pageSheets
}

// Result is the result of a render.
type Result struct {
	// SVG is the string that mermaid.render() returns.
	SVG string
	// Type is the diagram type id.
	Type string
}

// UnsupportedError reports a diagram type that mermaid has and that this
// port does not have yet.
type UnsupportedError struct{ Type string }

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("diagram type %q is not supported yet", e.Type)
}

// ContextOptions are the inputs of NewContext.
type ContextOptions struct {
	ID            string
	Config        *js.Obj[any]
	ConfigState   *config.State
	Now           time.Time
	Location      *time.Location
	RandomSeed    uint32
	ViewportWidth float64
	// RandomSkip is the number of random draws to discard at the start.
	RandomSkip int
	// Context cancels the work that uses the context. The default is
	// context.Background().
	Context context.Context
	// Fonts is the font library that measures text. The default is the
	// system library.
	Fonts *font.Library
	// ImageSize gives the natural size of an image by its URL. See
	// Options.ImageSize.
	ImageSize func(src string) (width, height float64, ok bool)
	// Page is the kind of web page. See Options.Page.
	Page Page
}

// NewContext builds the page of the reference oracle and the svg root, as
// renderDiagram does before it parses the diagram: body > div#container >
// div#d<id> > svg#<id> > g. Render uses it, and so do tests that call
// renderer functions directly.
func NewContext(o ContextOptions) *Context {
	if o.ID == "" {
		o.ID = DefaultID
	}
	doc := browser.NewDocumentWithFonts(o.Fonts)
	doc.PageSheets = PageSheets(o.Page)
	engine := doc.Engine.(*browser.Engine)
	if o.ViewportWidth > 0 {
		engine.ViewportWidth = o.ViewportWidth
	}
	engine.ImageSize = o.ImageSize
	ctx := &Context{
		ID:          o.ID,
		Config:      o.Config,
		ConfigState: o.ConfigState,
		Page:        o.Page,
		Doc:         doc,
		Engine:      engine,
		Body:        doc.Body,
		Now:         o.Now,
		Location:    o.Location,
		Sanitizer:   sanitize.New(),
		Cancel:      o.Context,
		rng:         o.RandomSeed,
	}
	if ctx.Cancel == nil {
		ctx.Cancel = context.Background()
	} else {
		doc.BeforeMeasure = ctx.CheckCanceled
	}
	if ctx.Now.IsZero() {
		ctx.Now = DefaultNow
	}
	if ctx.Location == nil {
		ctx.Location = time.UTC
	}
	if ctx.rng == 0 {
		ctx.rng = DefaultRandomSeed
	}
	for i := 0; i < o.RandomSkip; i++ {
		ctx.Random()
	}

	// The page of the oracle: body > div#container. mermaid.render gets the
	// container, so appendDivSvgG adds the enclosing div and the svg to it.
	ctx.Container = doc.Body.Append("div").Attr("id", "container")
	fontFamily := js.ToString(o.Config.Get("fontFamily"))
	enclosingDiv := ctx.Container.Append("div").Attr("id", "d"+o.ID).Attr("style", "font-family: "+fontFamily)
	svg := enclosingDiv.Append("svg").
		Attr("id", o.ID).
		Attr("width", "100%").
		Attr("xmlns", xmlnsSVG).
		Attr("xmlns:xlink", xmlnsXLink)
	svg.Append("g")
	ctx.SVG = svg
	return ctx
}

// finalSanitizeConfig is the DOMPurify config of renderDiagram.
var finalSanitizeConfig = purify.Config{
	AddTags:               []string{"foreignobject"},
	AddAttr:               []string{"dominant-baseline"},
	HTMLIntegrationPoints: map[string]bool{"foreignobject": true},
}

// Render renders a diagram. It follows renderDiagram in mermaidAPI.ts.
func Render(text string, opts Options) (res *Result, err error) {
	defer func() {
		// CheckCanceled stops the render with a panic that carries the
		// error of the context.
		if p := recover(); p != nil {
			c, ok := p.(canceled)
			if !ok {
				panic(p)
			}
			res, err = nil, c.err
		}
	}()
	if opts.Context != nil {
		if err := opts.Context.Err(); err != nil {
			return nil, err
		}
	}
	id := opts.ID
	if id == "" {
		id = DefaultID
	}

	// mermaid.initialize(site), then processAndSetConfigs(text).
	resolved, err := config.Resolve(text, opts.Config)
	if err != nil {
		return nil, err
	}
	cfg := resolved.Config
	code := resolved.Source
	diagramText := code.Cleaned

	maxTextSize := float64(maxTextLength)
	if v, ok := cfg.Get("maxTextSize").(float64); ok {
		maxTextSize = v
	}
	if float64(js.UTF16Len(diagramText)) > maxTextSize {
		diagramText = maxTextLengthExceededMsg
		code = config.DiagramCode{Raw: diagramText, Cleaned: diagramText}
	}

	securityLevel, _ := cfg.Get("securityLevel").(string)
	if securityLevel == "sandbox" {
		return nil, errors.New("securityLevel sandbox is not supported: it renders into an iframe")
	}
	isLoose := securityLevel == "loose"

	ctx := NewContext(ContextOptions{
		ID:            id,
		Config:        cfg,
		ConfigState:   resolved.State,
		Now:           opts.Now,
		Location:      opts.Location,
		RandomSeed:    opts.RandomSeed,
		ViewportWidth: opts.ViewportWidth,
		RandomSkip:    randomDrawsBeforeRender,
		Fonts:         opts.Fonts,
		Context:       opts.Context,
		ImageSize:     opts.ImageSize,
		Page:          opts.Page,
	})
	ctx.Title = resolved.Title
	doc := ctx.Doc
	svg := ctx.SVG
	enclosingDiv := svg.Parent
	firstChild := svg.FirstChild

	// Diagram.fromText.
	diagramType, err := config.DetectType(code.Cleaned, cfg)
	if err != nil {
		return nil, err
	}
	ctx.Type = diagramType
	def, ok := Lookup(diagramType)
	if !ok {
		return nil, &UnsupportedError{Type: diagramType}
	}
	db := def.New(ctx)
	if def.Init != nil {
		def.Init(ctx, db)
		// init can change the config with setConfig.
		ctx.Config = ctx.ConfigState.GetConfig()
		cfg = ctx.Config
	}
	if resolved.Title != "" {
		if ts, ok := db.(TitleSetter); ok {
			ts.SetDiagramTitle(resolved.Title)
		}
	}
	// A diagram that reports source positions parses the text with its
	// comments, so that the positions are those of the original source. The
	// text size limit applies to the text that the parser reads.
	source := code.Cleaned
	if p, ok := db.(CommentPreserver); ok && p.PreserveCommentsWhenParsing() && code.WithComments != "" {
		if float64(js.UTF16Len(code.WithComments)) <= maxTextSize {
			source = code.WithComments
		}
	}
	textToParse := EncodeEntities(source) + "\n"
	if code.FrontmatterLineOffset != 0 {
		if s, ok := db.(FrontmatterOffsetSetter); ok {
			s.SetFrontmatterLineOffset(code.FrontmatterLineOffset)
		}
	}
	if err := def.Parse(ctx, db, textToParse); err != nil {
		// Upstream draws the error diagram and then throws the error.
		return nil, err
	}

	// Styles: user styles, theme styles and config styles.
	var classDefs *js.Map[string, *ClassDef]
	if def.Classes != nil {
		classDefs = def.Classes(ctx, db, textToParse)
	}
	userStyles := createCssStyles(cfg, classDefs)
	rules, err := styles.CreateUserStyles(cfg, diagramType, userStyles, "#"+id)
	if err != nil {
		return nil, err
	}
	// document.createElement('style'): an HTML element inside the svg.
	style := doc.CreateElement("style")
	style.Text(rules)
	svg.InsertBefore(style, firstChild)

	ctx.CheckCanceled()
	if err := def.Draw(ctx, db, textToParse); err != nil {
		return nil, err
	}
	ctx.CheckCanceled()

	// Accessibility.
	var a11yTitle, a11yDescr string
	if t, ok := db.(AccTitler); ok {
		a11yTitle = t.GetAccTitle()
	}
	if d, ok := db.(AccDescriber); ok {
		a11yDescr = d.GetAccDescription()
	}
	svgNode := enclosingDiv.Select("svg")
	setA11yDiagramInfo(svgNode, diagramType)
	addSVGa11yTitleDescription(svgNode, a11yTitle, a11yDescr, svgNode.AttrOr("id", ""))

	// Serialize. The selector has lowercase "foreignobject", as upstream
	// does. A type selector matches an SVG element without regard to case.
	ctx.Body.Select(`[id="`+id+`"]`).SelectAll("foreignobject > *").Attr("xmlns", xmlnsXHTML)
	out := enclosingDiv.InnerHTML()
	out = cleanUpSvgCode(out, false, config.Evaluate(cfg.Get("arrowMarkerAbsolute")))
	if !isLoose {
		// DOMPurify.sanitize on the serialized SVG, with the hooks that
		// label sanitizing installed during the render.
		// A browser parses markup of any depth. The parser of this module
		// rejects markup with more than 512 open elements. That is an
		// error for the caller, not an empty diagram.
		clean, err := ctx.Sanitizer.Purifier().SanitizeErr(out, finalSanitizeConfig)
		if err != nil {
			return nil, fmt.Errorf("the SVG cannot be sanitized: %w", err)
		}
		out = clean
	}
	return &Result{SVG: out, Type: diagramType}, nil
}
