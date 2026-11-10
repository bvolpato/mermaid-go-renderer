// Original tests of ISO 32000-1 gradient shading serialization.
// This file is not an upstream port.

package pdf

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGradientRejectsWithoutMutation(t *testing.T) {
	type input struct {
		coords  [6]float64
		radial  bool
		stops   []GradientStop
		opacity float64
	}
	for _, tc := range []struct {
		name   string
		change func(*input)
	}{
		{"no stops", func(in *input) { in.stops = nil }},
		{"negative offset", func(in *input) { in.stops[0].Offset = -0.1 }},
		{"large offset", func(in *input) { in.stops[1].Offset = 1.1 }},
		{"NaN offset", func(in *input) { in.stops[0].Offset = math.NaN() }},
		{"infinite offset", func(in *input) { in.stops[1].Offset = math.Inf(1) }},
		{"decreasing offsets", func(in *input) { in.stops[0].Offset, in.stops[1].Offset = 1, 0 }},
		{"duplicate offsets", func(in *input) { in.stops[0].Offset, in.stops[1].Offset = 0.5, 0.5 }},
		{"duplicate start", func(in *input) { in.stops[1].Offset = 0 }},
		{"duplicate end", func(in *input) { in.stops[0].Offset = 1 }},
		{"negative opacity", func(in *input) { in.opacity = -0.1 }},
		{"large opacity", func(in *input) { in.opacity = 1.1 }},
		{"NaN opacity", func(in *input) { in.opacity = math.NaN() }},
		{"infinite opacity", func(in *input) { in.opacity = math.Inf(1) }},
		{"NaN coordinate", func(in *input) { in.coords[0] = math.NaN() }},
		{"infinite coordinate", func(in *input) { in.coords[3] = math.Inf(-1) }},
		{"zero axis", func(in *input) { in.coords = [6]float64{} }},
		{"negative start radius", func(in *input) { in.radial, in.coords = true, [6]float64{0, 0, -1, 5, 5, 10} }},
		{"negative end radius", func(in *input) { in.radial, in.coords = true, [6]float64{0, 0, 1, 5, 5, -1} }},
		{"zero radii", func(in *input) { in.radial, in.coords = true, [6]float64{0, 0, 0, 5, 5, 0} }},
		{"identical circles", func(in *input) { in.radial, in.coords = true, [6]float64{5, 5, 10, 5, 5, 10} }},
		{"NaN end radius", func(in *input) { in.radial, in.coords[5] = true, math.NaN() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input{coords: [6]float64{0, 0, 120, 0}, stops: []GradientStop{{0, Color{255, 0, 0, 255}}, {1, Color{0, 0, 255, 255}}}, opacity: 1}
			d := New(120, 40)
			if !d.Gradient(in.coords, false, in.stops, 1) {
				t.Fatal("valid gradient rejected")
			}
			before, err := d.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&in)
			if d.Gradient(in.coords, in.radial, in.stops, in.opacity) {
				t.Fatal("invalid gradient accepted")
			}
			after, err := d.Bytes()
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("rejected gradient changed the document: %v", err)
			}
		})
	}
	var empty Document
	stops := []GradientStop{{0, Color{A: 255}}}
	if empty.Gradient([6]float64{0, 0, 10, 0}, false, stops, 1) || empty.err != nil {
		t.Fatal("gradient changed a document without a page")
	}
}

func TestGradientCopiesStops(t *testing.T) {
	stops := make([]GradientStop, 2, 4)
	stops[0], stops[1] = GradientStop{0.25, Color{255, 0, 0, 128}}, GradientStop{0.75, Color{0, 0, 255, 128}}
	want := append([]GradientStop(nil), stops...)
	d := New(120, 40)
	if !d.Gradient([6]float64{0, 0, 120, 0, math.NaN(), math.Inf(1)}, false, stops, 0.3) {
		t.Fatal("valid gradient rejected")
	}
	if !reflect.DeepEqual(stops, want) {
		t.Fatal("gradient changed the caller's stops")
	}
	if got := d.states[0].opacity; got != float64(128)/255*0.3 {
		t.Fatalf("combined opacity = %v", got)
	}
	before, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	stops[0], stops[1] = GradientStop{}, GradientStop{}
	after, err := d.Bytes()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("caller mutation changed the gradient: %v", err)
	}
}

func TestGradientPoppler(t *testing.T) {
	tool, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm is unavailable")
	}
	red, green, blue := Color{255, 0, 0, 255}, Color{0, 255, 0, 255}, Color{0, 0, 255, 255}
	const (
		plainScene = iota
		transformedScene
		maskedScene
	)
	type sample struct {
		x, y int
		rgb  [3]int
	}
	for _, tc := range []struct {
		name   string
		coords [6]float64
		radial bool
		stops  []GradientStop
		scene  int
		want   []sample
	}{
		{"axial pad", [6]float64{20, 0, 100, 0}, false, []GradientStop{{0.25, red}, {0.75, blue}}, plainScene,
			[]sample{{10, 20, [3]int{255, 0, 0}}, {30, 20, [3]int{255, 0, 0}}, {60, 20, [3]int{128, 0, 128}}, {90, 20, [3]int{0, 0, 255}}, {110, 20, [3]int{0, 0, 255}}}},
		{"stitching", [6]float64{0, 0, 120, 0}, false, []GradientStop{{0, red}, {0.25, green}, {0.75, blue}, {1, red}}, plainScene,
			[]sample{{15, 20, [3]int{128, 128, 0}}, {60, 20, [3]int{0, 128, 128}}, {105, 20, [3]int{128, 0, 128}}}},
		{"radial pad", [6]float64{60, 60, 10, 60, 60, 40}, true, []GradientStop{{0, red}, {1, blue}}, plainScene,
			[]sample{{60, 60, [3]int{255, 0, 0}}, {85, 60, [3]int{128, 0, 128}}, {110, 60, [3]int{0, 0, 255}}}},
		{"radial centers", [6]float64{30, 60, 0, 60, 60, 40}, true, []GradientStop{{0, red}, {1, blue}}, plainScene,
			[]sample{{45, 60, [3]int{200, 0, 55}}, {90, 60, [3]int{36, 0, 219}}}},
		{"single stop", [6]float64{0, 0, 120, 0}, false, []GradientStop{{0.4, green}}, plainScene,
			[]sample{{10, 20, [3]int{0, 255, 0}}, {110, 20, [3]int{0, 255, 0}}}},
		{"constant green varying alpha", [6]float64{10, 0, 60, 0}, false, []GradientStop{{0, green}, {1, Color{0, 255, 0, 0}}}, plainScene,
			[]sample{{5, 20, [3]int{0, 255, 0}}, {35, 20, [3]int{128, 255, 128}}, {65, 20, [3]int{255, 255, 255}}}},
		{"alpha pad", [6]float64{20, 0, 100, 0}, false, []GradientStop{{0.25, Color{255, 0, 0, 64}}, {0.75, Color{255, 0, 0, 192}}}, plainScene,
			[]sample{{10, 20, [3]int{255, 191, 191}}, {30, 20, [3]int{255, 191, 191}}, {60, 20, [3]int{255, 127, 127}}, {90, 20, [3]int{255, 63, 63}}, {110, 20, [3]int{255, 63, 63}}}},
		{"alpha stitching", [6]float64{0, 0, 120, 0}, false, []GradientStop{{0, Color{0, 0, 255, 0}}, {0.25, Color{0, 0, 255, 128}}, {0.75, blue}, {1, Color{0, 0, 255, 0}}}, plainScene,
			[]sample{{15, 20, [3]int{191, 191, 255}}, {60, 20, [3]int{64, 64, 255}}, {105, 20, [3]int{128, 128, 255}}}},
		{"radial alpha", [6]float64{60, 60, 10, 60, 60, 40}, true, []GradientStop{{0, green}, {1, Color{0, 255, 0, 0}}}, plainScene,
			[]sample{{60, 60, [3]int{0, 255, 0}}, {85, 60, [3]int{128, 255, 128}}, {110, 60, [3]int{255, 255, 255}}}},
		{"alpha transform clip and group", [6]float64{0, 0, 20, 0}, false, []GradientStop{{0, Color{255, 0, 0, 128}}, {1, Color{0, 0, 255, 128}}}, transformedScene,
			[]sample{{5, 20, [3]int{255, 255, 255}}, {15, 20, [3]int{255, 191, 191}}, {40, 20, [3]int{223, 191, 223}}, {65, 20, [3]int{191, 191, 255}}, {75, 20, [3]int{255, 255, 255}}, {85, 20, [3]int{0, 255, 0}}}},
		{"varying alpha transform clip and nested groups", [6]float64{0, 0, 20, 0}, false, []GradientStop{{0, Color{255, 0, 0, 64}}, {1, Color{0, 0, 255, 192}}}, transformedScene,
			[]sample{{5, 20, [3]int{255, 255, 255}}, {15, 20, [3]int{255, 223, 223}}, {40, 20, [3]int{223, 191, 223}}, {65, 20, [3]int{159, 159, 255}}, {75, 20, [3]int{255, 255, 255}}, {85, 20, [3]int{0, 255, 0}}}},
		{"constant alpha through prior mask", [6]float64{0, 0, 20, 0}, false, []GradientStop{{0, Color{255, 0, 0, 128}}, {1, Color{0, 0, 255, 128}}}, maskedScene,
			[]sample{{15, 20, [3]int{255, 223, 223}}, {25, 20, [3]int{251, 223, 227}}, {40, 20, [3]int{255, 255, 255}}, {55, 20, [3]int{227, 223, 251}}, {65, 20, [3]int{223, 223, 255}}, {85, 20, [3]int{0, 255, 0}}}},
		{"varying alpha through prior mask", [6]float64{0, 0, 20, 0}, false, []GradientStop{{0, Color{255, 0, 0, 64}}, {1, Color{0, 0, 255, 192}}}, maskedScene,
			[]sample{{15, 20, [3]int{255, 239, 239}}, {25, 20, [3]int{253, 235, 237}}, {40, 20, [3]int{255, 255, 255}}, {55, 20, [3]int{216, 211, 249}}, {65, 20, [3]int{207, 207, 255}}, {85, 20, [3]int{0, 255, 0}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := New(120, 120)
			opacity := 1.0
			if tc.scene != plainScene {
				d.BeginGroup(1, false)
				d.BeginGroup(1, false)
				d.Save()
				d.Transform(Matrix{2, 0, 0, 1, 10, 10})
				d.Clip("0 0 30 20 re", false)
				d.Transform(Matrix{1, 0, 0, 1, 5, 0})
				opacity = 0.5
			}
			if tc.scene == maskedScene {
				// A partial-alpha Form detects applying the prior mask more than once.
				parent := d.current
				d.current = newDrawingContext(drawingState{matrix: Matrix{1, 0, 0, 1, 0, 0}})
				d.Draw("10 10 20 20 re 50 10 20 20 re", false, Style{Color: Color{A: 128}})
				d.forms = append(d.forms, &form{width: 120, height: 120, content: d.current})
				d.current = parent
				d.current.state.mask = len(d.forms)
			}
			if !d.Gradient(tc.coords, tc.radial, tc.stops, opacity) {
				t.Fatal("valid gradient rejected")
			}
			if tc.scene != plainScene {
				d.Restore()
				d.EndGroup()
				d.EndGroup()
				d.Draw("80 10 20 20 re", false, Style{Color: green})
			}
			data, err := d.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			prefix := filepath.Join(t.TempDir(), "gradient")
			if err := os.WriteFile(prefix+".pdf", data, 0600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command(tool, "-r", "96", "-singlefile", "-png", prefix+".pdf", prefix).CombinedOutput(); err != nil || len(output) != 0 {
				t.Fatalf("pdftoppm: %v\n%s", err, output)
			}
			file, err := os.Open(prefix + ".png")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			img, err := png.Decode(file)
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds() != image.Rect(0, 0, 120, 120) {
				t.Fatalf("rendered bounds = %v", img.Bounds())
			}
			for _, sample := range tc.want {
				r, g, b, _ := img.At(sample.x, sample.y).RGBA()
				got := [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
				for channel, value := range got {
					if delta := value - sample.rgb[channel]; delta < -8 || delta > 8 {
						t.Errorf("pixel (%d, %d) = %v, want %v", sample.x, sample.y, got, sample.rgb)
						break
					}
				}
			}
		})
	}
}
