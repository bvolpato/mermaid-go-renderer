package rough

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The file testdata/chromium.json comes from
// tools/reference/gen-rough-chromium.mjs. It has rough shapes from the SVGs
// that mermaid 12.2.0 made in headless Chromium, with the call that mermaid
// made for each shape. The port must make the same path elements, byte for
// byte.
//
// The transcendental functions of that browser are correctly rounded, so the
// test gives the port the correctly rounded functions of package js. It
// also reports the result with the wrappers of package js, which the port
// calls by default.

// chromiumMathFuncs are the transcendental functions of the reference browser.
func chromiumMathFuncs() mathFuncs {
	return mathFuncs{sin: js.SinCR, cos: js.CosCR, tan: js.TanCR, atan: js.AtanCR, asin: js.AsinCR, pow: js.Pow}
}

type chromiumShape struct {
	Kind     string            `json:"kind"`
	Source   string            `json:"source"`
	F        string            `json:"f"`
	A        []json.RawMessage `json:"a"`
	O        json.RawMessage   `json:"o"`
	Position uint32            `json:"position"`
	Draws    int               `json:"draws"`
	Paths    [][][2]string     `json:"paths"`
}

func TestChromiumReference(t *testing.T) {
	data, err := os.ReadFile("testdata/chromium.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Meta struct {
			RandomSeed uint32 `json:"randomSeed"`
		} `json:"meta"`
		Shapes []chromiumShape `json:"shapes"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Shapes) == 0 {
		t.Fatal("the file has no shapes")
	}
	// run makes the call of the shape and returns its path elements. The d
	// attribute is in the form of the golden file.
	run := func(s chromiumShape, m *mathFuncs) (got [][][2]string, bytes, draws int, err error) {
		// The options are a JSON object here. The key order has no effect.
		parsed, err := js.ParseJSON(s.O)
		if err != nil {
			return nil, 0, 0, err
		}
		options := js.AsObj(parsed)
		rnd := &oracleRandom{}
		rnd.seek(file.Meta.RandomSeed, s.Position)
		g := NewGenerator(Config{Random: rnd.next})
		if m != nil {
			g.m = *m
		}
		var d *Drawable
		switch s.F {
		case "rectangle":
			var a [4]float64
			for i := range a {
				if err := json.Unmarshal(s.A[i], &a[i]); err != nil {
					return nil, 0, 0, err
				}
			}
			d, err = g.Rectangle(a[0], a[1], a[2], a[3], options)
		case "path":
			var pathData string
			if err := json.Unmarshal(s.A[0], &pathData); err != nil {
				return nil, 0, 0, err
			}
			d, err = g.Path(pathData, options)
		default:
			err = fmt.Errorf("unknown shape function %q", s.F)
		}
		if err != nil {
			return nil, 0, 0, err
		}
		for _, p := range g.Draw(d) {
			var attrs [][2]string
			for _, a := range p.Attrs {
				v := a.Value
				if a.Name == "d" {
					bytes += len(v)
					head := v
					if len(head) > 120 {
						head = head[:120]
					}
					v = fmt.Sprintf("%d:%s:%s", len(v), hashString(v), head)
				}
				attrs = append(attrs, [2]string{a.Name, v})
			}
			got = append(got, attrs)
		}
		return got, bytes, rnd.calls, nil
	}
	wrappersIdentical := 0
	for _, s := range file.Shapes {
		name := s.Kind + " " + strings.TrimPrefix(s.Source, "mermaid-e2e/")
		m := chromiumMathFuncs()
		got, bytes, draws, err := run(s, &m)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if fmt.Sprint(got) != fmt.Sprint(s.Paths) {
			t.Errorf("%s: the path elements differ from Chromium:\n got  %v\n want %v", name, got, s.Paths)
			continue
		}
		if draws != s.Draws {
			t.Errorf("%s: %d Math.random draws, want %d", name, draws, s.Draws)
		}
		t.Logf("%s: %d path elements with %d bytes of path data are identical to Chromium", name, len(got), bytes)
		// The same call with the wrappers of package js.
		if got, _, _, err := run(s, nil); err == nil && fmt.Sprint(got) == fmt.Sprint(s.Paths) {
			wrappersIdentical++
		}
	}
	t.Logf("with the wrappers js.Sin, js.Cos, js.Tan, js.Atan and js.Asin, %d of %d shapes are identical to Chromium", wrappersIdentical, len(file.Shapes))
}
