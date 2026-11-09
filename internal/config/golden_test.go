package config

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden files hold results of the real mermaid 12.2.0 in V8. They are
// written by tools/reference/gen-config.mjs.

func readGolden(t *testing.T, name string, into any) {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
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
	if err := json.Unmarshal(data, into); err != nil {
		t.Fatal(err)
	}
}

// decode reads a value in the encoding of the generators: {"$u":1} is
// undefined and {"$n":"NaN"} is a number that JSON cannot hold.
func decode(t *testing.T, raw json.RawMessage) any {
	t.Helper()
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatalf("%v in %.200s", err, raw)
	}
	return revive(v)
}

func revive(v any) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = revive(x[i])
		}
	case *js.Obj[any]:
		if x.Len() == 1 {
			if x.Has("$u") {
				return nil
			}
			if s, ok := x.Get("$n").(string); ok {
				switch s {
				case "NaN":
					return math.NaN()
				case "Infinity":
					return math.Inf(1)
				case "-Infinity":
					return math.Inf(-1)
				case "-0":
					return math.Copysign(0, -1)
				}
			}
		}
		for _, k := range x.Keys() {
			x.Set(k, revive(x.Get(k)))
		}
	}
	return v
}

// diff returns the path of the first difference between two dynamic values,
// or "". Objects must have the same keys in the same order, strings must be
// equal byte for byte, and numbers must have the same bits.
func diff(path string, got, want any) string {
	switch w := want.(type) {
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok || g == nil {
			return fmt.Sprintf("%s: got %s, want an object", path, show(got))
		}
		gk, wk := g.Keys(), w.Keys()
		for i := 0; i < len(gk) || i < len(wk); i++ {
			switch {
			case i >= len(gk):
				return fmt.Sprintf("%s: key %d: missing %q", path, i, wk[i])
			case i >= len(wk):
				return fmt.Sprintf("%s: key %d: extra %q", path, i, gk[i])
			case gk[i] != wk[i]:
				return fmt.Sprintf("%s: key %d: got %q, want %q", path, i, gk[i], wk[i])
			}
			if d := diff(path+"."+gk[i], g.Get(gk[i]), w.Get(wk[i])); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		for i := range w {
			if d := diff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		return ""
	case float64:
		g, ok := got.(float64)
		if !ok || !(math.Float64bits(g) == math.Float64bits(w) || (math.IsNaN(g) && math.IsNaN(w))) {
			return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
		}
		return ""
	}
	if js.IsObject(got) || !js.StrictEquals(got, want) {
		return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
	}
	return ""
}

func show(v any) string {
	if v == nil {
		return "undefined"
	}
	if f, ok := v.(float64); ok {
		return js.Str(f)
	}
	s := fmt.Sprintf("%T %s", v, js.StringifyJSON(v))
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// pool is the value pool of a golden file: each distinct value is stored
// once, and a config is a list of [key, pool index] pairs.
type pool struct {
	t      *testing.T
	raw    []json.RawMessage
	mu     sync.Mutex
	values map[int]any
}

func newPool(t *testing.T, raw []json.RawMessage) *pool {
	return &pool{t: t, raw: raw, values: map[int]any{}}
}

// value returns pool entry i. The decoded value is shared: do not modify it.
func (p *pool) value(i int) any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, ok := p.values[i]; ok {
		return v
	}
	v := decode(p.t, p.raw[i])
	p.values[i] = v
	return v
}

// forEachCase calls check for every case on several goroutines, so that the
// golden tests also exercise concurrent calls of Resolve. It returns the
// problems in case order.
func forEachCase(n int, check func(i int) string) []string {
	problems := make([]string, n)
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				problems[i] = check(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		next <- i
	}
	close(next)
	wg.Wait()
	return problems
}

// report prints the first problems and returns the number of failed cases.
func report(t *testing.T, problems []string, name func(i int) string) int {
	t.Helper()
	failed := 0
	for i, problem := range problems {
		if problem == "" {
			continue
		}
		failed++
		if failed <= 15 {
			t.Errorf("%s:\n  %s", name(i), problem)
		}
	}
	return failed
}

// object builds the object that pool entry i describes.
func (p *pool) object(i int) *js.Obj[any] {
	p.t.Helper()
	var pairs [][]json.RawMessage
	if err := json.Unmarshal(p.raw[i], &pairs); err != nil {
		p.t.Fatal(err)
	}
	obj := js.NewObj[any]()
	for _, pair := range pairs {
		var key string
		var index int
		if err := json.Unmarshal(pair[0], &key); err != nil {
			p.t.Fatal(err)
		}
		if err := json.Unmarshal(pair[1], &index); err != nil {
			p.t.Fatal(err)
		}
		obj.Set(key, p.value(index))
	}
	return obj
}

// goldenError is an exception that upstream throws.
type goldenError struct {
	Name    string
	Message string
}

// goldenCase is the record that run() in gen-config.mjs writes.
type goldenCase struct {
	File                  string
	Text                  *string
	Site                  *int
	InitError             *goldenError
	Error                 *goldenError
	Cleaned               string
	WithComments          string
	FrontmatterLineOffset int
	Title                 string
	DisplayMode           string
	DiagramType           string
	Directive             int
	Config                int
}

// checkCase runs Resolve and compares everything with one golden case. It
// returns a description of the first difference, or "".
func checkCase(p *pool, c *goldenCase, text string, site *js.Obj[any]) string {
	var before string
	if site != nil {
		before = js.StringifyJSON(site)
	}
	got, err := Resolve(text, site)
	if site != nil && js.StringifyJSON(site) != before {
		return "Resolve modified the site config"
	}
	want := c.Error
	if want == nil {
		want = c.InitError
	}
	if want != nil {
		if err == nil {
			return fmt.Sprintf("no error, want %s: %s", want.Name, want.Message)
		}
		e, ok := err.(*Error)
		if !ok {
			return fmt.Sprintf("error %v is not a *config.Error", err)
		}
		if e.Name != want.Name {
			return fmt.Sprintf("error %v, want %s: %s", err, want.Name, want.Message)
		}
		// The text of a TypeError depends on the V8 call site, and the text of
		// a YAMLException on the parser. Other messages must be equal.
		if e.Name != "TypeError" && e.Name != "YAMLException" && e.Message != want.Message {
			return fmt.Sprintf("error message %q, want %q", e.Message, want.Message)
		}
		return ""
	}
	if err != nil {
		return fmt.Sprintf("unexpected error %v", err)
	}
	switch {
	case got.Code != c.Cleaned:
		return fmt.Sprintf("code.cleaned: got %q, want %q", got.Code, c.Cleaned)
	case got.Source.Cleaned != got.Code:
		return "Source.Cleaned differs from Code"
	case got.Source.Raw != text:
		return "Source.Raw differs from the input"
	case got.Source.WithComments != c.WithComments:
		return fmt.Sprintf("code.withComments: got %q, want %q", got.Source.WithComments, c.WithComments)
	case got.Source.FrontmatterLineOffset != c.FrontmatterLineOffset:
		return fmt.Sprintf("code.frontmatterLineOffset: got %d, want %d", got.Source.FrontmatterLineOffset, c.FrontmatterLineOffset)
	case got.Title != c.Title:
		return fmt.Sprintf("title: got %q, want %q", got.Title, c.Title)
	case got.DisplayMode != c.DisplayMode:
		return fmt.Sprintf("displayMode: got %q, want %q", got.DisplayMode, c.DisplayMode)
	case got.DiagramType != c.DiagramType:
		return fmt.Sprintf("diagramType: got %q, want %q", got.DiagramType, c.DiagramType)
	}
	if d := diff("directive", got.Directive, p.value(c.Directive)); d != "" {
		return d
	}
	return diff("config", got.Config, p.object(c.Config))
}

// TestCorpus is golden set 1: every fixture of the mermaid e2e corpus, after
// mermaid.initialize({ startOnLoad: false }).
func TestCorpus(t *testing.T) {
	var g struct {
		Site  json.RawMessage
		Pool  []json.RawMessage
		Cases []goldenCase
	}
	readGolden(t, "corpus.json.gz", &g)
	p := newPool(t, g.Pool)
	root := filepath.Join("..", "..", "testdata", "corpus", "mermaid-e2e")
	problems := forEachCase(len(g.Cases), func(i int) string {
		c := &g.Cases[i]
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.File)))
		if err != nil {
			return err.Error()
		}
		site := decode(t, g.Site).(*js.Obj[any])
		return checkCase(p, c, string(data), site)
	})
	failed := report(t, problems, func(i int) string { return g.Cases[i].File })
	t.Logf("%d golden cases, %d failed", len(g.Cases), failed)
	if len(g.Cases) < 2000 {
		t.Errorf("the corpus golden file has only %d cases", len(g.Cases))
	}
}

// TestDefaults compares the default config, configKeys and the detector order
// with the real mermaid.
func TestDefaults(t *testing.T) {
	var g struct {
		DefaultConfig json.RawMessage
		ConfigKeys    []string
		NotConfigKeys []string
		Detectors     []string
	}
	readGolden(t, "defaults.json.gz", &g)
	if d := diff("defaultConfig", Default(), decode(t, g.DefaultConfig)); d != "" {
		t.Error(d)
	}
	for _, k := range g.ConfigKeys {
		if !IsConfigKey(k) {
			t.Errorf("IsConfigKey(%q) is false, want true", k)
		}
	}
	for _, k := range g.NotConfigKeys {
		if IsConfigKey(k) {
			t.Errorf("IsConfigKey(%q) is true, want false", k)
		}
	}
	if len(configKeySet) != len(g.ConfigKeys) {
		t.Errorf("configKeys has %d keys, want %d", len(configKeySet), len(g.ConfigKeys))
	}
	if got := fmt.Sprint(DetectorIDs()); got != fmt.Sprint(g.Detectors) {
		t.Errorf("detectors:\n  got  %s\n  want %s", got, fmt.Sprint(g.Detectors))
	}
}

// TestSiteConfigs is golden set 4: site configs passed to mermaid.initialize,
// crossed with a sample of the corpus.
func TestSiteConfigs(t *testing.T) {
	var g struct {
		Sites []json.RawMessage
		Pool  []json.RawMessage
		Cases []goldenCase
	}
	readGolden(t, "site.json.gz", &g)
	p := newPool(t, g.Pool)
	root := filepath.Join("..", "..", "testdata", "corpus", "mermaid-e2e")
	texts := map[string]string{}
	threw := 0
	for i := range g.Cases {
		c := &g.Cases[i]
		if _, ok := texts[c.File]; !ok {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(c.File)))
			if err != nil {
				t.Fatal(err)
			}
			texts[c.File] = string(data)
		}
		if c.Error != nil || c.InitError != nil {
			threw++
		}
	}
	problems := forEachCase(len(g.Cases), func(i int) string {
		c := &g.Cases[i]
		site := decode(t, g.Sites[*c.Site]).(*js.Obj[any])
		return checkCase(p, c, texts[c.File], site)
	})
	failed := report(t, problems, func(i int) string {
		return fmt.Sprintf("site %s with %s", g.Sites[*g.Cases[i].Site], g.Cases[i].File)
	})
	t.Logf("%d golden cases (%d sites, %d fixtures, %d cases throw upstream), %d failed", len(g.Cases), len(g.Sites), len(texts), threw, failed)
}

// TestTextCases covers the preprocessing steps with hand-written and random
// texts: front matter, directives, comments, sanitization, type detection,
// and the inputs for which upstream throws.
func TestTextCases(t *testing.T) {
	var g struct {
		Sites []json.RawMessage
		Pool  []json.RawMessage
		Cases []goldenCase
	}
	readGolden(t, "text.json.gz", &g)
	p := newPool(t, g.Pool)
	threw := map[string]int{}
	for i := range g.Cases {
		if e := g.Cases[i].Error; e != nil {
			threw[e.Name]++
		}
	}
	problems := forEachCase(len(g.Cases), func(i int) string {
		c := &g.Cases[i]
		// The site is undefined for mermaid.initialize() with no argument.
		site, _ := decode(t, g.Sites[*c.Site]).(*js.Obj[any])
		return checkCase(p, c, *c.Text, site)
	})
	failed := report(t, problems, func(i int) string {
		return fmt.Sprintf("case %d, site %s, text %q", i, g.Sites[*g.Cases[i].Site], *g.Cases[i].Text)
	})
	t.Logf("%d golden cases (upstream throws: %v), %d failed", len(g.Cases), threw, failed)
}
