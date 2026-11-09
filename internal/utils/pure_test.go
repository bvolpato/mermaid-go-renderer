package utils

import (
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// pure is the golden file of the functions that need no layout.
func pure(t *testing.T) *js.Obj[any] {
	t.Helper()
	return loadGolden(t, "pure.json.gz")
}

func TestZeroWidthSpace(t *testing.T) {
	if got := pure(t).Get("zeroWidthSpace"); got != ZeroWidthSpace {
		t.Errorf("ZERO_WIDTH_SPACE is %q, want %q", ZeroWidthSpace, got)
	}
}

// TestGenerateIdAndRandom replays the calls of the page, which starts
// Math.random with the seed 0: the zero value of a context.
func TestGenerateIdAndRandom(t *testing.T) {
	g := pure(t)
	ctx := &render.Context{}
	f := &failures{t: t}
	ids, _ := g.Get("generateId").([]any)
	if len(ids) < 3000 {
		t.Fatalf("generateId: %d cases", len(ids))
	}
	for i, want := range ids {
		got := GenerateId(ctx)
		f.check(got == want, "generateId call %d: got %q, want %q", i+1, got, want)
	}
	f.done("generateId")

	f = &failures{t: t}
	for i, c := range cases(t, g, "random", 2000) {
		got := Random(ctx, int(number(c, "length")))
		f.check(got == c.Get("out"), "random case %d: got %q, want %q", i, got, c.Get("out"))
	}
	f.done("random")
}

func TestInterpolateToCurve(t *testing.T) {
	f := &failures{t: t}
	for i, c := range cases(t, pure(t), "interpolateToCurve", 2000) {
		def := d3CurveTypes[str(c, "def")]
		want := d3CurveTypes[str(c, "out")]
		if def == nil || want == nil {
			t.Fatalf("case %d: unknown curve name", i)
		}
		got := InterpolateToCurve(str(c, "interpolate"), def)
		f.check(reflect.ValueOf(got).Pointer() == reflect.ValueOf(want).Pointer(),
			"interpolateToCurve(%q): want %s", str(c, "interpolate"), str(c, "out"))
	}
	f.done("interpolateToCurve")
}

func TestFormatUrl(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "formatUrl", 1500) {
		var got any
		if s, ok := FormatUrl(str(c, "url"), c.Get("config").(*js.Obj[any])); ok {
			got = s
		}
		want := untag(c.Get("out"))
		f.check(same(got, want), "formatUrl(%q, %s): got %s, want %s", str(c, "url"), show(c.Get("config")), show(got), show(want))
	}
	f.done("formatUrl")
}

func TestRoundNumber(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "roundNumber", 2000) {
		num := untag(c.Get("num")).(float64)
		var got float64
		if precision, ok := untag(c.Get("precision")).(float64); ok {
			got = RoundNumber(num, precision)
		} else {
			got = RoundNumber(num)
		}
		want := untag(c.Get("out"))
		f.check(same(got, want), "roundNumber(%s, %s): got %s, want %s", show(num), show(untag(c.Get("precision"))), show(got), show(want))
	}
	f.done("roundNumber")
}

// checkPoint compares a result that is a point or an error with a case.
func checkPoint(f *failures, name string, c *js.Obj[any], got Point, err error) {
	if c.Has("error") {
		f.check(err != nil && err.Error() == c.Get("error"), "%s(%s): got %v and %v, want the error %q",
			name, show(c.Get("points")), got, err, c.Get("error"))
		return
	}
	want := untag(c.Get("out"))
	f.check(err == nil && same(pointValue(got), want), "%s(%s): got %s and %v, want %s",
		name, show(c.Get("points")), show(pointValue(got)), err, show(want))
}

func TestCalcLabelPosition(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "calcLabelPosition", 3000) {
		got, err := CalcLabelPosition(toPoints(c.Get("points")))
		checkPoint(f, "calcLabelPosition", c, got, err)
	}
	f.done("calcLabelPosition")
}

func TestCalculatePoint(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "calculatePoint", 3000) {
		got, err := CalculatePoint(toPoints(c.Get("points")), number(c, "distance"))
		checkPoint(f, "calculatePoint", c, got, err)
	}
	f.done("calculatePoint")
}

func TestCalcCardinalityPosition(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "calcCardinalityPosition", 3000) {
		points := toPoints(c.Get("points"))
		var initial *Point
		switch str(c, "initial") {
		case "first":
			if len(points) > 0 {
				initial = &points[0]
			}
		case "last":
			if len(points) > 0 {
				initial = &points[len(points)-1]
			}
		default:
			initial = &Point{}
		}
		got, err := CalcCardinalityPosition(c.Get("rel").(bool), points, initial)
		checkPoint(f, "calcCardinalityPosition", c, got, err)
		// The function reverses the points of the caller in place.
		after := toPoints(c.Get("after"))
		f.check(samePoints(points, after), "calcCardinalityPosition(%s, %s): the points are %v afterwards, want %v",
			show(c.Get("points")), str(c, "initial"), points, after)
	}
	f.done("calcCardinalityPosition")
}

func TestCalcTerminalLabelPosition(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "calcTerminalLabelPosition", 3000) {
		points := toPoints(c.Get("points"))
		got, err := CalcTerminalLabelPosition(number(c, "size"), str(c, "position"), points)
		checkPoint(f, "calcTerminalLabelPosition "+str(c, "position"), c, got, err)
		after := toPoints(c.Get("after"))
		f.check(samePoints(points, after), "calcTerminalLabelPosition changed its points")
	}
	f.done("calcTerminalLabelPosition")
}

func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.(string)
	}
	return out
}

func TestGetStylesFromArray(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "getStylesFromArray", 2000) {
		style, labelStyle := GetStylesFromArray(stringList(c.Get("arr")))
		want := c.Get("out").(*js.Obj[any])
		f.check(style == want.Get("style") && labelStyle == want.Get("labelStyle"),
			"getStylesFromArray(%s): got %q and %q, want %s", show(c.Get("arr")), style, labelStyle, show(want))
	}
	f.done("getStylesFromArray")
}

func TestParseFontSize(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "parseFontSize", 2000) {
		in := untag(c.Get("v"))
		n, px, ok := ParseFontSize(in)
		got := []any{nil, nil}
		if ok {
			got = []any{n, px}
		}
		want := untag(c.Get("out"))
		f.check(same(got, want), "parseFontSize(%s): got %s, want %s", show(in), show(got), show(want))
	}
	f.done("parseFontSize")
}

func TestGetEdgeId(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "getEdgeId", 2000) {
		o := c.Get("opts").(*js.Obj[any])
		opts := EdgeIdOptions{Counter: int(number(o, "counter")), Prefix: str(o, "prefix"), Suffix: str(o, "suffix")}
		got := GetEdgeId(str(c, "from"), str(c, "to"), opts, str(c, "id"))
		f.check(got == c.Get("out"), "getEdgeId(%q, %q, %s, %s): got %q, want %q",
			str(c, "from"), str(c, "to"), show(o), show(untag(c.Get("id"))), got, c.Get("out"))
	}
	f.done("getEdgeId")
}

func TestHandleUndefinedAttr(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "handleUndefinedAttr", 9) {
		in := untag(c.Get("v"))
		got := HandleUndefinedAttr(in)
		// The oracle gives null where the Go function gives nil: both
		// remove the attribute.
		want := untag(c.Get("out"))
		if js.IsNullish(want) {
			want = nil
		}
		f.check(same(got, want), "handleUndefinedAttr(%s): got %s, want %s", show(in), show(got), show(want))
	}
	f.done("handleUndefinedAttr")
}

func TestIsLabelCoordinateInPath(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "isLabelCoordinateInPath", 2500) {
		p := c.Get("point").(*js.Obj[any])
		got := IsLabelCoordinateInPath(Point{X: number(p, "x"), Y: number(p, "y")}, str(c, "d"))
		f.check(got == c.Get("out"), "isLabelCoordinateInPath(%s, %q): got %v", show(p), str(c, "d"), got)
	}
	f.done("isLabelCoordinateInPath")
}

func TestIsSubstringInArray(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "isSubstringInArray", 1500) {
		got, err := IsSubstringInArray(str(c, "str"), stringList(c.Get("arr")))
		if c.Has("error") {
			f.check(err != nil, "isSubstringInArray(%q): got %d, want the error %q", str(c, "str"), got, c.Get("error"))
			continue
		}
		f.check(err == nil && float64(got) == c.Get("out"), "isSubstringInArray(%q, %s): got %d and %v, want %s",
			str(c, "str"), show(c.Get("arr")), got, err, show(c.Get("out")))
	}
	f.done("isSubstringInArray")
}

func TestCleanAndMerge(t *testing.T) {
	f := &failures{t: t}
	errors := 0
	for _, c := range cases(t, pure(t), "cleanAndMerge", 3000) {
		a, b := untag(c.Get("a")), untag(c.Get("b"))
		got, err := CleanAndMergeErr(a, b)
		if c.Has("error") {
			errors++
			f.check(err != nil, "cleanAndMerge(%s, %s): got %s, want the error %q", show(a), show(b), show(got), c.Get("error"))
			continue
		}
		want := untag(c.Get("out"))
		f.check(err == nil && same(got, want), "cleanAndMerge(%s, %s):\n got %s (%v)\nwant %s", show(a), show(b), show(got), err, show(want))
		// The function without the error gives the same result.
		f.check(same(CleanAndMerge(untag(c.Get("a")), untag(c.Get("b"))), want), "CleanAndMerge differs from CleanAndMergeErr")
		// The inputs are not changed.
		f.check(same(a, untag(c.Get("a"))) && same(b, untag(c.Get("b"))), "cleanAndMerge(%s, %s) changed its arguments", show(a), show(b))
	}
	if errors == 0 {
		t.Error("cleanAndMerge: no case with an error")
	}
	f.done("cleanAndMerge")
}

func TestEntityDecode(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "entityDecode", 2500) {
		got := EntityDecode(str(c, "html"))
		f.check(got == c.Get("out"), "entityDecode(%q): got %q, want %q", str(c, "html"), got, c.Get("out"))
	}
	f.done("entityDecode")
}

func TestInitIDGenerator(t *testing.T) {
	f := &failures{t: t}
	ctx := newTestContext()
	for _, c := range cases(t, pure(t), "initIDGenerator", 18) {
		deterministic, _ := untag(c.Get("deterministic")).(bool)
		g := NewInitIDGenerator(ctx, deterministic, str(c, "seed"))
		var got []any
		for i := 0; i < int(number(c, "calls")); i++ {
			got = append(got, g.Next())
		}
		f.check(same(got, c.Get("out")), "InitIDGenerator(%v, %q): got %s, want %s", deterministic, str(c, "seed"), show(got), show(c.Get("out")))
	}
	f.done("InitIDGenerator")
}

func TestGetSubGraphTitleMargins(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "getSubGraphTitleMargins", 500) {
		m := GetSubGraphTitleMargins(c.Get("config").(*js.Obj[any]))
		got := js.NewObj[any]()
		got.Set("subGraphTitleTopMargin", m.SubGraphTitleTopMargin)
		got.Set("subGraphTitleBottomMargin", m.SubGraphTitleBottomMargin)
		got.Set("subGraphTitleTotalMargin", m.SubGraphTitleTotalMargin)
		want := untag(c.Get("out"))
		f.check(same(got, want), "getSubGraphTitleMargins(%s): got %s, want %s", show(c.Get("config")), show(got), show(want))
	}
	f.done("getSubGraphTitleMargins")
}

func TestToBase64(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "toBase64", 1000) {
		got := ToBase64(str(c, "s"))
		f.check(got == c.Get("out"), "toBase64(%q): got %q, want %q", str(c, "s"), got, c.Get("out"))
	}
	f.done("toBase64")
}

func TestMarkerOffsets(t *testing.T) {
	g := pure(t)
	for name, lookup := range map[string]func(string) (float64, bool){"markerOffsets": MarkerOffset, "markerOffsets2": MarkerOffset2} {
		want := g.Get(name).(*js.Obj[any])
		if want.Len() == 0 {
			t.Fatalf("%s is empty", name)
		}
		want.Each(func(k string, v any) {
			if got, ok := lookup(k); !ok || got != v {
				t.Errorf("%s[%q]: got %v and %v, want %v", name, k, got, ok, v)
			}
		})
		if _, ok := lookup("no such marker"); ok {
			t.Errorf("%s has a key that upstream does not have", name)
		}
	}
	if len(markerOffsets) != g.Get("markerOffsets").(*js.Obj[any]).Len() || len(markerOffsets2) != g.Get("markerOffsets2").(*js.Obj[any]).Len() {
		t.Error("the marker offset tables have another number of keys than upstream")
	}
}

func TestGetLineFunctionsWithOffset(t *testing.T) {
	f := &failures{t: t}
	for _, c := range cases(t, pure(t), "lineWithOffset", 3000) {
		edge := c.Get("edge").(*js.Obj[any])
		data := toPoints(c.Get("data"))
		fns := GetLineFunctionsWithOffset(str(edge, "arrowTypeStart"), str(edge, "arrowTypeEnd"))
		got := make([]any, len(data))
		for i, d := range data {
			got[i] = []any{fns.X(d, i, data), fns.Y(d, i, data)}
		}
		want := untag(c.Get("out"))
		f.check(same(got, want), "getLineFunctionsWithOffset(%s) on %s:\n got %s\nwant %s", show(edge), show(c.Get("data")), show(got), show(want))
	}
	f.done("getLineFunctionsWithOffset")
}

func TestErrorHelpers(t *testing.T) {
	g := pure(t).Get("errors").(*js.Obj[any])
	gotMessages := []any{
		GetErrorMessage(errNoSuitablePoint),
		GetErrorMessage("text"),
		GetErrorMessage(12.0),
		GetErrorMessage(nil),
		GetErrorMessage(js.Null),
	}
	// The first message of the oracle is the message of `new Error('boom')`.
	wantMessages := g.Get("getErrorMessage").([]any)
	wantMessages[0] = errNoSuitablePoint.Error()
	if !same(gotMessages, wantMessages) {
		t.Errorf("getErrorMessage: got %s, want %s", show(gotMessages), show(wantMessages))
	}

	withStr := js.NewObj[any]()
	withStr.Set("str", "x")
	withMessage := js.NewObj[any]()
	withMessage.Set("message", "x")
	gotDetailed := []any{IsDetailedError(withStr), IsDetailedError(withMessage), IsDetailedError(errNoSuitablePoint)}
	if !same(gotDetailed, g.Get("isDetailedError")) {
		t.Errorf("isDetailedError: got %s, want %s", show(gotDetailed), show(g.Get("isDetailedError")))
	}
	if !IsDetailedError(&DetailedError{Str: "x"}) {
		t.Error("isDetailedError is false for a DetailedError")
	}
}

func TestImperativeState(t *testing.T) {
	g := pure(t).Get("imperativeState").(*js.Obj[any])
	type records struct {
		foo float64
		bar []any
	}
	value := func(r records) any {
		o := js.NewObj[any]()
		o.Set("foo", r.foo)
		o.Set("bar", r.bar)
		return o
	}
	st := NewImperativeState(func() records { return records{foo: 1, bar: []any{}} })
	st.Records.foo = 2
	st.Records.bar = append(st.Records.bar, 1.0)
	if got := value(st.Records); !same(got, g.Get("beforeReset")) {
		t.Errorf("before reset: got %s, want %s", show(got), show(g.Get("beforeReset")))
	}
	st.Reset()
	if got := value(st.Records); !same(got, g.Get("afterReset")) {
		t.Errorf("after reset: got %s, want %s", show(got), show(g.Get("afterReset")))
	}
}

// isString is not in the bundle of mermaid 12.2.0, so it has no oracle.
func TestIsString(t *testing.T) {
	if !IsString("") || !IsString("a") || IsString(nil) || IsString(1.0) || IsString(js.Null) {
		t.Error("isString")
	}
}

func TestRunFunc(t *testing.T) {
	var got []any
	window := js.NewObj[any]()
	ns := js.NewObj[any]()
	window.Set("app", ns)
	ns.Set("callback", func(params ...any) { got = params })
	ns.Set("value", 1.0)
	if err := RunFunc(window, "app.callback", "a", 2.0); err != nil || len(got) != 2 || got[0] != "a" || got[1] != 2.0 {
		t.Errorf("runFunc: got %v and %v", got, err)
	}
	if err := RunFunc(window, "missing.callback"); err == nil {
		t.Error("runFunc with a missing object gives no error")
	}
	if err := RunFunc(window, "app.value"); err == nil {
		t.Error("runFunc with a value that is not a function gives no error")
	}
}
