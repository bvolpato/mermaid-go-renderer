package svgdraw

// The golden file testdata/dom.json.gz comes from
// tools/reference/oracle-utils.mjs, which runs the functions of
// svgDrawCommon.ts of mermaid 12.2.0 in headless Chromium and records the
// markup that they build. Rerun it with `node oracle-utils.mjs` in
// tools/reference.

import (
	"compress/gzip"
	"io"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func loadGolden(t *testing.T) *js.Obj[any] {
	t.Helper()
	f, err := os.Open("testdata/dom.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return v.(*js.Obj[any])
}

// untag decodes the tagged form of the golden file: {"$u":1} is undefined.
func untag(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = untag(e)
		}
		return out
	case *js.Obj[any]:
		if x.Len() == 1 && x.Has("$u") {
			return nil
		}
		out := js.NewObj[any]()
		x.Each(func(k string, e any) { out.Set(k, untag(e)) })
		return out
	}
	return v
}

func cases(t *testing.T, g *js.Obj[any], name string, atLeast int) []*js.Obj[any] {
	t.Helper()
	list, _ := g.Get(name).([]any)
	if len(list) < atLeast {
		t.Fatalf("%s: %d cases, want at least %d", name, len(list), atLeast)
	}
	out := make([]*js.Obj[any], len(list))
	for i, c := range list {
		out[i] = c.(*js.Obj[any])
	}
	return out
}

func str(o *js.Obj[any], key string) string {
	s, _ := o.Get(key).(string)
	return s
}

func number(o *js.Obj[any], key string) float64 {
	n, _ := o.Get(key).(float64)
	return n
}

// newTestContext makes the context of a render in a document that is built
// like the page of the reference: body > div#container.
func newTestContext() *render.Context {
	doc := browser.NewDocument()
	container := doc.Body.Append("div").Attr("id", "container")
	return &render.Context{
		ID:        render.DefaultID,
		Doc:       doc,
		Engine:    doc.Engine.(*browser.Engine),
		Body:      doc.Body,
		Container: container,
		Now:       render.DefaultNow,
	}
}

// newSVG appends an svg element to the container, with the given number of
// children, as the page does for each case.
func newSVG(ctx *render.Context, before int) *dom.Node {
	svg := ctx.Container.Append("svg")
	for i := 0; i < before; i++ {
		if i%2 == 1 {
			svg.Append("g")
		} else {
			svg.Append("circle")
		}
	}
	return svg
}

type failures struct {
	t     *testing.T
	total int
	count int
}

func (f *failures) check(ok bool, format string, args ...any) {
	f.total++
	if ok {
		return
	}
	f.count++
	if f.count <= 12 {
		f.t.Errorf(format, args...)
	}
}

func (f *failures) done(name string) {
	if f.count > 0 {
		f.t.Errorf("%s: %d of %d cases fail", name, f.count, f.total)
	} else {
		f.t.Logf("%s: %d cases", name, f.total)
	}
}

func TestDrawRect(t *testing.T) {
	f := &failures{t: t}
	ctx := newTestContext()
	for i, c := range cases(t, loadGolden(t), "drawRect", 1500) {
		d := untag(c.Get("data")).(*js.Obj[any])
		rectData := RectData{
			X:      number(d, "x"),
			Y:      number(d, "y"),
			Fill:   d.Get("fill"),
			Stroke: d.Get("stroke"),
			Width:  number(d, "width"),
			Height: number(d, "height"),
			Class:  str(d, "class"),
			Color:  str(d, "color"),
			Rx:     number(d, "rx"),
			Ry:     number(d, "ry"),
			Anchor: str(d, "anchor"),
			Name:   str(d, "name"),
		}
		if attrs, ok := d.Get("attrs").(*js.Obj[any]); ok {
			rectData.Attrs = attrs
		}
		svg := newSVG(ctx, 0)
		elem := DrawRect(svg, rectData)
		html := svg.InnerHTML()
		svg.Remove()
		f.check(html == c.Get("html") && elem.OuterHTML() == c.Get("ret"), "drawRect case %d:\n got %s\nwant %s", i, html, c.Get("html"))
	}
	f.done("drawRect")
}

func TestDrawBackgroundRect(t *testing.T) {
	f := &failures{t: t}
	ctx := newTestContext()
	for i, c := range cases(t, loadGolden(t), "drawBackgroundRect", 800) {
		b := untag(c.Get("bounds")).(*js.Obj[any])
		bounds := Bound{
			Startx: number(b, "startx"),
			Stopx:  number(b, "stopx"),
			Starty: number(b, "starty"),
			Stopy:  number(b, "stopy"),
			Fill:   b.Get("fill"),
			Stroke: b.Get("stroke"),
		}
		svg := newSVG(ctx, int(number(c, "before")))
		DrawBackgroundRect(svg, bounds)
		html := svg.InnerHTML()
		svg.Remove()
		f.check(html == c.Get("html"), "drawBackgroundRect case %d:\n got %s\nwant %s", i, html, c.Get("html"))
	}
	f.done("drawBackgroundRect")
}

func TestDrawText(t *testing.T) {
	f := &failures{t: t}
	ctx := newTestContext()
	for i, c := range cases(t, loadGolden(t), "drawText", 1500) {
		d := untag(c.Get("data")).(*js.Obj[any])
		textData := TextData{
			X:          number(d, "x"),
			Y:          number(d, "y"),
			Anchor:     str(d, "anchor"),
			Text:       str(d, "text"),
			TextMargin: number(d, "textMargin"),
			Class:      str(d, "class"),
		}
		svg := newSVG(ctx, 0)
		elem := DrawText(svg, textData)
		html := svg.InnerHTML()
		svg.Remove()
		f.check(html == c.Get("html") && elem.OuterHTML() == c.Get("ret"), "drawText case %d:\n got %s\nwant %s", i, html, c.Get("html"))
	}
	f.done("drawText")
}

func TestDrawImage(t *testing.T) {
	g := loadGolden(t)
	ctx := newTestContext()
	for name, draw := range map[string]func(*dom.Node, float64, float64, string){"drawImage": DrawImage, "drawEmbeddedImage": DrawEmbeddedImage} {
		f := &failures{t: t}
		for i, c := range cases(t, g, name, 400) {
			svg := newSVG(ctx, 0)
			draw(svg, number(c, "x"), number(c, "y"), str(c, "link"))
			html := svg.InnerHTML()
			svg.Remove()
			f.check(html == c.Get("html"), "%s case %d (%q):\n got %s\nwant %s", name, i, str(c, "link"), html, c.Get("html"))
		}
		f.done(name)
	}
}

func TestGetNoteRect(t *testing.T) {
	want := loadGolden(t).Get("getNoteRect").(*js.Obj[any])
	r := GetNoteRect()
	got := js.NewObj[any]()
	got.Set("x", r.X)
	got.Set("y", r.Y)
	got.Set("width", r.Width)
	got.Set("height", r.Height)
	got.Set("fill", r.Fill)
	got.Set("stroke", r.Stroke)
	got.Set("anchor", r.Anchor)
	got.Set("rx", r.Rx)
	got.Set("ry", r.Ry)
	if js.StringifyJSON(got) != js.StringifyJSON(want) {
		t.Errorf("getNoteRect: got %s, want %s", js.StringifyJSON(got), js.StringifyJSON(want))
	}
	if r.Class != "" || r.Color != "" || r.Name != "" || r.Attrs != nil {
		t.Error("getNoteRect sets a field that upstream does not set")
	}
}

func TestGetTextObj(t *testing.T) {
	want := loadGolden(t).Get("getTextObj").(*js.Obj[any])
	o := GetTextObj()
	got := js.NewObj[any]()
	got.Set("x", o.X)
	got.Set("y", o.Y)
	got.Set("width", o.Width)
	got.Set("height", o.Height)
	got.Set("text-anchor", o.TextAnchor)
	got.Set("style", o.Style)
	got.Set("textMargin", o.TextMargin)
	got.Set("rx", o.Rx)
	got.Set("ry", o.Ry)
	got.Set("tspan", o.Tspan)
	if js.StringifyJSON(got) != js.StringifyJSON(want) {
		t.Errorf("getTextObj: got %s, want %s", js.StringifyJSON(got), js.StringifyJSON(want))
	}
	if o.Fill != nil || o.Anchor != nil || o.Valign != nil || o.Class != nil || o.Dy != nil ||
		o.FontFamily != nil || o.FontSize != nil || o.FontWeight != nil || o.Text != "" {
		t.Error("getTextObj sets a field that upstream does not set")
	}
}

func TestCreateTooltip(t *testing.T) {
	want := loadGolden(t).Get("createTooltip").(*js.Obj[any])
	ctx := newTestContext()
	first := CreateTooltip(ctx)
	if got := first.OuterHTML(); got != want.Get("html") {
		t.Errorf("createTooltip:\n got %s\nwant %s", got, want.Get("html"))
	}
	// The second call returns the element of the first call.
	second := CreateTooltip(ctx)
	if (first == second) != want.Get("same").(bool) {
		t.Errorf("createTooltip returns another element for the second call")
	}
	if first.Parent != ctx.Body {
		t.Errorf("the tooltip is not a child of the body")
	}
	var children []any
	for _, el := range ctx.Body.Children() {
		children = append(children, el.Tag)
	}
	if js.StringifyJSON(children) != js.StringifyJSON(want.Get("bodyChildren")) {
		t.Errorf("children of the body: got %s, want %s", js.StringifyJSON(children), js.StringifyJSON(want.Get("bodyChildren")))
	}
}
