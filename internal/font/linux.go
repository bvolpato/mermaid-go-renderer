// Ported from Chromium 156.0.8078.80: the font selection of Blink
// (third_party/blink/renderer/platform/fonts/font_fallback_list.cc,
// font_selector.cc, font_cache.cc, alternate_font_family.h,
// font_fallback_iterator.cc, skia/font_cache_skia.cc,
// linux/font_cache_linux.cc, shaping/harfbuzz_face.cc) and the fontconfig
// interface of Skia at revision aa789d9b
// (src/ports/SkFontConfigInterface_direct.cpp; BSD 3-clause license,
// LICENSE-Skia.txt).
//
// The license of each Blink file is the license in its header. The headers
// are the same at Chromium 156.0.8078.80 and 156.0.8078.4:
//
//   - font_selector.cc and font_fallback_iterator.cc: BSD-3-Clause,
//     Copyright 2017 and 2015 The Chromium Authors (LICENSE-Chromium.txt).
//   - font_fallback_list.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (C) 2006 Apple Computer, Inc.  All rights reserved.
//   - font_cache.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (C) 2006, 2008 Apple Inc. All rights reserved.
//     Copyright (C) 2007 Nicholas Shanks <webkit@nickshanks.com>
//   - alternate_font_family.h: BSD-3-Clause, with the terms in the file.
//     Copyright (C) 2006, 2008 Apple Inc. All rights reserved.
//     Copyright (C) 2007 Nicholas Shanks <webkit@nickshanks.com>
//     Copyright (C) 2013 Google, Inc. All rights reserved.
//   - skia/font_cache_skia.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (c) 2006, 2007, 2008, 2009 Google Inc. All rights reserved.
//   - shaping/harfbuzz_face.cc: BSD-3-Clause, with the terms in the file.
//     Copyright (c) 2012 Google Inc. All rights reserved.
//   - linux/font_cache_linux.cc: BSD-2-Clause, with the terms in the file.
//     Copyright (C) 2012 Google Inc. All rights reserved.
//
// NOTICE-Blink.txt in this directory has the headers of the files that have
// the terms in the file, with the conditions and the disclaimers.

package font

import (
	"sort"
	"strings"
)

// This file has the font selection of Chromium on Linux.
//
// Blink asks Skia for a family by name, and Skia asks fontconfig.
// fontconfig always answers with a font: the best one it has. So Skia
// accepts the answer only when the font has the name that was asked for, or
// when the two names are in one class of metric-compatible fonts. A request
// for one of the names sans, serif and monospace takes any answer.
//
// internal/font/testdata/linux-family.json.gz has what Chromium gives for
// many names.

// linuxEquivClasses is kFontEquivMap of Skia: classes of fonts with the
// same metrics. A name is in the first class that lists it.
var linuxEquivClasses = [][]string{
	{"arial", "arimo", "liberation sans"},
	{"times new roman", "liberation serif", "tinos"},
	{"courier new", "cousine", "liberation mono"},
	{"symbol", "symbol neu"},
	{"ms pgothic", "ｍｓ ｐゴシック", "noto sans cjk jp", "ipapgothic", "motoyag04gothic"},
	{"ms gothic", "ｍｓ ゴシック", "noto sans mono cjk jp", "ipagothic", "motoyag04gothicmono"},
	{"ms pmincho", "ｍｓ ｐ明朝", "noto serif cjk jp", "ipapmincho", "motoyag04mincho"},
	{"ms mincho", "ｍｓ 明朝", "noto serif cjk jp", "ipamincho", "motoyag04minchomono"},
	{"simsun", "宋体", "noto serif cjk sc", "msung gb18030", "song asc"},
	{"nsimsun", "新宋体", "noto serif cjk sc", "msung gb18030", "n song asc"},
	{"simhei", "黑体", "noto sans cjk sc", "myingheigb18030", "myingheib5hk"},
	{"pmingliu", "新細明體", "noto serif cjk tc", "msung b5hk"},
	{"mingliu", "細明體", "noto serif cjk tc", "msung b5hk"},
	{"pmingliu_hkscs", "新細明體_hkscs", "noto serif cjk tc", "msung b5hk"},
	{"mingliu_hkscs", "細明體_hkscs", "noto serif cjk tc", "msung b5hk"},
	{"cambria", "caladea"},
	{"calibri", "carlito"},
}

// linuxEquivClass is GetFontEquivClass: the index of the class of a family
// name, or -1.
func linuxEquivClass(name string) int {
	for i, class := range linuxEquivClasses {
		for _, member := range class {
			if member == name {
				return i
			}
		}
	}
	return -1
}

// linuxFallbackAllowed is IsFallbackFontAllowed: a request for one of
// these names takes the font that fontconfig gives, whatever its name is.
func linuxFallbackAllowed(name string) bool {
	return name == "" || name == "sans" || name == "serif" || name == "monospace"
}

// linuxMetricAliases lists, for a family, the families that the default
// configuration of fontconfig gives in its place with the same strength,
// in the order of the configuration (30-metric-aliases.conf). Only the
// families that Skia can accept are here.
var linuxMetricAliases = map[string][]string{
	"arial":            {"arimo", "liberation sans", "albany", "albany amt"},
	"arimo":            {"arial", "liberation sans", "albany", "albany amt"},
	"liberation sans":  {"arial", "arimo", "albany", "albany amt"},
	"times new roman":  {"tinos", "liberation serif", "thorndale", "thorndale amt"},
	"tinos":            {"times new roman", "liberation serif", "thorndale", "thorndale amt"},
	"liberation serif": {"times new roman", "tinos", "thorndale", "thorndale amt"},
	"courier new":      {"cousine", "liberation mono", "cumberland", "cumberland amt"},
	"cousine":          {"courier new", "liberation mono", "cumberland", "cumberland amt"},
	"liberation mono":  {"courier new", "cousine", "cumberland", "cumberland amt"},
	"cambria":          {"caladea"},
	"caladea":          {"cambria"},
	"calibri":          {"carlito"},
	"carlito":          {"calibri"},
	"symbol":           {"symbolneu", "symbol neu"},
}

// linuxDefaultFamilies lists the families that the default configuration of
// fontconfig prefers for a request that names no installed family
// (60-latin.conf). A request for the name monospace gets the list of
// monospace, every other request gets the list of sans-serif
// (49-sansserif.conf), and a request for serif gets the list of serif.
var linuxDefaultFamilies = map[string][]string{
	"serif": {"noto serif", "dejavu serif", "times new roman", "thorndale amt", "luxi serif",
		"nimbus roman no9 l", "nimbus roman", "times"},
	"sans-serif": {"noto sans", "dejavu sans", "verdana", "arial", "albany amt", "luxi sans",
		"nimbus sans l", "nimbus sans", "helvetica", "lucida sans unicode", "bpg glaho international", "tahoma"},
	"monospace": {"dejavu sans mono", "noto mono", "noto sans mono", "inconsolata", "andale mono",
		"courier new", "cumberland amt", "luxi mono", "nimbus mono l", "nimbus mono", "nimbus mono ps", "courier"},
}

// fcWeight is the weight of fontconfig for a CSS weight
// (fcpattern_from_skfontstyle of Skia): linear between the named weights,
// cut to an integer.
func fcWeight(css int) int {
	ranges := [...][2]float32{
		{100, 0}, {200, 40}, {300, 50}, {350, 55}, {380, 75}, {400, 80}, {500, 100},
		{600, 180}, {700, 200}, {800, 205}, {900, 210}, {1000, 215},
	}
	val := float32(css)
	if val < ranges[0][0] {
		return int(ranges[0][1])
	}
	for i := 0; i < len(ranges)-1; i++ {
		if val < ranges[i+1][0] {
			oldMin, oldMax := ranges[i][0], ranges[i+1][0]
			newMin, newMax := ranges[i][1], ranges[i+1][1]
			return int(newMin + ((val - oldMin) * (newMax - newMin) / (oldMax - oldMin)))
		}
	}
	return int(ranges[len(ranges)-1][1])
}

// fcStyleDistance orders the faces for a request as FcFontSort does: the
// slant first, then the weight. A lower result is a better match.
func fcStyleDistance(f *faceRef, weight int, italic bool) int {
	slant := 0
	if f.italic != italic {
		slant = 100 // FC_SLANT_ITALIC - FC_SLANT_ROMAN
	}
	d := fcWeight(weight) - fcWeight(f.weight)
	if d < 0 {
		d = -d
	}
	return slant*1000 + d
}

// fcPickRef returns the face that fontconfig sorts first for a weight and
// a slant. Faces with the same distance keep the order of the font files,
// so the first one wins: for the weight 550, Liberation Sans Bold and
// Regular are equally far, and the file of Bold comes first.
func fcPickRef(refs []*faceRef, weight int, italic bool) *faceRef {
	var best *faceRef
	bestDist := 0
	for _, f := range refs {
		if d := fcStyleDistance(f, weight, italic); best == nil || d < bestDist {
			best, bestDist = f, d
		}
	}
	return best
}

// fileOrder sorts faces as fontconfig lists them: by file, and by index in
// a collection.
func fileOrder(refs []*faceRef) {
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].path != refs[j].path {
			return refs[i].path < refs[j].path
		}
		return refs[i].index < refs[j].index
	})
}

// linuxRefsOf returns the faces of an installed family in the order of the
// font files. The caller holds l.mu.
func (l *Library) linuxRefsOf(family string) []*faceRef {
	refs := l.refsOf(family)
	if len(refs) < 2 {
		return refs
	}
	sorted := append([]*faceRef(nil), refs...)
	fileOrder(sorted)
	return sorted
}

// linuxPick parses and returns the face of an installed family that
// fontconfig sorts first for a weight and a slant, or nil for a family
// that is not installed. first, when it is not nil, gets the family name of
// the first face of the family in the order of the font files.
func (l *Library) linuxPick(family string, weight int, italic bool, first *string) *Face {
	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		refs := l.linuxRefsOf(family)
		if len(refs) == 0 {
			return nil
		}
		if first != nil {
			*first = refs[0].family
		}
		pick := func(refs []*faceRef) *faceRef { return fcPickRef(refs, weight, italic) }
		if face := l.loadPicked(refs, pick); face != nil {
			return face
		}
		// No font of the family parses: see loadFamily.
	}
}

// linuxBestOf parses and returns the face with the best style among the
// faces of the families, in their order. A face with the same distance as
// an earlier one does not replace it. accept, when it is not nil, selects
// the faces that can be the answer: it gets the parsed face.
func (l *Library) linuxBestOf(families []string, weight int, italic bool, accept func(*Face) bool) *Face {
	l.mu.Lock()
	defer l.mu.Unlock()
	var refs []*faceRef
	for _, fam := range families {
		refs = append(refs, l.linuxRefsOf(fam)...)
	}
	// The order of the distances, and the order of the list for equal
	// distances: the first face that parses and is accepted is the face
	// that a scan of all parsed faces finds.
	sort.SliceStable(refs, func(i, j int) bool {
		return fcStyleDistance(refs[i], weight, italic) < fcStyleDistance(refs[j], weight, italic)
	})
	for _, ref := range refs {
		if face := l.faceOf(ref); face != nil && (accept == nil || accept(face)) {
			return face
		}
	}
	return nil
}

// linuxAllFamilies returns the lowercase names of all families of the
// library in the order of their first font file. The caller holds no lock.
func (l *Library) linuxAllFamilies() []string {
	l.mu.Lock()
	type entry struct{ family, path string }
	var entries []entry
	if len(l.dirs) > 0 {
		l.buildIndex()
		for _, e := range l.index {
			entries = append(entries, entry{strings.ToLower(e.Family), e.Path})
		}
	}
	l.mu.Unlock()
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	var names []string
	seen := map[string]bool{}
	for _, e := range entries {
		if !seen[e.family] {
			seen[e.family] = true
			names = append(names, e.family)
		}
	}
	if l.embed == embedFonts {
		// The embedded files come after the directories, in the order of
		// their names: Mono, Sans, Serif.
		for _, fam := range []string{"liberation mono", "liberation sans", "liberation serif"} {
			if !seen[fam] {
				seen[fam] = true
				names = append(names, fam)
			}
		}
	}
	return names
}

// fcDefault returns the face that fontconfig gives for a request that names
// no installed family. generic is the generic family of the request:
// "serif", "sans-serif" or "monospace".
//
// fontconfig takes the first installed family of the preference list of
// the generic family. When none is installed, all fonts match the request
// equally, and the first font file with the best style wins. With only the
// Liberation fonts that is Liberation Mono, for every generic family.
func (l *Library) fcDefault(generic string, weight int, italic bool) *Face {
	if s := l.fontconfig(); s != nil {
		return s.matchFace(generic, weight, italic)
	}
	for _, fam := range linuxDefaultFamilies[generic] {
		if face := l.linuxPick(fam, weight, italic, nil); face != nil {
			return face
		}
	}
	return l.linuxBestOf(l.linuxAllFamilies(), weight, italic, nil)
}

// fcMatch is SkFontConfigInterfaceDirect::matchFamilyName: the face that
// fontconfig gives for a family name, or nil when Skia does not accept it.
// name is lowercase.
func (l *Library) fcMatch(name string, weight int, italic bool) *Face {
	if s := l.fontconfig(); s != nil {
		// A library with font directories follows the configuration of
		// fontconfig. See fontconfig_match.go.
		return s.matchFace(name, weight, italic)
	}
	if face := l.linuxPick(name, weight, italic, nil); face != nil {
		return face
	}
	if linuxFallbackAllowed(name) {
		generic := "sans-serif"
		if name == "serif" || name == "monospace" {
			generic = name
		}
		return l.fcDefault(generic, weight, italic)
	}
	// The family is not installed. Skia accepts the answer of fontconfig
	// only when it is in the class of the name.
	class := linuxEquivClass(name)
	if class < 0 {
		return nil
	}
	for _, alias := range linuxMetricAliases[name] {
		var family string
		face := l.linuxPick(alias, weight, italic, &family)
		if face == nil {
			continue
		}
		// The first installed alias is the answer of fontconfig.
		if linuxEquivClass(strings.ToLower(family)) != class {
			return nil
		}
		return face
	}
	return nil
}

// linuxAlternateFamily is AlternateFamilyName of Blink: the name that Blink
// tries when a family gives no font.
func linuxAlternateFamily(name string) string {
	switch name {
	case "courier":
		return "courier new"
	case "courier new":
		return "courier"
	case "times":
		return "times new roman"
	case "times new roman":
		return "times"
	case "arial":
		return "helvetica"
	case "helvetica":
		return "arial"
	}
	return ""
}

// linuxFamily returns the face that Chromium on Linux uses for a family
// name, or nil when it has no font for the name
// (FontCache::GetFontPlatformData).
func (l *Library) linuxFamily(family string, weight int, italic bool) *Face {
	name := strings.ToLower(strings.TrimSpace(family))
	if name == "system-ui" {
		// The name gives the system font, with or without quotes. The
		// system font of a headless browser is the default font of
		// fontconfig.
		if s := l.fontconfig(); s != nil {
			return s.systemFont(weight, italic)
		}
		return l.fcDefault("sans-serif", weight, italic)
	}
	if f := l.fcMatch(name, weight, italic); f != nil {
		return f
	}
	if alt := linuxAlternateFamily(name); alt != "" {
		return l.fcMatch(alt, weight, italic)
	}
	return nil
}

// linuxStandardFamily is the standard font of the settings of Chromium.
const linuxStandardFamily = "times new roman"

// linuxGeneric returns the face of a CSS generic family: the family of the
// settings of Chromium for it (FontSelector::FamilyNameFromSettings). The
// result is nil for a generic family that Chromium on Linux does not have,
// such as ui-serif.
func (l *Library) linuxGeneric(name string, weight int, italic bool) *Face {
	switch name {
	case "serif":
		return l.linuxFamily("times new roman", weight, italic)
	case "sans-serif":
		return l.linuxFamily("arial", weight, italic)
	case "monospace":
		// The setting is the name Monospace, which fontconfig takes as a
		// request for its default monospace font.
		return l.linuxFamily("monospace", weight, italic)
	case "cursive":
		return l.linuxFamily("comic sans ms", weight, italic)
	case "fantasy":
		return l.linuxFamily("impact", weight, italic)
	case "math":
		return l.linuxFamily("latin modern math", weight, italic)
	case "emoji":
		return l.linuxFamily("noto color emoji", weight, italic)
	case "system-ui":
		return l.linuxFamily("system-ui", weight, italic)
	}
	return nil
}

// linuxChain returns the faces of a font-family list on Linux
// (FontFallbackList::GetFontData).
func (l *Library) linuxChain(families []string, weight int, italic bool) []*Face {
	var faces []*Face
	seen := map[*Face]bool{}
	add := func(f *Face) {
		if f != nil && !seen[f] {
			seen[f] = true
			faces = append(faces, f)
		}
	}
	for _, fam := range families {
		if name, quoted := unquoteFamily(fam); quoted {
			// A quoted name is a family name, also when it is the keyword
			// of a generic family: "serif" asks fontconfig for the name
			// serif, and the keyword serif asks for Times New Roman.
			add(l.linuxFamily(name, weight, italic))
			continue
		}
		lower := strings.ToLower(fam)
		if isGenericFamily(lower) {
			add(l.linuxGeneric(lower, weight, italic))
			continue
		}
		add(l.linuxFamily(fam, weight, italic))
	}
	if len(faces) == 0 {
		// No family gives a font: the standard font of the settings.
		add(l.linuxFamily(linuxStandardFamily, weight, italic))
	}
	if len(faces) == 0 {
		// FontCache::GetLastResortFallbackFont: the name sans.
		add(l.fcDefault("sans", weight, italic))
	}
	if len(faces) == 0 {
		add(l.lastResort(weight, italic))
	}
	return faces
}

// linuxFaceFor returns the face that renders a character on Linux
// (FontFallbackIterator). A character that no font has is drawn with the
// missing glyph of the first face.
func (c *Chain) linuxFaceFor(r rune) *Face {
	for _, f := range c.faces {
		if f.HasGlyph(r) {
			return f
		}
	}
	if r == 0x2028 || r == 0x2029 {
		// Blink draws a line separator and a paragraph separator that the
		// font does not have with the space glyph of the font.
		return c.Primary()
	}
	if linuxNoFallback(r) {
		return c.Primary()
	}
	l := c.lib
	bold := c.weight >= 600
	if (bold || c.italic) && c.first != "" {
		// FontCache::FallbackOnStandardFontStyle: for a bold or italic
		// text Blink first tries the regular face of the first family.
		// Blink makes that face bold or italic by synthesis, which this
		// engine does not do.
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
	if found != nil {
		return found
	}
	return c.Primary()
}

// fcCharFallback is gfx::GetFallbackFontForChar: the font that fontconfig
// gives for a character. The request has no family, no weight and no
// slant, so the answer is the first regular face, in the order of the
// default font, that has the character. Blink makes the face bold or
// italic by synthesis when the text is bold or italic.
func (l *Library) fcCharFallback(r rune) *Face {
	if s := l.fontconfig(); s != nil {
		return s.charFallback(r, false)
	}
	hasGlyph := func(f *Face) bool { return f.HasGlyph(r) }
	for _, fam := range linuxDefaultFamilies["sans-serif"] {
		if face := l.linuxBestOf([]string{fam}, 400, false, hasGlyph); face != nil {
			return face
		}
	}
	return l.linuxBestOf(l.linuxAllFamilies(), 400, false, hasGlyph)
}

// linuxShapeText returns the text that HarfBuzz shapes with a face on
// Linux. Blink gives the glyph of a space to a line separator and to a
// paragraph separator when the font has no glyph for them
// (HarfBuzzGetGlyph in harfbuzz_face.cc).
func (f *Face) linuxShapeText(text []rune) []rune {
	for i, r := range text {
		if (r == 0x2028 || r == 0x2029) && !f.HasGlyph(r) {
			out := append([]rune(nil), text...)
			for j := i; j < len(out); j++ {
				if (out[j] == 0x2028 || out[j] == 0x2029) && !f.HasGlyph(out[j]) {
					out[j] = ' '
				}
			}
			return out
		}
	}
	return text
}

// linuxNoFallback reports whether Blink looks for no fallback font for a
// character: a private use character or a noncharacter
// (FontCache::FallbackFontForCharacter). Such a character gets the missing
// glyph of the first face.
func linuxNoFallback(r rune) bool {
	switch {
	case r >= 0xe000 && r <= 0xf8ff, r >= 0xf0000 && r <= 0xffffd, r >= 0x100000 && r <= 0x10fffd:
		return true
	case r >= 0xfdd0 && r <= 0xfdef, r&0xfffe == 0xfffe:
		return true
	}
	return false
}
