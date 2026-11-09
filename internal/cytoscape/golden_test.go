package cytoscape

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file testdata/layouts.json.gz is written by
// tools/reference/gen-cytoscape.mjs. It has one record for each cytoscape
// instance that real mermaid creates in Chromium when it renders a mindmap
// or an architecture fixture of the corpus: what mermaid passes to cytoscape
// and what it reads back. See tools/reference/probes/cytoscape.js.

// jnum is a number of the golden file. A number that JSON cannot hold is
// { "$num": "NaN" }, "Infinity", "-Infinity" or "-0".
type jnum float64

func (n *jnum) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '{' {
		var o struct {
			Num string `json:"$num"`
		}
		if err := json.Unmarshal(data, &o); err != nil {
			return err
		}
		switch o.Num {
		case "NaN":
			*n = jnum(math.NaN())
		case "Infinity":
			*n = jnum(math.Inf(1))
		case "-Infinity":
			*n = jnum(math.Inf(-1))
		case "-0":
			*n = jnum(math.Copysign(0, -1))
		default:
			return fmt.Errorf("unknown $num %q", o.Num)
		}
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*n = jnum(f)
	return nil
}

type jpoint struct {
	X jnum `json:"x"`
	Y jnum `json:"y"`
}

type jbox struct {
	X1 jnum `json:"x1"`
	Y1 jnum `json:"y1"`
	X2 jnum `json:"x2"`
	Y2 jnum `json:"y2"`
	W  jnum `json:"w"`
	H  jnum `json:"h"`
}

type goldenAdd struct {
	Group    string          `json:"group"`
	Data     json.RawMessage `json:"data"`
	Classes  string          `json:"classes"`
	Position *jpoint         `json:"position"`
}

type goldenLayout struct {
	Options json.RawMessage `json:"options"`
	Runs    int             `json:"runs"`
}

type goldenEdge struct {
	ID             string  `json:"id"`
	StartX         *jnum   `json:"startX"`
	StartY         *jnum   `json:"startY"`
	MidX           *jnum   `json:"midX"`
	MidY           *jnum   `json:"midY"`
	EndX           *jnum   `json:"endX"`
	EndY           *jnum   `json:"endY"`
	SourceEndpoint *jpoint `json:"sourceEndpoint"`
	Midpoint       *jpoint `json:"midpoint"`
	TargetEndpoint *jpoint `json:"targetEndpoint"`
	HasRscratch    bool    `json:"hasRscratch"`
	RScratch       *struct {
		StartX *jnum `json:"startX"`
		StartY *jnum `json:"startY"`
		MidX   *jnum `json:"midX"`
		MidY   *jnum `json:"midY"`
		EndX   *jnum `json:"endX"`
		EndY   *jnum `json:"endY"`
	} `json:"rscratch"`
}

type goldenNode struct {
	ID          string  `json:"id"`
	Position    *jpoint `json:"position"`
	BoundingBox *jbox   `json:"boundingBox"`
}

type goldenRead struct {
	Destroyed bool         `json:"destroyed"`
	Nodes     []goldenNode `json:"nodes"`
	Edges     []goldenEdge `json:"edges"`
}

type goldenArchNode struct {
	ID    string `json:"id"`
	Icon  string `json:"icon"`
	Title string `json:"title"`
	In    string `json:"in"`
}

type goldenArchitecture struct {
	Config struct {
		IconSize                  jnum `json:"iconSize"`
		FontSize                  jnum `json:"fontSize"`
		Padding                   jnum `json:"padding"`
		IdealEdgeLengthMultiplier jnum `json:"idealEdgeLengthMultiplier"`
		EdgeElasticity            jnum `json:"edgeElasticity"`
		Randomize                 bool `json:"randomize"`
		NodeSeparation            jnum `json:"nodeSeparation"`
		NumIter                   jnum `json:"numIter"`
		Seed                      jnum `json:"seed"`
	} `json:"config"`
	Services  []goldenArchNode `json:"services"`
	Junctions []goldenArchNode `json:"junctions"`
	Groups    []goldenArchNode `json:"groups"`
	Edges     []struct {
		LhsID  string `json:"lhsId"`
		RhsID  string `json:"rhsId"`
		LhsDir string `json:"lhsDir"`
		RhsDir string `json:"rhsDir"`
		Title  string `json:"title"`
	} `json:"edges"`
}

type goldenRecord struct {
	Name string `json:"name"`
	// Type is the diagram type: mindmap or architecture.
	Type string `json:"type"`
	// Layout is the layout that mermaid runs: cose-bilkent or fcose.
	Layout string `json:"layout"`
	// SameSvg tells that the render with the probe gave the reference SVG.
	SameSvg      bool                `json:"sameSvg"`
	RandomBefore int                 `json:"randomBefore"`
	Style        json.RawMessage     `json:"style"`
	Initial      json.RawMessage     `json:"initial"`
	Adds         []goldenAdd         `json:"adds"`
	Layouts      []goldenLayout      `json:"layouts"`
	Measure      [][]json.RawMessage `json:"measure"`
	Random       int                 `json:"random"`
	Result       goldenRead          `json:"result"`
	After        goldenRead          `json:"after"`
	Architecture *goldenArchitecture `json:"architecture"`
}

type goldenFile struct {
	Env struct {
		UserAgent        string `json:"userAgent"`
		CytoscapeVersion string `json:"cytoscapeVersion"`
		Now              int64  `json:"now"`
	} `json:"env"`
	Records []*goldenRecord `json:"records"`
}

var (
	goldenOnce sync.Once
	golden     *goldenFile
	goldenErr  error
)

func loadGolden(t testing.TB) *goldenFile {
	t.Helper()
	goldenOnce.Do(func() {
		f, err := os.Open("testdata/layouts.json.gz")
		if err != nil {
			goldenErr = err
			return
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			goldenErr = err
			return
		}
		golden = &goldenFile{}
		goldenErr = json.NewDecoder(zr).Decode(golden)
	})
	if goldenErr != nil {
		t.Fatal(goldenErr)
	}
	return golden
}

// pageEnv is the page of the reference oracle for one record: the seeded
// Math.random after the draws that happen before the render, the fixed
// clock, and the widths that the canvas of Chromium gave for the labels.
type pageEnv struct {
	t            testing.TB
	rng          uint32
	randomCalls  int
	measure      map[string]float64
	measureCalls int
}

func newPageEnv(t testing.TB, rec *goldenRecord) *pageEnv {
	p := &pageEnv{t: t, rng: 0x6d6d6467, measure: map[string]float64{}}
	for i := 0; i < rec.RandomBefore; i++ {
		p.random()
	}
	p.randomCalls = 0
	for _, m := range rec.Measure {
		var font, text string
		var width float64
		if err := json.Unmarshal(m[0], &font); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(m[1], &text); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(m[2], &width); err != nil {
			t.Fatal(err)
		}
		p.measure[font+"\n"+text] = width
	}
	return p
}

// random is the mulberry32 generator of tools/reference/oracle.mjs.
func (p *pageEnv) random() float64 {
	p.randomCalls++
	p.rng += 0x6d2b79f5
	t := p.rng
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return float64(t^(t>>14)) / 4294967296
}

func (p *pageEnv) env() Env {
	return Env{
		Random: p.random,
		Now:    time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC),
		MeasureText: func(font, text string) float64 {
			p.measureCalls++
			w, ok := p.measure[font+"\n"+text]
			if !ok {
				p.t.Errorf("the layout measures %q with font %q, and the browser did not", text, font)
			}
			return w
		},
	}
}

// diff collects the differences of one record.
type diff struct {
	list []string
}

func (d *diff) num(what string, got float64, want jnum) {
	if math.Float64bits(got) != math.Float64bits(float64(want)) && !(math.IsNaN(got) && math.IsNaN(float64(want))) {
		d.list = append(d.list, fmt.Sprintf("%s: got %v (%#x), want %v (%#x)", what, got, math.Float64bits(got), float64(want), math.Float64bits(float64(want))))
	}
}

// opt compares with a value that is absent in the record when JavaScript had
// undefined.
func (d *diff) opt(what string, got float64, want *jnum) {
	if want == nil {
		if !math.IsNaN(got) {
			d.list = append(d.list, fmt.Sprintf("%s: got %v, want undefined", what, got))
		}
		return
	}
	d.num(what, got, *want)
}

func (d *diff) point(what string, got Point, want *jpoint) {
	if want == nil {
		d.list = append(d.list, what+": the record has no value")
		return
	}
	d.num(what+".x", got.X, want.X)
	d.num(what+".y", got.Y, want.Y)
}

func (d *diff) box(what string, got Box, want *jbox) {
	if want == nil {
		d.list = append(d.list, what+": the record has no value")
		return
	}
	d.num(what+".x1", got.X1, want.X1)
	d.num(what+".y1", got.Y1, want.Y1)
	d.num(what+".x2", got.X2, want.X2)
	d.num(what+".y2", got.Y2, want.Y2)
	d.num(what+".w", got.W, want.W)
	d.num(what+".h", got.H, want.H)
}

func (d *diff) errorf(format string, args ...any) {
	d.list = append(d.list, fmt.Sprintf(format, args...))
}

func (d *diff) report(t *testing.T) {
	t.Helper()
	for i, s := range d.list {
		if i == 8 {
			t.Errorf("and %d more differences", len(d.list)-i)
			break
		}
		t.Error(s)
	}
}

// coseBilkentInput builds the arguments of CoseBilkent from the cy.add calls
// of the record.
func coseBilkentInput(t testing.TB, rec *goldenRecord) ([]Node, []Edge) {
	var nodes []Node
	var edges []Edge
	for _, a := range rec.Adds {
		switch a.Group {
		case "nodes":
			var data struct {
				ID     string `json:"id"`
				Width  jnum   `json:"width"`
				Height jnum   `json:"height"`
			}
			if err := json.Unmarshal(a.Data, &data); err != nil {
				t.Fatal(err)
			}
			nodes = append(nodes, Node{ID: data.ID, Width: float64(data.Width), Height: float64(data.Height), X: float64(a.Position.X), Y: float64(a.Position.Y)})
		case "edges":
			var data struct {
				ID     string `json:"id"`
				Source string `json:"source"`
				Target string `json:"target"`
			}
			if err := json.Unmarshal(a.Data, &data); err != nil {
				t.Fatal(err)
			}
			edges = append(edges, Edge{ID: data.ID, Start: data.Source, End: data.Target})
		}
	}
	return nodes, edges
}

func checkCoseBilkent(rec *goldenRecord, res *CoseBilkentResult, d *diff) {
	if len(res.Nodes) != len(rec.Result.Nodes) || len(res.Edges) != len(rec.Result.Edges) {
		d.errorf("got %d nodes and %d edges, want %d and %d", len(res.Nodes), len(res.Edges), len(rec.Result.Nodes), len(rec.Result.Edges))
		return
	}
	for i, want := range rec.Result.Nodes {
		got := res.Nodes[i]
		if got.ID != want.ID {
			d.errorf("node %d: id %q, want %q", i, got.ID, want.ID)
			continue
		}
		d.point("node "+want.ID, Point{got.X, got.Y}, want.Position)
	}
	for i, want := range rec.Result.Edges {
		got := res.Edges[i]
		if got.ID != want.ID {
			d.errorf("edge %d: id %q, want %q", i, got.ID, want.ID)
			continue
		}
		d.opt("edge "+want.ID+" startX", got.StartX, want.StartX)
		d.opt("edge "+want.ID+" startY", got.StartY, want.StartY)
		d.opt("edge "+want.ID+" midX", got.MidX, want.MidX)
		d.opt("edge "+want.ID+" midY", got.MidY, want.MidY)
		d.opt("edge "+want.ID+" endX", got.EndX, want.EndX)
		d.opt("edge "+want.ID+" endY", got.EndY, want.EndY)
	}
}

// architectureInput builds the argument of Architecture from the record: the
// db values, and the two constraints from the options of the fcose layout.
func architectureInput(t testing.TB, rec *goldenRecord) *ArchitectureInput {
	a := rec.Architecture
	in := &ArchitectureInput{Config: ArchitectureConfig{
		IconSize:                  float64(a.Config.IconSize),
		FontSize:                  float64(a.Config.FontSize),
		Padding:                   float64(a.Config.Padding),
		IdealEdgeLengthMultiplier: float64(a.Config.IdealEdgeLengthMultiplier),
		EdgeElasticity:            float64(a.Config.EdgeElasticity),
		NodeSeparation:            float64(a.Config.NodeSeparation),
		NumIter:                   float64(a.Config.NumIter),
		Randomize:                 a.Config.Randomize,
		Seed:                      float64(a.Config.Seed),
	}}
	conv := func(list []goldenArchNode) []ArchitectureNode {
		out := make([]ArchitectureNode, len(list))
		for i, n := range list {
			out[i] = ArchitectureNode{ID: n.ID, Icon: n.Icon, Title: n.Title, In: n.In}
		}
		return out
	}
	in.Groups, in.Services, in.Junctions = conv(a.Groups), conv(a.Services), conv(a.Junctions)
	for _, e := range a.Edges {
		in.Edges = append(in.Edges, ArchitectureEdge{LhsID: e.LhsID, RhsID: e.RhsID, LhsDir: e.LhsDir, RhsDir: e.RhsDir, Title: e.Title})
	}
	var options struct {
		Alignment struct {
			Horizontal [][]string `json:"horizontal"`
			Vertical   [][]string `json:"vertical"`
		} `json:"alignmentConstraint"`
		Relative []struct {
			Left   string `json:"left"`
			Right  string `json:"right"`
			Top    string `json:"top"`
			Bottom string `json:"bottom"`
			Gap    jnum   `json:"gap"`
		} `json:"relativePlacementConstraint"`
	}
	if err := json.Unmarshal(rec.Layouts[0].Options, &options); err != nil {
		t.Fatal(err)
	}
	in.Alignment = AlignmentConstraint{Horizontal: options.Alignment.Horizontal, Vertical: options.Alignment.Vertical}
	for _, r := range options.Relative {
		in.RelativePlacement = append(in.RelativePlacement, RelativePlacement{Left: r.Left, Right: r.Right, Top: r.Top, Bottom: r.Bottom, Gap: float64(r.Gap)})
	}
	return in
}

func checkArchitecture(rec *goldenRecord, res *ArchitectureResult, d *diff) {
	if len(res.Nodes) != len(rec.Result.Nodes) || len(res.Edges) != len(rec.Result.Edges) {
		d.errorf("got %d nodes and %d edges, want %d and %d", len(res.Nodes), len(res.Edges), len(rec.Result.Nodes), len(rec.Result.Edges))
		return
	}
	for i, want := range rec.Result.Edges {
		got := res.Edges[i]
		if got.ID != want.ID {
			d.errorf("edge %d: id %q, want %q", i, got.ID, want.ID)
			continue
		}
		d.point("edge "+want.ID+" sourceEndpoint", got.SourceEndpoint, want.SourceEndpoint)
		d.point("edge "+want.ID+" midpoint", got.Midpoint, want.Midpoint)
		d.point("edge "+want.ID+" targetEndpoint", got.TargetEndpoint, want.TargetEndpoint)
		if got.HasRScratch != want.HasRscratch {
			d.errorf("edge %s: HasRScratch %v, want %v", want.ID, got.HasRScratch, want.HasRscratch)
		}
		if e := rec.Architecture.Edges[got.Index]; e.LhsID+"-"+e.RhsID != got.ID {
			d.errorf("edge %s: Index %d is the edge %s-%s", got.ID, got.Index, e.LhsID, e.RhsID)
		}
	}
	for i, want := range rec.Result.Nodes {
		got := res.Nodes[i]
		if got.ID != want.ID {
			d.errorf("node %d: id %q, want %q", i, got.ID, want.ID)
			continue
		}
		if want.BoundingBox != nil {
			if got.Type != "group" {
				d.errorf("node %s: type %q, want group", got.ID, got.Type)
			}
			d.box("node "+want.ID+" boundingBox", got.BoundingBox, want.BoundingBox)
		} else {
			d.point("node "+want.ID, Point{got.X, got.Y}, want.Position)
		}
	}
}

// TestGolden runs each layout that mermaid ran in Chromium and compares the
// numbers that mermaid reads, bit for bit.
func TestGolden(t *testing.T) {
	g := loadGolden(t)
	if g.Env.CytoscapeVersion != CytoscapeVersion {
		t.Fatalf("the golden file is from cytoscape %s, the bundle is %s", g.Env.CytoscapeVersion, CytoscapeVersion)
	}
	if g.Env.UserAgent != userAgent {
		t.Errorf("the user agent of the browser is %q", g.Env.UserAgent)
	}
	counts := map[string][2]int{}
	for _, rec := range g.Records {
		rec := rec
		ok := t.Run(rec.Name, func(t *testing.T) {
			if !rec.SameSvg {
				t.Fatal("the record is not from a render that gave the reference SVG")
			}
			page := newPageEnv(t, rec)
			d := &diff{}
			switch rec.Layout {
			case "cose-bilkent":
				nodes, edges := coseBilkentInput(t, rec)
				res, err := CoseBilkent(context.Background(), page.env(), nodes, edges)
				if err != nil {
					t.Fatal(err)
				}
				checkCoseBilkent(rec, res, d)
			case "fcose":
				res, err := Architecture(context.Background(), page.env(), architectureInput(t, rec))
				if err != nil {
					t.Fatal(err)
				}
				checkArchitecture(rec, res, d)
			default:
				t.Fatalf("unknown layout %q", rec.Layout)
			}
			if page.randomCalls != rec.Random {
				d.errorf("%d Math.random calls, want %d", page.randomCalls, rec.Random)
			}
			if page.measureCalls != len(rec.Measure) {
				d.errorf("%d measureText calls, want %d", page.measureCalls, len(rec.Measure))
			}
			d.report(t)
		})
		c := counts[rec.Layout]
		c[0]++
		if ok {
			c[1]++
		}
		counts[rec.Layout] = c
	}
	for _, layout := range []string{"cose-bilkent", "fcose"} {
		t.Logf("%s: %d of %d layouts bit-exact", layout, counts[layout][1], counts[layout][0])
	}
}

// dynamic converts JSON of the golden file to a dynamic value of the package js.
func dynamic(t testing.TB, data json.RawMessage) any {
	if len(data) == 0 {
		return nil
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return decodeNums(v)
}

// decodeNums replaces each { $num: text } of a dynamic value by its number.
func decodeNums(v any) any {
	switch x := v.(type) {
	case []any:
		for i := range x {
			x[i] = decodeNums(x[i])
		}
	case *js.Obj[any]:
		if s, ok := x.Get("$num").(string); ok && x.Len() == 1 {
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
		for _, k := range x.Keys() {
			x.Set(k, decodeNums(x.Get(k)))
		}
	}
	return v
}

// TestRunGolden replays the cose-bilkent records through the general call:
// the style, the elements and the layout options as recorded. The points of
// the edges and the positions must be the same as in the browser, and so
// must the bounding boxes and endpoints that the probe read after the
// render.
func TestRunGolden(t *testing.T) {
	g := loadGolden(t)
	n := 0
	for _, rec := range g.Records {
		if rec.Layout != "cose-bilkent" {
			continue
		}
		// This test runs the interpreter, so a default run leaves out the
		// large layouts. A full run (CYTOSCAPE_FULL=1) has every layout.
		if !fullRun() && len(rec.Adds) > 20 {
			continue
		}
		rec := rec
		n++
		t.Run(rec.Name, func(t *testing.T) {
			page := newPageEnv(t, rec)
			graph := &Graph{Style: dynamic(t, rec.Style), Layout: dynamic(t, rec.Initial), DataDimensions: true}
			for _, a := range rec.Adds {
				el := Element{Group: a.Group, Data: dynamic(t, a.Data).(*js.Obj[any]), Classes: a.Classes}
				if a.Position != nil {
					el.Position = &Point{float64(a.Position.X), float64(a.Position.Y)}
				}
				graph.Elements = append(graph.Elements, el)
			}
			for _, l := range rec.Layouts {
				graph.Layouts = append(graph.Layouts, dynamic(t, l.Options).(*js.Obj[any]))
			}
			res, err := Run(context.Background(), page.env(), graph)
			if err != nil {
				t.Fatal(err)
			}
			d := &diff{}
			if len(res.Nodes) != len(rec.After.Nodes) || len(res.Edges) != len(rec.After.Edges) {
				t.Fatalf("got %d nodes and %d edges, want %d and %d", len(res.Nodes), len(res.Edges), len(rec.After.Nodes), len(rec.After.Edges))
			}
			for i, want := range rec.After.Nodes {
				got := res.Nodes[i]
				d.point("node "+want.ID, got.Position, want.Position)
				d.point("node "+want.ID+" (at the read of mermaid)", got.Position, rec.Result.Nodes[i].Position)
				d.box("node "+want.ID+" boundingBox", got.BoundingBox, want.BoundingBox)
			}
			for i, want := range rec.After.Edges {
				got := res.Edges[i]
				at := rec.Result.Edges[i]
				d.opt("edge "+want.ID+" startX", got.RScratch.Start.X, at.StartX)
				d.opt("edge "+want.ID+" startY", got.RScratch.Start.Y, at.StartY)
				d.opt("edge "+want.ID+" midX", got.RScratch.Mid.X, at.MidX)
				d.opt("edge "+want.ID+" midY", got.RScratch.Mid.Y, at.MidY)
				d.opt("edge "+want.ID+" endX", got.RScratch.End.X, at.EndX)
				d.opt("edge "+want.ID+" endY", got.RScratch.End.Y, at.EndY)
				d.point("edge "+want.ID+" sourceEndpoint", got.SourceEndpoint, want.SourceEndpoint)
				d.point("edge "+want.ID+" midpoint", got.Midpoint, want.Midpoint)
				d.point("edge "+want.ID+" targetEndpoint", got.TargetEndpoint, want.TargetEndpoint)
			}
			d.report(t)
		})
	}
	if n == 0 {
		t.Fatal("no cose-bilkent record")
	}
}

// TestGoldenDetectsChange is the control of TestGolden. The comparison must
// see a result that differs in the last bit, and the layout must depend on
// its input.
func TestGoldenDetectsChange(t *testing.T) {
	g := loadGolden(t)
	for _, rec := range g.Records {
		if rec.Layout != "cose-bilkent" || len(rec.Result.Edges) < 3 {
			continue
		}
		nodes, edges := coseBilkentInput(t, rec)
		res, err := CoseBilkent(context.Background(), newPageEnv(t, rec).env(), nodes, edges)
		if err != nil {
			t.Fatal(err)
		}
		d := &diff{}
		checkCoseBilkent(rec, res, d)
		if len(d.list) != 0 {
			t.Fatalf("%s: the layout is not the layout of the browser: %v", rec.Name, d.list)
		}
		res.Edges[1].MidX = math.Nextafter(res.Edges[1].MidX, math.Inf(1))
		checkCoseBilkent(rec, res, d)
		if len(d.list) != 1 {
			t.Fatalf("%s: a result that differs in the last bit gave %d differences, want 1", rec.Name, len(d.list))
		}

		nodes[1].Width += 0.5
		res, err = CoseBilkent(context.Background(), newPageEnv(t, rec).env(), nodes, edges)
		if err != nil {
			t.Fatal(err)
		}
		d = &diff{}
		checkCoseBilkent(rec, res, d)
		if len(d.list) == 0 {
			t.Fatalf("%s: a wider node gave the same layout", rec.Name)
		}
		return
	}
	t.Fatal("no record to change")
}
