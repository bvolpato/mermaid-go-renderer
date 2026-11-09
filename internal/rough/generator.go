// Ported from roughjs 4.6.6, roughjs/bin/generator.js (MIT).

package rough

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

const nos = "none"

// maxWork is the number of loop steps that one shape method can run. The
// JavaScript code has loops that do not end for some inputs, for example a
// pattern fill of a shape with a coordinate that is not finite. A shape that
// needs more steps than maxWork needs more memory than a browser has, so the
// limit does not change the result for an input that a browser can draw.
const maxWork = 1 << 22

// maxCallDepth is the recursion depth at which the port reports the
// RangeError "Maximum call stack size exceeded" of JavaScript. The recursive
// functions need less than 1200 levels for finite input.
const maxCallDepth = 5000

// maxSimplifyDepth is the same limit for simplifyPoints. Its depth grows
// with the number of points, so the limit is higher. JavaScript reports the
// stack overflow at a depth of about 8000.
const maxSimplifyDepth = 100000

// Generator is the RoughGenerator class of roughjs.
//
// A Generator is not safe for concurrent use. Like the JavaScript object, it
// keeps the random number generator of its default options between calls.
type Generator struct {
	defaultOptions *ResolvedOptions
	random         func() float64
	m              mathFuncs
	work           int
}

// NewGenerator returns a generator with the given configuration.
func NewGenerator(cfg Config) *Generator {
	g := &Generator{random: cfg.Random, m: jsMathFuncs()}
	if g.random == nil {
		g.random = rand.Float64
	}
	g.defaultOptions = newDefaultOptions(g)
	if cfg.Options != nil {
		g.defaultOptions = g._o(cfg.Options)
	}
	return g
}

// mathRandom is Math.random.
func (g *Generator) mathRandom() float64 { return g.random() }

// step counts one step of a loop that has no fixed bound.
func (g *Generator) step() {
	g.work++
	if g.work > maxWork {
		hang("the shape needs too many steps")
	}
}

// need reports the hang error now when a loop needs more than the remaining
// steps. It avoids the work and the memory of a loop that cannot finish.
func (g *Generator) need(steps float64) {
	if steps > float64(maxWork-g.work) {
		hang("the shape needs too many steps")
	}
}

// throw raises a JavaScript error. The exported shape methods recover it and
// return it as an *Error.
func throw(name, message string) {
	panic(&Error{Name: name, Message: message})
}

// hang raises the error for input that makes the JavaScript code loop
// without end.
func hang(message string) {
	throw("Hang", message+" (roughjs does not terminate for this input)")
}

// run calls a shape method body and turns a raised error into a result.
func (g *Generator) run(body func() *Drawable) (d *Drawable, err error) {
	g.work = 0
	defer func() {
		if r := recover(); r != nil {
			d = nil
			if e, ok := r.(*Error); ok {
				err = e
			} else {
				err = &Error{Name: "InternalError", Message: fmt.Sprint(r)}
			}
		}
	}()
	return body(), nil
}

// _o is Object.assign({}, this.defaultOptions, options). Without options it
// returns the default options object itself, so the random number generator
// of the default options continues from one call to the next.
func (g *Generator) _o(options Options) *ResolvedOptions {
	if options == nil {
		return g.defaultOptions
	}
	o := g.defaultOptions.clone()
	options.Each(func(key string, v any) {
		o.assign(key, v)
	})
	return o
}

func (g *Generator) _d(shape string, sets []OpSet, options *ResolvedOptions) *Drawable {
	if sets == nil {
		sets = []OpSet{}
	}
	return &Drawable{Shape: shape, Sets: sets, Options: options}
}

// Line is RoughGenerator.line.
func (g *Generator) Line(x1, y1, x2, y2 float64, options Options) *Drawable {
	o := g._o(options)
	return g._d("line", []OpSet{line(x1, y1, x2, y2, o)}, o)
}

// Rectangle is RoughGenerator.rectangle.
func (g *Generator) Rectangle(x, y, width, height float64, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		o := g._o(options)
		paths := []OpSet{}
		outline := rectangle(x, y, width, height, o)
		if truthy(o.fill) {
			points := []*Point{{x, y}, {x + width, y}, {x + width, y + height}, {x, y + height}}
			if isString(o.fillStyle, "solid") {
				paths = append(paths, solidFillPolygon([][]*Point{points}, o))
			} else {
				paths = append(paths, patternFillPolygons([][]*Point{points}, o))
			}
		}
		if o.hasStroke() {
			paths = append(paths, outline)
		}
		return g._d("rectangle", paths, o)
	})
}

// Ellipse is RoughGenerator.ellipse.
func (g *Generator) Ellipse(x, y, width, height float64, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		return g.ellipse(x, y, width, height, options)
	})
}

func (g *Generator) ellipse(x, y, width, height float64, options Options) *Drawable {
	o := g._o(options)
	paths := []OpSet{}
	ellipseParams := generateEllipseParams(width, height, o)
	ellipseResponse := ellipseWithParams(x, y, o, ellipseParams)
	if truthy(o.fill) {
		if isString(o.fillStyle, "solid") {
			shape := ellipseWithParams(x, y, o, ellipseParams).opset
			shape.Type = SetFillPath
			paths = append(paths, shape)
		} else {
			paths = append(paths, patternFillPolygons([][]*Point{ellipseResponse.estimatedPoints}, o))
		}
	}
	if o.hasStroke() {
		paths = append(paths, ellipseResponse.opset)
	}
	return g._d("ellipse", paths, o)
}

// Circle is RoughGenerator.circle.
func (g *Generator) Circle(x, y, diameter float64, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		ret := g.ellipse(x, y, diameter, diameter, options)
		ret.Shape = "circle"
		return ret
	})
}

// LinearPath is RoughGenerator.linearPath.
func (g *Generator) LinearPath(points []Point, options Options) *Drawable {
	o := g._o(options)
	return g._d("linearPath", []OpSet{linearPath(pointRefs(points), false, o)}, o)
}

// Arc is RoughGenerator.arc. The angles are in radians.
func (g *Generator) Arc(x, y, width, height, start, stop float64, closed bool, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		o := g._o(options)
		paths := []OpSet{}
		outline := arc(x, y, width, height, start, stop, closed, true, o)
		if closed && truthy(o.fill) {
			if isString(o.fillStyle, "solid") {
				fillOptions := o.clone()
				fillOptions.disableMultiStroke = true
				shape := arc(x, y, width, height, start, stop, true, false, fillOptions)
				shape.Type = SetFillPath
				paths = append(paths, shape)
			} else {
				paths = append(paths, patternFillArc(x, y, width, height, start, stop, o))
			}
		}
		if o.hasStroke() {
			paths = append(paths, outline)
		}
		return g._d("arc", paths, o)
	})
}

// Curve is RoughGenerator.curve for one list of points.
//
// A pattern fill of a curve with fewer than 3 points rotates the given
// points in place and rotates them back, as roughjs does. The points can
// change in the last bits.
func (g *Generator) Curve(points []Point, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		var pointsList [][]*Point
		if len(points) > 0 {
			pointsList = [][]*Point{pointRefs(points)}
		}
		return g.curve(pointsList, options)
	})
}

// Curves is RoughGenerator.curve for a list of point lists (Point[][]).
func (g *Generator) Curves(pointsList [][]Point, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		var list [][]*Point
		for _, points := range pointsList {
			list = append(list, pointRefs(points))
		}
		return g.curve(list, options)
	})
}

func (g *Generator) curve(pointsList [][]*Point, options Options) *Drawable {
	o := g._o(options)
	paths := []OpSet{}
	outline := curve(pointsList, o)
	if truthy(o.fill) && !isString(o.fill, nos) {
		if isString(o.fillStyle, "solid") {
			fillOptions := o.clone()
			fillOptions.disableMultiStroke = true
			if truthyNum(o.roughness) {
				fillOptions.roughness = o.roughness + o.fillShapeRoughnessGain
			} else {
				fillOptions.roughness = 0
			}
			fillShape := curve(pointsList, fillOptions)
			paths = append(paths, OpSet{
				Type: SetFillPath,
				Ops:  g._mergedShape(fillShape.Ops),
			})
		} else {
			polyPoints := []*Point{}
			for _, points := range pointsList {
				if len(points) < 3 {
					polyPoints = append(polyPoints, points...)
				} else if len(points) == 3 {
					polyPoints = append(polyPoints, pointsOnBezierCurves(g, curveToBezier([]*Point{
						points[0],
						points[0],
						points[1],
						points[2],
					}, 0), 10, (1+o.roughness)/2)...)
				} else {
					polyPoints = append(polyPoints, pointsOnBezierCurves(g, curveToBezier(points, 0), 10, (1+o.roughness)/2)...)
				}
			}
			if len(polyPoints) > 0 {
				paths = append(paths, patternFillPolygons([][]*Point{polyPoints}, o))
			}
		}
	}
	if o.hasStroke() {
		paths = append(paths, outline)
	}
	return g._d("curve", paths, o)
}

// Polygon is RoughGenerator.polygon.
//
// A pattern fill rotates the given points in place and rotates them back, as
// roughjs does. The points can change in the last bits.
func (g *Generator) Polygon(points []Point, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		o := g._o(options)
		paths := []OpSet{}
		refs := pointRefs(points)
		outline := linearPath(refs, true, o)
		if truthy(o.fill) {
			if isString(o.fillStyle, "solid") {
				paths = append(paths, solidFillPolygon([][]*Point{refs}, o))
			} else {
				paths = append(paths, patternFillPolygons([][]*Point{refs}, o))
			}
		}
		if o.hasStroke() {
			paths = append(paths, outline)
		}
		return g._d("polygon", paths, o)
	})
}

// Path is RoughGenerator.path. The error is the error that roughjs throws
// for path data that it cannot parse.
func (g *Generator) Path(d string, options Options) (*Drawable, error) {
	return g.run(func() *Drawable {
		o := g._o(options)
		paths := []OpSet{}
		if d == "" {
			return g._d("path", paths, o)
		}
		// d.replace(/\n/g, ' ').replace(/(-\s)/g, '-').replace('/(\s\s)/g', ' ')
		// The last call has a string as its pattern, not a regular
		// expression. It replaces the first occurrence of that text.
		d = strings.ReplaceAll(d, "\n", " ")
		d = replaceMinusSpace(d)
		d = strings.Replace(d, "/(ss)/g", " ", 1)
		hasFill := truthy(o.fill) && !isString(o.fill, "transparent") && !isString(o.fill, nos)
		hasStroke := o.hasStroke()
		simplified := truthyNum(o.simplification) && (o.simplification < 1)
		var distance float64
		if simplified {
			distance = 4 - float64(4*orNum(o.simplification, 1))
		} else {
			distance = (1 + o.roughness) / 2
		}
		sets := pointsOnPath(g, d, 1, distance)
		shape := svgPath(d, o)
		if hasFill {
			if isString(o.fillStyle, "solid") {
				if len(sets) == 1 {
					fillOptions := o.clone()
					fillOptions.disableMultiStroke = true
					if truthyNum(o.roughness) {
						fillOptions.roughness = o.roughness + o.fillShapeRoughnessGain
					} else {
						fillOptions.roughness = 0
					}
					fillShape := svgPath(d, fillOptions)
					paths = append(paths, OpSet{
						Type: SetFillPath,
						Ops:  g._mergedShape(fillShape.Ops),
					})
				} else {
					paths = append(paths, solidFillPolygon(sets, o))
				}
			} else {
				paths = append(paths, patternFillPolygons(sets, o))
			}
		}
		if hasStroke {
			if simplified {
				for _, set := range sets {
					paths = append(paths, linearPath(set, false, o))
				}
			} else {
				paths = append(paths, shape)
			}
		}
		return g._d("path", paths, o)
	})
}

// OpsToPath is RoughGenerator.opsToPath. fixedDecimals is the optional
// second argument: nil, or a number of decimal places. JavaScript throws a
// RangeError for more than 100 places. OpsToPath does not round in that case.
func (g *Generator) OpsToPath(drawing *OpSet, fixedDecimals any) string {
	digits := -1
	switch fixedDecimals.(type) {
	case float64, int, int32, int64, uint32, float32:
		// typeof fixedDecimals === 'number' && fixedDecimals >= 0
		if f := toNumber(fixedDecimals); f >= 0 && f < 101 {
			digits = int(f)
		}
	}
	var sb strings.Builder
	for _, item := range drawing.Ops {
		data := item.Data
		if digits >= 0 {
			// item.data.map((d) => +d.toFixed(fixedDecimals))
			data = make([]float64, len(item.Data))
			for i, d := range item.Data {
				data[i] = js.Number(js.ToFixed(d, digits))
			}
		}
		// num writes `${data[i]}`. A missing element prints as undefined.
		num := func(i int) {
			if i < len(data) {
				sb.WriteString(js.Str(data[i]))
			} else {
				sb.WriteString("undefined")
			}
		}
		switch item.Op {
		case OpMove:
			sb.WriteByte('M')
			num(0)
			sb.WriteByte(' ')
			num(1)
			sb.WriteByte(' ')
		case OpBcurveTo:
			sb.WriteByte('C')
			num(0)
			sb.WriteByte(' ')
			num(1)
			sb.WriteString(", ")
			num(2)
			sb.WriteByte(' ')
			num(3)
			sb.WriteString(", ")
			num(4)
			sb.WriteByte(' ')
			num(5)
			sb.WriteByte(' ')
		case OpLineTo:
			sb.WriteByte('L')
			num(0)
			sb.WriteByte(' ')
			num(1)
			sb.WriteByte(' ')
		}
	}
	return js.TrimSpace(sb.String())
}

// ToPaths is RoughGenerator.toPaths.
func (g *Generator) ToPaths(drawable *Drawable) []PathInfo {
	sets := drawable.Sets
	o := drawable.Options
	if o == nil {
		o = g.defaultOptions
	}
	paths := []PathInfo{}
	for i := range sets {
		drawing := &sets[i]
		switch drawing.Type {
		case SetPath:
			paths = append(paths, PathInfo{
				D:           g.OpsToPath(drawing, nil),
				Stroke:      o.stroke,
				StrokeWidth: o.strokeWidth,
				Fill:        nos,
			})
		case SetFillPath:
			var fill any = nos
			if truthy(o.fill) {
				fill = o.fill
			}
			paths = append(paths, PathInfo{
				D:           g.OpsToPath(drawing, nil),
				Stroke:      nos,
				StrokeWidth: 0.0,
				Fill:        fill,
			})
		case SetFillSketch:
			paths = append(paths, g.fillSketch(drawing, o))
		}
	}
	return paths
}

func (g *Generator) fillSketch(drawing *OpSet, o *ResolvedOptions) PathInfo {
	fweight := o.fillWeight
	if o.fillWeightNum() < 0 {
		fweight = o.strokeWidthNum() / 2
	}
	var stroke any = nos
	if truthy(o.fill) {
		stroke = o.fill
	}
	return PathInfo{
		D:           g.OpsToPath(drawing, nil),
		Stroke:      stroke,
		StrokeWidth: fweight,
		Fill:        nos,
	}
}

func (g *Generator) _mergedShape(input []Op) []Op {
	out := []Op{}
	for i, d := range input {
		if i == 0 {
			out = append(out, d)
			continue
		}
		if d.Op == OpMove {
			continue
		}
		out = append(out, d)
	}
	return out
}

// pointRefs returns pointers to the elements of points. roughjs changes the
// points of the caller in place, and so does the port.
func pointRefs(points []Point) []*Point {
	refs := make([]*Point, len(points))
	for i := range points {
		refs[i] = &points[i]
	}
	return refs
}

// replaceMinusSpace is d.replace(/(-\s)/g, '-').
func replaceMinusSpace(d string) string {
	if !strings.Contains(d, "-") {
		return d
	}
	runes := []rune(d)
	out := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		out = append(out, runes[i])
		if runes[i] == '-' && i+1 < len(runes) && isRegExpSpace(runes[i+1]) {
			i++
		}
	}
	return string(out)
}

// isRegExpSpace reports whether r matches \s in a JavaScript regular expression.
func isRegExpSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}
