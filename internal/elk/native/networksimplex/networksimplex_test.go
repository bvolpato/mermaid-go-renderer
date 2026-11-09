package networksimplex

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// goldenCase is one case of testdata/networksimplex.json.gz, which
// tools/reference/gen-elk-networksimplex.mjs writes from the network simplex
// of elkjs.
type goldenCase struct {
	Seed    int          `json:"seed"`
	Kind    string       `json:"kind"`
	N       int          `json:"n"`
	Edges   [][4]float64 `json:"edges"`
	Connect bool         `json:"connect"`
	Balance bool         `json:"balance"`
	Limit   *int         `json:"limit"`
	Prev    []int        `json:"prev"`
	// Again is a second run on the same graph with other edge weights.
	Again *struct {
		Weights []float64 `json:"weights"`
		Balance bool      `json:"balance"`
		Limit   *int      `json:"limit"`
	} `json:"again"`

	// First has the layers after the first run of a case with a second run.
	First  []int   `json:"first"`
	Layers []int   `json:"layers"`
	Order  []int   `json:"order"`
	Out    [][]int `json:"out"`
	In     [][]int `json:"in"`
	Err    *string `json:"err"`
}

func loadGolden(t *testing.T) []*goldenCase {
	t.Helper()
	f, err := os.Open("testdata/networksimplex.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []*goldenCase
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

type goldenResult struct {
	first, layers, order []int
	out, in              [][]int
}

// runGolden builds the graph of a case and runs the algorithm on it. bail is
// the reason of a call of lgraph.Bail.
func runGolden(c *goldenCase) (res goldenResult, bail string) {
	defer func() {
		if r := recover(); r != nil {
			b, ok := r.(*lgraph.BailError)
			if !ok {
				panic(r)
			}
			bail = b.Reason
		}
	}()
	graph := NewNGraph()
	var nodes []*NNode
	index := map[*NNode]int{}
	for i := 0; i < c.N; i++ {
		node := NNodeOf().ID(i).Create(graph)
		index[node] = i
		nodes = append(nodes, node)
	}
	edgeIndex := map[*NEdge]int{}
	for i, e := range c.Edges {
		edge := NEdgeOf().
			Delta(int(e[3])).
			Weight(e[2]).
			Source(nodes[int(e[0])]).
			Target(nodes[int(e[1])]).
			Create()
		edgeIndex[edge] = i
	}
	if c.Connect {
		if root := graph.MakeConnected(); root != nil {
			index[root] = len(nodes)
			nodes = append(nodes, root)
		}
	}
	ns := ForGraph(graph)
	if c.Limit != nil {
		ns.WithIterationLimit(*c.Limit)
	}
	if c.Prev != nil {
		ns.WithPreviousLayering(append([]int(nil), c.Prev...))
	}
	ns.WithBalancing(c.Balance).Execute()
	if c.Again != nil {
		for _, n := range nodes {
			res.first = append(res.first, n.Layer)
		}
		for edge, i := range edgeIndex {
			edge.Weight = c.Again.Weights[i]
		}
		ns = ForGraph(graph)
		if c.Again.Limit != nil {
			ns.WithIterationLimit(*c.Again.Limit)
		}
		ns.WithBalancing(c.Again.Balance).Execute()
	}

	edgeRefs := func(edges []*NEdge) []int {
		out := make([]int, len(edges))
		for i, e := range edges {
			if k, ok := edgeIndex[e]; ok {
				out[i] = k
			} else {
				out[i] = -1
			}
		}
		return out
	}
	res.layers = make([]int, len(nodes))
	res.out = make([][]int, len(nodes))
	res.in = make([][]int, len(nodes))
	for i, n := range nodes {
		res.layers[i] = n.Layer
		res.out[i] = edgeRefs(n.GetOutgoingEdges())
		res.in[i] = edgeRefs(n.GetIncomingEdges())
	}
	res.order = make([]int, len(graph.Nodes))
	for i, n := range graph.Nodes {
		res.order[i] = index[n]
	}
	return res, ""
}

// TestNetworkSimplexGolden compares the layers, the node order and the edge
// lists after a run with the results of elkjs. Some cases run the algorithm
// a second time on the same graph with other weights, as the network simplex
// node placer does.
func TestNetworkSimplexGolden(t *testing.T) {
	cases := loadGolden(t)
	if len(cases) < 500 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	failed, bails, subtrees, again := 0, 0, 0, 0
	for i, c := range cases {
		name := fmt.Sprintf("case %d (seed %d, %s, %d nodes, %d edges)", i, c.Seed, c.Kind, c.N, len(c.Edges))
		res, bail := runGolden(c)
		var problem string
		switch {
		case c.Err != nil:
			bails++
			if bail == "" {
				problem = fmt.Sprintf("elkjs throws %q, the port returns a result", *c.Err)
			}
		case bail != "":
			problem = "the port stops: " + bail
		case c.Again != nil && !reflect.DeepEqual(res.first, c.First):
			problem = fmt.Sprintf("layers after the first run: got %v, want %v", res.first, c.First)
		case !reflect.DeepEqual(res.layers, c.Layers):
			problem = fmt.Sprintf("layers: got %v, want %v", res.layers, c.Layers)
		case !reflect.DeepEqual(res.order, c.Order):
			problem = fmt.Sprintf("node order: got %v, want %v", res.order, c.Order)
		case !equalLists(res.out, c.Out):
			problem = fmt.Sprintf("outgoing edges: got %v, want %v", res.out, c.Out)
		case !equalLists(res.in, c.In):
			problem = fmt.Sprintf("incoming edges: got %v, want %v", res.in, c.In)
		}
		if c.N >= removeSubtreesThresh && c.Err == nil {
			subtrees++
		}
		if c.Again != nil && c.Err == nil {
			again++
		}
		if problem != "" {
			failed++
			if failed <= 5 {
				t.Errorf("%s: %s", name, problem)
			}
		}
	}
	t.Logf("%d cases (%d where elkjs throws, %d with removed subtrees, %d with a second run), %d differ from elkjs", len(cases), bails, subtrees, again, failed)
	if failed > 0 {
		t.Errorf("%d of %d cases differ from elkjs", failed, len(cases))
	}
}

func equalLists(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for k := range a[i] {
			if a[i][k] != b[i][k] {
				return false
			}
		}
	}
	return true
}
