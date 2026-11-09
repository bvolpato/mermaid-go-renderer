// Ported from skia/src/core/SkScan_AAAPath.cpp, SkScan_AntiPath.cpp,
// SkAlphaRuns.h and SkScanPriv.h (Skia branch chrome/m156).
//
// This is the analytic anti-aliasing scan converter of Skia. Chromium
// rasterizes every anti-aliased path fill with it in software mode. The
// port keeps the fixed-point arithmetic and the order of the blits, because
// both decide the coverage values.

package raster

// ---- SkAlphaRuns ----

type alphaRuns struct {
	runs  []int16
	alpha []uint8
}

func (r *alphaRuns) reset(width int) {
	r.runs[0] = int16(width)
	r.runs[width] = 0
	r.alpha[0] = 0
}

func (r *alphaRuns) empty() bool {
	return r.alpha[0] == 0 && r.runs[r.runs[0]] == 0
}

// catchOverflow is SkAlphaRuns::CatchOverflow: 0 to 255 from 0 to 256.
func catchOverflow(alpha int) uint8 { return uint8(alpha - (alpha >> 8)) }

// breakRuns is SkAlphaRuns::Break.
func breakRuns(runs []int16, alpha []uint8, x, count int) {
	i := 0
	nextI := x
	for x > 0 {
		n := int(runs[i])
		if x < n {
			alpha[i+x] = alpha[i]
			runs[i] = int16(x)
			runs[i+x] = int16(n - x)
			break
		}
		i += n
		x -= n
	}
	i = nextI
	x = count
	for {
		n := int(runs[i])
		if x < n {
			alpha[i+x] = alpha[i]
			runs[i] = int16(x)
			runs[i+x] = int16(n - x)
			break
		}
		x -= n
		if x <= 0 {
			break
		}
		i += n
	}
}

// add is SkAlphaRuns::add. It returns the new offsetX.
func (r *alphaRuns) add(x int, startAlpha uint8, middleCount int, stopAlpha uint8, maxValue uint8, offsetX int) int {
	base := offsetX
	last := offsetX
	x -= offsetX
	if startAlpha != 0 {
		breakRuns(r.runs[base:], r.alpha[base:], x, 1)
		tmp := int(r.alpha[base+x]) + int(startAlpha)
		r.alpha[base+x] = uint8(tmp - (tmp >> 8))
		base += x + 1
		x = 0
	}
	if middleCount != 0 {
		breakRuns(r.runs[base:], r.alpha[base:], x, middleCount)
		base += x
		x = 0
		for {
			r.alpha[base] = catchOverflow(int(r.alpha[base]) + int(maxValue))
			n := int(r.runs[base])
			base += n
			middleCount -= n
			if middleCount <= 0 {
				break
			}
		}
		last = base
	}
	if stopAlpha != 0 {
		breakRuns(r.runs[base:], r.alpha[base:], x, 1)
		base += x
		r.alpha[base] = r.alpha[base] + stopAlpha
		last = base
	}
	return last
}

// ---- additive blitters ----

func addAlpha(alpha *uint8, delta uint8) {
	*alpha = catchOverflow(int(*alpha) + int(delta))
}

func safelyAddAlpha(alpha *uint8, delta uint8) {
	*alpha = uint8(min(0xFF, int(*alpha)+int(delta)))
}

// additiveBlitter is AdditiveBlitter.
type additiveBlitter interface {
	realBlitter() blitter
	addAntiH(x, y int, antialias []uint8)
	addAntiH1(x, y int, alpha uint8)
	addAntiHW(x, y, width int, alpha uint8)
	flushIfYChanged(y, nextY fixed)
}

const (
	maskMaxWidth   = 32
	maskMaxStorage = 1024
)

// maskAdditiveBlitter is MaskAdditiveBlitter: it adds coverage into a small
// A8 mask and blits the mask at the end.
type maskAdditiveBlitter struct {
	real     blitter
	mask     maskA8
	clipRect IRect
	// storage has one extra byte at each end: the scan converter can write
	// one byte outside of the mask because of its limited precision.
	storage [maskMaxStorage + 8]uint8
}

func maskCanHandleRect(bounds IRect) bool {
	width := bounds.width()
	if width > maskMaxWidth {
		return false
	}
	rb := int64((width + 3) &^ 3)
	return rb*int64(bounds.height()) <= maskMaxStorage
}

func (m *maskAdditiveBlitter) init(real blitter, ir, clipBounds IRect) {
	m.real = real
	m.mask = maskA8{image: m.storage[1:], bounds: ir, rowB: ir.width()}
	m.clipRect = ir
	if !m.clipRect.intersect(clipBounds) {
		m.clipRect = IRect{}
	}
	n := ir.height()*ir.width() + 2
	for i := 0; i < n; i++ {
		m.storage[i] = 0
	}
}

func (m *maskAdditiveBlitter) finish() {
	if !m.clipRect.isEmpty() {
		m.real.blitMask(&m.mask, m.clipRect)
	}
}

// at returns the index in storage of the pixel (x, y). x can be one pixel
// outside of the mask.
func (m *maskAdditiveBlitter) at(x, y int) int {
	return 1 + (y-m.mask.bounds.T)*m.mask.rowB + (x - m.mask.bounds.L)
}

func (m *maskAdditiveBlitter) realBlitter() blitter { return m }

func (m *maskAdditiveBlitter) addAntiH(x, y int, antialias []uint8) {}

func (m *maskAdditiveBlitter) addAntiH1(x, y int, alpha uint8) {
	addAlpha(&m.storage[m.at(x, y)], alpha)
}

func (m *maskAdditiveBlitter) addAntiHW(x, y, width int, alpha uint8) {
	o := m.at(x, y)
	for i := 0; i < width; i++ {
		addAlpha(&m.storage[o+i], alpha)
	}
}

func (m *maskAdditiveBlitter) flushIfYChanged(y, nextY fixed) {}

// The methods below make the mask act as the real blitter for the
// rectangles of aaa_walk_convex_edges: they set the alpha.
func (m *maskAdditiveBlitter) blitV(x, y, height int, alpha uint8) {
	if alpha == 0 {
		return
	}
	o := m.at(x, y)
	for i := 0; i < height; i++ {
		m.storage[o] = alpha
		o += m.mask.rowB
	}
}

func (m *maskAdditiveBlitter) blitRect(x, y, width, height int) {
	o := m.at(x, y)
	for i := 0; i < height; i++ {
		row := m.storage[o : o+width]
		for j := range row {
			row[j] = 0xFF
		}
		o += m.mask.rowB
	}
}

func (m *maskAdditiveBlitter) blitAntiRect(x, y, width, height int, leftAlpha, rightAlpha uint8) {
	m.blitV(x, y, height, leftAlpha)
	m.blitV(x+1+width, y, height, rightAlpha)
	m.blitRect(x+1, y, width, height)
}

func (m *maskAdditiveBlitter) blitH(x, y, width int)                        {}
func (m *maskAdditiveBlitter) blitAntiH(x, y int, aa []uint8, runs []int16) {}
func (m *maskAdditiveBlitter) blitMask(mk *maskA8, clip IRect)              {}
func (m *maskAdditiveBlitter) blitAntiH2(x, y int, a0, a1 uint8)            {}
func (m *maskAdditiveBlitter) blitAntiV2(x, y int, a0, a1 uint8)            {}

// runAdditiveBlitter is RunBasedAdditiveBlitter, and SafeRLEAdditiveBlitter
// when safe is set: it accumulates one scanline as runs and flushes it to
// the real blitter.
type runAdditiveBlitter struct {
	real    blitter
	safe    bool
	currY   int
	width   int
	left    int
	top     int
	runs    alphaRuns
	offsetX int
}

func (r *runAdditiveBlitter) init(real blitter, ir, clipBounds IRect, safe bool) {
	r.real = real
	r.safe = safe
	sect := ir
	if !sect.intersect(clipBounds) {
		sect = IRect{}
	}
	r.left = sect.L
	r.width = sect.R - sect.L
	r.top = sect.T
	r.currY = r.top - 1
	need := r.width + 2
	if cap(r.runs.runs) < need {
		r.runs.runs = make([]int16, need*2)
		r.runs.alpha = make([]uint8, need*2)
	}
	r.runs.runs = r.runs.runs[:cap(r.runs.runs)]
	r.runs.alpha = r.runs.alpha[:cap(r.runs.alpha)]
	if r.width > 0 {
		r.runs.reset(r.width)
	}
	r.offsetX = 0
}

func (r *runAdditiveBlitter) realBlitter() blitter { return r.real }

func snapAlpha(alpha uint8) uint8 {
	if alpha > 247 {
		return 0xFF
	}
	if alpha < 8 {
		return 0
	}
	return alpha
}

func (r *runAdditiveBlitter) flush() {
	if r.currY >= r.top {
		if r.width > 0 {
			for x := 0; r.runs.runs[x] != 0; x += int(r.runs.runs[x]) {
				r.runs.alpha[x] = snapAlpha(r.runs.alpha[x])
			}
			if !r.runs.empty() {
				r.real.blitAntiH(r.left, r.currY, r.runs.alpha, r.runs.runs)
				r.runs.reset(r.width)
				r.offsetX = 0
			}
		}
		r.currY = r.top - 1
	}
}

func (r *runAdditiveBlitter) finish() { r.flush() }

func (r *runAdditiveBlitter) checkY(y int) {
	if y != r.currY {
		r.flush()
		r.currY = y
	}
}

func (r *runAdditiveBlitter) check(x, width int) bool {
	return x >= 0 && width >= 0 && x+width <= r.width
}

func (r *runAdditiveBlitter) flushIfYChanged(y, nextY fixed) {
	if fixedFloorToInt(y) != fixedFloorToInt(nextY) {
		r.flush()
	}
}

func (r *runAdditiveBlitter) addAntiH(x, y int, antialias []uint8) {
	r.checkY(y)
	x -= r.left
	length := len(antialias)
	if x < 0 {
		length += x
		if length <= 0 {
			return
		}
		antialias = antialias[-x:]
		x = 0
	}
	length = min(length, r.width-x)
	if length <= 0 {
		return
	}
	if x < r.offsetX {
		r.offsetX = 0
	}
	r.offsetX = r.runs.add(x, 0, length, 0, 0, r.offsetX)
	runs, alpha := r.runs.runs, r.runs.alpha
	for i := 0; i < length; i += int(runs[x+i]) {
		for j := 1; j < int(runs[x+i]); j++ {
			runs[x+i+j] = 1
			alpha[x+i+j] = alpha[x+i]
		}
		runs[x+i] = 1
	}
	if r.safe {
		for i := 0; i < length; i++ {
			safelyAddAlpha(&alpha[x+i], antialias[i])
		}
	} else {
		for i := 0; i < length; i++ {
			addAlpha(&alpha[x+i], antialias[i])
		}
	}
}

func (r *runAdditiveBlitter) addAntiH1(x, y int, alpha uint8) {
	r.checkY(y)
	x -= r.left
	if x < r.offsetX {
		r.offsetX = 0
	}
	if r.check(x, 1) {
		if r.safe {
			r.offsetX = r.runs.add(x, 0, 1, 0, 0, r.offsetX)
			safelyAddAlpha(&r.runs.alpha[x], alpha)
		} else {
			r.offsetX = r.runs.add(x, 0, 1, 0, alpha, r.offsetX)
		}
	}
}

func (r *runAdditiveBlitter) addAntiHW(x, y, width int, alpha uint8) {
	r.checkY(y)
	x -= r.left
	if x < 0 {
		width += x
		x = 0
	}
	width = min(width, r.width-x)
	if width <= 0 {
		return
	}
	if x < r.offsetX {
		r.offsetX = 0
	}
	if r.check(x, width) {
		if r.safe {
			r.offsetX = r.runs.add(x, 0, width, 0, 0, r.offsetX)
			for i := x; i < x+width; i += int(r.runs.runs[i]) {
				safelyAddAlpha(&r.runs.alpha[i], alpha)
			}
		} else {
			r.offsetX = r.runs.add(x, 0, width, 0, alpha, r.offsetX)
		}
	}
}

// ---- coverage helpers ----

func trapezoidToAlpha(l1, l2 fixed) uint8 {
	area := (l1 + l2) / 2
	return uint8(area >> 8)
}

func partialTriangleToAlpha(a, b fixed) uint8 {
	area := (a >> 11) * (a >> 11) * (b >> 11)
	return uint8((area >> 8) & 0xFF)
}

func getPartialAlphaFixed(alpha uint8, partialHeight fixed) uint8 {
	return uint8(fixedRoundToInt(fixed(alpha) * partialHeight))
}

func getPartialAlpha(alpha, fullAlpha uint8) uint8 {
	return uint8((int(alpha) * int(fullAlpha)) >> 8)
}

func fixedToAlpha(f fixed) uint8 { return getPartialAlphaFixed(0xFF, f) }

func approximateIntersection(l1, r1, l2, r2 fixed) fixed {
	if l1 > r1 {
		l1, r1 = r1, l1
	}
	if l2 > r2 {
		l2, r2 = r2, l2
	}
	return (max(l1, l2) + min(r1, r2)) / 2
}

func computeAlphaAboveLine(alphas []uint8, l, r, dY fixed, fullAlpha uint8) {
	R := fixedCeilToInt(r)
	if R == 0 {
		return
	} else if R == 1 {
		alphas[0] = getPartialAlpha(uint8(((fixed(R)<<17)-l-r)>>9), fullAlpha)
	} else {
		first := fixed1 - l
		last := r - (fixed(R-1) << 16)
		firstH := fixedMul(first, dY)
		alphas[0] = uint8(fixedMul(first, firstH) >> 9)
		alpha16 := satAdd32(firstH, dY>>1)
		for i := 1; i < R-1; i++ {
			alphas[i] = uint8(alpha16 >> 8)
			alpha16 = satAdd32(alpha16, dY)
		}
		alphas[R-1] = fullAlpha - partialTriangleToAlpha(last, dY)
	}
}

func computeAlphaBelowLine(alphas []uint8, l, r, dY fixed, fullAlpha uint8) {
	R := fixedCeilToInt(r)
	if R == 0 {
		return
	} else if R == 1 {
		alphas[0] = getPartialAlpha(trapezoidToAlpha(l, r), fullAlpha)
	} else {
		first := fixed1 - l
		last := r - (fixed(R-1) << 16)
		lastH := fixedMul(last, dY)
		alphas[R-1] = uint8(fixedMul(last, lastH) >> 9)
		alpha16 := satAdd32(lastH, dY>>1)
		for i := R - 2; i > 0; i-- {
			alphas[i] = uint8((alpha16 >> 8) & 0xFF)
			alpha16 = satAdd32(alpha16, dY)
		}
		alphas[0] = fullAlpha - partialTriangleToAlpha(first, dY)
	}
}

// aaaState holds the scratch memory of one fill.
type aaaState struct {
	ab       additiveBlitter
	mask     *maskAdditiveBlitter // set when the mask blitter is in use
	alphas   []uint8
	runs     []int16
	builder  edgeBuilder
	maskB    maskAdditiveBlitter
	runB     runAdditiveBlitter
	rectClip rectClipBlitter
	fat      runScratch
}

func (s *aaaState) maskRowAt(x, y int) *uint8 { return &s.mask.storage[s.mask.at(x, y)] }

func (s *aaaState) blitSingleAlpha(y, x int, alpha, fullAlpha uint8, useMask, noRealBlitter bool) {
	if useMask {
		if fullAlpha == 0xFF && !noRealBlitter {
			*s.maskRowAt(x, y) = alpha
		} else {
			safelyAddAlpha(s.maskRowAt(x, y), getPartialAlpha(alpha, fullAlpha))
		}
	} else {
		if fullAlpha == 0xFF && !noRealBlitter {
			s.ab.realBlitter().blitV(x, y, 1, alpha)
		} else {
			s.ab.addAntiH1(x, y, getPartialAlpha(alpha, fullAlpha))
		}
	}
}

func (s *aaaState) blitTwoAlphas(y, x int, a1, a2, fullAlpha uint8, useMask, noRealBlitter bool) {
	if useMask {
		safelyAddAlpha(s.maskRowAt(x, y), a1)
		safelyAddAlpha(s.maskRowAt(x+1, y), a2)
	} else {
		if fullAlpha == 0xFF && !noRealBlitter {
			s.ab.realBlitter().blitAntiH2(x, y, a1, a2)
		} else {
			s.ab.addAntiH1(x, y, a1)
			s.ab.addAntiH1(x+1, y, a2)
		}
	}
}

func (s *aaaState) blitFullAlpha(y, x, length int, fullAlpha uint8, useMask, noRealBlitter bool) {
	if useMask {
		for i := 0; i < length; i++ {
			safelyAddAlpha(s.maskRowAt(x+i, y), fullAlpha)
		}
	} else {
		if fullAlpha == 0xFF && !noRealBlitter {
			s.ab.realBlitter().blitH(x, y, length)
		} else {
			s.ab.addAntiHW(x, y, length, fullAlpha)
		}
	}
}

func (s *aaaState) blitAAATrapezoidRow(y int, ul, ur, ll, lr, lDY, rDY fixed, fullAlpha uint8, useMask, noRealBlitter bool) {
	L, R := fixedFloorToInt(ul), fixedCeilToInt(lr)
	length := R - L
	if length == 1 {
		alpha := trapezoidToAlpha(ur-ul, lr-ll)
		s.blitSingleAlpha(y, L, alpha, fullAlpha, useMask, noRealBlitter)
		return
	}
	if length <= 0 {
		return
	}
	need := length + 1
	if cap(s.alphas) < need*2 {
		s.alphas = make([]uint8, need*4)
		s.runs = make([]int16, need*4)
	}
	alphas := s.alphas[:need]
	tempAlphas := s.alphas[need : need*2]
	runs := s.runs[:need]
	for i := 0; i < length; i++ {
		runs[i] = 1
		alphas[i] = fullAlpha
	}
	runs[length] = 0

	uL := fixedFloorToInt(ul)
	lL := fixedCeilToInt(ll)
	if uL+2 == lL {
		first := intToFixed(uL) + fixed1 - ul
		second := ll - ul - first
		a1 := fullAlpha - partialTriangleToAlpha(first, lDY)
		a2 := partialTriangleToAlpha(second, lDY)
		if alphas[0] > a1 {
			alphas[0] -= a1
		} else {
			alphas[0] = 0
		}
		if alphas[1] > a2 {
			alphas[1] -= a2
		} else {
			alphas[1] = 0
		}
	} else {
		computeAlphaBelowLine(tempAlphas[uL-L:], ul-intToFixed(uL), ll-intToFixed(uL), lDY, fullAlpha)
		for i := uL; i < lL; i++ {
			if alphas[i-L] > tempAlphas[i-L] {
				alphas[i-L] -= tempAlphas[i-L]
			} else {
				alphas[i-L] = 0
			}
		}
	}

	uR := fixedFloorToInt(ur)
	lR := fixedCeilToInt(lr)
	if uR+2 == lR {
		first := intToFixed(uR) + fixed1 - ur
		second := lr - ur - first
		a1 := partialTriangleToAlpha(first, rDY)
		a2 := fullAlpha - partialTriangleToAlpha(second, rDY)
		if alphas[length-2] > a1 {
			alphas[length-2] -= a1
		} else {
			alphas[length-2] = 0
		}
		if alphas[length-1] > a2 {
			alphas[length-1] -= a2
		} else {
			alphas[length-1] = 0
		}
	} else {
		computeAlphaAboveLine(tempAlphas[uR-L:], ur-intToFixed(uR), lr-intToFixed(uR), rDY, fullAlpha)
		for i := uR; i < lR; i++ {
			if alphas[i-L] > tempAlphas[i-L] {
				alphas[i-L] -= tempAlphas[i-L]
			} else {
				alphas[i-L] = 0
			}
		}
	}

	if useMask {
		for i := 0; i < length; i++ {
			safelyAddAlpha(s.maskRowAt(L+i, y), alphas[i])
		}
	} else {
		if fullAlpha == 0xFF && !noRealBlitter {
			s.ab.realBlitter().blitAntiH(L, y, alphas[:length], runs)
		} else {
			s.ab.addAntiH(L, y, alphas[:length])
		}
	}
}

func (s *aaaState) blitTrapezoidRow(y int, ul, ur, ll, lr, lDY, rDY fixed, fullAlpha uint8, useMask, noRealBlitter bool) {
	if ul > ur {
		return
	}
	if ll > lr {
		ll = approximateIntersection(ul, ll, ur, lr)
		lr = ll
	}
	if ul == ur && ll == lr {
		return
	}
	if ul > ll {
		ul, ll = ll, ul
	}
	if ur > lr {
		ur, lr = lr, ur
	}
	joinLeft := fixedCeilToFixed(ll)
	joinRite := fixedFloorToFixed(ur)
	if joinLeft <= joinRite {
		if ul < joinLeft {
			length := fixedCeilToInt(joinLeft - ul)
			if length == 1 {
				alpha := trapezoidToAlpha(joinLeft-ul, joinLeft-ll)
				s.blitSingleAlpha(y, int(ul>>16), alpha, fullAlpha, useMask, noRealBlitter)
			} else if length == 2 {
				first := joinLeft - fixed1 - ul
				second := ll - ul - first
				a1 := partialTriangleToAlpha(first, lDY)
				a2 := fullAlpha - partialTriangleToAlpha(second, lDY)
				s.blitTwoAlphas(y, int(ul>>16), a1, a2, fullAlpha, useMask, noRealBlitter)
			} else {
				s.blitAAATrapezoidRow(y, ul, joinLeft, ll, joinLeft, lDY, maxS32, fullAlpha, useMask, noRealBlitter)
			}
		}
		if joinLeft < joinRite {
			s.blitFullAlpha(y, fixedFloorToInt(joinLeft), fixedFloorToInt(joinRite-joinLeft), fullAlpha, useMask, noRealBlitter)
		}
		if lr > joinRite {
			length := fixedCeilToInt(lr - joinRite)
			if length == 1 {
				alpha := trapezoidToAlpha(ur-joinRite, lr-joinRite)
				s.blitSingleAlpha(y, int(joinRite>>16), alpha, fullAlpha, useMask, noRealBlitter)
			} else if length == 2 {
				first := joinRite + fixed1 - ur
				second := lr - ur - first
				a1 := fullAlpha - partialTriangleToAlpha(first, rDY)
				a2 := partialTriangleToAlpha(second, rDY)
				s.blitTwoAlphas(y, int(joinRite>>16), a1, a2, fullAlpha, useMask, noRealBlitter)
			} else {
				s.blitAAATrapezoidRow(y, joinRite, ur, joinRite, lr, maxS32, rDY, fullAlpha, useMask, noRealBlitter)
			}
		}
	} else {
		s.blitAAATrapezoidRow(y, ul, ur, ll, lr, lDY, rDY, fullAlpha, useMask, noRealBlitter)
	}
}

func compareEdges(a, b *aEdge) bool {
	if a.upperY != b.upperY {
		return a.upperY < b.upperY
	}
	if a.x != b.x {
		return a.x < b.x
	}
	return a.dx < b.dx
}

// sortEdges sorts the edge list. SkTQSort is not stable, but its order for
// equal keys does not matter: equal edges are identical in all the fields
// that the comparison and the walk read.
func sortEdges(list []*aEdge) {
	// Insertion sort for short lists, a simple quicksort otherwise.
	if len(list) <= 12 {
		for i := 1; i < len(list); i++ {
			e := list[i]
			j := i - 1
			for j >= 0 && compareEdges(e, list[j]) {
				list[j+1] = list[j]
				j--
			}
			list[j+1] = e
		}
		return
	}
	pivot := list[len(list)/2]
	i, j := 0, len(list)-1
	for i <= j {
		for compareEdges(list[i], pivot) {
			i++
		}
		for compareEdges(pivot, list[j]) {
			j--
		}
		if i <= j {
			list[i], list[j] = list[j], list[i]
			i++
			j--
		}
	}
	if j > 0 {
		sortEdges(list[:j+1])
	}
	if i < len(list)-1 {
		sortEdges(list[i:])
	}
}

func isSmoothEnough1(thisEdge, nextEdge *aEdge) bool {
	if thisEdge.curveCount < 0 {
		ddshift := uint(thisEdge.curveShift)
		return abs32(thisEdge.cdx)>>1 >= abs32(thisEdge.cddx)>>ddshift &&
			abs32(thisEdge.cdy)>>1 >= abs32(thisEdge.cddy)>>ddshift &&
			(thisEdge.cdy-(thisEdge.cddy>>ddshift))>>uint(thisEdge.toFixedShift) >= fixed1
	} else if thisEdge.curveCount > 0 {
		return abs32(thisEdge.qdx)>>1 >= abs32(thisEdge.qddx) &&
			abs32(thisEdge.qdy)>>1 >= abs32(thisEdge.qddy) &&
			(thisEdge.qdy-thisEdge.qddy)>>uint(thisEdge.curveShift) >= fixed1
	}
	return abs32(satSub32(nextEdge.dx, thisEdge.dx)) <= fixed1 &&
		nextEdge.lowerY-nextEdge.upperY >= fixed1
}

func isSmoothEnough(leftE, riteE, currE *aEdge, stopY int) bool {
	if currE.upperY >= fixed(uint32(stopY)<<16) {
		return false
	}
	if leftE.lowerY+fixed1 < riteE.lowerY {
		return isSmoothEnough1(leftE, currE)
	} else if leftE.lowerY > riteE.lowerY+fixed1 {
		return isSmoothEnough1(riteE, currE)
	}
	nextCurrE := currE.next
	if nextCurrE.upperY >= fixed(uint32(stopY)<<16) {
		return false
	}
	if nextCurrE.upperX < currE.upperX {
		currE, nextCurrE = nextCurrE, currE
	}
	return isSmoothEnough1(leftE, currE) && isSmoothEnough1(riteE, nextCurrE)
}

func clampFixed(v, lo, hi fixed) fixed {
	// std::clamp: the result is lo when v < lo, hi when hi < v.
	if v < lo {
		return lo
	}
	if hi < v {
		return hi
	}
	return v
}

// walkConvexEdges is aaa_walk_convex_edges.
func (s *aaaState) walkConvexEdges(prevHead *aEdge, startY, stopY int, leftBound, riteBound fixed, useMask bool) {
	blitter := s.ab
	leftE := prevHead.next
	riteE := leftE.next
	currE := riteE.next

	y := max(leftE.upperY, riteE.upperY)

	for {
		for leftE.lowerY <= y || riteE.lowerY <= y {
			for leftE.lowerY <= y {
				if !leftE.update() {
					if fixedFloorToInt(currE.upperY) >= stopY {
						return
					}
					leftE = currE
					currE = currE.next
				}
			}
			for riteE.lowerY <= y {
				if !riteE.update() {
					if fixedFloorToInt(currE.upperY) >= stopY {
						return
					}
					riteE = currE
					currE = currE.next
				}
			}
			prevY := y
			y = max(y, leftE.upperY, riteE.upperY)
			blitter.flushIfYChanged(prevY, y)
		}

		if fixedFloorToInt(y) >= stopY {
			break
		}

		leftE.goY(y)
		riteE.goY(y)

		if leftE.x > riteE.x || (leftE.x == riteE.x && leftE.dx > riteE.dx) {
			leftE, riteE = riteE, leftE
		}

		localBotFixed := min(leftE.lowerY, riteE.lowerY)
		if isSmoothEnough(leftE, riteE, currE, stopY) {
			localBotFixed = fixedCeilToFixed(localBotFixed)
		}
		localBotFixed = min(localBotFixed, intToFixed(stopY))

		left := clampFixed(leftE.x, leftBound, riteBound)
		dLeft := leftE.dx
		rite := clampFixed(riteE.x, leftBound, riteBound)
		dRite := riteE.dx
		if (dLeft | dRite) == 0 {
			fullLeft := fixedCeilToInt(left)
			fullRite := fixedFloorToInt(rite)
			partialLeft := intToFixed(fullLeft) - left
			partialRite := rite - intToFixed(fullRite)
			fullTop := fixedCeilToInt(y)
			fullBot := fixedFloorToInt(localBotFixed)
			partialTop := intToFixed(fullTop) - y
			partialBot := localBotFixed - intToFixed(fullBot)
			if fullTop > fullBot {
				partialTop -= fixed1 - partialBot
				partialBot = 0
			}

			if fullRite >= fullLeft {
				if partialTop > 0 {
					if partialLeft > 0 {
						blitter.addAntiH1(fullLeft-1, fullTop-1, fixedToAlpha(fixedMul(partialTop, partialLeft)))
					}
					blitter.addAntiHW(fullLeft, fullTop-1, fullRite-fullLeft, fixedToAlpha(partialTop))
					if partialRite > 0 {
						blitter.addAntiH1(fullRite, fullTop-1, fixedToAlpha(fixedMul(partialTop, partialRite)))
					}
					blitter.flushIfYChanged(y, y+partialTop)
				}

				if fullBot > fullTop &&
					(fullRite > fullLeft || fixedToAlpha(partialLeft) > 0 || fixedToAlpha(partialRite) > 0) {
					blitter.realBlitter().blitAntiRect(fullLeft-1, fullTop, fullRite-fullLeft, fullBot-fullTop,
						fixedToAlpha(partialLeft), fixedToAlpha(partialRite))
				}

				if partialBot > 0 {
					if partialLeft > 0 {
						blitter.addAntiH1(fullLeft-1, fullBot, fixedToAlpha(fixedMul(partialBot, partialLeft)))
					}
					blitter.addAntiHW(fullLeft, fullBot, fullRite-fullLeft, fixedToAlpha(partialBot))
					if partialRite > 0 {
						blitter.addAntiH1(fullRite, fullBot, fixedToAlpha(fixedMul(partialBot, partialRite)))
					}
				}
			} else {
				width := rite - left
				if width > 0 {
					if partialTop > 0 {
						blitter.addAntiHW(fullLeft-1, fullTop-1, 1, fixedToAlpha(fixedMul(partialTop, width)))
						blitter.flushIfYChanged(y, y+partialTop)
					}
					if fullBot > fullTop {
						blitter.realBlitter().blitV(fullLeft-1, fullTop, fullBot-fullTop, fixedToAlpha(width))
					}
					if partialBot > 0 {
						blitter.addAntiHW(fullLeft-1, fullBot, 1, fixedToAlpha(fixedMul(partialBot, width)))
					}
				}
			}

			y = localBotFixed
		} else {
			const snapDigit = fixed1 >> 4
			const snapHalf = snapDigit >> 1
			const snapMask = fixed(-1) ^ (snapDigit - 1)
			left += snapHalf
			rite += snapHalf
			minX := leftBound + snapHalf
			maxX := riteBound + snapHalf

			count := fixedCeilToInt(localBotFixed) - fixedFloorToInt(y)

			if count > 1 {
				if fixed(uint32(y)&0xFFFF0000) != y {
					count--
					nextY := fixedCeilToFixed(y + 1)
					dY := nextY - y
					nextLeft := clampFixed(satAdd32(left, fixedMul(dLeft, dY)), minX, maxX)
					nextRite := clampFixed(satAdd32(rite, fixedMul(dRite, dY)), minX, maxX)
					s.blitTrapezoidRow(int(y>>16), left&snapMask, rite&snapMask, nextLeft&snapMask, nextRite&snapMask,
						leftE.dy, riteE.dy, getPartialAlphaFixed(0xFF, dY), useMask, false)
					blitter.flushIfYChanged(y, nextY)
					left, rite, y = nextLeft, nextRite, nextY
				}

				for count > 1 {
					count--
					nextY := y + fixed1
					nextLeft := clampFixed(satAdd32(left, dLeft), minX, maxX)
					nextRite := clampFixed(satAdd32(rite, dRite), minX, maxX)
					s.blitTrapezoidRow(int(y>>16), left&snapMask, rite&snapMask, nextLeft&snapMask, nextRite&snapMask,
						leftE.dy, riteE.dy, 0xFF, useMask, false)
					blitter.flushIfYChanged(y, nextY)
					left, rite, y = nextLeft, nextRite, nextY
				}
			}

			dY := localBotFixed - y
			nextLeft := clampFixed(satAdd32(left, fixedMul(dLeft, dY)), minX, maxX)
			nextRite := clampFixed(satAdd32(rite, fixedMul(dRite, dY)), minX, maxX)
			s.blitTrapezoidRow(int(y>>16), left&snapMask, rite&snapMask, nextLeft&snapMask, nextRite&snapMask,
				leftE.dy, riteE.dy, getPartialAlphaFixed(0xFF, dY), useMask, false)
			blitter.flushIfYChanged(y, localBotFixed)
			left, rite, y = nextLeft, nextRite, localBotFixed
			left -= snapHalf
			rite -= snapHalf
		}

		leftE.x = left
		riteE.x = rite
		leftE.y = y
		riteE.y = y
	}
}

func updateNextNextY(y, nextY fixed, nextNextY *fixed) {
	if y > nextY && y < *nextNextY {
		*nextNextY = y
	}
}

func checkIntersection(edge *aEdge, nextY fixed, nextNextY *fixed) {
	if edge.prev.prev != nil && edge.prev.x+edge.prev.dx > edge.x+edge.dx {
		*nextNextY = nextY + (fixed1 >> edgeAccuracy)
	}
}

func checkIntersectionFwd(edge *aEdge, nextY fixed, nextNextY *fixed) {
	if edge.next.next != nil && edge.x+edge.dx > edge.next.x+edge.next.dx {
		*nextNextY = nextY + (fixed1 >> edgeAccuracy)
	}
}

func removeEdge(edge *aEdge) {
	edge.prev.next = edge.next
	edge.next.prev = edge.prev
}

func insertEdgeAfter(edge, afterMe *aEdge) {
	edge.prev = afterMe
	edge.next = afterMe.next
	afterMe.next.prev = edge
	afterMe.next = edge
}

func backwardInsertEdgeBasedOnX(edge *aEdge) {
	x := edge.x
	prev := edge.prev
	for prev.prev != nil && prev.x > x {
		prev = prev.prev
	}
	if prev.next != edge {
		removeEdge(edge)
		insertEdgeAfter(edge, prev)
	}
}

func backwardInsertStart(prev *aEdge, x fixed) *aEdge {
	for prev.prev != nil && prev.x > x {
		prev = prev.prev
	}
	return prev
}

func insertNewEdges(newEdge *aEdge, y fixed, nextNextY *fixed) {
	if newEdge.upperY > y {
		updateNextNextY(newEdge.upperY, y, nextNextY)
		return
	}
	prev := newEdge.prev
	if prev.x <= newEdge.x {
		for newEdge.upperY <= y {
			checkIntersection(newEdge, y, nextNextY)
			updateNextNextY(newEdge.lowerY, y, nextNextY)
			newEdge = newEdge.next
		}
		updateNextNextY(newEdge.upperY, y, nextNextY)
		return
	}
	start := backwardInsertStart(prev, newEdge.x)
	for {
		next := newEdge.next
		skip := false
		for {
			if start.next == newEdge {
				skip = true
				break
			}
			after := start.next
			if after.x >= newEdge.x {
				break
			}
			start = after
		}
		if !skip {
			removeEdge(newEdge)
			insertEdgeAfter(newEdge, start)
		}
		checkIntersection(newEdge, y, nextNextY)
		checkIntersectionFwd(newEdge, y, nextNextY)
		updateNextNextY(newEdge.lowerY, y, nextNextY)
		start = newEdge
		newEdge = next
		if !(newEdge.upperY <= y) {
			break
		}
	}
	updateNextNextY(newEdge.upperY, y, nextNextY)
}

func edgesTooClose(prev, next *aEdge, lowerY fixed) bool {
	const slack = fixed1
	return next != nil && prev != nil && next.upperY < lowerY &&
		prev.x+slack >= next.x-abs32(next.dx)
}

func edgesTooCloseInt(prevRite int, ul, ll fixed) bool {
	return prevRite > fixedFloorToInt(ul) || prevRite > fixedFloorToInt(ll)
}

// walkEdges is aaa_walk_edges.
func (s *aaaState) walkEdges(prevHead, nextTail *aEdge, evenOdd bool, startY, stopY int, leftClip, rightClip fixed, useMask, forceRLE, skipIntersect bool) {
	prevHead.x, prevHead.upperX = leftClip, leftClip
	nextTail.x, nextTail.upperX = rightClip, rightClip
	y := max(prevHead.next.upperY, intToFixed(startY))
	nextNextY := fixed(maxS32)

	{
		edge := prevHead.next
		for ; edge.upperY <= y; edge = edge.next {
			edge.goY(y)
			updateNextNextY(edge.lowerY, y, &nextNextY)
		}
		updateNextNextY(edge.upperY, y, &nextNextY)
	}

	windingMask := -1
	if evenOdd {
		windingMask = 1
	}

	for {
		w := 0
		inInterval := false
		prevX := prevHead.x
		nextY := min(nextNextY, fixedCeilToFixed(y+1))
		currE := prevHead.next
		leftE := prevHead
		left := leftClip
		var leftDY fixed
		prevRite := fixedFloorToInt(leftClip)

		nextNextY = maxS32

		yShift := uint(0)
		if (nextY-y)&(fixed1>>2) != 0 {
			yShift = 2
			nextY = y + (fixed1 >> 2)
		} else if (nextY-y)&(fixed1>>1) != 0 {
			yShift = 1
		}

		fullAlpha := fixedToAlpha(nextY - y)
		noRealBlitter := forceRLE

		for currE.upperY <= y {
			w += int(currE.winding)
			prevInInterval := inInterval
			inInterval = (w & windingMask) != 0

			isLeft := inInterval && !prevInInterval
			isRite := !inInterval && prevInInterval

			if isRite {
				rite := currE.x
				currE.goYShift(nextY, yShift)
				nextLeft := max(leftClip, leftE.x)
				rite = min(rightClip, rite)
				nextRite := min(rightClip, currE.x)
				s.blitTrapezoidRow(int(y>>16), left, rite, nextLeft, nextRite, leftDY, currE.dy, fullAlpha, useMask,
					noRealBlitter || (fullAlpha == 0xFF &&
						(edgesTooCloseInt(prevRite, left, leftE.x) || edgesTooClose(currE, currE.next, nextY))))
				prevRite = fixedCeilToInt(max(rite, currE.x))
			} else {
				if isLeft {
					left = max(currE.x, leftClip)
					leftDY = currE.dy
					leftE = currE
				}
				currE.goYShift(nextY, yShift)
			}

			next := currE.next

			for currE.lowerY <= nextY {
				if currE.curveCount < 0 {
					currE.keepContinuousCubic()
					if !currE.updateCubic() {
						break
					}
				} else if currE.curveCount > 0 {
					currE.keepContinuousQuad()
					if !currE.updateQuadratic() {
						break
					}
				} else {
					break
				}
			}

			if currE.lowerY <= nextY {
				removeEdge(currE)
			} else {
				updateNextNextY(currE.lowerY, nextY, &nextNextY)
				newX := currE.x
				if newX < prevX {
					backwardInsertEdgeBasedOnX(currE)
				} else {
					prevX = newX
				}
				if !skipIntersect {
					checkIntersection(currE, nextY, &nextNextY)
				}
			}

			currE = next
		}

		if inInterval {
			s.blitTrapezoidRow(int(y>>16), left, rightClip, max(leftClip, leftE.x), rightClip, leftDY, 0, fullAlpha, useMask,
				noRealBlitter || (fullAlpha == 0xFF && edgesTooClose(leftE.prev, leftE, nextY)))
		}

		if forceRLE {
			s.ab.flushIfYChanged(y, nextY)
		}

		y = nextY
		if y >= intToFixed(stopY) {
			break
		}

		insertNewEdges(currE, y, &nextNextY)
	}
}

// fillPath is aaa_fill_path.
func (s *aaaState) fillPath(p *Path, clipRect IRect, startY, stopY int, pathContainedInClip, useMask, forceRLE bool) {
	var clipPtr *IRect
	if !pathContainedInClip {
		clipPtr = &clipRect
	}
	count := s.builder.buildEdges(p, clipPtr)
	if count == 0 {
		return
	}
	list := s.builder.list
	sortEdges(list)
	for i := 1; i < count; i++ {
		list[i-1].next = list[i]
		list[i].prev = list[i-1]
	}
	edge, last := list[0], list[count-1]

	var headEdge, tailEdge aEdge
	headEdge.prev = nil
	headEdge.next = edge
	headEdge.upperY, headEdge.lowerY = minS32, minS32
	headEdge.x = minS32
	headEdge.dx = 0
	headEdge.dy = maxS32
	headEdge.upperX = minS32
	edge.prev = &headEdge

	tailEdge.prev = last
	tailEdge.next = nil
	tailEdge.upperY, tailEdge.lowerY = maxS32, maxS32
	tailEdge.x = maxS32
	tailEdge.dx = 0
	tailEdge.dy = maxS32
	tailEdge.upperX = maxS32
	last.next = &tailEdge

	if !pathContainedInClip && startY < clipRect.T {
		startY = clipRect.T
	}
	if !pathContainedInClip && stopY > clipRect.B {
		stopY = clipRect.B
	}

	leftBound := intToFixed(clipRect.L)
	rightBound := intToFixed(clipRect.R)
	if useMask {
		ir := p.bounds().roundOut()
		leftBound = max(leftBound, intToFixed(ir.L))
		rightBound = min(rightBound, intToFixed(ir.R))
	}

	if p.isConvex() && count >= 2 {
		s.walkConvexEdges(&headEdge, startY, stopY, leftBound, rightBound, useMask)
	} else {
		skipIntersect := len(p.pts) > (stopY-startY)*2
		s.walkEdges(&headEdge, &tailEdge, p.fill == fillEvenOdd, startY, stopY, leftBound, rightBound, useMask, forceRLE, skipIntersect)
	}
}

func tryBlitFatAntiRect(b blitter, p *Path, clip IRect, scratch *runScratch) bool {
	rect, ok := p.isRect()
	if !ok {
		return false
	}
	if !rect.intersect(clip.toRect()) {
		return true
	}
	bounds := rect.roundOut()
	if bounds.width() < 3 {
		return false
	}
	blitFatAntiRect(b, rect, scratch)
	return true
}

// aaaFillPath is SkScan::AAAFillPath.
func (s *aaaState) aaaFillPath(p *Path, b blitter, ir, clipBounds IRect, forceRLE bool) {
	containedInClip := clipBounds.contains(ir)
	if maskCanHandleRect(ir) && !forceRLE {
		if !tryBlitFatAntiRect(b, p, clipBounds, &s.fat) {
			s.maskB.init(b, ir, clipBounds)
			s.ab = &s.maskB
			s.mask = &s.maskB
			s.fillPath(p, clipBounds, ir.T, ir.B, containedInClip, true, forceRLE)
			s.maskB.finish()
			s.mask = nil
		}
	} else {
		s.runB.init(b, ir, clipBounds, !p.isConvex())
		s.ab = &s.runB
		s.mask = nil
		s.fillPath(p, clipBounds, ir.T, ir.B, containedInClip, false, forceRLE)
		s.runB.finish()
	}
}

// safeRoundOut is safeRoundOut of SkScan_AntiPath.cpp.
func safeRoundOut(r Rect) IRect {
	dst := r.roundOut()
	const limit = maxS32 >> 2
	dst.intersect(IRect{-limit, -limit, limit, limit})
	return dst
}

// antiFillPath is SkScan::AntiFillPath for a rectangular clip. The path is
// in device space.
func (s *aaaState) antiFillPath(p *Path, clip IRect, b blitter) {
	if clip.isEmpty() || p.isEmpty() {
		return
	}
	ir := safeRoundOut(p.bounds())
	if ir.isEmpty() {
		return
	}
	clipped := ir
	if !clipped.intersect(clip) {
		return
	}
	// SkScanClipper: a path that the clip cuts horizontally draws through
	// a clipping blitter. The scan converter clips vertically by itself.
	if !clip.contains(ir) && (clip.L > ir.L || clip.R < ir.R) {
		s.rectClip = rectClipBlitter{b: b, clip: clip}
		b = &s.rectClip
	}
	s.aaaFillPath(p, b, ir, clip, false)
}
