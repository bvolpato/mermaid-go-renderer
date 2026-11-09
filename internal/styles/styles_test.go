package styles

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/stylis"
)

// The golden file comes from tools/reference/gen-styles.mjs. It runs the
// built mermaid 12.2.0 in node: processAndSetConfigs and getConfig for the
// config, getStyles for the raw text, createUserStyles for the compiled text.
// The script loads stylis and es-toolkit at the versions that the mermaid
// bundle contains, which are older than the versions in node_modules.
type goldenFile struct {
	// Stylis and EsToolkit are the versions that made the expected values.
	// NewerEsToolkit is the version of es-toolkit in node_modules.
	Stylis         string `json:"stylis"`
	EsToolkit      string `json:"esToolkit"`
	NewerEsToolkit string `json:"newerEsToolkit"`
	SvgID          string `json:"svgId"`
	// Values holds each distinct top-level config value one time.
	Values []json.RawMessage `json:"values"`
	// Configs are lists of [key, index into Values].
	Configs  [][][2]json.RawMessage `json:"configs"`
	Outputs  []goldenOutput         `json:"outputs"`
	Fixtures []goldenFixture        `json:"fixtures"`
	Extra    []goldenExtra          `json:"extra"`
	Direct   []goldenOutput         `json:"direct"`
	Merge    []goldenMerge          `json:"merge"`
}

// goldenMerge is one call of merge({}, a, b) of es-toolkit. Newer is the
// result of the es-toolkit in node_modules. It is set only when that version
// gives another result than the bundled version.
type goldenMerge struct {
	A     json.RawMessage `json:"a"`
	B     json.RawMessage `json:"b"`
	Out   json.RawMessage `json:"out"`
	Newer json.RawMessage `json:"newer"`
}

// goldenOutput is the style text of one diagram type with one config.
type goldenOutput struct {
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Config    int             `json:"config"`
	Options   json.RawMessage `json:"options"`
	SvgID     *string         `json:"svgId"`
	User      string          `json:"user"`
	Raw       *string         `json:"raw"`
	CSSHash   string          `json:"cssHash"`
	CSSLength int             `json:"cssLength"`
	Error     *string         `json:"error"`
}

type goldenFixture struct {
	Path   string  `json:"path"`
	Output *int    `json:"output"`
	Error  *string `json:"error"`
}

type goldenExtra struct {
	Name   string `json:"name"`
	Output int    `json:"output"`
}

type golden struct {
	file    goldenFile
	values  []any
	configs []*js.Obj[any]
}

var goldenCache *golden

func loadGolden(t testing.TB) *golden {
	t.Helper()
	if goldenCache != nil {
		return goldenCache
	}
	f, err := os.Open("testdata/styles.json.gz")
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
	g := &golden{}
	if err := json.Unmarshal(data, &g.file); err != nil {
		t.Fatal(err)
	}
	for i, raw := range g.file.Values {
		g.values = append(g.values, parseValue(t, raw, fmt.Sprintf("value %d", i)))
	}
	goldenCache = g
	return g
}

func parseValue(t testing.TB, raw json.RawMessage, what string) any {
	t.Helper()
	v, err := js.ParseJSON(raw)
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	return decodeTagged(v)
}

// config builds a fresh config object, so that a test cannot change the
// config of another test.
func (g *golden) config(t testing.TB, index int) *js.Obj[any] {
	t.Helper()
	cfg := js.NewObj[any]()
	for _, entry := range g.file.Configs[index] {
		var key string
		var value int
		if err := json.Unmarshal(entry[0], &key); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(entry[1], &value); err != nil {
			t.Fatal(err)
		}
		cfg.Set(key, js.Clone(g.values[value]))
	}
	return cfg
}

func sha(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// firstDiff describes where two strings start to differ.
func firstDiff(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	from := max(i-60, 0)
	clip := func(s string) string {
		return fmt.Sprintf("%q", s[min(from, len(s)):min(i+100, len(s))])
	}
	return fmt.Sprintf("differ at byte %d (lengths %d and %d)\n got  %s\n want %s", i, len(got), len(want), clip(got), clip(want))
}

// checkOutput runs the port for one golden output and returns a description
// of the first difference, or "".
func checkOutput(t testing.TB, g *golden, out goldenOutput) string {
	cfg := g.config(t, out.Config)
	svgID := g.file.SvgID

	if out.Error != nil {
		if _, err := Build(out.Type, "", cfg, svgID); err == nil {
			return fmt.Sprintf("Build gave no error, mermaid throws %q", *out.Error)
		}
		return ""
	}

	user := CreateCssStyles(cfg, nil, nil)
	if user != out.User {
		return "createCssStyles: " + firstDiff(user, out.User)
	}

	opts := js.NewObj[any]()
	obj(cfg.Get("themeVariables")).Each(func(key string, v any) { opts.Set(key, v) })
	opts.Set("theme", cfg.Get("theme"))
	opts.Set("look", cfg.Get("look"))
	before := js.StringifyJSON(opts)
	raw, err := GetStyles(out.Type, user, opts, svgID, cfg)
	if err != nil {
		return fmt.Sprintf("GetStyles: error %v", err)
	}
	if raw != *out.Raw {
		return "getStyles: " + firstDiff(raw, *out.Raw)
	}
	if after := js.StringifyJSON(opts); after != before {
		return "GetStyles changed its options argument"
	}

	css, err := Build(out.Type, "", cfg, svgID)
	if err != nil {
		return fmt.Sprintf("Build: error %v", err)
	}
	if len(css) != out.CSSLength && isASCII(css) {
		return fmt.Sprintf("Build: %d bytes, want %d", len(css), out.CSSLength)
	}
	if sha(css) != out.CSSHash {
		return "Build: the compiled text has another SHA-256 than createUserStyles gives"
	}
	return ""
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

type tally struct{ total, ok int }

func logTallies(t *testing.T, title string, tallies map[string]*tally) {
	t.Helper()
	names := make([]string, 0, len(tallies))
	for name := range tallies {
		names = append(names, name)
	}
	sort.Strings(names)
	var sb strings.Builder
	total := tally{}
	for _, name := range names {
		c := tallies[name]
		fmt.Fprintf(&sb, "  %-16s %5d / %5d\n", name, c.ok, c.total)
		total.total += c.total
		total.ok += c.ok
	}
	t.Logf("%s\n%s  %-16s %5d / %5d", title, sb.String(), "total", total.ok, total.total)
}

// TestCorpusNode is comparison (a): for every fixture of the corpus, Build
// gives the text that createUserStyles gives in node for the same config.
func TestCorpusNode(t *testing.T) {
	g := loadGolden(t)
	if len(g.file.Fixtures) != 2145 {
		t.Fatalf("the golden file has %d fixtures, want 2145", len(g.file.Fixtures))
	}
	results := make(map[int]string)
	tallies := map[string]*tally{}
	reported := 0
	for _, fx := range g.file.Fixtures {
		if fx.Output == nil {
			// detectType finds no diagram type, so mermaid makes no style sheet.
			c := tallies["(no type)"]
			if c == nil {
				c = &tally{}
				tallies["(no type)"] = c
			}
			c.total++
			c.ok++
			continue
		}
		out := g.file.Outputs[*fx.Output]
		diff, done := results[*fx.Output]
		if !done {
			diff = checkOutput(t, g, out)
			results[*fx.Output] = diff
		}
		c := tallies[out.Type]
		if c == nil {
			c = &tally{}
			tallies[out.Type] = c
		}
		c.total++
		if diff == "" {
			c.ok++
		} else if !done {
			if reported < 10 {
				t.Errorf("%s (%s): %s", fx.Path, out.Type, diff)
			}
			reported++
		}
	}
	logTallies(t, "corpus fixtures that match the node output, by diagram type:", tallies)
	for name, c := range tallies {
		if c.ok != c.total {
			t.Errorf("%s: %d of %d fixtures match", name, c.ok, c.total)
		}
	}
}

// TestExtraNode covers the cases outside the corpus: every type with every
// theme and look, and the configs that reach the rare branches.
func TestExtraNode(t *testing.T) {
	g := loadGolden(t)
	if len(g.file.Extra) < 1500 {
		t.Fatalf("the golden file has %d extra cases, want at least 1500", len(g.file.Extra))
	}
	results := make(map[int]string)
	failed, throws := 0, 0
	for _, ex := range g.file.Extra {
		out := g.file.Outputs[ex.Output]
		diff, done := results[ex.Output]
		if !done {
			diff = checkOutput(t, g, out)
			results[ex.Output] = diff
		}
		if out.Error != nil {
			throws++
		}
		if diff != "" {
			failed++
			if failed <= 10 {
				t.Errorf("%s: %s", ex.Name, diff)
			}
		}
	}
	t.Logf("%d extra cases (%d where mermaid throws), %d failed", len(g.file.Extra), throws, failed)
	if failed > 0 {
		t.Errorf("%d of %d extra cases failed", failed, len(g.file.Extra))
	}
}

// TestDirectGetStyles calls GetStyles with options that the config code of
// mermaid would never make: missing variables, values of the wrong type and
// other svg ids.
func TestDirectGetStyles(t *testing.T) {
	g := loadGolden(t)
	if len(g.file.Direct) < 500 {
		t.Fatalf("the golden file has %d direct cases, want at least 500", len(g.file.Direct))
	}
	failed, throws := 0, 0
	for _, out := range g.file.Direct {
		cfg := g.config(t, out.Config)
		opts := js.AsObj(parseValue(t, out.Options, out.Name))
		raw, err := GetStyles(out.Type, out.User, opts, *out.SvgID, cfg)
		problem := ""
		switch {
		case out.Error != nil:
			throws++
			if err == nil {
				problem = fmt.Sprintf("no error, mermaid throws %q", *out.Error)
			}
		case err != nil:
			problem = fmt.Sprintf("error %v", err)
		case raw != *out.Raw:
			problem = firstDiff(raw, *out.Raw)
		default:
			if css := stylis.CompileCSS(*out.SvgID, raw); sha(css) != out.CSSHash {
				problem = "the compiled text has another SHA-256 than compileCSS gives"
			}
		}
		if problem != "" {
			failed++
			if failed <= 10 {
				t.Errorf("%s: %s", out.Name, problem)
			}
		}
	}
	t.Logf("%d direct cases (%d where mermaid throws), %d failed", len(g.file.Direct), throws, failed)
	if failed > 0 {
		t.Errorf("%d of %d direct cases failed", failed, len(g.file.Direct))
	}
}

// sameValue reports whether two dynamic values are equal, with the same
// property order.
func sameValue(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case float64:
		y, ok := b.(float64)
		return ok && (x == y || (x != x && y != y))
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !sameValue(x[i], y[i]) {
				return false
			}
		}
		return true
	case *js.Obj[any]:
		y, ok := b.(*js.Obj[any])
		if !ok || strings.Join(x.Keys(), "\x00") != strings.Join(y.Keys(), "\x00") {
			return false
		}
		for _, key := range x.Keys() {
			if !sameValue(x.Get(key), y.Get(key)) {
				return false
			}
		}
		return true
	}
	return a == b
}

// TestCleanAndMerge compares cleanAndMerge with merge({}, a, b) of es-toolkit.
func TestCleanAndMerge(t *testing.T) {
	g := loadGolden(t)
	if len(g.file.Merge) < 500 {
		t.Fatalf("the golden file has %d merge cases, want at least 500", len(g.file.Merge))
	}
	// The mermaid 12.2.0 bundle contains es-toolkit 1.45.1 and stylis 4.3.6.
	if g.file.EsToolkit != "1.45.1" || g.file.Stylis != "4.3.6" {
		t.Fatalf("the golden file comes from es-toolkit %s and stylis %s, want 1.45.1 and 4.3.6, the versions in the mermaid bundle", g.file.EsToolkit, g.file.Stylis)
	}
	failed := 0
	// versionCases counts the cases that tell the bundled es-toolkit from the
	// newer one. wrongVersion counts those where the port gives the newer result.
	versionCases, wrongVersion := 0, 0
	for i, c := range g.file.Merge {
		a := parseValue(t, c.A, "merge a")
		b := parseValue(t, c.B, "merge b")
		want := parseValue(t, c.Out, "merge out")
		before := js.StringifyJSON(a) + js.StringifyJSON(b)
		got := cleanAndMerge(a, b)
		if !sameValue(got, want) {
			failed++
			if failed <= 10 {
				t.Errorf("case %d: merge({}, %s, %s)\n got  %s\n want %s", i, c.A, c.B, js.StringifyJSON(got), c.Out)
			}
		}
		if js.StringifyJSON(a)+js.StringifyJSON(b) != before {
			t.Errorf("case %d: cleanAndMerge changed its arguments", i)
		}
		if c.Newer != nil {
			versionCases++
			if sameValue(got, parseValue(t, c.Newer, "merge newer")) {
				wrongVersion++
			}
		}
	}
	if g.file.NewerEsToolkit != g.file.EsToolkit && versionCases < 100 {
		t.Errorf("%d cases tell es-toolkit %s from es-toolkit %s, want at least 100", versionCases, g.file.EsToolkit, g.file.NewerEsToolkit)
	}
	if wrongVersion > 0 {
		t.Errorf("%d cases have the result of es-toolkit %s: the port must follow es-toolkit %s", wrongVersion, g.file.NewerEsToolkit, g.file.EsToolkit)
	}
	if failed > 0 {
		t.Errorf("%d of %d merge cases failed", failed, len(g.file.Merge))
	}
}

// The port follows the es-toolkit of the mermaid bundle (1.45.1) and not the
// newer es-toolkit of node_modules (1.53.1). 1.45.1 reads a null element of a
// source array as undefined, so the element does not replace a value of the
// target. The expected values are those of the golden file.
func TestCleanAndMergeBundledVersion(t *testing.T) {
	cases := []struct{ a, b, want string }{
		// 1.53.1 gives {"a":[null,3]}.
		{`{"a":[1,2]}`, `{"a":[null,3]}`, `{"a":[1,3]}`},
		// 1.53.1 gives {"a":[null]}. JSON has no undefined: the tagged form is
		// the one of the golden file.
		{`{}`, `{"a":[null]}`, `{"a":[{"\u0001js":"undefined"}]}`},
		// A null that is not in an array replaces the value in both versions.
		{`{"a":[1]}`, `{"a":null}`, `{"a":null}`},
	}
	for _, c := range cases {
		a := parseValue(t, json.RawMessage(c.a), "a")
		b := parseValue(t, json.RawMessage(c.b), "b")
		want := parseValue(t, json.RawMessage(c.want), "want")
		if got := cleanAndMerge(a, b); !sameValue(got, want) {
			t.Errorf("merge({}, %s, %s): got %s, want %s", c.a, c.b, js.StringifyJSON(got), c.want)
		}
	}
}

// TestConcurrentBuild runs Build from many goroutines. Run it with -race. The
// style functions share the theme variables of the default theme, and they
// must only read them.
func TestConcurrentBuild(t *testing.T) {
	g := loadGolden(t)
	type job struct {
		typ  string
		cfg  int
		hash string
	}
	var jobs []job
	seen := map[string]bool{}
	for _, out := range g.file.Outputs {
		if out.Error != nil || seen[out.Type] {
			continue
		}
		seen[out.Type] = true
		jobs = append(jobs, job{out.Type, out.Config, out.CSSHash})
	}
	configs := make([]*js.Obj[any], len(jobs))
	for i, j := range jobs {
		configs[i] = g.config(t, j.cfg)
	}
	errs := make(chan string, 4*len(jobs))
	done := make(chan struct{})
	for worker := 0; worker < 4; worker++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i, j := range jobs {
				// Each goroutine reads the same config objects.
				css, err := Build(j.typ, "", configs[i], g.file.SvgID)
				if err != nil || sha(css) != j.hash {
					errs <- fmt.Sprintf("%s: err=%v", j.typ, err)
				}
			}
		}()
	}
	for worker := 0; worker < 4; worker++ {
		<-done
	}
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func BenchmarkBuild(b *testing.B) {
	g := loadGolden(b)
	for _, typ := range []string{"flowchart-v2", "sequence", "mindmap"} {
		var cfg *js.Obj[any]
		for _, out := range g.file.Outputs {
			if out.Type == typ && out.Error == nil {
				cfg = g.config(b, out.Config)
				break
			}
		}
		b.Run(typ, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := Build(typ, "", cfg, "#my-svg"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
