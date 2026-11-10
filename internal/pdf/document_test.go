// These tests validate the original ISO 32000-1 writer with Poppler.
// This file does not port an upstream test suite.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/font/liberation"
)

func writeDocument(t *testing.T, d *Document) string {
	t.Helper()
	data, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "drawing.pdf")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func poppler(t *testing.T, tool string, args ...string) []byte {
	t.Helper()
	path, err := exec.LookPath(tool)
	if err != nil {
		t.Skipf("%s is unavailable", tool)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	var errors bytes.Buffer
	cmd.Stderr = &errors
	out, err := cmd.Output()
	if err != nil || errors.Len() != 0 {
		t.Fatalf("%s: %v: %s", tool, err, errors.Bytes())
	}
	return out
}

func pageImage(t *testing.T, path string, page int) image.Image {
	t.Helper()
	out := poppler(t, "pdftoppm", "-r", "96", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), "-singlefile", "-png", path)
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func pixel(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if math.Abs(float64(got.R)-float64(want.R)) > 2 || math.Abs(float64(got.G)-float64(want.G)) > 2 || math.Abs(float64(got.B)-float64(want.B)) > 2 {
		t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

func TestVectorStateAndGroups(t *testing.T) {
	d := New(140, 140)
	d.Save()
	d.Transform(Matrix{1, 0, 0, 1, 10, 10})
	d.Save()
	d.Transform(Matrix{2, 0, 0, 1, 0, 0})
	d.Clip("0 0 10 20 re", false)
	d.Transform(Matrix{1, 0, 0, 1, -10, 0})
	d.Draw("0 0 40 40 re", false, Style{Color: Color{R: 255, A: 255}})
	d.Restore()
	d.BeginGroup(1, false)
	d.Draw("0 30 50 20 re", false, Style{Color: Color{R: 255, A: 255}})
	d.Draw("25 30 50 20 re", false, Style{Color: Color{R: 255, A: 255}})
	d.EndGroupWithOpacity(.5, false)
	d.BeginGroup(.5, false)
	d.BeginGroup(.5, false)
	d.Draw("0 60 75 20 re", false, Style{Color: Color{B: 255, A: 255}})
	d.EndGroup()
	d.EndGroup()
	d.Draw("0 90 75 20 re", false, Style{Color: Color{R: 255, G: 255, A: 255}})
	d.BeginGroup(1, false)
	d.Draw("0 90 75 20 re", false, Style{Color: Color{G: 255, B: 255, A: 255}})
	d.EndGroupWithOpacity(.5, true)
	d.Restore()
	d.Draw("100 10 30 30 re 110 20 10 10 re", true, Style{Color: Color{G: 255, A: 255}})
	d.Draw("100 60 m 100 120 l", false, Style{Color: Color{A: 255}, Stroke: true, Width: 6, Cap: 1, Join: 2, Miter: 10, Dash: []float64{10, 20}})
	img := pageImage(t, writeDocument(t, d), 1)
	if img.Bounds().Dx() != 140 || img.Bounds().Dy() != 140 {
		t.Fatalf("page bounds = %v", img.Bounds())
	}
	for _, tc := range []struct {
		x, y int
		want color.NRGBA
	}{
		{15, 15, color.NRGBA{R: 255, A: 255}},
		{35, 15, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{20, 45, color.NRGBA{R: 255, G: 127, B: 127, A: 255}},
		{45, 45, color.NRGBA{R: 255, G: 127, B: 127, A: 255}},
		{20, 75, color.NRGBA{R: 191, G: 191, B: 255, A: 255}},
		{20, 105, color.NRGBA{R: 127, G: 255, A: 255}},
		{105, 15, color.NRGBA{G: 255, A: 255}},
		{115, 25, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{100, 65, color.NRGBA{A: 255}},
		{100, 80, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
	} {
		pixel(t, img, tc.x, tc.y, tc.want)
	}
}

func TestEmbeddedTextAndNewPage(t *testing.T) {
	data, err := liberation.Font("LiberationSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	faces, err := font.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	face := faces[0]
	face.Path = "embedded:LiberationSans-Regular.ttf"
	d := New(240, 50)
	x := 10.0
	for _, r := range "Hello café Ω" {
		gid, ok := face.NominalGlyph(r)
		if !ok || !d.Glyph(face, gid, string(r), 18, x, 30, Style{Color: Color{A: 255}}) {
			t.Fatalf("cannot emit %q", r)
		}
		f := d.fonts[0].(*fontResource)
		x += float64(f.widths[gid]) * 18 / float64(f.unitsPerEm)
	}
	d.Transform(Matrix{1, 0, 0, 1, 200, 200})
	d.Clip("0 0 1 1 re", false)
	d.NewPage(40, 30)
	d.Draw("0 0 40 30 re", false, Style{Color: Color{G: 255, A: 255}})
	path := writeDocument(t, d)
	text := poppler(t, "pdftotext", "-layout", path, "-")
	if !strings.Contains(string(text), "Hello café Ω") {
		t.Fatalf("extracted text = %q", text)
	}
	img := pageImage(t, path, 1)
	ink := 0
	for y := 10; y < 30; y++ {
		for x := 10; x < 130; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r < 30000 {
				ink++
			}
		}
	}
	if ink < 100 {
		t.Fatalf("embedded text painted only %d dark pixels", ink)
	}
	img = pageImage(t, path, 2)
	if img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 {
		t.Fatalf("second page bounds = %v", img.Bounds())
	}
	pixel(t, img, 20, 15, color.NRGBA{G: 255, A: 255})
}

func TestClipUnionFillRulesAndIntersection(t *testing.T) {
	d := New(160, 100)
	d.Save()
	d.Transform(Matrix{1, 0, 0, 1, 10, 10})
	d.ClipUnion([]ClipShape{
		{Path: "0 0 60 30 re"},
		{Path: "20 0 m 20 30 l 80 30 l 80 0 l h"},
	})
	d.BeginGroup(.5, false)
	d.Draw("0 0 60 30 re", false, Style{Color: Color{R: 255, A: 255}})
	d.Draw("20 0 60 30 re", false, Style{Color: Color{R: 255, A: 255}})
	d.EndGroup()
	d.Save()
	d.ClipUnion([]ClipShape{{Path: "30 0 10 100 re"}})
	d.Draw("0 0 160 100 re", false, Style{Color: Color{G: 255, A: 255}})
	d.Restore()
	d.Restore()
	d.Save()
	d.Transform(Matrix{1, 0, 0, 1, 10, 10})
	d.ClipUnion([]ClipShape{
		{Path: "0 40 60 30 re 10 45 40 20 re", EvenOdd: true},
		{Path: "20 45 10 10 re"},
	})
	d.Draw("0 0 160 100 re", false, Style{Color: Color{B: 255, A: 255}})
	d.Restore()
	d.Save()
	d.ClipUnion(nil)
	d.Draw("0 0 160 100 re", false, Style{Color: Color{A: 255}})
	d.Restore()
	d.Draw("140 10 10 10 re", false, Style{Color: Color{A: 255}})
	img := pageImage(t, writeDocument(t, d), 1)
	for _, tc := range []struct {
		x, y int
		want color.NRGBA
	}{
		{15, 20, color.NRGBA{R: 255, G: 127, B: 127, A: 255}},
		{35, 20, color.NRGBA{R: 255, G: 127, B: 127, A: 255}},
		{45, 20, color.NRGBA{G: 255, A: 255}},
		{85, 20, color.NRGBA{R: 255, G: 127, B: 127, A: 255}},
		{95, 20, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{15, 60, color.NRGBA{B: 255, A: 255}},
		{25, 60, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{35, 60, color.NRGBA{B: 255, A: 255}},
		{55, 60, color.NRGBA{R: 255, G: 255, B: 255, A: 255}},
		{145, 15, color.NRGBA{A: 255}},
	} {
		pixel(t, img, tc.x, tc.y, tc.want)
	}
}

func TestErrorsAndSerialization(t *testing.T) {
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if _, err := New(value, 10).Bytes(); err == nil {
			t.Errorf("accepted page width %v", value)
		}
	}
	for _, change := range []func(*Document){
		func(d *Document) { d.Draw("0 0 m Q", false, Style{}) },
		func(d *Document) { d.Clip("NaN 0 m", false) },
		func(d *Document) { d.Transform(Matrix{math.Inf(1)}) },
		func(d *Document) { d.Restore() },
		func(d *Document) { d.EndGroup() },
		func(d *Document) { d.BeginGroup(1, false) },
		func(d *Document) { d.BeginGroup(1, false); d.NewPage(10, 10) },
	} {
		d := New(10, 10)
		change(d)
		if _, err := d.Bytes(); err == nil {
			t.Error("accepted malformed drawing")
		}
	}
	d := New(20, 20)
	d.Draw("1e0 1e0 10 10 re", false, Style{Color: Color{A: 255}})
	data, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	again, err := d.Bytes()
	if err != nil || !bytes.Equal(data, again) {
		t.Fatal("serialization is not repeatable")
	}
	start := bytes.LastIndex(data, []byte("\nstartxref\n"))
	if start < 0 {
		t.Fatal("missing xref pointer")
	}
	var offset int
	if _, err := fmt.Sscanf(string(data[start:]), "\nstartxref\n%d", &offset); err != nil || offset >= len(data) || !bytes.HasPrefix(data[offset:], []byte("xref\n")) {
		t.Fatalf("invalid xref offset %d: %v", offset, err)
	}
	lines := strings.Split(string(data[offset:]), "\n")
	var first, count int
	if _, err := fmt.Sscanf(lines[1], "%d %d", &first, &count); err != nil || first != 0 {
		t.Fatal("invalid xref range")
	}
	for id := 1; id < count; id++ {
		if _, err := fmt.Sscanf(lines[id+2], "%d", &offset); err != nil || offset < 0 || offset >= len(data) || !bytes.HasPrefix(data[offset:], []byte(fmt.Sprintf("%d 0 obj\n", id))) {
			t.Fatalf("invalid object offset for %d", id)
		}
	}
}
