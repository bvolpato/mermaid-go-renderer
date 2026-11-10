// Original implementation of SFNT parsing and ISO 32000-1 PDF font embedding.
// This file is not an upstream port.

package pdf

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/webfontdata"
)

type fontGlyphKey struct {
	gid  uint32
	text string
}

type fontResource struct {
	data                       []byte
	name                       string
	unitsPerEm                 int
	widths                     []uint16
	bbox                       [4]int
	ascent, descent, capHeight int
	flags, stemV               int
	italicAngle                float64
	cids                       map[fontGlyphKey]uint16
	glyphs                     []fontGlyphKey
}

func newFontResource(face *font.Face) (*fontResource, error) {
	if face == nil {
		return nil, fmt.Errorf("pdf: nil font face")
	}
	if face.Index != 0 {
		return nil, fmt.Errorf("pdf: font collection index %d is unsupported", face.Index)
	}
	var data []byte
	var err error
	switch {
	case strings.HasPrefix(face.Path, "embedded:"):
		data, err = liberation.Font(strings.TrimPrefix(face.Path, "embedded:"))
	case strings.HasPrefix(face.Path, "webfont:"):
		data, err = webfontdata.Font(strings.TrimPrefix(face.Path, "webfont:"))
	default:
		data, err = os.ReadFile(face.Path)
	}
	if err != nil {
		return nil, fmt.Errorf("pdf: load font %q: %w", face.Path, err)
	}
	f, err := parseSFNT(data)
	if err != nil {
		return nil, fmt.Errorf("pdf: font %q: %w", face.Path, err)
	}
	return f, nil
}

type sfntTable struct {
	tag    string
	offset uint64
	length uint64
}

// parseSFNT reads only static TrueType outlines. The source bytes stay intact.
func parseSFNT(data []byte) (*fontResource, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("truncated SFNT header")
	}
	switch string(data[:4]) {
	case "ttcf":
		return nil, fmt.Errorf("font collections are unsupported")
	case "OTTO":
		return nil, fmt.Errorf("CFF fonts are unsupported")
	case "\x00\x01\x00\x00", "true":
	default:
		return nil, fmt.Errorf("unsupported SFNT signature %q", data[:4])
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	end := 12 + 16*n
	if n == 0 || end > len(data) {
		return nil, fmt.Errorf("invalid SFNT table directory")
	}
	tables := make(map[string][]byte, n)
	ranges := make([]sfntTable, 0, n)
	for i := range n {
		r := data[12+16*i : 28+16*i]
		tag := string(r[:4])
		if _, exists := tables[tag]; exists {
			return nil, fmt.Errorf("duplicate SFNT table %q", tag)
		}
		offset := uint64(binary.BigEndian.Uint32(r[8:]))
		length := uint64(binary.BigEndian.Uint32(r[12:]))
		if offset > uint64(len(data)) || length > uint64(len(data))-offset || (length != 0 && offset < uint64(end)) {
			return nil, fmt.Errorf("SFNT table %q is out of bounds", tag)
		}
		switch tag {
		case "fvar", "gvar", "avar", "cvar", "HVAR", "VVAR", "MVAR", "VARC":
			return nil, fmt.Errorf("variable font table %q is unsupported", tag)
		case "CFF ", "CFF2":
			return nil, fmt.Errorf("CFF table %q is unsupported", tag)
		case "COLR", "CPAL", "CBDT", "CBLC", "sbix", "SVG ", "EBDT", "EBLC", "bdat", "bloc":
			return nil, fmt.Errorf("color or bitmap font table %q is unsupported", tag)
		}
		tables[tag] = data[int(offset):int(offset+length)]
		if length != 0 {
			ranges = append(ranges, sfntTable{tag, offset, length})
		}
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].offset < ranges[j].offset })
	for i := 1; i < len(ranges); i++ {
		if ranges[i].offset < ranges[i-1].offset+ranges[i-1].length {
			return nil, fmt.Errorf("overlapping SFNT tables %q and %q", ranges[i-1].tag, ranges[i].tag)
		}
	}
	for _, req := range []struct {
		tag string
		min int
	}{{"head", 54}, {"hhea", 36}, {"maxp", 32}, {"hmtx", 4}, {"loca", 4}, {"glyf", 0}} {
		b, ok := tables[req.tag]
		if !ok || len(b) < req.min {
			return nil, fmt.Errorf("missing or truncated SFNT table %q", req.tag)
		}
	}
	head, hhea, maxp := tables["head"], tables["hhea"], tables["maxp"]
	if binary.BigEndian.Uint32(head) != 0x10000 || binary.BigEndian.Uint32(head[12:]) != 0x5f0f3cf5 {
		return nil, fmt.Errorf("invalid head table")
	}
	upem := int(binary.BigEndian.Uint16(head[18:]))
	locFormat := int16(binary.BigEndian.Uint16(head[50:]))
	if upem < 16 || upem > 16384 || (locFormat != 0 && locFormat != 1) || binary.BigEndian.Uint16(head[52:]) != 0 {
		return nil, fmt.Errorf("unsupported head metrics or glyph format")
	}
	if binary.BigEndian.Uint32(hhea) != 0x10000 || binary.BigEndian.Uint16(hhea[32:]) != 0 || binary.BigEndian.Uint32(maxp) != 0x10000 {
		return nil, fmt.Errorf("unsupported hhea or maxp format")
	}
	nGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
	nMetrics := int(binary.BigEndian.Uint16(hhea[34:]))
	if nGlyphs == 0 || nMetrics == 0 || nMetrics > nGlyphs || len(tables["hmtx"]) < 4*nMetrics+2*(nGlyphs-nMetrics) {
		return nil, fmt.Errorf("invalid horizontal font metrics")
	}
	if err := validateSFNTGlyphs(tables["loca"], tables["glyf"], nGlyphs, locFormat); err != nil {
		return nil, err
	}
	f := &fontResource{
		data: data, unitsPerEm: upem, widths: make([]uint16, nGlyphs),
		ascent: sfntInt16(hhea[4:]), descent: sfntInt16(hhea[6:]),
		flags: 4, stemV: 80, cids: make(map[fontGlyphKey]uint16),
		glyphs: []fontGlyphKey{{}},
	}
	for i := range f.bbox {
		f.bbox[i] = sfntInt16(head[36+2*i:])
	}
	if f.bbox[0] > f.bbox[2] || f.bbox[1] > f.bbox[3] {
		return nil, fmt.Errorf("invalid font bounding box")
	}
	for i := range f.widths {
		metric := min(i, nMetrics-1)
		f.widths[i] = binary.BigEndian.Uint16(tables["hmtx"][4*metric:])
	}
	f.capHeight = f.ascent
	if os2, ok := tables["OS/2"]; ok {
		if len(os2) < 78 {
			return nil, fmt.Errorf("truncated OS/2 table")
		}
		version := binary.BigEndian.Uint16(os2)
		fsType := binary.BigEndian.Uint16(os2[8:])
		if fsType&0x0e == 2 || (version >= 2 && fsType&0x200 != 0) {
			return nil, fmt.Errorf("font embedding is restricted by OS/2 fsType")
		}
		if version >= 1 && len(os2) < 86 || version >= 2 && len(os2) < 96 || version >= 5 && len(os2) < 100 {
			return nil, fmt.Errorf("truncated OS/2 table version %d", version)
		}
		if version > 5 {
			return nil, fmt.Errorf("unsupported OS/2 table version %d", version)
		}
		if version >= 2 && sfntInt16(os2[88:]) > 0 {
			f.capHeight = sfntInt16(os2[88:])
		}
		familyClass := os2[30]
		if familyClass >= 1 && familyClass <= 7 {
			f.flags |= 2
		}
	}
	if post, ok := tables["post"]; ok {
		if len(post) < 32 {
			return nil, fmt.Errorf("truncated post table")
		}
		f.italicAngle = float64(int32(binary.BigEndian.Uint32(post[4:]))) / 65536
		if binary.BigEndian.Uint32(post[12:]) != 0 {
			f.flags |= 1
		}
	}
	if f.italicAngle != 0 || binary.BigEndian.Uint16(head[44:])&2 != 0 {
		f.flags |= 64
	}
	f.name = sfntPostScriptName(tables["name"])
	if f.name == "" {
		hash := sha256.Sum256(data)
		f.name = fmt.Sprintf("PDFFont-%x", hash[:8])
	}
	return f, nil
}

func sfntInt16(b []byte) int { return int(int16(binary.BigEndian.Uint16(b))) }

func validateSFNTGlyphs(loca, glyf []byte, count int, format int16) error {
	size := 2
	if format == 1 {
		size = 4
	}
	if len(loca) < (count+1)*size {
		return fmt.Errorf("truncated loca table")
	}
	locations := make([]uint32, count+1)
	for i := range locations {
		if size == 2 {
			locations[i] = 2 * uint32(binary.BigEndian.Uint16(loca[2*i:]))
		} else {
			locations[i] = binary.BigEndian.Uint32(loca[4*i:])
		}
		if uint64(locations[i]) > uint64(len(glyf)) || (i > 0 && locations[i] < locations[i-1]) {
			return fmt.Errorf("invalid loca offset for glyph %d", i)
		}
	}
	components := make([][]uint16, count)
	for gid := range count {
		b := glyf[locations[gid]:locations[gid+1]]
		if len(b) == 0 {
			continue
		}
		if len(b) < 10 || sfntInt16(b[2:]) > sfntInt16(b[6:]) || sfntInt16(b[4:]) > sfntInt16(b[8:]) {
			return fmt.Errorf("invalid header for glyph %d", gid)
		}
		var err error
		if contours := sfntInt16(b); contours >= 0 {
			err = validateSFNTSimpleGlyph(b, contours)
		} else if contours == -1 {
			components[gid], err = sfntCompositeGlyph(b, count)
		} else {
			err = fmt.Errorf("unsupported contour count")
		}
		if err != nil {
			return fmt.Errorf("invalid glyph %d: %w", gid, err)
		}
	}
	// Walk iteratively so a malformed composite cannot exhaust the call stack.
	state := make([]uint8, count)
	type visit struct{ gid, next int }
	var stack []visit
	for gid := range count {
		if state[gid] != 0 {
			continue
		}
		state[gid] = 1
		stack = append(stack, visit{gid: gid})
		for len(stack) != 0 {
			v := &stack[len(stack)-1]
			if v.next == len(components[v.gid]) {
				state[v.gid] = 2
				stack = stack[:len(stack)-1]
				continue
			}
			child := int(components[v.gid][v.next])
			v.next++
			if state[child] == 1 {
				return fmt.Errorf("cyclic composite glyph %d", child)
			}
			if state[child] == 0 {
				state[child] = 1
				stack = append(stack, visit{gid: child})
			}
		}
	}
	return nil
}

func validateSFNTSimpleGlyph(b []byte, contours int) error {
	if contours == 0 && len(b) == 10 {
		return nil
	}
	pos := 10 + 2*contours
	if pos+2 > len(b) {
		return fmt.Errorf("truncated contour endpoints")
	}
	points := 0
	for i := range contours {
		end := int(binary.BigEndian.Uint16(b[10+2*i:])) + 1
		if end <= points {
			return fmt.Errorf("unordered contour endpoints")
		}
		points = end
	}
	pos += 2 + int(binary.BigEndian.Uint16(b[pos:]))
	if pos > len(b) {
		return fmt.Errorf("truncated glyph instructions")
	}
	coordinates := 0
	for point := 0; point < points; {
		if pos >= len(b) {
			return fmt.Errorf("truncated glyph flags")
		}
		flag := b[pos]
		pos++
		repeat := 1
		if flag&8 != 0 {
			if pos >= len(b) {
				return fmt.Errorf("truncated glyph flag repeat")
			}
			repeat += int(b[pos])
			pos++
		}
		if point+repeat > points {
			return fmt.Errorf("glyph flag repeat exceeds point count")
		}
		for _, axis := range []struct{ short, same byte }{{2, 16}, {4, 32}} {
			if flag&axis.short != 0 {
				coordinates += repeat
			} else if flag&axis.same == 0 {
				coordinates += 2 * repeat
			}
		}
		point += repeat
	}
	if coordinates > len(b)-pos {
		return fmt.Errorf("truncated glyph coordinates")
	}
	return nil
}

func sfntCompositeGlyph(b []byte, count int) ([]uint16, error) {
	pos := 10
	var components []uint16
	var instructions bool
	for {
		if pos+4 > len(b) {
			return nil, fmt.Errorf("truncated composite component")
		}
		flags := binary.BigEndian.Uint16(b[pos:])
		gid := binary.BigEndian.Uint16(b[pos+2:])
		if int(gid) >= count {
			return nil, fmt.Errorf("composite component glyph %d is out of range", gid)
		}
		components = append(components, gid)
		pos += 6
		if flags&1 != 0 {
			pos += 2
		}
		switch flags & (8 | 64 | 128) {
		case 0:
		case 8:
			pos += 2
		case 64:
			pos += 4
		case 128:
			pos += 8
		default:
			return nil, fmt.Errorf("conflicting composite transforms")
		}
		if pos > len(b) {
			return nil, fmt.Errorf("truncated composite arguments or transform")
		}
		instructions = instructions || flags&256 != 0
		if flags&32 == 0 {
			break
		}
	}
	if instructions && (pos+2 > len(b) || int(binary.BigEndian.Uint16(b[pos:])) > len(b)-pos-2) {
		return nil, fmt.Errorf("truncated composite instructions")
	}
	return components, nil
}

// sfntPostScriptName uses a valid name ID 6, or lets the caller use a hash.
func sfntPostScriptName(b []byte) string {
	if len(b) < 6 || binary.BigEndian.Uint16(b) > 1 {
		return ""
	}
	count := int(binary.BigEndian.Uint16(b[2:]))
	storage := int(binary.BigEndian.Uint16(b[4:]))
	if 6+12*count > len(b) || storage < 6+12*count || storage > len(b) {
		return ""
	}
	best, bestScore := "", -1
	for i := range count {
		r := b[6+12*i : 18+12*i]
		if binary.BigEndian.Uint16(r[6:]) != 6 {
			continue
		}
		length := int(binary.BigEndian.Uint16(r[8:]))
		offset := storage + int(binary.BigEndian.Uint16(r[10:]))
		if offset > len(b) || length > len(b)-offset {
			continue
		}
		platform, encoding := binary.BigEndian.Uint16(r), binary.BigEndian.Uint16(r[2:])
		raw := b[offset : offset+length]
		name, score := "", 0
		switch {
		case platform == 0 || platform == 3 && (encoding == 0 || encoding == 1 || encoding == 10):
			if len(raw)%2 != 0 {
				continue
			}
			units := make([]uint16, len(raw)/2)
			for j := range units {
				units[j] = binary.BigEndian.Uint16(raw[2*j:])
			}
			name = string(utf16.Decode(units))
			score = 1
			if platform == 3 && binary.BigEndian.Uint16(r[4:]) == 0x409 {
				score = 2
			}
		case platform == 1 && encoding == 0:
			name = string(raw)
		default:
			continue
		}
		if len(name) == 0 || len(name) > 63 {
			continue
		}
		valid := true
		for _, c := range name {
			if c < 33 || c > 126 || strings.ContainsRune("[](){}<>/%", c) {
				valid = false
				break
			}
		}
		if valid && score > bestScore {
			best, bestScore = name, score
		}
	}
	return best
}

func (f *fontResource) encodeGlyph(gid uint32, text string) (uint16, bool) {
	if gid >= uint32(len(f.widths)) || !utf8.ValidString(text) {
		return 0, false
	}
	// ISO 32000-1 limits each ToUnicode destination string to 512 bytes.
	units := 0
	for _, r := range text {
		units++
		if r > 0xffff {
			units++
		}
		if units > 256 {
			return 0, false
		}
	}
	key := fontGlyphKey{gid: gid, text: text}
	if cid, ok := f.cids[key]; ok {
		return cid, true
	}
	// CID zero is reserved for the missing glyph. Assigned CIDs start at one.
	if len(f.glyphs) > 65535 {
		return 0, false
	}
	cid := uint16(len(f.glyphs))
	f.cids[key] = cid
	f.glyphs = append(f.glyphs, key)
	return cid, true
}

func (f *fontResource) writePDF(w *objectWriter) int {
	name := strings.ReplaceAll(f.name, "#", "#23")
	file := w.addStream(fmt.Sprintf("/Length1 %d", len(f.data)), f.data)
	descriptor := w.addObject(fmt.Sprintf(
		"<< /Type /FontDescriptor /FontName /%s /Flags %d /FontBBox [%d %d %d %d] /ItalicAngle %s /Ascent %d /Descent %d /CapHeight %d /StemV %d /FontFile2 %d 0 R >>",
		name, f.flags, f.scale(f.bbox[0]), f.scale(f.bbox[1]), f.scale(f.bbox[2]), f.scale(f.bbox[3]),
		strconv.FormatFloat(f.italicAngle, 'f', -1, 64), f.scale(f.ascent), f.scale(f.descent), f.scale(f.capHeight), f.stemV, file))
	gidMap := make([]byte, 2*len(f.glyphs))
	var widths strings.Builder
	if len(f.glyphs) > 1 {
		widths.WriteString(" /W [1 [")
		for cid, key := range f.glyphs[1:] {
			binary.BigEndian.PutUint16(gidMap[2*(cid+1):], uint16(key.gid))
			fmt.Fprintf(&widths, "%d ", f.scale(int(f.widths[key.gid])))
		}
		widths.WriteString("]]")
	}
	mapping := w.addStream("", gidMap)
	descendant := w.addObject(fmt.Sprintf(
		"<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /DW %d%s /CIDToGIDMap %d 0 R >>",
		name, descriptor, f.scale(int(f.widths[0])), widths.String(), mapping))
	toUnicode := ""
	if cmap := f.toUnicode(); len(cmap) != 0 {
		id := w.addStream("", cmap)
		toUnicode = fmt.Sprintf(" /ToUnicode %d 0 R", id)
	}
	return w.addObject(fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R]%s >>", name, descendant, toUnicode))
}

func (f *fontResource) scale(value int) int {
	n := int64(value) * 1000
	if n < 0 {
		return -int((-n + int64(f.unitsPerEm)/2) / int64(f.unitsPerEm))
	}
	return int((n + int64(f.unitsPerEm)/2) / int64(f.unitsPerEm))
}

func (f *fontResource) toUnicode() []byte {
	var entries []string
	for cid, key := range f.glyphs {
		if key.text == "" {
			continue
		}
		var entry strings.Builder
		fmt.Fprintf(&entry, "<%04X> <", cid)
		for _, unit := range utf16.Encode([]rune(key.text)) {
			fmt.Fprintf(&entry, "%04X", unit)
		}
		entry.WriteString(">\n")
		entries = append(entries, entry.String())
	}
	if len(entries) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for start := 0; start < len(entries); start += 100 {
		end := min(start+100, len(entries))
		fmt.Fprintf(&b, "%d beginbfchar\n", end-start)
		for _, entry := range entries[start:end] {
			b.WriteString(entry)
		}
		b.WriteString("endbfchar\n")
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return []byte(b.String())
}
