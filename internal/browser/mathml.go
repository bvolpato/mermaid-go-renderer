// Ported in part from Chromium 156.0.8078.4, core/layout/mathml/
// math_{token,row,fraction,scripts,radical,under_over}_layout_algorithm.cc
// and core/css/mathml.css (BSD-3-Clause). See NOTICE in this directory.
// Table layout and unsupported MathML use a reduced form of these rules.
package browser

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
)

type mathBoxData struct {
	ascent    lu
	synthetic bool
	runs      []RasterTextFragment
	rules     []RasterMathRule
}

func isMathRoot(n *dom.Node) bool { return n != nil && n.NS == dom.NSMathML && n.Tag == "math" }

func mathHidden(n *dom.Node) bool {
	if n == nil {
		return false
	}
	// Legacy KaTeX contains both branches. Native math is the painted
	// branch here, so its HTML sibling must not contribute a second label.
	if n.NS == dom.NSHTML && n.HasClass("katex-html") && n.Parent != nil && n.Parent.HasClass("katex") {
		for c := n.Parent.FirstChild; c != nil; c = c.Next {
			if c != n && c.Type == dom.ElementNode && containsMath(c) {
				return true
			}
		}
	}
	if n.NS == dom.NSMathML {
		if n.Tag == "annotation" || n.Tag == "annotation-xml" {
			return true
		}
		if p := n.Parent; p != nil && p.NS == dom.NSMathML && (p.Tag == "semantics" || p.Tag == "maction") {
			for c := p.FirstChild; c != nil; c = c.Next {
				if c.Type == dom.ElementNode {
					return c != n
				}
			}
		}
	}
	return false
}

func mathChildIndex(n *dom.Node) int {
	i := 0
	if n.Parent != nil {
		for c := n.Parent.FirstChild; c != nil; c = c.Next {
			if c == n {
				return i
			}
			if c.Type == dom.ElementNode {
				i++
			}
		}
	}
	return i
}

func mathDisplayStyle(n *dom.Node) bool {
	for p := n; p != nil && p.NS == dom.NSMathML; p = p.Parent {
		if v, ok := p.GetAttr("displaystyle"); ok {
			return strings.EqualFold(v, "true")
		}
		if p.Tag == "math" {
			return strings.EqualFold(p.AttrOr("display", ""), "block")
		}
		if p.Tag == "mtable" {
			return false
		}
		if p.Parent != nil && p.Parent.NS == dom.NSMathML {
			switch p.Parent.Tag {
			case "mfrac", "msub", "msup", "msubsup", "munder", "mover", "munderover", "mroot":
				if p.Parent.Tag == "mfrac" || mathChildIndex(p) > 0 {
					return false
				}
			}
		}
	}
	return false
}

func (e *Engine) mathStyleDefaults(n *dom.Node, ts, parent *textStyle) {
	if n.Tag == "math" {
		ts.families = []string{e.Fonts.MathFamily()}
		ts.weight, ts.italic = 400, false
		ts.lhKind, ts.letterSpacing, ts.whiteSpace = lhNormal, 0, "nowrap"
		return
	}
	if n.Tag == "mphantom" {
		ts.visibility = "hidden"
	}
	p := n.Parent
	if p == nil || p.NS != dom.NSMathML {
		return
	}
	index := 0
	levels := 0
	switch p.Tag {
	case "msub", "msup", "msubsup", "munder", "mover", "munderover", "mmultiscripts":
		index = mathChildIndex(n)
		if index > 0 {
			levels = 1
		}
	case "mroot":
		index = mathChildIndex(n)
		if index > 0 {
			levels = 2
		}
	case "mfrac":
		if !mathDisplayStyle(p) {
			levels = 1
		}
	}
	if (index == 1 && p.AttrOr("accentunder", "") == "true") || (index > 0 && p.AttrOr("accent", "") == "true") {
		levels = 0
	}
	for range levels {
		ts.size = float64(float32(ts.size * 0.71))
	}
	if v, ok := n.GetAttr("mathsize"); ok {
		if size, ok := resolveFontSize(v, parent.size, false); ok {
			ts.size = size
		}
	}
}

func mathChildren(n *dom.Node) []*dom.Node {
	var out []*dom.Node
	for c := n.FirstChild; c != nil; c = c.Next {
		if c.Type == dom.ElementNode && !mathHidden(c) {
			out = append(out, c)
		}
	}
	return out
}

func (e *Engine) mathConstant(ts *textStyle, name string, fallback float64) lu {
	if f := e.chainOf(ts).Primary(); f != nil {
		if v, ok := f.MathConstant(name, ts.fontSize()); ok {
			return luFromPx(v)
		}
	}
	return luFromPx(fallback)
}

func (e *Engine) mathXHeight(ts *textStyle) float64 {
	if f := e.chainOf(ts).Primary(); f != nil {
		return f.Metrics(ts.fontSize()).XHeight
	}
	return ts.size / 2
}

func (e *Engine) mathRule(ts *textStyle) lu {
	fallback := ts.size / 20
	if f := e.chainOf(ts).Primary(); f != nil {
		fallback = f.UnderlineThickness(ts.fontSize())
	}
	return e.mathConstant(ts, "FractionRuleThickness", fallback)
}

func newMathBox(n *dom.Node, ts *textStyle) *htmlBox {
	return &htmlBox{node: n, style: ts, math: &mathBoxData{}}
}
func mathDescent(b *htmlBox) lu              { return b.h - b.math.ascent }
func setMathMetrics(b *htmlBox, w, a, d lu)  { b.w = max(0, w); b.h = max(0, a+d); b.math.ascent = a }
func appendMathChild(b, c *htmlBox, x, y lu) { c.x, c.y = x, y; b.children = append(b.children, c) }

func (e *Engine) layoutMath(n *dom.Node) *htmlBox {
	return e.layoutMathDepth(n, 0)
}

func (e *Engine) layoutMathDepth(n *dom.Node, depth int) *htmlBox {
	ts := e.textStyleOf(n)
	b := newMathBox(n, ts)
	if depth > 256 || e.display(n) == "none" {
		return b
	}
	kids := mathChildren(n)
	child := func(i int) *htmlBox { return e.layoutMathDepth(kids[i], depth+1) }
	switch n.Tag {
	case "mi", "mn", "mo", "mtext", "ms":
		e.mathToken(b, n.TextContent())
	case "mspace":
		w, _ := e.mathLength(n.AttrOr("width", "0"), ts, 0)
		a, _ := e.mathLength(n.AttrOr("height", "0"), ts, 0)
		d, _ := e.mathLength(n.AttrOr("depth", "0"), ts, 0)
		setMathMetrics(b, w, max(0, a), max(0, d))
	case "mfrac":
		if len(kids) == 2 {
			e.mathFraction(b, child(0), child(1))
			break
		}
		e.mathRow(b, kids, depth)
	case "msub", "msup", "msubsup":
		want := 2
		if n.Tag == "msubsup" {
			want = 3
		}
		if len(kids) != want {
			e.mathRow(b, kids, depth)
			break
		}
		base := child(0)
		var sub, sup *htmlBox
		if n.Tag == "msup" {
			sup = child(1)
		} else {
			sub = child(1)
		}
		if want == 3 {
			sup = child(2)
		}
		e.mathScripts(b, base, sub, sup)
	case "munder", "mover", "munderover":
		want := 2
		if n.Tag == "munderover" {
			want = 3
		}
		if len(kids) != want {
			e.mathRow(b, kids, depth)
			break
		}
		base := child(0)
		var under, over *htmlBox
		if n.Tag == "mover" {
			over = child(1)
		} else {
			under = child(1)
		}
		if want == 3 {
			over = child(2)
		}
		e.mathUnderOver(b, base, under, over)
	case "msqrt", "mroot":
		if n.Tag == "mroot" && len(kids) != 2 {
			e.mathRow(b, kids, depth)
			break
		}
		base := newMathBox(n, ts)
		base.math.synthetic = true
		var index *htmlBox
		if n.Tag == "mroot" {
			base = child(0)
			index = child(1)
		} else {
			e.mathRow(base, kids, depth)
		}
		e.mathRadical(b, base, index)
	case "mtable":
		e.mathTable(b, kids, depth)
	default:
		e.mathRow(b, kids, depth)
		if len(kids) == 0 {
			var text strings.Builder
			for c := n.FirstChild; c != nil; c = c.Next {
				if c.Type == dom.TextNode {
					text.WriteString(c.Data)
				}
			}
			if strings.TrimSpace(text.String()) != "" {
				e.mathToken(b, text.String())
			}
		}
	}
	if n.Tag == "mtd" {
		px := luFromPx(0.4 * ts.size)
		py := luFromPx(e.mathXHeight(ts) / 2)
		b.w += 2 * px
		b.h += 2 * py
		b.math.ascent += py
		for _, c := range b.children {
			c.x += px
			c.y += py
		}
	}
	if n.Tag == "mpadded" {
		for _, prop := range []string{"width", "height", "depth"} {
			v, ok := n.GetAttr(prop)
			if !ok {
				continue
			}
			base := b.w
			if prop == "height" {
				base = b.math.ascent
			}
			if prop == "depth" {
				base = mathDescent(b)
			}
			if value, ok := e.mathLength(v, ts, base); ok {
				if strings.HasPrefix(v, "+") || strings.HasPrefix(v, "-") {
					value += base
				}
				switch prop {
				case "width":
					b.w = max(0, value)
				case "height":
					b.h = max(0, value) + mathDescent(b)
					b.math.ascent = max(0, value)
				case "depth":
					b.h = b.math.ascent + max(0, value)
				}
			}
		}
	}
	return b
}

// mathItalic is ItalicMathVariant for the Latin and Greek ranges used by KaTeX.
func mathItalic(r rune) rune {
	switch {
	case r >= 'A' && r <= 'Z':
		return 0x1d434 + r - 'A'
	case r >= 'a' && r <= 'z':
		if r == 'h' {
			return 0x210e
		}
		return 0x1d44e + r - 'a'
	case r >= 0x391 && r <= 0x3a9 && r != 0x3a2:
		return 0x1d6e2 + r - 0x391
	case r >= 0x3b1 && r <= 0x3c9:
		return 0x1d6fc + r - 0x3b1
	case r == 0x2202:
		return 0x1d715
	case r == 0x2207:
		return 0x1d6fb
	case r == 0x3f5:
		return 0x1d716
	case r == 0x3d1:
		return 0x1d717
	case r == 0x3f0:
		return 0x1d718
	case r == 0x3d5:
		return 0x1d719
	case r == 0x3f1:
		return 0x1d71a
	case r == 0x3d6:
		return 0x1d71b
	}
	return r
}

func (e *Engine) mathToken(b *htmlBox, text string) {
	ts := b.style
	cs := collapseState{atStart: true}
	text = strings.Trim(string(processWhiteSpace(text, "normal", &cs, false)), " \t\r\n")
	inkText := text
	if b.node.Tag == "mi" && b.node.AttrOr("mathvariant", "") != "normal" && utf8.RuneCountInString(text) == 1 {
		for _, r := range text {
			text = string(mathItalic(r))
		}
	}
	if b.node.Tag == "ms" {
		text = b.node.AttrOr("lquote", "\"") + text + b.node.AttrOr("rquote", "\"")
	}
	runs := e.chainOf(ts).ShapeText([]rune(text), ts.fontSize())
	var ascent, descent lu
	var x float64
	inkItem := &inlineItem{style: ts, runes: []rune(inkText)}
	e.shapeItems([]*inlineItem{inkItem}, nil)
	for _, run := range runs {
		m := run.Face.Metrics(ts.fontSize())
		rf := RasterTextFragment{Owner: b.node, Scale: 1, X: float32(x), FontSize: ts.fontSize(), Ascent: m.Ascent, Descent: m.Descent}
		var gx float64
		for _, g := range run.Glyphs {
			rf.Glyphs = append(rf.Glyphs, RasterGlyph{Face: run.Face, ID: g.ID, Advance: g.Advance, XOffset: g.XOffset, YOffset: g.YOffset})
			gx += g.Advance
		}
		rf.Width = float32(gx)
		b.math.runs = append(b.math.runs, rf)
		x += run.Width
	}
	// Keep Chromium's advance for an absent mathematical alphabet glyph,
	// but paint its base italic form so embedded fonts keep labels readable.
	if text != inkText && len(b.math.runs) == 1 && len(b.math.runs[0].Glyphs) == 1 && b.math.runs[0].Glyphs[0].ID == 0 {
		italic := e.Fonts.Resolve(ts.families, ts.weight, true).ShapeText([]rune(inkText), ts.fontSize())
		if len(italic) == 1 && len(italic[0].Glyphs) == 1 && italic[0].Glyphs[0].ID != 0 {
			g := italic[0].Glyphs[0]
			dst := &b.math.runs[0].Glyphs[0]
			dst.Face, dst.ID, dst.XOffset, dst.YOffset = italic[0].Face, g.ID, g.XOffset, g.YOffset
		}
	}
	if ink, ok := e.inkBounds(inkItem); ok {
		ascent, descent = luFromPx(float64(-ink.y)), luFromPx(float64(ink.bottom()))
	}
	setMathMetrics(b, luFromPx(font.SnapCeil(x)), ascent, descent)
	for i := range b.math.runs {
		b.math.runs[i].Baseline = float32(ascent.px())
	}
}

func (e *Engine) mathLength(v string, ts *textStyle, base lu) (lu, bool) {
	v = strings.TrimSpace(v)
	named := map[string]float64{"veryverythinmathspace": 1, "verythinmathspace": 2, "thinmathspace": 3, "mediummathspace": 4, "thickmathspace": 5, "verythickmathspace": 6, "veryverythickmathspace": 7}
	if f, ok := named[v]; ok {
		return luFromPx(ts.size * f / 18), true
	}
	if strings.HasSuffix(v, "%") {
		if f, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 64); err == nil {
			return lu(float64(base) * f / 100), true
		}
	}
	if px, ok := e.cssLength(v, ts, true); ok {
		return luFromPx(px), true
	}
	return 0, false
}

func mathCoreOperator(n *dom.Node) *dom.Node {
	if n.Tag == "mo" {
		return n
	}
	kids := mathChildren(n)
	switch n.Tag {
	case "msub", "msup", "msubsup", "munder", "mover", "munderover", "mfrac":
		if len(kids) > 0 {
			return mathCoreOperator(kids[0])
		}
	}
	if len(kids) == 1 {
		return mathCoreOperator(kids[0])
	}
	return nil
}

func (e *Engine) mathOperatorSpace(n *dom.Node, index, total int) (lu, lu) {
	op := mathCoreOperator(n)
	if op == nil {
		return 0, 0
	}
	ts := e.textStyleOf(op)
	s := strings.TrimSpace(op.TextContent())
	l, r := float64(0), float64(0)
	switch {
	case strings.Contains("=<>≤≥≠≈∈∉∋⊂⊃⊆⊇∼≡→←↔⇒⇐⇔∝∣∥", s) && s != "":
		l, r = 5, 5
	case strings.Contains("+−±∓×÷∪∩∧∨⊕⊗⋅*", s) && s != "":
		l, r = 4, 4
	case s == "," || s == ";":
		r = 3
	case s == "∑" || s == "∏" || s == "∫" || s == "⋃" || s == "⋂":
		l, r = 3, 3
	}
	form := op.AttrOr("form", "")
	if form == "" {
		if index == 0 && total > 1 {
			form = "prefix"
		}
		if index == total-1 && total > 1 {
			form = "postfix"
		}
	}
	if (form == "prefix" || form == "postfix") && (s == "+" || s == "−") {
		l, r = 0, 0
	}
	fontSize := float64(int(ts.size + 0.5))
	left, right := luFromPx(fontSize*l/18), luFromPx(fontSize*r/18)
	if v, ok := op.GetAttr("lspace"); ok {
		if a, ok := e.mathLength(v, ts, 0); ok {
			left = max(0, a)
		}
	}
	if v, ok := op.GetAttr("rspace"); ok {
		if a, ok := e.mathLength(v, ts, 0); ok {
			right = max(0, a)
		}
	}
	return left, right
}

func (e *Engine) mathRow(b *htmlBox, kids []*dom.Node, depth int) {
	var x, a, d lu
	space := isMathRoot(b.node) || mathCoreOperator(b.node) == nil
	for i, n := range kids {
		if e.display(n) == "none" || e.isOutOfFlow(n) {
			continue
		}
		c := e.layoutMathDepth(n, depth+1)
		var l, r lu
		if space {
			l, r = e.mathOperatorSpace(n, i, len(kids))
		}
		x += l
		appendMathChild(b, c, x, 0)
		x += c.w + r
		a = max(a, c.math.ascent)
		d = max(d, mathDescent(c))
	}
	setMathMetrics(b, x, a, d)
	for _, c := range b.children {
		c.y = a - c.math.ascent
	}
}

func (e *Engine) mathFraction(b, num, den *htmlBox) {
	ts := b.style
	xheight := e.mathXHeight(ts)
	axis := e.mathConstant(ts, "AxisHeight", xheight/2)
	thick := e.mathRule(ts)
	if v, ok := b.node.GetAttr("linethickness"); ok {
		if w, ok := e.mathLength(v, ts, thick); ok {
			thick = max(0, w)
		}
	}
	display := mathDisplayStyle(b.node)
	gap := thick
	if display {
		fallback := ts.size / 20
		if f := e.chainOf(ts).Primary(); f != nil {
			fallback = f.UnderlineThickness(ts.fontSize())
		}
		gap = luFromPx(3 * fallback)
	}
	up, down := lu(0), lu(0)
	if thick > 0 {
		ngap := e.mathConstant(ts, "FractionNumeratorGapMin", gap.px())
		dgap := e.mathConstant(ts, "FractionDenominatorGapMin", gap.px())
		nshift, dshift := "FractionNumeratorShiftUp", "FractionDenominatorShiftDown"
		if display {
			ngap = e.mathConstant(ts, "FractionNumDisplayStyleGapMin", gap.px())
			dgap = e.mathConstant(ts, "FractionDenomDisplayStyleGapMin", gap.px())
			nshift = "FractionNumeratorDisplayStyleShiftUp"
			dshift = "FractionDenominatorDisplayStyleShiftDown"
		}
		up = max(e.mathConstant(ts, nshift, 0), axis+thick/2+ngap+mathDescent(num))
		down = max(e.mathConstant(ts, dshift, 0), thick/2+dgap+den.math.ascent-axis)
	} else {
		gap = e.mathConstant(ts, "StackGapMin", float64(3*e.mathRule(ts).px()))
		if display {
			gap = e.mathConstant(ts, "StackDisplayStyleGapMin", float64(7*e.mathRule(ts).px()))
		}
		diff := gap + den.math.ascent + mathDescent(num)
		up = diff / 2
		down = diff - up
	}
	a := max(0, up+num.math.ascent, -down+den.math.ascent)
	d := max(0, -up+mathDescent(num), down+mathDescent(den))
	w := max(num.w, den.w) + luFromPx(2)
	setMathMetrics(b, w, a, d)
	appendMathChild(b, num, (w-num.w)/2, a-up-num.math.ascent)
	appendMathChild(b, den, (w-den.w)/2, a+down-den.math.ascent)
	if thick > 0 {
		b.math.rules = append(b.math.rules, RasterMathRule{Owner: b.node, X: 1, Y: (a - axis - thick/2).px(), Width: max(0, w-luFromPx(2)).px(), Height: thick.px()})
	}
}

func (e *Engine) mathScripts(b, base, sub, sup *htmlBox) {
	ts := b.style
	xh := e.mathXHeight(ts)
	a, d := base.math.ascent, mathDescent(base)
	var up, down lu
	if sub != nil {
		down = max(e.mathConstant(ts, "SubscriptShiftDown", xh/3), d+e.mathConstant(ts, "SubscriptBaselineDropMin", xh/2), sub.math.ascent-e.mathConstant(ts, "SubscriptTopMax", 4*xh/5))
	}
	if sup != nil {
		up = max(e.mathConstant(ts, "SuperscriptShiftUp", xh), a-e.mathConstant(ts, "SuperscriptBaselineDropMax", xh/2), e.mathConstant(ts, "SuperscriptBottomMin", xh/4)+mathDescent(sup))
	}
	if sub != nil && sup != nil {
		gap := (down - sub.math.ascent) + (up - mathDescent(sup))
		minGap := e.mathConstant(ts, "SubSuperscriptGapMin", float64(int(ts.size)/5))
		if gap < minGap {
			delta := max(0, min(minGap-gap, e.mathConstant(ts, "SuperscriptBottomMaxWithSubscript", 4*xh/5)-(up-mathDescent(sup))))
			up += delta
			gap += delta
			if gap < minGap {
				down += minGap - gap
			}
		}
	}
	w := base.w
	if sub != nil {
		w = max(w, base.w+sub.w)
		d = max(d, down+mathDescent(sub))
	}
	if sup != nil {
		w = max(w, base.w+sup.w)
		a = max(a, up+sup.math.ascent)
	}
	w += e.mathConstant(ts, "SpaceAfterScript", float64(int(ts.size)/5))
	setMathMetrics(b, w, a, d)
	appendMathChild(b, base, 0, a-base.math.ascent)
	if sub != nil {
		appendMathChild(b, sub, base.w, a+down-sub.math.ascent)
	}
	if sup != nil {
		appendMathChild(b, sup, base.w, a-up-sup.math.ascent)
	}
}

func (e *Engine) mathUnderOver(b, base, under, over *htmlBox) {
	op := mathCoreOperator(base.node)
	if !mathDisplayStyle(b.node) && op != nil && strings.Contains("∑∏⋃⋂", op.TextContent()) {
		e.mathScripts(b, base, under, over)
		return
	}
	w, a, d := base.w, base.math.ascent, mathDescent(base)
	var overH, underH lu
	if over != nil {
		w = max(w, over.w)
		overH = over.h + e.mathConstant(b.style, "UpperLimitGapMin", 0)
		a += overH
	}
	if under != nil {
		w = max(w, under.w)
		underH = under.h + e.mathConstant(b.style, "LowerLimitGapMin", 0)
		d += underH
	}
	setMathMetrics(b, w, a, d)
	appendMathChild(b, base, (w-base.w)/2, overH)
	if over != nil {
		appendMathChild(b, over, (w-over.w)/2, 0)
	}
	if under != nil {
		appendMathChild(b, under, (w-under.w)/2, overH+base.h+underH-under.h)
	}
}

func (e *Engine) mathRadical(b, base, index *htmlBox) {
	ts := b.style
	rule := e.mathRule(ts)
	rulePx := rule.px()
	if f := e.chainOf(ts).Primary(); f != nil {
		rulePx = f.UnderlineThickness(ts.fontSize())
	}
	gap := luFromPx(5 * rulePx / 4)
	if mathDisplayStyle(b.node) {
		gap = luFromPx(rulePx + float64(e.mathXHeight(ts)/4))
	}
	gap = e.mathConstant(ts, "RadicalVerticalGap", gap.px())
	extra := e.mathConstant(ts, "RadicalExtraAscender", rule.px())
	surd := newMathBox(b.node, ts)
	surd.math.synthetic = true
	e.mathToken(surd, "√")
	surd.w = luFromPx(font.TextWidth(e.chainOf(ts).ShapeText([]rune{'√'}, ts.fontSize())))
	a := base.math.ascent + gap + rule + extra
	d := max(mathDescent(base), surd.h+extra-a)
	var ix, kbefore, kafter lu
	if index != nil {
		kbefore = luFromPx(5 * ts.size / 18)
		kafter = max(-index.w, luFromPx(-10*ts.size/18))
		ix = index.w + kbefore + kafter
		a = max(a, index.h-d+luFromPx(0.6*surd.h.px()))
	}
	setMathMetrics(b, ix+surd.w+base.w, a, d)
	appendMathChild(b, surd, ix, a-surd.math.ascent)
	appendMathChild(b, base, ix+surd.w, a-base.math.ascent)
	if index != nil {
		appendMathChild(b, index, kbefore, a+d-luFromPx(0.6*surd.h.px())-index.h)
	}
	b.math.rules = append(b.math.rules, RasterMathRule{Owner: b.node, X: (ix + surd.w).px(), Y: (a - base.math.ascent - gap - rule).px(), Width: base.w.px(), Height: rule.px()})
}

func (e *Engine) mathTable(b *htmlBox, rows []*dom.Node, depth int) {
	var cells [][]*htmlBox
	var widths []lu
	var heights []lu
	var ascents []lu
	for _, row := range rows {
		var line []*htmlBox
		var a, d lu
		for col, n := range mathChildren(row) {
			c := e.layoutMathDepth(n, depth+1)
			line = append(line, c)
			for len(widths) <= col {
				widths = append(widths, 0)
			}
			widths[col] = max(widths[col], c.w)
			a = max(a, c.math.ascent)
			d = max(d, mathDescent(c))
		}
		cells = append(cells, line)
		heights = append(heights, a+d)
		ascents = append(ascents, a)
	}
	var w, h lu
	gap := lu(0)
	for _, v := range widths {
		w += v + gap
	}
	if len(widths) > 0 {
		w -= gap
	}
	for _, v := range heights {
		h += v
	}
	axis := e.mathConstant(b.style, "AxisHeight", e.mathXHeight(b.style)/2)
	setMathMetrics(b, w, h/2+axis, h/2-axis)
	var y lu
	for i, line := range cells {
		row := newMathBox(rows[i], e.textStyleOf(rows[i]))
		setMathMetrics(row, w, ascents[i], heights[i]-ascents[i])
		var x lu
		for j, c := range line {
			appendMathChild(row, c, x+(widths[j]-c.w)/2, row.math.ascent-c.math.ascent)
			x += widths[j] + gap
		}
		appendMathChild(b, row, 0, y)
		y += heights[i]
	}
}

func (e *Engine) newMathInlineItem(n *dom.Node, avail lu) *inlineItem {
	box := e.layoutMath(n)
	it := e.newMarkItem(markAtomic, n, n.Parent, inlineEdge{})
	it.mark.atomicBox = box
	it.mark.atomicW = box.w
	it.mark.atomicH = box.h
	it.mark.atomicBaseline = box.math.ascent
	return it
}
