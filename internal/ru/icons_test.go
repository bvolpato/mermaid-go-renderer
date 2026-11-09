package ru

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// loadIconGolden reads testdata/icons.json.gz, which
// tools/reference/gen-icons.mjs records from mermaid in Chromium.
func loadIconGolden(t *testing.T) *js.Obj[any] {
	t.Helper()
	f, err := os.Open("testdata/icons.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return js.AsObj(v)
}

// TestIconsAgainstMermaid replays the steps of the golden file in one
// context, as the probe runs them in one page: the counter of replaceIDs
// goes on from step to step.
func TestIconsAgainstMermaid(t *testing.T) {
	golden := loadIconGolden(t)
	groups := golden.Get("groups").([]any)
	firstConfig := js.AsObj(js.AsObj(groups[0]).Get("config"))
	ctx := render.NewContext(render.ContextOptions{Config: firstConfig})
	// The shadow context counts the calls of Math.random.
	shadow := render.NewContext(render.ContextOptions{Config: firstConfig})

	if got, want := iconIDPrefix(ctx), js.ToString(golden.Get("randomPrefix")); got != want {
		t.Fatalf("randomPrefix: got %q, want %q", got, want)
	}

	registerErrors := golden.Get("registerErrors").([]any)
	packs := golden.Get("packs").([]any)
	for i, p := range packs {
		pack := js.AsObj(p)
		name, _ := pack.Get("name").(string)
		loader := IconLoader{Name: name, Icons: js.AsObj(pack.Get("icons"))}
		if loaded := js.AsObj(pack.Get("loader")); loaded != nil {
			loader.Loader = func() (*js.Obj[any], error) { return loaded, nil }
		} else if js.Truthy(pack.Get("loaderFails")) {
			loader.Loader = func() (*js.Obj[any], error) { return nil, errors.New("no network") }
		}
		err := RegisterIconPacks(ctx, loader)
		want := registerErrors[i]
		if js.IsNullish(want) != (err == nil) || (err != nil && err.Error() != js.ToString(want)) {
			t.Errorf("registerIconPacks %d: got %v, want %v", i, err, want)
		}
	}

	// The direct calls of the @iconify/utils functions.
	units := golden.Get("units").([]any)
	unitFailed := 0
	for ui, u := range units {
		unit := js.AsObj(u)
		got, err := runIconUnit(ctx, packs, unit)
		fail := func(format string, args ...any) {
			unitFailed++
			if unitFailed <= 20 {
				t.Errorf("unit %d %s: "+format, append([]any{ui, js.StringifyJSON(unit)}, args...)...)
			}
		}
		if unit.Has("error") != (err != nil) {
			fail("error: got %v, want %v", err, unit.Get("error"))
		} else if err == nil {
			want, perr := js.ParseJSON([]byte(js.ToString(unit.Get("json"))))
			if perr != nil {
				t.Fatal(perr)
			}
			if g, w := js.StringifyJSON(got), js.StringifyJSON(want); g != w {
				fail("\n got  %s\n want %s", g, w)
			}
		}
		for n := int(js.ToNumber(unit.Get("randomCalls"))); n > 0; n-- {
			shadow.Random()
		}
		if shadow.Random() != ctx.Random() {
			fail("Math.random calls differ from %v", unit.Get("randomCalls"))
			return
		}
	}
	t.Logf("%d units, %d different", len(units), unitFailed)

	total, failed := 0, 0
	for gi, g := range groups {
		group := js.AsObj(g)
		ctx.Config = js.AsObj(group.Get("config"))
		for si, s := range group.Get("steps").([]any) {
			step := js.AsObj(s)
			name := js.ToString(step.Get("name"))
			total++
			fail := func(format string, args ...any) {
				failed++
				if failed <= 20 {
					t.Errorf("group %d step %d %s %q %s: "+format, append([]any{gi, si, step.Get("op"), name, js.StringifyJSON(step.Get("customisations"))}, args...)...)
				}
			}
			if step.Get("op") == "available" {
				if got, want := IsIconAvailable(ctx, name), js.Truthy(step.Get("available")); got != want {
					fail("available: got %v, want %v", got, want)
				}
			} else {
				var customisations map[string]any
				if c := js.AsObj(step.Get("customisations")); c != nil {
					customisations = map[string]any{}
					c.Each(func(k string, v any) { customisations[k] = v })
				}
				var extra *js.Obj[string]
				if e := js.AsObj(step.Get("extraAttributes")); e != nil {
					extra = js.NewObj[string]()
					e.Each(func(k string, v any) { extra.Set(k, js.ToString(v)) })
				}
				got, err := GetIconSVG(ctx, name, customisations, extra)
				wantErr := step.Has("error")
				switch {
				case wantErr != (err != nil):
					fail("error: got %v, want %v", err, step.Get("error"))
				case err == nil && got != js.ToString(step.Get("svg")):
					fail("svg:\n got  %s\n want %s", got, js.ToString(step.Get("svg")))
				}
			}
			for n := int(js.ToNumber(step.Get("randomCalls"))); n > 0; n-- {
				shadow.Random()
			}
			if shadow.Random() != ctx.Random() {
				fail("Math.random calls differ from %v", step.Get("randomCalls"))
				return
			}
		}
	}
	t.Logf("%d steps, %d different", total, failed)
}

// runIconUnit runs one direct call of an @iconify/utils function and
// returns its result as a dynamic value.
func runIconUnit(ctx *render.Context, packs []any, unit *js.Obj[any]) (any, error) {
	str := func(key string) string { return js.ToString(unit.Get(key)) }
	switch str("fn") {
	case "stringToIcon":
		icon, ok := stringToIcon(str("value"), js.Truthy(unit.Get("validate")), js.Truthy(unit.Get("allowSimpleName")))
		if !ok {
			return js.Null, nil
		}
		o := js.NewObj[any]()
		o.Set("provider", icon.provider)
		o.Set("prefix", icon.prefix)
		o.Set("name", icon.name)
		return o, nil
	case "getIconData":
		data := js.AsObj(js.AsObj(packs[int(js.ToNumber(unit.Get("pack")))]).Get("icons"))
		icon, err := getIconData(data, str("name"))
		if err != nil {
			return nil, err
		}
		if icon == nil {
			return js.Null, nil
		}
		return icon, nil
	case "iconToSVG":
		res, err := iconToSVG(js.AsObj(unit.Get("icon")), js.AsObj(unit.Get("customisations")))
		if err != nil {
			return nil, err
		}
		attributes := js.NewObj[any]()
		res.attributes.Each(func(k, v string) { attributes.Set(k, v) })
		o := js.NewObj[any]()
		o.Set("attributes", attributes)
		o.Set("viewBox", res.viewBox)
		if res.body != nil {
			o.Set("body", res.body)
		}
		return o, nil
	case "calculateSize":
		v := calculateSize(unit.Get("size"), js.ToNumber(unit.Get("num"))/js.ToNumber(unit.Get("den")))
		o := js.NewObj[any]()
		o.Set("type", js.TypeOf(v))
		o.Set("text", js.ToString(v))
		return o, nil
	case "wrapSVGContent":
		return wrapSVGContent(str("body"), str("start"), str("end"))
	case "replaceIDs":
		return replaceIDs(ctx, str("body")), nil
	case "iconToHTML":
		attributes := js.NewObj[string]()
		js.AsObj(unit.Get("attributes")).Each(func(k string, v any) { attributes.Set(k, js.ToString(v)) })
		return iconToHTML(str("body"), attributes), nil
	}
	return nil, fmt.Errorf("unknown unit %s", str("fn"))
}

// TestSplitSVGDefsEnds checks the bodies for which the loop of upstream
// does not end: the port returns an error. A node script with a timeout
// showed that wrapSVGContent of @iconify/utils hangs for these bodies.
func TestSplitSVGDefsEnds(t *testing.T) {
	for _, body := range []string{"</defs><defs></defs>", "a</defs>b<defs>c</defs>", "</defs> <defs>"} {
		if _, err := wrapSVGContent(body, "<g>", "</g>"); err == nil {
			t.Errorf("wrapSVGContent(%q): no error", body)
		}
	}
}

// TestIconIDState checks the state of replaceIDs that a program can set:
// the prefix of the ids and the counter.
func TestIconIDState(t *testing.T) {
	cfg := js.NewObj[any]()
	cfg.Set("securityLevel", "loose")
	body := `<g id="a"><use href="#a"/></g>`

	ctx := render.NewContext(render.ContextOptions{Config: cfg})
	if got, want := replaceIDs(ctx, body), `<g id="IconifyId19bc186de00aa85aa0"><use href="#IconifyId19bc186de00aa85aa0"/></g>`; got != want {
		t.Errorf("default state: got %s, want %s", got, want)
	}
	if got, want := replaceIDs(ctx, body), `<g id="IconifyId19bc186de00aa85aa1"><use href="#IconifyId19bc186de00aa85aa1"/></g>`; got != want {
		t.Errorf("second call: got %s, want %s", got, want)
	}

	ctx = render.NewContext(render.ContextOptions{Config: cfg})
	SetIconIDPrefix(ctx, "P")
	SkipIconIDs(ctx, 7)
	if got, want := replaceIDs(ctx, body), `<g id="P7"><use href="#P7"/></g>`; got != want {
		t.Errorf("set state: got %s, want %s", got, want)
	}

	attrs := IconAttrs("class", "label-icon", "2", "b", "1", "a")
	if got, want := iconToHTML("", attrs), `<svg xmlns="http://www.w3.org/2000/svg" 1="a" 2="b" class="label-icon"></svg>`; got != want {
		t.Errorf("IconAttrs: got %s, want %s", got, want)
	}
}

// TestIconsMalformed registers icon sets with values of random types and
// asks for their icons. The functions must not panic: they give the
// unknown icon or an error.
func TestIconsMalformed(t *testing.T) {
	seed := uint32(0x1c0)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>8) % n
	}
	var value func(depth int) any
	value = func(depth int) any {
		switch next(10) {
		case 0:
			return nil
		case 1:
			return js.Null
		case 2:
			return next(2) == 0
		case 3:
			return float64(next(40)) - 8
		case 4:
			return []string{"", "1", "auto", "<g id=\"a\"/>", "</defs><defs>", "constructor", "x", "none"}[next(8)]
		case 5:
			arr := make([]any, next(3))
			for i := range arr {
				arr[i] = value(depth + 1)
			}
			return arr
		}
		o := js.NewObj[any]()
		if depth > 3 {
			return o
		}
		for _, key := range []string{"icons", "aliases", "x", "y", "parent", "body", "width", "height", "left", "top", "rotate", "hFlip", "vFlip", "constructor"} {
			if next(3) == 0 {
				o.Set(key, value(depth+1))
			}
		}
		return o
	}
	names := []string{"p:x", "p:y", "p:constructor", "p:toString", "x", "p-x", "@a:p:x", "p:", ""}
	sizes := []any{nil, js.Null, 48.0, "auto", "2em", "none", true, []any{1.0}}
	for i := 0; i < 3000; i++ {
		ctx := render.NewContext(render.ContextOptions{Config: js.NewObj[any]()})
		set, _ := value(0).(*js.Obj[any])
		if set == nil {
			set = js.NewObj[any]()
		}
		if err := RegisterIconPacks(ctx, IconLoader{Name: "p", Icons: set}); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			IsIconAvailable(ctx, name)
			customisations := map[string]any{"width": sizes[next(len(sizes))], "height": sizes[next(len(sizes))], "rotate": value(4), "hFlip": value(4), "fallbackPrefix": "p"}
			if svg, err := GetIconSVG(ctx, name, customisations, nil); err == nil && svg == "" {
				t.Errorf("empty markup for %q", name)
			}
		}
	}
}
