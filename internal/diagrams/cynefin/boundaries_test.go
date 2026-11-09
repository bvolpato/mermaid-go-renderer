package cynefin

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// boundariesGolden is testdata/boundaries.json.gz, which
// tools/reference/gen-cynefin-boundaries.mjs wrote with the real functions
// of cynefinBoundaries.ts for random inputs. A number is the hex text of
// its bits.
type boundariesGolden struct {
	SeededRandom []struct {
		Seed   string `json:"seed"`
		Result string `json:"result"`
	} `json:"seededRandom"`
	HashString []struct {
		Units  []rune  `json:"units"`
		Result float64 `json:"result"`
	} `json:"hashString"`
	ResolveSeed []struct {
		Configured json.RawMessage `json:"configured"`
		ID         string          `json:"id"`
		Result     string          `json:"result"`
	} `json:"resolveSeed"`
	Fold       []boundaryCase `json:"fold"`
	Horizontal []boundaryCase `json:"horizontal"`
	Cliff      []struct {
		Width  string `json:"width"`
		Height string `json:"height"`
		Result string `json:"result"`
	} `json:"cliff"`
	Confusion []struct {
		CX     string `json:"cx"`
		CY     string `json:"cy"`
		RX     string `json:"rx"`
		RY     string `json:"ry"`
		Result string `json:"result"`
	} `json:"confusion"`
}

type boundaryCase struct {
	Width     string          `json:"width"`
	Height    string          `json:"height"`
	Seed      string          `json:"seed"`
	Amplitude json.RawMessage `json:"amplitude"`
	Result    string          `json:"result"`
}

func fromBits(t *testing.T, text string) float64 {
	t.Helper()
	v, err := strconv.ParseUint(text, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float64frombits(v)
}

// sameBits compares two numbers by their bits. Every NaN is the same.
func sameBits(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
}

// goldenValue reads a dynamic value of the golden file: {"$u":1} is
// undefined and {"$b":"<bits>"} is a number.
func goldenValue(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if o := js.AsObj(v); o != nil && o.Len() == 1 {
		if o.Has("$u") {
			return nil
		}
		if b, ok := o.Get("$b").(string); ok {
			return fromBits(t, b)
		}
	}
	return v
}

// TestBoundaries compares the functions of cynefinBoundaries.ts with
// mermaid for random inputs: the numbers by their bits and the paths as
// text.
func TestBoundaries(t *testing.T) {
	var g boundariesGolden
	readGolden(t, "boundaries.json.gz", &g)
	if len(g.SeededRandom) < 1500 || len(g.HashString) < 400 || len(g.ResolveSeed) < 300 || len(g.Fold) < 400 ||
		len(g.Horizontal) < 400 || len(g.Cliff) < 300 || len(g.Confusion) < 300 {
		t.Fatal("the golden file has too few cases")
	}
	for _, c := range g.SeededRandom {
		seed := fromBits(t, c.Seed)
		if got, want := seededRandom(seed), fromBits(t, c.Result); !sameBits(got, want) {
			t.Errorf("seededRandom(%v): got %v, want %v", seed, got, want)
		}
	}
	for _, c := range g.HashString {
		text := js.FromUTF16Units(c.Units)
		if got := hashString(text); got != c.Result {
			t.Errorf("hashString(%q): got %v, want %v", text, got, c.Result)
		}
	}
	for _, c := range g.ResolveSeed {
		configured := goldenValue(t, c.Configured)
		if got, want := resolveSeed(configured, c.ID), fromBits(t, c.Result); !sameBits(got, want) {
			t.Errorf("resolveSeed(%s, %q): got %v, want %v", c.Configured, c.ID, got, want)
		}
	}
	paths := func(name string, cases []boundaryCase, fn func(width, height, seed float64, amplitude any) string) {
		for _, c := range cases {
			width, height, seed := fromBits(t, c.Width), fromBits(t, c.Height), fromBits(t, c.Seed)
			if got := fn(width, height, seed, goldenValue(t, c.Amplitude)); got != c.Result {
				t.Errorf("%s(%v, %v, %v, %s):\n got %s\nwant %s", name, width, height, seed, c.Amplitude, got, c.Result)
			}
		}
	}
	paths("generateFoldPath", g.Fold, generateFoldPath)
	paths("generateHorizontalBoundary", g.Horizontal, generateHorizontalBoundary)
	for _, c := range g.Cliff {
		width, height := fromBits(t, c.Width), fromBits(t, c.Height)
		if got := generateCliffPath(width, height, js.Str(height)); got != c.Result {
			t.Errorf("generateCliffPath(%v, %v):\n got %s\nwant %s", width, height, got, c.Result)
		}
	}
	for _, c := range g.Confusion {
		cx, cy, rx, ry := fromBits(t, c.CX), fromBits(t, c.CY), fromBits(t, c.RX), fromBits(t, c.RY)
		if got := generateConfusionPath(cx, cy, rx, ry); got != c.Result {
			t.Errorf("generateConfusionPath(%v, %v, %v, %v):\n got %s\nwant %s", cx, cy, rx, ry, got, c.Result)
		}
	}
	t.Logf("%d cases", len(g.SeededRandom)+len(g.HashString)+len(g.ResolveSeed)+len(g.Fold)+len(g.Horizontal)+len(g.Cliff)+len(g.Confusion))
}
