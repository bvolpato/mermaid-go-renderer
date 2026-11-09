// Ported from roughjs 4.6.6, roughjs/bin/renderer.js (MIT).
//
// The functions keep the names of the JavaScript source, with the leading
// underscore of the private helpers. Each product that is an operand of an
// addition or a subtraction has an explicit float64 conversion. The
// conversion stops the compiler from fusing the two operations into one
// fused multiply-add, which rounds once where JavaScript rounds twice.

package rough

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

func line(x1, y1, x2, y2 float64, o *ResolvedOptions) OpSet {
	return OpSet{Type: SetPath, Ops: _doubleLine(x1, y1, x2, y2, o, false)}
}

func linearPath(points []*Point, close bool, o *ResolvedOptions) OpSet {
	n := len(points)
	if n > 2 {
		ops := []Op{}
		for i := 0; i < n-1; i++ {
			ops = append(ops, _doubleLine(points[i][0], points[i][1], points[i+1][0], points[i+1][1], o, false)...)
		}
		if close {
			ops = append(ops, _doubleLine(points[n-1][0], points[n-1][1], points[0][0], points[0][1], o, false)...)
		}
		return OpSet{Type: SetPath, Ops: ops}
	} else if n == 2 {
		return line(points[0][0], points[0][1], points[1][0], points[1][1], o)
	}
	return OpSet{Type: SetPath, Ops: []Op{}}
}

func polygon(points []*Point, o *ResolvedOptions) OpSet {
	return linearPath(points, true, o)
}

func rectangle(x, y, width, height float64, o *ResolvedOptions) OpSet {
	points := []*Point{
		{x, y},
		{x + width, y},
		{x + width, y + height},
		{x, y + height},
	}
	return polygon(points, o)
}

// curve takes the list of point lists. The JavaScript function also accepts
// one list of points, which the caller wraps in a list. A nil pointsList
// stands for an empty input.
func curve(pointsList [][]*Point, o *ResolvedOptions) OpSet {
	if len(pointsList) > 0 {
		o1 := _curveWithOffset(pointsList[0], float64(1*(1+float64(o.roughness*0.2))), o)
		var o2 []Op
		if o.disableMultiStroke {
			o2 = []Op{}
		} else {
			o2 = _curveWithOffset(pointsList[0], float64(1.5*(1+float64(o.roughness*0.22))), cloneOptionsAlterSeed(o))
		}
		for i := 1; i < len(pointsList); i++ {
			points := pointsList[i]
			if len(points) > 0 {
				underlay := _curveWithOffset(points, float64(1*(1+float64(o.roughness*0.2))), o)
				var overlay []Op
				if o.disableMultiStroke {
					overlay = []Op{}
				} else {
					overlay = _curveWithOffset(points, float64(1.5*(1+float64(o.roughness*0.22))), cloneOptionsAlterSeed(o))
				}
				for _, item := range underlay {
					if item.Op != OpMove {
						o1 = append(o1, item)
					}
				}
				for _, item := range overlay {
					if item.Op != OpMove {
						o2 = append(o2, item)
					}
				}
			}
		}
		return OpSet{Type: SetPath, Ops: append(o1, o2...)}
	}
	return OpSet{Type: SetPath, Ops: []Op{}}
}

func ellipse(x, y, width, height float64, o *ResolvedOptions) OpSet {
	params := generateEllipseParams(width, height, o)
	return ellipseWithParams(x, y, o, params).opset
}

type ellipseParams struct {
	increment, rx, ry float64
}

type ellipseResult struct {
	estimatedPoints []*Point
	opset           OpSet
}

func generateEllipseParams(width, height float64, o *ResolvedOptions) ellipseParams {
	m := &o.gen.m
	pi := math.Pi
	psq := math.Sqrt(pi * 2 * math.Sqrt((m.pow(width/2, 2)+m.pow(height/2, 2))/2))
	stepCount := math.Ceil(js.Max(o.curveStepCount, float64((o.curveStepCount/math.Sqrt(200))*psq)))
	increment := (pi * 2) / stepCount
	rx := math.Abs(width / 2)
	ry := math.Abs(height / 2)
	curveFitRandomness := 1 - o.curveFitting
	rx += _offsetOpt(float64(rx*curveFitRandomness), o, 1)
	ry += _offsetOpt(float64(ry*curveFitRandomness), o, 1)
	return ellipseParams{increment: increment, rx: rx, ry: ry}
}

func ellipseWithParams(x, y float64, o *ResolvedOptions, ep ellipseParams) ellipseResult {
	inner := _offset(0.4, 1, o, 1)
	overlap := float64(ep.increment * _offset(0.1, inner, o, 1))
	ap1, cp1 := _computeEllipsePoints(ep.increment, x, y, ep.rx, ep.ry, 1, overlap, o)
	o1 := _curve(ap1, nil, o)
	if !o.disableMultiStroke && o.roughness != 0 {
		ap2, _ := _computeEllipsePoints(ep.increment, x, y, ep.rx, ep.ry, 1.5, 0, o)
		o2 := _curve(ap2, nil, o)
		o1 = append(o1, o2...)
	}
	return ellipseResult{
		estimatedPoints: cp1,
		opset:           OpSet{Type: SetPath, Ops: o1},
	}
}

func arc(x, y, width, height, start, stop float64, closed, roughClosure bool, o *ResolvedOptions) OpSet {
	m := &o.gen.m
	pi := math.Pi
	cx := x
	cy := y
	rx := math.Abs(width / 2)
	ry := math.Abs(height / 2)
	rx += _offsetOpt(float64(rx*0.01), o, 1)
	ry += _offsetOpt(float64(ry*0.01), o, 1)
	strt := start
	stp := stop
	o.gen.need(-strt / float64(pi*2))
	for strt < 0 {
		next := strt + float64(pi*2)
		if next == strt {
			// The start angle is too far below zero to change: the
			// JavaScript loop does not end.
			hang("the start angle of the arc never becomes positive")
		}
		o.gen.step()
		strt = next
		stp += float64(pi * 2)
	}
	if (stp - strt) > (pi * 2) {
		strt = 0
		stp = float64(pi * 2)
	}
	ellipseInc := (pi * 2) / o.curveStepCount
	arcInc := js.Min(ellipseInc/2, (stp-strt)/2)
	ops := _arc(arcInc, cx, cy, rx, ry, strt, stp, 1, o)
	if !o.disableMultiStroke {
		o2 := _arc(arcInc, cx, cy, rx, ry, strt, stp, 1.5, o)
		ops = append(ops, o2...)
	}
	if closed {
		if roughClosure {
			l1 := _doubleLine(cx, cy, cx+float64(rx*m.cos(strt)), cy+float64(ry*m.sin(strt)), o, false)
			l2 := _doubleLine(cx, cy, cx+float64(rx*m.cos(stp)), cy+float64(ry*m.sin(stp)), o, false)
			ops = append(ops, l1...)
			ops = append(ops, l2...)
		} else {
			ops = append(ops,
				Op{Op: OpLineTo, Data: []float64{cx, cy}},
				Op{Op: OpLineTo, Data: []float64{cx + float64(rx*m.cos(strt)), cy + float64(ry*m.sin(strt))}},
			)
		}
	}
	return OpSet{Type: SetPath, Ops: ops}
}

func svgPath(path string, o *ResolvedOptions) OpSet {
	segments := normalize(o.gen, absolutize(parsePath(path)))
	ops := []Op{}
	first := Point{0, 0}
	current := Point{0, 0}
	for _, seg := range segments {
		data := seg.data
		switch seg.key {
		case 'M':
			current = Point{data[0], data[1]}
			first = Point{data[0], data[1]}
		case 'L':
			ops = append(ops, _doubleLine(current[0], current[1], data[0], data[1], o, false)...)
			current = Point{data[0], data[1]}
		case 'C':
			x1, y1, x2, y2, x, y := data[0], data[1], data[2], data[3], data[4], data[5]
			ops = append(ops, _bezierTo(x1, y1, x2, y2, x, y, current, o)...)
			current = Point{x, y}
		case 'Z':
			ops = append(ops, _doubleLine(current[0], current[1], first[0], first[1], o, false)...)
			current = Point{first[0], first[1]}
		}
	}
	return OpSet{Type: SetPath, Ops: ops}
}

// Fills

func solidFillPolygon(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	ops := []Op{}
	for _, points := range polygonList {
		if len(points) > 0 {
			offset := orNum(o.maxRandomnessOffset, 0)
			n := len(points)
			if n > 2 {
				x := points[0][0] + _offsetOpt(offset, o, 1)
				y := points[0][1] + _offsetOpt(offset, o, 1)
				ops = append(ops, Op{Op: OpMove, Data: []float64{x, y}})
				for i := 1; i < n; i++ {
					x := points[i][0] + _offsetOpt(offset, o, 1)
					y := points[i][1] + _offsetOpt(offset, o, 1)
					ops = append(ops, Op{Op: OpLineTo, Data: []float64{x, y}})
				}
			}
		}
	}
	return OpSet{Type: SetFillPath, Ops: ops}
}

func patternFillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	return getFiller(o).fillPolygons(polygonList, o)
}

func patternFillArc(x, y, width, height, start, stop float64, o *ResolvedOptions) OpSet {
	m := &o.gen.m
	pi := math.Pi
	cx := x
	cy := y
	rx := math.Abs(width / 2)
	ry := math.Abs(height / 2)
	rx += _offsetOpt(float64(rx*0.01), o, 1)
	ry += _offsetOpt(float64(ry*0.01), o, 1)
	strt := start
	stp := stop
	o.gen.need(-strt / float64(pi*2))
	for strt < 0 {
		next := strt + float64(pi*2)
		if next == strt {
			hang("the start angle of the arc never becomes positive")
		}
		o.gen.step()
		strt = next
		stp += float64(pi * 2)
	}
	if (stp - strt) > (pi * 2) {
		strt = 0
		stp = float64(pi * 2)
	}
	increment := (stp - strt) / o.curveStepCount
	points := []*Point{}
	o.gen.need((stp - strt) / increment)
	for angle := strt; angle <= stp; angle = angle + increment {
		if increment <= 0 {
			hang("the angle increment of the arc is not positive")
		}
		o.gen.step()
		points = append(points, &Point{cx + float64(rx*m.cos(angle)), cy + float64(ry*m.sin(angle))})
	}
	points = append(points, &Point{cx + float64(rx*m.cos(stp)), cy + float64(ry*m.sin(stp))})
	points = append(points, &Point{cx, cy})
	return patternFillPolygons([][]*Point{points}, o)
}

// The helper object of renderer.js also has randOffset and
// randOffsetWithRange. No filler calls them, so the port leaves them out.

func doubleLineFillOps(x1, y1, x2, y2 float64, o *ResolvedOptions) []Op {
	return _doubleLine(x1, y1, x2, y2, o, true)
}

// Private helpers

func cloneOptionsAlterSeed(ops *ResolvedOptions) *ResolvedOptions {
	result := ops.clone()
	result.randomizer = nil
	if truthy(ops.seed) {
		// result.seed = ops.seed + 1
		switch ops.seed.(type) {
		case string, []any, []float64, *js.Obj[any]:
			result.seed = toString(ops.seed) + "1"
		default:
			result.seed = toNumber(ops.seed) + 1
		}
	}
	return result
}

// _random is the function random of renderer.js.
func _random(ops *ResolvedOptions) float64 {
	if ops.randomizer == nil {
		// new Random(ops.seed || 0)
		var seed any = 0.0
		if truthy(ops.seed) {
			seed = ops.seed
		}
		ops.randomizer = newRandom(seed, ops.gen)
	}
	return ops.randomizer.next()
}

func _offset(min, max float64, ops *ResolvedOptions, roughnessGain float64) float64 {
	return float64(ops.roughness * roughnessGain * (float64(_random(ops)*(max-min)) + min))
}

func _offsetOpt(x float64, ops *ResolvedOptions, roughnessGain float64) float64 {
	return _offset(-x, x, ops, roughnessGain)
}

func _doubleLine(x1, y1, x2, y2 float64, o *ResolvedOptions, filling bool) []Op {
	singleStroke := o.disableMultiStroke
	if filling {
		singleStroke = o.disableMultiStrokeFill
	}
	o1 := _line(x1, y1, x2, y2, o, true, false)
	if singleStroke {
		return o1
	}
	o2 := _line(x1, y1, x2, y2, o, true, true)
	return append(o1, o2...)
}

func _line(x1, y1, x2, y2 float64, o *ResolvedOptions, move, overlay bool) []Op {
	m := &o.gen.m
	lengthSq := m.pow(x1-x2, 2) + m.pow(y1-y2, 2)
	length := math.Sqrt(lengthSq)
	roughnessGain := 1.0
	if length < 200 {
		roughnessGain = 1
	} else if length > 500 {
		roughnessGain = 0.4
	} else {
		roughnessGain = float64(-0.0016668*length) + 1.233334
	}
	offset := orNum(o.maxRandomnessOffset, 0)
	if float64(offset*offset*100) > lengthSq {
		offset = length / 10
	}
	halfOffset := float64(offset / 2)
	divergePoint := 0.2 + float64(_random(o)*0.2)
	midDispX := o.bowing * o.maxRandomnessOffset * (y2 - y1) / 200
	midDispY := o.bowing * o.maxRandomnessOffset * (x1 - x2) / 200
	midDispX = _offsetOpt(midDispX, o, roughnessGain)
	midDispY = _offsetOpt(midDispY, o, roughnessGain)
	ops := make([]Op, 0, 2)
	randomHalf := func() float64 { return _offsetOpt(halfOffset, o, roughnessGain) }
	randomFull := func() float64 { return _offsetOpt(offset, o, roughnessGain) }
	preserveVertices := o.preserveVertices
	// vertex is (preserveVertices ? 0 : rnd()).
	vertex := func(rnd func() float64) float64 {
		if preserveVertices {
			return 0
		}
		return rnd()
	}
	if move {
		if overlay {
			x := x1 + vertex(randomHalf)
			y := y1 + vertex(randomHalf)
			ops = append(ops, Op{Op: OpMove, Data: []float64{x, y}})
		} else {
			x := x1 + vertex(randomFull)
			y := y1 + vertex(randomFull)
			ops = append(ops, Op{Op: OpMove, Data: []float64{x, y}})
		}
	}
	rnd := randomFull
	if overlay {
		rnd = randomHalf
	}
	d0 := midDispX + x1 + float64((x2-x1)*divergePoint) + rnd()
	d1 := midDispY + y1 + float64((y2-y1)*divergePoint) + rnd()
	d2 := midDispX + x1 + float64(2*(x2-x1)*divergePoint) + rnd()
	d3 := midDispY + y1 + float64(2*(y2-y1)*divergePoint) + rnd()
	d4 := x2 + vertex(rnd)
	d5 := y2 + vertex(rnd)
	ops = append(ops, Op{Op: OpBcurveTo, Data: []float64{d0, d1, d2, d3, d4, d5}})
	return ops
}

func _curveWithOffset(points []*Point, offset float64, o *ResolvedOptions) []Op {
	if len(points) == 0 {
		return []Op{}
	}
	ps := []*Point{}
	add := func(p *Point) {
		x := p[0] + _offsetOpt(offset, o, 1)
		y := p[1] + _offsetOpt(offset, o, 1)
		ps = append(ps, &Point{x, y})
	}
	add(points[0])
	add(points[0])
	for i := 1; i < len(points); i++ {
		add(points[i])
		if i == len(points)-1 {
			add(points[i])
		}
	}
	return _curve(ps, nil, o)
}

func _curve(points []*Point, closePoint *Point, o *ResolvedOptions) []Op {
	n := len(points)
	ops := []Op{}
	if n > 3 {
		s := 1 - o.curveTightness
		ops = append(ops, Op{Op: OpMove, Data: []float64{points[1][0], points[1][1]}})
		for i := 1; (i + 2) < n; i++ {
			cachedVertArray := points[i]
			b1x := cachedVertArray[0] + (float64(s*points[i+1][0])-float64(s*points[i-1][0]))/6
			b1y := cachedVertArray[1] + (float64(s*points[i+1][1])-float64(s*points[i-1][1]))/6
			b2x := points[i+1][0] + (float64(s*points[i][0])-float64(s*points[i+2][0]))/6
			b2y := points[i+1][1] + (float64(s*points[i][1])-float64(s*points[i+2][1]))/6
			b3x := points[i+1][0]
			b3y := points[i+1][1]
			ops = append(ops, Op{Op: OpBcurveTo, Data: []float64{b1x, b1y, b2x, b2y, b3x, b3y}})
		}
		if closePoint != nil {
			ro := o.maxRandomnessOffset
			x := closePoint[0] + _offsetOpt(ro, o, 1)
			y := closePoint[1] + _offsetOpt(ro, o, 1)
			ops = append(ops, Op{Op: OpLineTo, Data: []float64{x, y}})
		}
	} else if n == 3 {
		ops = append(ops, Op{Op: OpMove, Data: []float64{points[1][0], points[1][1]}})
		ops = append(ops, Op{
			Op: OpBcurveTo,
			Data: []float64{
				points[1][0], points[1][1],
				points[2][0], points[2][1],
				points[2][0], points[2][1],
			},
		})
	} else if n == 2 {
		ops = append(ops, _line(points[0][0], points[0][1], points[1][0], points[1][1], o, true, true)...)
	}
	return ops
}

func _computeEllipsePoints(increment, cx, cy, rx, ry, offset, overlap float64, o *ResolvedOptions) (allPoints, corePoints []*Point) {
	m := &o.gen.m
	pi := math.Pi
	coreOnly := o.roughness == 0
	corePoints = []*Point{}
	allPoints = []*Point{}
	if coreOnly {
		increment = float64(increment / 4)
		allPoints = append(allPoints, &Point{
			cx + float64(rx*m.cos(-increment)),
			cy + float64(ry*m.sin(-increment)),
		})
		o.gen.need(float64(pi*2) / increment)
		for angle := 0.0; angle <= pi*2; angle = angle + increment {
			if increment <= 0 {
				hang("the angle increment of the ellipse is not positive")
			}
			o.gen.step()
			p := &Point{
				cx + float64(rx*m.cos(angle)),
				cy + float64(ry*m.sin(angle)),
			}
			corePoints = append(corePoints, p)
			allPoints = append(allPoints, p)
		}
		allPoints = append(allPoints, &Point{
			cx + float64(rx*m.cos(0)),
			cy + float64(ry*m.sin(0)),
		})
		allPoints = append(allPoints, &Point{
			cx + float64(rx*m.cos(increment)),
			cy + float64(ry*m.sin(increment)),
		})
	} else {
		radOffset := _offsetOpt(0.5, o, 1) - (pi / 2)
		{
			x := _offsetOpt(offset, o, 1) + cx + float64(0.9*rx*m.cos(radOffset-increment))
			y := _offsetOpt(offset, o, 1) + cy + float64(0.9*ry*m.sin(radOffset-increment))
			allPoints = append(allPoints, &Point{x, y})
		}
		endAngle := float64(pi*2) + radOffset - 0.01
		o.gen.need((endAngle - radOffset) / increment)
		for angle := radOffset; angle < endAngle; angle = angle + increment {
			if increment <= 0 {
				hang("the angle increment of the ellipse is not positive")
			}
			o.gen.step()
			x := _offsetOpt(offset, o, 1) + cx + float64(rx*m.cos(angle))
			y := _offsetOpt(offset, o, 1) + cy + float64(ry*m.sin(angle))
			p := &Point{x, y}
			corePoints = append(corePoints, p)
			allPoints = append(allPoints, p)
		}
		{
			x := _offsetOpt(offset, o, 1) + cx + float64(rx*m.cos(radOffset+float64(pi*2)+float64(overlap*0.5)))
			y := _offsetOpt(offset, o, 1) + cy + float64(ry*m.sin(radOffset+float64(pi*2)+float64(overlap*0.5)))
			allPoints = append(allPoints, &Point{x, y})
		}
		{
			x := _offsetOpt(offset, o, 1) + cx + float64(0.98*rx*m.cos(radOffset+overlap))
			y := _offsetOpt(offset, o, 1) + cy + float64(0.98*ry*m.sin(radOffset+overlap))
			allPoints = append(allPoints, &Point{x, y})
		}
		{
			x := _offsetOpt(offset, o, 1) + cx + float64(0.9*rx*m.cos(radOffset+float64(overlap*0.5)))
			y := _offsetOpt(offset, o, 1) + cy + float64(0.9*ry*m.sin(radOffset+float64(overlap*0.5)))
			allPoints = append(allPoints, &Point{x, y})
		}
	}
	return allPoints, corePoints
}

func _arc(increment, cx, cy, rx, ry, strt, stp, offset float64, o *ResolvedOptions) []Op {
	m := &o.gen.m
	radOffset := strt + _offsetOpt(0.1, o, 1)
	points := []*Point{}
	{
		x := _offsetOpt(offset, o, 1) + cx + float64(0.9*rx*m.cos(radOffset-increment))
		y := _offsetOpt(offset, o, 1) + cy + float64(0.9*ry*m.sin(radOffset-increment))
		points = append(points, &Point{x, y})
	}
	o.gen.need((stp - radOffset) / increment)
	for angle := radOffset; angle <= stp; angle = angle + increment {
		if increment <= 0 {
			hang("the angle increment of the arc is not positive")
		}
		o.gen.step()
		x := _offsetOpt(offset, o, 1) + cx + float64(rx*m.cos(angle))
		y := _offsetOpt(offset, o, 1) + cy + float64(ry*m.sin(angle))
		points = append(points, &Point{x, y})
	}
	points = append(points, &Point{
		cx + float64(rx*m.cos(stp)),
		cy + float64(ry*m.sin(stp)),
	})
	points = append(points, &Point{
		cx + float64(rx*m.cos(stp)),
		cy + float64(ry*m.sin(stp)),
	})
	return _curve(points, nil, o)
}

func _bezierTo(x1, y1, x2, y2, x, y float64, current Point, o *ResolvedOptions) []Op {
	ops := []Op{}
	ros := [2]float64{orNum(o.maxRandomnessOffset, 1), orNum(o.maxRandomnessOffset, 1) + 0.3}
	var f Point
	iterations := 2
	if o.disableMultiStroke {
		iterations = 1
	}
	preserveVertices := o.preserveVertices
	for i := 0; i < iterations; i++ {
		if i == 0 {
			ops = append(ops, Op{Op: OpMove, Data: []float64{current[0], current[1]}})
		} else {
			mx, my := 0.0, 0.0
			if !preserveVertices {
				mx = _offsetOpt(ros[0], o, 1)
			}
			if !preserveVertices {
				my = _offsetOpt(ros[0], o, 1)
			}
			ops = append(ops, Op{Op: OpMove, Data: []float64{current[0] + mx, current[1] + my}})
		}
		if preserveVertices {
			f = Point{x, y}
		} else {
			fx := x + _offsetOpt(ros[i], o, 1)
			fy := y + _offsetOpt(ros[i], o, 1)
			f = Point{fx, fy}
		}
		c1x := x1 + _offsetOpt(ros[i], o, 1)
		c1y := y1 + _offsetOpt(ros[i], o, 1)
		c2x := x2 + _offsetOpt(ros[i], o, 1)
		c2y := y2 + _offsetOpt(ros[i], o, 1)
		ops = append(ops, Op{Op: OpBcurveTo, Data: []float64{c1x, c1y, c2x, c2y, f[0], f[1]}})
	}
	return ops
}
