package utils

import (
	"math"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// TestNoPanicOnMalformedInput feeds texts that are not valid input to the
// functions that read user text. A function must not panic and must not
// hang.
func TestNoPanicOnMalformedInput(t *testing.T) {
	// A small generator with a fixed seed.
	state := uint32(0x7574696c)
	next := func() uint32 {
		state += 0x6d2b79f5
		z := state
		z = (z ^ (z >> 15)) * (z | 1)
		z ^= z + (z^(z>>7))*(z|61)
		return z ^ (z >> 14)
	}
	pieces := []string{"", "\x00", "\xff\xfe", "\xed\xa0\x80", "&", "&#", "&#x", "&#x;", "&#99999999999999999999;", ";", "%", "%u", "%uD83D", "<br>", "<br", "<", ">", "\"", "'", ":", "//", "javascript:", "http:", "\\", "\n", "\r", "\t", " ", "é", "日", "🙂", "a", "0", "-", ".", "e", "px", "NaN", "Infinity", "(", "[", "{", "*", "+", "?", "|", "^", "$"}
	text := func() string {
		s := ""
		for n := next() % 8; n > 0; n-- {
			s += pieces[next()%uint32(len(pieces))]
		}
		return s
	}
	// plainText has ASCII pieces only: a character that no listed font has
	// makes the engine load the fallback fonts of the system, which is slow.
	plainText := func() string {
		s := ""
		for n := next() % 8; n > 0; n-- {
			if p := pieces[next()%uint32(len(pieces))]; js.UTF16Len(p) == len(p) && p != "\x00" {
				s += p
			}
		}
		return s
	}
	families := []any{nil, "arial", "\"trebuchet ms\", verdana, arial, sans-serif", "a;b", "", 12.0}
	numbers := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, 1e21, 1e308, -1e308, 5e-324, math.NaN(), math.Inf(1), math.Inf(-1)}
	number := func() float64 { return numbers[next()%uint32(len(numbers))] }
	loose := js.NewObj[any]()
	loose.Set("securityLevel", "loose")
	ctx := newTestContext()

	for i := 0; i < 3000; i++ {
		s := text()
		EntityDecode(s)
		ToBase64(s)
		ParseFontSize(s)
		FormatUrl(s, nil)
		FormatUrl(s, loose)
		InterpolateToCurve(s, nil)
		GetStylesFromArray([]string{s, text()})
		GetEdgeId(s, text(), EdgeIdOptions{Prefix: text()}, text())
		IsLabelCoordinateInPath(Point{X: number(), Y: number()}, s)
		_, _ = IsSubstringInArray(s, []string{text(), s})
		EncodeEntities(s)
		DecodeEntities(s)
		GetErrorMessage(s)

		points := make([]Point, next()%5)
		for j := range points {
			points[j] = Point{X: number(), Y: number()}
		}
		_, _ = CalcLabelPosition(points)
		_, _ = CalculatePoint(points, number())
		_, _ = CalcTerminalLabelPosition(number(), s, points)
		var initial *Point
		if len(points) > 0 {
			initial = &points[0]
		}
		_, _ = CalcCardinalityPosition(i%2 == 0, points, initial)
		fns := GetLineFunctionsWithOffset(s, "arrow_point")
		for j, p := range points {
			fns.X(p, j, points)
			fns.Y(p, j, points)
		}
		fns.X(Point{}, 0, nil)
		fns.Y(Point{}, 0, nil)
		RoundNumber(number(), number())

		// The text functions measure in the document. A text can give the
		// error of upstream, for example a text of white space only.
		if i%10 == 0 {
			p := plainText()
			family := families[next()%uint32(len(families))]
			config := WrapLabelConfig{FontSize: p, FontWeight: number(), FontFamily: family, JoinWith: p}
			_, _ = WrapLabel(ctx, p+" "+plainText(), number(), config)
			_, _ = BreakString(ctx, p, number(), plainText(), config)
			_, _ = CalculateTextDimensions(ctx, p, TextDimensionConfig{FontSize: number(), FontWeight: p, FontFamily: family})
			DrawSimpleText(ctx.Container, TextObj{Text: s, X: number(), Y: number(), Anchor: p, FontSize: s, FontWeight: s, Fill: s, Class: s}).Remove()
		}
	}

	// Values that are not objects, objects that hold themselves, and an
	// array length that is too large.
	cyclic := js.NewObj[any]()
	cyclic.Set("self", cyclic)
	cyclic.Set("list", []any{cyclic, 1.0})
	CleanAndMerge(cyclic, cyclic)
	CleanAndMerge(nil, nil)
	CleanAndMerge("text", 12.0)
	CleanAndMerge([]any{1.0}, []any{js.Null})
	target := js.NewObj[any]()
	target.Set("a", []any{1.0})
	source := js.NewObj[any]()
	long := js.NewObj[any]()
	long.Set("length", 4294967295.0)
	long.Set("4000000000", "x")
	source.Set("a", long)
	CleanAndMerge(target, source)
	if _, err := CleanAndMergeErr(target, source); err == nil {
		t.Error("CleanAndMergeErr gives no error for an array that is too long")
	}
	arrayLike := js.NewObj[any]()
	arrayLike.Set("length", 9007199254740991.0)
	target.Set("b", arrayLike)
	source.Set("b", []any{1.0})
	CleanAndMerge(target, source)
}
