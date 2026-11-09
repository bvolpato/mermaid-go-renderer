// Package axis is a port of d3-axis 3.0.0. An axis renders the ticks of a
// scale into an SVG group: a path with the class "domain", and one group
// with the class "tick" for each tick, with a line and a text.
//
// Ported from d3-axis 3.0.0, d3-axis/src/axis.js and d3-axis/src/identity.js
// (ISC). The parts of d3-selection that the axis uses on its container (the
// data join with a key, order, enter, merge) are ported from d3-selection
// 3.0.0, d3-selection/src/selection: data.js, order.js, enter.js, insert.js
// (ISC).
//
// Transitions do not exist here: Call is axis(selection), never
// axis(transition).
package axis

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Orient is the side of the axis on which the ticks are.
type Orient int

// The orientations of d3-axis.
const (
	OrientTop Orient = iota + 1
	OrientRight
	OrientBottom
	OrientLeft
)

// Scale is the part of a d3 scale that an axis reads. D is the type of the
// values of the domain. The functions TimeScale, LinearScale, BandScale and
// PointScale make a Scale from the scales of internal/d3/scale.
type Scale[D any] interface {
	// Ticks is scale.ticks(...tickArguments). ok is false for a scale that
	// has no ticks method, such as a band scale.
	Ticks(tickArguments []any) (values []D, ok bool)
	// Domain is scale.domain().
	Domain() []D
	// TickFormat is scale.tickFormat(...tickArguments). ok is false for a
	// scale that has no tickFormat method.
	TickFormat(tickArguments []any) (format func(d D) string, ok bool, err error)
	// Range is scale.range().
	Range() []float64
	// Bandwidth is scale.bandwidth(). ok is false for a scale that has no
	// bandwidth method.
	Bandwidth() (width float64, ok bool)
	// Round is scale.round(). The axis calls it only on a scale that has a
	// bandwidth.
	Round() bool
	// Copy is scale.copy().
	Copy() Scale[D]
	// Apply is +scale(d). It is NaN where the scale returns undefined.
	Apply(d D) float64
	// String is String(d): the identity format of a tick.
	String(d D) string
}

func translateX(x float64) string {
	return "translate(" + js.Str(x) + ",0)"
}

func translateY(y float64) string {
	return "translate(0," + js.Str(y) + ")"
}

// number is number(scale).
func number[D any](scale Scale[D]) func(d D) float64 {
	return func(d D) float64 { return scale.Apply(d) }
}

// center is center(scale, offset).
func center[D any](scale Scale[D], offset float64) func(d D) float64 {
	bandwidth, _ := scale.Bandwidth()
	offset = js.Max(0, bandwidth-offset*2) / 2
	if scale.Round() {
		offset = js.Round(offset)
	}
	return func(d D) float64 { return scale.Apply(d) + offset }
}

// Axis is an axis generator: the function that d3.axisBottom(scale) and the
// other three constructors return.
type Axis[D any] struct {
	orient        Orient
	scale         Scale[D]
	tickArguments []any
	tickValues    []D // nil is null
	tickFormat    func(d D) string
	tickSizeInner float64
	tickSizeOuter float64
	tickPadding   float64
	offset        float64

	// data holds node.__data__ of the tick groups that this axis made or
	// joined, and applied holds the containers that have node.__axis.
	// d3 keeps both on the DOM nodes. Here they belong to the Axis, so a
	// second axis on the same container sees a container without them.
	data    map[*dom.Node]D
	applied map[*dom.Node]bool
}

func newAxis[D any](orient Orient, scale Scale[D]) *Axis[D] {
	return &Axis[D]{
		orient:        orient,
		scale:         scale,
		tickArguments: []any{},
		tickSizeInner: 6,
		tickSizeOuter: 6,
		tickPadding:   3,
		// window.devicePixelRatio > 1 ? 0 : 0.5. The reference browser has a
		// device pixel ratio of 1.
		offset: 0.5,
	}
}

// Top is d3.axisTop(scale).
func Top[D any](scale Scale[D]) *Axis[D] { return newAxis(OrientTop, scale) }

// Right is d3.axisRight(scale).
func Right[D any](scale Scale[D]) *Axis[D] { return newAxis(OrientRight, scale) }

// Bottom is d3.axisBottom(scale).
func Bottom[D any](scale Scale[D]) *Axis[D] { return newAxis(OrientBottom, scale) }

// Left is d3.axisLeft(scale).
func Left[D any](scale Scale[D]) *Axis[D] { return newAxis(OrientLeft, scale) }

// Scale is axis.scale(scale).
func (a *Axis[D]) Scale(scale Scale[D]) *Axis[D] { a.scale = scale; return a }

// Ticks is axis.ticks(...arguments): the arguments for scale.ticks and
// scale.tickFormat. A number is a float64. A nil argument is null or
// undefined.
func (a *Axis[D]) Ticks(arguments ...any) *Axis[D] {
	a.tickArguments = append([]any{}, arguments...)
	return a
}

// TickArguments is axis.tickArguments(arguments). A nil slice is null.
func (a *Axis[D]) TickArguments(arguments []any) *Axis[D] {
	a.tickArguments = append([]any{}, arguments...)
	return a
}

// TickValues is axis.tickValues(values). A nil slice is null: the ticks
// come from the scale.
func (a *Axis[D]) TickValues(values []D) *Axis[D] {
	if values == nil {
		a.tickValues = nil
	} else {
		a.tickValues = append([]D{}, values...)
	}
	return a
}

// TickFormat is axis.tickFormat(format). A nil function is null: the format
// comes from the scale.
func (a *Axis[D]) TickFormat(format func(d D) string) *Axis[D] { a.tickFormat = format; return a }

// TickSize is axis.tickSize(size).
func (a *Axis[D]) TickSize(size float64) *Axis[D] {
	a.tickSizeInner, a.tickSizeOuter = size, size
	return a
}

// TickSizeInner is axis.tickSizeInner(size).
func (a *Axis[D]) TickSizeInner(size float64) *Axis[D] { a.tickSizeInner = size; return a }

// TickSizeOuter is axis.tickSizeOuter(size).
func (a *Axis[D]) TickSizeOuter(size float64) *Axis[D] { a.tickSizeOuter = size; return a }

// TickPadding is axis.tickPadding(padding).
func (a *Axis[D]) TickPadding(padding float64) *Axis[D] { a.tickPadding = padding; return a }

// Offset is axis.offset(offset). The default is 0.5, as on a display with a
// device pixel ratio of 1. Use 0 for a higher ratio.
func (a *Axis[D]) Offset(offset float64) *Axis[D] { a.offset = offset; return a }

func at(a []float64, i int) float64 {
	if i < 0 || i >= len(a) {
		return math.NaN()
	}
	return a[i]
}

// key is the key function of the data join: scale(d) + "".
func (a *Axis[D]) key(d D) string {
	v := a.scale.Apply(d)
	if math.IsNaN(v) {
		return "undefined"
	}
	return js.Str(v)
}

// Call is axis(selection) for a selection of one element: it renders the
// axis into the container. The error is the error of the tick format of the
// scale.
func (a *Axis[D]) Call(selection *dom.Node) error {
	if selection == nil {
		return nil
	}
	scale := a.scale
	orient := a.orient
	k := 1.0
	if orient == OrientTop || orient == OrientLeft {
		k = -1
	}
	x := "y"
	if orient == OrientLeft || orient == OrientRight {
		x = "x"
	}
	transform := translateY
	if orient == OrientTop || orient == OrientBottom {
		transform = translateX
	}
	offset := a.offset
	tickSizeInner, tickSizeOuter := a.tickSizeInner, a.tickSizeOuter

	var values []D
	if a.tickValues == nil {
		if v, ok := scale.Ticks(a.tickArguments); ok {
			values = v
		} else {
			values = scale.Domain()
		}
	} else {
		values = a.tickValues
	}
	format := a.tickFormat
	if format == nil {
		f, ok, err := scale.TickFormat(a.tickArguments)
		if err != nil {
			return err
		}
		if ok {
			format = f
		} else {
			format = scale.String
		}
	}
	spacing := js.Max(tickSizeInner, 0) + a.tickPadding
	rng := scale.Range()
	range0 := at(rng, 0) + offset
	range1 := at(rng, len(rng)-1) + offset
	var position func(d D) float64
	if _, ok := scale.Bandwidth(); ok {
		position = center(scale.Copy(), offset)
	} else {
		position = number(scale.Copy())
	}

	// path = selection.selectAll(".domain").data([null])
	var path *dom.Node
	if domains := selection.SelectAll(".domain"); len(domains) > 0 {
		path = domains[0]
	}

	// tick = selection.selectAll(".tick").data(values, scale).order()
	tick, tickExit := a.bindKey(selection.SelectAll(".tick"), values)
	order(tick)

	// tickEnter = tick.enter().append("g").attr("class", "tick")
	tickEnter := make([]*dom.Node, len(values))
	for i0, i1 := 0, 0; i0 < len(values); i0++ {
		if tick[i0] != nil {
			continue
		}
		// The enter node goes before the update node that follows it.
		if i0 >= i1 {
			i1 = i0 + 1
		}
		for i1 < len(values) && tick[i1] == nil {
			i1++
		}
		var next *dom.Node
		if i1 < len(values) {
			next = tick[i1]
		}
		g := selection.Append("g")
		if next != nil && next.Parent == selection {
			selection.InsertBefore(g, next)
		}
		g.Attr("class", "tick")
		a.bind(g, values[i0])
		tickEnter[i0] = g
	}

	// line = tick.select("line"), text = tick.select("text")
	line := make([]*dom.Node, len(values))
	text := make([]*dom.Node, len(values))
	for i, node := range tick {
		line[i] = node.Select("line")
		text[i] = node.Select("text")
	}

	// path = path.merge(path.enter().insert("path", ".tick") ...)
	if path == nil {
		path = selection.Insert("path", ".tick").
			Attr("class", "domain").
			Attr("stroke", "currentColor")
	}

	// tick = tick.merge(tickEnter)
	for i := range tick {
		if tick[i] == nil {
			tick[i] = tickEnter[i]
		}
	}

	// line = line.merge(tickEnter.append("line") ...)
	for i, node := range tickEnter {
		if node == nil {
			continue
		}
		l := node.Append("line").
			Attr("stroke", "currentColor").
			Attr(x+"2", k*tickSizeInner)
		if line[i] == nil {
			line[i] = l
		}
	}

	// text = text.merge(tickEnter.append("text") ...)
	dy := "0.32em"
	if orient == OrientTop {
		dy = "0em"
	} else if orient == OrientBottom {
		dy = "0.71em"
	}
	for i, node := range tickEnter {
		if node == nil {
			continue
		}
		t := node.Append("text").
			Attr("fill", "currentColor").
			Attr(x, k*spacing).
			Attr("dy", dy)
		if text[i] == nil {
			text[i] = t
		}
	}

	// tickExit.remove()
	for _, node := range tickExit {
		node.Remove()
		delete(a.data, node)
	}

	var d string
	if orient == OrientLeft || orient == OrientRight {
		if tickSizeOuter != 0 && !math.IsNaN(tickSizeOuter) {
			d = "M" + js.Str(k*tickSizeOuter) + "," + js.Str(range0) + "H" + js.Str(offset) + "V" + js.Str(range1) + "H" + js.Str(k*tickSizeOuter)
		} else {
			d = "M" + js.Str(offset) + "," + js.Str(range0) + "V" + js.Str(range1)
		}
	} else {
		if tickSizeOuter != 0 && !math.IsNaN(tickSizeOuter) {
			d = "M" + js.Str(range0) + "," + js.Str(k*tickSizeOuter) + "V" + js.Str(offset) + "H" + js.Str(range1) + "V" + js.Str(k*tickSizeOuter)
		} else {
			d = "M" + js.Str(range0) + "," + js.Str(offset) + "H" + js.Str(range1)
		}
	}
	path.Attr("d", d)

	for i, node := range tick {
		node.Attr("opacity", 1).
			Attr("transform", transform(position(values[i])+offset))
	}

	for _, node := range line {
		node.Attr(x+"2", k*tickSizeInner)
	}

	for i, node := range text {
		if node == nil {
			continue
		}
		node.Attr(x, k*spacing).
			Text(format(values[i]))
	}

	// selection.filter(entering)
	if !a.applied[selection] {
		anchor := "middle"
		if orient == OrientRight {
			anchor = "start"
		} else if orient == OrientLeft {
			anchor = "end"
		}
		selection.
			Attr("fill", "none").
			Attr("font-size", 10).
			Attr("font-family", "sans-serif").
			Attr("text-anchor", anchor)
	}

	// selection.each(function() { this.__axis = position; })
	if a.applied == nil {
		a.applied = map[*dom.Node]bool{}
	}
	a.applied[selection] = true
	return nil
}

func (a *Axis[D]) bind(node *dom.Node, d D) {
	if a.data == nil {
		a.data = map[*dom.Node]D{}
	}
	a.data[node] = d
}

// bindKey is bindKey of d3-selection/src/selection/data.js with the scale
// as the key function. It returns the update nodes by the index of their
// datum, with nil where a datum has no node, and the exit nodes.
func (a *Axis[D]) bindKey(group []*dom.Node, data []D) (update, exit []*dom.Node) {
	update = make([]*dom.Node, len(data))
	nodeByKeyValue := map[string]*dom.Node{}
	keyValues := make([]string, len(group))
	exitAt := make([]*dom.Node, len(group))

	// Compute the key for each node. If multiple nodes have the same key,
	// the duplicates are added to exit.
	for i, node := range group {
		keyValue := "undefined"
		if d, ok := a.data[node]; ok {
			keyValue = a.key(d)
		}
		keyValues[i] = keyValue
		if _, has := nodeByKeyValue[keyValue]; has {
			exitAt[i] = node
		} else {
			nodeByKeyValue[keyValue] = node
		}
	}

	// Compute the key for each datum. If there a node associated with this
	// key, join and add it to update. If there is not (or the key is a
	// duplicate), add it to enter.
	for i, d := range data {
		keyValue := a.key(d)
		if node := nodeByKeyValue[keyValue]; node != nil {
			update[i] = node
			a.bind(node, d)
			delete(nodeByKeyValue, keyValue)
		}
	}

	// Add any remaining nodes that were not bound to data to exit.
	for i, node := range group {
		if nodeByKeyValue[keyValues[i]] == node {
			exitAt[i] = node
		}
	}
	for _, node := range exitAt {
		if node != nil {
			exit = append(exit, node)
		}
	}
	return update, exit
}

// order is selection.order(): it moves the nodes so that the document order
// is the order of the selection.
func order(group []*dom.Node) {
	i := len(group) - 1
	if i < 0 {
		return
	}
	next := group[i]
	for i--; i >= 0; i-- {
		node := group[i]
		if node == nil {
			continue
		}
		if next != nil && !strictlyFollows(node, next) && next.Parent != nil {
			next.Parent.InsertBefore(node, next)
		}
		next = node
	}
}

// strictlyFollows reports whether node.compareDocumentPosition(next) is
// DOCUMENT_POSITION_FOLLOWING and nothing else: next comes after node in
// document order and node does not contain next.
func strictlyFollows(node, next *dom.Node) bool {
	if node == next {
		return false
	}
	pa, pb := ancestors(node), ancestors(next)
	if pa[0] != pb[0] {
		return false
	}
	i := 0
	for i < len(pa) && i < len(pb) && pa[i] == pb[i] {
		i++
	}
	if i == len(pa) || i == len(pb) {
		// One contains the other.
		return false
	}
	// pa[i] and pb[i] are different children of the same parent.
	for c := pa[i].Next; c != nil; c = c.Next {
		if c == pb[i] {
			return true
		}
	}
	return false
}

// ancestors returns the path from the root to n.
func ancestors(n *dom.Node) []*dom.Node {
	var path []*dom.Node
	for e := n; e != nil; e = e.Parent {
		path = append(path, e)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}
