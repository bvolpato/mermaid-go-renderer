// Ported from mermaid 12.2.0, diagrams/radar/renderer.ts (MIT).

package radar

import (
	"errors"
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// pi is Math.PI. It is a variable so that Go computes with float64 values,
// as JavaScript does.
var pi float64 = math.Pi

// Chromium rounds Math.sin and Math.cos correctly.
func sin(x float64) float64 { return js.SinCR(x) }
func cos(x float64) float64 { return js.CosCR(x) }

// num is the number conversion of a config value in arithmetic.
func num(config *js.Obj[any], key string) float64 {
	return js.ToNumber(config.Get(key))
}

type point struct{ x, y float64 }

// draw is draw(_text, id, _version, diagram).
func draw(ctx *render.Context, rdb render.DB, _ string) error {
	db := rdb.(*DB)
	axes := db.GetAxes()
	curves := db.GetCurves()
	options := db.GetOptions()
	config := db.GetConfig()
	title := db.GetDiagramTitle()

	svg := ctx.SelectSvgElement()

	// Draw the main frame
	g := drawFrame(svg, config)

	// The maximum value for the radar chart is the 'max' option if it exists,
	// otherwise it is the maximum value of the curves
	var maxValue float64
	if options.Max != nil {
		maxValue = *options.Max
	} else {
		maxima := make([]float64, len(curves))
		for i, curve := range curves {
			maxima[i] = js.Max(curve.Entries...)
		}
		maxValue = js.Max(maxima...)
	}
	minValue := options.Min
	radius := js.Min(num(config, "width"), num(config, "height")) / 2

	// Draw graticule
	drawGraticule(g, axes, radius, options.Ticks, options.Graticule)

	// Draw the axes
	drawAxes(g, axes, radius, config)

	// Draw the curves
	if err := drawCurves(g, axes, curves, minValue, maxValue, options.Graticule, config); err != nil {
		return err
	}

	// Draw Legend
	drawLegend(g, curves, options.ShowLegend, config)

	// Draw Title
	g.Append("text").
		Attr("class", "radarTitle").
		Text(title).
		Attr("x", 0).
		Attr("y", float64(-num(config, "height")/2)-num(config, "marginTop"))
	return nil
}

// drawFrame is drawFrame(svg, config): it returns a g element to center the
// radar chart.
func drawFrame(svg *dom.Node, config *js.Obj[any]) *dom.Node {
	totalWidth := js.Add(js.Add(config.Get("width"), config.Get("marginLeft")), config.Get("marginRight"))
	totalHeight := js.Add(js.Add(config.Get("height"), config.Get("marginTop")), config.Get("marginBottom"))
	centerX := js.Add(config.Get("marginLeft"), float64(num(config, "width")/2))
	centerY := js.Add(config.Get("marginTop"), float64(num(config, "height")/2))
	useMaxWidth := true
	if v := config.Get("useMaxWidth"); !js.IsNullish(v) {
		useMaxWidth = js.Truthy(v)
	}
	render.ConfigureSvgSize(svg, js.ToNumber(totalHeight), js.ToNumber(totalWidth), useMaxWidth)

	svg.Attr("viewBox", "0 0 "+js.ToString(totalWidth)+" "+js.ToString(totalHeight)).Attr("overflow", "visible")
	// g element to center the radar chart
	return svg.Append("g").Attr("transform", "translate("+js.ToString(centerX)+", "+js.ToString(centerY)+")")
}

// drawGraticule is drawGraticule(g, axes, radius, ticks, graticule).
func drawGraticule(g *dom.Node, axes []RadarAxis, radius, ticks float64, graticule string) {
	if graticule == "circle" {
		// Draw a circle for each tick
		for i := 0.0; i < ticks; i++ {
			r := (radius * (i + 1)) / ticks
			g.Append("circle").Attr("r", r).Attr("class", "radarGraticule")
		}
	} else if graticule == "polygon" {
		// Draw a polygon
		numAxes := float64(len(axes))
		for i := 0.0; i < ticks; i++ {
			r := (radius * (i + 1)) / ticks
			points := make([]string, len(axes))
			for j := range axes {
				angle := (2*float64(j)*pi)/numAxes - float64(pi/2)
				x := float64(r * cos(angle))
				y := float64(r * sin(angle))
				points[j] = js.Str(x) + "," + js.Str(y)
			}
			g.Append("polygon").Attr("points", strings.Join(points, " ")).Attr("class", "radarGraticule")
		}
	}
}

// drawAxes is drawAxes(g, axes, radius, config).
func drawAxes(g *dom.Node, axes []RadarAxis, radius float64, config *js.Obj[any]) {
	numAxes := float64(len(axes))

	for i := range axes {
		label := axes[i].Label
		angle := (2*float64(i)*pi)/numAxes - float64(pi/2)
		cosA := cos(angle)
		sinA := sin(angle)

		g.Append("line").
			Attr("x1", 0).
			Attr("y1", 0).
			Attr("x2", radius*num(config, "axisScaleFactor")*cosA).
			Attr("y2", radius*num(config, "axisScaleFactor")*sinA).
			Attr("class", "radarAxisLine")

		// Anchor labels based on their angular position so that text extends
		// away from the chart center rather than overflowing the viewBox.
		textAnchor := "middle"
		if cosA > 0.01 {
			textAnchor = "start"
		} else if cosA < -0.01 {
			textAnchor = "end"
		}
		dominantBaseline := "central"
		if sinA > 0.01 {
			dominantBaseline = "hanging"
		} else if sinA < -0.01 {
			dominantBaseline = "auto"
		}

		// Small pixel offset to push labels slightly outward from the axis endpoint,
		// giving extra clearance so text doesn't sit right at the chart boundary.
		labelPad := 4.0

		g.Append("text").
			Text(label).
			Attr("x", float64(radius*num(config, "axisLabelFactor")*cosA)+float64(labelPad*cosA)).
			Attr("y", float64(radius*num(config, "axisLabelFactor")*sinA)+float64(labelPad*sinA)).
			Attr("text-anchor", textAnchor).
			Attr("dominant-baseline", dominantBaseline).
			Attr("class", "radarAxisLabel")
	}
}

// drawCurves is drawCurves(g, axes, curves, minValue, maxValue, graticule,
// config).
func drawCurves(g *dom.Node, axes []RadarAxis, curves []RadarCurve, minValue, maxValue float64, graticule string, config *js.Obj[any]) error {
	numAxes := float64(len(axes))
	radius := js.Min(num(config, "width"), num(config, "height")) / 2

	for index, curve := range curves {
		if len(curve.Entries) != len(axes) {
			// Skip curves that do not have an entry for each axis.
			continue
		}
		// Compute points for the curve.
		points := make([]point, len(curve.Entries))
		for i, entry := range curve.Entries {
			angle := (2*pi*float64(i))/numAxes - float64(pi/2)
			r := relativeRadius(entry, minValue, maxValue, radius)
			x := float64(r * cos(angle))
			y := float64(r * sin(angle))
			points[i] = point{x, y}
		}

		if graticule == "circle" {
			// Draw a closed curve through the points.
			d, err := closedRoundCurve(points, num(config, "curveTension"))
			if err != nil {
				return err
			}
			g.Append("path").
				Attr("d", d).
				Attr("class", "radarCurve-"+js.Str(float64(index)))
		} else if graticule == "polygon" {
			// Draw a polygon for each curve.
			texts := make([]string, len(points))
			for i, p := range points {
				texts[i] = js.Str(p.x) + "," + js.Str(p.y)
			}
			g.Append("polygon").
				Attr("points", strings.Join(texts, " ")).
				Attr("class", "radarCurve-"+js.Str(float64(index)))
		}
	}
	return nil
}

// relativeRadius is relativeRadius(value, minValue, maxValue, radius).
func relativeRadius(value, minValue, maxValue, radius float64) float64 {
	clippedValue := js.Min(js.Max(value, minValue), maxValue)
	return (radius * (clippedValue - minValue)) / (maxValue - minValue)
}

// closedRoundCurve is closedRoundCurve(points, tension).
func closedRoundCurve(points []point, tension float64) (string, error) {
	// Catmull-Rom spline helper function
	numPoints := len(points)
	if numPoints == 0 {
		// points[0] is undefined: upstream throws a TypeError.
		return "", errors.New("Cannot read properties of undefined (reading 'x')")
	}
	var d strings.Builder
	d.WriteString("M" + js.Str(points[0].x) + "," + js.Str(points[0].y))
	// For each segment from point i to point (i+1) mod n, compute control points.
	for i := 0; i < numPoints; i++ {
		p0 := points[(i-1+numPoints)%numPoints]
		p1 := points[i]
		p2 := points[(i+1)%numPoints]
		p3 := points[(i+2)%numPoints]
		// Calculate the control points for the cubic Bezier segment
		cp1 := point{
			x: p1.x + float64((p2.x-p0.x)*tension),
			y: p1.y + float64((p2.y-p0.y)*tension),
		}
		cp2 := point{
			x: p2.x - float64((p3.x-p1.x)*tension),
			y: p2.y - float64((p3.y-p1.y)*tension),
		}
		d.WriteString(" C" + js.Str(cp1.x) + "," + js.Str(cp1.y) + " " + js.Str(cp2.x) + "," + js.Str(cp2.y) + " " + js.Str(p2.x) + "," + js.Str(p2.y))
	}
	d.WriteString(" Z")
	return d.String(), nil
}

// drawLegend is drawLegend(g, curves, showLegend, config).
func drawLegend(g *dom.Node, curves []RadarCurve, showLegend bool, config *js.Obj[any]) {
	if !showLegend {
		return
	}

	// Create a legend group and position it in the top-right corner of the chart.
	legendX := float64((js.ToNumber(js.Add(float64(num(config, "width")/2), config.Get("marginRight"))) * 3) / 4)
	legendY := float64((-js.ToNumber(js.Add(float64(num(config, "height")/2), config.Get("marginTop"))) * 3) / 4)
	lineHeight := 20.0

	for index, curve := range curves {
		itemGroup := g.Append("g").
			Attr("transform", "translate("+js.Str(legendX)+", "+js.Str(legendY+float64(float64(index)*lineHeight))+")")

		// Draw a square marker for this curve.
		itemGroup.Append("rect").
			Attr("width", 12).
			Attr("height", 12).
			Attr("class", "radarLegendBox-"+js.Str(float64(index)))

		// Draw the label text next to the marker.
		itemGroup.Append("text").
			Attr("x", 16).
			Attr("y", 0).
			Attr("class", "radarLegendText").
			Text(curve.Label)
	}
}
