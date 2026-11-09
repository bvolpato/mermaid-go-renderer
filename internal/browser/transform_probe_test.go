package browser

import (
	"encoding/json"
	"math"
	"os"
	"regexp"
	"testing"
)

var reAngle = regexp.MustCompile(`(rotate|skewX|skewY)\(\s*([^,\s)]+)`)

// usesLibm reports whether the matrix of a transform list depends on the
// sin, cos or tan function of the C library: a skew, or a rotation by an
// angle that is not a multiple of 45 degrees. Chromium takes these
// functions from the C library of the platform (macOS for the reference),
// whose results are not always correctly rounded. This package rounds
// correctly, so the last bit can differ.
func usesLibm(transform string) bool {
	for _, m := range reAngle.FindAllStringSubmatch(transform, -1) {
		angle, _, ok := parseSVGNumber(m[2])
		if !ok {
			return true
		}
		if m[1] != "rotate" {
			if angle != 0 {
				return true
			}
			continue
		}
		if n45 := float64(angle) / 45; n45 != math.Trunc(n45) {
			return true
		}
	}
	return false
}

// TestTransformAgainstChromium replays the probe cases of
// tools/reference/gen-browser-transform-probe.mjs: values of the SVG
// transform attribute. The expected values are the matrix that Chromium
// uses for layout (getCTM) and the bounding box of a rect that the matrix
// maps. Every number must be the same float64, except for a list that
// depends on the C library (see usesLibm): its matrix must be within
// 1e-12 of the expected one, relative to the larger value.
func TestTransformAgainstChromium(t *testing.T) {
	var cases struct {
		Cases []struct {
			Name      string     `json:"name"`
			Transform string     `json:"transform"`
			Rect      [4]float64 `json:"rect"`
		} `json:"cases"`
	}
	var results []struct {
		Name   string     `json:"name"`
		Matrix [6]float64 `json:"matrix"`
		BBox   [4]float64 `json:"bbox"`
	}
	readJSON := func(name string, v any) {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	readJSON("transform-cases.json", &cases)
	readJSON("transform-results.json", &results)
	if len(cases.Cases) == 0 || len(cases.Cases) != len(results) {
		t.Fatalf("%d cases, %d results", len(cases.Cases), len(results))
	}
	same := func(a, b float64) bool {
		// Zero has no sign in the matrix that getCTM returns.
		return a == b || math.Float64bits(a) == math.Float64bits(b)
	}
	closeTo := func(a, b float64) bool {
		return math.Abs(a-b) <= 1e-12*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	}
	matrices, boxes, libm, libmExact := 0, 0, 0, 0
	for i, c := range cases.Cases {
		want := results[i]
		m := parseTransformList(c.Transform)
		got := [6]float64{m.a, m.b, m.c, m.d, m.e, m.f}
		ok := true
		for k := range got {
			ok = ok && same(got[k], want.Matrix[k])
		}
		if usesLibm(c.Transform) {
			libm++
			if ok {
				libmExact++
				continue
			}
			for k := range got {
				if !closeTo(got[k], want.Matrix[k]) {
					t.Errorf("%s matrix of %q\n got  %v\n want %v", c.Name, c.Transform, got, want.Matrix)
					break
				}
			}
			continue
		}
		if !ok {
			t.Errorf("%s matrix of %q\n got  %v\n want %v", c.Name, c.Transform, got, want.Matrix)
			continue
		}
		matrices++

		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		svg := container.Append("svg").Attr("id", "my-svg").Attr("width", "100%")
		group := svg.Append("g").Attr("id", "p")
		group.Append("g").Attr("transform", c.Transform).Append("rect").
			Attr("x", c.Rect[0]).Attr("y", c.Rect[1]).Attr("width", c.Rect[2]).Attr("height", c.Rect[3])
		b := group.BBox()
		box := [4]float64{b.X, b.Y, b.Width, b.Height}
		if box != want.BBox {
			t.Errorf("%s bbox with %q of rect %v\n got  %v\n want %v", c.Name, c.Transform, c.Rect, box, want.BBox)
			continue
		}
		boxes++
	}
	t.Logf("exact matrices=%d and boxes=%d of %d lists; %d more lists depend on the C library, %d of them exact",
		matrices, boxes, len(cases.Cases)-libm, libm, libmExact)
}
