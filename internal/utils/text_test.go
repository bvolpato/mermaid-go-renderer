package utils

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// textConfig reads { fontSize, fontWeight, fontFamily, joinWith } of a case.
// A property that has the value undefined is nil, as a missing property.
func textConfig(c *js.Obj[any]) WrapLabelConfig {
	return WrapLabelConfigOf(untag(c.Get("config")).(*js.Obj[any]))
}

// TestTextMeasurement replays the calls of the page in the same order with
// one context, because the memoize caches of upstream take part in the
// results: a call can get the cached result of an earlier call that has the
// same key text.
func TestTextMeasurement(t *testing.T) {
	g := loadGolden(t, "text.json.gz")
	ops := cases(t, g, "ops", 6000)
	ctx := newTestContext()
	// The sizes, the lines and the pieces of a text come from the text
	// widths of Chromium on the macOS reference machine. A machine with
	// other fonts compares only the errors.
	reference := fonttest.IsMacOSReference()
	counts := map[string]*failures{}
	for i, op := range ops {
		fn := str(op, "fn")
		f := counts[fn]
		if f == nil {
			f = &failures{t: t}
			counts[fn] = f
		}
		text := str(op, "text")
		config := textConfig(op)
		var got any
		var err error
		switch fn {
		case "dims":
			var d TextDimensions
			d, err = CalculateTextDimensions(ctx, text, config.TextDimensionConfig())
			o := js.NewObj[any]()
			o.Set("width", d.Width)
			o.Set("height", d.Height)
			if d.HasLineHeight {
				o.Set("lineHeight", d.LineHeight)
			}
			got = o
		case "width":
			got, err = CalculateTextWidth(ctx, text, config.TextDimensionConfig())
		case "height":
			got, err = CalculateTextHeight(ctx, text, config.TextDimensionConfig())
		case "wrap":
			got, err = WrapLabel(ctx, text, number(op, "maxWidth"), config)
		case "break":
			hyphen := "-"
			if h, ok := untag(op.Get("hyphen")).(string); ok {
				hyphen = h
			}
			var b BreakStringOutput
			b, err = BreakString(ctx, text, number(op, "maxWidth"), hyphen, config)
			lines := make([]any, len(b.HyphenatedStrings))
			for j, s := range b.HyphenatedStrings {
				lines[j] = s
			}
			o := js.NewObj[any]()
			o.Set("hyphenatedStrings", lines)
			o.Set("remainingWord", b.RemainingWord)
			got = o
		default:
			t.Fatalf("op %d: unknown function %q", i, fn)
		}
		if op.Has("error") {
			f.check(err != nil && err.Error() == op.Get("error"), "op %d %s(%q, %s): got %s and %v, want the error %q",
				i, fn, text, show(op.Get("config")), show(got), err, op.Get("error"))
			continue
		}
		want := untag(op.Get("out"))
		f.check(err == nil && (!reference || same(got, want)), "op %d %s(%q, %s, %s):\n got %s (%v)\nwant %s",
			i, fn, text, show(op.Get("maxWidth")), show(op.Get("config")), show(got), err, show(want))
	}
	for _, fn := range []string{"dims", "width", "height", "wrap", "break"} {
		if counts[fn] == nil {
			t.Errorf("no %s case", fn)
			continue
		}
		counts[fn].done(fn)
	}

	// A call that fails leaves its svg element in the body, as upstream.
	var left []any
	for _, el := range ctx.Body.Children() {
		name := el.Tag
		if id := el.ID(); id != "" {
			name += "#" + id
		}
		left = append(left, name)
	}
	if !same(left, g.Get("leftInBody")) {
		t.Errorf("children of the body after the calls: got %s, want %s", show(left), show(g.Get("leftInBody")))
	}
	fonttest.RequireMacOSReference(t)
}

// newSVG appends an svg element to the container, with the given number of
// children, as the page does for a case that builds DOM.
func newSVG(container *dom.Node, before int) *dom.Node {
	svg := container.Append("svg")
	for i := 0; i < before; i++ {
		if i%2 == 1 {
			svg.Append("g")
		} else {
			svg.Append("circle")
		}
	}
	return svg
}

func TestGetTextObj(t *testing.T) {
	want := untag(loadGolden(t, "dom.json.gz").Get("getTextObj")).(*js.Obj[any])
	o := GetTextObj()
	got := js.NewObj[any]()
	got.Set("x", o.X)
	got.Set("y", o.Y)
	got.Set("fill", o.Fill)
	got.Set("anchor", o.Anchor)
	got.Set("style", o.Style)
	got.Set("width", o.Width)
	got.Set("height", o.Height)
	got.Set("textMargin", o.TextMargin)
	got.Set("rx", o.Rx)
	got.Set("ry", o.Ry)
	got.Set("valign", o.Valign)
	got.Set("text", o.Text)
	if !same(got, want) {
		t.Errorf("getTextObj: got %s, want %s", show(got), show(want))
	}
	if o.FontFamily != nil || o.FontSize != nil || o.FontWeight != nil || o.Class != nil {
		t.Error("getTextObj sets a field that upstream does not set")
	}
}

func TestDrawSimpleText(t *testing.T) {
	f := &failures{t: t}
	ctx := newTestContext()
	for i, c := range cases(t, loadGolden(t, "dom.json.gz"), "drawSimpleText", 1200) {
		d := untag(c.Get("data")).(*js.Obj[any])
		textData := TextObj{
			Text:       str(d, "text"),
			X:          number(d, "x"),
			Y:          number(d, "y"),
			Anchor:     str(d, "anchor"),
			FontFamily: d.Get("fontFamily"),
			FontSize:   d.Get("fontSize"),
			FontWeight: d.Get("fontWeight"),
			Fill:       d.Get("fill"),
			Class:      d.Get("class"),
			TextMargin: number(d, "textMargin"),
		}
		svg := newSVG(ctx.Container, 0)
		elem := DrawSimpleText(svg, textData)
		html := svg.InnerHTML()
		svg.Remove()
		f.check(html == c.Get("html") && elem.OuterHTML() == c.Get("ret"), "drawSimpleText case %d %s:\n got %s\nwant %s", i, show(d), html, c.Get("html"))
	}
	f.done("drawSimpleText")
}
