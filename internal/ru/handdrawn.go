// Ported from mermaid 12.2.0,
// rendering-elements/shapes/handDrawnShapeStyles.ts (MIT).

package ru

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/rough"
)

// StyleEntry is one entry of a style map: [key, value]. HasValue is false
// when the style text had no colon (the value is undefined upstream).
type StyleEntry struct {
	Key      string
	Value    string
	HasValue bool
}

// Join is style.join(':'): an undefined value prints as an empty string.
func (s StyleEntry) Join() string { return s.Key + ":" + s.Value }

// StyleMap is the insertion-ordered Map that styles2Map builds.
type StyleMap struct {
	entries []StyleEntry
}

// Get is stylesMap.get(key). The second result is false when the key is
// absent or its value is undefined.
func (m *StyleMap) Get(key string) (string, bool) {
	for i := range m.entries {
		if m.entries[i].Key == key {
			return m.entries[i].Value, m.entries[i].HasValue
		}
	}
	return "", false
}

// Entries returns the entries in insertion order: [...stylesMap].
func (m *StyleMap) Entries() []StyleEntry { return m.entries }

func (m *StyleMap) set(e StyleEntry) {
	for i := range m.entries {
		if m.entries[i].Key == e.Key {
			m.entries[i] = e
			return
		}
	}
	m.entries = append(m.entries, e)
}

// Styles2Map is styles2Map(styles).
func Styles2Map(styles []string) *StyleMap {
	m := &StyleMap{}
	for _, style := range styles {
		// const [key, value] = style.split(':'): only the first two parts.
		parts := strings.Split(style, ":")
		e := StyleEntry{Key: js.TrimSpace(parts[0])}
		if len(parts) > 1 {
			e.Value, e.HasValue = js.TrimSpace(parts[1]), true
		}
		m.set(e)
	}
	return m
}

func normalizeStyleList(styles string) []string {
	if styles == "" {
		return nil
	}
	var out []string
	for _, style := range strings.Split(styles, ";") {
		if s := js.TrimSpace(style); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// CompileStyles is compileStyles(node).
func CompileStyles(node *Node) *StyleMap {
	var all []string
	all = append(all, node.CSSCompiledStyles...)
	all = append(all, node.CSSStyles...)
	all = append(all, normalizeStyleList(node.LabelStyle)...)
	return Styles2Map(all)
}

// IsLabelStyle is isLabelStyle(key).
func IsLabelStyle(key string) bool {
	switch key {
	case "color", "font-size", "font-family", "font-weight", "font-style", "text-decoration",
		"text-align", "text-transform", "line-height", "letter-spacing", "word-spacing",
		"text-shadow", "text-overflow", "white-space", "word-wrap", "word-break",
		"overflow-wrap", "hyphens":
		return true
	}
	return false
}

// NodeStyles is the result of styles2String(node).
type NodeStyles struct {
	LabelStyles      string
	NodeStyles       string
	StylesArray      []StyleEntry
	BorderStyles     []string
	BackgroundStyles []string
}

// Styles2String is styles2String(node).
func Styles2String(node *Node) NodeStyles {
	stylesArray := CompileStyles(node).Entries()
	var labelStyles, nodeStyles, borderStyles, backgroundStyles []string
	for _, style := range stylesArray {
		key := style.Key
		if IsLabelStyle(key) {
			labelStyles = append(labelStyles, style.Join()+" !important")
		} else {
			nodeStyles = append(nodeStyles, style.Join()+" !important")
			if strings.Contains(key, "stroke") {
				borderStyles = append(borderStyles, style.Join()+" !important")
			}
			if key == "fill" {
				backgroundStyles = append(backgroundStyles, style.Join()+" !important")
			}
		}
	}
	return NodeStyles{
		LabelStyles:      strings.Join(labelStyles, ";"),
		NodeStyles:       strings.Join(nodeStyles, ";"),
		StylesArray:      stylesArray,
		BorderStyles:     borderStyles,
		BackgroundStyles: backgroundStyles,
	}
}

// SolidStateFill is solidStateFill(color): rough options for a solid fill.
func SolidStateFill(ctx *render.Context, color any) *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("fill", color)
	o.Set("hachureAngle", 120.0)
	o.Set("hachureGap", 1.5)
	o.Set("fillWeight", 1.5)
	o.Set("roughness", 0.7)
	o.Set("stroke", color)
	o.Set("seed", ctx.Cfg("handDrawnSeed"))
	return o
}

// UserNodeOverrides is userNodeOverrides(node, options): the rough options
// of a node, with the overrides of the caller applied on top.
func UserNodeOverrides(ctx *render.Context, node *Node, options *js.Obj[any]) *js.Obj[any] {
	themeVariables := js.AsObj(ctx.Cfg("themeVariables"))
	var nodeBorder, mainBkg any
	if themeVariables != nil {
		nodeBorder, mainBkg = themeVariables.Get("nodeBorder"), themeVariables.Get("mainBkg")
	}
	stylesMap := CompileStyles(node)

	result := js.NewObj[any]()
	result.Set("roughness", 0.7)
	if v, ok := stylesMap.Get("fill"); ok && v != "" {
		result.Set("fill", v)
	} else {
		result.Set("fill", mainBkg)
	}
	result.Set("fillStyle", "hachure")
	result.Set("fillWeight", 1.5)
	result.Set("hachureGap", 1.5)
	if v, ok := stylesMap.Get("stroke"); ok && v != "" {
		result.Set("stroke", v)
	} else {
		result.Set("stroke", nodeBorder)
	}
	result.Set("seed", ctx.Cfg("handDrawnSeed"))
	if v, ok := stylesMap.Get("stroke-width"); ok && strings.Replace(v, "px", "", 1) != "" {
		result.Set("strokeWidth", strings.Replace(v, "px", "", 1))
	} else {
		result.Set("strokeWidth", 1.3)
	}
	result.Set("fillLineDash", []any{0.0, 0.0})
	dash, _ := stylesMap.Get("stroke-dasharray")
	result.Set("strokeLineDash", getStrokeDashArray(dash))
	// Object.assign(result, options).
	if options != nil {
		options.Each(func(k string, v any) { result.Set(k, v) })
	}
	return result
}

func getStrokeDashArray(strokeDasharrayStyle string) []any {
	if strokeDasharrayStyle == "" {
		return []any{0.0, 0.0}
	}
	var dashArray []float64
	for _, f := range strings.Fields(js.TrimSpace(strokeDasharrayStyle)) {
		dashArray = append(dashArray, js.Number(f))
	}
	nanToZero := func(v float64) float64 {
		if v != v {
			return 0
		}
		return v
	}
	if len(dashArray) == 1 {
		val := nanToZero(dashArray[0])
		return []any{val, val}
	}
	first := nanToZero(dashArray[0])
	second := nanToZero(dashArray[1])
	return []any{first, second}
}

// RoughSVG stands for rough.svg(element): it draws rough shapes as detached
// SVG nodes, as RoughSVG.draw does.
type RoughSVG struct {
	doc *dom.Document
	gen *rough.Generator
}

// NewRoughSVG is rough.svg(shapeSvg). Each call makes a new generator, as
// upstream does. Its random numbers come from Math.random of the page.
func NewRoughSVG(ctx *render.Context, _ *dom.Node) *RoughSVG {
	return &RoughSVG{doc: ctx.Doc, gen: rough.NewGenerator(rough.Config{Random: ctx.Random})}
}

// Generator returns the rough generator, for the calls that need it.
func (r *RoughSVG) Generator() *rough.Generator { return r.gen }

// Draw is RoughSVG.draw(drawable): a detached <g> with one <path> for each
// set of the drawable.
func (r *RoughSVG) Draw(d *rough.Drawable) *dom.Node {
	g := r.doc.CreateElementNS(dom.NSSVG, "g")
	for _, el := range r.gen.Draw(d) {
		path := r.doc.CreateElementNS(dom.NSSVG, "path")
		for _, a := range el.Attrs {
			path.Attr(a.Name, a.Value)
		}
		g.AppendChild(path)
	}
	return g
}

// Rectangle is rc.rectangle(x, y, w, h, options).
func (r *RoughSVG) Rectangle(x, y, w, h float64, o *js.Obj[any]) (*dom.Node, error) {
	d, err := r.gen.Rectangle(x, y, w, h, o)
	if err != nil {
		return nil, err
	}
	return r.Draw(d), nil
}

// Path is rc.path(d, options).
func (r *RoughSVG) Path(pathD string, o *js.Obj[any]) (*dom.Node, error) {
	d, err := r.gen.Path(pathD, o)
	if err != nil {
		return nil, err
	}
	return r.Draw(d), nil
}

// Circle is rc.circle(x, y, diameter, options).
func (r *RoughSVG) Circle(x, y, diameter float64, o *js.Obj[any]) (*dom.Node, error) {
	d, err := r.gen.Circle(x, y, diameter, o)
	if err != nil {
		return nil, err
	}
	return r.Draw(d), nil
}

// Ellipse is rc.ellipse(x, y, w, h, options).
func (r *RoughSVG) Ellipse(x, y, w, h float64, o *js.Obj[any]) (*dom.Node, error) {
	d, err := r.gen.Ellipse(x, y, w, h, o)
	if err != nil {
		return nil, err
	}
	return r.Draw(d), nil
}

// Line is rc.line(x1, y1, x2, y2, options).
func (r *RoughSVG) Line(x1, y1, x2, y2 float64, o *js.Obj[any]) *dom.Node {
	return r.Draw(r.gen.Line(x1, y1, x2, y2, o))
}

// Polygon is rc.polygon(points, options).
func (r *RoughSVG) Polygon(points []rough.Point, o *js.Obj[any]) (*dom.Node, error) {
	d, err := r.gen.Polygon(points, o)
	if err != nil {
		return nil, err
	}
	return r.Draw(d), nil
}
