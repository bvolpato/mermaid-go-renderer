// This file ports no upstream file. It shapes a text in runs and adds the
// widths. Three small functions round as LayoutUnit of Blink does
// (third_party/blink/renderer/platform/geometry/layout_unit.h, which has the
// BSD-3-Clause license at Chromium 156.0.8078.4, with the terms in the file
// and the copyright of Google).

package font

import (
	"math"
	"slices"
	"unicode"

	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// Run is a stretch of text that one face renders.
type Run struct {
	Face   *Face
	Start  int // rune offset in the text
	Glyphs []Glyph
	Width  float64 // float32 sum of the advances
}

// ShapeText splits text into runs of one script, as the RunSegmenter of
// Blink does, splits each run by face (font fallback) and shapes each piece
// with the script of its run. It corresponds to one ShapeResult in Blink.
//
// The result is shared: the caller must not change it. A diagram measures
// the same texts many times, in one render and in the next ones, and the
// shaping of a text with a chain of faces at a size has one result. So the
// results of short texts are kept (see shapedTexts).
func (c *Chain) ShapeText(text []rune, size float64) []Run {
	if len(text) == 0 || len(text) > maxKeptShape {
		return c.shapeText(text, size)
	}
	key := shapeKey{chain: c, size: size, hash: hashRunes(text), length: len(text)}
	if kept, ok := shapedTexts.Get(key); ok && slices.Equal(kept.text, text) {
		return kept.runs
	}
	runs := c.shapeText(text, size)
	shapedTexts.Put(key, &shapedText{text: slices.Clone(text), runs: runs})
	return runs
}

// maxKeptShape is the longest text, in characters, whose shaping is kept.
const maxKeptShape = 160

// shapeKey is the key of a kept shaping. The hash stands for the text: the
// entry has the text itself, and a lookup compares it.
type shapeKey struct {
	chain  *Chain
	size   float64
	hash   uint64
	length int
}

type shapedText struct {
	text []rune
	runs []Run
}

// shapedTexts has the results of ShapeText. An entry has about 60 bytes for
// each character.
var shapedTexts = memo.NewMap[shapeKey, *shapedText](4096)

// hashRunes is the FNV-1a hash of the characters of a text.
func hashRunes(text []rune) uint64 {
	h := uint64(14695981039346656037)
	for _, r := range text {
		h ^= uint64(uint32(r))
		h *= 1099511628211
	}
	return h
}

// shapeText is ShapeText without the kept results.
func (c *Chain) shapeText(text []rune, size float64) []Run {
	var runs []Run
	// emoji says which characters are in a run with emoji presentation.
	// Blink tries the emoji font of the system first for such a run.
	emoji := emojiPresentation(text)
	for _, sr := range scriptRuns(text) {
		start := sr.start
		var cur *Face
		// notdef draws a character that no font has. For a chain with web
		// fonts it depends on the characters of the run: Blink shapes a
		// stretch with emoji presentation and a stretch of text apart, and
		// each has its first font. notdefEnd is the end of the stretch
		// that notdef is for.
		var notdef *Face
		notdefEnd := sr.start
		flush := func(end int) {
			if end > start && cur != nil {
				g := cur.shape(text[start:end], size, sr.script)
				runs = append(runs, Run{Face: cur, Start: start, Glyphs: g, Width: Width(g)})
			}
			start = end
		}
		for i := sr.start; i < sr.end; i++ {
			r := text[i]
			if c.web && i >= notdefEnd {
				isEmoji := emoji != nil && emoji[i]
				for notdefEnd = i + 1; notdefEnd < sr.end && (emoji != nil && emoji[notdefEnd]) == isEmoji; notdefEnd++ {
				}
				notdef = c.webFirstCandidate(text[i:notdefEnd])
			}
			var f *Face
			if cur != nil && (unicode.Is(unicode.Mn, r) || r == 0x200d || (r >= 0xfe00 && r <= 0xfe0f)) {
				f = cur // a mark or joiner stays with its base
			} else if emoji != nil && emoji[i] {
				f = c.emojiFaceFor(r, cur, notdef)
			} else if c.web {
				f = c.webFaceFor(r, notdef)
			} else {
				f = c.FaceFor(r)
			}
			if f != cur {
				flush(i)
				cur = f
			}
		}
		flush(sr.end)
	}
	return runs
}

// TextWidth is the width of a text item before snapping: the float32 sum of
// the run widths, as ShapeResult::Width() is.
func TextWidth(runs []Run) float64 {
	var w float32
	for i := range runs {
		w += float32(runs[i].Width)
	}
	return float64(w)
}

// SnapCeil is LayoutUnit::FromFloatCeil: the next multiple of 1/64 px.
func SnapCeil(w float64) float64 {
	return math.Ceil(float64(float32(w)*64)) / 64
}

// SnapRound is LayoutUnit::FromFloatRound.
func SnapRound(w float64) float64 {
	return math.Round(float64(float32(w)*64)) / 64
}

// SnapFloor is LayoutUnit::FromFloatFloor.
func SnapFloor(w float64) float64 {
	return math.Floor(float64(float32(w)*64)) / 64
}
