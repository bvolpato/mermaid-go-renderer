// Ported from Chromium 156.0.8078.80 (Blink): the font faces of a page and
// the choice of a face for a font-family value
// (third_party/blink/renderer/core/css/font_face_cache.cc,
// css_segmented_font_face.cc, css_font_selector.cc,
// third_party/blink/renderer/platform/fonts/font_selection_algorithm.cc,
// font_custom_platform_data.cc, font_fallback_list.cc and
// font_fallback_iterator.cc).
//
// The license of each Blink file is the license in its header:
//
//   - font_face_cache.cc: BSD-2-Clause, with the terms in the file.
//     Copyright (C) 2007, 2008, 2011 Apple Inc. All rights reserved.
//     Copyright (C) 2013 Google Inc. All rights reserved.
//   - css_segmented_font_face.cc: BSD-2-Clause, with the terms in the file.
//     Copyright (C) 2008 Apple Inc. All rights reserved.
//   - css_font_selector.cc: BSD-2-Clause, with the terms in the file.
//     Copyright (C) 2007, 2008, 2011 Apple Inc. All rights reserved.
//   - font_selection_algorithm.cc: BSD-2-Clause, with the terms in the
//     file. Copyright (C) 2017 Apple Inc. All rights reserved.
//   - font_custom_platform_data.cc: BSD-3-Clause, with the terms in the
//     file. Copyright (C) 2007 Apple Computer, Inc. Copyright (c) 2007,
//     2008, 2009, Google Inc. All rights reserved. Copyright (C) 2010
//     Company 100, Inc.
//   - font_fallback_list.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (C) 2006 Apple Computer, Inc.  All rights reserved.
//   - font_fallback_iterator.cc: BSD-3-Clause, Copyright 2015 The Chromium
//     Authors (LICENSE-Chromium.txt).
//
// NOTICE-Blink.txt in this directory has the headers of the files that have
// the terms in the file.

package font

import (
	"path"
	"strings"
	"sync"

	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// This file has the web fonts of the page of mermaid-cli: the @font-face
// rules of the style sheets of internal/webfonts, as the font selector of
// Blink keeps and uses them.
//
// A family of web fonts is a set of faces. Faces with the same style and
// weight descriptors are one segmented face: each of its fonts has the
// characters of a unicode-range. A text gets the segmented face that is
// closest to its weight and slant (FontSelectionAlgorithm), and each
// character gets the first font of that face whose range has the character
// and that has a glyph for it. A later rule comes before an earlier rule.
// A character that no font of the family has goes to the next family of
// the font-family list, and at the end to the fallback of the platform.
//
// The page of mermaid-cli loads every font face before it draws, so a face
// is never in the state of a font that still loads.

// italicSlope is kItalicSlopeValue of Blink: the slope of font-style
// italic, and of oblique without an angle, in degrees.
const italicSlope = 14

// boldThreshold is kBoldThreshold of Blink.
const boldThreshold = 600

// webSegmented is CSSSegmentedFontFace: the rules of a family that have the
// same capabilities.
type webSegmented struct {
	slopeMin, slopeMax   float64
	weightMin, weightMax float64
	// rules has the rules in the order of the style sheets.
	rules []*webRule
}

// webFamily is the CapabilitiesSet of a family name.
type webFamily struct {
	sets []*webSegmented
}

// webFontSet is FontFaceCache: the font faces of a page by family name.
type webFontSet struct {
	// families is keyed by the family name in lower case: Blink compares
	// the names without regard to case.
	families map[string]*webFamily
}

var (
	pageFontsOnce sync.Once
	pageFonts     *webFontSet
)

// pageWebFonts returns the font faces of the page of mermaid-cli. It reads
// the style sheets one time.
func pageWebFonts() *webFontSet {
	pageFontsOnce.Do(func() {
		set := &webFontSet{families: map[string]*webFamily{}}
		for _, sheet := range webfonts.Sheets {
			text, err := webfonts.FS.ReadFile(sheet.Path)
			if err != nil || !strings.Contains(string(text), "@font-face") {
				continue
			}
			for _, rule := range parseFontFaceRules(string(text)) {
				rule.url = path.Join(path.Dir(sheet.Path), rule.url)
				set.add(&rule)
			}
		}
		pageFonts = set
	})
	return pageFonts
}

// add is FontFaceCache::AddFontFace.
func (s *webFontSet) add(rule *webRule) {
	key := strings.ToLower(rule.family)
	family := s.families[key]
	if family == nil {
		family = &webFamily{}
		s.families[key] = family
	}
	for _, set := range family.sets {
		if set.slopeMin == rule.slopeMin && set.slopeMax == rule.slopeMax &&
			set.weightMin == rule.weightMin && set.weightMax == rule.weightMax {
			set.rules = append(set.rules, rule)
			return
		}
	}
	family.sets = append(family.sets, &webSegmented{
		slopeMin: rule.slopeMin, slopeMax: rule.slopeMax,
		weightMin: rule.weightMin, weightMax: rule.weightMax,
		rules: []*webRule{rule},
	})
}

// fontSelection is FontSelectionAlgorithm: it compares the capabilities of
// the segmented faces of a family for a request. The widths are not here:
// every rule of the page has the normal width.
type fontSelection struct {
	weight, slope float64
	// The bounds of the capabilities of all faces of the family.
	slopeMin, slopeMax   float64
	weightMin, weightMax float64
}

// styleDistance is FontSelectionAlgorithm::StyleDistance.
func (a *fontSelection) styleDistance(c *webSegmented) float64 {
	const italicThreshold = 14 // kItalicThreshold
	if c.slopeMin <= a.slope && a.slope <= c.slopeMax {
		return 0
	}
	if a.slope >= italicThreshold {
		if c.slopeMin > a.slope {
			return c.slopeMin - a.slope
		}
		threshold := max(a.slope, a.slopeMax)
		return threshold - c.slopeMax
	}
	if a.slope >= 0 {
		if c.slopeMax >= 0 && c.slopeMax < a.slope {
			return a.slope - c.slopeMax
		}
		if c.slopeMin > a.slope {
			return c.slopeMin
		}
		threshold := max(a.slope, a.slopeMax)
		return threshold - c.slopeMax
	}
	if a.slope > -italicThreshold {
		if c.slopeMin > a.slope && c.slopeMin <= 0 {
			return c.slopeMin - a.slope
		}
		if c.slopeMax < a.slope {
			return -c.slopeMax
		}
		threshold := min(a.slope, a.slopeMin)
		return c.slopeMin - threshold
	}
	if c.slopeMax < a.slope {
		return a.slope - c.slopeMax
	}
	threshold := min(a.slope, a.slopeMin)
	return c.slopeMin - threshold
}

// weightDistance is FontSelectionAlgorithm::WeightDistance.
func (a *fontSelection) weightDistance(c *webSegmented) float64 {
	const lowerThreshold, upperThreshold = 400, 500
	if c.weightMin <= a.weight && a.weight <= c.weightMax {
		return 0
	}
	if a.weight >= lowerThreshold && a.weight <= upperThreshold {
		if c.weightMin > a.weight && c.weightMin <= upperThreshold {
			return c.weightMin - a.weight
		}
		if c.weightMax < a.weight {
			return upperThreshold - c.weightMax
		}
		threshold := min(a.weight, a.weightMin)
		return c.weightMin - threshold
	}
	if a.weight < lowerThreshold {
		if c.weightMax < a.weight {
			return a.weight - c.weightMax
		}
		threshold := min(a.weight, a.weightMin)
		return c.weightMin - threshold
	}
	if c.weightMin > a.weight {
		return c.weightMin - a.weight
	}
	threshold := max(a.weight, a.weightMax)
	return threshold - c.weightMax
}

// isBetterMatch is FontSelectionAlgorithm::IsBetterMatchForRequest.
func (a *fontSelection) isBetterMatch(first, second *webSegmented) bool {
	if d1, d2 := a.styleDistance(first), a.styleDistance(second); d1 != d2 {
		return d1 < d2
	}
	return a.weightDistance(first) < a.weightDistance(second)
}

// pick is FontFaceCache::FontSelectionQueryResult::GetOrCreate: the
// segmented face of the family for a weight and a slant.
//
// Blink keeps the faces of a family in a hash map and takes the first one
// when two are equally far from the request. Here the first one in the
// order of the style sheets wins. The families of the page have no such
// pair for the weights 1 to 1000.
func (f *webFamily) pick(weight int, italic bool) *webSegmented {
	a := fontSelection{weight: float64(weight)}
	if italic {
		a.slope = italicSlope
	}
	for i, set := range f.sets {
		if i == 0 {
			a.slopeMin, a.slopeMax = set.slopeMin, set.slopeMax
			a.weightMin, a.weightMax = set.weightMin, set.weightMax
			continue
		}
		a.slopeMin, a.slopeMax = min(a.slopeMin, set.slopeMin), max(a.slopeMax, set.slopeMax)
		a.weightMin, a.weightMax = min(a.weightMin, set.weightMin), max(a.weightMax, set.weightMax)
	}
	var best *webSegmented
	for _, set := range f.sets {
		if best == nil || a.isBetterMatch(set, best) {
			best = set
		}
	}
	return best
}

// WithWebFonts returns a library that has the @font-face rules of the page
// of mermaid-cli (internal/webfonts) in front of the fonts of l: the
// families Recursive Variable, Open Sans and Font Awesome. The platform
// rules are those of l, and a family that is not a web font gives the font
// that l gives.
//
// The result is made one time for a library. A font file is read when a
// text first uses a face of it, one time for the process.
func (l *Library) WithWebFonts() *Library {
	if l.base != nil {
		return l
	}
	l.webOnce.Do(func() {
		l.webLib = l.newWebLibrary(true)
	})
	return l.webLib
}

// WithWebFontsInChromium returns the library of WithWebFonts with the
// generic families and the standard font of l in place of those of the
// headless shell (see shellGenericFamily). It is the page of
// tools/reference/oracle.mjs --webfonts, which runs in Chromium: the tests
// that replay probes of that page use it. A render of the page of
// mermaid-cli uses WithWebFonts.
func (l *Library) WithWebFontsInChromium() *Library {
	if l.base != nil {
		return l
	}
	l.webChromiumOnce.Do(func() {
		l.webChromiumLib = l.newWebLibrary(false)
	})
	return l.webChromiumLib
}

// newWebLibrary makes a library with web fonts on top of l. shell selects
// the generic families of the headless shell.
func (l *Library) newWebLibrary(shell bool) *Library {
	w := newLibrary(l.platform, l.embed, l.dirs)
	w.fcDir, w.fcSystemConfig = l.fcDir, l.fcSystemConfig
	w.base = l
	w.webShell = shell
	w.webFaces = map[webFaceKey]*Face{}
	return w
}

// chainEntry is one font of the list of a chain of a library with web
// fonts: FontDataForRangeSet of Blink.
type chainEntry struct {
	// ranges is the unicode-range of a web font, or nil for a font that
	// has every character.
	ranges *rangeSet
	// face is the face of a system font.
	face *Face
	// ref is the face of a web font. Its font is read on first use.
	ref *webFaceRef
	// standard is the standard font of the browser at the end of the list.
	// The library looks for it on first use.
	standard *standardFace
}

// standardFace is the standard font of the settings of the browser for a
// weight and a slant, found when a text first needs it.
type standardFace struct {
	once sync.Once
	find func() *Face
	face *Face
}

// get returns the face of the entry, or nil for a font that does not
// load.
func (e *chainEntry) get() *Face {
	if e.ref != nil {
		return e.ref.get()
	}
	if e.standard != nil {
		e.standard.once.Do(func() { e.standard.face = e.standard.find() })
		return e.standard.face
	}
	return e.face
}

// resolveWeb builds the chain of a font-family list for a library with web
// fonts (FontFallbackList::GetFontData with the font selector of the
// page).
func (l *Library) resolveWeb(families []string, weight int, italic bool) *Chain {
	base := l.base
	c := &Chain{lib: base, weight: weight, italic: italic, web: true}
	if len(families) > 0 {
		c.first, _ = unquoteFamily(families[0])
	}
	seen := map[*Face]bool{}
	add := func(f *Face) {
		if f != nil && !seen[f] {
			seen[f] = true
			c.entries = append(c.entries, chainEntry{face: f})
		}
	}
	set := pageWebFonts()
	for _, fam := range families {
		name, quoted := unquoteFamily(fam)
		lower := strings.ToLower(name)
		// CSSFontSelector::GetFontData: a name that is not the keyword of
		// a generic family gives the web font of that name.
		if quoted || !isGenericFamily(lower) {
			if family := set.families[lower]; family != nil {
				seg := family.pick(weight, italic)
				// CSSSegmentedFontFace::GetFontData: a later rule comes
				// first.
				for i := len(seg.rules) - 1; i >= 0; i-- {
					rule := seg.rules[i]
					c.entries = append(c.entries, chainEntry{ranges: rule.ranges, ref: l.webFaceRef(rule, seg, weight, italic)})
				}
				continue
			}
		}
		add(base.familyFace(fam, weight, italic, l.webShell))
	}
	// After the families of the list comes the standard font of the
	// settings, and a last resort when that is missing too. A chain that
	// has no other font needs it at once, as its primary font.
	c.entries = append(c.entries, chainEntry{standard: &standardFace{find: func() *Face {
		if faces := base.standardFaces(weight, italic, l.webShell); len(faces) > 0 {
			return faces[0]
		}
		return nil
	}}})
	return c
}

// The page of mermaid-cli runs in the headless shell of Chromium
// (puppeteer with headless: "shell"), and not in the Chromium that
// oracle.mjs runs. The headless shell has no font preferences of its own:
// the generic families and the standard font are the defaults of Blink
// (WebPreferences in third_party/blink/common/web_preferences/
// web_preferences.cc). Chromium has other fonts for them on macOS (Times,
// Helvetica, Courier) and for two of them on Linux.
//
// The reference of mmdc on macOS shows it: a text without a font family at
// 16px has the line box of Times New Roman, 17px high, and in the page of
// oracle.mjs the line box of Times, 18px high.

// shellGenericFamily returns the family of a generic family in the
// headless shell: the defaults of WebPreferences. ok is false for a generic
// family that has no default there.
func shellGenericFamily(platform Platform, name string) (family string, ok bool) {
	switch name {
	case "serif":
		return "Times New Roman", true
	case "sans-serif":
		return "Arial", true
	case "monospace":
		if platform == PlatformDarwin {
			return "Menlo", true
		}
		return "Courier New", true
	case "cursive":
		return "Script", true
	case "fantasy":
		return "Impact", true
	}
	return "", false
}

// shellStandardFamily is the standard font of the headless shell.
const shellStandardFamily = "Times New Roman"

// familyFace returns the face of the system for one name of a font-family
// list in a page with web fonts, or nil: the step of Resolve and of
// linuxChain for one name. shell selects the generic families of the
// headless shell.
func (l *Library) familyFace(fam string, weight int, italic, shell bool) *Face {
	name, quoted := unquoteFamily(fam)
	lower := strings.ToLower(name)
	if !quoted && isGenericFamily(lower) {
		if family, ok := shellGenericFamily(l.platform, lower); ok && shell {
			return l.Match(family, weight, italic)
		}
		if l.platform == PlatformLinux {
			return l.linuxGeneric(lower, weight, italic)
		}
		for _, g := range l.genericFamily(lower) {
			if f := l.Match(g, weight, italic); f != nil {
				return f
			}
		}
		return nil
	}
	if l.platform == PlatformLinux {
		return l.linuxFamily(name, weight, italic)
	}
	return l.Match(name, weight, italic)
}

// standardFaces returns the faces that end the font list of a page with
// web fonts: the standard font of the browser, which
// FontFallbackList::GetFontData adds after the last family of the list.
// shell selects the standard font of the headless shell. The fonts that
// Resolve and linuxChain take for a list that gives no font follow, for a
// machine without the standard font.
func (l *Library) standardFaces(weight int, italic, shell bool) []*Face {
	var faces []*Face
	add := func(f *Face) {
		if f != nil && len(faces) == 0 {
			faces = append(faces, f)
		}
	}
	if shell || l.platform == PlatformLinux {
		// The standard font of Chromium on Linux is Times New Roman too.
		add(l.Match(shellStandardFamily, weight, italic))
	}
	if l.platform == PlatformLinux {
		if len(faces) == 0 {
			add(l.fcDefault("sans", weight, italic))
		}
	} else {
		for _, generic := range []string{"serif", "sans-serif"} {
			for _, g := range l.genericFamily(generic) {
				add(l.Match(g, weight, italic))
			}
		}
	}
	if len(faces) == 0 {
		add(l.lastResort(weight, italic))
	}
	return faces
}

// webPrimary is FontFallbackList::DeterminePrimarySimpleFontData for the
// space character: the first font of the list whose range has the space.
// The font does not need a glyph for it.
func (c *Chain) webPrimary() *Face {
	c.primaryOnce.Do(func() {
		for i := range c.entries {
			e := &c.entries[i]
			if !e.ranges.contains(' ') {
				continue
			}
			if f := e.get(); f != nil {
				c.primary = f
				return
			}
		}
		// No font has the range of the space: the first font that loads.
		for i := range c.entries {
			if f := c.entries[i].get(); f != nil {
				c.primary = f
				return
			}
		}
	})
	return c.primary
}

// hasGlyphFor reports whether Blink finds a glyph for a character in a
// font of the list: HarfBuzzGetGlyph. A character outside the range of a
// web font has no glyph there. A line separator and a paragraph separator
// get the glyph of the space.
func (e *chainEntry) hasGlyphFor(r rune) *Face {
	if !e.ranges.contains(r) {
		return nil
	}
	f := e.get()
	if f == nil {
		return nil
	}
	if f.HasGlyph(r) {
		return f
	}
	if e.ref != nil && (r == 0x2028 || r == 0x2029) && f.HasGlyph(' ') {
		return f
	}
	return nil
}

// webFirstCandidate is the first font that FontFallbackIterator returns
// for a run of text: the font that draws the missing glyph of a character
// that no font has. A web font is a candidate when its range has a
// character of the run.
func (c *Chain) webFirstCandidate(run []rune) *Face {
	for i := range c.entries {
		e := &c.entries[i]
		if e.ranges != nil {
			contributes := false
			for _, r := range run {
				if e.ranges.contains(r) {
					contributes = true
					break
				}
			}
			if !contributes {
				continue
			}
		}
		if f := e.get(); f != nil {
			return f
		}
	}
	return c.webPrimary()
}

// webFaceFor returns the face that renders a character for a chain with
// web fonts. notdef is the face that draws a character that no font has.
func (c *Chain) webFaceFor(r rune, notdef *Face) *Face {
	for i := range c.entries {
		if f := c.entries[i].hasGlyphFor(r); f != nil {
			return f
		}
	}
	if f := c.systemFaceFor(r); f != nil {
		return f
	}
	return notdef
}

// systemFaceFor returns the fallback font of the platform for a character
// that no font of the list has, or nil: the steps of FaceFor and of
// linuxFaceFor after the list.
func (c *Chain) systemFaceFor(r rune) *Face {
	l := c.lib
	bold := c.weight >= boldThreshold
	if l.platform != PlatformLinux {
		return l.systemFallback(r, bold, c.italic)
	}
	if r == 0x2028 || r == 0x2029 || linuxNoFallback(r) {
		return nil
	}
	if (bold || c.italic) && c.first != "" {
		if f := l.linuxFamily(c.first, 400, false); f != nil && f.HasGlyph(r) {
			return f
		}
	}
	key := fallbackKey{r, bold, c.italic}
	l.mu.Lock()
	found, ok := l.fallback[key]
	l.mu.Unlock()
	if !ok {
		found = l.fcCharFallback(r)
		l.mu.Lock()
		l.fallback[key] = found
		l.mu.Unlock()
	}
	return found
}
