// Ported from skrifa 0.46.2 (src/outline/autohint/shape.rs) and from
// read-fonts 0.43.3 (src/tables/gsub.rs, src/tables/layout.rs,
// src/tables/layout/script.rs, src/tables/layout/feature.rs, and the
// generated readers of the GSUB table).
//
// skrifa, read-fonts and font-types have the license MIT OR Apache-2.0
// (LICENSE-skrifa.txt). skrifa ported this code from FreeType 2.13.2
// (src/autofit/afshaper.c), which has the FreeType License
// (LICENSE-FreeType.txt).

package font

import "unicode/utf8"

// ahMaxNestingDepth stops the recursion of contextual lookups. It is the
// limit of HarfBuzz.
const ahMaxNestingDepth = 64

// The work limit of a shaper is the largest number of steps that it takes
// in the GSUB table: ahGsubWorkBase steps, and ahGsubWorkPerByte steps for
// each byte of the table. skrifa has no such limit. A font with damaged
// offsets can make the tables of a lookup overlap, and then the walk over
// all lookups takes a time that grows with the product of the counts of
// each level. The fonts of the tests take less than one step for each byte
// of the table, for all styles together. So the limit changes nothing for a
// font that skrifa handles in a sensible time.
const (
	ahGsubWorkBase    = 1 << 16
	ahGsubWorkPerByte = 256
)

// ahShaperMode is ShaperMode: how much shaping the hinter applies. Shaping
// affects the styles of glyphs and the glyphs that the metrics come from.
type ahShaperMode uint8

const (
	// ahShaperNominal maps characters to nominal glyphs and does not use
	// the layout tables.
	ahShaperNominal ahShaperMode = iota
	// ahShaperBestEffort applies simple substitutions, and extends the
	// coverage of styles with the layout tables. skrifa uses this mode
	// when it has the feature autohint_shaping, which is a default
	// feature.
	ahShaperBestEffort
)

// ahShapedGlyph is ShapedGlyph.
type ahShapedGlyph struct {
	id      uint32
	yOffset int32
}

// ahShaperCoverageKind is ShaperCoverageKind.
type ahShaperCoverageKind uint8

const (
	// ahShaperCoverageScript is the coverage of one script.
	ahShaperCoverageScript ahShaperCoverageKind = iota
	// ahShaperCoverageDefault includes the script DFLT also.
	ahShaperCoverageDefault
)

// ahShaper is Shaper: it maps characters to glyphs, and it extends the
// coverage of a style to glyphs that are not in the character map.
type ahShaper struct {
	font *ahFont
	mode ahShaperMode
	// gsub is the GSUB table. hasGsub is false in the nominal mode and for
	// a font without the table.
	gsub    ahData
	hasGsub bool
	// work is the number of steps in the GSUB table that remain (see
	// ahGsubWorkBase).
	work int
}

func newAHShaper(font *ahFont, mode ahShaperMode) *ahShaper {
	s := &ahShaper{font: font, mode: mode}
	if mode != ahShaperNominal && font.hasGsub {
		s.gsub, s.hasGsub = font.gsub, true
		s.work = ahGsubWorkBase + ahGsubWorkPerByte*len(font.gsub)
	}
	return s
}

// spend takes one step of the work limit. It returns false when no step
// remains.
func (s *ahShaper) spend() bool {
	if s.work <= 0 {
		return false
	}
	s.work--
	return true
}

// The lists of the GSUB table.
func (s *ahShaper) scriptList() (ahData, bool) {
	if !s.hasGsub {
		return nil, false
	}
	return s.gsub.resolve(int(s.gsub.u16(4)), 2)
}

func (s *ahShaper) featureList() (ahData, bool) {
	if !s.hasGsub {
		return nil, false
	}
	return s.gsub.resolve(int(s.gsub.u16(6)), 2)
}

func (s *ahShaper) lookupList() (ahData, bool) {
	if !s.hasGsub {
		return nil, false
	}
	return s.gsub.resolve(int(s.gsub.u16(8)), 2)
}

func (s *ahShaper) lookupCount() uint16 {
	list, ok := s.lookupList()
	if !ok {
		return 0
	}
	return list.u16(0)
}

// ahScriptTagsFromUnicode is ScriptTags::from_unicode: the OpenType script
// tags for a Unicode script, in the order of preference.
func ahScriptTagsFromUnicode(unicodeScript uint32) []uint32 {
	var tags []uint32
	// The scripts that have a second OpenType tag
	// (UNICODE_TO_NEW_OPENTYPE_SCRIPT_TAGS).
	var newTag uint32
	switch unicodeScript {
	case ahTag("Beng"):
		newTag = ahTag("bng2")
	case ahTag("Deva"):
		newTag = ahTag("dev2")
	case ahTag("Gujr"):
		newTag = ahTag("gjr2")
	case ahTag("Guru"):
		newTag = ahTag("gur2")
	case ahTag("Knda"):
		newTag = ahTag("knd2")
	case ahTag("Mlym"):
		newTag = ahTag("mlm2")
	case ahTag("Mymr"):
		newTag = ahTag("mym2")
	case ahTag("Orya"):
		newTag = ahTag("ory2")
	case ahTag("Taml"):
		newTag = ahTag("tml2")
	case ahTag("Telu"):
		newTag = ahTag("tel2")
	}
	if newTag != 0 {
		// Myanmar has mym2, and there is no mym3.
		if newTag != ahTag("mym2") {
			tags = append(tags, newTag&^0xFF|'3')
		}
		tags = append(tags, newTag)
	}
	// old_tag_from_unicode.
	var oldTag uint32
	switch unicodeScript {
	case ahTag("Zmth"):
		oldTag = ahTag("math")
	case ahTag("Hira"):
		oldTag = ahTag("kana")
	case ahTag("Laoo"):
		oldTag = ahTag("lao ")
	case ahTag("Yiii"):
		oldTag = ahTag("yi  ")
	case ahTag("Nkoo"):
		oldTag = ahTag("nko ")
	case ahTag("Vaii"):
		oldTag = ahTag("vai ")
	default:
		// The first character becomes a small letter.
		first := byte(unicodeScript >> 24)
		if first >= 'A' && first <= 'Z' {
			first += 'a' - 'A'
		}
		oldTag = unicodeScript&0x00FFFFFF | uint32(first)<<24
	}
	return append(tags, oldTag)
}

// ahScriptIndexForTag is ScriptList::index_for_tag.
func ahScriptIndexForTag(scriptList ahData, tag uint32) (int, bool) {
	records := scriptList.array(2, int(scriptList.u16(0)), 6)
	return ahBinarySearchBy(len(records)/6, func(i int) int {
		switch recTag := records.u32(6 * i); {
		case recTag < tag:
			return -1
		case recTag > tag:
			return 1
		}
		return 0
	})
}

// ahScript returns the script table of a record of the script list.
func ahScript(scriptList ahData, index int) (ahData, bool) {
	records := scriptList.array(2, int(scriptList.u16(0)), 6)
	if index < 0 || 6*index+6 > len(records) {
		return nil, false
	}
	return scriptList.resolve(int(records.u16(6*index+4)), 4)
}

// ahSelectScript is ScriptList::select: the first script of the list of tags
// that the font has, and then the scripts DFLT, dflt and latn.
func ahSelectScript(scriptList ahData, tags []uint32) (int, bool) {
	for _, tag := range tags {
		if index, ok := ahScriptIndexForTag(scriptList, tag); ok {
			return index, true
		}
	}
	for _, tag := range []uint32{ahTag("DFLT"), ahTag("dflt"), ahTag("latn")} {
		if index, ok := ahScriptIndexForTag(scriptList, tag); ok {
			return index, true
		}
	}
	return 0, false
}

// ahDefaultLangSys is Script::default_lang_sys. present is false for a
// script without a default language system, and ok is false for a table
// that cannot be read.
func ahDefaultLangSys(script ahData) (langSys ahData, present, ok bool) {
	off := int(script.u16(0))
	if off == 0 {
		return nil, false, false
	}
	langSys, ok = script.resolve(off, 6)
	return langSys, true, ok
}

// ahLangSysFeatureIndices returns the feature indices of a language system.
func ahLangSysFeatureIndices(langSys ahData) ahData {
	return langSys.array(6, int(langSys.u16(4)), 2)
}

// ahFeatureRecord returns the tag and the table of a record of the feature
// list. present is false for an index outside of the list, and ok is false
// for a table that cannot be read.
func ahFeatureRecord(featureList ahData, index int) (tag uint32, feature ahData, present, ok bool) {
	records := featureList.array(2, int(featureList.u16(0)), 6)
	if index < 0 || 6*index+6 > len(records) {
		return 0, nil, false, false
	}
	tag = records.u32(6 * index)
	feature, ok = featureList.resolve(int(records.u16(6*index+4)), 4)
	return tag, feature, true, ok
}

// ahFeatureLookupIndices returns the lookup indices of a feature.
func ahFeatureLookupIndices(feature ahData) ahData {
	return feature.array(4, int(feature.u16(2)), 2)
}

// ahCoverage is CoverageTable of read-fonts.
type ahCoverage struct {
	format uint16
	// items has the glyphs of format 1, or the range records of format 2.
	items ahData
}

// ahReadCoverage resolves an offset to a coverage table.
func ahReadCoverage(parent ahData, off int) (ahCoverage, bool) {
	t, ok := parent.resolve(off, 2)
	if !ok {
		return ahCoverage{}, false
	}
	format := t.u16(0)
	if (format != 1 && format != 2) || len(t) < 4 {
		return ahCoverage{}, false
	}
	size := 2
	if format == 2 {
		size = 6
	}
	return ahCoverage{format: format, items: t.array(4, int(t.u16(2)), size)}, true
}

// iter is CoverageTable::iter: it calls fn for every glyph of the table. It
// stops when fn returns false, and then it returns false.
func (c ahCoverage) iter(fn func(gid uint16) bool) bool {
	if c.format == 1 {
		for i := 0; i+2 <= len(c.items); i += 2 {
			if !fn(c.items.u16(i)) {
				return false
			}
		}
		return true
	}
	for i := 0; i+6 <= len(c.items); i += 6 {
		start, end := int(c.items.u16(i)), int(c.items.u16(i+2))
		for gid := start; gid <= end; gid++ {
			if !fn(uint16(gid)) {
				return false
			}
		}
	}
	return true
}

// get is CoverageTable::get: the coverage index of a glyph.
func (c ahCoverage) get(gid uint32) (uint16, bool) {
	if gid > 0xFFFF {
		return 0, false
	}
	g := uint16(gid)
	if c.format == 1 {
		idx, found := ahBinarySearchBy(len(c.items)/2, func(i int) int {
			switch v := c.items.u16(2 * i); {
			case v < g:
				return -1
			case v > g:
				return 1
			}
			return 0
		})
		return uint16(idx), found
	}
	idx, found := ahBinarySearchBy(len(c.items)/6, func(i int) int {
		if c.items.u16(6*i+2) < g {
			return -1
		}
		if c.items.u16(6*i) > g {
			return 1
		}
		return 0
	})
	if !found {
		return 0, false
	}
	startCoverageIndex := int(c.items.u16(6*idx + 4))
	index := startCoverageIndex + int(g-c.items.u16(6*idx))
	if index > 0xFFFF {
		return 0, false
	}
	return uint16(index), true
}

// ahSubtables is SubstitutionSubtables of read-fonts: the subtables of a
// lookup.
type ahSubtables struct {
	// kind is the type of the lookup: 1 to 6, or 8. An extension lookup has
	// the type of its subtables. The kind is 0 for an extension lookup
	// without a subtable that can be read.
	kind      int
	lookup    ahData
	offsets   ahData
	extension bool
}

// ahLookupSubtables is LookupList::lookups().get(index) and
// SubstitutionLookup::subtables.
func ahLookupSubtables(lookupList ahData, lookupIndex int) (ahSubtables, bool) {
	var s ahSubtables
	offsets := lookupList.array(2, int(lookupList.u16(0)), 2)
	if 2*lookupIndex+2 > len(offsets) {
		return s, false
	}
	lookup, ok := lookupList.resolve(int(offsets.u16(2*lookupIndex)), 6)
	if !ok {
		return s, false
	}
	s.lookup = lookup
	s.offsets = lookup.array(6, int(lookup.u16(4)), 2)
	switch lookupType := int(lookup.u16(0)); lookupType {
	case 1, 2, 3, 4, 5, 6, 8:
		s.kind = lookupType
	case 7:
		// The type comes from the first extension subtable that can be
		// read.
		s.extension = true
		found := false
		for i := 0; i+2 <= len(s.offsets); i += 2 {
			if ext, ok := lookup.resolve(int(s.offsets.u16(i)), 8); ok {
				s.kind = int(ext.u16(2))
				found = true
				break
			}
		}
		if !found {
			return s, true
		}
		switch s.kind {
		case 1, 2, 3, 4, 5, 6, 8:
		default:
			return s, false
		}
	default:
		return s, false
	}
	return s, true
}

// each calls fn for every subtable whose offset can be resolved. The caller
// checks the size of the subtable. It stops when fn returns false or when
// the work limit of the shaper is reached, and then it returns false.
func (s ahSubtables) each(shaper *ahShaper, fn func(t ahData) bool) bool {
	for i := 0; i+2 <= len(s.offsets); i += 2 {
		if !shaper.spend() {
			return false
		}
		var t ahData
		var ok bool
		if s.extension {
			var ext ahData
			if ext, ok = s.lookup.resolve(int(s.offsets.u16(i)), 8); ok {
				t, ok = ext.resolve(int(ext.u32(4)), 0)
			}
		} else {
			t, ok = s.lookup.resolve(int(s.offsets.u16(i)), 0)
		}
		if !ok {
			continue
		}
		if !fn(t) {
			return false
		}
	}
	return true
}

// eachOffset calls fn for every table that an array of 16 bit offsets points
// at. A null offset and a table that is shorter than minSize are skipped. It
// stops when fn returns false or when the work limit of the shaper is
// reached, and then it returns false.
func (s *ahShaper) eachOffset(parent ahData, arrayOff, count, minSize int, fn func(t ahData) bool) bool {
	offsets := parent.array(arrayOff, count, 2)
	for i := 0; i+2 <= len(offsets); i += 2 {
		if !s.spend() {
			return false
		}
		t, ok := parent.resolve(int(offsets.u16(i)), minSize)
		if !ok {
			continue
		}
		if !fn(t) {
			return false
		}
	}
	return true
}

// clusterShaper is Shaper::cluster_shaper. For now skrifa applies
// substitutions only for a style with a feature.
func (s *ahShaper) clusterShaper(style *ahStyleClass) *ahClusterShaper {
	c := &ahClusterShaper{shaper: s}
	if s.mode != ahShaperBestEffort || !style.hasFeature || !s.hasGsub {
		return c
	}
	scriptList, ok := s.scriptList()
	if !ok {
		return c
	}
	selected, ok := ahSelectScript(scriptList, ahScriptTagsFromUnicode(style.script.tag))
	if !ok {
		return c
	}
	script, ok := ahScript(scriptList, selected)
	if !ok {
		return c
	}
	langSys, present, ok := ahDefaultLangSys(script)
	if !present || !ok {
		return c
	}
	featureList, ok := s.featureList()
	if !ok {
		return c
	}
	// LangSys::feature_index_for_tag: the first feature of the language
	// system with the tag.
	records := featureList.array(2, int(featureList.u16(0)), 6)
	indices := ahLangSysFeatureIndices(langSys)
	featureIx := -1
	for i := 0; i+2 <= len(indices); i += 2 {
		ix := int(indices.u16(i))
		if 6*ix+6 <= len(records) && records.u32(6*ix) == style.feature {
			featureIx = ix
			break
		}
	}
	if featureIx < 0 {
		return c
	}
	_, feature, present, ok := ahFeatureRecord(featureList, featureIx)
	if !present || !ok {
		return c
	}
	lookupList, ok := s.lookupList()
	if !ok {
		return c
	}
	c.lookupList = lookupList
	c.feature = feature
	c.singleFeature = true
	return c
}

// computeCoverage is Shaper::compute_coverage: it uses the layout tables to
// compute the coverage of a style. It reports that the style of a glyph
// changed.
func (s *ahShaper) computeCoverage(style *ahStyleClass, coverageKind ahShaperCoverageKind, glyphStyles []ahGlyphStyle, visitedSet *ahVisitedLookupSet) bool {
	if !s.hasGsub {
		return false
	}
	scriptList, ok1 := s.scriptList()
	featureList, ok2 := s.featureList()
	lookupList, ok3 := s.lookupList()
	if !ok1 || !ok2 || !ok3 {
		return false
	}
	// The value 0 stands for no tag.
	var scriptTags [3]uint32
	for i, tag := range ahScriptTagsFromUnicode(style.script.tag) {
		if i < len(scriptTags) {
			scriptTags[i] = tag
		}
	}
	defaultScript := ahTag("DFLT")
	if coverageKind == ahShaperCoverageDefault {
		if scriptTags[0] == 0 {
			scriptTags[0] = defaultScript
		} else if scriptTags[1] == 0 {
			scriptTags[1] = defaultScript
		} else if scriptTags[1] != defaultScript {
			scriptTags[2] = defaultScript
		}
	} else {
		// The script classes have some tags that are not standard. skrifa
		// compares the tags of the script classes with the OpenType tag
		// here, as written.
		switch scriptTags[0] {
		case ahTag("Khms"), ahTag("Latb"), ahTag("Latp"):
			return false
		}
	}
	// Check each script that is in the GSUB table.
	handler := newAHGsubHandler(s, lookupList, style, glyphStyles, visitedSet)
	for _, tag := range scriptTags {
		if tag == 0 {
			continue
		}
		index, ok := ahScriptIndexForTag(scriptList, tag)
		if !ok {
			continue
		}
		script, ok := ahScript(scriptList, index)
		if !ok {
			continue
		}
		// All language systems of the script, and the default one last.
		var langSystems []ahData
		langSysRecords := script.array(4, int(script.u16(2)), 6)
		for i := 0; i+6 <= len(langSysRecords); i += 6 {
			if langSys, ok := script.resolve(int(langSysRecords.u16(i+4)), 6); ok {
				langSystems = append(langSystems, langSys)
			}
		}
		if langSys, present, ok := ahDefaultLangSys(script); present && ok {
			langSystems = append(langSystems, langSys)
		}
		for _, langSys := range langSystems {
			indices := ahLangSysFeatureIndices(langSys)
			for i := 0; i+2 <= len(indices) && s.spend(); i += 2 {
				tag, feature, present, ok := ahFeatureRecord(featureList, int(indices.u16(i)))
				if !present {
					continue
				}
				// A style with a feature looks at that feature only.
				if style.hasFeature && style.feature != tag {
					continue
				}
				if !ok {
					continue
				}
				lookupIndices := ahFeatureLookupIndices(feature)
				for k := 0; k+2 <= len(lookupIndices) && s.spend(); k += 2 {
					handler.processLookup(lookupIndices.u16(k))
				}
			}
		}
	}
	first, end, ok := handler.finish()
	if !ok {
		return false
	}
	// Some glyphs were captured. Assign the style to those without one.
	result := false
	for i := first; i < end; i++ {
		if glyphStyles[i].maybeAssignGsubOutputStyle(style) {
			result = true
		}
	}
	return result
}

// ahClusterShaper is ClusterShaper: it shapes one cluster of characters.
type ahClusterShaper struct {
	shaper *ahShaper
	// singleFeature says that the shaper applies the lookups of feature.
	// Else it maps the characters to their nominal glyphs.
	singleFeature bool
	lookupList    ahData
	feature       ahData
}

// shape is ClusterShaper::shape: the glyphs of a cluster of characters. The
// result is empty for a cluster that is rejected.
func (c *ahClusterShaper) shape(input string, output []ahShapedGlyph) []ahShapedGlyph {
	// First the nominal glyph of each character.
	output = output[:0]
	for len(input) > 0 {
		ch, size := utf8.DecodeRuneInString(input)
		input = input[size:]
		gid, _ := c.shaper.font.charmap.mapChar(uint32(ch))
		output = append(output, ahShapedGlyph{id: gid})
	}
	if !c.singleFeature {
		// The nominal mode rejects a cluster with more than one glyph.
		if c.shaper.mode == ahShaperNominal && len(output) > 1 {
			output = output[:0]
		}
		return output
	}
	didSubst := false
	lookupIndices := ahFeatureLookupIndices(c.feature)
	for i := 0; i+2 <= len(lookupIndices) && c.shaper.spend(); i += 2 {
		for glyphIx := 0; glyphIx < len(output); glyphIx++ {
			if c.applyLookup(lookupIndices.u16(i), output, glyphIx, 0) {
				didSubst = true
			}
		}
	}
	// A cluster that the feature did not change is rejected.
	if !didSubst {
		output = output[:0]
	}
	return output
}

// applyLookup is ClusterShaper::apply_lookup. It applies single
// substitutions only.
func (c *ahClusterShaper) applyLookup(lookupIndex uint16, cluster []ahShapedGlyph, glyphIx, nestingDepth int) bool {
	if nestingDepth > ahMaxNestingDepth {
		return false
	}
	if glyphIx >= len(cluster) {
		return false
	}
	glyph := &cluster[glyphIx]
	subtables, ok := ahLookupSubtables(c.lookupList, int(lookupIndex))
	if !ok || subtables.kind != 1 {
		return false
	}
	applied := false
	subtables.each(c.shaper, func(t ahData) bool {
		if len(t) < 6 {
			return true
		}
		switch t.u16(0) {
		case 1:
			coverage, ok := ahReadCoverage(t, int(t.u16(2)))
			if !ok {
				return true
			}
			if _, ok := coverage.get(glyph.id); !ok {
				return true
			}
			delta := int32(t.i16(4))
			glyph.id = uint32(uint16(int32(glyph.id) + delta))
			applied = true
			return false
		case 2:
			coverage, ok := ahReadCoverage(t, int(t.u16(2)))
			if !ok {
				return true
			}
			covIx, ok := coverage.get(glyph.id)
			if !ok {
				return true
			}
			substitutes := t.array(6, int(t.u16(4)), 2)
			if 2*int(covIx)+2 > len(substitutes) {
				return true
			}
			glyph.id = uint32(substitutes.u16(2 * int(covIx)))
			applied = true
			return false
		}
		return true
	})
	return applied
}

// ahGsubHandler is GsubHandler: it captures the glyphs of the GSUB table
// that are not in the character map.
//
// FreeType collects all lookups of a set of scripts and features, and then
// all output glyphs of each lookup. For a style with a feature, one
// character of the blue strings of the style must have a substitution in
// those lookups. Else the style is not assigned, because its alignment
// zones are not different.
type ahGsubHandler struct {
	shaper      *ahShaper
	charmap     *ahCharmap
	lookupList  ahData
	style       *ahStyleClass
	glyphStyles []ahGlyphStyle
	// needBlueSubsts says that a substitution for a character of the blue
	// strings is still to be found.
	needBlueSubsts bool
	// minGid and maxGid are the range of glyphs that were captured.
	minGid, maxGid int
	lookupDepth    int
	visitedSet     *ahVisitedLookupSet
}

func newAHGsubHandler(shaper *ahShaper, lookupList ahData, style *ahStyleClass, glyphStyles []ahGlyphStyle, visitedSet *ahVisitedLookupSet) *ahGsubHandler {
	return &ahGsubHandler{
		shaper:         shaper,
		charmap:        &shaper.font.charmap,
		lookupList:     lookupList,
		style:          style,
		glyphStyles:    glyphStyles,
		needBlueSubsts: style.hasFeature,
		minGid:         len(glyphStyles),
		maxGid:         0,
		visitedSet:     visitedSet,
	}
}

// processLookup is GsubHandler::process_lookup. It returns false when the
// lookups are nested too deep, and when the work limit of the shaper is
// reached.
func (h *ahGsubHandler) processLookup(lookupIndex uint16) bool {
	if h.lookupDepth == ahMaxNestingDepth {
		return false
	}
	// Skip a lookup that was processed already.
	if !h.visitedSet.insert(lookupIndex) {
		return true
	}
	h.lookupDepth++
	result := h.processLookupInner(lookupIndex)
	h.lookupDepth--
	return result
}

// seqLookupRecords processes the lookups of an array of sequence lookup
// records.
func (h *ahGsubHandler) seqLookupRecords(t ahData, off, count int) bool {
	records := t.array(off, count, 4)
	for i := 0; i+4 <= len(records); i += 4 {
		if !h.shaper.spend() || !h.processLookup(records.u16(i+2)) {
			return false
		}
	}
	return true
}

// sequenceRule processes the lookups of a SequenceRule or a
// ClassSequenceRule.
func (h *ahGsubHandler) sequenceRule(rule ahData) bool {
	glyphCount := int(rule.u16(0))
	off := 4 + 2*max(glyphCount-1, 0)
	return h.seqLookupRecords(rule, off, int(rule.u16(2)))
}

// chainedSequenceRule processes the lookups of a ChainedSequenceRule or a
// ChainedClassSequenceRule.
func (h *ahGsubHandler) chainedSequenceRule(rule ahData) bool {
	off := 2 + 2*int(rule.u16(0))
	inputGlyphCount := int(rule.u16(off))
	off += 2 + 2*max(inputGlyphCount-1, 0)
	lookaheadGlyphCount := int(rule.u16(off))
	off += 2 + 2*lookaheadGlyphCount
	return h.seqLookupRecords(rule, off+2, int(rule.u16(off)))
}

// processLookupInner is GsubHandler::process_lookup_inner. A false result
// stops the processing of the lookups that contain this lookup.
func (h *ahGsubHandler) processLookupInner(lookupIndex uint16) bool {
	subtables, ok := ahLookupSubtables(h.lookupList, int(lookupIndex))
	if !ok {
		return true
	}
	// The input coverage is checked for the blue strings when that is still
	// required, and the lookup is not under a contextual lookup.
	checkBlues := func(t ahData, coverageOff int) {
		if h.needBlueSubsts && h.lookupDepth == 1 {
			if coverage, ok := ahReadCoverage(t, coverageOff); ok {
				h.checkBlueCoverage(coverage)
			}
		}
	}
	captureArray := func(ids ahData) bool {
		for i := 0; i+2 <= len(ids); i += 2 {
			if !h.captureGlyph(uint32(ids.u16(i))) {
				return false
			}
		}
		return true
	}
	switch subtables.kind {
	case 1: // Single
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 6 {
				return true
			}
			switch t.u16(0) {
			case 1:
				coverage, ok := ahReadCoverage(t, int(t.u16(2)))
				if !ok {
					return true
				}
				delta := int32(t.i16(4))
				if !coverage.iter(func(gid uint16) bool {
					return h.captureGlyph(uint32(uint16(int32(gid) + delta)))
				}) {
					return false
				}
				if h.needBlueSubsts && h.lookupDepth == 1 {
					h.checkBlueCoverage(coverage)
				}
			case 2:
				if !captureArray(t.array(6, int(t.u16(4)), 2)) {
					return false
				}
				checkBlues(t, int(t.u16(2)))
			}
			return true
		})
	case 2: // Multiple
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 6 {
				return true
			}
			if !h.shaper.eachOffset(t, 6, int(t.u16(4)), 2, func(seq ahData) bool {
				return captureArray(seq.array(2, int(seq.u16(0)), 2))
			}) {
				return false
			}
			checkBlues(t, int(t.u16(2)))
			return true
		})
	case 4: // Ligature
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 6 {
				return true
			}
			return h.shaper.eachOffset(t, 6, int(t.u16(4)), 2, func(set ahData) bool {
				return h.shaper.eachOffset(set, 2, int(set.u16(0)), 4, func(lig ahData) bool {
					return h.captureGlyph(uint32(lig.u16(0)))
				})
			})
		})
	case 3: // Alternate
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 6 {
				return true
			}
			return h.shaper.eachOffset(t, 6, int(t.u16(4)), 2, func(set ahData) bool {
				return captureArray(set.array(2, int(set.u16(0)), 2))
			})
		})
	case 5: // Contextual
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 2 {
				return true
			}
			switch t.u16(0) {
			case 1:
				if len(t) < 6 {
					return true
				}
				return h.shaper.eachOffset(t, 6, int(t.u16(4)), 2, func(set ahData) bool {
					return h.shaper.eachOffset(set, 2, int(set.u16(0)), 4, h.sequenceRule)
				})
			case 2:
				if len(t) < 8 {
					return true
				}
				return h.shaper.eachOffset(t, 8, int(t.u16(6)), 2, func(set ahData) bool {
					return h.shaper.eachOffset(set, 2, int(set.u16(0)), 4, h.sequenceRule)
				})
			case 3:
				if len(t) < 6 {
					return true
				}
				return h.seqLookupRecords(t, 6+2*int(t.u16(2)), int(t.u16(4)))
			}
			return true
		})
	case 6: // ChainContextual
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 2 {
				return true
			}
			switch t.u16(0) {
			case 1:
				if len(t) < 6 {
					return true
				}
				return h.shaper.eachOffset(t, 6, int(t.u16(4)), 2, func(set ahData) bool {
					return h.shaper.eachOffset(set, 2, int(set.u16(0)), 8, h.chainedSequenceRule)
				})
			case 2:
				if len(t) < 12 {
					return true
				}
				return h.shaper.eachOffset(t, 12, int(t.u16(10)), 2, func(set ahData) bool {
					return h.shaper.eachOffset(set, 2, int(set.u16(0)), 8, h.chainedSequenceRule)
				})
			case 3:
				if len(t) < 10 {
					return true
				}
				off := 4 + 2*int(t.u16(2))
				off += 2 + 2*int(t.u16(off))
				off += 2 + 2*int(t.u16(off))
				return h.seqLookupRecords(t, off+2, int(t.u16(off)))
			}
			return true
		})
	case 8: // Reverse
		return subtables.each(h.shaper, func(t ahData) bool {
			if len(t) < 10 {
				return true
			}
			off := 6 + 2*int(t.u16(4))
			off += 2 + 2*int(t.u16(off))
			return captureArray(t.array(off+2, int(t.u16(off)), 2))
		})
	}
	return true
}

// finish is GsubHandler::finish: it ends the processing of a set of
// lookups, and it returns the range of glyphs that were captured.
func (h *ahGsubHandler) finish() (first, end int, ok bool) {
	h.visitedSet.clear()
	if h.minGid > h.maxGid || len(h.glyphStyles) == 0 {
		// No glyph was captured.
		return 0, 0, false
	}
	first, end = h.minGid, h.maxGid+1
	if h.needBlueSubsts {
		// No substitution was found for the blue strings, so the style is
		// ignored. Clear the marker of the glyphs.
		if end > len(h.glyphStyles) {
			return 0, 0, false
		}
		for i := first; i < end; i++ {
			h.glyphStyles[i].clearFromGsub()
		}
		return 0, 0, false
	}
	return first, end, true
}

// checkBlueCoverage is GsubHandler::check_blue_coverage: it checks a
// coverage table for the characters of the blue strings of the style.
func (h *ahGsubHandler) checkBlueCoverage(coverage ahCoverage) {
	for _, blue := range h.style.script.blues {
		for _, ch := range blue.str {
			gid, ok := h.charmap.mapChar(uint32(ch))
			if !ok {
				continue
			}
			if _, ok := coverage.get(gid); ok {
				// The condition holds, so no other subtable is checked.
				h.needBlueSubsts = false
				return
			}
		}
	}
}

// captureGlyph is GsubHandler::capture_glyph. It returns false when the
// work limit of the shaper is reached.
func (h *ahGsubHandler) captureGlyph(gid uint32) bool {
	if !h.shaper.spend() {
		return false
	}
	if int64(gid) < int64(len(h.glyphStyles)) {
		h.glyphStyles[gid].setFromGsubOutput()
		h.minGid = min(int(gid), h.minGid)
		h.maxGid = max(int(gid), h.maxGid)
	}
	return true
}

// ahVisitedLookupSet is VisitedLookupSet: the lookups that were processed.
type ahVisitedLookupSet struct {
	storage []byte
}

// insert adds a lookup to the set. It returns false for a lookup that is in
// the set already, and for a lookup that the set has no place for.
func (s *ahVisitedLookupSet) insert(lookupIndex uint16) bool {
	byteIx := int(lookupIndex) / 8
	bitMask := byte(1) << (lookupIndex % 8)
	if byteIx >= len(s.storage) {
		return false
	}
	if s.storage[byteIx]&bitMask == 0 {
		s.storage[byteIx] |= bitMask
		return true
	}
	return false
}

func (s *ahVisitedLookupSet) clear() {
	for i := range s.storage {
		s.storage[i] = 0
	}
}
