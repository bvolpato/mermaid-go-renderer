package raster

import (
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"math/bits"
	"sync"
)

type pngEncState struct {
	rawBuf  []byte
	idatBuf []byte
	hash    [4096]uint32
	zw      *zlib.Writer
	chunk   []byte
}

var pngEncPool = sync.Pool{
	New: func() any {
		return &pngEncState{}
	},
}

const (
	maxFastPNGRaw   = 8 << 20
	maxPooledPNGBuf = 8 << 20
)

var (
	pngHeader = []byte("\x89PNG\r\n\x1a\n")
	pngIEND   = []byte{0, 0, 0, 0, 'I', 'E', 'N', 'D', 0xae, 0x42, 0x60, 0x82}
)

// Precomputed RFC 1951 fixed Huffman tables (LSB-first bit-reversed).
var (
	fixedLitCode [256]uint16
	fixedLitLen  [256]uint8
	// fixedLenPacked[L] holds (code | extraVal<<codeBits) for match length L in [3..258].
	fixedLenPacked [259]uint32
	fixedLenBits   [259]uint8
	fixedDistCode  [30]uint8
	fixedDistBase  [30]uint16
	fixedDistExtra [30]uint8
)

func init() {
	revBits := func(v uint16, n uint8) uint16 {
		return bits.Reverse16(v) >> (16 - n)
	}
	for i := 0; i <= 143; i++ {
		fixedLitCode[i] = revBits(uint16(0x30+i), 8)
		fixedLitLen[i] = 8
	}
	for i := 144; i <= 255; i++ {
		fixedLitCode[i] = revBits(uint16(0x190+(i-144)), 9)
		fixedLitLen[i] = 9
	}

	// RFC 1951 Section 3.2.5 length codes 257..285.
	lenBases := [29]int{
		3, 4, 5, 6, 7, 8, 9, 10,
		11, 13, 15, 17, 19, 23, 27, 31,
		35, 43, 51, 59, 67, 83, 99, 115,
		131, 163, 195, 227, 258,
	}
	lenExtra := [29]uint8{
		0, 0, 0, 0, 0, 0, 0, 0,
		1, 1, 1, 1, 2, 2, 2, 2,
		3, 3, 3, 3, 4, 4, 4, 4,
		5, 5, 5, 5, 0,
	}
	for idx := 0; idx < 29; idx++ {
		sym := 257 + idx
		var code uint16
		var cb uint8
		if sym <= 279 {
			code = revBits(uint16(sym-256), 7)
			cb = 7
		} else {
			code = revBits(uint16(0xc0+(sym-280)), 8)
			cb = 8
		}
		lo := lenBases[idx]
		hi := lo + (1 << lenExtra[idx]) - 1
		if idx == 28 {
			hi = 258
		}
		for l := lo; l <= hi && l <= 258; l++ {
			extra := uint32(l - lo)
			fixedLenPacked[l] = uint32(code) | (extra << cb)
			fixedLenBits[l] = cb + lenExtra[idx]
		}
	}

	// RFC 1951 Section 3.2.5 distance codes 0..29 (5-bit fixed codes, bit-reversed).
	dBases := [30]uint16{
		1, 2, 3, 4, 5, 7, 9, 13, 17, 25,
		33, 49, 65, 97, 129, 193, 257, 385, 513, 769,
		1025, 1537, 2049, 3073, 4097, 6145, 8193, 12289, 16385, 24577,
	}
	dExtra := [30]uint8{
		0, 0, 0, 0, 1, 1, 2, 2, 3, 3,
		4, 4, 5, 5, 6, 6, 7, 7, 8, 8,
		9, 9, 10, 10, 11, 11, 12, 12, 13, 13,
	}
	fixedDistBase = dBases
	fixedDistExtra = dExtra
	for i := 0; i < 30; i++ {
		fixedDistCode[i] = bits.Reverse8(uint8(i)) >> 3
	}
}

func fixedDistBits(dist int) (val uint32, nbits uint8) {
	if dist <= 4 {
		return uint32(fixedDistCode[dist-1]), 5
	}
	// dist >= 5: group index from highest set bit of (dist-1).
	u := uint(dist - 1)
	msb := bits.Len(u) - 1 // >= 2
	code := 2*msb + int((u>>(msb-1))&1)
	extra := uint32(dist - int(fixedDistBase[code]))
	return uint32(fixedDistCode[code]) | (extra << 5), 5 + fixedDistExtra[code]
}

// adler32UpdateNoMod advances an RFC 1950 Adler-32 state (s1, s2) over p
// without modulo reduction; caller ensures total unreduced bytes <= 5552.
func adler32UpdateNoMod(s1, s2 uint32, p []byte) (uint32, uint32) {
	for len(p) >= 32 {
		u0 := binary.LittleEndian.Uint64(p[0:8])
		u1 := binary.LittleEndian.Uint64(p[8:16])
		u2 := binary.LittleEndian.Uint64(p[16:24])
		u3 := binary.LittleEndian.Uint64(p[24:32])
		if (u0 | u1 | u2 | u3) == 0 {
			s2 += s1 << 5
			p = p[32:]
			continue
		}
		for _, w := range [4]uint64{u0, u1, u2, u3} {
			if w == 0 {
				s2 += s1 << 3
				continue
			}
			s1 += uint32(byte(w))
			s2 += s1
			s1 += uint32(byte(w >> 8))
			s2 += s1
			s1 += uint32(byte(w >> 16))
			s2 += s1
			s1 += uint32(byte(w >> 24))
			s2 += s1
			s1 += uint32(byte(w >> 32))
			s2 += s1
			s1 += uint32(byte(w >> 40))
			s2 += s1
			s1 += uint32(byte(w >> 48))
			s2 += s1
			s1 += uint32(byte(w >> 56))
			s2 += s1
		}
		p = p[32:]
	}
	for len(p) >= 8 {
		w := binary.LittleEndian.Uint64(p[:8])
		if w == 0 {
			s2 += s1 << 3
			p = p[8:]
			continue
		}
		s1 += uint32(byte(w))
		s2 += s1
		s1 += uint32(byte(w >> 8))
		s2 += s1
		s1 += uint32(byte(w >> 16))
		s2 += s1
		s1 += uint32(byte(w >> 24))
		s2 += s1
		s1 += uint32(byte(w >> 32))
		s2 += s1
		s1 += uint32(byte(w >> 40))
		s2 += s1
		s1 += uint32(byte(w >> 48))
		s2 += s1
		s1 += uint32(byte(w >> 56))
		s2 += s1
		p = p[8:]
	}
	for _, b := range p {
		s1 += uint32(b)
		s2 += s1
	}
	return s1, s2
}

// deflateFixedZlib compresses src into dst as a standard RFC 1950 zlib stream
// containing one RFC 1951 fixed-Huffman DEFLATE block, optimized for Sub-filtered
// PNG rows (fast 32-byte zero-run RLE, 4-byte hash LZ77, direct output writes,
// and fused Adler-32 calculation with deferred modulo reduction).
func deflateFixedZlib(dst []byte, src []byte, table *[4096]uint32) []byte {
	const mod = 65521
	const nmax = 5552
	n := len(src)
	// Start compact to stay L2-cache-hot; grow on demand if output exceeds cap.
	initCap := max(n>>3, 8192)
	if cap(dst) < initCap {
		dst = make([]byte, initCap)
	} else {
		dst = dst[:cap(dst)]
	}
	grow := func(need int) {
		newCap := max(len(dst)*2, need+4096)
		grown := make([]byte, newCap)
		copy(grown, dst)
		dst = grown
	}
	// Zlib header: CM=8, CINFO=7 (32K window), FLEVEL=0, FCHECK=1.
	dst[0] = 0x78
	dst[1] = 0x01
	di := 2
	clear(table[:])

	var bitBuf uint64
	var bitCnt uint8
	// Start final fixed-Huffman block: BFINAL=1, BTYPE=01 -> 0b011 (3 bits).
	bitBuf = 0b011
	bitCnt = 3

	var s1, s2 uint32 = 1, 0
	adlerRem := nmax
	// Precomputed fixed Huffman code for 258-byte zero match (dist=1):
	// fixedLenPacked[258] (8 bits) + dist=1 (5 zero bits) = 13 bits.
	code258 := uint64(fixedLenPacked[258])
	bits258 := fixedLenBits[258] + 5
	code516 := code258 | (code258 << bits258)
	bits516 := bits258 * 2

	i := 0
	for i < n {
		if di+32 > len(dst) {
			grow(di + 32)
		}
		// Fast path: runs of zero bytes (dominant in Sub-filtered PNG rows).
		if src[i] == 0 {
			// Emit one literal 0x00, then encode remaining zeros as dist=1 matches.
			bitBuf |= uint64(fixedLitCode[0]) << bitCnt
			bitCnt += fixedLitLen[0]
			if bitCnt >= 32 {
				binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
				di += 4
				bitBuf >>= 32
				bitCnt -= 32
			}
			i++
			// Count consecutive zeros using 32-byte and 8-byte loads.
			start := i
			for i+32 <= n {
				u0 := binary.LittleEndian.Uint64(src[i : i+8])
				u1 := binary.LittleEndian.Uint64(src[i+8 : i+16])
				u2 := binary.LittleEndian.Uint64(src[i+16 : i+24])
				u3 := binary.LittleEndian.Uint64(src[i+24 : i+32])
				if (u0 | u1 | u2 | u3) != 0 {
					break
				}
				i += 32
			}
			for i+8 <= n {
				w := binary.LittleEndian.Uint64(src[i : i+8])
				if w != 0 {
					i += bits.TrailingZeros64(w) >> 3
					goto doneZeroScan
				}
				i += 8
			}
			for i < n && src[i] == 0 {
				i++
			}
		doneZeroScan:
			rem := i - start
			z := rem + 1
			// Fused O(1) Adler-32 update for z zero bytes.
			if z <= adlerRem {
				s2 += s1 * uint32(z)
				adlerRem -= z
			} else {
				s1 %= mod
				s2 = uint32((uint64(s2) + uint64(s1)*uint64(z)) % mod)
				adlerRem = nmax
			}
			for rem >= 516 {
				if di+8 > len(dst) {
					grow(di + 8)
				}
				bitBuf |= code516 << bitCnt
				bitCnt += bits516
				if bitCnt >= 32 {
					binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
					di += 4
					bitBuf >>= 32
					bitCnt -= 32
				}
				rem -= 516
			}
			if rem >= 258 {
				bitBuf |= code258 << bitCnt
				bitCnt += bits258
				if bitCnt >= 32 {
					binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
					di += 4
					bitBuf >>= 32
					bitCnt -= 32
				}
				rem -= 258
			}
			if rem >= 3 {
				// Length code + dist=1 (5 zero bits).
				bitBuf |= uint64(fixedLenPacked[rem]) << bitCnt
				bitCnt += fixedLenBits[rem] + 5
				if bitCnt >= 32 {
					binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
					di += 4
					bitBuf >>= 32
					bitCnt -= 32
				}
				rem = 0
			}
			for rem > 0 {
				bitBuf |= uint64(fixedLitCode[0]) << bitCnt
				bitCnt += fixedLitLen[0]
				if bitCnt >= 32 {
					binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
					di += 4
					bitBuf >>= 32
					bitCnt -= 32
				}
				rem--
			}
			continue
		}

		// Try 4-byte hash match.
		if i+4 <= n {
			v := binary.LittleEndian.Uint32(src[i : i+4])
			h := (v * 0x1e35a7bd) >> 20 // 12-bit hash -> 0..4095
			prev := table[h]
			table[h] = uint32(i + 1)
			if prev != 0 {
				cand := int(prev - 1)
				dist := i - cand
				if dist <= 32768 && binary.LittleEndian.Uint32(src[cand:cand+4]) == v {
					maxLen := n - i
					if maxLen > 258 {
						maxLen = 258
					}
					s1s := src[i : i+maxLen]
					s2s := src[cand : cand+maxLen]
					ml := 4
					for ml+8 <= maxLen {
						diff := binary.LittleEndian.Uint64(s1s[ml:ml+8]) ^ binary.LittleEndian.Uint64(s2s[ml:ml+8])
						if diff != 0 {
							ml += bits.TrailingZeros64(diff) >> 3
							goto doneMatch
						}
						ml += 8
					}
					for ml < maxLen && s1s[ml] == s2s[ml] {
						ml++
					}
				doneMatch:
					if ml > adlerRem {
						s1 %= mod
						s2 %= mod
						adlerRem = nmax
					}
					s1, s2 = adler32UpdateNoMod(s1, s2, s1s[:ml])
					adlerRem -= ml
					dval, dbits := fixedDistBits(dist)
					bitBuf |= uint64(fixedLenPacked[ml]) << bitCnt
					bitCnt += fixedLenBits[ml]
					bitBuf |= uint64(dval) << bitCnt
					bitCnt += dbits
					if bitCnt >= 32 {
						binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
						di += 4
						bitBuf >>= 32
						bitCnt -= 32
					}
					i += ml
					continue
				}
			}
		}

		c := src[i]
		if adlerRem == 0 {
			s1 %= mod
			s2 %= mod
			adlerRem = nmax
		}
		s1 += uint32(c)
		s2 += s1
		adlerRem--
		bitBuf |= uint64(fixedLitCode[c]) << bitCnt
		bitCnt += fixedLitLen[c]
		if bitCnt >= 32 {
			binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
			di += 4
			bitBuf >>= 32
			bitCnt -= 32
		}
		i++
	}

	if di+16 > len(dst) {
		grow(di + 16)
	}
	// End-of-block symbol 256: 7 zero bits.
	bitCnt += 7
	for bitCnt >= 8 {
		dst[di] = byte(bitBuf)
		di++
		bitBuf >>= 8
		bitCnt -= 8
	}
	if bitCnt > 0 {
		dst[di] = byte(bitBuf)
		di++
	}

	// Zlib trailer: big-endian Adler-32.
	s1 %= mod
	s2 %= mod
	sum := (s2 << 16) | s1
	binary.BigEndian.PutUint32(dst[di:], sum)
	di += 4
	return dst[:di]
}

func isOpaqueRGBA(img *image.RGBA) bool {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	rowBytes := w * 4
	for y := 0; y < h; y++ {
		off := img.PixOffset(b.Min.X, b.Min.Y+y)
		row := img.Pix[off : off+rowBytes]
		i := 3
		for ; i+12 < len(row); i += 16 {
			if row[i]&row[i+4]&row[i+8]&row[i+12] != 255 {
				return false
			}
		}
		for ; i < len(row); i += 4 {
			if row[i] != 255 {
				return false
			}
		}
	}
	return true
}

func writePNGChunk(w io.Writer, tag [4]byte, data []byte) error {
	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[0:4], uint32(len(data)))
	copy(hdr[4:8], tag[:])
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	crc := crc32.Update(0, crc32.IEEETable, tag[:])
	crc = crc32.Update(crc, crc32.IEEETable, data)
	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc)
	_, err := w.Write(tail[:])
	return err
}

func filterSubOpaqueRow(dst, src []byte, width int) {
	r0, g0, b0 := src[0], src[1], src[2]
	dst[0], dst[1], dst[2] = r0, g0, b0
	if width <= 1 {
		return
	}
	clear(dst[3:])
	prevPx := binary.LittleEndian.Uint32(src[0:4])
	prevPair := uint64(prevPx) | uint64(prevPx)<<32
	pr, pg, pb := r0, g0, b0
	si, di := 4, 3
	for si+31 < len(src) && di+23 < len(dst) {
		p0 := binary.LittleEndian.Uint64(src[si : si+8])
		p1 := binary.LittleEndian.Uint64(src[si+8 : si+16])
		p2 := binary.LittleEndian.Uint64(src[si+16 : si+24])
		p3 := binary.LittleEndian.Uint64(src[si+24 : si+32])
		if ((p0 ^ prevPair) | (p1 ^ prevPair) | (p2 ^ prevPair) | (p3 ^ prevPair)) == 0 {
			si += 32
			di += 24
			continue
		}
		pairs := [4]uint64{p0, p1, p2, p3}
		for _, pair := range pairs {
			if pair == prevPair {
				di += 6
				continue
			}
			px := uint32(pair)
			if px != prevPx {
				r, g, bl := byte(px), byte(px>>8), byte(px>>16)
				dst[di] = r - pr
				dst[di+1] = g - pg
				dst[di+2] = bl - pb
				pr, pg, pb = r, g, bl
				prevPx = px
				prevPair = uint64(px) | uint64(px)<<32
			}
			px = uint32(pair >> 32)
			if px != prevPx {
				r, g, bl := byte(px), byte(px>>8), byte(px>>16)
				dst[di+3] = r - pr
				dst[di+4] = g - pg
				dst[di+5] = bl - pb
				pr, pg, pb = r, g, bl
				prevPx = px
				prevPair = uint64(px) | uint64(px)<<32
			}
			di += 6
		}
		si += 32
	}
	for si+7 < len(src) && di+5 < len(dst) {
		if binary.LittleEndian.Uint64(src[si:si+8]) == prevPair {
			si += 8
			di += 6
			continue
		}
		px := binary.LittleEndian.Uint32(src[si : si+4])
		if px != prevPx {
			r, g, bl := byte(px), byte(px>>8), byte(px>>16)
			dst[di] = r - pr
			dst[di+1] = g - pg
			dst[di+2] = bl - pb
			pr, pg, pb = r, g, bl
			prevPx = px
			prevPair = uint64(px) | uint64(px)<<32
		}
		si += 4
		di += 3
		px = binary.LittleEndian.Uint32(src[si : si+4])
		if px != prevPx {
			r, g, bl := byte(px), byte(px>>8), byte(px>>16)
			dst[di] = r - pr
			dst[di+1] = g - pg
			dst[di+2] = bl - pb
			pr, pg, pb = r, g, bl
			prevPx = px
			prevPair = uint64(px) | uint64(px)<<32
		}
		si += 4
		di += 3
	}
	for ; si+3 < len(src) && di+2 < len(dst); si, di = si+4, di+3 {
		px := binary.LittleEndian.Uint32(src[si : si+4])
		if px != prevPx {
			r, g, bl := byte(px), byte(px>>8), byte(px>>16)
			dst[di] = r - pr
			dst[di+1] = g - pg
			dst[di+2] = bl - pb
			pr, pg, pb = r, g, bl
			prevPx = px
		}
	}
}

func filterSubRGBARow(dst, src []byte) {
	var pr, pg, pb, pa byte
	for i := 0; i+3 < len(src); i += 4 {
		a := src[i+3]
		var r, g, bl byte
		if a == 255 {
			r, g, bl = src[i], src[i+1], src[i+2]
		} else if a != 0 {
			a32 := uint32(a)
			r = uint8(((uint32(src[i]) * 0xffff) / a32) >> 8)
			g = uint8(((uint32(src[i+1]) * 0xffff) / a32) >> 8)
			bl = uint8(((uint32(src[i+2]) * 0xffff) / a32) >> 8)
		}
		dst[i] = r - pr
		dst[i+1] = g - pg
		dst[i+2] = bl - pb
		dst[i+3] = a - pa
		pr, pg, pb, pa = r, g, bl, a
	}
}

// isSolidOpaqueRow reports whether every RGBA pixel in src equals the first
// pixel, returning that pixel in little-endian uint32 form.
func isSolidOpaqueRow(src []byte) (uint32, bool) {
	px := binary.LittleEndian.Uint32(src[0:4])
	pair := uint64(px) | uint64(px)<<32
	i := 0
	for i+32 <= len(src) {
		p0 := binary.LittleEndian.Uint64(src[i : i+8])
		p1 := binary.LittleEndian.Uint64(src[i+8 : i+16])
		p2 := binary.LittleEndian.Uint64(src[i+16 : i+24])
		p3 := binary.LittleEndian.Uint64(src[i+24 : i+32])
		if ((p0 ^ pair) | (p1 ^ pair) | (p2 ^ pair) | (p3 ^ pair)) != 0 {
			return 0, false
		}
		i += 32
	}
	for i+8 <= len(src) {
		if binary.LittleEndian.Uint64(src[i:i+8]) != pair {
			return 0, false
		}
		i += 8
	}
	if i+4 <= len(src) && binary.LittleEndian.Uint32(src[i:i+4]) != px {
		return 0, false
	}
	return px, true
}

// isSolidOpaqueRowColor reports whether every RGBA pixel in src equals px.
func isSolidOpaqueRowColor(src []byte, px uint32) bool {
	pair := uint64(px) | uint64(px)<<32
	i := 0
	for i+32 <= len(src) {
		p0 := binary.LittleEndian.Uint64(src[i : i+8])
		p1 := binary.LittleEndian.Uint64(src[i+8 : i+16])
		p2 := binary.LittleEndian.Uint64(src[i+16 : i+24])
		p3 := binary.LittleEndian.Uint64(src[i+24 : i+32])
		if ((p0 ^ pair) | (p1 ^ pair) | (p2 ^ pair) | (p3 ^ pair)) != 0 {
			return false
		}
		i += 32
	}
	for i+8 <= len(src) {
		if binary.LittleEndian.Uint64(src[i:i+8]) != pair {
			return false
		}
		i += 8
	}
	if i+4 <= len(src) && binary.LittleEndian.Uint32(src[i:i+4]) != px {
		return false
	}
	return true
}

// encodeFastOpaqueZlib streams opaque RGBA rows through Sub filtering and
// fixed-Huffman zlib compression with a zero-write fast path for solid rows
// and consecutive identical solid-row runs.
func encodeFastOpaqueZlib(dst []byte, raw []byte, img *image.RGBA, b image.Rectangle, width, height, rowLen, srcLen int, table *[4096]uint32) []byte {
	const mod = 65521
	const nmax = 5552
	n := len(raw)
	initCap := max(n>>3, 8192)
	if cap(dst) < initCap {
		dst = make([]byte, initCap)
	} else {
		dst = dst[:cap(dst)]
	}
	grow := func(need int) {
		newCap := max(len(dst)*2, need+4096)
		grown := make([]byte, newCap)
		copy(grown, dst)
		dst = grown
	}
	dst[0] = 0x78
	dst[1] = 0x01
	di := 2
	clear(table[:])

	var bitBuf uint64
	var bitCnt uint8
	bitBuf = 0b011
	bitCnt = 3

	var s1, s2 uint32 = 1, 0
	adlerRem := nmax

	code258 := uint64(fixedLenPacked[258])
	bits258 := fixedLenBits[258] + 5
	code516 := code258 | (code258 << bits258)
	bits516 := bits258 * 2

	emitLit := func(c byte) {
		bitBuf |= uint64(fixedLitCode[c]) << bitCnt
		bitCnt += fixedLitLen[c]
		if bitCnt >= 32 {
			binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
			di += 4
			bitBuf >>= 32
			bitCnt -= 32
		}
	}
	emitSolidRow := func(r, g, bl byte) {
		if di+32 > len(dst) {
			grow(di + 32)
		}
		emitLit(1)
		emitLit(r)
		emitLit(g)
		emitLit(bl)
		zeros := rowLen - 4
		if zeros <= 0 {
			return
		}
		emitLit(0)
		rem := zeros - 1
		for rem >= 516 {
			if di+8 > len(dst) {
				grow(di + 8)
			}
			bitBuf |= code516 << bitCnt
			bitCnt += bits516
			if bitCnt >= 32 {
				binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
				di += 4
				bitBuf >>= 32
				bitCnt -= 32
			}
			rem -= 516
		}
		if di+24 > len(dst) {
			grow(di + 24)
		}
		if rem >= 258 {
			bitBuf |= code258 << bitCnt
			bitCnt += bits258
			if bitCnt >= 32 {
				binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
				di += 4
				bitBuf >>= 32
				bitCnt -= 32
			}
			rem -= 258
		}
		if rem >= 3 {
			bitBuf |= uint64(fixedLenPacked[rem]) << bitCnt
			bitCnt += fixedLenBits[rem] + 5
			if bitCnt >= 32 {
				binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
				di += 4
				bitBuf >>= 32
				bitCnt -= 32
			}
			rem = 0
		}
		for rem > 0 {
			emitLit(0)
			rem--
		}
	}
	adlerSolidRow := func(r, g, bl byte) {
		if rowLen > adlerRem {
			s1 %= mod
			s2 %= mod
			adlerRem = nmax
		}
		s1 += 1
		s2 += s1
		s1 += uint32(r)
		s2 += s1
		s1 += uint32(g)
		s2 += s1
		s1 += uint32(bl)
		s2 += s1
		zeros := rowLen - 4
		if zeros <= 0 {
			adlerRem -= 4
			return
		}
		if zeros+4 <= adlerRem {
			s2 += s1 * uint32(zeros)
			adlerRem -= zeros + 4
		} else {
			s1 %= mod
			s2 = uint32((uint64(s2) + uint64(s1)*uint64(zeros)) % mod)
			adlerRem = nmax
		}
	}

	validBase := 0
	for y := 0; y < height; y++ {
		off := img.PixOffset(b.Min.X, b.Min.Y+y)
		srcRow := img.Pix[off : off+srcLen]
		if px, ok := isSolidOpaqueRow(srcRow); ok {
			runRows := 1
			for y+runRows < height {
				nextOff := img.PixOffset(b.Min.X, b.Min.Y+y+runRows)
				if !isSolidOpaqueRowColor(img.Pix[nextOff:nextOff+srcLen], px) {
					break
				}
				runRows++
			}
			r, g, bl := byte(px), byte(px>>8), byte(px>>16)
			emitSolidRow(r, g, bl)
			adlerSolidRow(r, g, bl)
			extra := runRows - 1
			if extra > 0 {
				for k := 0; k < extra; k++ {
					adlerSolidRow(r, g, bl)
				}
				if rowLen <= 32768 {
					dval, dbits := fixedDistBits(rowLen)
					m258 := uint64(fixedLenPacked[258]) | (uint64(dval) << fixedLenBits[258])
					b258 := fixedLenBits[258] + dbits
					total := extra * rowLen
					for total >= 258 {
						if di+8 > len(dst) {
							grow(di + 8)
						}
						bitBuf |= m258 << bitCnt
						bitCnt += b258
						if bitCnt >= 32 {
							binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
							di += 4
							bitBuf >>= 32
							bitCnt -= 32
						}
						total -= 258
					}
					if di+24 > len(dst) {
						grow(di + 24)
					}
					if total >= 3 {
						bitBuf |= uint64(fixedLenPacked[total]) << bitCnt
						bitCnt += fixedLenBits[total]
						bitBuf |= uint64(dval) << bitCnt
						bitCnt += dbits
						if bitCnt >= 32 {
							binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
							di += 4
							bitBuf >>= 32
							bitCnt -= 32
						}
						total = 0
					}
					for total > 0 {
						pos := rowLen - total
						var c byte
						switch pos {
						case 0:
							c = 1
						case 1:
							c = r
						case 2:
							c = g
						case 3:
							c = bl
						}
						emitLit(c)
						total--
					}
				} else {
					for k := 0; k < extra; k++ {
						emitSolidRow(r, g, bl)
					}
				}
			}
			y += extra
			validBase = (y + 1) * rowLen
			continue
		}

		base := y * rowLen
		rowEnd := base + rowLen
		raw[base] = 1
		filterSubOpaqueRow(raw[base+1:rowEnd], srcRow, width)

		i := base
		for i < rowEnd {
			if di+32 > len(dst) {
				grow(di + 32)
			}
			if raw[i] == 0 {
				bitBuf |= uint64(fixedLitCode[0]) << bitCnt
				bitCnt += fixedLitLen[0]
				if bitCnt >= 32 {
					binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
					di += 4
					bitBuf >>= 32
					bitCnt -= 32
				}
				i++
				start := i
				for i+32 <= rowEnd {
					u0 := binary.LittleEndian.Uint64(raw[i : i+8])
					u1 := binary.LittleEndian.Uint64(raw[i+8 : i+16])
					u2 := binary.LittleEndian.Uint64(raw[i+16 : i+24])
					u3 := binary.LittleEndian.Uint64(raw[i+24 : i+32])
					if (u0 | u1 | u2 | u3) != 0 {
						break
					}
					i += 32
				}
				for i+8 <= rowEnd {
					w := binary.LittleEndian.Uint64(raw[i : i+8])
					if w != 0 {
						i += bits.TrailingZeros64(w) >> 3
						goto doneRowZeroScan
					}
					i += 8
				}
				for i < rowEnd && raw[i] == 0 {
					i++
				}
			doneRowZeroScan:
				rem := i - start
				z := rem + 1
				if z <= adlerRem {
					s2 += s1 * uint32(z)
					adlerRem -= z
				} else {
					s1 %= mod
					s2 = uint32((uint64(s2) + uint64(s1)*uint64(z)) % mod)
					adlerRem = nmax
				}
				for rem >= 516 {
					if di+8 > len(dst) {
						grow(di + 8)
					}
					bitBuf |= code516 << bitCnt
					bitCnt += bits516
					if bitCnt >= 32 {
						binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
						di += 4
						bitBuf >>= 32
						bitCnt -= 32
					}
					rem -= 516
				}
				if rem >= 258 {
					bitBuf |= code258 << bitCnt
					bitCnt += bits258
					if bitCnt >= 32 {
						binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
						di += 4
						bitBuf >>= 32
						bitCnt -= 32
					}
					rem -= 258
				}
				if rem >= 3 {
					bitBuf |= uint64(fixedLenPacked[rem]) << bitCnt
					bitCnt += fixedLenBits[rem] + 5
					if bitCnt >= 32 {
						binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
						di += 4
						bitBuf >>= 32
						bitCnt -= 32
					}
					rem = 0
				}
				for rem > 0 {
					bitBuf |= uint64(fixedLitCode[0]) << bitCnt
					bitCnt += fixedLitLen[0]
					if bitCnt >= 32 {
						binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
						di += 4
						bitBuf >>= 32
						bitCnt -= 32
					}
					rem--
				}
				continue
			}

			if i+4 <= rowEnd {
				v := binary.LittleEndian.Uint32(raw[i : i+4])
				h := (v * 0x1e35a7bd) >> 20
				prev := table[h]
				table[h] = uint32(i + 1)
				if prev != 0 {
					cand := int(prev - 1)
					dist := i - cand
					if cand >= validBase && dist <= 32768 && binary.LittleEndian.Uint32(raw[cand:cand+4]) == v {
						maxLen := rowEnd - i
						if maxLen > 258 {
							maxLen = 258
						}
						s1s := raw[i : i+maxLen]
						s2s := raw[cand : cand+maxLen]
						ml := 4
						for ml+8 <= maxLen {
							diff := binary.LittleEndian.Uint64(s1s[ml:ml+8]) ^ binary.LittleEndian.Uint64(s2s[ml:ml+8])
							if diff != 0 {
								ml += bits.TrailingZeros64(diff) >> 3
								goto doneRowMatch
							}
							ml += 8
						}
						for ml < maxLen && s1s[ml] == s2s[ml] {
							ml++
						}
					doneRowMatch:
						if ml > adlerRem {
							s1 %= mod
							s2 %= mod
							adlerRem = nmax
						}
						s1, s2 = adler32UpdateNoMod(s1, s2, s1s[:ml])
						adlerRem -= ml
						dval, dbits := fixedDistBits(dist)
						bitBuf |= uint64(fixedLenPacked[ml]) << bitCnt
						bitCnt += fixedLenBits[ml]
						bitBuf |= uint64(dval) << bitCnt
						bitCnt += dbits
						if bitCnt >= 32 {
							binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
							di += 4
							bitBuf >>= 32
							bitCnt -= 32
						}
						i += ml
						continue
					}
				}
			}

			c := raw[i]
			if adlerRem == 0 {
				s1 %= mod
				s2 %= mod
				adlerRem = nmax
			}
			s1 += uint32(c)
			s2 += s1
			adlerRem--
			bitBuf |= uint64(fixedLitCode[c]) << bitCnt
			bitCnt += fixedLitLen[c]
			if bitCnt >= 32 {
				binary.LittleEndian.PutUint32(dst[di:], uint32(bitBuf))
				di += 4
				bitBuf >>= 32
				bitCnt -= 32
			}
			i++
		}
	}

	if di+16 > len(dst) {
		grow(di + 16)
	}
	bitCnt += 7
	for bitCnt >= 8 {
		dst[di] = byte(bitBuf)
		di++
		bitBuf >>= 8
		bitCnt -= 8
	}
	if bitCnt > 0 {
		dst[di] = byte(bitBuf)
		di++
	}
	s1 %= mod
	s2 %= mod
	sum := (s2 << 16) | s1
	binary.BigEndian.PutUint32(dst[di:], sum)
	di += 4
	return dst[:di]
}

// encodeFastRGBA writes an *image.RGBA as a standard lossless PNG using the
// Sub row filter and a fast fixed-Huffman zlib encoder (falling back to stdlib
// zlib.BestSpeed for very large images).
func encodeFastRGBA(w io.Writer, img *image.RGBA, knownOpaque bool) error {
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width <= 0 || height <= 0 {
		enc := png.Encoder{CompressionLevel: png.BestSpeed, BufferPool: &sharedPNGBufPool}
		return enc.Encode(w, img)
	}
	opaque := knownOpaque || isOpaqueRGBA(img)
	colorType := byte(6) // RGBA
	bpp := 4
	if opaque {
		colorType = 2 // RGB
		bpp = 3
	}

	s := pngEncPool.Get().(*pngEncState)
	rowLen := 1 + width*bpp
	rawTotal := int64(rowLen) * int64(height)
	srcLen := width * 4

	var idat []byte
	var encodeErr error
	if rawTotal <= maxFastPNGRaw {
		n := int(rawTotal)
		if cap(s.rawBuf) < n {
			s.rawBuf = make([]byte, n)
		}
		raw := s.rawBuf[:n]
		if opaque {
			s.idatBuf = encodeFastOpaqueZlib(s.idatBuf[:0], raw, img, b, width, height, rowLen, srcLen, &s.hash)
		} else {
			for y := 0; y < height; y++ {
				base := y * rowLen
				raw[base] = 1
				off := img.PixOffset(b.Min.X, b.Min.Y+y)
				filterSubRGBARow(raw[base+1:base+rowLen], img.Pix[off:off+srcLen])
			}
			s.idatBuf = deflateFixedZlib(s.idatBuf[:0], raw, &s.hash)
		}
		idat = s.idatBuf
	} else {
		var zbuf sliceWriter
		zbuf.buf = s.idatBuf[:0]
		if s.zw == nil {
			zw, err := zlib.NewWriterLevel(&zbuf, zlib.BestSpeed)
			if err != nil {
				return err
			}
			s.zw = zw
		} else {
			s.zw.Reset(&zbuf)
		}
		chunkCap := max(65536, rowLen)
		if cap(s.chunk) < chunkCap {
			s.chunk = make([]byte, 0, chunkCap)
		}
		chunk := s.chunk[:0]
		for y := 0; y < height; y++ {
			if len(chunk)+rowLen > cap(chunk) {
				if _, err := s.zw.Write(chunk); err != nil {
					encodeErr = err
					break
				}
				chunk = chunk[:0]
			}
			base := len(chunk)
			chunk = chunk[:base+rowLen]
			chunk[base] = 1
			off := img.PixOffset(b.Min.X, b.Min.Y+y)
			if opaque {
				filterSubOpaqueRow(chunk[base+1:base+rowLen], img.Pix[off:off+srcLen], width)
			} else {
				filterSubRGBARow(chunk[base+1:base+rowLen], img.Pix[off:off+srcLen])
			}
		}
		if encodeErr == nil && len(chunk) > 0 {
			_, encodeErr = s.zw.Write(chunk)
		}
		if encodeErr == nil {
			encodeErr = s.zw.Close()
		}
		s.idatBuf = zbuf.buf
		idat = s.idatBuf
	}

	var err error
	if encodeErr != nil {
		err = encodeErr
	} else if _, err = w.Write(pngHeader); err == nil {
		var ihdr [13]byte
		binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
		binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
		ihdr[8] = 8 // bit depth
		ihdr[9] = colorType
		if err = writePNGChunk(w, [4]byte{'I', 'H', 'D', 'R'}, ihdr[:]); err == nil {
			if err = writePNGChunk(w, [4]byte{'I', 'D', 'A', 'T'}, idat); err == nil {
				_, err = w.Write(pngIEND)
			}
		}
	}

	if cap(s.rawBuf) <= maxPooledPNGBuf && cap(s.idatBuf) <= maxPooledPNGBuf && cap(s.chunk) <= maxPooledPNGBuf {
		s.idatBuf = s.idatBuf[:0]
		pngEncPool.Put(s)
	}
	return err
}

type sliceWriter struct {
	buf []byte
}

func (w *sliceWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}
