package block

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// The golden file testdata/db.json.gz is the block db of real mermaid:
// tools/reference/gen-block-db.mjs writes it with the browser oracle. The
// tests parse the same texts and compare everything, with the order of the
// list items and of the properties.
//
// The file is read with js.ParseJSON, which keeps the order of the
// properties. A case is an object with these properties:
//
//	name       the path of a corpus fixture, or the name of another case
//	text       the text, absent for a corpus fixture
//	fresh      true for a case that ran in a new page
//	site       the site config: the argument of mermaid.initialize
//	idCounter  the counter of generateId of blockDB.ts before the render
//	error      the message when mermaid rejects the text or fails to draw it
//	errorName  the name of the class of that exception
//	type       the diagram type
//	parsed     the db after the parse:
//	             blocks       db.getBlocks(), the hierarchy
//	             flat         the entries of the block database, in order
//	             treeRefs     for the blocks of the hierarchy in pre-order,
//	                          the index of the same object in flat, or -1
//	             columns      db.getColumns(key) for each entry of flat
//	             edges        db.getEdges()
//	             classes      the entries of db.getClasses()
//	             dbFunctions  the title and accessibility functions that
//	                          the db has (it has none)
//	             blockConfig  db.getConfig()
//	             configDelta  the config, as the difference from baseConfig
//	measured   the size of each entry of flat at the start of layout(): what
//	           calculateBlockSizes measured
//	drawn      sizes: the size of each entry of flat after the draw, and
//	           bounds: the box that layout() returns
//
// A number that JSON cannot hold is {"$n": "NaN"} (or Infinity, -Infinity,
// -0). The key of a block whose id is undefined is {"$undefined": true}.

func loadGoldenFile(tb testing.TB, name string) *js.Obj[any] {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		tb.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		tb.Fatal(err)
	}
	return js.AsObj(v)
}

func loadGolden(tb testing.TB) *js.Obj[any] {
	tb.Helper()
	return loadGoldenFile(tb, "db.json.gz")
}

func caseText(tb testing.TB, c *js.Obj[any]) string {
	tb.Helper()
	if text, ok := c.Get("text").(string); ok {
		return text
	}
	name, _ := c.Get("name").(string)
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(name)))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

// siteConfig is the argument of mermaid.initialize in the page of the
// oracle: { startOnLoad: false, ...site }.
func siteConfig(c *js.Obj[any]) *js.Obj[any] {
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	if extra := js.AsObj(c.Get("site")); extra != nil {
		extra.Each(func(k string, v any) { site.Set(k, v) })
	}
	return site
}

// parsed is the result of the steps of mermaid.render up to the draw
// function of the diagram.
type parsed struct {
	ctx *render.Context
	// typ is the diagram type.
	typ string
	// db is nil when the diagram type is not a block diagram.
	db *DB
}

const maxTextExceeded = "graph TB;a[Maximum text size in diagram exceeded];style a fill:#faa"

// parseText runs the steps of render.Render before the draw function:
// mermaid.initialize(site), processAndSetConfigs, Diagram.fromText. warm
// says that the page has rendered a block diagram before, so the DOMPurify
// hooks of mermaid are installed. idCounter is the counter of generateId
// of blockDB.ts before the render.
func parseText(text string, site *js.Obj[any], warm bool, idCounter int) (*parsed, error) {
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, err
	}
	cfg := resolved.Config
	code := resolved.Source
	if v, ok := cfg.Get("maxTextSize").(float64); ok && float64(js.UTF16Len(code.Cleaned)) > v {
		code = config.DiagramCode{Raw: maxTextExceeded, Cleaned: maxTextExceeded}
	}
	// The page makes one draw from Math.random before the render.
	ctx := render.NewContext(render.ContextOptions{Config: cfg, ConfigState: resolved.State, RandomSkip: 1})
	ctx.Title = resolved.Title
	for i := 0; i < idCounter; i++ {
		ctx.Counter("block.generateId")
	}
	if warm {
		ctx.Sanitizer.RemoveScript("")
	}
	diagramType, err := config.DetectType(code.Cleaned, cfg)
	if err != nil {
		return nil, err
	}
	ctx.Type = diagramType
	p := &parsed{ctx: ctx, typ: diagramType}
	def, ok := render.Lookup(diagramType)
	if !ok {
		return p, nil
	}
	db := def.New(ctx)
	if def.Init != nil {
		def.Init(ctx, db)
		ctx.Config = ctx.ConfigState.GetConfig()
	}
	if resolved.Title != "" {
		if ts, ok := db.(render.TitleSetter); ok {
			ts.SetDiagramTitle(resolved.Title)
		}
	}
	if err := def.Parse(ctx, db, render.EncodeEntities(code.Cleaned)+"\n"); err != nil {
		return nil, err
	}
	p.db, _ = db.(*DB)
	return p, nil
}

// errorMessage is error.message of the JavaScript exception.
func errorMessage(err error) string {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Message
	}
	var be *jsError
	if errors.As(err, &be) {
		return be.Message
	}
	var je *jison.Error
	if errors.As(err, &je) {
		return je.Message
	}
	return err.Error()
}

// ---- conversion of the Go side to JavaScript values ----

// numJSON is a number as the replacer of the page script prints it.
func numJSON(v float64) any {
	special := ""
	switch {
	case math.IsNaN(v):
		special = "NaN"
	case math.IsInf(v, 1):
		special = "Infinity"
	case math.IsInf(v, -1):
		special = "-Infinity"
	case v == 0 && math.Signbit(v):
		special = "-0"
	default:
		return v
	}
	o := js.NewObj[any]()
	o.Set("$n", special)
	return o
}

func anyStrings(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func blockList(list []*Block) []any {
	out := make([]any, len(list))
	for i, b := range list {
		out[i] = blockJSON(b)
	}
	return out
}

// blockJSON is a block as JSON.stringify prints the JavaScript object: the
// properties in the order in which upstream adds them, without the
// properties that are undefined.
func blockJSON(b *Block) *js.Obj[any] {
	o := js.NewObj[any]()
	for _, name := range b.props {
		switch name {
		case "id":
			if !b.IDUndefined {
				o.Set(name, b.ID)
			}
		case "label":
			if b.HasLabel {
				o.Set(name, b.Label)
			}
		case "type":
			if b.Type != "" {
				o.Set(name, b.Type)
			}
		case "start":
			if b.Start != "" {
				o.Set(name, b.Start)
			}
		case "end":
			o.Set(name, b.End)
		case "thickness":
			o.Set(name, b.Thickness)
		case "pattern":
			o.Set(name, b.Pattern)
		case "arrowTypeEnd":
			o.Set(name, b.ArrowTypeEnd)
		case "arrowTypeStart":
			o.Set(name, b.ArrowTypeStart)
		case "directions":
			if b.Directions != nil {
				o.Set(name, anyStrings(b.Directions))
			}
		case "widthInColumns":
			if !isUndef(b.WidthInColumns) {
				o.Set(name, numJSON(b.WidthInColumns))
			}
		case "width":
			if !isUndef(b.Width) {
				o.Set(name, numJSON(b.Width))
			}
		case "columns":
			if !isUndef(b.Columns) {
				o.Set(name, numJSON(b.Columns))
			}
		case "colorIndex":
			if !isUndef(b.ColorIndex) {
				o.Set(name, numJSON(b.ColorIndex))
			}
		case "children":
			switch {
			case b.hasDocument:
				// The statements as the parser left them.
				document := make([]any, len(b.document))
				for i, e := range b.document {
					switch x := e.(type) {
					case *Block:
						document[i] = blockJSON(x)
					case []*Block:
						document[i] = blockList(x)
					}
				}
				o.Set(name, document)
			case b.Children != nil:
				o.Set(name, blockList(b.Children))
			}
		case "classes":
			if b.Classes != nil {
				o.Set(name, anyStrings(b.Classes))
			}
		case "styles":
			if b.Styles != nil {
				o.Set(name, anyStrings(b.Styles))
			}
		case "css":
			o.Set(name, b.CSS)
		case "styleClass":
			o.Set(name, b.StyleClass)
		case "stylesStr":
			o.Set(name, b.StylesStr)
		case "0", "1", "2":
			if i, _ := strconv.Atoi(name); i < len(b.spread) {
				o.Set(name, blockJSON(b.spread[i]))
			}
		}
	}
	return o
}

func keyJSON(k blockKey) any {
	if k.undefined {
		o := js.NewObj[any]()
		o.Set("$undefined", true)
		return o
	}
	return k.id
}

// treeRefs lists, for the blocks of the hierarchy in pre-order, the index
// of the same block in the flat list, or -1.
func treeRefs(blocks []*Block, flat []*Block, out []any) []any {
	for _, b := range blocks {
		index := -1
		for i, f := range flat {
			if f == b {
				index = i
				break
			}
		}
		out = append(out, float64(index))
		if b.Children != nil {
			out = treeRefs(b.Children, flat, out)
		}
	}
	return out
}

// dbFunctions lists the title and accessibility functions that the db
// has, in the order of the page script.
func dbFunctions(db any) []any {
	out := []any{}
	if _, ok := db.(interface{ GetAccTitle() string }); ok {
		out = append(out, "getAccTitle")
	}
	if _, ok := db.(interface{ GetAccDescription() string }); ok {
		out = append(out, "getAccDescription")
	}
	if _, ok := db.(interface{ GetDiagramTitle() string }); ok {
		out = append(out, "getDiagramTitle")
	}
	if _, ok := db.(interface{ SetDiagramTitle(string) }); ok {
		out = append(out, "setDiagramTitle")
	}
	if _, ok := db.(interface{ SetAccTitle(string) }); ok {
		out = append(out, "setAccTitle")
	}
	if _, ok := db.(interface{ SetAccDescription(string) }); ok {
		out = append(out, "setAccDescription")
	}
	return out
}

// snapshot is the state of the db, as the snapshot function of
// tools/reference/block-db-page.js builds it, without the config.
func snapshot(d *DB) *js.Obj[any] {
	keys := d.blockDatabase.Keys()
	values := d.blockDatabase.Values()
	flat := make([]any, len(keys))
	columns := make([]any, len(keys))
	for i, k := range keys {
		flat[i] = []any{keyJSON(k), blockJSON(values[i])}
		columns[i] = numJSON(d.getColumns(k))
	}
	classes := []any{}
	d.GetClasses().Each(func(id string, def *ClassDef) {
		class := js.NewObj[any]()
		class.Set("id", def.ID)
		class.Set("styles", anyStrings(def.Styles))
		class.Set("textStyles", anyStrings(def.TextStyles))
		classes = append(classes, []any{id, class})
	})
	var blockConfig any = js.Null
	if conf := d.GetConfig(); conf != nil {
		blockConfig = conf
	}
	o := js.NewObj[any]()
	o.Set("blocks", blockList(d.GetBlocks()))
	o.Set("flat", flat)
	o.Set("treeRefs", treeRefs(d.GetBlocks(), values, []any{}))
	o.Set("columns", columns)
	o.Set("edges", blockList(d.GetEdges()))
	o.Set("classes", classes)
	o.Set("dbFunctions", dbFunctions(d))
	o.Set("blockConfig", blockConfig)
	return o
}

// ---- comparison ----

// diffValues returns a description of the first difference between two
// JavaScript values, with the order of the properties, or "".
func diffValues(path string, got, want any) string {
	switch w := want.(type) {
	case *js.Obj[any]:
		g, ok := got.(*js.Obj[any])
		if !ok || g == nil {
			break
		}
		gk, wk := g.Keys(), w.Keys()
		for i := 0; i < len(gk) && i < len(wk); i++ {
			if gk[i] != wk[i] {
				return fmt.Sprintf("%s: property %d is %q, want %q (got %v, want %v)", path, i, gk[i], wk[i], gk, wk)
			}
		}
		if len(gk) != len(wk) {
			return fmt.Sprintf("%s: properties %v, want %v", path, gk, wk)
		}
		for _, k := range wk {
			if d := diffValues(path+"."+k, g.Get(k), w.Get(k)); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			break
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := diffValues(path+"["+strconv.Itoa(i)+"]", g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
		return ""
	}
	if gf, ok := got.(float64); ok {
		if wf, ok := want.(float64); ok && math.Float64bits(gf) == math.Float64bits(wf) {
			return ""
		}
	} else if reflect.DeepEqual(got, want) {
		return ""
	}
	return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
}

func show(v any) string {
	s := js.StringifyJSON(v)
	if v == nil {
		s = "undefined"
	}
	if len(s) > 300 {
		return s[:300] + "..."
	}
	return s
}

// generic converts a JavaScript value to the value that encoding/json
// gives for its JSON text: the order of the properties is lost.
func generic(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = generic(e)
		}
		return out
	case *js.Obj[any]:
		if x == nil {
			return nil
		}
		out := map[string]any{}
		x.Each(func(k string, e any) {
			if e != nil {
				out[k] = generic(e)
			}
		})
		return out
	}
	if js.IsNullish(v) {
		return nil
	}
	return v
}

// flatten is the flatten function of the generator: the leaves of a config
// by path. An object with no property is a leaf.
func flatten(v any, prefix []string, out map[string]any) {
	if obj, ok := v.(map[string]any); ok && len(obj) > 0 {
		for k, e := range obj {
			flatten(e, append(append([]string{}, prefix...), k), out)
		}
		return
	}
	key, _ := json.Marshal(prefix)
	out[string(key)] = v
}

// wantConfig returns the leaves of the config of a case: the base config
// with the delta of the case.
func wantConfig(base map[string]any, parsed *js.Obj[any]) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	delta := js.AsObj(parsed.Get("configDelta"))
	del, _ := delta.Get("del").([]any)
	for _, path := range del {
		key, _ := json.Marshal(generic(path))
		delete(out, string(key))
	}
	set, _ := delta.Get("set").([]any)
	for _, entry := range set {
		pair := entry.([]any)
		key, _ := json.Marshal(generic(pair[0]))
		out[string(key)] = generic(pair[1])
	}
	return out
}

// compareConfig compares the config of the render with the config of the
// case, leaf by leaf.
func compareConfig(base map[string]any, wantParsed *js.Obj[any], cfg *js.Obj[any]) string {
	want := wantConfig(base, wantParsed)
	got := map[string]any{}
	// JSON.stringify drops the properties that are undefined.
	var gotValue any
	if err := json.Unmarshal([]byte(js.StringifyJSON(cfg)), &gotValue); err != nil {
		return "config: " + err.Error()
	}
	flatten(gotValue, nil, got)
	for k, w := range want {
		g, ok := got[k]
		if !ok {
			return fmt.Sprintf("config%s: missing, want %v", k, w)
		}
		if !reflect.DeepEqual(g, w) {
			return fmt.Sprintf("config%s: got %v, want %v", k, g, w)
		}
	}
	for k, g := range got {
		if _, ok := want[k]; !ok {
			return fmt.Sprintf("config%s: got %v, want none", k, g)
		}
	}
	return ""
}

// compareParsed compares the db after the parse with the dump of a case.
func compareParsed(base map[string]any, c *js.Obj[any], p *parsed) string {
	wantParsed := js.AsObj(c.Get("parsed"))
	if p.db == nil {
		return fmt.Sprintf("diagram type %q has no block db", p.typ)
	}
	got := snapshot(p.db)
	for _, k := range []string{"blocks", "flat", "treeRefs", "columns", "edges", "classes", "dbFunctions", "blockConfig"} {
		if d := diffValues("parsed."+k, got.Get(k), wantParsed.Get(k)); d != "" {
			return d
		}
	}
	return compareConfig(base, wantParsed, p.ctx.Config)
}

type result struct {
	name    string
	text    string
	problem string
	// isError is true for a case that mermaid rejects or fails to draw.
	isError bool
	hasDB   bool
	corpus  bool
}

// runGolden parses every case of the golden file and compares the db with
// the dump of upstream. A case without a dump must fail to parse with the
// message of upstream.
func runGolden(tb testing.TB) []result {
	tb.Helper()
	g := loadGolden(tb)
	base := map[string]any{}
	flatten(generic(g.Get("baseConfig")), nil, base)
	cases, _ := g.Get("cases").([]any)
	results := make([]result, 0, len(cases))
	for _, item := range cases {
		c := js.AsObj(item)
		name, _ := c.Get("name").(string)
		text := caseText(tb, c)
		_, hasText := c.Get("text").(string)
		r := result{name: name, text: text, corpus: !hasText}
		fresh, _ := c.Get("fresh").(bool)
		idCounter, _ := c.Get("idCounter").(float64)
		wantError, isError := c.Get("error").(string)
		wantType, _ := c.Get("type").(string)
		r.isError = isError
		r.hasDB = c.Get("parsed") != nil

		p, err := parseText(text, siteConfig(c), !fresh, int(idCounter))
		switch {
		case r.hasDB && err != nil:
			r.problem = "error " + strconv.Quote(errorMessage(err)) + ", want a db"
		case r.hasDB:
			// The case can have an error too: upstream then failed after
			// the parse, in the style sheet or in the draw function.
			r.problem = compareParsed(base, c, p)
		case isError && err == nil:
			r.problem = fmt.Sprintf("no error, want %q", wantError)
		case isError:
			if got := errorMessage(err); got != wantError {
				r.problem = fmt.Sprintf("error %q, want %q", got, wantError)
			}
		case err != nil:
			r.problem = "error " + strconv.Quote(errorMessage(err)) + ", want none"
		case p.typ != wantType:
			r.problem = fmt.Sprintf("diagram type %q, want %q", p.typ, wantType)
		}
		results = append(results, r)
	}
	return results
}

// TestDBGolden compares the db with the dump of real mermaid for every case
// of the golden file.
func TestDBGolden(t *testing.T) {
	results := runGolden(t)
	var total, failed, corpus, corpusFailed, withDB, withError int
	for _, r := range results {
		total++
		if r.corpus {
			corpus++
		}
		if r.hasDB {
			withDB++
		}
		if r.isError {
			withError++
		}
		if r.problem == "" {
			continue
		}
		failed++
		if r.corpus {
			corpusFailed++
		}
		if failed <= 25 {
			text := r.text
			if r.corpus {
				text = ""
			}
			t.Errorf("%s: %s\n%s", r.name, r.problem, indent(text))
		}
	}
	t.Logf("%d cases (%d corpus fixtures, %d with a db dump, %d with an error of mermaid): %d agree, %d differ (%d corpus fixtures differ)",
		total, corpus, withDB, withError, total-failed, failed, corpusFailed)
}

func indent(text string) string {
	if text == "" {
		return ""
	}
	return "    " + strings.ReplaceAll(strings.TrimRight(text, "\n"), "\n", "\n    ")
}
