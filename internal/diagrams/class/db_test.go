package class

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
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the db of real mermaid at the moment when the renderer
// draws: tools/reference/gen-class-db.mjs writes it with the browser oracle.
// The tests parse the same texts and compare everything. Arrays and maps
// are compared in order. The order of the keys of an object is not
// compared.

type goldenFile struct {
	BaseConfig map[string]any `json:"baseConfig"`
	Cases      []goldenCase   `json:"cases"`
}

type goldenCase struct {
	Name string `json:"name"`
	// Text is absent for a corpus fixture: Name is then its path.
	Text *string `json:"text"`
	// Fresh is true for a case that ran in a new page.
	Fresh bool `json:"fresh"`
	// Site is the site config: the argument of mermaid.initialize.
	Site map[string]any `json:"site"`
	// ClassCounter is the value of the module-level counter of the class
	// DOM ids before the parse.
	ClassCounter int     `json:"classCounter"`
	Error        *string `json:"error"`
	ErrorName    string  `json:"errorName"`
	Type         string  `json:"type"`

	Nodes       []any `json:"nodes"`
	Edges       []any `json:"edges"`
	Other       any   `json:"other"`
	ConfigDelta *struct {
		Set [][]any    `json:"set"`
		Del [][]string `json:"del"`
	} `json:"configDelta"`
	DataDirection  string `json:"dataDirection"`
	Layout         string `json:"layout"`
	Classes        []any  `json:"classes"`
	Relations      []any  `json:"relations"`
	Notes          []any  `json:"notes"`
	Namespaces     []any  `json:"namespaces"`
	Interfaces     []any  `json:"interfaces"`
	StyleClasses   []any  `json:"styleClasses"`
	Direction      string `json:"direction"`
	Title          string `json:"title"`
	AccTitle       string `json:"accTitle"`
	AccDescription string `json:"accDescription"`
	DomIDs         []any  `json:"domIds"`
	Tooltips       []any  `json:"tooltips"`
	Functions      int    `json:"functions"`
	DisplayDetails []any  `json:"displayDetails"`
	ClassConfig    any    `json:"classConfig"`
}

func loadGolden(tb testing.TB) *goldenFile {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "db.json.gz"))
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
	var g goldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

func caseText(tb testing.TB, c *goldenCase) string {
	tb.Helper()
	if c.Text != nil {
		return *c.Text
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(c.Name)))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

// parsed is the result of the steps of mermaid.render up to the draw
// function of the diagram.
type parsed struct {
	ctx *render.Context
	// typ is the diagram type, or "" when no detector matched.
	typ string
	// db is nil when the diagram type is not a class diagram.
	db *DB
}

const maxTextExceeded = "graph TB;a[Maximum text size in diagram exceeded];style a fill:#faa"

// parseText runs the steps of render.Render before the draw function:
// mermaid.initialize(site), processAndSetConfigs, Diagram.fromText. warm
// says that the page has rendered a class diagram before, so the DOMPurify
// hooks of mermaid are installed. classCounter is the value of the counter
// of the class DOM ids in the page.
func parseText(text string, site *js.Obj[any], warm bool, classCounter int) (*parsed, error) {
	resolved, err := config.Resolve(text, site)
	if err != nil {
		return nil, err
	}
	cfg := resolved.Config
	code := resolved.Source
	if v, ok := cfg.Get("maxTextSize").(float64); ok && float64(js.UTF16Len(code.Cleaned)) > v {
		code = config.DiagramCode{Raw: maxTextExceeded, Cleaned: maxTextExceeded}
	}
	ctx := render.NewContext(render.ContextOptions{Config: cfg, ConfigState: resolved.State})
	ctx.Title = resolved.Title
	if warm {
		ctx.Sanitizer.RemoveScript("")
	}
	for i := 0; i < classCounter; i++ {
		ctx.Counter(classCounterName)
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

// errorMessage is error.name and error.message of the JavaScript exception.
func errorMessage(err error) (name, message string) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message
	}
	var te *TypeError
	if errors.As(err, &te) {
		return "TypeError", te.Message
	}
	var je *jison.Error
	if errors.As(err, &je) {
		return je.Name, je.Message
	}
	return "Error", err.Error()
}

// ---- conversion of the Go side to JSON data ----

// generic converts a dynamic JavaScript value to the value that
// encoding/json gives for JSON.stringify of it: an undefined property is
// absent, undefined in an array is null, and a number that is not finite is
// { $num: "NaN" }, as the oracle writes it.
func generic(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case bool, string:
		return x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return map[string]any{"$num": js.Str(x)}
		}
		return x
	case int:
		return float64(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = generic(e)
		}
		return out
	case []string:
		return stringsJSON(x)
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
	panic(fmt.Sprintf("generic: unexpected %T", v))
}

func stringsJSON(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// memberJSON is JSON.stringify of a ClassMember.
func memberJSON(m *ClassMember) map[string]any {
	out := map[string]any{
		"memberType": m.MemberType,
		"visibility": m.Visibility,
		"classifier": m.Classifier,
		"text":       m.Text,
		"id":         m.ID,
	}
	if m.MemberType == "method" {
		out["parameters"] = m.Parameters
		out["returnType"] = m.ReturnType
	}
	return out
}

func membersJSON(list []*ClassMember) []any {
	out := make([]any, len(list))
	for i, m := range list {
		out[i] = memberJSON(m)
	}
	return out
}

// classJSON is JSON.stringify of a ClassNode.
func classJSON(c *ClassNode) map[string]any {
	out := map[string]any{
		"id":          c.ID,
		"type":        c.Type,
		"label":       c.Label,
		"text":        c.Text,
		"shape":       c.Shape,
		"cssClasses":  c.CSSClasses,
		"methods":     membersJSON(c.Methods),
		"members":     membersJSON(c.Members),
		"annotations": stringsJSON(c.Annotations),
		"styles":      stringsJSON(c.Styles),
		"domId":       c.DomID,
	}
	if c.HasParent {
		out["parent"] = c.Parent
	}
	if c.LinkDefined {
		out["link"] = c.Link
	}
	if c.HasLinkTarget {
		out["linkTarget"] = c.LinkTarget
	}
	if c.HaveCallback {
		out["haveCallback"] = true
	}
	if c.HasTooltip {
		out["tooltip"] = c.Tooltip
	}
	return out
}

// relationType is the JSON value of a relation type: a number, or 'none'.
func relationType(t int) any {
	if t == relNone {
		return "none"
	}
	return float64(t)
}

// relationJSON is JSON.stringify of a ClassRelation.
func relationJSON(r *ClassRelation) map[string]any {
	out := map[string]any{
		"id1": r.ID1,
		"id2": r.ID2,
		"relation": map[string]any{
			"type1":    relationType(r.Relation.Type1),
			"type2":    relationType(r.Relation.Type2),
			"lineType": float64(r.Relation.LineType),
		},
		"relationTitle1": r.RelationTitle1,
		"relationTitle2": r.RelationTitle2,
	}
	if r.HasTitle {
		out["title"] = r.Title
	}
	return out
}

// noteJSON is JSON.stringify of a ClassNote.
func noteJSON(n *ClassNote) map[string]any {
	out := map[string]any{"id": n.ID, "text": n.Text, "index": float64(n.Index)}
	if n.HasClass {
		out["class"] = n.Class
	}
	if n.HasParent {
		out["parent"] = n.Parent
	}
	return out
}

// namespaceJSON is a NamespaceNode in the form of the golden file: the
// maps are the lists of their keys.
func namespaceJSON(ns *NamespaceNode) map[string]any {
	out := map[string]any{
		"id":       ns.ID,
		"label":    ns.Label,
		"domId":    ns.DomID,
		"explicit": ns.Explicit,
		"classes":  stringsJSON(ns.Classes.Keys()),
		"notes":    stringsJSON(ns.Notes.Keys()),
		"children": stringsJSON(ns.Children.Keys()),
	}
	if ns.HasParent {
		out["parent"] = ns.Parent
	}
	return out
}

// The node and edge models of the unified renderer have no separate state
// for an undefined string: the empty string stands for it. These are the
// properties that upstream can set to the empty string. The comparison
// removes them from the expected object when they are empty.
var (
	emptyNodeStrings = []string{"tooltip", "link", "linkTarget", "parentId"}
	emptyEdgeStrings = []string{"startLabelRight", "endLabelLeft", "arrowheadStyle"}
)

// nodeJSON is a layout node of getData() as JSON data.
func nodeJSON(n *ru.Node) map[string]any {
	out := map[string]any{"id": n.ID, "isGroup": n.IsGroup, "shape": n.Shape}
	str := func(key, v string) {
		if v != "" {
			out[key] = v
		}
	}
	if n.HasLabel {
		out["label"] = n.Label
	}
	if n.CSSClassesSet {
		out["cssClasses"] = n.CSSClasses
	}
	if n.CSSStylesSet {
		out["cssStyles"] = stringsJSON(n.CSSStyles)
	}
	if n.Padding == n.Padding {
		out["padding"] = n.Padding
	}
	if n.ColorIndex == n.ColorIndex {
		out["colorIndex"] = n.ColorIndex
	}
	if n.HaveCallback {
		out["haveCallback"] = true
	}
	str("domId", n.DomID)
	str("link", n.Link)
	str("linkTarget", n.LinkTarget)
	str("tooltip", n.Tooltip)
	str("parentId", n.ParentID)
	str("look", n.Look)
	str("labelType", n.LabelType)
	for k, v := range n.Extra {
		switch x := v.(type) {
		case []*ClassMember:
			out[k] = membersJSON(x)
		default:
			out[k] = generic(v)
		}
	}
	return out
}

// edgeJSON is a layout edge of getData() as JSON data.
func edgeJSON(e *ru.Edge) map[string]any {
	out := map[string]any{"id": e.ID}
	str := func(key, v string) {
		if v != "" {
			out[key] = v
		}
	}
	if e.HasLabel {
		out["label"] = e.Label
	}
	str("start", e.Start)
	str("end", e.End)
	str("type", e.Type)
	str("labelpos", e.Labelpos)
	str("thickness", e.Thickness)
	str("classes", e.Classes)
	str("arrowTypeStart", e.ArrowTypeStart)
	str("arrowTypeEnd", e.ArrowTypeEnd)
	str("startLabelRight", e.StartLabelRight)
	str("endLabelLeft", e.EndLabelLeft)
	str("arrowheadStyle", e.ArrowheadStyle)
	out["labelStyle"] = stringsJSON(e.LabelStyle)
	// The style of a relation is the empty string upstream. The edge model
	// holds a string as an array with one element.
	if len(e.Style) == 1 && e.Style[0] == "" {
		out["style"] = ""
	} else {
		out["style"] = stringsJSON(e.Style)
	}
	str("pattern", e.Pattern)
	str("look", e.Look)
	str("labelType", e.LabelType)
	return out
}

// dropEmpty removes the properties of keys that are the empty string from
// the objects of a list.
func dropEmpty(list []any, keys []string) []any {
	out := make([]any, len(list))
	for i, e := range list {
		obj, ok := e.(map[string]any)
		if !ok {
			out[i] = e
			continue
		}
		cp := make(map[string]any, len(obj))
		for k, v := range obj {
			cp[k] = v
		}
		for _, k := range keys {
			if cp[k] == "" {
				delete(cp, k)
			}
		}
		out[i] = cp
	}
	return out
}

// goldenConfig returns the config of a case: the base config with the
// delta of the case applied.
func goldenConfig(base map[string]any, c *goldenCase) map[string]any {
	var clone func(v any) any
	clone = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			out := make(map[string]any, len(x))
			for k, e := range x {
				out[k] = clone(e)
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, e := range x {
				out[i] = clone(e)
			}
			return out
		}
		return v
	}
	cfg := clone(base).(map[string]any)
	if c.ConfigDelta == nil {
		return cfg
	}
	for _, path := range c.ConfigDelta.Del {
		m := cfg
		for i, key := range path {
			if i == len(path)-1 {
				delete(m, key)
				break
			}
			next, ok := m[key].(map[string]any)
			if !ok {
				break
			}
			m = next
		}
	}
	for _, entry := range c.ConfigDelta.Set {
		pathAny, _ := entry[0].([]any)
		m := cfg
		for i, k := range pathAny {
			key, _ := k.(string)
			if i == len(pathAny)-1 {
				m[key] = entry[1]
				break
			}
			next, ok := m[key].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[key] = next
			}
			m = next
		}
	}
	return cfg
}

// firstDiff returns the path of the first difference of two JSON values,
// or "".
func firstDiff(path string, got, want any) string {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return fmt.Sprintf("%s: got %s, want an object", path, short(got))
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			gv, gok := g[k]
			wv, wok := w[k]
			if !gok {
				return fmt.Sprintf("%s.%s: missing, want %s", path, k, short(wv))
			}
			if !wok {
				return fmt.Sprintf("%s.%s: got %s, want no property", path, k, short(gv))
			}
			if d := firstDiff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
		return ""
	case []any:
		g, ok := got.([]any)
		if !ok {
			return fmt.Sprintf("%s: got %s, want an array", path, short(got))
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := firstDiff(fmt.Sprintf("%s[%d]", path, i), g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: got %d elements, want %d", path, len(g), len(w))
		}
		return ""
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Sprintf("%s: got %s, want %s", path, short(got), short(want))
	}
	return ""
}

func short(v any) string {
	data, _ := json.Marshal(v)
	s := string(data)
	if len(s) > 300 {
		s = s[:300] + "..."
	}
	return s
}

// dumpDB returns the content of the db in the form of the golden file (see
// tools/reference/class-db-page.js).
func dumpDB(p *parsed) (map[string]any, error) {
	d := p.db
	d.SetDiagramId(render.DefaultID)
	data := d.GetData()
	if d.err != nil {
		return nil, d.err
	}
	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = nodeJSON(n)
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = edgeJSON(e)
	}
	classes := []any{}
	domIDs := []any{}
	tooltips := []any{}
	displayDetails := []any{}
	var firstErr error
	d.GetClasses().Each(func(key string, c *ClassNode) {
		classes = append(classes, []any{key, classJSON(c)})
		domID, err := d.LookUpDomId(key)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		domIDs = append(domIDs, domID)
		tooltip, has, err := d.GetTooltip(key)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if has {
			tooltips = append(tooltips, tooltip)
		} else {
			tooltips = append(tooltips, nil)
		}
		details := []any{}
		for _, list := range [][]*ClassMember{c.Members, c.Methods} {
			for _, m := range list {
				dd := m.GetDisplayDetails()
				details = append(details, map[string]any{
					"displayText":     dd.DisplayText,
					"cssStyle":        dd.CSSStyle,
					"classifierStyle": m.ParseClassifier(),
				})
			}
		}
		displayDetails = append(displayDetails, details)
	})
	if firstErr != nil {
		return nil, firstErr
	}
	relations := []any{}
	for _, r := range d.GetRelations() {
		relations = append(relations, relationJSON(r))
	}
	notes := []any{}
	d.GetNotes().Each(func(key string, n *ClassNote) {
		notes = append(notes, []any{key, noteJSON(n)})
	})
	namespaces := []any{}
	d.GetNamespaces().Each(func(key string, ns *NamespaceNode) {
		namespaces = append(namespaces, []any{key, namespaceJSON(ns)})
	})
	interfaces := []any{}
	for _, i := range d.GetInterfaces() {
		interfaces = append(interfaces, map[string]any{"id": i.ID, "label": i.Label, "classId": i.ClassID})
	}
	styleClasses := []any{}
	d.GetStyleClasses().Each(func(key string, sc *StyleClass) {
		styleClasses = append(styleClasses, []any{key, map[string]any{
			"id":         sc.ID,
			"styles":     stringsJSON(sc.Styles),
			"textStyles": stringsJSON(sc.TextStyles),
		}})
	})
	layout := ""
	if v := data.Config.Get("layout"); !js.IsNullish(v) {
		layout = js.ToString(v)
	}
	return map[string]any{
		"type":           p.typ,
		"nodes":          nodes,
		"edges":          edges,
		"other":          map[string]any{},
		"config":         generic(data.Config),
		"dataDirection":  data.Direction,
		"layout":         ru.GetRegisteredLayoutAlgorithm(layout, ""),
		"classes":        classes,
		"relations":      relations,
		"notes":          notes,
		"namespaces":     namespaces,
		"interfaces":     interfaces,
		"styleClasses":   styleClasses,
		"direction":      d.GetDirection(),
		"title":          d.GetDiagramTitle(),
		"accTitle":       d.GetAccTitle(),
		"accDescription": d.GetAccDescription(),
		"domIds":         domIDs,
		"tooltips":       tooltips,
		"functions":      float64(1 + len(d.Callbacks())),
		"displayDetails": displayDetails,
		"classConfig":    generic(d.GetClassConfig()),
	}, nil
}

// goldenDump is the expected dump of a case, in the form of dumpDB.
func goldenDump(g *goldenFile, c *goldenCase) map[string]any {
	return map[string]any{
		"type":           c.Type,
		"nodes":          dropEmpty(c.Nodes, emptyNodeStrings),
		"edges":          dropEmpty(c.Edges, emptyEdgeStrings),
		"other":          c.Other,
		"config":         goldenConfig(g.BaseConfig, c),
		"dataDirection":  c.DataDirection,
		"layout":         c.Layout,
		"classes":        c.Classes,
		"relations":      c.Relations,
		"notes":          c.Notes,
		"namespaces":     c.Namespaces,
		"interfaces":     c.Interfaces,
		"styleClasses":   c.StyleClasses,
		"direction":      c.Direction,
		"title":          c.Title,
		"accTitle":       c.AccTitle,
		"accDescription": c.AccDescription,
		"domIds":         c.DomIDs,
		"tooltips":       c.Tooltips,
		"functions":      float64(c.Functions),
		"displayDetails": c.DisplayDetails,
		"classConfig":    c.ClassConfig,
	}
}

func siteConfig(tb testing.TB, site map[string]any) *js.Obj[any] {
	tb.Helper()
	if site == nil {
		return nil
	}
	data, err := json.Marshal(site)
	if err != nil {
		tb.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		tb.Fatal(err)
	}
	return js.AsObj(v)
}

// compareCase parses the text of a case and compares the result with the
// golden record. It returns a description of the first difference, or "".
func compareCase(tb testing.TB, g *goldenFile, c *goldenCase) string {
	tb.Helper()
	text := caseText(tb, c)
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)
	if s := siteConfig(tb, c.Site); s != nil {
		s.Each(func(k string, v any) { site.Set(k, v) })
	}
	classCounter := c.ClassCounter
	if c.Fresh {
		classCounter = 0
	}
	p, err := parseText(text, site, !c.Fresh, classCounter)
	var dump map[string]any
	if err == nil && p.db != nil {
		dump, err = dumpDB(p)
	}
	if c.Error != nil {
		if err == nil {
			return fmt.Sprintf("no error, want the error %q", *c.Error)
		}
		name, message := errorMessage(err)
		if message != *c.Error {
			return fmt.Sprintf("error %q, want %q", message, *c.Error)
		}
		if c.ErrorName != "" && name != c.ErrorName {
			return fmt.Sprintf("error name %q, want %q", name, c.ErrorName)
		}
		return ""
	}
	if err != nil {
		return fmt.Sprintf("error %q, want no error", err.Error())
	}
	if p.db == nil {
		if p.typ != c.Type {
			return fmt.Sprintf("type %q, want %q", p.typ, c.Type)
		}
		return ""
	}
	return firstDiff("", dump, goldenDump(g, c))
}

// knownDivergences are the cases in which this port differs from upstream
// on purpose. The value is the first difference of the case: the test
// fails when the case has another difference, or none.
var knownDivergences = map[string]string{
	// The site config has class.padding as the string '20'. Upstream copies
	// the string to the nodes. The node model holds a number.
	"site9": `.nodes[0].padding: got 20, want "20"`,
}

// TestDBAgainstMermaid compares the db with the db of real mermaid for
// every case of the golden file: every fixture of the class-diagram family
// of the corpus, mutations of the fixtures and hand-written inputs.
func TestDBAgainstMermaid(t *testing.T) {
	g := loadGolden(t)
	diffs := make([]string, len(g.Cases))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for i := range g.Cases {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			diffs[i] = compareCase(t, g, &g.Cases[i])
		}(i)
	}
	wg.Wait()
	counts := map[string]int{}
	fails := map[string]int{}
	known := 0
	for i := range g.Cases {
		c := &g.Cases[i]
		kind := "extra"
		switch {
		case strings.Contains(c.Name, "#mutant"):
			kind = "mutant"
		case strings.HasPrefix(c.Name, "testdata/"):
			kind = "corpus"
		case strings.HasPrefix(c.Name, "site"):
			kind = "site"
		}
		counts[kind]++
		d := diffs[i]
		if want, ok := knownDivergences[c.Name]; ok {
			known++
			fails[kind]++
			if d != want {
				t.Errorf("%s: known divergence: got the difference %q, want %q", c.Name, d, want)
			}
			continue
		}
		if d != "" {
			fails[kind]++
			t.Errorf("%s: %s", c.Name, d)
		}
	}
	for _, kind := range []string{"corpus", "mutant", "extra", "site"} {
		t.Logf("%s: %d cases, %d equal", kind, counts[kind], counts[kind]-fails[kind])
	}
	t.Logf("known divergences: %d", known)
}
