package block

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"

	// The shapes register themselves in the unified renderer.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)

// renderCase renders a text as the page of the oracle does. A panic of the
// renderer is returned as an error with the prefix "panic: ".
func renderCase(text string, site *js.Obj[any]) (svg string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	res, err := render.Render(text, render.Options{Config: site})
	if err != nil {
		return "", err
	}
	return res.SVG, nil
}

// foreignCause names the part of the port, outside this package, that
// makes the outcome of a case differ from upstream. It returns "" for a
// case that must agree.
func foreignCause(c *js.Obj[any]) string {
	if msg, _ := c.Get("error").(string); strings.HasPrefix(msg, "Failed to execute 'insertRule' on 'CSSStyleSheet'") {
		// A classDef whose name is not a CSS identifier: the browser
		// rejects the rule when mermaid makes the style sheet. The style
		// sheet code of internal/render skips the rule.
		return "internal/render: no error for a class name that is not a CSS identifier"
	}
	return ""
}

// TestRenderGolden renders every case of the golden file and compares the
// outcome with upstream:
//
//   - a case that upstream rejects or fails to draw must fail with the same
//     message, and a case that upstream draws must not fail,
//   - the block sizes that the port measures, and the block sizes and the
//     intersect functions after the draw,
//   - the SVG, for the cases of testdata/render.json.gz (the SVG of a
//     corpus fixture is in the reference directory: the parity command
//     compares it).
//
// The outcome must agree. The sizes and the SVG depend on the shapes, the
// edges and the measurement engine, which other packages own: the test
// reports their differences and fails for them only when the environment
// variable BLOCK_RENDER_STRICT is set.
func TestRenderGolden(t *testing.T) {
	strict := os.Getenv("BLOCK_RENDER_STRICT") != ""
	g := loadGolden(t)
	cases, _ := g.Get("cases").([]any)
	svgs := map[string]string{}
	for _, item := range js.AsObj(loadGoldenFile(t, "render.json.gz")).Get("cases").([]any) {
		c := js.AsObj(item)
		name, _ := c.Get("name").(string)
		svgs[name], _ = c.Get("svg").(string)
	}

	// Register the diagram with a db that reports the measured sizes.
	def, ok := render.Lookup("block")
	if !ok {
		t.Fatal("the block diagram is not registered")
	}
	var captured *DB
	var measured []*Size
	hasMeasured := false
	// idCounter and warm are the state of the page of the case: the
	// counter of generateId of blockDB.ts before the render, and whether
	// the DOMPurify hooks of mermaid are installed.
	idCounter, warm := 0, false
	wrapped := *def
	wrapped.New = func(ctx *render.Context) render.DB {
		for i := 0; i < idCounter; i++ {
			ctx.Counter("block.generateId")
		}
		if warm {
			ctx.Sanitizer.RemoveScript("")
		}
		db := NewDB(ctx)
		captured, measured, hasMeasured = db, nil, false
		db.beforeLayout = func() {
			measured = measured[:0]
			for _, b := range db.GetBlocksFlat() {
				var size *Size
				if b.Size != nil {
					s := *b.Size
					size = &s
				}
				measured = append(measured, size)
			}
			hasMeasured = true
		}
		return db
	}
	render.Register(&wrapped)
	defer render.Register(def)

	var total, outcomeFailed, foreign, otherType int
	var measureTotal, measureDiff, drawnTotal, drawnDiff int
	var svgTotal, svgExact, svgEquivalent, svgDiff, stringPadding int
	problems := 0
	report := func(fatal bool, name, text, format string, args ...any) {
		problems++
		if problems > 30 {
			return
		}
		msg := fmt.Sprintf("%s: %s\n%s", name, fmt.Sprintf(format, args...), indent(text))
		if fatal {
			t.Error(msg)
		} else {
			t.Log(msg)
		}
	}
	for _, item := range cases {
		c := js.AsObj(item)
		fresh, _ := c.Get("fresh").(bool)
		counter, _ := c.Get("idCounter").(float64)
		idCounter, warm = int(counter), !fresh
		name, _ := c.Get("name").(string)
		text := caseText(t, c)
		shown := text
		if _, hasText := c.Get("text").(string); !hasText {
			shown = ""
		}
		total++
		captured, hasMeasured = nil, false
		svg, err := renderCase(text, siteConfig(c))
		var unsupported *render.UnsupportedError
		if errors.As(err, &unsupported) {
			// The text is not a block diagram, and the test links no
			// other diagram (a text over maxTextSize is a flowchart).
			total--
			otherType++
			continue
		}

		// The outcome.
		wantError, isError := c.Get("error").(string)
		outcome := ""
		switch {
		case err != nil && strings.HasPrefix(err.Error(), "panic: "):
			outcome = err.Error()
		case isError && err == nil:
			outcome = fmt.Sprintf("no error, want %q", wantError)
		case isError:
			if got := errorMessage(err); got != wantError {
				outcome = fmt.Sprintf("error %q, want %q", got, wantError)
			}
		case err != nil:
			outcome = fmt.Sprintf("error %q, want none", errorMessage(err))
		}
		if outcome != "" {
			if cause := foreignCause(c); cause != "" && !strings.HasPrefix(outcome, "panic: ") {
				foreign++
				t.Logf("%s: %s (%s)", name, outcome, cause)
			} else {
				outcomeFailed++
				report(true, name, shown, "%s", outcome)
			}
			continue
		}
		if !paddingIsNumber(c) {
			// Upstream adds the padding as a string.
			stringPadding++
			continue
		}

		// The measured sizes.
		if want, ok := c.Get("measured").([]any); ok && hasMeasured && captured != nil {
			measureTotal++
			flat := captured.GetBlocksFlat()
			copies := make([]*Block, len(flat))
			for i, b := range flat {
				copyOf := *b
				if i < len(measured) {
					copyOf.Size = measured[i]
				}
				copies[i] = &copyOf
			}
			if d := diffSizes("measured", copies, want); d != "" {
				measureDiff++
				report(strict, name, shown, "%s", d)
			}
		}
		// The sizes and the intersect functions after the draw.
		if drawn := js.AsObj(c.Get("drawn")); drawn != nil && captured != nil {
			drawnTotal++
			want, _ := drawn.Get("sizes").([]any)
			flat := captured.GetBlocksFlat()
			d := diffSizes("drawn", flat, want)
			if d == "" {
				for i, b := range flat {
					if wantFn, _ := js.AsObj(want[i]).Get("intersect").(bool); wantFn != (b.Intersect != nil) {
						d = fmt.Sprintf("drawn[%d] (block %q): intersect function %v, want %v", i, b.idString(), b.Intersect != nil, wantFn)
						break
					}
				}
			}
			if d != "" {
				drawnDiff++
				report(strict, name, shown, "%s", d)
			}
		}
		// The SVG.
		if want, ok := svgs[name]; ok && err == nil {
			svgTotal++
			switch verdict, diff := parity.CompareSVG(svg, want, 0.01); verdict {
			case parity.Exact:
				svgExact++
			case parity.Equivalent:
				svgEquivalent++
			default:
				svgDiff++
				report(strict, name, shown, "svg: %v", diff)
			}
		}
	}
	if total == 0 {
		t.Fatal("no case")
	}
	t.Logf("%d cases: the outcome of %d agrees, of %d differs, of %d differs for a cause outside this package; %d more cases are not block diagrams", total, total-outcomeFailed-foreign, outcomeFailed, foreign, otherType)
	t.Logf("measured sizes: %d of %d cases agree bit for bit; sizes and intersect functions after the draw: %d of %d", measureTotal-measureDiff, measureTotal, drawnTotal-drawnDiff, drawnTotal)
	t.Logf("svg: %d cases, %d exact, %d equivalent, %d different; %d cases with a padding that is not a number are not compared", svgTotal, svgExact, svgEquivalent, svgDiff, stringPadding)
}
