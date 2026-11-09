package font

import (
	"encoding/binary"
	"testing"
)

// TestAutohintGsubWorkLimit computes the glyph styles of a font whose GSUB
// table has one contextual lookup with 1000 subtables at one offset. Each
// level below has 1000 offsets to one table also, down to 1000 lookup
// records. A walk over all of them takes 1000^4 steps. The work limit of
// the shaper must end the walk.
func TestAutohintGsubWorkLimit(t *testing.T) {
	const n = 1000
	u16 := func(b []byte, v ...int) []byte {
		for _, x := range v {
			b = binary.BigEndian.AppendUint16(b, uint16(x))
		}
		return b
	}
	repeat := func(b []byte, v, count int) []byte {
		for i := 0; i < count; i++ {
			b = u16(b, v)
		}
		return b
	}

	// The sequence rule: one glyph, and n records that point to lookup 0.
	rule := u16(nil, 1, n)
	for i := 0; i < n; i++ {
		rule = u16(rule, 0, 0)
	}
	// The rule set: n offsets to the rule, which comes after the set.
	ruleSetLen := 2 + 2*n
	ruleSet := repeat(u16(nil, n), ruleSetLen, n)
	// The subtable of format 1: n offsets to the rule set, which comes
	// after the subtable.
	subtableLen := 6 + 2*n
	subtable := repeat(u16(nil, 1, 0, n), subtableLen, n)
	// The lookup of type 5: n offsets to the subtable, which comes after
	// the lookup.
	lookupLen := 6 + 2*n
	lookup := repeat(u16(nil, 5, 0, n), lookupLen, n)
	lookup = append(lookup, subtable...)
	lookup = append(lookup, ruleSet...)
	lookup = append(lookup, rule...)

	// The script latn with a default language system that has feature 0.
	scriptList := u16(nil, 1)
	scriptList = append(scriptList, "latn"...)
	scriptList = u16(scriptList, 8)    // the script
	scriptList = u16(scriptList, 4, 0) // the default language system, no other
	scriptList = u16(scriptList, 0, 0xFFFF, 1, 0)
	// The feature liga with lookup 0.
	featureList := u16(nil, 1)
	featureList = append(featureList, "liga"...)
	featureList = u16(featureList, 8)
	featureList = u16(featureList, 0, 1, 0)
	lookupList := u16(nil, 1, 4)
	lookupList = append(lookupList, lookup...)

	gsub := u16(nil, 1, 0, 10, 10+len(scriptList), 10+len(scriptList)+len(featureList))
	gsub = append(gsub, scriptList...)
	gsub = append(gsub, featureList...)
	gsub = append(gsub, lookupList...)

	font := &ahFont{gsub: gsub, hasGsub: true}
	shaper := newAHShaper(font, ahShaperBestEffort)
	styles := newAHGlyphStyleMap(16, shaper)
	if shaper.work != 0 {
		t.Errorf("the walk did not reach the work limit: %d steps remain", shaper.work)
	}
	// No lookup has an output glyph, so every glyph has the fallback style.
	for gid := uint32(0); gid < 16; gid++ {
		style, ok := styles.style(gid)
		if !ok || style.styleClass() != &ahStyleClasses[ahStyleHani] {
			t.Fatalf("glyph %d: style %v", gid, style)
		}
	}
}
