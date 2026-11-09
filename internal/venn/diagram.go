// Ported from @upsetjs/venn.js 2.0.0, src/diagram.js (the part of
// build/venn.esm.js that mermaid 12.2.0 bundles).

package venn

import (
	"math"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// DiagramOptions are the settings of a chart that mermaid changes:
// VennDiagram().width(width).height(height). All other settings have their
// defaults: padding 15, orientation Math.PI / 2, normalize, wrap and styled
// on, no font size, no rounding.
type DiagramOptions struct {
	// Width and Height are JavaScript values: a number, or a value that
	// converts to a number. The chart writes them to the attributes of the
	// svg element as they are.
	Width  any
	Height any
}

// Chart is the result of Diagram.
type Chart struct {
	// Circles is the layout: the circle of each set, by set id.
	Circles *js.Obj[*Circle]
	// TextCentres has the text centre of each area. The key of an area is
	// its set ids joined with commas.
	TextCentres *js.Obj[*TextCentre]

	datum map[*dom.Node]*Area
}

// Datum returns the area that d3 binds to an element of the chart
// (node.__data__): the g element of an area, its path and its text. It is
// nil for other elements.
func (c *Chart) Datum(n *dom.Node) *Area {
	if c == nil {
		return nil
	}
	return c.datum[n]
}

// colourScheme is the default colourScheme: d3.schemeCategory10.
var colourScheme = []string{
	"#1f77b4",
	"#ff7f0e",
	"#2ca02c",
	"#d62728",
	"#9467bd",
	"#8c564b",
	"#e377c2",
	"#7f7f7f",
	"#bcbd22",
	"#17becf",
}

// Diagram is the chart function that VennDiagram() returns, called on a
// selection of one new element: d3.select(element).datum(data).call(chart).
// It appends an svg element with a g element for each area: a path and a
// text. The element must have no svg element yet: the transition from a
// previous diagram is not ported.
//
// venn.js measures the text of a label to wrap it. A text that is not in
// the document has the length 0 in Chromium, so a label does not wrap when
// the element is detached, as it is in mermaid.
func Diagram(env *Env, selection *dom.Node, data []*Area, o DiagramOptions) (*Chart, error) {
	e := newEngine(env)
	return run(func() *Chart {
		checkSets(data)
		return e.chart(selection, data, o)
	})
}

// chart is the function chart(selection) of VennDiagram.
func (e *engine) chart(selection *dom.Node, data []*Area, o DiagramOptions) *Chart {
	width := js.ToNumber(o.Width)
	height := js.ToNumber(o.Height)
	const padding = 15.0
	const orientation = pi / 2

	// handle 0-sized sets by removing from input
	toRemove := map[string]bool{}
	for _, datum := range data {
		if datum.Size == 0 && len(datum.Sets) == 1 {
			toRemove[datum.Sets[0]] = true
		}
	}
	kept := []*Area{}
	for _, datum := range data {
		removed := false
		for _, set := range datum.Sets {
			if toRemove[set] {
				removed = true
				break
			}
		}
		if !removed {
			kept = append(kept, datum)
		}
	}
	data = kept

	circles := js.NewObj[*Circle]()
	textCentres := js.NewObj[*TextCentre]()

	if len(data) > 0 {
		solution := e.venn(data)

		solution = e.normalizeSolution(solution, orientation)

		circles = scaleSolution(solution, width, height, padding)
		textCentres = e.computeTextCentres(circles, data)
	}

	// Figure out the current label for each set. These can change
	// and D3 won't necessarily update (fixes https://github.com/benfred/venn.js/issues/103)
	labels := js.NewObj[string]()
	for _, datum := range data {
		if datum.HasLabel && datum.Label != "" {
			labels.Set(setsKey(datum.Sets), datum.Label)
		}
	}

	// label is label(d). The second result is false when the label is
	// undefined.
	label := func(d *Area) (string, bool) {
		key := setsKey(d.Sets)
		if v, ok := labels.Lookup(key); ok {
			return v, true
		}
		if objectPrototypeKeys[key] {
			// `key in labels` is true for a property of Object.prototype,
			// and the label is then a function. wrapText calls
			// label.split on it.
			throw("TypeError", "label.split is not a function")
		}
		if len(d.Sets) == 1 {
			return d.Sets[0], true
		}
		return "", false
	}

	// create svg if not already existing
	if len(selection.SelectAll("svg")) == 0 {
		selection.Append("svg")
	}

	svg := selection.Select("svg")

	svg.Attr("width", o.Width).Attr("height", o.Height)

	// update data, joining on the set ids. Every datum is new.
	chart := &Chart{Circles: circles, TextCentres: textCentres, datum: map[*dom.Node]*Area{}}

	// create new nodes
	enter := make([]*dom.Node, len(data))
	for i, d := range data {
		kind := "intersection"
		if len(d.Sets) == 1 {
			kind = "circle"
		}
		enter[i] = svg.Append("g")
		chart.datum[enter[i]] = d
		enter[i].Attr("class", "venn-area venn-"+kind)
	}
	for i, d := range data {
		enter[i].Attr("data-venn-sets", strings.Join(d.Sets, "_"))
	}

	enterPath := make([]*dom.Node, len(data))
	for i, d := range data {
		enterPath[i] = enter[i].Append("path")
		chart.datum[enterPath[i]] = d
	}
	enterText := make([]*dom.Node, len(data))
	for i, d := range data {
		enterText[i] = enter[i].Append("text")
		chart.datum[enterText[i]] = d
	}
	for i := range data {
		enterText[i].Attr("class", "label")
	}
	for i, d := range data {
		text, _ := label(d)
		enterText[i].Text(text)
	}
	for i := range data {
		enterText[i].Attr("text-anchor", "middle")
	}
	for i := range data {
		enterText[i].Attr("dy", ".35em")
	}
	for i := range data {
		enterText[i].Attr("x", width/2)
	}
	for i := range data {
		enterText[i].Attr("y", height/2)
	}

	// apply minimal style if wanted
	colourMap := js.NewObj[string]()
	colourIndex := 0
	// colours is colours(key). The second result is false when the value
	// is a property of Object.prototype, which is not a color: the style
	// stays as it is.
	colours := func(sets []string) (string, bool) {
		key := setsKey(sets)
		if v, ok := colourMap.Lookup(key); ok {
			return v, true
		}
		if objectPrototypeKeys[key] {
			return "", false
		}
		ret := colourScheme[colourIndex]
		colourMap.Set(key, ret)
		colourIndex += 1
		if colourIndex >= len(colourScheme) {
			colourIndex = 0
		}
		return ret, true
	}
	for i := range data {
		enterPath[i].Style("fill-opacity", "0")
	}
	for i, d := range data {
		if len(d.Sets) == 1 {
			if colour, ok := colours(d.Sets); ok {
				enterPath[i].Style("fill", colour)
			}
		}
	}
	for i, d := range data {
		if len(d.Sets) == 1 {
			enterPath[i].Style("fill-opacity", ".25")
		}
	}

	for i, d := range data {
		if len(d.Sets) == 1 {
			if colour, ok := colours(d.Sets); ok {
				enterText[i].Style("fill", colour)
			}
		} else {
			enterText[i].Style("fill", "#444")
		}
	}

	// update existing, using pathTween if necessary
	update := selection
	updatePaths := update.SelectAll("path")
	for _, path := range updatePaths {
		d := chart.datum[path]
		if d == nil {
			throwUndefined("sets")
		}
		members := make([]*Circle, len(d.Sets))
		for i, set := range d.Sets {
			members[i] = circles.Get(set)
		}
		path.Attr("d", e.intersectionAreaPath(members, 0, false))
	}

	updateText := []*dom.Node{}
	for _, text := range update.SelectAll("text") {
		d := chart.datum[text]
		if d == nil {
			throwUndefined("sets")
		}
		if inObject(textCentres, setsKey(d.Sets)) {
			updateText = append(updateText, text)
		}
	}
	for _, text := range updateText {
		value, _ := label(chart.datum[text])
		text.Text(value)
	}
	for _, text := range updateText {
		centre := textCentres.Get(setsKey(chart.datum[text].Sets))
		if centre == nil {
			throwUndefined("x")
		}
		text.Attr("x", math.Floor(centre.X))
	}
	for _, text := range updateText {
		centre := textCentres.Get(setsKey(chart.datum[text].Sets))
		text.Attr("y", math.Floor(centre.Y))
	}

	for _, text := range updateText {
		wrapText(circles, label, text, chart.datum[text])
	}

	// remove old: the exit selection is empty.

	return chart
}

// computedTextLength is tspan.getComputedTextLength(). An element that is
// not in the document has no layout in Chromium, and the length is 0.
func computedTextLength(n *dom.Node) float64 {
	root := n
	for root.Parent != nil {
		root = root.Parent
	}
	if doc := n.Document(); doc == nil || root != doc.Body {
		return 0
	}
	return n.ComputedTextLength()
}

// splitWhiteSpace is label.split(/\s+/).
func splitWhiteSpace(s string) []string {
	parts := []string{}
	start := 0
	inSpace := false
	for i, r := range s {
		if js.IsWhiteSpace(r) {
			if !inSpace {
				parts = append(parts, s[start:i])
				inSpace = true
			}
		} else if inSpace {
			start = i
			inSpace = false
		}
	}
	if inSpace {
		parts = append(parts, "")
	} else {
		parts = append(parts, s[start:])
	}
	return parts
}

// wrapText is the function that wrapText(circles, labeller) returns, called
// for one text element with its datum.
//
// sometimes text doesn't fit inside the circle, if thats the case lets wrap
// the text here such that it fits
// todo: looks like this might be merged into d3 (
// https://github.com/mbostock/d3/issues/1642),
// also worth checking out is
// http://engineering.findthebest.com/wrapping-axis-labels-in-d3-js/
// this seems to be one of those things that should be easy but isn't
func wrapText(circles *js.Obj[*Circle], labeller func(*Area) (string, bool), text *dom.Node, data *Area) {
	first := "undefined"
	if len(data.Sets) > 0 {
		first = data.Sets[0]
	}
	width := circleOf(circles, first, "radius").Radius
	if width == 0 || math.IsNaN(width) {
		width = 50
	}
	label, _ := labeller(data)

	// words is label.split(/\s+/).reverse(). pop takes the last element.
	words := splitWhiteSpace(label)
	for i, j := 0, len(words)-1; i < j; i, j = i+1, j-1 {
		words[i], words[j] = words[j], words[i]
	}
	// pop is words.pop(). The second result is false for undefined.
	pop := func() (string, bool) {
		if len(words) == 0 {
			return "", false
		}
		word := words[len(words)-1]
		words = words[:len(words)-1]
		return word, true
	}
	const maxLines = 3
	minChars := float64(js.UTF16Len(label)+len(words)) / maxLines

	word, _ := pop()
	line := []string{word}
	lineNumber := 0
	const lineHeight = 1.1 // ems
	text.Text("")          // clear
	tspans := []*dom.Node{}

	appendTspan := func(word string) *dom.Node {
		tspan := text.Document().CreateElementNS(text.NS, "tspan")
		tspan.Text(word)
		tspans = append(tspans, tspan)
		text.AppendChild(tspan)
		return tspan
	}
	tspan := appendTspan(word)

	for {
		word, ok := pop()
		if !ok || word == "" {
			break
		}
		line = append(line, word)
		joined := strings.Join(line, " ")
		tspan.Text(joined)
		if float64(js.UTF16Len(joined)) > minChars && computedTextLength(tspan) > width {
			line = line[:len(line)-1]
			tspan.Text(strings.Join(line, " "))
			line = []string{word}
			tspan = appendTspan(word)
			lineNumber++
		}
	}

	lineHeightV := float64(lineHeight)
	initial := 0.35 - float64(float64(float64(lineNumber)*lineHeightV)/2)
	x, ok := text.GetAttr("x")
	if !ok {
		x = "null"
	}
	y, ok := text.GetAttr("y")
	if !ok {
		y = "null"
	}
	for i, t := range tspans {
		t.Attr("x", x)
		t.Attr("y", y)
		t.Attr("dy", js.Str(initial+float64(float64(i)*lineHeightV))+"em")
	}
}

// circleMargin is circleMargin(current, interior, exterior).
func circleMargin(current Point, interior, exterior []*Circle) float64 {
	if len(interior) == 0 {
		throwUndefined("radius")
	}
	margin := interior[0].Radius - distance(interior[0].X, interior[0].Y, current.X, current.Y)

	for i := 1; i < len(interior); i++ {
		m := interior[i].Radius - distance(interior[i].X, interior[i].Y, current.X, current.Y)
		if m <= margin {
			margin = m
		}
	}

	for i := 0; i < len(exterior); i++ {
		m := distance(exterior[i].X, exterior[i].Y, current.X, current.Y) - exterior[i].Radius
		if m <= margin {
			margin = m
		}
	}
	return margin
}

// computeTextCentre is computeTextCentre(interior, exterior,
// symmetricalTextCentre) with symmetricalTextCentre false.
//
// compute the center of some circles by maximizing the margin of
// the center point relative to the circles (interior) after subtracting
// nearby circles (exterior)
func (e *engine) computeTextCentre(interior, exterior []*Circle) *TextCentre {
	// get an initial estimate by sampling around the interior circles
	// and taking the point with the biggest margin
	points := []Point{}
	for _, c := range interior {
		points = append(points, Point{X: c.X, Y: c.Y})
		points = append(points, Point{X: c.X + float64(c.Radius/2), Y: c.Y})
		points = append(points, Point{X: c.X - float64(c.Radius/2), Y: c.Y})
		points = append(points, Point{X: c.X, Y: c.Y + float64(c.Radius/2)})
		points = append(points, Point{X: c.X, Y: c.Y - float64(c.Radius/2)})
	}

	if len(points) == 0 {
		// circleMargin reads interior[0].radius.
		throwUndefined("radius")
	}
	initial := points[0]
	margin := circleMargin(points[0], interior, exterior)

	for i := 1; i < len(points); i++ {
		m := circleMargin(points[i], interior, exterior)
		if m >= margin {
			initial = points[i]
			margin = m
		}
	}

	// maximize the margin numerically
	solution := e.nelderMead(
		func(p []float64) float64 { return -1 * circleMargin(Point{X: p[0], Y: p[1]}, interior, exterior) },
		[]float64{initial.X, initial.Y},
		nelderMeadParams{maxIterations: 500, minErrorDelta: 1e-10},
	).x

	ret := &TextCentre{X: solution[0], Y: solution[1]}

	// check solution, fallback as needed (happens if fully overlapped
	// etc)
	valid := true
	for _, i := range interior {
		if distance(ret.X, ret.Y, i.X, i.Y) > i.Radius {
			valid = false
			break
		}
	}

	for _, ext := range exterior {
		if distance(ret.X, ret.Y, ext.X, ext.Y) < ext.Radius {
			valid = false
			break
		}
	}
	if valid {
		return ret
	}

	if len(interior) == 1 {
		return &TextCentre{X: interior[0].X, Y: interior[0].Y}
	}
	stats := &areaStats{}
	e.intersectionArea(interior, stats)

	if len(stats.arcs) == 0 {
		return &TextCentre{X: 0, Y: -1000, Disjoint: true}
	}
	if len(stats.arcs) == 1 {
		return &TextCentre{X: stats.arcs[0].Circle.X, Y: stats.arcs[0].Circle.Y}
	}
	if len(exterior) > 0 {
		// try again without other circles
		return e.computeTextCentre(interior, nil)
	}
	// take average of all the points in the intersection
	// polygon. this should basically never happen
	// and has some issues:
	// https://github.com/benfred/venn.js/issues/48#issuecomment-146069777
	p1s := make([]Point, len(stats.arcs))
	for i, a := range stats.arcs {
		p1s[i] = a.P1
	}
	centre := getCenter(p1s)
	return &TextCentre{X: centre.X, Y: centre.Y}
}

// getOverlappingCircles is getOverlappingCircles(circles).
//
// given a dictionary of {setid : circle}, returns
// a dictionary of setid to list of circles that completely overlap it
func getOverlappingCircles(circles *js.Obj[*Circle]) *js.Obj[[]string] {
	ret := js.NewObj[[]string]()
	circleids := circles.Keys()
	for _, circleid := range circleids {
		ret.Set(circleid, []string{})
	}
	for i := 0; i < len(circleids); i++ {
		ci := circleids[i]
		a := circles.Get(ci)
		for j := i + 1; j < len(circleids); j++ {
			cj := circleids[j]
			b := circles.Get(cj)
			d := distance(a.X, a.Y, b.X, b.Y)

			if d+b.Radius <= a.Radius+1e-10 {
				ret.Set(cj, append(ret.Get(cj), ci))
			} else if d+a.Radius <= b.Radius+1e-10 {
				ret.Set(ci, append(ret.Get(ci), cj))
			}
		}
	}
	return ret
}

// computeTextCentres is computeTextCentres(circles, areas,
// symmetricalTextCentre) with symmetricalTextCentre false. The key of an
// area in the result is its set ids joined with commas.
func (e *engine) computeTextCentres(circles *js.Obj[*Circle], areas []*Area) *js.Obj[*TextCentre] {
	ret := js.NewObj[*TextCentre]()
	overlapped := getOverlappingCircles(circles)
	for i := 0; i < len(areas); i++ {
		area := areas[i].Sets
		areaids := js.NewObj[bool]()
		exclude := js.NewObj[bool]()

		for j := 0; j < len(area); j++ {
			areaids.Set(area[j], true)
			overlaps, ok := overlapped.Lookup(area[j])
			if !ok {
				throwUndefined("length")
			}
			// keep track of any circles that overlap this area,
			// and don't consider for purposes of computing the text
			// centre
			for k := 0; k < len(overlaps); k++ {
				exclude.Set(overlaps[k], true)
			}
		}

		interior := []*Circle{}
		exterior := []*Circle{}
		for _, setid := range circles.Keys() {
			if inObject(areaids, setid) {
				interior = append(interior, circles.Get(setid))
			} else if !inObject(exclude, setid) {
				exterior = append(exterior, circles.Get(setid))
			}
		}
		centre := e.computeTextCentre(interior, exterior)
		ret.Set(setsKey(area), centre)
		// if (centre.disjoint && areas[i].size > 0)
		//   console.log('WARNING: area ' + area + ' not represented on screen');
	}
	return ret
}

// joinPath is ret.join(' ') for the parts of a path: a string, or a number
// that prints as String(number) does.
func joinPath(parts ...any) string {
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteByte(' ')
		}
		switch v := p.(type) {
		case string:
			sb.WriteString(v)
		case float64:
			sb.WriteString(js.Str(v))
		case int:
			sb.WriteString(js.Str(float64(v)))
		}
	}
	return sb.String()
}

// circlePath is circlePath(x, y, r).
func circlePath(x, y, r float64) string {
	return joinPath(
		"\nM", x, y,
		"\nm", -r, 0,
		"\na", r, r, 0, 1, 0, r*2, 0,
		"\na", r, r, 0, 1, 0, -r*2, 0,
	)
}

// intersectionAreaArcs is intersectionAreaArcs(circles).
func (e *engine) intersectionAreaArcs(circles []*Circle) []Arc {
	if len(circles) == 0 {
		return []Arc{}
	}
	stats := &areaStats{}
	e.intersectionArea(circles, stats)
	return stats.arcs
}

// arcsToPath is arcsToPath(arcs, round). hasRound is false when round is
// undefined or null.
func (e *engine) arcsToPath(arcs []Arc, round float64, hasRound bool) string {
	if len(arcs) == 0 {
		return "M 0 0"
	}
	rFactor := e.m.Pow(10, orFloat(round, 0))
	r := func(v float64) float64 {
		if hasRound {
			return js.Round(v*rFactor) / rFactor
		}
		return v
	}
	if len(arcs) == 1 {
		circle := arcs[0].Circle
		return circlePath(r(circle.X), r(circle.Y), r(circle.Radius))
	}
	// draw path around arcs
	ret := []any{"\nM", r(arcs[0].P2.X), r(arcs[0].P2.Y)}
	for _, arc := range arcs {
		radius := r(arc.Circle.Radius)
		large := 0
		if arc.Large {
			large = 1
		}
		sweep := 0
		if arc.Sweep {
			sweep = 1
		}
		ret = append(ret, "\nA", radius, radius, 0, large, sweep, r(arc.P1.X), r(arc.P1.Y))
	}
	return joinPath(ret...)
}

// intersectionAreaPath is intersectionAreaPath(circles, round): it returns
// a svg path of the intersection area of a bunch of circles.
func (e *engine) intersectionAreaPath(circles []*Circle, round float64, hasRound bool) string {
	return e.arcsToPath(e.intersectionAreaArcs(circles), round, hasRound)
}

// LayoutOptions are the options of layout() that mermaid sets. All other
// options have their defaults: the default loss function, normalize on,
// orientation Math.PI / 2, round 2.
type LayoutOptions struct {
	Width   float64
	Height  float64
	Padding float64
}

// LayoutArea is one element of the result of layout():
// { data, text, circles, arcs, path, distinctPath }.
type LayoutArea struct {
	// Data is the area of the input.
	Data *Area
	// Text is the centre of the text of the area.
	Text *TextCentre
	// Circles has the circle of each set of the area, in the order of the
	// sets. SetID is the id of the set.
	Circles []*Circle
	// Arcs are the arcs of the border of the area.
	Arcs []Arc
	// Path is the SVG path of the area, with numbers rounded to 2 places.
	Path string
	// DistinctPath is Path and the paths of the areas that have more sets
	// and all sets of this area.
	DistinctPath string
}

// Layout is layout(data, options): the layout of the diagram without the
// DOM.
func Layout(env *Env, data []*Area, o LayoutOptions) ([]*LayoutArea, error) {
	e := newEngine(env)
	return run(func() []*LayoutArea {
		checkSets(data)
		return e.layout(data, o)
	})
}

// layout is layout(data, options).
func (e *engine) layout(data []*Area, o LayoutOptions) []*LayoutArea {
	const orientation = pi / 2
	const round = 2

	solution := e.venn(data)

	solution = e.normalizeSolution(solution, orientation)

	circles := scaleSolution(solution, o.Width, o.Height, o.Padding)
	textCentres := e.computeTextCentres(circles, data)

	circleLookup := js.NewMap[string, *Circle]()
	for _, set := range circles.Keys() {
		c := circles.Get(set)
		circleLookup.Set(set, &Circle{
			SetID:  set,
			X:      c.X,
			Y:      c.Y,
			Radius: c.Radius,
		})
	}
	type helper struct {
		circles []*Circle
		arcs    []Arc
		path    string
		area    *Area
		has     *js.Set[string]
	}
	helpers := make([]*helper, len(data))
	for i, area := range data {
		circles := make([]*Circle, len(area.Sets))
		for j, s := range area.Sets {
			circles[j] = circleLookup.Get(s)
		}
		arcs := e.intersectionAreaArcs(circles)
		path := e.arcsToPath(arcs, round, true)
		helpers[i] = &helper{circles: circles, arcs: arcs, path: path, area: area, has: js.NewSet(area.Sets...)}
	}

	genDistinctPath := func(sets []string) string {
		r := ""
		for _, h := range helpers {
			if h.has.Len() > len(sets) {
				every := true
				for _, s := range sets {
					if !h.has.Has(s) {
						every = false
						break
					}
				}
				if every {
					r += " " + h.path
				}
			}
		}
		return r
	}

	result := make([]*LayoutArea, len(helpers))
	for i, h := range helpers {
		result[i] = &LayoutArea{
			Data:         h.area,
			Text:         textCentres.Get(setsKey(h.area.Sets)),
			Circles:      h.circles,
			Arcs:         h.arcs,
			Path:         h.path,
			DistinctPath: h.path + genDistinctPath(h.area.Sets),
		}
	}
	return result
}
