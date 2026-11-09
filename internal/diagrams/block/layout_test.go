package block

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// goldenNumber reads a number of the golden file. ok is false for a value
// that is not a number: upstream makes strings when the padding of the
// config is a string.
func goldenNumber(v any) (n float64, ok bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case *js.Obj[any]:
		switch x.Get("$n") {
		case "NaN":
			return math.NaN(), true
		case "Infinity":
			return math.Inf(1), true
		case "-Infinity":
			return math.Inf(-1), true
		case "-0":
			return math.Copysign(0, -1), true
		}
	}
	return 0, false
}

// goldenSize reads the size of one entry of measured or drawn.sizes. size
// is nil when the block has no size. ok is false when a value is not a
// number.
func goldenSize(entry any) (size *Size, ok bool) {
	o := js.AsObj(js.AsObj(entry).Get("size"))
	if o == nil {
		return nil, true
	}
	var s Size
	for _, f := range []struct {
		name string
		dst  *float64
	}{{"width", &s.Width}, {"height", &s.Height}, {"x", &s.X}, {"y", &s.Y}} {
		n, ok := goldenNumber(o.Get(f.name))
		if !ok {
			return nil, false
		}
		*f.dst = n
	}
	return &s, true
}

func sameBits(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (a != a && b != b)
}

func showSize(s *Size) string {
	if s == nil {
		return "none"
	}
	return fmt.Sprintf("{width: %s, height: %s, x: %s, y: %s}", js.Str(s.Width), js.Str(s.Height), js.Str(s.X), js.Str(s.Y))
}

// diffSizes compares the sizes of the blocks of the flat list with the
// sizes of a dump, bit for bit.
func diffSizes(what string, flat []*Block, want []any) string {
	if len(flat) != len(want) {
		return fmt.Sprintf("%s: %d blocks, want %d", what, len(flat), len(want))
	}
	for i, b := range flat {
		w, ok := goldenSize(want[i])
		if !ok {
			return fmt.Sprintf("%s[%d]: the size of upstream is not a number", what, i)
		}
		g := b.Size
		same := (g == nil) == (w == nil)
		if same && g != nil {
			same = sameBits(g.Width, w.Width) && sameBits(g.Height, w.Height) && sameBits(g.X, w.X) && sameBits(g.Y, w.Y)
		}
		if !same {
			return fmt.Sprintf("%s[%d] (block %s): got %s, want %s", what, i, strconv.Quote(b.idString()), showSize(g), showSize(w))
		}
	}
	return ""
}

// paddingIsNumber reports whether the padding of the block config of a
// case is a number, or absent.
func paddingIsNumber(c *js.Obj[any]) bool {
	padding := js.Path(c.Get("parsed"), "blockConfig", "padding")
	_, isNumber := padding.(float64)
	return isNumber || js.IsNullish(padding)
}

// TestLayoutGolden runs layout() on the block sizes that the browser
// measured and compares the size and the position of every block, and the
// bounds, with the result of upstream, bit for bit. The test does not
// depend on the measurements of the Go engine.
func TestLayoutGolden(t *testing.T) {
	g := loadGolden(t)
	cases, _ := g.Get("cases").([]any)
	var total, corpus, failed, corpusFailed, withError, stringPadding int
	for _, item := range cases {
		c := js.AsObj(item)
		measured, hasMeasured := c.Get("measured").([]any)
		drawn := js.AsObj(c.Get("drawn"))
		if !hasMeasured || drawn == nil {
			continue
		}
		name, _ := c.Get("name").(string)
		_, hasText := c.Get("text").(string)
		if !paddingIsNumber(c) {
			// Upstream adds the padding as a string, and the sizes become
			// strings. The port converts the padding to a number.
			stringPadding++
			continue
		}
		total++
		if !hasText {
			corpus++
		}
		fail := func(format string, args ...any) {
			failed++
			if !hasText {
				corpusFailed++
			}
			if failed <= 25 {
				text := ""
				if hasText {
					text = caseText(t, c)
				}
				t.Errorf("%s: %s\n%s", name, fmt.Sprintf(format, args...), indent(text))
			}
		}
		fresh, _ := c.Get("fresh").(bool)
		idCounter, _ := c.Get("idCounter").(float64)
		p, err := parseText(caseText(t, c), siteConfig(c), !fresh, int(idCounter))
		if err != nil || p.db == nil {
			fail("no db: %v", err)
			continue
		}
		flat := p.db.GetBlocksFlat()
		if len(flat) != len(measured) {
			fail("%d blocks, want %d", len(flat), len(measured))
			continue
		}
		// The state at the start of layout(): the measured sizes.
		bad := false
		for i, b := range flat {
			size, ok := goldenSize(measured[i])
			if !ok {
				bad = true
				break
			}
			b.Size = size
		}
		if bad {
			fail("a measured size is not a number")
			continue
		}

		bounds, ok, err := layout(p.db)
		wantError, isError := c.Get("error").(string)
		if isError {
			withError++
		}
		if err != nil {
			if got := errorMessage(err); !isError || got != wantError {
				fail("layout error %q, want %q", got, wantError)
				continue
			}
		}
		wantSizes, _ := drawn.Get("sizes").([]any)
		if d := diffSizes("sizes", flat, wantSizes); d != "" {
			fail("%s", d)
			continue
		}
		if err != nil {
			// layout() threw: it has no result. The sizes are the state at
			// the throw.
			continue
		}
		wantBounds := js.AsObj(drawn.Get("bounds"))
		if !ok || wantBounds == nil {
			fail("bounds: got %v, want %v", ok, wantBounds != nil)
			continue
		}
		for _, f := range []struct {
			name string
			got  float64
		}{{"x", bounds.X}, {"y", bounds.Y}, {"width", bounds.Width}, {"height", bounds.Height}} {
			want, isNumber := goldenNumber(wantBounds.Get(f.name))
			if !isNumber || !sameBits(f.got, want) {
				fail("bounds.%s: got %s, want %s", f.name, js.Str(f.got), show(wantBounds.Get(f.name)))
				break
			}
		}
	}
	if total == 0 {
		t.Fatal("no case with a layout")
	}
	t.Logf("%d cases with a layout (%d corpus fixtures, %d where upstream fails in or after the layout): %d agree bit for bit, %d differ (%d corpus fixtures differ); %d more cases have a padding that is not a number and are not compared",
		total, corpus, withError, total-failed, failed, corpusFailed, stringPadding)
}
