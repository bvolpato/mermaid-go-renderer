package alg

import (
	"compress/gzip"
	"io"
	"math"
	"os"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file comes from tools/reference/gen-graphlib.mjs. Each case
// holds a graph in the JSON form of graphlib and the results of the alg
// functions of dagre-d3-es on that graph.

func loadGolden(t *testing.T) []any {
	t.Helper()
	f, err := os.Open("testdata/alg.json.gz")
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
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	cases, _ := js.Path(v, "cases").([]any)
	return cases
}

func strList(list []string) any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

func strLists(lists [][]string) any {
	out := make([]any, len(lists))
	for i, l := range lists {
		out[i] = strList(l)
	}
	return out
}

func strs(v any) []string {
	a, _ := v.([]any)
	out := make([]string, len(a))
	for i, x := range a {
		out[i], _ = x.(string)
	}
	return out
}

func dist(x float64) any {
	switch {
	case math.IsInf(x, 1):
		return "Infinity"
	case math.IsInf(x, -1):
		return "-Infinity"
	case math.IsNaN(x):
		return "NaN"
	}
	return x
}

func pathResult(res *js.Obj[*PathEntry]) any {
	out := []any{}
	res.Each(func(v string, e *PathEntry) {
		var pred any = js.Null
		if e.HasPredecessor {
			pred = e.Predecessor
		}
		out = append(out, []any{v, dist(e.Distance), pred})
	})
	return out
}

func allPathsResult(res *js.Obj[*js.Obj[*PathEntry]]) any {
	out := []any{}
	res.Each(func(v string, r *js.Obj[*PathEntry]) {
		out = append(out, []any{v, pathResult(r)})
	})
	return out
}

func edgeList(list []graphlib.Edge) any {
	out := make([]any, len(list))
	for i, e := range list {
		var name any = js.Null
		if e.Name != "" {
			name = e.Name
		}
		out[i] = []any{e.V, e.W, name}
	}
	return out
}

func TestAlgGolden(t *testing.T) {
	cases := loadGolden(t)
	failed := 0
	for _, cv := range cases {
		c := js.AsObj(cv)
		name, _ := c.Get("name").(string)
		written, _ := c.Get("written").(string)
		parsed, err := js.ParseJSON([]byte(written))
		if err != nil {
			t.Fatal(err)
		}
		g, err := graphlib.Read(parsed)
		if err != nil {
			t.Fatal(err)
		}
		weightFn := func(e graphlib.Edge) float64 {
			w, _ := g.EdgeObj(e).(float64)
			return w
		}
		undirectedEdges := func(v string) []graphlib.Edge { return g.NodeEdges(v) }
		source, _ := c.Get("source").(string)
		starts := strs(c.Get("starts"))
		nodes := g.Nodes()

		ok := true
		// check compares a value with the golden value of a key.
		check := func(key string, got any) {
			t.Helper()
			if g, w := js.StringifyJSON(got), js.StringifyJSON(c.Get(key)); g != w {
				ok = false
				t.Errorf("%s: %s differs\n  got:  %.500s\n  want: %.500s", name, key, g, w)
			}
		}
		// attempt compares the result or the failure of a function that
		// throws in JavaScript. wantMessage is true when the error message
		// must also match.
		attempt := func(key string, got any, err error, wantMessage bool) {
			t.Helper()
			want := js.AsObj(c.Get(key))
			wantErr, failed := want.Lookup("error")
			switch {
			case failed && err == nil:
				ok = false
				t.Errorf("%s: %s: no error, JavaScript throws %q", name, key, wantErr)
			case !failed && err != nil:
				ok = false
				t.Errorf("%s: %s: error %v, JavaScript does not throw", name, key, err)
			case failed:
				if wantMessage && err.Error() != wantErr {
					ok = false
					t.Errorf("%s: %s: error %q, want %q", name, key, err.Error(), wantErr)
				}
			default:
				if g, w := js.StringifyJSON(got), js.StringifyJSON(want.Get("result")); g != w {
					ok = false
					t.Errorf("%s: %s differs\n  got:  %.500s\n  want: %.500s", name, key, g, w)
				}
			}
		}

		check("components", strLists(Components(g)))
		check("tarjan", strLists(Tarjan(g)))
		check("findCycles", strLists(FindCycles(g)))
		check("isAcyclic", IsAcyclic(g))

		pre, err := Preorder(g, starts)
		attempt("preorder", strList(pre), err, true)
		post, err := Postorder(g, starts)
		attempt("postorder", strList(post), err, true)
		pre, err = Preorder(g, nodes)
		attempt("preorderAll", strList(pre), err, true)
		post, err = Postorder(g, nodes)
		attempt("postorderAll", strList(post), err, true)
		pre, err = Preorder(g, append(append([]string{}, starts...), "missing"))
		attempt("preorderMissing", strList(pre), err, true)

		sorted, err := Topsort(g)
		attempt("topsort", strList(sorted), err, false)

		res, err := Dijkstra(g, source, weightFn, nil)
		if err == nil {
			attempt("dijkstra", pathResult(res), nil, false)
		} else {
			attempt("dijkstra", nil, err, false)
		}
		res, err = Dijkstra(g, source, nil, nil)
		if err == nil {
			attempt("dijkstraDefault", pathResult(res), nil, false)
		} else {
			attempt("dijkstraDefault", nil, err, false)
		}
		res, err = Dijkstra(g, source, weightFn, undirectedEdges)
		if err == nil {
			attempt("dijkstraUndirected", pathResult(res), nil, false)
		} else {
			attempt("dijkstraUndirected", nil, err, false)
		}
		all, err := DijkstraAll(g, weightFn, nil)
		if err == nil {
			attempt("dijkstraAll", allPathsResult(all), nil, false)
		} else {
			attempt("dijkstraAll", nil, err, false)
		}
		attempt("floydWarshall", allPathsResult(FloydWarshall(g, weightFn, nil)), nil, false)

		tree, err := Prim(g, weightFn)
		if err == nil {
			o := js.NewObj[any]()
			o.Set("nodes", strList(tree.Nodes()))
			o.Set("edges", edgeList(tree.Edges()))
			attempt("prim", o, nil, false)
		} else {
			attempt("prim", nil, err, false)
		}

		if !ok {
			failed++
			if failed >= 5 {
				t.Fatalf("stop after %d failed cases", failed)
			}
		}
	}
	t.Logf("%d alg cases, %d failed", len(cases), failed)
	if len(cases) < 100 {
		t.Errorf("only %d cases", len(cases))
	}
}
