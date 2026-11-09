package config

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/memo"
)

// graph prints dynamic values with the identity of their objects and
// arrays: the first time an object appears it gets a number, and a later
// appearance prints only the number. Two lists of values give the same text
// when they have the same content, the same key order, and share objects in
// the same places.
type graph struct {
	sb   strings.Builder
	objs map[*js.Obj[any]]int
	arrs map[any]int
}

func graphOf(values ...any) string {
	g := &graph{objs: map[*js.Obj[any]]int{}, arrs: map[any]int{}}
	for i, v := range values {
		fmt.Fprintf(&g.sb, "\n[%d] ", i)
		g.value(v)
	}
	return g.sb.String()
}

func (g *graph) value(v any) {
	switch x := v.(type) {
	case nil:
		g.sb.WriteString("undefined")
	case *js.Obj[any]:
		if x == nil {
			g.sb.WriteString("nil-object")
			return
		}
		if id, ok := g.objs[x]; ok {
			fmt.Fprintf(&g.sb, "@%d", id)
			return
		}
		id := len(g.objs)
		g.objs[x] = id
		fmt.Fprintf(&g.sb, "#%d{", id)
		for _, k := range x.Keys() {
			fmt.Fprintf(&g.sb, "%q:", k)
			g.value(x.Get(k))
			g.sb.WriteByte(',')
		}
		g.sb.WriteByte('}')
	case []any:
		if key, ok := js.Identity(x); ok {
			if id, seen := g.arrs[key]; seen {
				fmt.Fprintf(&g.sb, "@a%d", id)
				return
			}
			g.arrs[key] = len(g.arrs)
			fmt.Fprintf(&g.sb, "#a%d", g.arrs[key])
		}
		g.sb.WriteByte('[')
		for _, e := range x {
			g.value(e)
			g.sb.WriteByte(',')
		}
		g.sb.WriteByte(']')
	case float64:
		fmt.Fprintf(&g.sb, "num:%x", math.Float64bits(x))
	case string:
		fmt.Fprintf(&g.sb, "%q", x)
	default:
		fmt.Fprintf(&g.sb, "%T:%v", v, v)
	}
}

// firstDiff shows where two texts start to differ.
func firstDiff(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	clip := func(s string) string {
		start := max(i-80, 0)
		return s[start:min(i+80, len(s))]
	}
	return fmt.Sprintf("at byte %d:\n  got  ...%s...\n  want ...%s...", i, clip(got), clip(want))
}

// randomConfigValue makes a dynamic value with the cases that
// assignWithDepth has branches for.
func randomConfigValue(rng *rand.Rand, depth int, shared []any) any {
	switch n := rng.Intn(20); {
	case n < 5 && depth > 0:
		o := js.NewObj[any]()
		keys := []string{"a", "b", "c", "theme", "0", "7", "x y", "__proto__", "length", "flowchart", "zz"}
		for i, count := 0, rng.Intn(7); i < count; i++ {
			o.Set(keys[rng.Intn(len(keys))], randomConfigValue(rng, depth-1, shared))
		}
		if rng.Intn(6) == 0 && o.Len() > 0 {
			o.Delete(o.Keys()[0])
		}
		return o
	case n < 7 && depth > 0:
		arr := []any{}
		for i, count := 0, rng.Intn(5); i < count; i++ {
			arr = append(arr, randomConfigValue(rng, depth-1, shared))
		}
		if len(arr) > 1 && rng.Intn(2) == 0 {
			arr = append(arr, arr[0], arr[0])
		}
		return arr
	case n < 8:
		return js.Null
	case n < 9:
		return nil
	case n < 10:
		return (*js.Obj[any])(nil)
	case n < 11:
		return math.NaN()
	case n < 12:
		return rng.Intn(2) == 0
	case n < 13 && len(shared) > 0:
		return shared[rng.Intn(len(shared))]
	case n < 14:
		return 3
	case n < 17:
		return float64(rng.Intn(5))
	}
	return []string{"x", "dark", "", "1"}[rng.Intn(4)]
}

// TestAssignNewAgainstPort compares js.AssignNew, the copy that assignToNew
// uses, with the port of assignWithDepth for a new destination: the same
// content, the same key order, and the same objects shared with the source.
func TestAssignNewAgainstPort(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fast, total := 0, 4000
	for i := 0; i < total; i++ {
		var shared []any
		for j := 0; j < 3; j++ {
			shared = append(shared, randomConfigValue(rng, 2, nil))
		}
		src, ok := randomConfigValue(rng, 5, shared).(*js.Obj[any])
		if !ok || src == nil {
			continue
		}
		if rng.Intn(3) == 0 {
			js.FreezeValue(src)
		}
		before := graphOf(src)
		for depth := 0; depth <= 3; depth++ {
			got, ok := js.AssignNew(src, depth)
			if !ok {
				continue
			}
			fast++
			want := AssignWithDepth(js.NewObj[any](), src, depth)
			// The source is in both lists, so the text shows which objects
			// the copy shares with it.
			if g, w := graphOf(src, got), graphOf(src, want); g != w {
				t.Fatalf("case %d, depth %d: AssignNew differs from the port %s", i, depth, firstDiff(g, w))
			}
			// The copy can change without a change of the source.
			got.Set("added", true)
			for _, k := range got.Keys() {
				if o, isObj := got.Get(k).(*js.Obj[any]); isObj && o != nil && depth > 0 {
					o.Set("added", true)
				}
			}
			if after := graphOf(src); after != before {
				t.Fatalf("case %d, depth %d: a change of the copy reached the source %s", i, depth, firstDiff(after, before))
			}
		}
	}
	if fast < total/4 {
		t.Errorf("AssignNew copied only %d trees", fast)
	}
	t.Logf("%d copies compared", fast)
}

// stateScript runs the calls that diagrams make on a config state, and a
// few more, and returns everything that they return, as a graph: the content
// and the objects that the results share.
func stateScript(r *Resolved) string {
	s := r.State
	out := []any{r.Config, r.Directive}
	// A section of the defaults first, then all of them: the section is the
	// same object in both.
	out = append(out, s.DefaultSection("c4"), s.DefaultSection("secure"), s.DefaultSection("theme"), s.DefaultSection("no such key"))
	out = append(out, s.GetConfig(), s.DefaultConfig(), s.GetUserDefinedConfig(), s.DefaultSection("flowchart"), s.DefaultSection("secure"))
	// A diagram changes a value below the levels that getConfig copies. The
	// state shares that object, as upstream does, so later copies have the
	// change.
	if deep := js.AsObj(js.Path(r.Config, "c4", "flowchart", "subGraphTitleMargin")); deep != nil {
		deep.Set("top", 42.0)
	}
	out = append(out, s.GetConfig(), s.DefaultConfig())
	// setConfig, as the init function of the flowchart calls it.
	conf := s.GetConfig()
	section := js.NewObj[any]()
	section.Set("arrowMarkerAbsolute", conf.Get("arrowMarkerAbsolute"))
	section.Set("curve", "basis")
	conf.Set("flowchart", section)
	conf.Set("theme", "forest")
	conf.Set("secure", []any{"x"})
	after, err := s.SetConfig(conf)
	out = append(out, after, fmt.Sprint(err), s.GetConfig(), s.GetUserDefinedConfig(), s.DefaultConfig())
	// The functions that no diagram calls.
	out = append(out, s.GetSiteConfig())
	update := js.NewObj[any]()
	update.Set("fontSize", 20.0)
	site, err := s.UpdateSiteConfig(update)
	out = append(out, site, fmt.Sprint(err), s.GetConfig())
	out = append(out, fmt.Sprint(s.Reset()), s.GetConfig(), s.DefaultConfig(), s.GetSiteConfig())
	return graphOf(out...)
}

// scriptInputs are site configs and diagram texts for the comparison of
// Resolve with the functions of State.
func scriptInputs() (sites []string, texts []string) {
	sites = []string{
		``,
		`{}`,
		`{"startOnLoad":false}`,
		`{"theme":"dark"}`,
		`{"theme":"base","themeVariables":{"primaryColor":"#ff0000"}}`,
		`{"theme":"nope","fontFamily":"courier"}`,
		`{"flowchart":{"theme":"forest","curve":"linear","subGraphTitleMargin":{"top":3}}}`,
		`{"secure":["secure","theme"],"theme":"neutral"}`,
		`{"themeVariables":{"xyChart":{"titleColor":"#123456"}},"look":"handDrawn"}`,
		// Configs that are not kept as templates: an object and an array
		// below the levels that a copy of the config makes new.
		`{"c4":{"flowchart":{"subGraphTitleMargin":{"top":1,"deep":{"x":1}}}}}`,
		`{"flowchart":{"padding":{"list":[1,2,{"a":1}]}}}`,
		`{"journey":{"sectionFills":["#000",["#111"]]}}`,
		`{"theme":["dark"]}`,
	}
	texts = []string{
		"flowchart TD\n  A --> B\n",
		"pie\n  \"a\": 1\n",
		"---\nconfig:\n  theme: forest\n---\nflowchart TD\n  A --> B\n",
		"---\ntitle: T\nconfig:\n  theme: dark\n  themeVariables:\n    primaryColor: '#00ff00'\n  flowchart:\n    curve: basis\n---\ngraph LR\n  A --> B\n",
		"%%{init: {\"theme\": \"base\", \"themeVariables\": {\"lineColor\": \"#f00\"}, \"sequence\": {\"mirrorActors\": false}}}%%\nsequenceDiagram\n  A->>B: hi\n",
		"---\nconfig:\n  look: handDrawn\n  layout: elk\n  flowchart:\n    look: neo\n---\nflowchart-elk TD\n  A --> B\n",
		"---\nconfig:\n  fontFamily: monospace\n  secure: [a]\n  __proto__: {x: 1}\n  themeCSS: '.x { fill: red }'\n---\nclassDiagram\n  class A\n",
		"---\nconfig:\n  c4:\n    flowchart:\n      subGraphTitleMargin:\n        top: 9\n---\nC4Context\n  Person(a, \"A\")\n",
		"not a diagram\n",
		"---\nconfig:\n  theme: 'null'\n---\ngantt\n  title G\n",
	}
	return sites, texts
}

func parseSite(t *testing.T, text string) *js.Obj[any] {
	t.Helper()
	if text == "" {
		return nil
	}
	v, err := js.ParseJSON([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return v.(*js.Obj[any])
}

// TestResolveAgainstState compares Resolve, which starts from a template,
// with the functions of State alone, for every pair of a site config and a
// text: the results, and what a script of State calls returns afterwards.
// Each pair runs two times, so that the second run takes the template from
// the cache.
func TestResolveAgainstState(t *testing.T) {
	memo.Purge()
	sites, texts := scriptInputs()
	for round := 0; round < 2; round++ {
		for _, siteText := range sites {
			for _, text := range texts {
				want, wantErr := resolveState(text, parseSite(t, siteText))
				got, gotErr := Resolve(text, parseSite(t, siteText))
				name := fmt.Sprintf("round %d, site %s, text %q", round, siteText, text)
				if fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
					t.Fatalf("%s: error %v, want %v", name, gotErr, wantErr)
				}
				if wantErr != nil {
					continue
				}
				if got.Code != want.Code || got.Source != want.Source || got.Title != want.Title ||
					got.DisplayMode != want.DisplayMode || got.DiagramType != want.DiagramType {
					t.Fatalf("%s: the text results differ: %+v, want %+v", name, got, want)
				}
				if g, w := stateScript(got), stateScript(want); g != w {
					t.Fatalf("%s: the state differs %s", name, firstDiff(g, w))
				}
			}
		}
	}
	if templates.Len() == 0 || initTemplates.Len() == 0 {
		t.Error("no template was cached")
	}
}

// TestTemplatesAreIsolated changes everything that a state from a template
// hands out, in the order that reaches the frozen objects in the most ways,
// and checks that a later Resolve gives the result of the functions of
// State.
func TestTemplatesAreIsolated(t *testing.T) {
	memo.Purge()
	sites, texts := scriptInputs()
	for _, siteText := range sites[:9] {
		for _, text := range texts[:8] {
			want, err := resolveState(text, parseSite(t, siteText))
			if err != nil {
				continue
			}
			wantGraph := graphOf(want.Config, want.Directive, want.State.DefaultConfig(), want.State.GetUserDefinedConfig())
			for order := 0; order < 3; order++ {
				r, err := Resolve(text, parseSite(t, siteText))
				if err != nil {
					t.Fatal(err)
				}
				s := r.State
				targets := []func() any{
					func() any { return s.DefaultConfig() },
					func() any { return r.Config },
					func() any { return r.Directive },
					func() any { return s.GetConfig() },
					func() any { return s.GetUserDefinedConfig() },
					func() any { return s.GetSiteConfig() },
				}
				for i := range targets {
					scribble(targets[(i+order*2)%len(targets)]())
				}
			}
			got, err := Resolve(text, parseSite(t, siteText))
			if err != nil {
				t.Fatal(err)
			}
			gotGraph := graphOf(got.Config, got.Directive, got.State.DefaultConfig(), got.State.GetUserDefinedConfig())
			if gotGraph != wantGraph {
				t.Fatalf("site %s, text %q: Resolve differs after results were changed %s", siteText, text, firstDiff(gotGraph, wantGraph))
			}
		}
	}
	if d := diff("default", Default(), mustDefaults(t)); d != "" {
		t.Errorf("the default config changed: %s", d)
	}
}

// mustDefaults reads the default config of upstream from the golden file.
func mustDefaults(t *testing.T) any {
	t.Helper()
	var g struct{ DefaultConfig json.RawMessage }
	readGolden(t, "defaults.json.gz", &g)
	return decode(t, g.DefaultConfig)
}

// TestResolveConcurrent runs Resolve for many inputs from many goroutines,
// while each goroutine changes what it got. Each result must be the result
// of the functions of State. The race detector finds a shared object that
// changes.
func TestResolveConcurrent(t *testing.T) {
	memo.Purge()
	sites, texts := scriptInputs()
	type input struct{ site, text, want string }
	var inputs []input
	for _, siteText := range sites {
		for _, text := range texts {
			want, err := resolveState(text, parseSite(t, siteText))
			if err != nil {
				continue
			}
			inputs = append(inputs, input{siteText, text, stateScript(want)})
		}
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(g)))
			for i := 0; i < 150; i++ {
				in := inputs[rng.Intn(len(inputs))]
				got, err := Resolve(in.text, parseSite(t, in.site))
				if err != nil {
					t.Errorf("site %s, text %q: %v", in.site, in.text, err)
					return
				}
				if script := stateScript(got); script != in.want {
					t.Errorf("site %s, text %q: the state differs %s", in.site, in.text, firstDiff(script, in.want))
					return
				}
				scribble(got.Config)
				scribble(got.State.GetConfig())
				scribble(got.State.DefaultConfig())
			}
		}(g)
	}
	wg.Wait()
}

// TestCacheKeyIsExact checks that values that differ in a way that a config
// can show have different cache keys, and that values that must not be
// templates have none.
func TestCacheKeyIsExact(t *testing.T) {
	obj := func(kv ...any) *js.Obj[any] {
		o := js.NewObj[any]()
		for i := 0; i+1 < len(kv); i += 2 {
			o.Set(kv[i].(string), kv[i+1])
		}
		return o
	}
	key := func(v any) (string, bool) {
		e := keyEncoder{ok: true}
		e.value(v, 1)
		return string(e.buf), e.ok
	}
	distinct := []any{
		obj(),
		obj("a", nil),
		obj("a", js.Null),
		obj("a", false),
		obj("a", 0.0),
		obj("a", math.Copysign(0, -1)),
		obj("a", 0),
		obj("a", ""),
		obj("a", "0"),
		obj("a", math.NaN()),
		obj("a", obj()),
		obj("a", []any{}),
		obj("a", (*js.Obj[any])(nil)),
		obj("a", 1.0, "b", 2.0),
		obj("b", 2.0, "a", 1.0),
		obj("ab", "c"),
		obj("a", "bc"),
		obj("a", obj("b", "c")),
		obj("a", "b", "c", nil),
		obj("a", []any{"b", "c"}),
		obj("a", []any{"bc"}),
		obj("a", []any{"b"}, "c", nil),
	}
	seen := map[string]int{}
	for i, v := range distinct {
		k, ok := key(v)
		if !ok {
			t.Errorf("value %d has no key: %s", i, js.StringifyJSON(v))
			continue
		}
		if j, dup := seen[k]; dup {
			t.Errorf("values %d and %d have the same key", j, i)
		}
		seen[k] = i
	}
	sharedObj := obj("x", 1.0)
	noKey := []any{
		obj("a", obj("b", obj("c", []any{}))),     // an array below the third level
		obj("a", []any{obj()}),                    // an object in an array
		obj("a", []any{[]any{}}),                  // an array in an array
		obj("a", sharedObj, "b", sharedObj),       // an object in two places
		obj("a", map[string]any{}),                // not a dynamic value
		obj("a", obj("b", []string{"x"})),         // not a dynamic value
		obj("a", obj("b", obj("c", obj("d", 1)))), // fine: checked below
	}
	for i, v := range noKey[:len(noKey)-1] {
		if _, ok := key(v); ok {
			t.Errorf("value %d must not have a key: %#v", i, v)
		}
	}
	if _, ok := key(noKey[len(noKey)-1]); !ok {
		t.Error("an object below the third level must have a key")
	}
}
