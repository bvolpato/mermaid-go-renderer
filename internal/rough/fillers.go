// Ported from roughjs 4.6.6, roughjs/bin/fillers/filler.js, hachure-filler.js,
// hatch-filler.js, zigzag-filler.js, zigzag-line-filler.js, dashed-filler.js,
// dot-filler.js and scan-line-hachure.js (MIT).

package rough

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// patternFiller is the PatternFiller interface of roughjs.
type patternFiller interface {
	fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet
}

// getFiller returns the filler for o.fillStyle. roughjs keeps one instance
// of each filler in a module variable. The fillers have no state, so the port
// makes a new value for each call.
func getFiller(o *ResolvedOptions) patternFiller {
	// let fillerName = o.fillStyle || 'hachure'
	fillerName, _ := o.fillStyle.(string)
	switch fillerName {
	case "zigzag":
		return zigZagFiller{}
	case "cross-hatch":
		return hatchFiller{}
	case "dots":
		return dotFiller{}
	case "dashed":
		return dashedFiller{}
	case "zigzag-line":
		return zigZagLineFiller{}
	}
	return hachureFiller{}
}

// polygonHachureLines is the function of scan-line-hachure.js.
func polygonHachureLines(polygonList [][]*Point, o *ResolvedOptions) [][2]*Point {
	angle := o.hachureAngle + 90
	gap := o.hachureGap
	if gap < 0 {
		gap = float64(o.strokeWidthNum() * 4)
	}
	gap = mathRound(js.Max(gap, 0.1))
	skipOffset := 1.0
	if o.roughness >= 1 {
		// (o.randomizer?.next() || Math.random()) > 0.7
		v := math.NaN()
		if o.randomizer != nil {
			v = o.randomizer.next()
		}
		if !truthyNum(v) {
			v = o.gen.mathRandom()
		}
		if v > 0.7 {
			skipOffset = gap
		}
	}
	return hachureLines(o.gen, polygonList, gap, angle, orNum(skipOffset, 1))
}

// hachureFiller is the HachureFiller class.
type hachureFiller struct{}

func (f hachureFiller) fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	return f._fillPolygons(polygonList, o)
}

func (f hachureFiller) _fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	lines := polygonHachureLines(polygonList, o)
	ops := f.renderLines(lines, o)
	return OpSet{Type: SetFillSketch, Ops: ops}
}

func (hachureFiller) renderLines(lines [][2]*Point, o *ResolvedOptions) []Op {
	ops := []Op{}
	for _, line := range lines {
		ops = append(ops, doubleLineFillOps(line[0][0], line[0][1], line[1][0], line[1][1], o)...)
	}
	return ops
}

// hatchFiller is the HatchFiller class.
type hatchFiller struct{ hachureFiller }

func (f hatchFiller) fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	set := f._fillPolygons(polygonList, o)
	o2 := o.clone()
	o2.hachureAngle = o.hachureAngle + 90
	set2 := f._fillPolygons(polygonList, o2)
	set.Ops = append(set.Ops, set2.Ops...)
	return set
}

// zigZagFiller is the ZigZagFiller class.
type zigZagFiller struct{ hachureFiller }

func (f zigZagFiller) fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	m := &o.gen.m
	pi := math.Pi
	gap := o.hachureGap
	if gap < 0 {
		gap = float64(o.strokeWidthNum() * 4)
	}
	gap = js.Max(gap, 0.1)
	o2 := o.clone()
	o2.hachureGap = gap
	lines := polygonHachureLines(polygonList, o2)
	zigZagAngle := float64((pi / 180) * o.hachureAngle)
	zigzagLines := [][2]*Point{}
	dgx := float64(gap * 0.5 * m.cos(zigZagAngle))
	dgy := float64(gap * 0.5 * m.sin(zigZagAngle))
	for _, line := range lines {
		p1, p2 := line[0], line[1]
		if truthyNum(lineLength(m, p1, p2)) {
			zigzagLines = append(zigzagLines,
				[2]*Point{{p1[0] - dgx, p1[1] + dgy}, {p2[0], p2[1]}},
				[2]*Point{{p1[0] + dgx, p1[1] - dgy}, {p2[0], p2[1]}},
			)
		}
	}
	ops := f.renderLines(zigzagLines, o)
	return OpSet{Type: SetFillSketch, Ops: ops}
}

// zigZagLineFiller is the ZigZagLineFiller class.
type zigZagLineFiller struct{}

func (f zigZagLineFiller) fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	gap := o.hachureGap
	if o.hachureGap < 0 {
		gap = float64(o.strokeWidthNum() * 4)
	}
	zo := o.zigzagOffset
	if o.zigzagOffset < 0 {
		zo = gap
	}
	o = o.clone()
	o.hachureGap = gap + zo
	lines := polygonHachureLines(polygonList, o)
	return OpSet{Type: SetFillSketch, Ops: f.zigzagLines(lines, zo, o)}
}

func (zigZagLineFiller) zigzagLines(lines [][2]*Point, zo float64, o *ResolvedOptions) []Op {
	m := &o.gen.m
	pi := math.Pi
	ops := []Op{}
	for _, line := range lines {
		length := lineLength(m, line[0], line[1])
		count := mathRound(length / (2 * zo))
		p1 := line[0]
		p2 := line[1]
		if p1[0] > p2[0] {
			p1 = line[1]
			p2 = line[0]
		}
		alpha := m.atan((p2[1] - p1[1]) / (p2[0] - p1[0]))
		if math.IsInf(count, 1) {
			hang("the zigzag fill has a line with an infinite number of segments")
		}
		o.gen.need(count)
		for i := 0.0; i < count; i++ {
			o.gen.step()
			lstart := float64(i * 2 * zo)
			lend := float64((i + 1) * 2 * zo)
			dz := math.Sqrt(2 * m.pow(zo, 2))
			start := Point{p1[0] + float64(lstart*m.cos(alpha)), p1[1] + float64(lstart*m.sin(alpha))}
			end := Point{p1[0] + float64(lend*m.cos(alpha)), p1[1] + float64(lend*m.sin(alpha))}
			middle := Point{start[0] + float64(dz*m.cos(alpha+pi/4)), start[1] + float64(dz*m.sin(alpha+pi/4))}
			ops = append(ops, doubleLineFillOps(start[0], start[1], middle[0], middle[1], o)...)
			ops = append(ops, doubleLineFillOps(middle[0], middle[1], end[0], end[1], o)...)
		}
	}
	return ops
}

// dashedFiller is the DashedFiller class.
type dashedFiller struct{}

func (f dashedFiller) fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	lines := polygonHachureLines(polygonList, o)
	return OpSet{Type: SetFillSketch, Ops: f.dashedLine(lines, o)}
}

func (dashedFiller) dashedLine(lines [][2]*Point, o *ResolvedOptions) []Op {
	m := &o.gen.m
	// The fallback for a negative dashOffset or dashGap.
	hachureGap := o.hachureGap
	if o.hachureGap < 0 {
		hachureGap = float64(o.strokeWidthNum() * 4)
	}
	offset := o.dashOffset
	if o.dashOffset < 0 {
		offset = hachureGap
	}
	gap := o.dashGap
	if o.dashGap < 0 {
		gap = hachureGap
	}
	ops := []Op{}
	for _, line := range lines {
		length := lineLength(m, line[0], line[1])
		count := math.Floor(length / (offset + gap))
		startOffset := (length + gap - float64(count*(offset+gap))) / 2
		p1 := line[0]
		p2 := line[1]
		if p1[0] > p2[0] {
			p1 = line[1]
			p2 = line[0]
		}
		alpha := m.atan((p2[1] - p1[1]) / (p2[0] - p1[0]))
		if math.IsInf(count, 1) {
			hang("the dashed fill has a line with an infinite number of dashes")
		}
		o.gen.need(count)
		for i := 0.0; i < count; i++ {
			o.gen.step()
			lstart := float64(i * (offset + gap))
			lend := lstart + offset
			start := Point{
				p1[0] + float64(lstart*m.cos(alpha)) + float64(startOffset*m.cos(alpha)),
				p1[1] + float64(lstart*m.sin(alpha)) + float64(startOffset*m.sin(alpha)),
			}
			end := Point{
				p1[0] + float64(lend*m.cos(alpha)) + float64(startOffset*m.cos(alpha)),
				p1[1] + float64(lend*m.sin(alpha)) + float64(startOffset*m.sin(alpha)),
			}
			ops = append(ops, doubleLineFillOps(start[0], start[1], end[0], end[1], o)...)
		}
	}
	return ops
}

// dotFiller is the DotFiller class.
type dotFiller struct{}

func (f dotFiller) fillPolygons(polygonList [][]*Point, o *ResolvedOptions) OpSet {
	o = o.clone()
	o.hachureAngle = 0
	lines := polygonHachureLines(polygonList, o)
	return f.dotsOnLines(lines, o)
}

func (dotFiller) dotsOnLines(lines [][2]*Point, o *ResolvedOptions) OpSet {
	m := &o.gen.m
	ops := []Op{}
	gap := o.hachureGap
	if gap < 0 {
		gap = float64(o.strokeWidthNum() * 4)
	}
	gap = js.Max(gap, 0.1)
	fweight := o.fillWeightNum()
	if fweight < 0 {
		fweight = float64(o.strokeWidthNum() / 2)
	}
	ro := float64(gap / 4)
	for _, line := range lines {
		length := lineLength(m, line[0], line[1])
		dl := length / gap
		count := math.Ceil(dl) - 1
		offset := length - float64(count*gap)
		x := float64((line[0][0]+line[1][0])/2) - float64(gap/4)
		minY := js.Min(line[0][1], line[1][1])
		if math.IsInf(count, 1) {
			hang("the dots fill has a line with an infinite number of dots")
		}
		o.gen.need(count)
		for i := 0.0; i < count; i++ {
			o.gen.step()
			y := minY + offset + float64(i*gap)
			cx := (x - ro) + float64(o.gen.mathRandom()*2*ro)
			cy := (y - ro) + float64(o.gen.mathRandom()*2*ro)
			el := ellipse(cx, cy, fweight, fweight, o)
			ops = append(ops, el.Ops...)
		}
	}
	return OpSet{Type: SetFillSketch, Ops: ops}
}
