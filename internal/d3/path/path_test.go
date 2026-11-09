package path

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/d3/internal/d3test"
)

type golden struct {
	Math   d3test.Math
	Digits []struct {
		D  string
		OK bool
	}
	Groups struct {
		Paths []struct {
			Ops    [][2]string
			Digits float64
			S, R   string
			E      []int
		}
	}
}

// mathExceptions lists, for each build mode (see d3test.Mode), the number of
// golden cases that differ from Node.js only because a Math function returns
// another last bit. The test proves each one: the case passes when it runs
// again with the results that Node.js returned for the same calls. Measured
// with go1.27.3.
//
// d3-path calls Math.cos and Math.sin in arc, and Math.tan and Math.acos in
// arcTo. The port has the correctly rounded values, as Chromium has for
// Math.sin and Math.cos. Node.js is off by 1 ulp in 10 calls of Math.sin, 8
// of Math.cos, 7 of Math.acos and 6 of Math.tan. With the 3 digits of
// d3-shape, no path differs.
var mathExceptions = map[string]map[string]int{
	"arm64":       {"path": 30, "pathRound, 15 digits or more": 13},
	"arm64/nofma": {"path": 30, "pathRound, 15 digits or more": 13},
	"amd64":       {"path": 30, "pathRound, 15 digits or more": 13},
}

type op struct {
	name string
	args []float64
}

// apply runs the operations and returns the path data and the indexes of the
// operations that failed.
func apply(p *Path, ops []op) (string, []int) {
	errs := []int{}
	for i, o := range ops {
		a := o.args
		var err error
		switch o.name {
		case "moveTo":
			p.MoveTo(a[0], a[1])
		case "lineTo":
			p.LineTo(a[0], a[1])
		case "quadraticCurveTo":
			p.QuadraticCurveTo(a[0], a[1], a[2], a[3])
		case "bezierCurveTo":
			p.BezierCurveTo(a[0], a[1], a[2], a[3], a[4], a[5])
		case "arcTo":
			err = p.ArcTo(a[0], a[1], a[2], a[3], a[4])
		case "arc":
			err = p.Arc(a[0], a[1], a[2], a[3], a[4], a[5] != 0)
		case "rect":
			p.Rect(a[0], a[1], a[2], a[3])
		case "closePath":
			p.ClosePath()
		}
		if err != nil {
			errs = append(errs, i)
		}
	}
	return p.String(), errs
}

// TestAgainstD3 compares with values that tools/reference/gen-d3-path.mjs
// recorded from d3-path.
func TestAgainstD3(t *testing.T) {
	var g golden
	d3test.Load(t, "testdata/path.json.gz", &g)
	c := d3test.NewChecker(t, g.Math)

	for i, tc := range g.Groups.Paths {
		ops := make([]op, len(tc.Ops))
		for j, o := range tc.Ops {
			var args d3test.FS
			if err := args.UnmarshalJSON([]byte(strconv.Quote(o[1]))); err != nil {
				t.Fatal(err)
			}
			ops[j] = op{o[0], args}
		}
		c.Check("path", fmt.Sprintf("#%d %v", i, ops), func() string {
			got, errs := apply(New(), ops)
			if got != tc.S {
				return fmt.Sprintf("got %q, want %q", got, tc.S)
			}
			if fmt.Sprint(errs) != fmt.Sprint(tc.E) {
				return fmt.Sprintf("errors at %v, want %v", errs, tc.E)
			}
			return ""
		})
		// More than 15 digits mean no rounding.
		group := "pathRound, 0 to 5 digits"
		if tc.Digits >= 15 {
			group = "pathRound, 15 digits or more"
		}
		c.Check(group, fmt.Sprintf("#%d digits=%v %v", i, tc.Digits, ops), func() string {
			p, err := NewRound(tc.Digits)
			if err != nil {
				return err.Error()
			}
			if got, _ := apply(p, ops); got != tc.R {
				return fmt.Sprintf("got %q, want %q", got, tc.R)
			}
			return ""
		})
	}
	for _, tc := range g.Digits {
		d := math.NaN()
		switch tc.D {
		case "Infinity":
			d = math.Inf(1)
		case "NaN":
		default:
			d, _ = strconv.ParseFloat(tc.D, 64)
		}
		if _, err := NewRound(d); (err == nil) != tc.OK {
			t.Errorf("NewRound(%v): error %v, want ok=%v", d, err, tc.OK)
		}
	}

	c.Report()
	c.CheckExceptions(mathExceptions)
}
