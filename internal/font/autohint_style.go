// Ported from skrifa 0.46.2 (src/outline/autohint/style.rs).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afglobal.c, src/autofit/afstyles.h, src/autofit/afscript.h,
// src/autofit/afcover.h), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

// ahGlyphStyle is GlyphStyle: the script and the style of one glyph.
type ahGlyphStyle uint16

const (
	ahGlyphStyleIndexMask  ahGlyphStyle = 0xFF
	ahGlyphStyleUnassigned              = ahGlyphStyleIndexMask
	// ahGlyphStyleNonBase marks a character that is not a base character:
	// usually a mark.
	ahGlyphStyleNonBase ahGlyphStyle = 0x100
	ahGlyphStyleDigit   ahGlyphStyle = 0x200
	// ahGlyphStyleFromGsubOutput marks a glyph that is the output of a
	// substitution for the script that is in work.
	ahGlyphStyleFromGsubOutput ahGlyphStyle = 0x8000
)

func (s ahGlyphStyle) isUnassigned() bool {
	return s&ahGlyphStyleIndexMask == ahGlyphStyleUnassigned
}

func (s ahGlyphStyle) isNonBase() bool { return s&ahGlyphStyleNonBase != 0 }

func (s ahGlyphStyle) isDigit() bool { return s&ahGlyphStyleDigit != 0 }

// styleClass returns the style class of the glyph, or nil for a glyph
// without a style.
func (s ahGlyphStyle) styleClass() *ahStyleClass {
	ix, ok := s.styleIndex()
	if !ok || ix >= len(ahStyleClasses) {
		return nil
	}
	return &ahStyleClasses[ix]
}

func (s ahGlyphStyle) styleIndex() (int, bool) {
	ix := s & ahGlyphStyleIndexMask
	if ix != ahGlyphStyleUnassigned {
		return int(ix), true
	}
	return 0, false
}

// maybeAssign is maybe_assign. FreeType walks the styles in order, so the
// style with the lower index wins. The other bits stay, because FreeType
// lets a later style set the bit for a character that is not a base
// character.
func (s *ahGlyphStyle) maybeAssign(other ahGlyphStyle) {
	if other&ahGlyphStyleIndexMask <= *s&ahGlyphStyleIndexMask {
		*s = (*s &^ ahGlyphStyleIndexMask) | other
	}
}

func (s *ahGlyphStyle) setFromGsubOutput() { *s |= ahGlyphStyleFromGsubOutput }

func (s *ahGlyphStyle) clearFromGsub() { *s &^= ahGlyphStyleFromGsubOutput }

// maybeAssignGsubOutputStyle is maybe_assign_gsub_output_style: it assigns
// the style to a glyph that is the output of a substitution and has no
// style. It reports that it assigned the style.
func (s *ahGlyphStyle) maybeAssignGsubOutputStyle(style *ahStyleClass) bool {
	styleIx := ahGlyphStyle(style.index)
	if *s&ahGlyphStyleFromGsubOutput != 0 && s.isUnassigned() {
		s.clearFromGsub()
		*s = (*s &^ ahGlyphStyleIndexMask) | styleIx
		return true
	}
	return false
}

// ahUnmappedStyle marks a style without metrics in the metrics map.
const ahUnmappedStyle = 0xFF

// ahGlyphStyleMap is GlyphStyleMap: the style of each glyph of a font, and
// the styles that the font uses.
type ahGlyphStyleMap struct {
	// styles has the style of each glyph.
	styles []ahGlyphStyle
	// metricsMap maps the index of a style class to the index of its
	// metrics.
	metricsMap [len(ahStyleClasses)]uint8
	// metricsCount is the number of styles in use.
	metricsCount uint8
}

// newAHGlyphStyleMap is GlyphStyleMap::new: the styles of the glyphs of a
// font.
func newAHGlyphStyleMap(glyphCount uint32, shaper *ahShaper) *ahGlyphStyleMap {
	lookupCount := int(shaper.lookupCount())
	visitedSet := &ahVisitedLookupSet{storage: make([]byte, (lookupCount+7)/8)}
	m := &ahGlyphStyleMap{styles: make([]ahGlyphStyle, glyphCount)}
	for i := range m.styles {
		m.styles[i] = ahGlyphStyleUnassigned
	}
	for i := range m.metricsMap {
		m.metricsMap[i] = ahUnmappedStyle
	}
	// Step 1: the styles of the glyphs that an OpenType feature covers.
	for i := range ahStyleClasses {
		style := &ahStyleClasses[i]
		if style.hasFeature && shaper.computeCoverage(style, ahShaperCoverageScript, m.styles, visitedSet) {
			m.useStyle(style.index)
		}
	}
	// Step 2: the styles of the glyphs of the character map. The characters
	// come in order, so the range of the character before is tried first.
	haveLast := false
	var lastRange ahStyleRange
	shaper.font.charmap.mappings(func(ch, gid uint32) {
		if int64(gid) >= int64(len(m.styles)) {
			return
		}
		style := &m.styles[gid]
		if haveLast && lastRange.contains(ch) {
			style.maybeAssign(lastRange.style)
			return
		}
		ix, found := ahBinarySearchBy(len(ahStyleRanges), func(i int) int {
			switch first := ahStyleRanges[i].first; {
			case first < ch:
				return -1
			case first > ch:
				return 1
			}
			return 0
		})
		if !found {
			if ix > 0 {
				ix--
			}
		}
		if ix >= len(ahStyleRanges) {
			return
		}
		r := ahStyleRanges[ix]
		if r.contains(ch) {
			style.maybeAssign(r.style)
			if styleIx, ok := r.style.styleIndex(); ok {
				m.useStyle(styleIx)
			}
			haveLast, lastRange = true, r
		}
	})
	// Step 3a: the coverage of the scripts.
	for i := range ahStyleClasses {
		style := &ahStyleClasses[i]
		if !style.hasFeature && shaper.computeCoverage(style, ahShaperCoverageScript, m.styles, visitedSet) {
			m.useStyle(style.index)
		}
	}
	// Step 3b: the coverage of the default script, which is Latin in
	// FreeType.
	defaultStyle := &ahStyleClasses[ahStyleLatn]
	if shaper.computeCoverage(defaultStyle, ahShaperCoverageDefault, m.styles, visitedSet) {
		m.useStyle(defaultStyle.index)
	}
	// Step 4: all glyphs that remain get the style for CJK ideographs, as in
	// FreeType.
	needHani := false
	for i := range m.styles {
		if m.styles[i].isUnassigned() {
			m.styles[i] &^= ahGlyphStyleIndexMask
			m.styles[i] |= ahStyleHani
			needHani = true
		}
	}
	if needHani {
		m.useStyle(ahStyleHani)
	}
	// Step 5: mark the glyphs of the ASCII digits.
	for digitChar := uint32('0'); digitChar <= '9'; digitChar++ {
		if gid, ok := shaper.font.charmap.mapChar(digitChar); ok && int64(gid) < int64(len(m.styles)) {
			m.styles[gid] |= ahGlyphStyleDigit
		}
	}
	return m
}

// style returns the style of a glyph. ok is false for a glyph that the font
// does not have.
func (m *ahGlyphStyleMap) style(glyphID uint32) (ahGlyphStyle, bool) {
	if int64(glyphID) >= int64(len(m.styles)) {
		return 0, false
	}
	return m.styles[glyphID], true
}

// metricsIndex is metrics_index: the index of the metrics of a style.
func (m *ahGlyphStyleMap) metricsIndex(style ahGlyphStyle) (int, bool) {
	ix, ok := style.styleIndex()
	if !ok || ix >= len(m.metricsMap) {
		return 0, false
	}
	metricsIx := m.metricsMap[ix]
	if metricsIx == ahUnmappedStyle {
		return 0, false
	}
	return int(metricsIx), true
}

func (m *ahGlyphStyleMap) useStyle(styleIx int) {
	mapped := &m.metricsMap[styleIx]
	if *mapped == ahUnmappedStyle {
		// The style is seen for the first time.
		*mapped = m.metricsCount
		m.metricsCount++
	}
}

// ahScriptGroup is ScriptGroup: it selects the algorithms of the hinter for
// the glyphs of a script.
type ahScriptGroup uint8

const (
	// ahGroupDefault has all scripts that are not CJK or Indic. FreeType
	// names it Latin.
	ahGroupDefault ahScriptGroup = iota
	ahGroupCjk
	ahGroupIndic
)

// ahBlueString has the characters that define one alignment zone.
type ahBlueString struct {
	str   string
	zones ahBlueZones
}

// ahScriptClass is ScriptClass: the properties of a script.
type ahScriptClass struct {
	name  string
	group ahScriptGroup
	// tag is the Unicode tag of the script.
	tag uint32
	// hintTopToBottom says that the edges are processed from the top to
	// the bottom.
	hintTopToBottom bool
	// stdChars has the characters that define the standard width and
	// height of stems.
	stdChars string
	// blues has the characters that define the alignment zones.
	blues []ahBlueString
}

// ahStyleClass is StyleClass: the properties of a style. A style is a
// script, or the glyphs of a script that an OpenType feature covers.
type ahStyleClass struct {
	name   string
	index  int
	script *ahScriptClass
	// feature is the tag of the OpenType feature, when hasFeature is true.
	feature    uint32
	hasFeature bool
}

// ahStyleRange is StyleRange: the style of a range of characters.
type ahStyleRange struct {
	first, last uint32
	style       ahGlyphStyle
}

func (r ahStyleRange) contains(ch uint32) bool { return r.first <= ch && ch <= r.last }
