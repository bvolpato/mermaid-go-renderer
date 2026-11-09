// Ported from Chromium 156.0.8078.4 (Blink), in part: the layout of text.
// The functions of this file follow the parts of Blink that their comments
// name:
//
//   - LayoutSVGInlineText::ComputeNewScaledFontForStyle:
//     core/layout/svg/layout_svg_inline_text.cc (LGPL-2.0-or-later).
//   - SVGLength: core/svg/svg_length.cc (LGPL-2.0-or-later).
//   - FontMetrics::FixedAscent and FixedDescent:
//     platform/fonts/font_metrics.h (LGPL-2.0-or-later).
//   - ShapeResult::ComputeInkBounds and ComputeRunInkBounds:
//     platform/fonts/shaping/shape_result.cc (BSD-3-Clause, with the
//     copyright of Google and BlackBerry in the file).
//   - FragmentItem::ObjectBoundingBox: core/layout/inline/fragment_item.cc
//     (BSD-3-Clause).
//   - SvgTextLayoutAlgorithm: core/layout/svg/svg_text_layout_algorithm.cc
//     (BSD-3-Clause).
//   - The fast path of the CSS parser for a length:
//     core/css/parser/css_parser_fast_paths.cc (BSD-3-Clause).
//   - LayoutUnit, TextRunLayoutUnit and InlineLayoutUnit:
//     platform/geometry/layout_unit.h (BSD-3-Clause, with the copyright of
//     Google in the file).
//
// The paths are in third_party/blink/renderer. The license of each file is
// the license in its header at that version. See NOTICE in this directory.

package browser

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

// This file lays out text. It follows LayoutNG of Blink: text nodes become
// inline items, consecutive items with the same font are shaped together,
// and the width of each item is snapped up to a LayoutUnit (1/64 px).

// lu is a LayoutUnit: a length in 1/64 px.
type lu int64

const luPerPx = 64

func luFromFloatCeil(v float64) lu  { return lu(math.Ceil(float64(float32(v) * luPerPx))) }
func luFromFloatRound(v float64) lu { return lu(math.Round(float64(float32(v) * luPerPx))) }
func luFromFloatFloor(v float64) lu { return lu(math.Floor(float64(float32(v) * luPerPx))) }
func luFromPx(v float64) lu         { return lu(math.Trunc(v * luPerPx)) }
func (v lu) px() float64            { return float64(v) / luPerPx }
func (v lu) f32() float32           { return float32(float32(v) / luPerPx) }

// inlineItem is the text of one text node, with the style of its parent.
type inlineItem struct {
	node  *dom.Node // the text node
	owner *dom.Node // the element whose style applies
	style *textStyle
	runes []rune // after white-space processing

	// SVG positioning that applies to the first character of the item.
	hasX, hasY bool
	x, y       float32
	dx, dy     float32
	newChunk   bool

	// Shaping results.
	glyphs  []itemGlyph
	width   lu // snapped inline size
	metrics font.Metrics
	forced  bool // a forced line break follows the item (HTML)

	// mark is set for an HTML item that has no characters and stands for a
	// place in a line: a br element, an inline element with no content, or
	// an edge of an inline box. See htmlinline.go.
	mark *inlineMark
	// shapeBoundary is true for an HTML item that is not shaped together
	// with the item before it. See collectInline.
	shapeBoundary bool
}

// itemGlyph is a glyph of an item with the face that draws it.
type itemGlyph struct {
	face    *font.Face
	glyph   font.Glyph
	runeIdx int // index in item.runes of the cluster start
}

func isCollapsibleSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}

func collapses(ws string) bool {
	switch ws {
	case "pre", "pre-wrap", "break-spaces":
		return false
	}
	return true
}

// collapseState carries white-space collapsing across items.
type collapseState struct {
	atStart   bool
	prevSpace bool
	lastSpace *inlineItem // the item that holds the last emitted collapsible space
}

// processWhiteSpace applies the white-space rules of CSS Text to raw text.
// keepNewlines is true for HTML with white-space values that preserve line
// breaks; the caller then splits on '\n'.
func processWhiteSpace(raw string, ws string, st *collapseState, svg bool) []rune {
	out := make([]rune, 0, len(raw))
	if !collapses(ws) {
		for _, r := range raw {
			if r == '\r' {
				continue
			}
			if svg && (r == '\n' || r == '\t') {
				r = ' '
			}
			out = append(out, r)
		}
		if len(out) > 0 {
			st.atStart, st.prevSpace, st.lastSpace = false, false, nil
		}
		return out
	}
	for _, r := range raw {
		if isCollapsibleSpace(r) {
			if ws == "pre-line" && r == '\n' {
				// A preserved line break removes the spaces around it.
				for len(out) > 0 && out[len(out)-1] == ' ' {
					out = out[:len(out)-1]
				}
				out = append(out, '\n')
				st.atStart, st.prevSpace = true, false
				continue
			}
			if st.atStart || st.prevSpace {
				continue
			}
			out = append(out, ' ')
			st.prevSpace = true
			continue
		}
		out = append(out, r)
		st.atStart, st.prevSpace = false, false
	}
	return out
}

// applyTextTransform applies text-transform before shaping.
func applyTextTransform(runes []rune, transform string) []rune {
	switch transform {
	case "uppercase":
		return []rune(strings.ToUpper(string(runes)))
	case "lowercase":
		return []rune(strings.ToLower(string(runes)))
	case "capitalize":
		out := append([]rune(nil), runes...)
		start := true
		for i, r := range out {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				if start {
					out[i] = unicode.ToUpper(r)
				}
				start = false
			} else {
				start = true
			}
		}
		return out
	}
	return runes
}

func sameFont(a, b *textStyle) bool {
	if a == b {
		return true
	}
	if a.size != b.size || a.weight != b.weight || a.italic != b.italic || a.letterSpacing != b.letterSpacing || len(a.families) != len(b.families) {
		return false
	}
	for i := range a.families {
		if a.families[i] != b.families[i] {
			return false
		}
	}
	return true
}

// shapeItems shapes a sequence of items. Neighbors with the same font are
// shaped as one string, so that kerning works across element boundaries, and
// the glyphs are then split back by item. boundary reports whether shaping
// must not cross between item i-1 and item i.
func (e *Engine) shapeItems(items []*inlineItem, boundary func(i int) bool) {
	for start := 0; start < len(items); {
		end := start + 1
		for end < len(items) && sameFont(items[start].style, items[end].style) && !(boundary != nil && boundary(end)) {
			end++
		}
		group := items[start:end]
		st := group[0].style
		chain := e.chainOf(st)
		total := 0
		for _, it := range group {
			total += len(it.runes)
		}
		text := make([]rune, 0, total)
		offsets := make([]int, len(group)+1)
		for i, it := range group {
			offsets[i] = len(text)
			text = append(text, it.runes...)
		}
		offsets[len(group)] = len(text)
		runs := chain.ShapeText(text, st.fontSize())
		primary := chain.Primary()
		for i, it := range group {
			it.glyphs = it.glyphs[:0]
			if primary != nil {
				it.metrics = primary.Metrics(st.fontSize())
			}
			lo, hi := offsets[i], offsets[i+1]
			// Blink adds the advances in fixed point (TextRunLayoutUnit),
			// so the sum is exact. An advance is a multiple of 1/65536 px,
			// and a float64 sum of such values is exact too. A float32
			// running sum can fall below a multiple of 1/64 px that the
			// exact sum passes, and the width is then 1/64 px smaller.
			var w float64
			for _, run := range runs {
				for _, g := range run.Glyphs {
					idx := run.Start + g.Cluster
					if idx < lo || idx >= hi {
						continue
					}
					if st.letterSpacing != 0 {
						g.Advance += st.letterSpacing
					}
					it.glyphs = append(it.glyphs, itemGlyph{face: run.Face, glyph: g, runeIdx: idx - lo})
					w += g.Advance
				}
			}
			it.width = luFromFloatCeil(float64(float32(w)))
			if it.width < 0 {
				it.width = 0
			}
		}
		start = end
	}
}

// glyphRangeWidth is the sum of the advances of the glyphs of an item whose
// cluster starts in [lo, hi). The sum is exact, as in shapeItems, and the
// result is its float32.
func (it *inlineItem) glyphRangeWidth(lo, hi int) float32 {
	var w float64
	for i := range it.glyphs {
		if idx := it.glyphs[i].runeIdx; idx >= lo && idx < hi {
			w += it.glyphs[i].glyph.Advance
		}
	}
	return float32(w)
}

// ---- glyph ink bounds ----

// inkBounds is ShapeResult::ComputeInkBounds for the glyphs of an item: the
// union of the glyph bounds, with x relative to the start of the item and y
// relative to the baseline (y down).
//
// The rectangle of a glyph is rectFromBounds of the bounds of its outline
// (gfx::SkRectToRectF). The rectangles of the glyphs are united edge by
// edge: the left and the top are the smallest, the right and the bottom are
// the largest sums of an origin and a size, and the size of the result is
// the difference of its edges. The last bit of the result depends on these
// steps when the font size is not a multiple of a power of two.
// testdata/svgscale.json has the values of Chromium.
func (e *Engine) inkBounds(it *inlineItem) (rectF, bool) {
	var l, t, r, b float32
	have := false
	// Blink adds the advances in fixed point (InlineLayoutUnit in
	// ShapeResult::ComputeRunInkBounds), so the origin of a glyph is the
	// float32 of the exact sum. An advance is a multiple of 1/65536 px, and
	// the float64 sum of such values is exact.
	var originSum float64
	size := it.style.fontSize()
	for i := range it.glyphs {
		g := &it.glyphs[i]
		origin := float32(originSum)
		if left, top, right, bottom, empty, handled := g.face.ScalerBounds(g.glyph.ID, size); handled {
			// On Linux the font rasterizer gives the bounds of a glyph:
			// whole pixels around the hinted outline.
			if glyph := rectFromBounds(left, top, right, bottom); !empty && !glyph.isEmpty() {
				glyph.x += origin
				glyph.x += float32(g.glyph.XOffset)
				glyph.y -= float32(g.glyph.YOffset)
				if !have {
					l, t, r, b, have = glyph.x, glyph.y, glyph.right(), glyph.bottom(), true
				} else {
					l, t = min(l, glyph.x), min(t, glyph.y)
					r, b = max(r, glyph.right()), max(b, glyph.bottom())
				}
			}
			originSum += g.glyph.Advance
			continue
		}
		if left, top, right, bottom, ok := g.face.ColorGlyphBounds(g.glyph.ID, size, it.style.italic && !g.face.Italic); ok {
			// A glyph of a color bitmap font has the bounds of its image.
			glyph := rectFromBounds(left, top, right, bottom)
			if !glyph.isEmpty() {
				glyph.x += origin
				glyph.x += float32(g.glyph.XOffset)
				glyph.y -= float32(g.glyph.YOffset)
				if !have {
					l, t, r, b, have = glyph.x, glyph.y, glyph.right(), glyph.bottom(), true
				} else {
					l, t = min(l, glyph.x), min(t, glyph.y)
					r, b = max(r, glyph.right()), max(b, glyph.bottom())
				}
			}
			originSum += g.glyph.Advance
			continue
		}
		minX, minY, maxX, maxY, ok := g.face.OutlineBounds(g.glyph.ID)
		if ok {
			scale := size / g.face.UnitsPerEm()
			left := float32(float64(minX) * scale)
			right := float32(float64(maxX) * scale)
			top := float32(-float64(maxY) * scale)
			bottom := float32(-float64(minY) * scale)
			glyph := rectFromBounds(left, top, right, bottom)
			if !glyph.isEmpty() {
				glyph.x += origin
				glyph.x += float32(g.glyph.XOffset)
				glyph.y -= float32(g.glyph.YOffset)
				if !have {
					l, t, r, b, have = glyph.x, glyph.y, glyph.right(), glyph.bottom(), true
				} else {
					l, t = min(l, glyph.x), min(t, glyph.y)
					r, b = max(r, glyph.right()), max(b, glyph.bottom())
				}
			}
		}
		originSum += g.glyph.Advance
	}
	if !have {
		return rectF{}, false
	}
	return rectF{l, t, r - l, b - t}, true
}

// ---- SVG text ----

type textFragment struct {
	item     *inlineItem
	x        float32 // start of the fragment
	baseline float32 // y of the specified text position
	width    float32
	rect     rectF // logical rectangle
	bbox     rectF // logical rectangle united with the ink bounds
	glyphY   float32
}

type svgTextLayout struct {
	frags []textFragment
	// scale is the factor between the font size of the text and the font
	// size on the screen (screenFontScale). The width of a fragment is in
	// the scaled space.
	scale float32
}

func isSVGTextContainer(n *dom.Node) bool {
	if n.Type != dom.ElementNode || n.NS != dom.NSSVG {
		return false
	}
	switch n.Tag {
	case "tspan", "a", "textPath", "tref", "altGlyph":
		return true
	}
	return false
}

// firstLength parses the first value of an SVG length list attribute.
func (e *Engine) firstLength(n *dom.Node, name string, emSize float64) (float32, bool) {
	v, ok := n.GetAttr(name)
	if !ok {
		return 0, false
	}
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, ", \t\n"); i >= 0 {
		v = v[:i]
	}
	if v == "" {
		return 0, false
	}
	if f, ok := exValue(v); ok {
		// The ex of an attribute is the x-height of the font of the
		// element. SVGLength multiplies in float32.
		return float32(float32(f) * float32(e.exSize(e.textStyleOf(n)))), true
	}
	if percent, ok := percentValue(v); ok {
		// A percentage refers to the viewport of the nearest svg element.
		return clampCSSLength(e.svgPercentLength(n, name, percent)), true
	}
	px, ok := e.svgLength(v, emSize, 0)
	// A length is a CSS length in Blink, which has limits.
	return clampCSSLength(px), ok
}

// svgLength converts an SVG length attribute value to user units, as
// SVGLength does. Blink keeps the number as a float32 and multiplies it by
// the unit factor in float32. percentBase is the reference for a percentage.
func (e *Engine) svgLength(v string, emSize float64, percentBase float64) (float32, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return 0, false
	}
	end := len(v)
	for end > 0 && (v[end-1] < '0' || v[end-1] > '9') && v[end-1] != '.' {
		end--
	}
	num, err := strconv.ParseFloat(v[:end], 64)
	if err != nil {
		return 0, false
	}
	n := float32(num)
	var factor float32
	switch v[end:] {
	case "", "px":
		if f, ok := fastPathLength(v[:end]); ok {
			return float32(f), true
		}
		return n, true
	case "%":
		return float32(n * float32(percentBase) / 100), true
	case "em":
		factor = float32(emSize)
	case "rem":
		factor = 16
	case "ex", "ch":
		factor = float32(emSize) / 2
	case "pt":
		factor = 96.0 / 72
	case "pc":
		factor = 16
	case "in":
		factor = 96
	case "cm":
		factor = 96 / 2.54
	case "mm":
		factor = 96 / 25.4
	case "q":
		factor = 96 / 101.6
	default:
		return 0, false
	}
	return float32(n * factor), true
}

// fastPathLength reads the number of a length attribute as the fast path of
// the CSS parser of Blink does for a simple length in px or with no unit:
// an optional minus sign, digits and an optional fraction. The fast path
// keeps the first 7 digits of the fraction and drops the rest. The result
// is the double `whole + fraction / scale`. A number in another form (a plus
// sign, an exponent) does not take the fast path: the second result is
// false, and the caller rounds the whole number.
//
// Blink also leaves the fast path for a value that has white space after
// the number. The callers trim the value, so that such a value takes the
// fast path here.
func fastPathLength(s string) (float64, bool) {
	negative := false
	if s != "" && s[0] == '-' {
		negative = true
		s = s[1:]
	}
	i := 0
	whole := 0.0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		whole = float64(whole*10) + float64(s[i]-'0')
		i++
	}
	digits := i
	fraction, scale := 0.0, 1.0
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			if scale < 10000000 {
				fraction = float64(fraction*10) + float64(s[i]-'0')
				scale *= 10
			}
			i++
			digits++
		}
	}
	if i != len(s) || digits == 0 {
		return 0, false
	}
	value := whole + fraction/scale
	if negative {
		value = -value
	}
	return value, true
}

func parseCSSNumber(s string) (float64, bool) {
	px, ok := resolveLength(s, 0, true)
	return px, ok
}

// layoutSVGText lays out a <text> element.
func (e *Engine) layoutSVGText(text *dom.Node) *svgTextLayout {
	st := e.styleOf(text)
	if st.layoutOK {
		if l, ok := st.layout.(*svgTextLayout); ok {
			return l
		}
	}
	var items []*inlineItem
	cs := &collapseState{atStart: true}

	// Blink lays out SVG text with the font that has the size of the text
	// on the screen, and maps the boxes back to the user space. scale is the
	// factor between the two spaces. It is 1 when no ancestor scales.
	scale := e.screenFontScale(text)
	scaledStyles := map[*textStyle]*textStyle{}
	itemStyle := func(ts *textStyle) *textStyle {
		if scale == 1 {
			return ts
		}
		scaled, ok := scaledStyles[ts]
		if !ok {
			// LayoutSVGInlineText::ComputeNewScaledFontForStyle.
			c := *ts
			c.size = float64(float32(ts.size) * scale)
			c.letterSpacing = float64(float32(ts.letterSpacing) * scale)
			scaled = &c
			scaledStyles[ts] = scaled
		}
		return scaled
	}

	type pending struct {
		hasX, hasY, hasDX, hasDY bool
		x, y, dx, dy             float32
	}
	var walk func(n *dom.Node, pos pending)
	walk = func(n *dom.Node, pos pending) {
		ts := e.textStyleOf(n)
		if x, ok := e.firstLength(n, "x", ts.size); ok {
			pos.hasX, pos.x = true, x
		}
		if y, ok := e.firstLength(n, "y", ts.size); ok {
			pos.hasY, pos.y = true, y
		}
		if dx, ok := e.firstLength(n, "dx", ts.size); ok {
			pos.hasDX, pos.dx = true, dx
		}
		if dy, ok := e.firstLength(n, "dy", ts.size); ok {
			pos.hasDY, pos.dy = true, dy
		}
		for c := n.FirstChild; c != nil; c = c.Next {
			switch c.Type {
			case dom.TextNode:
				runes := processWhiteSpace(c.Data, ts.whiteSpace, cs, true)
				if len(runes) == 0 {
					continue
				}
				runes = applyTextTransform(runes, ts.textTransform)
				it := &inlineItem{node: c, owner: n, style: itemStyle(ts), runes: runes}
				if pos.hasX {
					it.hasX, it.x = true, pos.x
				}
				if pos.hasY {
					it.hasY, it.y = true, pos.y
				}
				if pos.hasDX {
					it.dx = pos.dx
				}
				if pos.hasDY {
					it.dy = pos.dy
				}
				it.newChunk = pos.hasX || pos.hasY
				pos = pending{}
				if cs.prevSpace {
					cs.lastSpace = it
				}
				items = append(items, it)
			case dom.ElementNode:
				if !isSVGTextContainer(c) || e.display(c) == "none" {
					continue
				}
				walk(c, pos)
				// A position given by an ancestor applies to the first
				// character only. If the child had characters, it is used.
				if hasItems(items, c) {
					pos = pending{}
				}
			}
		}
	}
	walk(text, pending{})

	// Trailing collapsible white space of the whole text is removed.
	if cs.prevSpace && cs.lastSpace != nil {
		it := cs.lastSpace
		if n := len(it.runes); n > 0 && it.runes[n-1] == ' ' {
			it.runes = it.runes[:n-1]
		}
	}
	kept := items[:0]
	for i, it := range items {
		if len(it.runes) == 0 {
			// The position moves to the next item with characters.
			if i+1 < len(items) {
				next := items[i+1]
				if it.hasX && !next.hasX {
					next.hasX, next.x = true, it.x
				}
				if it.hasY && !next.hasY {
					next.hasY, next.y = true, it.y
				}
				next.dx += it.dx
				next.dy += it.dy
				next.newChunk = next.newChunk || it.newChunk
			}
			continue
		}
		kept = append(kept, it)
	}
	items = kept

	e.shapeItems(items, func(i int) bool { return items[i].newChunk })

	layout := &svgTextLayout{frags: make([]textFragment, len(items)), scale: scale}
	var curX, curY float32
	chunkStart := 0
	finishChunk := func(end int) {
		if end <= chunkStart {
			return
		}
		anchor := layout.frags[chunkStart].item.style.textAnchor
		if anchor != "middle" && anchor != "end" {
			return
		}
		first := layout.frags[chunkStart]
		lo, hi := first.x, first.x+first.width
		for i := chunkStart + 1; i < end; i++ {
			f := layout.frags[i]
			lo, hi = min(lo, f.x), max(hi, f.x+f.width)
		}
		var shift float32
		if anchor == "middle" {
			shift = first.x - float32((lo+hi)/2)
		} else {
			shift = first.x - hi
		}
		for i := chunkStart; i < end; i++ {
			layout.frags[i].x += shift
		}
	}
	// For scaled text: SvgTextLayoutAlgorithm in the scaled space, with the
	// steps "Adjust positions: dx, dy" and "Adjust positions: x, y". cssX is
	// the position of an item in its line box (CSS_positions): the sum of
	// the widths before it. The position of the line in the block direction
	// counts as 0. The order of the float32 operations is the one that
	// gives the values of Chromium in testdata/svgscale.json.
	var inline lu
	var dxShift, dyShift, xShift, yShift float32
	for i, it := range items {
		if it.newChunk {
			finishChunk(i)
			chunkStart = i
		}
		if scale != 1 {
			cssX := inline.f32()
			if it.hasX {
				dxShift = 0
			}
			if it.hasY {
				dyShift = 0
			}
			dxShift += float32(it.dx * scale)
			dyShift += float32(it.dy * scale)
			x := cssX + dxShift
			y := dyShift
			if it.hasX {
				xShift = float32(it.x*scale) - cssX
			}
			if it.hasY {
				yShift = float32(it.y * scale)
			}
			x += xShift
			y += yShift
			layout.frags[i] = textFragment{item: it, x: x, baseline: y, width: it.width.f32()}
			inline += it.width
			continue
		}
		if it.hasX {
			curX = it.x
		}
		if it.hasY {
			curY = it.y
		}
		curX += it.dx
		curY += it.dy
		layout.frags[i] = textFragment{item: it, x: curX, baseline: curY, width: it.width.f32()}
		curX += it.width.f32()
	}
	finishChunk(len(items))

	for i := range layout.frags {
		f := &layout.frags[i]
		it := f.item
		m := it.metrics
		ascent := float32(m.Ascent)
		baselineAscent, height := e.baselineBox(it.owner, m)
		f.rect = rectF{f.x, f.baseline - baselineAscent, f.width, height}
		f.glyphY = f.rect.y + ascent
		f.bbox = f.rect
		if ink, ok := e.inkBounds(it); ok {
			ink.y += ascent
			ink.x += f.rect.x
			ink.y += f.rect.y
			f.bbox = ink.union(f.rect)
		}
		if scale != 1 {
			// FragmentItem::ObjectBoundingBox: back to the user space.
			inv := 1 / scale
			f.bbox = rectF{float32(f.bbox.x * inv), float32(f.bbox.y * inv), float32(f.bbox.w * inv), float32(f.bbox.h * inv)}
		}
	}
	st.layout, st.layoutOK = layout, true
	return layout
}

func hasItems(items []*inlineItem, el *dom.Node) bool {
	for i := len(items) - 1; i >= 0; i-- {
		for p := items[i].owner; p != nil; p = p.Parent {
			if p == el {
				return true
			}
		}
	}
	return false
}

// baselineTypes returns the dominant-baseline of owner and the
// alignment-baseline that the element sets, or "" when it sets none. A
// keyword that the property does not have is not a value of it: Blink drops
// such a declaration. auto, use-script, no-change and reset-size are the
// alphabetic baseline here.
func (e *Engine) baselineTypes(owner *dom.Node) (dominant, alignment string) {
	dominant = "alphabetic"
	// dominant-baseline is inherited. The walk is the one of lookup, and it
	// goes past a declaration with a keyword that is not a value.
inherit:
	for cur := owner; cur != nil && cur.Type == dom.ElementNode; cur = cur.Parent {
		v, ok := e.declared(cur, "dominant-baseline")
		if !ok {
			continue
		}
		switch v = strings.ToLower(strings.TrimSpace(v)); v {
		case "central", "middle", "mathematical", "hanging", "text-before-edge", "text-after-edge", "ideographic":
			dominant = v
			break inherit
		case "auto", "alphabetic", "use-script", "no-change", "reset-size":
			break inherit
		}
	}
	if ab, ok := e.declared(owner, "alignment-baseline"); ok {
		switch ab = strings.ToLower(strings.TrimSpace(ab)); ab {
		case "alphabetic", "central", "middle", "mathematical", "hanging",
			"text-before-edge", "text-after-edge", "ideographic":
			alignment = ab
		case "before-edge":
			alignment = "text-before-edge"
		case "after-edge":
			alignment = "text-after-edge"
		}
	}
	return dominant, alignment
}

// fixedAscentDescent is FontMetrics::FixedAscent(baseline type) and
// FixedDescent(baseline type): the distances from a baseline to the top and
// to the bottom of the text box. Blink rounds each of them to a LayoutUnit.
// The last result is false for the alphabetic baseline.
func fixedAscentDescent(baseline string, m font.Metrics) (ascent, descent lu, ok bool) {
	asc := float32(m.Ascent)
	desc := float32(m.Descent)
	height := float32(m.Ascent + m.Descent)
	var a, d float32
	switch baseline {
	case "central":
		a, d = height/2, height/2
	case "middle":
		half := float32(float32(m.XHeight) / 2)
		a, d = asc-half, desc+half
	case "mathematical":
		a, d = float32(asc*0.5), desc+float32(asc*0.5)
	case "hanging":
		a, d = float32(asc*0.2), desc+float32(asc*0.8)
	case "text-before-edge":
		a, d = 0, height
	case "text-after-edge", "ideographic":
		a, d = height, 0
	default:
		return 0, 0, false
	}
	return luFromFloatRound(float64(a)), luFromFloatRound(float64(d)), true
}

// baselineBox is the vertical extent of the text box of owner around the
// specified y position: the distance from the top of the box to that
// position, and the height of the box.
//
// The dominant-baseline gives the box: FixedAscent above the position and
// FixedDescent below it. The two are rounded separately, so the height can
// differ from ascent + descent by 1/64 px. An alignment-baseline then moves
// the box, so that its own baseline is at the position, and keeps the
// height.
func (e *Engine) baselineBox(owner *dom.Node, m font.Metrics) (ascent, height float32) {
	dominant, alignment := e.baselineTypes(owner)
	ascent = float32(m.Ascent)
	height = float32(m.Ascent + m.Descent)
	if a, d, ok := fixedAscentDescent(dominant, m); ok {
		ascent, height = a.f32(), (a + d).f32()
	}
	if alignment != "" {
		if a, _, ok := fixedAscentDescent(alignment, m); ok {
			ascent = a.f32()
		} else {
			ascent = float32(m.Ascent)
		}
	}
	return ascent, height
}

// textElementOf returns the <text> ancestor of an SVG text content element.
func textElementOf(n *dom.Node) *dom.Node {
	for p := n; p != nil; p = p.Parent {
		if p.Type == dom.ElementNode && p.NS == dom.NSSVG && p.Tag == "text" {
			return p
		}
	}
	return nil
}

func isDescendantOf(n, anc *dom.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == anc {
			return true
		}
	}
	return false
}

// svgTextBBox is the object bounding box of a text content element.
func (e *Engine) svgTextBBox(n *dom.Node) (rectF, bool) {
	text := textElementOf(n)
	if text == nil {
		return rectF{}, false
	}
	layout := e.layoutSVGText(text)
	var bbox rectF
	have := false
	for i := range layout.frags {
		f := &layout.frags[i]
		if n != text && !isDescendantOf(f.item.owner, n) {
			continue
		}
		if have {
			bbox = bbox.union(f.bbox)
		} else {
			bbox, have = f.bbox, true
		}
	}
	return bbox, have
}

// svgTextLength is getComputedTextLength().
func (e *Engine) svgTextLength(n *dom.Node) float64 {
	text := textElementOf(n)
	if text == nil {
		return 0
	}
	layout := e.layoutSVGText(text)
	var total float32
	for i := range layout.frags {
		f := &layout.frags[i]
		if n != text && !isDescendantOf(f.item.owner, n) {
			continue
		}
		total += f.width
	}
	if layout.scale != 1 {
		total /= layout.scale
	}
	return float64(total)
}
