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

// adler32Fast computes RFC 1950 Adler-32 with 32-byte and 8-byte zero-run
// skipping (dominant in Sub-filtered PNG rows).
func adler32Fast(p []byte) uint32 {
	const mod = 65521
	const nmax = 5552
	var s1, s2 uint32 = 1, 0
	for len(p) > 0 {
		n := len(p)
		if n > nmax {
			n = nmax
		}
		q := p[:n]
		p = p[n:]
		for len(q) >= 32 {
			u0 := binary.LittleEndian.Uint64(q[0:8])
			u1 := binary.LittleEndian.Uint64(q[8:16])
			u2 := binary.LittleEndian.Uint64(q[16:24])
			u3 := binary.LittleEndian.Uint64(q[24:32])
			if (u0 | u1 | u2 | u3) == 0 {
				s2 += s1 << 5
				q = q[32:]
				continue
			}
			for k := 0; k < 4; k++ {
				if binary.LittleEndian.Uint64(q[:8]) == 0 {
					s2 += s1 << 3
					q = q[8:]
					continue
				}
				s1 += uint32(q[0])
				s2 += s1
				s1 += uint32(q[1])
				s2 += s1
				s1 += uint32(q[2])
				s2 += s1
				s1 += uint32(q[3])
				s2 += s1
				s1 += uint32(q[4])
				s2 += s1
				s1 += uint32(q[5])
				s2 += s1
				s1 += uint32(q[6])
				s2 += s1
				s1 += uint32(q[7])
				s2 += s1
				q = q[8:]
			}
		}
		for len(q) >= 8 {
			if binary.LittleEndian.Uint64(q[:8]) == 0 {
				s2 += s1 << 3
				q = q[8:]
				continue
			}
			s1 += uint32(q[0])
			s2 += s1
			s1 += uint32(q[1])
			s2 += s1
			s1 += uint32(q[2])
			s2 += s1
			s1 += uint32(q[3])
			s2 += s1
			s1 += uint32(q[4])
			s2 += s1
			s1 += uint32(q[5])
			s2 += s1
			s1 += uint32(q[6])
			s2 += s1
			s1 += uint32(q[7])
			s2 += s1
			q = q[8:]
		}
		for _, b := range q {
			s1 += uint32(b)
			s2 += s1
		}
		s1 %= mod
		s2 %= mod
	}
	return (s2 << 16) | s1
}

// deflateFixedZlib compresses src into dst as a standard RFC 1950 zlib stream
// containing one RFC 1951 fixed-Huffman DEFLATE block, optimized for Sub-filtered
// PNG rows (fast 32-byte zero-run RLE + 4-byte hash LZ77).
func deflateFixedZlib(dst []byte, src []byte, table *[4096]uint32) []byte {
	// Zlib header: CM=8, CINFO=7 (32K window), FLEVEL=0, FCHECK=1.
	dst = append(dst, 0x78, 0x01)
	clear(table[:])

	var bitBuf uint64
	var bitCnt uint8
	// Start final fixed-Huffman block: BFINAL=1, BTYPE=01 -> 0b011 (3 bits).
	bitBuf = 0b011
	bitCnt = 3

	n := len(src)
	i := 0
	for i < n {
		// Fast path: runs of zero bytes (dominant in Sub-filtered PNG rows).
		if src[i] == 0 {
			// Emit one literal 0x00, then encode remaining zeros as dist=1 matches.
			bitBuf |= uint64(fixedLitCode[0]) << bitCnt
			bitCnt += fixedLitLen[0]
			if bitCnt >= 32 {
				dst = append(dst, byte(bitBuf), byte(bitBuf>>8), byte(bitBuf>>16), byte(bitBuf>>24))
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
			for rem >= 3 {
				ml := rem
				if ml > 258 {
					ml = 258
				}
				// Length code + dist=1 (5 zero bits).
				bitBuf |= uint64(fixedLenPacked[ml]) << bitCnt
				bitCnt += fixedLenBits[ml] + 5
				if bitCnt >= 32 {
					dst = append(dst, byte(bitBuf), byte(bitBuf>>8), byte(bitBuf>>16), byte(bitBuf>>24))
					bitBuf >>= 32
					bitCnt -= 32
				}
				rem -= ml
			}
			for rem > 0 {
				bitBuf |= uint64(fixedLitCode[0]) << bitCnt
				bitCnt += fixedLitLen[0]
				if bitCnt >= 32 {
					dst = append(dst, byte(bitBuf), byte(bitBuf>>8), byte(bitBuf>>16), byte(bitBuf>>24))
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
					s1 := src[i : i+maxLen]
					s2 := src[cand : cand+maxLen]
					ml := 4
					for ml+8 <= maxLen {
						diff := binary.LittleEndian.Uint64(s1[ml:ml+8]) ^ binary.LittleEndian.Uint64(s2[ml:ml+8])
						if diff != 0 {
							ml += bits.TrailingZeros64(diff) >> 3
							goto doneMatch
						}
						ml += 8
					}
					for ml < maxLen && s1[ml] == s2[ml] {
						ml++
					}
				doneMatch:
					dval, dbits := fixedDistBits(dist)
					bitBuf |= uint64(fixedLenPacked[ml]) << bitCnt
					bitCnt += fixedLenBits[ml]
					bitBuf |= uint64(dval) << bitCnt
					bitCnt += dbits
					if bitCnt >= 32 {
						dst = append(dst, byte(bitBuf), byte(bitBuf>>8), byte(bitBuf>>16), byte(bitBuf>>24))
						bitBuf >>= 32
						bitCnt -= 32
					}
					i += ml
					continue
				}
			}
		}

		c := src[i]
		bitBuf |= uint64(fixedLitCode[c]) << bitCnt
		bitCnt += fixedLitLen[c]
		if bitCnt >= 32 {
			dst = append(dst, byte(bitBuf), byte(bitBuf>>8), byte(bitBuf>>16), byte(bitBuf>>24))
			bitBuf >>= 32
			bitCnt -= 32
		}
		i++
	}

	// End-of-block symbol 256: 7 zero bits.
	bitCnt += 7
	for bitCnt >= 8 {
		dst = append(dst, byte(bitBuf))
		bitBuf >>= 8
		bitCnt -= 8
	}
	if bitCnt > 0 {
		dst = append(dst, byte(bitBuf))
	}

	// Zlib trailer: big-endian Adler-32.
	sum := adler32Fast(src)
	dst = append(dst, byte(sum>>24), byte(sum>>16), byte(sum>>8), byte(sum))
	return dst
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
		for k := 0; k < 4; k++ {
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
			for y := 0; y < height; y++ {
				base := y * rowLen
				raw[base] = 1
				off := img.PixOffset(b.Min.X, b.Min.Y+y)
				filterSubOpaqueRow(raw[base+1:base+rowLen], img.Pix[off:off+srcLen], width)
			}
		} else {
			for y := 0; y < height; y++ {
				base := y * rowLen
				raw[base] = 1
				off := img.PixOffset(b.Min.X, b.Min.Y+y)
				filterSubRGBARow(raw[base+1:base+rowLen], img.Pix[off:off+srcLen])
			}
		}
		s.idatBuf = deflateFixedZlib(s.idatBuf[:0], raw, &s.hash)
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
