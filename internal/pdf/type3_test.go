// These tests validate Type 3 outline embedding, Unicode extraction, and rights.
// They do not port an upstream test suite.
package pdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/webfontdata"
	ot "github.com/go-text/typesetting/font/opentype"
)

func type3Recursive(t *testing.T, weight int) *font.Face {
	t.Helper()
	face := font.Embedded().WithWebFonts().Resolve([]string{"Recursive Variable"}, weight, false).Primary()
	if face == nil || !strings.HasPrefix(face.Path, "webfont:files/recursive/") {
		t.Fatalf("Recursive font did not load: %+v", face)
	}
	return face
}

func TestType3VariableFont(t *testing.T) {
	face := type3Recursive(t, 400)
	if _, err := newFontResource(face); err == nil {
		t.Fatal("Recursive unexpectedly has static TrueType outlines")
	}
	f, err := newType3Resource(face)
	if err != nil {
		t.Fatal(err)
	}
	gid, ok := face.NominalGlyph('A')
	if !ok {
		t.Fatal("missing A")
	}
	for i, text := range []string{"A", "fi", "😀", ""} {
		code, ok := f.encodeGlyph(uint32(gid), text)
		if !ok || code != uint16(i+1) || f.glyphCode(code) != fmt.Sprintf("%02X", code) {
			t.Fatalf("encodeGlyph(%q) = (%d, %v)", text, code, ok)
		}
		if again, ok := f.encodeGlyph(uint32(gid), text); !ok || again != code {
			t.Fatal("duplicate glyph/text pair consumed a character code")
		}
	}
	for _, text := range []string{"\xff", strings.Repeat("A", 257), strings.Repeat("😀", 129)} {
		if _, ok := f.encodeGlyph(uint32(gid), text); ok {
			t.Fatalf("accepted invalid mapping of %d bytes", len(text))
		}
	}
	if _, ok := f.encodeGlyph(f.glyphCount, "A"); ok {
		t.Fatal("accepted out-of-range glyph")
	}
	for _, text := range []string{strings.Repeat("A", 256), strings.Repeat("😀", 128)} {
		if _, ok := f.encodeGlyph(uint32(gid), text); !ok {
			t.Fatal("rejected a valid 512-byte UTF-16BE mapping")
		}
	}
	var w objectWriter
	ref := f.writePDF(&w)
	dict := w.objects[ref-1]
	cmap := string(fontTestStream(t, w.objects[fontTestRef(t, dict, "ToUnicode")-1]))
	for _, want := range []string{"<00> <FF>", "<01> <0041>", "<02> <00660069>", "<03> <D83DDE00>"} {
		if !strings.Contains(cmap, want) {
			t.Errorf("missing ToUnicode mapping %q", want)
		}
	}
	if strings.Contains(cmap, "<04>") {
		t.Fatal("empty source text produced a Unicode mapping")
	}
	// Different source sequences for one outline share the CharProc stream.
	if strings.Count(string(dict), " 1 0 R") != len(f.keys) || len(f.outlines) != 1 {
		t.Fatalf("outline was duplicated: %s", dict)
	}
	glyph := fontTestStream(t, w.objects[0])
	if !bytes.Contains(glyph, []byte(" d1\n")) || !bytes.HasSuffix(glyph, []byte("h\nf\n")) {
		t.Fatal("glyph program did not declare bounds and paint its closed outline")
	}
}

func TestType3EmbeddingRights(t *testing.T) {
	for _, tc := range []struct {
		version, flags uint16
		allowed        bool
	}{
		{0, 0, true}, {1, 0x300, true}, {0, 2, false},
		{2, 0, true}, {2, 4, true}, {2, 8, true}, {2, 6, true}, {2, 14, true},
		{2, 0x100, false}, {2, 0x200, false}, {2, 0x304, false},
		{3, 0, true}, {3, 2, false}, {3, 4, true}, {3, 8, true}, {3, 6, false}, {3, 12, false},
		{4, 0x100, false}, {4, 0x200, false}, {5, 8, true}, {6, 0, false},
	} {
		t.Run(fmt.Sprintf("v%d-%04x", tc.version, tc.flags), func(t *testing.T) {
			os2 := make([]byte, 100)
			binary.BigEndian.PutUint16(os2, tc.version)
			binary.BigEndian.PutUint16(os2[8:], tc.flags)
			err := type3EmbeddingRights(os2)
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v, error=%v", tc.allowed, err)
			}
		})
	}
	for _, tc := range []struct{ version, length int }{{0, 77}, {1, 85}, {2, 95}, {5, 99}} {
		os2 := make([]byte, tc.length)
		binary.BigEndian.PutUint16(os2, uint16(tc.version))
		if err := type3EmbeddingRights(os2); err == nil {
			t.Errorf("accepted truncated version %d", tc.version)
		}
	}
}

func type3Collection(t *testing.T, sources ...[]byte) []byte {
	t.Helper()
	data := make([]byte, 12+4*len(sources))
	copy(data, "ttcf")
	binary.BigEndian.PutUint32(data[4:], 0x10000)
	binary.BigEndian.PutUint32(data[8:], uint32(len(sources)))
	for i, source := range sources {
		for len(data)%4 != 0 {
			data = append(data, 0)
		}
		offset := len(data)
		binary.BigEndian.PutUint32(data[12+4*i:], uint32(offset))
		data = append(data, source...)
		for j := range int(binary.BigEndian.Uint16(source[4:])) {
			record := data[offset+12+16*j : offset+28+16*j]
			binary.BigEndian.PutUint32(record[8:], binary.BigEndian.Uint32(record[8:])+uint32(offset))
		}
	}
	return data
}

func TestType3CFFAndCollection(t *testing.T) {
	const cffPath = "files/fontawesome-free/webfonts/fa-solid-900.woff2"
	data, err := webfontdata.Font(cffPath)
	if err != nil {
		t.Fatal(err)
	}
	faces, err := font.LoadFor(font.PlatformLinux, data)
	if err != nil {
		t.Fatal(err)
	}
	face := faces[0]
	face.Path = "webfont:" + cffPath
	f, err := newType3Resource(face)
	if err != nil {
		t.Fatal(err)
	}
	gid, ok := face.NominalGlyph('\uf1b9')
	if !ok {
		t.Fatal("missing car icon")
	}
	if _, ok := f.encodeGlyph(uint32(gid), "car"); !ok || f.outlines[uint32(gid)].path == "" {
		t.Fatal("CFF outline could not be embedded")
	}

	regular, err := liberation.Font("LiberationSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	bold, err := liberation.Font("LiberationSans-Bold.ttf")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "faces.ttc")
	data = type3Collection(t, regular, bold)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	faces, err = font.LoadFile(path)
	if err != nil || len(faces) != 2 {
		t.Fatalf("load collection: %v", err)
	}
	for _, face := range faces {
		f, err := newType3Resource(face)
		if err != nil {
			t.Fatal(err)
		}
		gid, _ := face.NominalGlyph('A')
		if _, ok := f.encodeGlyph(uint32(gid), "A"); !ok {
			t.Fatalf("collection face %d could not be embedded", face.Index)
		}
	}
	// Rights belong to the selected collection face, rather than face zero.
	bold = bytes.Clone(bold)
	for i := range int(binary.BigEndian.Uint16(bold[4:])) {
		r := bold[12+16*i : 28+16*i]
		if string(r[:4]) == "OS/2" {
			offset := binary.BigEndian.Uint32(r[8:])
			binary.BigEndian.PutUint16(bold[offset+8:], 0x100)
		}
	}
	if err := os.WriteFile(path, type3Collection(t, regular, bold), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newType3Resource(faces[0]); err != nil {
		t.Fatalf("rights of face one affected face zero: %v", err)
	}
	if _, err := newType3Resource(faces[1]); err == nil {
		t.Fatal("ignored no-subsetting right on selected collection face")
	}
}

func TestType3OutlineCurves(t *testing.T) {
	point := func(x, y float32) ot.SegmentPoint { return ot.SegmentPoint{X: x, Y: y} }
	segments := []ot.Segment{
		{Op: ot.SegmentOpMoveTo, Args: [3]ot.SegmentPoint{point(0, 0)}},
		{Op: ot.SegmentOpQuadTo, Args: [3]ot.SegmentPoint{point(3, 6), point(6, 0)}},
		{Op: ot.SegmentOpCubeTo, Args: [3]ot.SegmentPoint{point(7, -2), point(8, -2), point(9, 0)}},
		{Op: ot.SegmentOpMoveTo, Args: [3]ot.SegmentPoint{point(1, 1)}},
		{Op: ot.SegmentOpLineTo, Args: [3]ot.SegmentPoint{point(2, 1)}},
	}
	path, bbox, ok := type3Outline(segments, 2)
	if !ok || path != "0 0 m\n4 8 8 8 12 0 c\n14 -4 16 -4 18 0 c\nh\n2 2 m\n4 2 l\nh\nf\n" || bbox != [4]float64{0, -4, 18, 8} {
		t.Fatalf("curve conversion: %q, bbox=%v, ok=%v", path, bbox, ok)
	}
	segments[0].Args[0].X = float32(math.Inf(1))
	if _, _, ok := type3Outline(segments, 1); ok {
		t.Fatal("accepted non-finite outline coordinates")
	}
}

func TestType3MalformedGlyph(t *testing.T) {
	data, err := liberation.Font("LiberationSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Clone(data)
	face := font.Embedded().Resolve([]string{"Arial"}, 400, false).Primary()
	gid, _ := face.NominalGlyph('A')
	tables := make(map[string][]byte)
	for i := range int(binary.BigEndian.Uint16(data[4:])) {
		r := data[12+16*i : 28+16*i]
		offset, length := binary.BigEndian.Uint32(r[8:]), binary.BigEndian.Uint32(r[12:])
		tables[string(r[:4])] = data[offset : offset+length]
	}
	var offset uint32
	if binary.BigEndian.Uint16(tables["head"][50:]) == 1 {
		offset = binary.BigEndian.Uint32(tables["loca"][4*gid:])
	} else {
		offset = 2 * uint32(binary.BigEndian.Uint16(tables["loca"][2*gid:]))
	}
	glyph := tables["glyf"][offset:]
	clear(glyph[:17])
	binary.BigEndian.PutUint16(glyph, 2)
	// Descending contour endpoints pass the source loader but the outline
	// decoder indexes past the single point allocated from the last endpoint.
	binary.BigEndian.PutUint16(glyph[10:], 1)
	glyph[16] = 0x31
	path := filepath.Join(t.TempDir(), "malformed.ttf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	faces, err := font.LoadFor(font.PlatformDarwin, data)
	if err != nil {
		t.Fatal(err)
	}
	faces[0].Path = path
	f, err := newType3Resource(faces[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.encodeGlyph(uint32(gid), "A"); ok {
		t.Fatal("accepted malformed glyph outline")
	}
}

func TestType3PopplerExtractionAndChunks(t *testing.T) {
	tool, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("pdftotext is not installed")
	}
	face := type3Recursive(t, 400)
	bold := type3Recursive(t, 800)
	if face.Path != bold.Path || face == bold {
		t.Fatal("test requires distinct variations of the same font file")
	}
	doc := New(700, 8000)
	style := Style{Color: Color{A: 255}}
	for row, face := range []*font.Face{face, bold} {
		x := 10.0
		for _, r := range "Read Write café" {
			gid, _ := face.NominalGlyph(r)
			if !doc.Glyph(face, uint32(gid), string(r), 18, x, float64(30+row*30), style) {
				t.Fatalf("default font glyph %q was outlined", r)
			}
			x += 14
		}
	}
	if len(doc.fonts) != 2 {
		t.Fatalf("distinct variations shared a PDF resource: %d", len(doc.fonts))
	}
	a, _ := face.NominalGlyph('A')
	for i := range 270 {
		text := fmt.Sprintf("mapping%03d", i)
		if !doc.Glyph(face, uint32(a), text, 18, 10, float64(100+i*25), style) {
			t.Fatalf("glyph mapping %d did not roll over to a new Type 3 resource", i)
		}
	}
	for i, text := range []string{"fi", "😀", "é", "e\u0301"} {
		if !doc.Glyph(face, uint32(a), text, 18, 300, float64(100+i*25), style) {
			t.Fatalf("could not embed Unicode mapping %q", text)
		}
	}
	if len(doc.fonts) != 3 {
		t.Fatalf("expected one rollover resource: %d", len(doc.fonts))
	}
	regular := doc.fonts[0].(*type3Resource)
	boldResource := doc.fonts[1].(*type3Resource)
	r, _ := face.NominalGlyph('R')
	if regular.outlines[uint32(r)].path == boldResource.outlines[uint32(r)].path {
		t.Fatal("distinct variation instances produced the same outline")
	}
	data, err := doc.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "type3.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(tool, "-raw", path, "-").CombinedOutput()
	if err != nil {
		t.Fatalf("pdftotext: %v: %s", err, out)
	}
	for _, text := range []string{"mapping000", "mapping254", "mapping255", "mapping269", "fi", "😀", "é", "e\u0301"} {
		if !strings.Contains(string(out), text) {
			t.Errorf("extraction lost %q: %q", text, out)
		}
	}
}
