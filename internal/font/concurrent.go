// This file ports no upstream file. It has the caches and the shaper pool
// that make the font library safe for concurrent use.

package font

import (
	"sync/atomic"

	gtfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/harfbuzz"
)

// A Library, its faces and its chains are shared by every render, and
// renders run in many goroutines at the same time. So every lookup must be
// safe for concurrent use.
//
// The parsed font of go-text (font.Font) does not change after it is
// loaded. A go-text face (font.Face) is the font plus caches that its
// methods NominalGlyph and GlyphExtents write, so a go-text face is not
// safe for concurrent use. The rules here:
//
//   - Face.gt is shared. Only its methods that do not write are called on
//     it: the advances, the glyph data and the tables.
//   - A character is mapped to a glyph with Face.NominalGlyph, which has a
//     cache of its own that is safe for concurrent use.
//   - HarfBuzz maps characters and reads glyph extents while it shapes.
//     Each shaper of the pool has a go-text face of its own, and one
//     goroutine uses a shaper at a time.
//   - Results that are computed on demand are stored in a sync.Map, behind
//     a sync.Once or behind the mutex of the library.

// glyphCache maps characters to glyph ids. It is a direct-mapped cache
// whose entries are read and written with atomic operations, so it needs no
// lock. An entry has the character plus one in the high 32 bits (zero is an
// empty entry) and the glyph id plus one in the low 32 bits (zero is a
// character that the font does not have).
type glyphCache [256]atomic.Uint64

func (c *glyphCache) get(r rune) (gid uint32, ok, hit bool) {
	v := c[uint32(r)&255].Load()
	if uint32(v>>32) != uint32(r)+1 {
		return 0, false, false
	}
	low := uint32(v)
	if low == 0 {
		return 0, false, true
	}
	return low - 1, true, true
}

func (c *glyphCache) set(r rune, gid uint32, ok bool) {
	v := uint64(uint32(r)+1) << 32
	if ok {
		v |= uint64(gid + 1)
	}
	c[uint32(r)&255].Store(v)
}

// NominalGlyph returns the glyph that the font maps a character to. The
// second result is false when the font does not have the character. It is
// safe for concurrent use, unlike NominalGlyph of the go-text face.
func (f *Face) NominalGlyph(r rune) (uint32, bool) {
	if gid, ok, hit := f.glyphs.get(r); hit {
		return gid, ok
	}
	// The lookup in the font itself does not write.
	g, ok := f.gt.Font.NominalGlyph(r)
	f.glyphs.set(r, uint32(g), ok)
	return uint32(g), ok
}

// newShaper returns a HarfBuzz font and a buffer for one goroutine. The
// HarfBuzz font gets a go-text face of its own, because shaping writes the
// caches of the face.
func (f *Face) newShaper() *shaper {
	return &shaper{font: harfbuzz.NewFont(gtfont.NewFace(f.gt.Font)), buf: harfbuzz.NewBuffer()}
}
