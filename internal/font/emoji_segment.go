// Ported from Chromium (Blink):
// third_party/blink/renderer/platform/fonts/symbols_iterator.cc,
// utf16_ragel_iterator.h (GetEmojiSegmentationCategory) and
// third_party/emoji-segmenter/src/emoji_presentation_scanner.rl.
//
// The port does not record the version. The licenses are those of Chromium
// 156.0.8078.4, the reference browser, which has emoji-segmenter at
// revision 955936be8b39:
//
//   - symbols_iterator.cc and utf16_ragel_iterator.h: BSD-3-Clause,
//     Copyright 2015 and 2019 The Chromium Authors (LICENSE-Chromium.txt).
//   - emoji_presentation_scanner.rl: Apache-2.0. The file says:
//
//	Copyright 2019 Google LLC
//
//	Licensed under the Apache License, Version 2.0 (the "License");
//	you may not use this file except in compliance with the License.
//	You may obtain a copy of the License at
//
//	    https://www.apache.org/licenses/LICENSE-2.0
//
//	Unless required by applicable law or agreed to in writing, software
//	distributed under the License is distributed on an "AS IS" BASIS,
//	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	See the License for the specific language governing permissions and
//	limitations under the License.
//
// This file is a Go translation of the scanner. It is a changed version of
// emoji_presentation_scanner.rl. LICENSE-emoji-segmenter.txt in this
// directory has the license. emoji-segmenter has no NOTICE file.

package font

import "sort"

// Blink splits a text into runs that have text presentation and runs that
// have emoji presentation, before it chooses fonts (SymbolsIterator). For a
// run with emoji presentation it tries the emoji font of the system first.
// A character with emoji presentation is, for example, a character that
// Unicode draws as an emoji by default, a character with the variation
// selector 16 after it, a keycap sequence or a flag.

// The categories of the scanner.
const (
	emojiCatEmoji uint8 = iota
	emojiCatTextPresentation
	emojiCatEmojiPresentation
	emojiCatModifierBase
	emojiCatModifier
	emojiCatVSBase
	emojiCatRegionalIndicator
	emojiCatKeycapBase
	emojiCatCombiningEnclosingKeycap
	emojiCatCombiningEnclosingCircleBackslash
	emojiCatZWJ
	emojiCatVS15
	emojiCatVS16
	emojiCatTagBase
	emojiCatTagSequence
	emojiCatTagTerm
	// emojiCatOther is kMaxEmojiScannerCategory: a character that is no
	// part of an emoji.
	emojiCatOther
)

// inRanges reports whether r is in a table of sorted ranges: first, last,
// first, last.
func inRanges(table []rune, r rune) bool {
	i := sort.Search(len(table)/2, func(i int) bool { return table[2*i+1] >= r })
	return i < len(table)/2 && table[2*i] <= r
}

// emojiSegmentationCategory is GetEmojiSegmentationCategory.
func emojiSegmentationCategory(r rune) uint8 {
	// Most characters of a text are below the first emoji.
	if r < 0x23 {
		return emojiCatOther
	}
	// Specific ones first.
	switch {
	case r == 0x20e3:
		return emojiCatCombiningEnclosingKeycap
	case r == 0x20e0:
		return emojiCatCombiningEnclosingCircleBackslash
	case r == 0x200d:
		return emojiCatZWJ
	case r == 0xfe0e:
		return emojiCatVS15
	case r == 0xfe0f:
		return emojiCatVS16
	case r == 0x1f3f4:
		return emojiCatTagBase
	case (r >= 0xe0030 && r <= 0xe0039) || (r >= 0xe0061 && r <= 0xe007a):
		return emojiCatTagSequence
	case r == 0xe007f:
		return emojiCatTagTerm
	case inRanges(emojiModifierBaseRanges[:], r):
		return emojiCatModifierBase
	case r >= 0x1f3fb && r <= 0x1f3ff:
		return emojiCatModifier
	case r >= 0x1f1e6 && r <= 0x1f1ff:
		return emojiCatRegionalIndicator
	case (r >= '0' && r <= '9') || r == '#' || r == '*':
		return emojiCatKeycapBase
	case inRanges(emojiPresentationRanges[:], r):
		return emojiCatEmojiPresentation
	case inRanges(emojiRanges[:], r):
		// Emoji and not Emoji_Presentation.
		return emojiCatTextPresentation
	}
	return emojiCatOther
}

// isAnyEmoji is the rule any_emoji of the scanner.
func isAnyEmoji(cat uint8) bool {
	switch cat {
	case emojiCatTextPresentation, emojiCatEmojiPresentation, emojiCatKeycapBase,
		emojiCatModifierBase, emojiCatTagBase, emojiCatEmoji:
		return true
	}
	return false
}

// emojiToken is one step of the scanner text_and_emoji_run: the length of
// the longest token at p and whether it has emoji presentation. The rules
// of the scanner are:
//
//	text_presentation_emoji = any_emoji VS15                      => text
//	with_presentation_emoji = emoji_presentation
//	  | emoji_presentation_sequence | emoji_modifier_sequence
//	  | emoji_flag_sequence | emoji_keycap_sequence
//	  | emoji_tag_sequence | emoji_zwj_sequence
//	  | emoji_combining_enclosing_circle_backslash_sequence       => emoji
//	any                                                           => text
func emojiToken(cats []uint8, p int) (length int, emoji bool) {
	n := len(cats)
	at := func(i int) uint8 {
		if i < n {
			return cats[i]
		}
		return emojiCatOther
	}
	c := cats[p]
	any := isAnyEmoji(c)

	// text_presentation_emoji = any_emoji VS15. No token with emoji
	// presentation starts with these two characters.
	if any && at(p+1) == emojiCatVS15 {
		return 2, false
	}

	best := 0
	// emoji_presentation = EMOJI_EMOJI_PRESENTATION | TAG_BASE |
	//   EMOJI_MODIFIER_BASE | EMOJI_MODIFIER | REGIONAL_INDICATOR
	switch c {
	case emojiCatEmojiPresentation, emojiCatTagBase, emojiCatModifierBase, emojiCatModifier, emojiCatRegionalIndicator:
		best = 1
	}
	// emoji_presentation_sequence = any_emoji VS16
	// emoji_combining_enclosing_circle_backslash_sequence = any_emoji COMBINING_ENCLOSING_CIRCLE_BACKSLASH
	if any && (at(p+1) == emojiCatVS16 || at(p+1) == emojiCatCombiningEnclosingCircleBackslash) {
		best = max(best, 2)
	}
	// emoji_modifier_sequence = EMOJI_MODIFIER_BASE EMOJI_MODIFIER
	if c == emojiCatModifierBase && at(p+1) == emojiCatModifier {
		best = max(best, 2)
	}
	// emoji_flag_sequence = REGIONAL_INDICATOR REGIONAL_INDICATOR
	if c == emojiCatRegionalIndicator && at(p+1) == emojiCatRegionalIndicator {
		best = max(best, 2)
	}
	// emoji_keycap_sequence = KEYCAP_BASE VS16 COMBINING_ENCLOSING_KEYCAP
	if c == emojiCatKeycapBase && at(p+1) == emojiCatVS16 && at(p+2) == emojiCatCombiningEnclosingKeycap {
		best = max(best, 3)
	}
	// emoji_tag_sequence = TAG_BASE TAG_SEQUENCE+ TAG_TERM
	if c == emojiCatTagBase && at(p+1) == emojiCatTagSequence {
		i := p + 1
		for at(i) == emojiCatTagSequence {
			i++
		}
		if at(i) == emojiCatTagTerm {
			best = max(best, i+1-p)
		}
	}
	// emoji_zwj_element = emoji_presentation_sequence | emoji_modifier_sequence | any_emoji
	// emoji_zwj_sequence = emoji_zwj_element ( ZWJ emoji_zwj_element )+
	var sequence func(pos, elements int)
	element := func(pos, elements int) {
		ec := at(pos)
		if !isAnyEmoji(ec) {
			return
		}
		if at(pos+1) == emojiCatVS16 {
			sequence(pos+2, elements+1)
		}
		if ec == emojiCatModifierBase && at(pos+1) == emojiCatModifier {
			sequence(pos+2, elements+1)
		}
		sequence(pos+1, elements+1)
	}
	sequence = func(pos, elements int) {
		if elements >= 2 {
			best = max(best, pos-p)
		}
		if at(pos) == emojiCatZWJ {
			element(pos+1, elements)
		}
	}
	element(p, 0)

	if best > 0 {
		return best, true
	}
	// any
	return 1, false
}

// emojiPresentation returns, for each character of text, whether it is in a
// run with emoji presentation: FontFallbackPriority::kEmojiEmoji in Blink.
// The result is nil when no character of the text has emoji presentation.
func emojiPresentation(text []rune) []bool {
	// A fast path for a text without emoji, as SymbolsIterator has one.
	// Every token with emoji presentation has a character of a category
	// other than the keycap base: a digit, "#" and "*" start such a token
	// only in front of the variation selector 16 or a joiner.
	relevant := false
	for _, r := range text {
		if r < 0x80 {
			continue
		}
		if cat := emojiSegmentationCategory(r); cat != emojiCatOther && cat != emojiCatKeycapBase {
			relevant = true
			break
		}
	}
	if !relevant {
		return nil
	}
	cats := make([]uint8, len(text))
	for i, r := range text {
		cats[i] = emojiSegmentationCategory(r)
	}
	var out []bool
	for p := 0; p < len(text); {
		length, emoji := emojiToken(cats, p)
		if emoji {
			if out == nil {
				out = make([]bool, len(text))
			}
			for i := p; i < p+length; i++ {
				out[i] = true
			}
		}
		p += length
	}
	return out
}
