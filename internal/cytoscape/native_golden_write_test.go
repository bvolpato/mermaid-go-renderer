package cytoscape

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/nativetest"
)

type (
	nativeNode = native.Node
	nativeEdge = native.Edge
)

// The golden files of internal/cytoscape/native are written here, because
// the oracle is the JavaScript in the interpreter of this package:
//
//	CYTOSCAPE_WRITE_GOLDENS=1 go test ./internal/cytoscape -run TestWriteNativeGoldens -timeout 0
//
// A case has the input, the result of the JavaScript, the draws of
// Math.random that it took, and one digest for each record of the debug
// hooks (testdata/trace.js). The cases are the layouts of the corpus
// (testdata/layouts.json.gz), the layouts of the documentation corpora
// (native/testdata/docs-inputs.json.gz, which TestRecordDocsInputs writes with
// the build tag recorddocs) and random
// inputs. CYTOSCAPE_GOLDEN_RANDOM sets the number of random inputs.

// goldenSeed is the first seed of the random cases of the golden files. The
// side by side tests start at another seed, so they run inputs that are not
// in the files.
const goldenSeed = 1000000

func goldenRandomCount(def int) int {
	if s := os.Getenv("CYTOSCAPE_GOLDEN_RANDOM"); s != "" {
		var n int
		if _, err := fmt.Sscan(s, &n); err == nil {
			return n
		}
	}
	return def
}

func goldenError(err error) *nativetest.GoldenError {
	var jsErr *Error
	if errors.As(err, &jsErr) {
		return &nativetest.GoldenError{Name: jsErr.Name, Message: jsErr.Message}
	}
	return &nativetest.GoldenError{Name: "Error", Message: err.Error()}
}

// coseBilkentGoldenCase runs the JavaScript with the debug hooks.
func coseBilkentGoldenCase(name string, nodes []nativetest.GoldenNode, edges [][3]string, random func() float64) *nativetest.CoseBilkentCase {
	c := &nativetest.CoseBilkentCase{Name: name, Nodes: nodes, Edges: edges, Draws: []nativetest.Num{}}
	in := make([]Node, len(nodes))
	for i, n := range nodes {
		in[i] = Node{ID: n.ID, Width: float64(n.W), Height: float64(n.H), X: float64(n.X), Y: float64(n.Y)}
	}
	inEdges := make([]Edge, len(edges))
	for i, e := range edges {
		inEdges[i] = Edge{ID: e[0], Start: e[1], End: e[2]}
	}
	tracer := &nativetest.Tracer{RandomCalls: func() int { return len(c.Draws) }}
	env := Env{Now: time.Unix(0, 0), Random: func() float64 {
		v := random()
		c.Draws = append(c.Draws, nativetest.Num(v))
		return v
	}}
	res, err := coseBilkent(context.Background(), env, runtimeOptions{patch: tracePatch(tracer.Trace)}, in, inEdges)
	if err != nil {
		c.Error = goldenError(err)
	} else {
		tracer.Trace("cy.result", coseBilkentResultValues(res))
		c.Result = &nativetest.CoseBilkentGolden{}
		for _, n := range res.Nodes {
			c.Result.Nodes = append(c.Result.Nodes, [2]nativetest.Num{nativetest.Num(n.X), nativetest.Num(n.Y)})
		}
		for _, e := range res.Edges {
			c.Result.Edges = append(c.Result.Edges, [6]nativetest.Num{
				nativetest.Num(e.StartX), nativetest.Num(e.StartY), nativetest.Num(e.MidX), nativetest.Num(e.MidY), nativetest.Num(e.EndX), nativetest.Num(e.EndY),
			})
		}
	}
	c.Trace = tracer.Records
	return c
}

func loadDocsInputs(t *testing.T) *nativetest.DocsInputs {
	docs := &nativetest.DocsInputs{}
	if err := nativetest.ReadGzipJSON(filepath.Join("native", "testdata", "docs-inputs.json.gz"), docs); err != nil {
		t.Fatalf("the inputs of the documentation corpora: %v (run TestRecordDocsInputs with -tags recorddocs)", err)
	}
	return docs
}

func TestWriteNativeGoldens(t *testing.T) {
	which := os.Getenv("CYTOSCAPE_WRITE_GOLDENS")
	if which == "" {
		t.Skip("set CYTOSCAPE_WRITE_GOLDENS=1 (or cose-bilkent, or fcose) to write the golden files of native/testdata")
	}
	docs := loadDocsInputs(t)
	if which == "1" || which == "cose-bilkent" {
		file := &nativetest.CoseBilkentFile{}
		for _, rec := range loadGolden(t).Records {
			if rec.Layout != "cose-bilkent" {
				continue
			}
			nodes, edges := coseBilkentInput(t, rec)
			gn := make([]nativetest.GoldenNode, len(nodes))
			for i, n := range nodes {
				gn[i] = nativetest.GoldenNode{ID: n.ID, W: nativetest.Num(n.Width), H: nativetest.Num(n.Height), X: nativetest.Num(n.X), Y: nativetest.Num(n.Y)}
			}
			ge := make([][3]string, len(edges))
			for i, e := range edges {
				ge[i] = [3]string{e.ID, e.Start, e.End}
			}
			c := coseBilkentGoldenCase("corpus/"+rec.Name, gn, ge, nativetest.NewMulberry32(nativetest.PageSeed, rec.RandomBefore).Random)
			// The case must be the layout of Chromium.
			res := &CoseBilkentResult{}
			for i, n := range c.Result.Nodes {
				res.Nodes = append(res.Nodes, PositionedNode{ID: nodes[i].ID, X: float64(n[0]), Y: float64(n[1])})
			}
			for i, e := range c.Result.Edges {
				res.Edges = append(res.Edges, PositionedEdge{ID: edges[i].ID, StartX: float64(e[0]), StartY: float64(e[1]), MidX: float64(e[2]), MidY: float64(e[3]), EndX: float64(e[4]), EndY: float64(e[5])})
			}
			d := &diff{}
			checkCoseBilkent(rec, res, d)
			if len(d.list) > 0 {
				t.Fatalf("%s: the JavaScript with the debug hooks is not the layout of Chromium: %s", rec.Name, d.list[0])
			}
			file.Cases = append(file.Cases, c)
		}
		for _, d := range docs.CoseBilkent {
			file.Cases = append(file.Cases, coseBilkentGoldenCase("docs/"+d.Name, d.Nodes, d.Edges, nativetest.NewMulberry32(nativetest.PageSeed, 1).Random))
		}
		n := goldenRandomCount(240)
		for i := 0; i < n; i++ {
			seed := uint32(goldenSeed + i)
			m := nativetest.RandomMindmap(seed)
			// A large input costs the most bytes: the golden files have few
			// of them, and the side by side tests have more.
			if len(m.Nodes) > 60 && i%4 != 0 {
				m.Nodes, m.Edges = truncateMindmap(m, 60)
			}
			gn := make([]nativetest.GoldenNode, len(m.Nodes))
			for k, x := range m.Nodes {
				gn[k] = nativetest.GoldenNode{ID: x.ID, W: nativetest.Num(x.Width), H: nativetest.Num(x.Height), X: nativetest.Num(x.X), Y: nativetest.Num(x.Y)}
			}
			ge := make([][3]string, len(m.Edges))
			for k, x := range m.Edges {
				ge[k] = [3]string{x.ID, x.Start, x.End}
			}
			file.Cases = append(file.Cases, coseBilkentGoldenCase(fmt.Sprintf("random/%s-%d", m.Kind, seed), gn, ge, nativetest.NewMulberry32(seed, 0).Random))
		}
		path := filepath.Join("native", "testdata", "cosebilkent.json.gz")
		if err := nativetest.WriteGzipJSON(path, file); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		t.Logf("%s: %d cases, %d bytes", path, len(file.Cases), info.Size())
	}
}

// truncateMindmap keeps the first n nodes of a tree or forest, and the edges
// between them.
func truncateMindmap(m nativetest.Mindmap, n int) ([]nativeNode, []nativeEdge) {
	keep := map[string]bool{}
	for _, x := range m.Nodes[:n] {
		keep[x.ID] = true
	}
	var edges []nativeEdge
	for _, e := range m.Edges {
		if keep[e.Start] && keep[e.End] {
			edges = append(edges, e)
		}
	}
	return m.Nodes[:n], edges
}

// architectureGoldenCase runs the JavaScript with the debug hooks.
func architectureGoldenCase(name string, in *native.ArchitectureInput, random func() float64, measure func(font, text string) float64) *nativetest.ArchitectureCase {
	c := &nativetest.ArchitectureCase{Name: name, Input: *in, Draws: []nativetest.Num{}, Measure: []nativetest.Measure{}}
	tracer := &nativetest.Tracer{RandomCalls: func() int { return len(c.Draws) }}
	env := Env{
		Now: time.Unix(0, 0),
		Random: func() float64 {
			v := random()
			c.Draws = append(c.Draws, nativetest.Num(v))
			return v
		},
		MeasureText: func(font, text string) float64 {
			w := measure(font, text)
			c.Measure = append(c.Measure, nativetest.Measure{Font: font, Text: text, Width: nativetest.Num(w)})
			return w
		},
	}
	res, err := architecture(context.Background(), env, runtimeOptions{patch: tracePatch(tracer.Trace)}, fromNativeInput(in))
	if err != nil {
		c.Error = goldenError(err)
	} else {
		tracer.Trace("cy.result", architectureResultValues(res))
		c.Result = &nativetest.ArchitectureGolden{Edges: [][6]nativetest.Num{}}
		for _, n := range res.Nodes {
			if n.Type == "group" {
				b := n.BoundingBox
				c.Result.Nodes = append(c.Result.Nodes, []nativetest.Num{nativetest.Num(b.X1), nativetest.Num(b.Y1), nativetest.Num(b.X2), nativetest.Num(b.Y2), nativetest.Num(b.W), nativetest.Num(b.H)})
			} else {
				c.Result.Nodes = append(c.Result.Nodes, []nativetest.Num{nativetest.Num(n.X), nativetest.Num(n.Y)})
			}
		}
		for _, e := range res.Edges {
			c.Result.Edges = append(c.Result.Edges, [6]nativetest.Num{
				nativetest.Num(e.SourceEndpoint.X), nativetest.Num(e.SourceEndpoint.Y), nativetest.Num(e.Midpoint.X), nativetest.Num(e.Midpoint.Y), nativetest.Num(e.TargetEndpoint.X), nativetest.Num(e.TargetEndpoint.Y),
			})
		}
	}
	c.Trace = tracer.Records
	return c
}

// TestWriteNativeGoldensFcose writes native/testdata/fcose.json.gz. See
// TestWriteNativeGoldens.
func TestWriteNativeGoldensFcose(t *testing.T) {
	which := os.Getenv("CYTOSCAPE_WRITE_GOLDENS")
	if which != "1" && which != "fcose" {
		t.Skip("set CYTOSCAPE_WRITE_GOLDENS=1 (or fcose) to write native/testdata/fcose.json.gz")
	}
	docs := loadDocsInputs(t)
	file := &nativetest.ArchitectureFile{}
	for _, rec := range loadGolden(t).Records {
		if rec.Layout != "fcose" {
			continue
		}
		page := newPageEnv(t, rec)
		env := page.env()
		c := architectureGoldenCase("corpus/"+rec.Name, nativeInput(architectureInput(t, rec)), env.Random, env.MeasureText)
		// The case must be the layout of Chromium.
		res, err := architecture(context.Background(), newPageEnv(t, rec).env(), runtimeOptions{}, architectureInput(t, rec))
		if err != nil {
			t.Fatal(err)
		}
		d := &diff{}
		checkArchitecture(rec, res, d)
		if len(d.list) > 0 || c.Result == nil {
			t.Fatalf("%s: the JavaScript is not the layout of Chromium", rec.Name)
		}
		for i, e := range res.Edges {
			if float64(c.Result.Edges[i][0]) != e.SourceEndpoint.X && e.SourceEndpoint.X == e.SourceEndpoint.X {
				t.Fatalf("%s: the JavaScript with the debug hooks gives another layout", rec.Name)
			}
		}
		file.Cases = append(file.Cases, c)
	}
	for _, d := range docs.Architecture {
		var calls []nativetest.Measure
		in := d.Input
		file.Cases = append(file.Cases, architectureGoldenCase("docs/"+d.Name, &in, nativetest.NewMulberry32(nativetest.PageSeed, 1).Random, nativetest.MeasureReplay(d.Measure, &calls)))
	}
	n := goldenRandomCount(160)
	for i := 0; i < n; i++ {
		seed := uint32(goldenSeed + i)
		a := nativetest.RandomArchitecture(seed)
		file.Cases = append(file.Cases, architectureGoldenCase(fmt.Sprintf("random/%s-%d", a.Kind, seed), &a.Input, nativetest.NewMulberry32(seed, 0).Random, nativetest.FakeMeasure))
	}
	path := filepath.Join("native", "testdata", "fcose.json.gz")
	if err := nativetest.WriteGzipJSON(path, file); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	t.Logf("%s: %d cases, %d bytes", path, len(file.Cases), info.Size())
}
