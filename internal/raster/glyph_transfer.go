// The transfer from glyph coverage to mask values. Ported from
// skia/src/core/SkMaskGamma.cpp, SkMaskGamma.h, SkScalerContext.cpp
// (PreprocessRec, setLuminanceColor), src/ports/SkTypeface_mac_ct.cpp
// (onFilterRec) and src/ports/SkScalerContext_mac_ct.cpp (generateImage)
// of the Skia branch chrome/m156.
//
// Skia on macOS makes an 8-bit mask in three steps:
//
//  1. CoreGraphics draws the glyph in black on white with font smoothing.
//  2. Skia squares each pixel value (it assumes that CoreGraphics applied a
//     gamma of 2) and takes the difference to white as the coverage.
//  3. Skia sends the coverage through a table that depends on the
//     luminance of the text color (the "pre-blend"), so that the blend in
//     the non-linear space of the bitmap looks like a linear blend.
//     Chromium builds the tables with the sRGB transfer function and no
//     contrast, and Skia halves the luminance first.
//
// Chromium on Linux fills the outline of a glyph with the scan converter
// of Skia (SkScalerContext::GenerateImageFromPath) and applies only step 3,
// with a gamma of 1.2 and a contrast of 0.2 (SK_GAMMA_EXPONENT and
// SK_GAMMA_CONTRAST of the Chromium build), and without halving the
// luminance.
//
// Step 1 is the only unknown. The model is that the pixel value of
// CoreGraphics is linear in the coverage of the grown outline. With it the
// flat glyphs of the calibration sheet differ from Chromium by 0.3 of 255
// (root mean square, TestFitGrowth). The two other modes are for
// experiments: they give stems whose measured width changes with the
// subpixel position.

package raster

import (
	"math"
	"sync"
)

// transferMode selects the transfer from coverage to mask values.
type transferMode uint8

const (
	// transferCG is the chain of Skia on macOS with a linear CoreGraphics
	// pixel value.
	transferCG transferMode = iota
	// transferLinear makes the mask value equal to the coverage.
	transferLinear
	// transferPreBlend applies only the pre-blend table: the model in
	// which CoreGraphics applies the gamma that Skia assumes.
	transferPreBlend
)

const lumBits = 3

// scale255 is sk_t_scale255<3>: it repeats 3 bits over 8 bits.
func scale255(v uint8) uint8 {
	base := uint32(v) << (8 - lumBits)
	lum := base
	for sh := uint(lumBits); sh < 8; sh += lumBits {
		lum |= base >> sh
	}
	return uint8(lum)
}

// canonicalLum is SkMaskGamma::CanonicalColor for one channel.
func canonicalLum(v uint8) uint8 { return scale255(v >> (8 - lumBits)) }

// lumBucket returns the index of the pre-blend table of a text color. Skia
// cuts the color to 3 bits per channel (SkScalerContextRec::
// setLuminanceColor), halves it (SkTypeface_Mac::onFilterRec), cuts it
// again, takes the luminance (SkScalerContext::PreprocessRec) and uses its
// top 3 bits.
func lumBucket(r, g, b uint8) uint8 {
	if model.Transfer == transferLinear {
		return 0
	}
	r = canonicalLum(canonicalLum(r) / 2)
	g = canonicalLum(canonicalLum(g) / 2)
	b = canonicalLum(canonicalLum(b) / 2)
	return computeLuminance(r, g, b) >> (8 - lumBits)
}

// lumBucketSkia is lumBucket for a typeface that does not change the
// luminance color: the FreeType and Fontations typefaces of Linux.
func lumBucketSkia(r, g, b uint8) uint8 {
	return computeLuminance(canonicalLum(r), canonicalLum(g), canonicalLum(b)) >> (8 - lumBits)
}

// computeLuminance is SkComputeLuminance.
func computeLuminance(r, g, b uint8) uint8 {
	return uint8((uint32(r)*54 + uint32(g)*183 + uint32(b)*19) >> 8)
}

func srgbToLuma(v float32) float32 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return float32(math.Pow(float64((v+0.055)/1.055), float64(float32(2.4))))
}

func srgbFromLuma(v float32) float32 {
	if v <= 0.0031308 {
		return float32(v * 12.92)
	}
	return float32(1.055*float32(math.Pow(float64(v), float64(float32(1/float32(2.4)))))) - 0.055
}

var (
	transferOnce sync.Once
	// preBlendTables are the tables of SkTMaskGamma for macOS, one per
	// luminance: the sRGB transfer function and no contrast.
	preBlendTables [1 << lumBits][256]uint8
	// cgTables map the darkness of a CoreGraphics pixel (255 minus its
	// value) to the mask value, for each luminance: steps 2 and 3.
	cgTables [1 << lumBits][256]uint8
	// linuxTables are the tables of SkTMaskGamma for Linux: a gamma of 1.2
	// and a contrast of 0.2.
	linuxTables [1 << lumBits][256]uint8
)

// The text gamma and contrast of the Chromium build for Linux.
const (
	linuxTextGamma    = float32(1.2)
	linuxTextContrast = float32(0.2)
)

// buildCorrectingLUT is SkTMaskGamma_build_correcting_lut: the table for
// one luminance of the text color. toLuma and fromLuma are the
// SkColorSpaceLuminance of the device gamma.
func buildCorrectingLUT(table *[256]uint8, lum uint8, contrast float32, toLuma, fromLuma func(float32) float32) {
	src := float32(lum) / 255
	linSrc := toLuma(src)
	// The guess of the destination: the perceptual inverse of the source.
	dst := 1 - src
	linDst := toLuma(dst)
	// The contrast tapers off to 0 as the source becomes white.
	adjusted := float32(contrast * linDst)
	applyContrast := func(srca float32) float32 {
		return srca + float32(float32(float32(1-srca)*adjusted)*srca)
	}
	if float32(math.Abs(float64(src-dst))) < 1.0/256.0 {
		for j := 0; j < 256; j++ {
			srca := applyContrast(float32(j) / 255)
			table[j] = uint8(math.Floor(float64(float32(255*srca)) + 0.5))
		}
		return
	}
	for j := 0; j < 256; j++ {
		srca := applyContrast(float32(j) / 255)
		dsta := 1 - srca
		linOut := float32(linSrc*srca) + float32(dsta*linDst)
		out := fromLuma(linOut)
		result := (out - dst) / (src - dst)
		table[j] = uint8(math.Floor(float64(float32(255*result)) + 0.5))
	}
}

// buildTransferTables is the constructor of SkTMaskGamma for the two
// platforms, followed by the tables of the CoreGraphics chain.
func buildTransferTables() {
	gammaTo := func(v float32) float32 { return float32(math.Pow(float64(v), float64(linuxTextGamma))) }
	gammaFrom := func(v float32) float32 {
		return float32(math.Pow(float64(v), float64(float32(1/linuxTextGamma))))
	}
	for i := 0; i < 1<<lumBits; i++ {
		lum := scale255(uint8(i))
		buildCorrectingLUT(&preBlendTables[i], lum, 0, srgbToLuma, srgbFromLuma)
		buildCorrectingLUT(&linuxTables[i], lum, linuxTextContrast, gammaTo, gammaFrom)
		table := &preBlendTables[i]
		for j := 0; j < 256; j++ {
			// sk_pow2_table of the pixel value, then rgb_to_a8.
			p := uint32(255 - j)
			lin := (p*p + 128) / 255
			cgTables[i][j] = table[255-lin]
		}
	}
}

// applyLinuxPreBlend sends the values of a mask through the table of a
// text luminance (applyLUTToA8Mask).
func applyLinuxPreBlend(mask []uint8, bucket uint8) {
	transferOnce.Do(buildTransferTables)
	table := &linuxTables[bucket&(1<<lumBits-1)]
	for i, v := range mask {
		mask[i] = table[v]
	}
}

// coverageToMask converts coverage values from 0 to 1 into the mask values
// for a text color with the given table index.
func coverageToMask(dst []uint8, cov []float32, bucket uint8) {
	transferOnce.Do(buildTransferTables)
	var table *[256]uint8
	switch model.Transfer {
	case transferCG:
		table = &cgTables[bucket&(1<<lumBits-1)]
	case transferPreBlend:
		table = &preBlendTables[bucket&(1<<lumBits-1)]
	}
	for i, c := range cov {
		v := uint8(float32(c*255) + 0.5)
		if table != nil {
			v = table[v]
		}
		dst[i] = v
	}
}
