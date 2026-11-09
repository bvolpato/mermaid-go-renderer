package venn

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/browser"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden files come from tools/reference/gen-venn.mjs. They have the
// results of the real @upsetjs/venn.js 2.0.0 for seeded random inputs.
//
// testdata/venn.json.gz is from node, with the fdlibm functions as the Math
// functions (fdlibm_ref_test.go has the same functions) and the sort of
// node. testdata/venn-chromium.json.gz is from the page of the browser
// oracle: the copy of venn.js in the mermaid bundle, with the Math and the
// sort of Chromium. The port runs with its defaults for that file.
//
// Every number is compared by its 64 bits.

type goldenCase struct {
	Name     string              `json:"name"`
	Kind     string              `json:"kind"`
	Family   string              `json:"family"`
	Areas    [][]json.RawMessage `json:"areas"`
	Width    string              `json:"width"`
	Height   string              `json:"height"`
	Padding  string              `json:"padding"`
	Circles  [][]string          `json:"circles"`
	Fn       string              `json:"fn"`
	Values   []string            `json:"values"`
	Interior [][]string          `json:"interior"`
	Exterior [][]string          `json:"exterior"`
	Solution [][]string          `json:"solution"`
	Attach   bool                `json:"attach"`
	Out      map[string]any      `json:"out"`
}

type goldenFile struct {
	Meta  map[string]any `json:"meta"`
	Cases []goldenCase   `json:"cases"`
}

func loadGoldenFile(t testing.TB, name string) *goldenFile {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var g goldenFile
	if err := json.NewDecoder(zr).Decode(&g); err != nil {
		t.Fatal(err)
	}
	return &g
}

// hexFloat is hex(v) of the generator: the 64 bits of the double.
func hexFloat(v float64) string {
	if math.IsNaN(v) {
		return "7ff8000000000000"
	}
	return fmt.Sprintf("%016x", math.Float64bits(v))
}

// unhex reads a number of a golden file. The cases run on several
// goroutines, so a malformed file ends the test with a panic.
func unhex(s string) float64 {
	bits, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		panic(fmt.Sprintf("golden file: number %q: %v", s, err))
	}
	return math.Float64frombits(bits)
}

func decodeAreas(raw [][]json.RawMessage) []*Area {
	areas := make([]*Area, len(raw))
	for i, r := range raw {
		a := &Area{}
		var size string
		var label, weight *string
		for j, dst := range []any{&a.Sets, &size, &label, &weight} {
			if err := json.Unmarshal(r[j], dst); err != nil {
				panic(fmt.Sprintf("golden file: area %s: %v", r[j], err))
			}
		}
		if a.Sets == nil {
			a.Sets = []string{}
		}
		a.Size = unhex(size)
		if label != nil {
			a.Label, a.HasLabel = *label, true
		}
		if weight != nil {
			a.Weight, a.HasWeight = unhex(*weight), true
		}
		areas[i] = a
	}
	return areas
}

func decodeCircles(list [][]string) []*Circle {
	circles := make([]*Circle, len(list))
	for i, c := range list {
		circles[i] = &Circle{X: unhex(c[0]), Y: unhex(c[1]), Radius: unhex(c[2])}
	}
	return circles
}

func solutionOut(o *js.Obj[*Circle]) []any {
	out := []any{}
	for _, k := range o.Keys() {
		c := o.Get(k)
		out = append(out, []any{k, hexFloat(c.X), hexFloat(c.Y), hexFloat(c.Radius)})
	}
	return out
}

func centreOut(c *TextCentre) any {
	if c == nil {
		return nil
	}
	return []any{hexFloat(c.X), hexFloat(c.Y), c.Disjoint}
}

func arcOut(a Arc, id any) []any {
	return []any{id, hexFloat(a.Width), hexFloat(a.P1.X), hexFloat(a.P1.Y), hexFloat(a.P2.X), hexFloat(a.P2.Y), a.Large, a.Sweep}
}

// seededRandom is the Math.random of the oracle: mulberry32.
type seededRandom struct {
	state uint32
	calls int
}

func (r *seededRandom) reset() { r.state, r.calls = 0x6d6d6467, 0 }

func (r *seededRandom) next() float64 {
	r.calls++
	r.state += 0x6d2b79f5
	t := r.state
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

// goldenEnv is the environment of one golden file.
type goldenEnv struct {
	rnd *seededRandom
	env *Env
}

func nodeEnv() *goldenEnv {
	r := &seededRandom{}
	return &goldenEnv{rnd: r, env: &Env{Random: r.next, Math: fdlibmMath, Sort: js.V8TimSort}}
}

func chromiumEnv() *goldenEnv {
	r := &seededRandom{}
	return &goldenEnv{rnd: r, env: &Env{Random: r.next}}
}

// stage runs one try block of the probe. It returns the message of the
// exception, or "".
func stage(body func()) (message string) {
	_, err := run(func() struct{} {
		body()
		return struct{}{}
	})
	if err != nil {
		return err.(*Error).Message
	}
	return ""
}

// jsonText is JSON.stringify for a value of arrays, strings, booleans and
// null.
func jsonText(sb *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
	case string:
		sb.WriteString(js.QuoteJSON(x))
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case []any:
		sb.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				sb.WriteByte(',')
			}
			jsonText(sb, e)
		}
		sb.WriteByte(']')
	default:
		panic(fmt.Sprintf("jsonText: %T", v))
	}
}

func hasProto(areas []*Area) bool {
	for _, a := range areas {
		for _, s := range a.Sets {
			if s == "__proto__" {
				return true
			}
		}
	}
	return false
}

// runLayout is runLayout of the probe.
func runLayout(g *goldenEnv, c *goldenCase) map[string]any {
	out := map[string]any{}
	width, height, padding := unhex(c.Width), unhex(c.Height), unhex(c.Padding)

	// The steps of layout(), one by one.
	g.rnd.reset()
	e := newEngine(g.env)
	if msg := stage(func() {
		data := decodeAreas(c.Areas)
		checkSets(data)
		solution := e.venn(data)
		out["venn"] = solutionOut(solution)
		out["vennRandom"] = float64(g.rnd.calls)
		normalized := e.normalizeSolution(solution, pi/2)
		out["normalized"] = solutionOut(normalized)
		scaled := scaleSolution(normalized, width, height, padding)
		out["scaled"] = solutionOut(scaled)
		centres := e.computeTextCentres(scaled, data)
		list := []any{}
		for _, a := range data {
			list = append(list, centreOut(centres.Get(setsKey(a.Sets))))
		}
		out["centres"] = list
	}); msg != "" {
		out["stepsError"] = msg
	}

	// layout() as mermaid calls it.
	g.rnd.reset()
	res, err := Layout(g.env, decodeAreas(c.Areas), LayoutOptions{Width: width, Height: height, Padding: padding})
	if err != nil {
		out["layoutError"] = err.(*Error).Message
	} else {
		list := []any{}
		for _, a := range res {
			circles := []any{}
			for _, k := range a.Circles {
				if k == nil {
					circles = append(circles, nil)
				} else {
					circles = append(circles, []any{k.SetID, hexFloat(k.X), hexFloat(k.Y), hexFloat(k.Radius)})
				}
			}
			arcs := []any{}
			for _, arc := range a.Arcs {
				arcs = append(arcs, arcOut(arc, arc.Circle.SetID))
			}
			list = append(list, []any{centreOut(a.Text), circles, arcs, a.Path, a.DistinctPath})
		}
		out["layout"] = list
		out["layoutRandom"] = float64(g.rnd.calls)
	}

	// The numbers of the chart of VennDiagram, without the DOM.
	g.rnd.reset()
	e = newEngine(g.env)
	if msg := stage(func() {
		all := decodeAreas(c.Areas)
		checkSets(all)
		toRemove := map[string]bool{}
		for _, datum := range all {
			if datum.Size == 0 && len(datum.Sets) == 1 {
				toRemove[datum.Sets[0]] = true
			}
		}
		data := []*Area{}
		for _, datum := range all {
			keep := true
			for _, set := range datum.Sets {
				if toRemove[set] {
					keep = false
				}
			}
			if keep {
				data = append(data, datum)
			}
		}
		chart := map[string]any{"circles": []any{}, "paths": []any{}, "centres": []any{}}
		out["chart"] = chart
		if len(data) > 0 {
			solution := e.venn(data)
			solution = e.normalizeSolution(solution, pi/2)
			circles := scaleSolution(solution, width, height, 15)
			centres := e.computeTextCentres(circles, data)
			chart["circles"] = solutionOut(circles)
			paths := []any{}
			for _, d := range data {
				members := make([]*Circle, len(d.Sets))
				for i, set := range d.Sets {
					members[i] = circles.Get(set)
				}
				paths = append(paths, e.intersectionAreaPath(members, 0, false))
			}
			chart["paths"] = paths
			cs := []any{}
			for _, d := range data {
				centre := centres.Get(setsKey(d.Sets))
				cs = append(cs, []any{js.Str(math.Floor(centre.X)), js.Str(math.Floor(centre.Y))})
			}
			chart["centres"] = cs
		}
	}); msg != "" {
		out["chartError"] = msg
	}
	return out
}

func digestOf(out map[string]any) string {
	var chart any
	if c, ok := out["chart"].(map[string]any); ok {
		chart = []any{c["circles"], c["paths"], c["centres"]}
	}
	var sb strings.Builder
	jsonText(&sb, []any{out["normalized"], out["layout"], chart})
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])[:32]
}

func runIntersectionArea(g *goldenEnv, c *goldenCase) map[string]any {
	out := map[string]any{}
	e := newEngine(g.env)
	if msg := stage(func() {
		circles := decodeCircles(c.Circles)
		stats := &areaStats{}
		area := e.intersectionArea(circles, stats)
		arcs := []any{}
		for _, a := range stats.arcs {
			// circles.indexOf finds the first equal element.
			index := -1
			for i, k := range circles {
				if k == a.Circle {
					index = i
					break
				}
			}
			arcs = append(arcs, arcOut(a, float64(index)))
		}
		inner := []any{}
		for _, p := range stats.innerPoints {
			inner = append(inner, []any{hexFloat(p.x), hexFloat(p.y), float64(p.parentIndex[0]), float64(p.parentIndex[1])})
		}
		out["area"] = hexFloat(area)
		out["arcArea"] = hexFloat(stats.arcArea)
		out["polygonArea"] = hexFloat(stats.polygonArea)
		out["arcs"] = arcs
		out["inner"] = inner
		out["points"] = float64(len(stats.intersectionPoints))
		out["path"] = e.intersectionAreaPath(circles, 0, false)
		out["path2"] = e.intersectionAreaPath(circles, 2, true)
		out["path0"] = e.intersectionAreaPath(circles, 0, true)
	}); msg != "" {
		return map[string]any{"error": msg}
	}
	return out
}

func runScalar(g *goldenEnv, c *goldenCase) map[string]any {
	out := map[string]any{}
	e := newEngine(g.env)
	v := make([]float64, len(c.Values))
	for i, s := range c.Values {
		v[i] = unhex(s)
	}
	if msg := stage(func() {
		switch c.Fn {
		case "circleOverlap":
			out["value"] = hexFloat(e.circleOverlap(v[0], v[1], v[2]))
		case "circleArea":
			out["value"] = hexFloat(e.circleArea(v[0], v[1]))
		case "distanceFromIntersectArea":
			out["value"] = hexFloat(e.distanceFromIntersectArea(v[0], v[1], v[2]))
		case "circleCircleIntersection":
			pts := circleCircleIntersection(&Circle{X: v[0], Y: v[1], Radius: v[2]}, &Circle{X: v[3], Y: v[4], Radius: v[5]})
			list := []any{}
			for _, p := range pts {
				list = append(list, []any{hexFloat(p.X), hexFloat(p.Y)})
			}
			out["points"] = list
		default:
			panic(fmt.Sprintf("golden file: unknown function %q", c.Fn))
		}
	}); msg != "" {
		return map[string]any{"error": msg}
	}
	return out
}

func runTextCentre(g *goldenEnv, c *goldenCase) map[string]any {
	out := map[string]any{}
	e := newEngine(g.env)
	if msg := stage(func() {
		out["centre"] = centreOut(e.computeTextCentre(decodeCircles(c.Interior), decodeCircles(c.Exterior)))
	}); msg != "" {
		return map[string]any{"error": msg}
	}
	return out
}

func runNormalize(g *goldenEnv, c *goldenCase) map[string]any {
	out := map[string]any{}
	e := newEngine(g.env)
	if msg := stage(func() {
		solution := js.NewObj[*Circle]()
		for _, s := range c.Solution {
			solution.Set(s[0], &Circle{X: unhex(s[1]), Y: unhex(s[2]), Radius: unhex(s[3])})
		}
		normalized := e.normalizeSolution(solution, pi/2)
		out["normalized"] = solutionOut(normalized)
		scaled := scaleSolution(normalized, unhex(c.Width), unhex(c.Height), unhex(c.Padding))
		out["scaled"] = solutionOut(scaled)
	}); msg != "" {
		return map[string]any{"error": msg}
	}
	return out
}

func runInitial(g *goldenEnv, c *goldenCase) map[string]any {
	out := map[string]any{}
	try := func(name string, fn func(e *engine, areas []*Area) *js.Obj[*Circle]) {
		g.rnd.reset()
		e := newEngine(g.env)
		if msg := stage(func() {
			data := decodeAreas(c.Areas)
			checkSets(data)
			areas := e.addMissingAreas(data)
			circles := fn(e, areas)
			out[name] = solutionOut(circles)
			out[name+"Loss"] = hexFloat(e.lossFunction(circles, areas))
			out[name+"Random"] = float64(g.rnd.calls)
		}); msg != "" {
			out[name+"Error"] = msg
		}
	}
	try("greedy", func(e *engine, areas []*Area) *js.Obj[*Circle] { return e.greedyLayout(areas) })
	try("mds", func(e *engine, areas []*Area) *js.Obj[*Circle] { return e.constrainedMDSLayout(areas) })
	try("best", func(e *engine, areas []*Area) *js.Obj[*Circle] { return e.bestInitialLayout(areas) })
	e := newEngine(g.env)
	if msg := stage(func() {
		list := []any{}
		for _, a := range e.addMissingAreas(decodeAreas(c.Areas)) {
			sets := []any{}
			for _, s := range a.Sets {
				sets = append(sets, s)
			}
			list = append(list, []any{sets, hexFloat(a.Size)})
		}
		out["areas"] = list
	}); msg != "" {
		out["areasError"] = msg
	}
	return out
}

// runChart is runChart of the probe: the chart on a new div, detached or a
// child of the container of the page.
func runChart(g *goldenEnv, c *goldenCase) map[string]any {
	g.rnd.reset()
	doc := browser.NewDocument()
	container := doc.Body.Append("div").Attr("id", "container")
	div := doc.CreateElement("div")
	if c.Attach {
		container.AppendChild(div)
	}
	_, err := Diagram(g.env, div, decodeAreas(c.Areas), DiagramOptions{Width: unhex(c.Width), Height: unhex(c.Height)})
	if err != nil {
		return map[string]any{"error": err.(*Error).Message}
	}
	return map[string]any{"html": div.InnerHTML(), "random": float64(g.rnd.calls)}
}

// thrownStrings are the exceptions of venn.js that are strings. The other
// exceptions are TypeErrors, and the port has other messages for them.
var thrownStrings = map[string]bool{
	"Initial bisect points must have opposite signs": true,
	"ERROR: missing pairwise overlap information":    true,
}

// sameError compares the message of an exception with the golden message.
func sameError(got, want string) bool {
	if thrownStrings[want] || thrownStrings[got] {
		return got == want
	}
	return true
}

// compareStages compares the results of the try blocks of a case: stages
// maps the name of the error key of a block to the keys of its results.
func compareStages(t *testing.T, name string, got, want map[string]any, stages map[string][]string, proto bool) bool {
	ok := true
	for errKey, keys := range stages {
		wantErr, wantFailed := want[errKey].(string)
		gotErr, gotFailed := got[errKey].(string)
		switch {
		case wantFailed && !gotFailed:
			t.Errorf("%s: no %s, venn.js has %q", name, errKey, wantErr)
			ok = false
		case !wantFailed && gotFailed:
			if proto {
				// The port fails for the set id "__proto__" in every
				// case. venn.js does not fail in some of them.
				continue
			}
			t.Errorf("%s: %s %q, venn.js has none", name, errKey, gotErr)
			ok = false
		case wantFailed:
			if !sameError(gotErr, wantErr) {
				t.Errorf("%s: %s %q, venn.js has %q", name, errKey, gotErr, wantErr)
				ok = false
			}
		default:
			for _, key := range keys {
				w, has := want[key]
				if !has {
					continue
				}
				if !reflect.DeepEqual(got[key], w) {
					gotJSON, _ := json.Marshal(got[key])
					wantJSON, _ := json.Marshal(w)
					t.Errorf("%s: %s differs\n got  %s\n want %s", name, key, clip(string(gotJSON)), clip(string(wantJSON)))
					ok = false
				}
			}
		}
	}
	return ok
}

func clip(s string) string {
	if len(s) > 1500 {
		return s[:1500] + "..."
	}
	return s
}

var layoutStages = map[string][]string{
	"stepsError":  {"venn", "vennRandom", "normalized", "scaled", "centres"},
	"layoutError": {"layout", "layoutRandom"},
	"chartError":  {"chart"},
}

var initialStages = map[string][]string{
	"greedyError": {"greedy", "greedyLoss", "greedyRandom"},
	"mdsError":    {"mds", "mdsLoss", "mdsRandom"},
	"bestError":   {"best", "bestLoss", "bestRandom"},
	"areasError":  {"areas"},
}

var simpleStages = map[string][]string{
	"error": {"area", "arcArea", "polygonArea", "arcs", "inner", "points", "path", "path2", "path0", "value", "centre", "normalized", "scaled", "html", "random"},
}

// runCase runs one case and compares the results.
func runCase(t *testing.T, g *goldenEnv, c *goldenCase) bool {
	switch c.Kind {
	case "layout":
		got := runLayout(g, c)
		proto := hasProto(decodeAreas(c.Areas))
		ok := compareStages(t, c.Name, got, c.Out, layoutStages, proto)
		_, e1 := got["stepsError"]
		_, e2 := got["layoutError"]
		_, e3 := got["chartError"]
		if ok && !e1 && !e2 && !e3 {
			if d := digestOf(got); d != c.Out["digest"] {
				t.Errorf("%s: the digest of the normalized circles, the layout and the chart differs: %s, want %s", c.Name, d, c.Out["digest"])
				ok = false
			}
		}
		return ok
	case "intersectionArea":
		return compareStages(t, c.Name, runIntersectionArea(g, c), c.Out, simpleStages, false)
	case "scalar":
		return compareStages(t, c.Name, runScalar(g, c), c.Out, simpleStages, false)
	case "textCentre":
		return compareStages(t, c.Name, runTextCentre(g, c), c.Out, simpleStages, false)
	case "normalize":
		return compareStages(t, c.Name, runNormalize(g, c), c.Out, simpleStages, false)
	case "initial":
		return compareStages(t, c.Name, runInitial(g, c), c.Out, initialStages, hasProto(decodeAreas(c.Areas)))
	case "chart":
		return compareStages(t, c.Name, runChart(g, c), c.Out, simpleStages, hasProto(decodeAreas(c.Areas)))
	}
	t.Errorf("%s: unknown kind %q", c.Name, c.Kind)
	return false
}

// runGolden runs the cases of a golden file on all processors. Each
// goroutine has its own environment, because a case resets the random
// numbers.
func runGolden(t *testing.T, file string, newEnv func() *goldenEnv, minCases int) {
	golden := loadGoldenFile(t, file)
	if len(golden.Cases) < minCases {
		t.Fatalf("only %d golden cases", len(golden.Cases))
	}
	var mu sync.Mutex
	total := map[string]int{}
	passed := map[string]int{}
	failures := 0
	jobs := make(chan *goldenCase)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := newEnv()
			for c := range jobs {
				mu.Lock()
				skip := failures > 25
				mu.Unlock()
				if skip {
					continue
				}
				ok := runCase(t, g, c)
				mu.Lock()
				total[c.Kind]++
				if ok {
					passed[c.Kind]++
				} else {
					failures++
				}
				mu.Unlock()
			}
		}()
	}
	for i := range golden.Cases {
		jobs <- &golden.Cases[i]
	}
	close(jobs)
	wg.Wait()
	if failures > 25 {
		t.Errorf("too many failures: the other cases did not run")
	}
	kinds := make([]string, 0, len(total))
	for kind := range total {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		t.Logf("%s: %d of %d cases are bit-exact", kind, passed[kind], total[kind])
	}
}

// TestGoldenNode compares the port with venn.js in node.
func TestGoldenNode(t *testing.T) {
	runGolden(t, "venn.json.gz", nodeEnv, 4000)
}

// TestGoldenChromium compares the port, with its default Math functions and
// its default sort, with venn.js in Chromium.
func TestGoldenChromium(t *testing.T) {
	if _, err := os.Stat("testdata/venn-chromium.json.gz"); err != nil {
		t.Skip("testdata/venn-chromium.json.gz is not generated yet")
	}
	runGolden(t, "venn-chromium.json.gz", chromiumEnv, 1000)
}
