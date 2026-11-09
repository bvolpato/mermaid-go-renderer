package architecture

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

// The golden file testdata/layout.json.gz is written by
// tools/reference/gen-architecture-layout.mjs. It has one record for each
// fixture of the corpus and for each hand-written input: what the renderer
// of real mermaid gives to cytoscape in Chromium (the style sheet, the
// cy.add calls, the layout options), what the canvas measures for the
// labels, what the renderer reads back after the layout, and the SVG that
// mermaid.render returns in a new page. See
// tools/reference/probes/cytoscape.js for the fields.

// lnum is a number of the golden file. A number that JSON cannot hold is
// { "$num": "NaN" }, "Infinity", "-Infinity" or "-0".
type lnum float64

func (n *lnum) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '{' {
		var o struct {
			Num string `json:"$num"`
		}
		if err := json.Unmarshal(data, &o); err != nil {
			return err
		}
		switch o.Num {
		case "NaN":
			*n = lnum(math.NaN())
		case "Infinity":
			*n = lnum(math.Inf(1))
		case "-Infinity":
			*n = lnum(math.Inf(-1))
		case "-0":
			*n = lnum(math.Copysign(0, -1))
		default:
			return fmt.Errorf("unknown $num %q", o.Num)
		}
		return nil
	}
	var f float64
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	*n = lnum(f)
	return nil
}

type lpoint struct {
	X lnum `json:"x"`
	Y lnum `json:"y"`
}

type lbox struct {
	X1 lnum `json:"x1"`
	Y1 lnum `json:"y1"`
	X2 lnum `json:"x2"`
	Y2 lnum `json:"y2"`
	W  lnum `json:"w"`
	H  lnum `json:"h"`
}

type layoutRecord struct {
	Name   string `json:"name"`
	Text   string `json:"text"`
	Corpus bool   `json:"corpus"`
	// Error is the error of mermaid for a text that it does not render.
	Error string `json:"error"`
	// SameSvg tells that the render with the hooks of the probe gave the
	// reference SVG.
	SameSvg bool `json:"sameSvg"`
	// ProbeError is the error of the probe for a case that mermaid renders.
	// Such a record has the SVG and no layout.
	ProbeError string `json:"probeError"`
	// SVG is the result of mermaid.render in a new page.
	SVG   string `json:"svg"`
	Style []struct {
		Selector string            `json:"selector"`
		Style    map[string]string `json:"style"`
	} `json:"style"`
	Adds []struct {
		Group   string         `json:"group"`
		Data    map[string]any `json:"data"`
		Classes string         `json:"classes"`
	} `json:"adds"`
	Layouts []struct {
		Options struct {
			Name                        string `json:"name"`
			Quality                     string `json:"quality"`
			Randomize                   any    `json:"randomize"`
			NodeSeparation              lnum   `json:"nodeSeparation"`
			NumIter                     lnum   `json:"numIter"`
			StyleEnabled                bool   `json:"styleEnabled"`
			Animate                     bool   `json:"animate"`
			NodeDimensionsIncludeLabels bool   `json:"nodeDimensionsIncludeLabels"`
			IdealEdgeLength             struct {
				Fn map[string]lnum `json:"$fn"`
			} `json:"idealEdgeLength"`
			EdgeElasticity struct {
				Fn map[string]lnum `json:"$fn"`
			} `json:"edgeElasticity"`
			Alignment struct {
				Horizontal [][]string `json:"horizontal"`
				Vertical   [][]string `json:"vertical"`
			} `json:"alignmentConstraint"`
			Relative []map[string]any `json:"relativePlacementConstraint"`
		} `json:"options"`
		Runs int `json:"runs"`
	} `json:"layouts"`
	// Measure has the measureText calls of the canvas: the font that
	// cytoscape assigns, the text, the width.
	Measure [][]any `json:"measure"`
	// Random is the number of Math.random calls of the page generator
	// during the layout.
	Random int `json:"random"`
	Result struct {
		Edges []struct {
			ID             string `json:"id"`
			SourceEndpoint lpoint `json:"sourceEndpoint"`
			Midpoint       lpoint `json:"midpoint"`
			TargetEndpoint lpoint `json:"targetEndpoint"`
			HasRscratch    bool   `json:"hasRscratch"`
		} `json:"edges"`
		Nodes []struct {
			ID          string `json:"id"`
			Position    lpoint `json:"position"`
			BoundingBox *lbox  `json:"boundingBox"`
		} `json:"nodes"`
	} `json:"result"`
}

func loadLayoutRecords(t testing.TB) []*layoutRecord {
	var file struct {
		Records []*layoutRecord `json:"records"`
	}
	loadGzipJSON(t, "testdata/layout.json.gz", &file)
	return file.Records
}

// layoutDiff collects the differences of one record.
type layoutDiff struct {
	list []string
}

func (d *layoutDiff) errorf(format string, args ...any) {
	d.list = append(d.list, fmt.Sprintf(format, args...))
}

func (d *layoutDiff) num(what string, got float64, want lnum) {
	if math.Float64bits(got) != math.Float64bits(float64(want)) && !(math.IsNaN(got) && math.IsNaN(float64(want))) {
		d.errorf("%s: got %v (%#x), want %v (%#x)", what, got, math.Float64bits(got), float64(want), math.Float64bits(float64(want)))
	}
}

func (d *layoutDiff) point(what string, got cytoscape.Point, want lpoint) {
	d.num(what+".x", got.X, want.X)
	d.num(what+".y", got.Y, want.Y)
}

func (d *layoutDiff) report(t *testing.T) {
	t.Helper()
	for i, s := range d.list {
		if i == 10 {
			t.Errorf("and %d more differences", len(d.list)-i)
			break
		}
		t.Error(s)
	}
}

// endpointOf is the value of sourceEndpoint and targetEndpoint of addEdges.
func endpointOf(dir string) string {
	switch dir {
	case "L":
		return "0 50%"
	case "R":
		return "100% 50%"
	case "T":
		return "50% 0"
	}
	return "50% 100%"
}

// expectedAdds builds the arguments of the cy.add calls of addGroups,
// addServices, addJunctions and addEdges from the input that the port gives
// to cytoscape, in the form of the golden file. The four flags of an edge
// are not a part of that input, because the layout does not read them: they
// come from the db.
//
// The input of the port has no label for a title that is the empty string,
// where upstream has the label "". checkInput counts those.
func expectedAdds(in *cytoscape.ArchitectureInput, db *ArchitectureDB) []map[string]any {
	set := func(m map[string]any, key, value string) {
		if value != "" {
			m[key] = value
		}
	}
	adds := []map[string]any{}
	for _, g := range in.Groups {
		data := map[string]any{"type": "group", "id": g.ID}
		set(data, "icon", g.Icon)
		set(data, "label", g.Title)
		set(data, "parent", g.In)
		adds = append(adds, map[string]any{"group": "nodes", "data": data, "classes": "node-group"})
	}
	for _, s := range in.Services {
		data := map[string]any{"type": "service", "id": s.ID, "width": in.Config.IconSize, "height": in.Config.IconSize}
		set(data, "icon", s.Icon)
		set(data, "label", s.Title)
		set(data, "parent", s.In)
		adds = append(adds, map[string]any{"group": "nodes", "data": data, "classes": "node-service"})
	}
	for _, j := range in.Junctions {
		data := map[string]any{"type": "junction", "id": j.ID, "width": in.Config.IconSize, "height": in.Config.IconSize}
		set(data, "parent", j.In)
		adds = append(adds, map[string]any{"group": "nodes", "data": data, "classes": "node-junction"})
	}
	parsed := db.GetEdges()
	for i, e := range in.Edges {
		edgeType := "straight"
		if isArchitectureDirectionXY(e.LhsDir, e.RhsDir) {
			edgeType = "segments"
		}
		data := map[string]any{
			"id":             e.LhsID + "-" + e.RhsID,
			"source":         e.LhsID,
			"sourceDir":      e.LhsDir,
			"sourceArrow":    parsed[i].LhsInto,
			"sourceGroup":    parsed[i].LhsGroup,
			"sourceEndpoint": endpointOf(e.LhsDir),
			"target":         e.RhsID,
			"targetDir":      e.RhsDir,
			"targetArrow":    parsed[i].RhsInto,
			"targetGroup":    parsed[i].RhsGroup,
			"targetEndpoint": endpointOf(e.RhsDir),
		}
		set(data, "label", e.Title)
		adds = append(adds, map[string]any{"group": "edges", "data": data, "classes": edgeType})
	}
	return adds
}

// checkInput compares what the port gives to cytoscape with what mermaid
// gives to it: the elements, the two style values that come from the
// config, and the options of the fcose layout.
func checkInput(rec *layoutRecord, o *observed, d *layoutDiff, emptyLabels *int) {
	in := o.observer.input
	if in == nil {
		d.errorf("the port did not call the layout")
		return
	}

	// The elements.
	want := make([]map[string]any, len(rec.Adds))
	for i, a := range rec.Adds {
		data := map[string]any{}
		for k, v := range a.Data {
			if k == "label" && v == "" {
				// See expectedAdds.
				*emptyLabels++
				continue
			}
			if k == "width" || k == "height" {
				// The size is the iconSize of the config as it is. The port
				// gives cytoscape the number that it converts to.
				v = js.ToNumber(v)
			}
			data[k] = v
		}
		want[i] = map[string]any{"group": a.Group, "data": data, "classes": a.Classes}
	}
	got := expectedAdds(in, o.db)
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.Marshal(got)
		wj, _ := json.Marshal(want)
		d.errorf("the elements differ\n got: %s\nwant: %s", gj, wj)
	}

	// The style values.
	for _, s := range rec.Style {
		switch s.Selector {
		// cytoscape leaves out a value that it does not accept, for example
		// a negative padding.
		case "node[label]":
			if want, ok := s.Style["font-size"]; ok && js.Str(in.Config.FontSize)+"px" != want {
				d.errorf("font-size %q, want %q", js.Str(in.Config.FontSize)+"px", want)
			}
		case ".node-group":
			if want, ok := s.Style["padding"]; ok && js.Str(in.Config.Padding)+"px" != want {
				d.errorf("padding %q, want %q", js.Str(in.Config.Padding)+"px", want)
			}
		}
	}

	// The layout options.
	if len(rec.Layouts) != 1 {
		d.errorf("%d layouts in the record", len(rec.Layouts))
		return
	}
	if rec.Layouts[0].Runs != 2 {
		d.errorf("the layout of the record ran %d times, and the port runs it 2 times", rec.Layouts[0].Runs)
	}
	opts := rec.Layouts[0].Options
	if opts.Name != "fcose" || opts.Quality != "proof" || opts.StyleEnabled || opts.Animate || opts.NodeDimensionsIncludeLabels {
		d.errorf("the fixed options of the record are not the options of the port")
	}
	if in.Config.Randomize != js.Truthy(opts.Randomize) {
		d.errorf("randomize %v, want %v", in.Config.Randomize, opts.Randomize)
	}
	d.num("nodeSeparation", in.Config.NodeSeparation, opts.NodeSeparation)
	d.num("numIter", in.Config.NumIter, opts.NumIter)

	// idealEdgeLength and edgeElasticity are functions of the edge: the
	// record has their values for each edge. The port gives the config
	// values and the parents to the same functions.
	parent := map[string]string{}
	for _, list := range [][]cytoscape.ArchitectureNode{in.Groups, in.Services, in.Junctions} {
		for _, n := range list {
			parent[n.ID] = n.In
		}
	}
	sameGroupIdealLength := in.Config.IdealEdgeLengthMultiplier * in.Config.IconSize
	crossGroupIdealLength := 0.5 * in.Config.IconSize
	for _, e := range in.Edges {
		id := e.LhsID + "-" + e.RhsID
		length, elasticity := crossGroupIdealLength, 0.001
		if parent[e.LhsID] == parent[e.RhsID] {
			length, elasticity = sameGroupIdealLength, in.Config.EdgeElasticity
		}
		if want, ok := opts.IdealEdgeLength.Fn[id]; !ok {
			d.errorf("the record has no idealEdgeLength for the edge %s", id)
		} else {
			d.num("idealEdgeLength of "+id, length, want)
		}
		if want, ok := opts.EdgeElasticity.Fn[id]; !ok {
			d.errorf("the record has no edgeElasticity for the edge %s", id)
		} else {
			d.num("edgeElasticity of "+id, elasticity, want)
		}
	}

	// The constraints, order included.
	if !reflect.DeepEqual(in.Alignment.Horizontal, opts.Alignment.Horizontal) {
		d.errorf("alignmentConstraint.horizontal %v, want %v", in.Alignment.Horizontal, opts.Alignment.Horizontal)
	}
	if !reflect.DeepEqual(in.Alignment.Vertical, opts.Alignment.Vertical) {
		d.errorf("alignmentConstraint.vertical %v, want %v", in.Alignment.Vertical, opts.Alignment.Vertical)
	}
	if len(in.RelativePlacement) != len(opts.Relative) {
		d.errorf("%d relative placements, want %d", len(in.RelativePlacement), len(opts.Relative))
		return
	}
	for i, p := range in.RelativePlacement {
		got := map[string]any{"gap": p.Gap}
		if p.Left != "" || p.Right != "" {
			got["left"], got["right"] = p.Left, p.Right
		}
		if p.Top != "" || p.Bottom != "" {
			got["top"], got["bottom"] = p.Top, p.Bottom
		}
		if !reflect.DeepEqual(got, opts.Relative[i]) {
			d.errorf("relative placement %d: %v, want %v", i, got, opts.Relative[i])
		}
	}
}

// checkMeasure compares the label widths that the port measures for
// cytoscape with the widths of the canvas of Chromium, bit for bit.
func checkMeasure(rec *layoutRecord, o *observed, d *layoutDiff) int {
	if len(o.measure) != len(rec.Measure) {
		d.errorf("%d measureText calls, want %d", len(o.measure), len(rec.Measure))
		return 0
	}
	for i, m := range rec.Measure {
		got := o.measure[i]
		font, _ := m[0].(string)
		text, _ := m[1].(string)
		width, _ := m[2].(float64)
		if got.font != font || got.text != text {
			d.errorf("measureText call %d: font %q text %q, want font %q text %q", i, got.font, got.text, font, text)
			continue
		}
		if math.Float64bits(got.width) != math.Float64bits(width) {
			d.errorf("measureText(%q) with font %q: %v, want %v", text, font, got.width, width)
		}
	}
	return len(rec.Measure)
}

// checkResult compares what the port reads from cytoscape after the layout
// with what mermaid reads, bit for bit.
func checkResult(rec *layoutRecord, res *cytoscape.ArchitectureResult, random int, d *layoutDiff) {
	if res == nil {
		d.errorf("the layout of the port has no result")
		return
	}
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
			bb := got.BoundingBox
			d.num("node "+want.ID+" boundingBox.x1", bb.X1, want.BoundingBox.X1)
			d.num("node "+want.ID+" boundingBox.y1", bb.Y1, want.BoundingBox.Y1)
			d.num("node "+want.ID+" boundingBox.x2", bb.X2, want.BoundingBox.X2)
			d.num("node "+want.ID+" boundingBox.y2", bb.Y2, want.BoundingBox.Y2)
			d.num("node "+want.ID+" boundingBox.w", bb.W, want.BoundingBox.W)
			d.num("node "+want.ID+" boundingBox.h", bb.H, want.BoundingBox.H)
		} else {
			d.point("node "+want.ID, cytoscape.Point{X: got.X, Y: got.Y}, want.Position)
		}
	}
	if random != rec.Random {
		d.errorf("%d Math.random calls of the layout, want %d", random, rec.Random)
	}
}

// TestLayoutGolden renders each text of the golden file and compares the
// whole pipeline with mermaid in Chromium: the input of cytoscape, the label
// widths, the output of cytoscape bit for bit, and the SVG.
//
// All of these have the label widths of Chromium on the macOS reference
// machine. A machine with other fonts compares only the errors: it must fail
// where mermaid fails, and render where mermaid renders. Then the test is
// marked as skipped.
func TestLayoutGolden(t *testing.T) {
	records := loadLayoutRecords(t)
	if len(records) < 100 {
		t.Fatalf("only %d records", len(records))
	}
	reference := fonttest.IsMacOSReference()
	var layouts, layoutsOK, measures, svgs, exact, equivalent, known, errs, errsOK, emptyLabels atomic.Int64
	t.Run("records", func(t *testing.T) {
		for _, rec := range records {
			if testing.Short() && !rec.Corpus {
				continue
			}
			t.Run(rec.Name, func(t *testing.T) {
				t.Parallel()
				o := renderObserved(t, rec.Text, nil)
				err := o.err
				if rec.Error != "" {
					errs.Add(1)
					if err == nil {
						t.Fatalf("mermaid fails with %q, the port renders\ninput: %q", rec.Error, rec.Text)
					}
					if err.Error() != rec.Error && !engineTypeError(err, rec.Error) {
						t.Fatalf("error %q, want %q\ninput: %q", err, rec.Error, rec.Text)
					}
					errsOK.Add(1)
					return
				}
				if err != nil {
					t.Fatalf("the port fails: %v\ninput: %q", err, rec.Text)
				}
				if !reference {
					return
				}

				// The SVG.
				if reason := knownDivergence(rec.Text); reason != "" {
					known.Add(1)
					t.Logf("the SVG is not compared: %s", reason)
				} else {
					svgs.Add(1)
					switch verdict, diff := parity.CompareSVG(o.svg, rec.SVG, 0.01); verdict {
					case parity.Exact:
						exact.Add(1)
					case parity.Equivalent:
						equivalent.Add(1)
						t.Logf("the SVG is equivalent, not exact (largest difference %v)", diff.MaxDelta)
					default:
						t.Errorf("the SVG differs: %v\ninput: %q", diff, rec.Text)
					}
				}

				if rec.ProbeError != "" {
					return
				}
				if !rec.SameSvg {
					t.Fatal("the record is not from a render that gave the reference SVG")
				}
				layouts.Add(1)
				d := &layoutDiff{}
				empty := 0
				checkInput(rec, o, d, &empty)
				emptyLabels.Add(int64(empty))
				measures.Add(int64(checkMeasure(rec, o, d)))
				checkResult(rec, o.observer.result, o.random, d)
				if len(d.list) == 0 {
					layoutsOK.Add(1)
				} else {
					t.Logf("input: %q", rec.Text)
				}
				d.report(t)
			})
		}
	})
	t.Logf("%d records: %d of %d layouts bit-exact (input, %d label widths, output); SVG: %d exact, %d equivalent, %d different, %d not compared (known divergence); %d of %d errors of mermaid agree; %d labels that are the empty string are passed as no label",
		len(records), layoutsOK.Load(), layouts.Load(), measures.Load(), exact.Load(), equivalent.Load(), svgs.Load()-exact.Load()-equivalent.Load(), known.Load(), errsOK.Load(), errs.Load(), emptyLabels.Load())
	fonttest.RequireMacOSReference(t)
}

// TestLayoutGoldenDetectsChange is the control of TestLayoutGolden: the
// comparison must see a result that differs in the last bit.
func TestLayoutGoldenDetectsChange(t *testing.T) {
	for _, rec := range loadLayoutRecords(t) {
		if rec.Error != "" || rec.ProbeError != "" || len(rec.Result.Edges) < 2 || !strings.Contains(rec.Text, "group") {
			continue
		}
		o := renderObserved(t, rec.Text, nil)
		if o.err != nil {
			t.Fatal(o.err)
		}
		d := &layoutDiff{}
		checkResult(rec, o.observer.result, o.random, d)
		if len(d.list) != 0 {
			// The layout has the sizes of the labels. With other fonts it
			// can differ from the layout of the browser, and then there is
			// no result to change.
			fonttest.RequireMacOSReference(t)
			t.Fatalf("%s: the layout is not the layout of the browser: %v", rec.Name, d.list)
		}
		// A copy: other tests read the result of the render.
		changed := *o.observer.result
		changed.Edges = append([]cytoscape.ArchitectureEdgeResult(nil), changed.Edges...)
		e := &changed.Edges[1]
		e.Midpoint.X = math.Nextafter(e.Midpoint.X, math.Inf(1))
		checkResult(rec, &changed, o.random, d)
		if len(d.list) != 1 {
			t.Fatalf("%s: a result that differs in the last bit gave %d differences, want 1", rec.Name, len(d.list))
		}
		return
	}
	t.Fatal("no record to change")
}
