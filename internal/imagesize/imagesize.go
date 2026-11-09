// This file ports no upstream file. It reads the natural size of an image
// from its data, with the rules of Chromium that
// tools/reference/gen-image-size.mjs records. One comment names
// SVGImage::ConcreteObjectSize of Blink for the size of an SVG image. The
// class SVGImage is in
// third_party/blink/renderer/core/svg/graphics/svg_image.cc, which has the
// BSD-2-Clause license at Chromium 156.0.8078.4, with the copyright of Eric
// Seidel, Apple and Research In Motion in the file. That version has no
// function with the name ConcreteObjectSize in that file.

// Package imagesize reads the natural size of an image from its data: what
// img.naturalWidth and img.naturalHeight are in Chromium after the image
// loads. The layout engine and the image shapes of the unified renderer use
// it for data URLs, so that a render needs no network.
//
// The rules for each image format are what Chromium does
// (tools/reference/gen-image-size.mjs records them, and
// internal/ru/shapes/set3 has the test with those records).
package imagesize

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"strconv"
	"strings"
)

// Size is the natural size of an image: img.naturalWidth and
// img.naturalHeight.
type Size struct {
	Width, Height float64
}

// ErrDecode is the error of img.decode() for an image that the browser
// cannot load or cannot decode. Its text is the message of the
// EncodingError of Chromium.
var ErrDecode = errors.New("The source image cannot be decoded.")

// CleanURL returns an image source as the URL parser sees it: without the
// leading and trailing C0 control characters and spaces, and without tabs
// and newlines.
func CleanURL(src string) string {
	src = strings.TrimFunc(src, func(r rune) bool { return r <= 0x20 })
	return strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace(src)
}

// IsDataURL reports whether a cleaned image source is a data URL.
func IsDataURL(src string) bool {
	return len(src) >= 5 && strings.EqualFold(src[:5], "data:")
}

// FromDataURL returns the natural size of the image of a data URL. src is a
// cleaned source for which IsDataURL is true. The error is ErrDecode when
// Chromium cannot decode the image.
func FromDataURL(src string) (Size, error) {
	return dataURL(src[5:])
}

// dataURL reads the image of a data URL. rest is the URL after "data:". It
// follows the data URL processor of the Fetch standard.
func dataURL(rest string) (Size, error) {
	// The fragment is not part of the data.
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		rest = rest[:i]
	}
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return Size{}, ErrDecode
	}
	mimeType := strings.Trim(rest[:comma], " \t\n\f\r")
	body := PercentDecode(rest[comma+1:])
	// The type ends with ";base64", with spaces before base64 allowed.
	if i := strings.LastIndexByte(mimeType, ';'); i >= 0 && strings.EqualFold(strings.TrimLeft(mimeType[i+1:], " "), "base64") {
		decoded, ok := ForgivingBase64Decode(body)
		if !ok {
			return Size{}, ErrDecode
		}
		body = decoded
		mimeType = mimeType[:i]
	}
	essence := mimeType
	if i := strings.IndexByte(essence, ';'); i >= 0 {
		essence = essence[:i]
	}
	essence = strings.ToLower(strings.Trim(essence, " \t\n\f\r"))
	// The type selects the SVG decoder. The other decoders are selected by
	// the first bytes of the data, with any type.
	if essence == "image/svg+xml" {
		return svgImageSize(body)
	}
	return rasterImageSize(body)
}

// PercentDecode is the percent-decode of the URL standard.
func PercentDecode(s string) []byte {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			hi, ok1 := hexDigit(s[i+1])
			lo, ok2 := hexDigit(s[i+2])
			if ok1 && ok2 {
				out = append(out, hi<<4|lo)
				i += 2
				continue
			}
		}
		out = append(out, s[i])
	}
	return out
}

func hexDigit(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

// ForgivingBase64Decode is the forgiving-base64 decode of the Infra
// standard: white space is removed and the padding is optional.
func ForgivingBase64Decode(data []byte) ([]byte, bool) {
	clean := make([]byte, 0, len(data))
	for _, c := range data {
		switch c {
		case '\t', '\n', '\f', '\r', ' ':
			continue
		}
		clean = append(clean, c)
	}
	if len(clean)%4 == 0 {
		if bytes.HasSuffix(clean, []byte("==")) {
			clean = clean[:len(clean)-2]
		} else if bytes.HasSuffix(clean, []byte("=")) {
			clean = clean[:len(clean)-1]
		}
	}
	if len(clean)%4 == 1 {
		return nil, false
	}
	out, err := base64.RawStdEncoding.DecodeString(string(clean))
	if err != nil {
		return nil, false
	}
	return out, true
}

// rasterImageSize is the natural size of a bitmap image. The first bytes
// select the decoder. Each decoder needs the part of the file that Chromium
// needs before img.decode() succeeds.
func rasterImageSize(data []byte) (Size, error) {
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return pngImageSize(data)
	case bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}):
		return jpegImageSize(data)
	case bytes.HasPrefix(data, []byte("GIF87a")), bytes.HasPrefix(data, []byte("GIF89a")):
		return gifImageSize(data)
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return webpImageSize(data)
	case bytes.HasPrefix(data, []byte("BM")):
		return bmpImageSize(data)
	}
	return Size{}, ErrDecode
}

func size(w, h uint32) (Size, error) {
	if w == 0 || h == 0 {
		return Size{}, ErrDecode
	}
	return Size{Width: float64(w), Height: float64(h)}, nil
}

// pngImageSize reads the header of a PNG file. The decoder gives the size
// when it reaches the first image data chunk, so the file must have every
// chunk before that one, and the start of that one.
func pngImageSize(data []byte) (Size, error) {
	const headerEnd = 8 + 4 + 4 + 13 + 4
	if len(data) < headerEnd || binary.BigEndian.Uint32(data[8:12]) != 13 || string(data[12:16]) != "IHDR" {
		return Size{}, ErrDecode
	}
	if crc32.ChecksumIEEE(data[12:29]) != binary.BigEndian.Uint32(data[29:33]) {
		return Size{}, ErrDecode
	}
	width := binary.BigEndian.Uint32(data[16:20])
	height := binary.BigEndian.Uint32(data[20:24])
	bitDepth, colorType := data[24], data[25]
	validDepth := false
	switch colorType {
	case 0:
		validDepth = bitDepth == 1 || bitDepth == 2 || bitDepth == 4 || bitDepth == 8 || bitDepth == 16
	case 3:
		validDepth = bitDepth == 1 || bitDepth == 2 || bitDepth == 4 || bitDepth == 8
	case 2, 4, 6:
		validDepth = bitDepth == 8 || bitDepth == 16
	}
	if !validDepth || data[26] != 0 || data[27] != 0 || data[28] > 1 || width > math.MaxInt32 || height > math.MaxInt32 {
		return Size{}, ErrDecode
	}
	for pos := headerEnd; ; {
		if pos+8 > len(data) {
			return Size{}, ErrDecode
		}
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		switch string(data[pos+4 : pos+8]) {
		case "IDAT":
			return size(width, height)
		case "IEND":
			return Size{}, ErrDecode
		}
		if length < 0 || pos+12+length > len(data) {
			return Size{}, ErrDecode
		}
		pos += 12 + length
	}
}

// jpegImageSize reads the headers of a JPEG file up to the start of the
// first scan, which is what the decoder needs. An Exif orientation that
// turns the image by a quarter swaps the width and the height.
func jpegImageSize(data []byte) (Size, error) {
	var width, height uint32
	swap := false
	for pos := 2; ; {
		if pos+2 > len(data) || data[pos] != 0xff {
			return Size{}, ErrDecode
		}
		marker := data[pos+1]
		if marker == 0xff {
			// A fill byte.
			pos++
			continue
		}
		pos += 2
		if marker == 0x01 || (marker >= 0xd0 && marker <= 0xd7) {
			continue
		}
		if marker == 0xd9 || pos+2 > len(data) {
			return Size{}, ErrDecode
		}
		length := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if length < 2 || pos+length > len(data) {
			return Size{}, ErrDecode
		}
		segment := data[pos+2 : pos+length]
		switch marker {
		case 0xc0, 0xc1, 0xc2:
			if len(segment) < 5 {
				return Size{}, ErrDecode
			}
			height = uint32(binary.BigEndian.Uint16(segment[1:3]))
			width = uint32(binary.BigEndian.Uint16(segment[3:5]))
		case 0xe1:
			if o := exifOrientation(segment); o >= 5 && o <= 8 {
				swap = true
			}
		case 0xda:
			if swap {
				width, height = height, width
			}
			return size(width, height)
		}
		pos += length
	}
}

// exifOrientation reads the orientation tag of an Exif segment, or 0.
func exifOrientation(segment []byte) int {
	if !bytes.HasPrefix(segment, []byte("Exif\x00\x00")) {
		return 0
	}
	tiff := segment[6:]
	if len(tiff) < 8 {
		return 0
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 0
	}
	offset := int(order.Uint32(tiff[4:8]))
	if offset < 0 || offset+2 > len(tiff) {
		return 0
	}
	count := int(order.Uint16(tiff[offset : offset+2]))
	for i := 0; i < count; i++ {
		entry := offset + 2 + 12*i
		if entry+12 > len(tiff) {
			return 0
		}
		if order.Uint16(tiff[entry:entry+2]) == 0x0112 {
			return int(order.Uint16(tiff[entry+8 : entry+10]))
		}
	}
	return 0
}

// gifImageSize reads a GIF file up to the first image, or up to the
// trailer of a file with no image. The natural size is the size of the
// logical screen, made larger when the first image does not fit in it.
func gifImageSize(data []byte) (Size, error) {
	if len(data) < 13 {
		return Size{}, ErrDecode
	}
	width := uint32(binary.LittleEndian.Uint16(data[6:8]))
	height := uint32(binary.LittleEndian.Uint16(data[8:10]))
	pos := 13
	if flags := data[10]; flags&0x80 != 0 {
		pos += 3 << (flags&7 + 1)
	}
	for {
		if pos >= len(data) {
			return Size{}, ErrDecode
		}
		switch data[pos] {
		case 0x2c:
			// The image descriptor: the position and the size of the frame.
			if pos+9 > len(data) {
				return Size{}, ErrDecode
			}
			x := uint32(binary.LittleEndian.Uint16(data[pos+1 : pos+3]))
			y := uint32(binary.LittleEndian.Uint16(data[pos+3 : pos+5]))
			w := uint32(binary.LittleEndian.Uint16(data[pos+5 : pos+7]))
			h := uint32(binary.LittleEndian.Uint16(data[pos+7 : pos+9]))
			if w == 0 || h == 0 {
				// A frame with no size has the size of the screen.
				w, h = width, height
			}
			return size(max(width, x+w), max(height, y+h))
		case 0x3b:
			// The trailer: the file has no image.
			return size(width, height)
		case 0x21:
			// An extension: a label and data blocks up to an empty block.
			pos += 2
			for {
				if pos >= len(data) {
					return Size{}, ErrDecode
				}
				n := int(data[pos])
				pos += 1 + n
				if n == 0 {
					break
				}
			}
		default:
			return Size{}, ErrDecode
		}
	}
}

// webpImageSize reads the header of a WebP file. The decoder needs the
// whole file.
func webpImageSize(data []byte) (Size, error) {
	if len(data) < 20 || uint64(binary.LittleEndian.Uint32(data[4:8]))+8 > uint64(len(data)) {
		return Size{}, ErrDecode
	}
	payload := data[20:]
	switch string(data[12:16]) {
	case "VP8X":
		if len(payload) < 10 {
			return Size{}, ErrDecode
		}
		w := uint32(payload[4]) | uint32(payload[5])<<8 | uint32(payload[6])<<16
		h := uint32(payload[7]) | uint32(payload[8])<<8 | uint32(payload[9])<<16
		return size(w+1, h+1)
	case "VP8 ":
		if len(payload) < 10 || payload[3] != 0x9d || payload[4] != 0x01 || payload[5] != 0x2a {
			return Size{}, ErrDecode
		}
		w := uint32(binary.LittleEndian.Uint16(payload[6:8])) & 0x3fff
		h := uint32(binary.LittleEndian.Uint16(payload[8:10])) & 0x3fff
		return size(w, h)
	case "VP8L":
		if len(payload) < 5 || payload[0] != 0x2f {
			return Size{}, ErrDecode
		}
		bits := binary.LittleEndian.Uint32(payload[1:5])
		return size(bits&0x3fff+1, bits>>14&0x3fff+1)
	}
	return Size{}, ErrDecode
}

// bmpImageSize reads the headers of a BMP file.
func bmpImageSize(data []byte) (Size, error) {
	if len(data) < 18 {
		return Size{}, ErrDecode
	}
	infoSize := int(binary.LittleEndian.Uint32(data[14:18]))
	if infoSize < 12 || len(data) < 14+infoSize {
		return Size{}, ErrDecode
	}
	if infoSize == 12 {
		return size(uint32(binary.LittleEndian.Uint16(data[18:20])), uint32(binary.LittleEndian.Uint16(data[20:22])))
	}
	if infoSize < 16 {
		return Size{}, ErrDecode
	}
	w := int32(binary.LittleEndian.Uint32(data[18:22]))
	h := int32(binary.LittleEndian.Uint32(data[22:26]))
	if h < 0 {
		// A negative height is a top-down bitmap.
		h = -h
	}
	if w <= 0 || h <= 0 {
		return Size{}, ErrDecode
	}
	return Size{Width: float64(w), Height: float64(h)}, nil
}

const svgNamespace = "http://www.w3.org/2000/svg"

// svgImageSize is the natural size of an SVG image: the concrete object
// size of the outermost svg element for the default object size of 300 by
// 150 (SVGImage::ConcreteObjectSize in Blink), rounded. The document must
// be well-formed XML with an svg element of the SVG namespace as its root.
func svgImageSize(data []byte) (Size, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	var root *xml.StartElement
	depth := 0
	closed := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Size{}, ErrDecode
		}
		switch t := token.(type) {
		case xml.StartElement:
			if closed {
				return Size{}, ErrDecode
			}
			if root == nil {
				if t.Name.Local != "svg" || t.Name.Space != svgNamespace {
					return Size{}, ErrDecode
				}
				start := t.Copy()
				root = &start
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				closed = true
			}
		case xml.CharData:
			// Text outside the root element must be white space.
			if depth == 0 && strings.Trim(string(t), " \t\r\n") != "" {
				return Size{}, ErrDecode
			}
		}
	}
	if root == nil || !closed {
		return Size{}, ErrDecode
	}

	attr := func(name string) (string, bool) {
		for _, a := range root.Attr {
			if a.Name.Space == "" && a.Name.Local == name {
				return a.Value, true
			}
		}
		return "", false
	}
	var width, height float32
	hasWidth, hasHeight := false, false
	if v, ok := attr("width"); ok {
		width, hasWidth = svgLength(v)
	}
	if v, ok := attr("height"); ok {
		height, hasHeight = svgLength(v)
	}
	// The aspect ratio of the view box, as its width and its height.
	var ratioW, ratioH float32
	if v, ok := attr("viewBox"); ok {
		if box, ok := svgNumberList(v); ok && len(box) == 4 && box[2] > 0 && box[3] > 0 {
			ratioW, ratioH = box[2], box[3]
		}
	}
	hasRatio := ratioW > 0
	alignNone := false
	if v, ok := attr("preserveAspectRatio"); ok {
		fields := strings.Fields(v)
		alignNone = len(fields) > 0 && fields[0] == "none"
	}

	const defaultWidth, defaultHeight = float32(300), float32(150)
	var w, h float32
	switch {
	case hasWidth && hasHeight:
		w, h = width, height
	case alignNone:
		// The aspect ratio does not give a missing size when
		// preserveAspectRatio is none.
		w, h = defaultWidth, defaultHeight
	case hasWidth:
		w, h = width, defaultHeight
		if hasRatio {
			h = width * ratioH / ratioW
		}
	case hasHeight:
		w, h = defaultWidth, height
		if hasRatio {
			w = height * ratioW / ratioH
		}
	case hasRatio:
		// The largest size with the ratio that fits the default size.
		if defaultWidth*ratioH > defaultHeight*ratioW {
			w, h = defaultHeight*ratioW/ratioH, defaultHeight
		} else {
			w, h = defaultWidth, defaultWidth*ratioH/ratioW
		}
	default:
		w, h = defaultWidth, defaultHeight
	}
	return Size{Width: roundImageSize(w), Height: roundImageSize(h)}, nil
}

// roundImageSize rounds a size to a whole number of pixels. Chromium keeps
// sizes in layout units, which have a largest value.
func roundImageSize(v float32) float64 {
	const largest = 33554428
	r := math.Floor(float64(v) + 0.5)
	if r > largest {
		return largest
	}
	return r
}

// svgUnits are the lengths of the units in px, for the default font of
// the browser at 16px. The viewport units are 0: an image has no viewport.
var svgUnits = map[string]float64{
	"": 1, "px": 1, "pt": 96.0 / 72, "pc": 16, "in": 96, "cm": 96 / 2.54, "mm": 96 / 25.4, "q": 96 / 101.6,
	"em": 16, "rem": 16, "ic": 16, "ch": 8, "lh": 18, "ex": 7.1796875, "cap": 10.5859375,
	"vw": 0, "vh": 0, "vmin": 0, "vmax": 0,
}

// svgLength parses the width or the height attribute of the svg element.
// The second result is false for a percentage and for a value that is not
// a length: the size is then not specified. A negative length is 0.
func svgLength(value string) (float32, bool) {
	value = strings.Trim(value, " \t\r\n\f")
	number, rest, ok := svgNumber(value)
	if !ok {
		return 0, false
	}
	factor, known := svgUnits[strings.ToLower(rest)]
	if !known {
		return 0, false
	}
	px := float32(number * factor)
	if px < 0 {
		px = 0
	}
	return px, true
}

// svgNumber parses a number at the start of s: a sign, digits with an
// optional fraction, and an optional exponent. A dot needs a digit after
// it. It returns the number and the rest of s.
func svgNumber(s string) (float64, string, bool) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		fraction := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			fraction++
		}
		if fraction == 0 {
			return 0, "", false
		}
		digits += fraction
	}
	if digits == 0 {
		return 0, "", false
	}
	// An exponent needs a digit. Without one, the letter starts the unit.
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	number, err := strconv.ParseFloat(s[:i], 64)
	return number, s[i:], err == nil
}

// svgNumberList parses numbers that white space or commas separate.
func svgNumberList(s string) ([]float32, bool) {
	var out []float32
	for _, field := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '\f' }) {
		number, rest, ok := svgNumber(field)
		if !ok || rest != "" {
			return nil, false
		}
		out = append(out, float32(number))
	}
	return out, true
}
