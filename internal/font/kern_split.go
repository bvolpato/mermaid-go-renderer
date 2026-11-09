// Ported from HarfBuzz, src/hb-kern.hh: hb_kern_machine_t::kern
// (MIT-Modern-Variant). The port does not record the version of HarfBuzz.
// The notice that follows is the notice of src/hb-kern.hh at the revision of
// HarfBuzz that Chromium 156.0.8078.4 has (f5efbbef3841):
//
//	Copyright © 2017  Google, Inc.
//
//	 This is part of HarfBuzz, a text shaping library.
//
//	Permission is hereby granted, without written agreement and without
//	license or royalty fees, to use, copy, modify, and distribute this
//	software and its documentation for any purpose, provided that the
//	above copyright notice and the following two paragraphs appear in
//	all copies of this software.
//
//	IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE TO ANY PARTY FOR
//	DIRECT, INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL DAMAGES
//	ARISING OUT OF THE USE OF THIS SOFTWARE AND ITS DOCUMENTATION, EVEN
//	IF THE COPYRIGHT HOLDER HAS BEEN ADVISED OF THE POSSIBILITY OF SUCH
//	DAMAGE.
//
//	THE COPYRIGHT HOLDER SPECIFICALLY DISCLAIMS ANY WARRANTIES, INCLUDING,
//	BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
//	FITNESS FOR A PARTICULAR PURPOSE.  THE SOFTWARE PROVIDED HEREUNDER IS
//	ON AN "AS IS" BASIS, AND THE COPYRIGHT HOLDER HAS NO OBLIGATION TO
//	PROVIDE MAINTENANCE, SUPPORT, UPDATES, ENHANCEMENTS, OR MODIFICATIONS.
//
//	Google Author(s): Behdad Esfahbod
//
// LICENSE-HarfBuzz.txt in this directory has the file COPYING of HarfBuzz.

package font

import (
	gtfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/harfbuzz"
)

// This file has the kern machine of HarfBuzz (hb_kern_machine_t::kern).
//
// A font without kerning in GPOS kerns with its legacy kern table. The kern
// machine of HarfBuzz splits the kerning of a pair of glyphs:
//
//	kern = font->em_scale_x (kern);
//	kern1 = kern >> 1;
//	kern2 = kern - kern1;
//	pos[i].x_advance += kern1;
//	pos[j].x_advance += kern2;
//	pos[j].x_offset += kern2;
//
// Blink shapes with the font size as the scale, in 16.16 fixed point, so
// the halves are halves of the scaled value. Face.shape shapes in font
// units, where the half of an odd value is rounded down to a whole unit.
// The sum of the two halves is the same, but the border between the two
// glyphs is half a unit off, and that changes the width of a part of the
// text by 1/64 px in some cases.
//
// machineKerning computes the split at the scale of Blink.

// machineKerning returns the adjustments that the kern machine makes to
// the advances and to the offsets of the glyphs of a shaped buffer, in
// 16.16 px. nominal has the advances of the glyphs in font units, and
// scale16 is hb_font_t::em_scale_x.
//
// ok is false when the positions of the buffer are not the result of the
// kern machine alone: the font has no kern table, kerning comes from GPOS
// or from a state table, or another positioning step changed the glyphs.
// The function tests this: it runs the machine in font units, on the pairs
// of adjacent glyphs, and compares the result with the buffer.
func (f *Face) machineKerning(info []harfbuzz.GlyphInfo, pos []harfbuzz.GlyphPosition, nominal []int32, scale16 func(int32) int64) (adv, off []int64, ok bool) {
	kern := f.gt.Kern
	if len(kern) == 0 || len(f.gt.Kerx) != 0 || len(info) < 2 {
		return nil, nil, false
	}
	n := len(info)
	unitAdv := make([]int32, n)
	unitOff := make([]int32, n)
	adv = make([]int64, n)
	off = make([]int64, n)
	kerned := false
	for _, st := range kern {
		if !st.IsExtended && st.IsVariation() {
			continue
		}
		if !st.IsHorizontal() {
			continue
		}
		driver, simple := st.Data.(gtfont.SimpleKerns)
		if !simple || st.IsCrossStream() || st.IsBackwards() {
			return nil, nil, false
		}
		for i := 0; i+1 < n; i++ {
			j := i + 1
			rawKern := int32(driver.KernPair(info[i].Glyph, info[j].Glyph))
			if rawKern == 0 {
				continue
			}
			kerned = true
			unit1 := rawKern >> 1
			unit2 := rawKern - unit1
			unitAdv[i] += unit1
			unitAdv[j] += unit2
			unitOff[j] += unit2

			scaled := scale16(rawKern)
			kern1 := scaled >> 1
			kern2 := scaled - kern1
			adv[i] += kern1
			adv[j] += kern2
			off[j] += kern2
		}
	}
	if !kerned {
		return nil, nil, false
	}
	for i := range info {
		if pos[i].XAdvance-nominal[i] != unitAdv[i] || pos[i].XOffset != unitOff[i] || pos[i].YOffset != 0 || pos[i].YAdvance != 0 {
			return nil, nil, false
		}
	}
	return adv, off, true
}
