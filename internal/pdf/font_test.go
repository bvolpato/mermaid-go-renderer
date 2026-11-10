// Original tests of SFNT parsing and ISO 32000-1 PDF font embedding.
// This file is not an upstream port.

package pdf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/webfontdata"
)

func fontTestTables() map[string][]byte {
	tables := map[string][]byte{
		"head": make([]byte, 54), "hhea": make([]byte, 36),
		"maxp": make([]byte, 32), "hmtx": make([]byte, 10),
		"loca": make([]byte, 8), "glyf": {}, "post": make([]byte, 32),
		"OS/2": make([]byte, 96), "name": fontTestName("Example#Regular", 3),
	}
	head := tables["head"]
	binary.BigEndian.PutUint32(head, 0x10000)
	binary.BigEndian.PutUint32(head[12:], 0x5f0f3cf5)
	binary.BigEndian.PutUint16(head[18:], 2048)
	for i, value := range []int16{-100, -400, 1900, 1600} {
		binary.BigEndian.PutUint16(head[36+2*i:], uint16(value))
	}
	hhea := tables["hhea"]
	binary.BigEndian.PutUint32(hhea, 0x10000)
	binary.BigEndian.PutUint16(hhea[4:], 1600)
	binary.BigEndian.PutUint16(hhea[6:], uint16(65536-400))
	binary.BigEndian.PutUint16(hhea[34:], 2)
	binary.BigEndian.PutUint32(tables["maxp"], 0x10000)
	binary.BigEndian.PutUint16(tables["maxp"][4:], 3)
	binary.BigEndian.PutUint16(tables["hmtx"], 500)
	binary.BigEndian.PutUint16(tables["hmtx"][4:], 1024)
	binary.BigEndian.PutUint16(tables["post"], 3)
	angle := int32(-819200)
	binary.BigEndian.PutUint32(tables["post"][4:], uint32(angle))
	binary.BigEndian.PutUint32(tables["post"][12:], 1)
	binary.BigEndian.PutUint16(tables["OS/2"], 2)
	tables["OS/2"][30] = 2
	binary.BigEndian.PutUint16(tables["OS/2"][88:], 1400)
	return tables
}

func fontTestName(name string, platform uint16) []byte {
	var raw []byte
	if platform == 1 {
		raw = []byte(name)
	} else {
		for _, unit := range utf16.Encode([]rune(name)) {
			raw = binary.BigEndian.AppendUint16(raw, unit)
		}
	}
	b := make([]byte, 18)
	binary.BigEndian.PutUint16(b[2:], 1)
	binary.BigEndian.PutUint16(b[4:], 18)
	binary.BigEndian.PutUint16(b[6:], platform)
	if platform == 3 {
		binary.BigEndian.PutUint16(b[8:], 1)
		binary.BigEndian.PutUint16(b[10:], 0x409)
	}
	binary.BigEndian.PutUint16(b[12:], 6)
	binary.BigEndian.PutUint16(b[14:], uint16(len(raw)))
	return append(b, raw...)
}

func fontTestSFNT(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	b := make([]byte, 12+16*len(tags))
	binary.BigEndian.PutUint32(b, 0x10000)
	binary.BigEndian.PutUint16(b[4:], uint16(len(tags)))
	for i, tag := range tags {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
		r := b[12+16*i : 28+16*i]
		copy(r, tag)
		binary.BigEndian.PutUint32(r[8:], uint32(len(b)))
		binary.BigEndian.PutUint32(r[12:], uint32(len(tables[tag])))
		b = append(b, tables[tag]...)
	}
	return b
}

func fontTestParse(t *testing.T, data []byte) *fontResource {
	t.Helper()
	f, err := parseSFNT(data)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestFontSFNTMetrics(t *testing.T) {
	data := fontTestSFNT(fontTestTables())
	f := fontTestParse(t, data)
	if !bytes.Equal(f.data, data) || f.name != "Example#Regular" || f.unitsPerEm != 2048 {
		t.Fatalf("source or name changed: name=%q units=%d", f.name, f.unitsPerEm)
	}
	if !reflect.DeepEqual(f.widths, []uint16{500, 1024, 1024}) {
		t.Fatalf("horizontal metrics: %v", f.widths)
	}
	if f.bbox != [4]int{-100, -400, 1900, 1600} || f.ascent != 1600 || f.descent != -400 || f.capHeight != 1400 || f.italicAngle != -12.5 || f.flags != 71 {
		t.Fatalf("unexpected descriptor: %+v", *f)
	}
	for _, tc := range []struct{ units, want int }{{1024, 500}, {1, 0}, {-100, -49}, {-400, -195}} {
		if got := f.scale(tc.units); got != tc.want {
			t.Errorf("scale(%d)=%d, want %d", tc.units, got, tc.want)
		}
	}
	copy(data, "true")
	fontTestParse(t, data)
}

func TestFontSources(t *testing.T) {
	for _, name := range liberation.Names() {
		t.Run(name, func(t *testing.T) {
			data, err := liberation.Font(name)
			if err != nil {
				t.Fatal(err)
			}
			f, err := newFontResource(&font.Face{Path: "embedded:" + name})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(f.data, data) {
				t.Fatal("embedded font bytes changed")
			}
		})
	}
	const webPath = "files/open-sans/files/open-sans-latin-400-normal.woff2"
	web, err := webfontdata.Font(webPath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := newFontResource(&font.Face{Path: "webfont:" + webPath})
	if err != nil || !bytes.Equal(f.data, web) {
		t.Fatalf("web font: %v", err)
	}
	path := filepath.Join(t.TempDir(), "source.ttf")
	data := fontTestSFNT(fontTestTables())
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	f, err = newFontResource(&font.Face{Path: path})
	if err != nil || !bytes.Equal(f.data, data) {
		t.Fatalf("file font: %v", err)
	}
	for _, face := range []*font.Face{
		nil, {Path: path, Index: 1}, {Path: path, Index: -1},
		{Path: path + ".missing"}, {Path: "embedded:missing.ttf"},
		{Path: "webfont:missing.woff2"},
		{Path: "webfont:files/recursive/files/recursive-latin-wght-normal.woff2"},
		{Path: "webfont:files/fontawesome-free/webfonts/fa-solid-900.woff2"},
	} {
		if _, err := newFontResource(face); err == nil {
			t.Errorf("accepted unsupported font face: %+v", face)
		}
	}
}

func TestFontCIDAllocation(t *testing.T) {
	f := fontTestParse(t, fontTestSFNT(fontTestTables()))
	for i, key := range []fontGlyphKey{{0, "missing"}, {1, "A"}, {1, "fi"}, {2, "😀"}, {2, ""}} {
		cid, ok := f.encodeGlyph(key.gid, key.text)
		if !ok || int(cid) != i+1 {
			t.Fatalf("encodeGlyph(%d,%q)=(%d,%v)", key.gid, key.text, cid, ok)
		}
		if again, ok := f.encodeGlyph(key.gid, key.text); !ok || again != cid {
			t.Fatal("repeated glyph and text received a new CID")
		}
	}
	before := len(f.glyphs)
	for _, key := range []fontGlyphKey{{3, "A"}, {0xffffffff, "A"}, {1, "\xff"}, {1, strings.Repeat("A", 257)}, {1, strings.Repeat("😀", 129)}} {
		if _, ok := f.encodeGlyph(key.gid, key.text); ok {
			t.Errorf("accepted invalid glyph or mapping: gid=%d bytes=%d", key.gid, len(key.text))
		}
	}
	if len(f.glyphs) != before {
		t.Fatal("rejected mappings consumed CIDs")
	}
	for _, text := range []string{strings.Repeat("A", 256), strings.Repeat("😀", 128)} {
		if _, ok := f.encodeGlyph(1, text); !ok {
			t.Fatal("rejected a 512-byte UTF-16 mapping")
		}
	}
	full := fontTestParse(t, fontTestSFNT(fontTestTables()))
	for i := 1; i <= 65535; i++ {
		if cid, ok := full.encodeGlyph(1, strconv.Itoa(i)); !ok || int(cid) != i {
			t.Fatalf("CID limit at %d: (%d,%v)", i, cid, ok)
		}
	}
	if _, ok := full.encodeGlyph(1, "overflow"); ok {
		t.Fatal("CID allocation wrapped around")
	}
	if cid, ok := full.encodeGlyph(1, "1"); !ok || cid != 1 {
		t.Fatal("existing CID was lost at capacity")
	}
}

func fontTestRef(t *testing.T, body []byte, key string) int {
	t.Helper()
	re := regexp.MustCompile(`/` + key + `\s+(?:\[\s*)?(\d+) 0 R`)
	m := re.FindSubmatch(body)
	if len(m) != 2 {
		t.Fatalf("missing %s reference in %s", key, body)
	}
	id, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func fontTestStream(t *testing.T, body []byte) []byte {
	t.Helper()
	marker := []byte("\nstream\n")
	pos := bytes.Index(body, marker)
	if pos < 0 {
		t.Fatalf("not a stream: %q", body)
	}
	z, err := zlib.NewReader(bytes.NewReader(body[pos+len(marker):]))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFontPDFEmbedding(t *testing.T) {
	data := fontTestSFNT(fontTestTables())
	f := fontTestParse(t, data)
	for _, key := range []fontGlyphKey{{0, "�"}, {1, "A"}, {1, "fi"}, {2, "😀"}, {2, ""}} {
		if _, ok := f.encodeGlyph(key.gid, key.text); !ok {
			t.Fatal("glyph encoding failed")
		}
	}
	var w objectWriter
	root := f.writePDF(&w)
	type0 := w.objects[root-1]
	for _, want := range []string{"/Subtype /Type0", "/Encoding /Identity-H", "/BaseFont /Example#23Regular"} {
		if !bytes.Contains(type0, []byte(want)) {
			t.Errorf("Type0 is missing %q: %s", want, type0)
		}
	}
	cidFont := w.objects[fontTestRef(t, type0, "DescendantFonts")-1]
	for _, want := range []string{"/Subtype /CIDFontType2", "/Registry (Adobe) /Ordering (Identity) /Supplement 0", "/DW 244", "/W [1 [244 500 500 500 500 ]]"} {
		if !bytes.Contains(cidFont, []byte(want)) {
			t.Errorf("CIDFont is missing %q: %s", want, cidFont)
		}
	}
	descriptor := w.objects[fontTestRef(t, cidFont, "FontDescriptor")-1]
	for _, want := range []string{"/FontName /Example#23Regular", "/Flags 71", "/FontBBox [-49 -195 928 781]", "/ItalicAngle -12.5", "/Ascent 781", "/Descent -195", "/CapHeight 684", "/StemV 80"} {
		if !bytes.Contains(descriptor, []byte(want)) {
			t.Errorf("descriptor is missing %q: %s", want, descriptor)
		}
	}
	fontFile := w.objects[fontTestRef(t, descriptor, "FontFile2")-1]
	if !bytes.Contains(fontFile, fmt.Appendf(nil, "/Length1 %d ", len(data))) || !bytes.Equal(fontTestStream(t, fontFile), data) {
		t.Fatal("FontFile2 did not preserve the complete source font or Length1")
	}
	gids := fontTestStream(t, w.objects[fontTestRef(t, cidFont, "CIDToGIDMap")-1])
	if !bytes.Equal(gids, []byte{0, 0, 0, 0, 0, 1, 0, 1, 0, 2, 0, 2}) {
		t.Fatalf("CIDToGIDMap=%x", gids)
	}
	cmap := string(fontTestStream(t, w.objects[fontTestRef(t, type0, "ToUnicode")-1]))
	for _, want := range []string{"<0000> <FFFF>", "4 beginbfchar", "<0001> <FFFD>", "<0002> <0041>", "<0003> <00660069>", "<0004> <D83DDE00>"} {
		if !strings.Contains(cmap, want) {
			t.Errorf("ToUnicode is missing %q: %s", want, cmap)
		}
	}
	if strings.Contains(cmap, "<0005>") {
		t.Fatal("empty text produced a Unicode mapping")
	}
	var again objectWriter
	if other := f.writePDF(&again); other != root || !reflect.DeepEqual(w.objects, again.objects) {
		t.Fatal("font serialization is not deterministic")
	}
}

func TestFontEmptyUnicodeAndCMapBlocks(t *testing.T) {
	f := fontTestParse(t, fontTestSFNT(fontTestTables()))
	for _, used := range []bool{false, true} {
		if used {
			f.encodeGlyph(0, "")
		}
		var w objectWriter
		root := f.writePDF(&w)
		if bytes.Contains(w.objects[root-1], []byte("/ToUnicode")) || len(f.toUnicode()) != 0 {
			t.Fatal("empty Unicode metadata produced a text map")
		}
	}
	for i := range 205 {
		f.encodeGlyph(1, fmt.Sprintf("text%d", i))
	}
	cmap := string(f.toUnicode())
	if strings.Count(cmap, "100 beginbfchar\n") != 2 || strings.Count(cmap, "5 beginbfchar\n") != 1 || strings.Count(cmap, "endbfchar\n") != 3 {
		t.Fatal("ToUnicode did not split mappings into blocks of at most 100")
	}
}

func TestFontMalformedSFNT(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string][]byte)
	}{
		{"missing head", func(m map[string][]byte) { delete(m, "head") }},
		{"short head", func(m map[string][]byte) { m["head"] = m["head"][:53] }},
		{"head magic", func(m map[string][]byte) { m["head"][12] = 0 }},
		{"zero units", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["head"][18:], 0) }},
		{"bad units", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["head"][18:], 16385) }},
		{"bbox", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["head"][36:], 2000) }},
		{"loca format", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["head"][50:], 2) }},
		{"glyph format", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["head"][52:], 1) }},
		{"maxp version", func(m map[string][]byte) { binary.BigEndian.PutUint32(m["maxp"], 0x5000) }},
		{"zero glyphs", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["maxp"][4:], 0) }},
		{"zero metrics", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["hhea"][34:], 0) }},
		{"too many metrics", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["hhea"][34:], 4) }},
		{"short hmtx tail", func(m map[string][]byte) { m["hmtx"] = m["hmtx"][:9] }},
		{"short loca", func(m map[string][]byte) { m["loca"] = m["loca"][:6] }},
		{"loca out of bounds", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["loca"][6:], 1) }},
		{"short post", func(m map[string][]byte) { m["post"] = m["post"][:31] }},
		{"short OS2", func(m map[string][]byte) { m["OS/2"] = m["OS/2"][:77] }},
		{"short OS2 version", func(m map[string][]byte) { m["OS/2"] = m["OS/2"][:95] }},
		{"restricted embedding", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["OS/2"][8:], 2) }},
		{"bitmap embedding", func(m map[string][]byte) { binary.BigEndian.PutUint16(m["OS/2"][8:], 0x200) }},
	}
	for _, tag := range []string{"fvar", "gvar", "avar", "cvar", "HVAR", "VVAR", "MVAR", "VARC", "CFF ", "CFF2", "COLR", "CPAL", "CBDT", "CBLC", "sbix", "SVG ", "EBDT", "EBLC", "bdat", "bloc"} {
		cases = append(cases, struct {
			name string
			edit func(map[string][]byte)
		}{tag, func(m map[string][]byte) { m[tag] = []byte{0} }})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fontTestTables()
			tc.edit(m)
			if _, err := parseSFNT(fontTestSFNT(m)); err == nil {
				t.Fatal("accepted malformed or unsupported SFNT")
			}
		})
	}
	data := fontTestSFNT(fontTestTables())
	for _, signature := range []string{"ttcf", "OTTO", "wOFF", "wOF2", "junk"} {
		bad := bytes.Clone(data)
		copy(bad, signature)
		if _, err := parseSFNT(bad); err == nil {
			t.Errorf("accepted signature %q", signature)
		}
	}
	for _, mutate := range []func([]byte){
		func(b []byte) { binary.BigEndian.PutUint16(b[4:], 65535) },
		func(b []byte) { binary.BigEndian.PutUint32(b[20:], 0xfffffff0) },
		func(b []byte) { binary.BigEndian.PutUint32(b[24:], 0xffffffff) },
		func(b []byte) { binary.BigEndian.PutUint32(b[20:], 4) },
		func(b []byte) { copy(b[28:32], b[12:16]) },
		func(b []byte) { copy(b[52:56], b[20:24]) },
	} {
		bad := bytes.Clone(data)
		mutate(bad)
		if _, err := parseSFNT(bad); err == nil {
			t.Error("accepted malformed SFNT directory")
		}
	}
	for end := range len(data) {
		if _, err := parseSFNT(data[:end]); err == nil {
			t.Fatalf("accepted truncated font at byte %d", end)
		}
	}
}

func TestFontGlyphValidation(t *testing.T) {
	simple := make([]byte, 20)
	binary.BigEndian.PutUint16(simple, 1)
	binary.BigEndian.PutUint16(simple[10:], 1)
	simple[14], simple[15], simple[16] = 0x3f, 1, 10
	simple[17], simple[18], simple[19] = 20, 10, 20
	composite := make([]byte, 18)
	binary.BigEndian.PutUint16(composite, 0xffff)
	binary.BigEndian.PutUint16(composite[10:], 0x0a)
	binary.BigEndian.PutUint16(composite[12:], 0)
	binary.BigEndian.PutUint16(composite[16:], 0x4000)
	for _, format := range []int16{0, 1} {
		m := fontTestTables()
		binary.BigEndian.PutUint16(m["head"][50:], uint16(format))
		m["glyf"] = append(bytes.Clone(simple), composite...)
		offsets := []uint32{0, 20, 38, 38}
		m["loca"] = nil
		for _, offset := range offsets {
			if format == 0 {
				m["loca"] = binary.BigEndian.AppendUint16(m["loca"], uint16(offset/2))
			} else {
				m["loca"] = binary.BigEndian.AppendUint32(m["loca"], offset)
			}
		}
		fontTestParse(t, fontTestSFNT(m))
	}
	if err := validateSFNTSimpleGlyph(make([]byte, 10), 0); err != nil {
		t.Fatal("rejected a zero-contour glyph without instructions:", err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"short endpoints", simple[:11]}, {"short instructions", simple[:13]},
		{"short flags", simple[:14]}, {"short repeat", simple[:15]},
		{"short coordinates", simple[:19]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateSFNTSimpleGlyph(tc.data, 1); err == nil {
				t.Fatal("accepted truncated glyph")
			}
		})
	}
	bad := bytes.Clone(simple)
	bad[15] = 2
	if err := validateSFNTSimpleGlyph(bad, 1); err == nil {
		t.Fatal("accepted a flag repeat beyond the point count")
	}
	for _, end := range []int{10, 13, 15, 17} {
		if _, err := sfntCompositeGlyph(composite[:end], 3); err == nil {
			t.Errorf("accepted a composite truncated at byte %d", end)
		}
	}
	for _, change := range []func([]byte){
		func(b []byte) { binary.BigEndian.PutUint16(b[12:], 3) },
		func(b []byte) { binary.BigEndian.PutUint16(b[10:], 0xcb) },
		func(b []byte) { binary.BigEndian.PutUint16(b[10:], 0x10b) },
	} {
		bad := bytes.Clone(composite)
		change(bad)
		if _, err := sfntCompositeGlyph(bad, 3); err == nil {
			t.Error("accepted an invalid composite glyph")
		}
	}
	m := fontTestTables()
	m["glyf"] = composite
	m["loca"] = []byte{0, 0, 0, 9, 0, 9, 0, 9}
	if _, err := parseSFNT(fontTestSFNT(m)); err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("composite cycle: %v", err)
	}
}

func TestFontSafeNameFallback(t *testing.T) {
	for _, platform := range []uint16{0, 1, 3} {
		if got := sfntPostScriptName(fontTestName("SafeName-Regular", platform)); got != "SafeName-Regular" {
			t.Errorf("platform %d name=%q", platform, got)
		}
	}
	for _, name := range []string{"", "Unsafe/Name", "Unsafe Name", "Unsafe)Name", "é", strings.Repeat("X", 64)} {
		m := fontTestTables()
		m["name"] = fontTestName(name, 3)
		f := fontTestParse(t, fontTestSFNT(m))
		if !strings.HasPrefix(f.name, "PDFFont-") || strings.ContainsAny(f.name, " /()") {
			t.Errorf("unsafe fallback name %q", f.name)
		}
	}
	m := fontTestTables()
	delete(m, "name")
	delete(m, "post")
	delete(m, "OS/2")
	f := fontTestParse(t, fontTestSFNT(m))
	if f.capHeight != f.ascent || f.flags != 4 || f.italicAngle != 0 {
		t.Fatal("invalid fallback descriptor")
	}
	data := fontTestName("Name", 3)
	for i := range len(data) {
		_ = sfntPostScriptName(data[:i])
	}
}

func TestFontMalformedInputDoesNotPanic(t *testing.T) {
	rng := rand.New(rand.NewSource(4096))
	seed := fontTestSFNT(fontTestTables())
	for range 1000 {
		data := bytes.Clone(seed)
		for range 1 + rng.Intn(8) {
			data[rng.Intn(len(data))] = byte(rng.Intn(256))
		}
		if f, err := parseSFNT(data); err == nil {
			f.encodeGlyph(0, "text😀")
			var w objectWriter
			f.writePDF(&w)
		}
		noise := make([]byte, rng.Intn(512))
		_, _ = rng.Read(noise)
		_, _ = parseSFNT(noise)
		_ = sfntPostScriptName(noise)
	}
}
