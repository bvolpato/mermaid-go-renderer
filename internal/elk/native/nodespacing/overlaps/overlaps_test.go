package overlaps

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
)

// exact is a double that the golden file keeps exact: a JSON number, or one
// of the texts "-0", "NaN", "Infinity" and "-Infinity".
type exact float64

func (e *exact) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		switch s {
		case "-0":
			*e = exact(math.Copysign(0, -1))
		case "NaN":
			*e = exact(math.NaN())
		case "Infinity":
			*e = exact(math.Inf(1))
		case "-Infinity":
			*e = exact(math.Inf(-1))
		default:
			return fmt.Errorf("unknown number %q", s)
		}
		return nil
	}
	f, err := strconv.ParseFloat(string(data), 64)
	*e = exact(f)
	return err
}

// same compares two doubles bit for bit. Each NaN is equal to each other
// NaN: JavaScript has one NaN, and the golden file has the text "NaN".
func same(got float64, want exact) bool {
	if got != got {
		return float64(want) != float64(want)
	}
	return math.Float64bits(got) == math.Float64bits(float64(want))
}

// TestRemoveOverlaps compares the overlap removal with elkjs 0.9.3 for
// random rectangles. tools/reference/gen-elk-nodespacing.mjs writes the
// golden file testdata/overlaps.json.gz.
func TestRemoveOverlaps(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "overlaps.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Direction string     `json:"direction"`
			Gap       float64    `json:"gap"`
			Start     exact      `json:"start"`
			Rects     [][4]exact `json:"rects"`
			Out       [][4]exact `json:"out"`
			Strip     exact      `json:"strip"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("the golden file has no case")
	}
	directions := map[string]OverlapRemovalDirection{
		"UP":    OverlapRemovalDirectionUp,
		"DOWN":  OverlapRemovalDirectionDown,
		"LEFT":  OverlapRemovalDirectionLeft,
		"RIGHT": OverlapRemovalDirectionRight,
	}
	failed := 0
	for i, c := range file.Cases {
		direction, ok := directions[c.Direction]
		if !ok {
			t.Fatalf("case %d: unknown direction %q", i, c.Direction)
		}
		remover := CreateForDirection(direction).WithGap(0, c.Gap)
		rects := make([]*emath.ElkRectangle, len(c.Rects))
		for k, r := range c.Rects {
			rects[k] = emath.NewElkRectangle(float64(r[0]), float64(r[1]), float64(r[2]), float64(r[3]))
			remover.AddRectangle(rects[k])
		}
		strip := remover.WithStartCoordinate(float64(c.Start)).RemoveOverlaps()
		bad := !same(strip, c.Strip)
		for k, r := range rects {
			want := c.Out[k]
			if !same(r.X, want[0]) || !same(r.Y, want[1]) || !same(r.Width, want[2]) || !same(r.Height, want[3]) {
				bad = true
			}
		}
		if bad {
			failed++
			if failed <= 5 {
				t.Errorf("case %d (%s, gap %v, start %v): the strip size is %v and the rectangles are %v, elkjs has %v and %v", i, c.Direction, c.Gap, float64(c.Start), strip, rects, float64(c.Strip), c.Out)
			}
		}
	}
	t.Logf("%d cases, %d differ from elkjs", len(file.Cases), failed)
	if failed > 0 {
		t.Errorf("%d of %d cases differ from elkjs", failed, len(file.Cases))
	}
}
