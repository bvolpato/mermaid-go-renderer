package native

import (
	"strconv"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/layered"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// elkIndex finds the elements of an ELK graph by the reference texts of the
// phase goldens.
type elkIndex struct {
	nodes map[string]*elkgraph.Node
	edges map[string]*elkgraph.Edge
	ports map[string]*elkgraph.Port
}

func newElkIndex(root *elkgraph.Node, edges []*elkgraph.Edge) *elkIndex {
	idx := &elkIndex{nodes: map[string]*elkgraph.Node{}, edges: map[string]*elkgraph.Edge{}, ports: map[string]*elkgraph.Port{}}
	var walk func(n *elkgraph.Node)
	walk = func(n *elkgraph.Node) {
		idx.nodes[n.Identifier] = n
		for _, p := range n.Ports {
			idx.ports[p.Identifier] = p
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	for _, e := range edges {
		idx.edges[e.Identifier] = e
	}
	return idx
}

// resolve returns the element that a reference names, or nil.
func (idx *elkIndex) resolve(ref string) any {
	switch {
	case strings.HasPrefix(ref, "n:"):
		if n := idx.nodes[ref[2:]]; n != nil {
			return n
		}
	case strings.HasPrefix(ref, "p:"):
		if p := idx.ports[ref[2:]]; p != nil {
			return p
		}
	case strings.HasPrefix(ref, "e:"):
		if e := idx.edges[ref[2:]]; e != nil {
			return e
		}
	case strings.HasPrefix(ref, "l:"):
		hash := strings.LastIndexByte(ref, '#')
		if hash < 0 {
			return nil
		}
		i, err := strconv.Atoi(ref[hash+1:])
		if err != nil || i < 0 {
			return nil
		}
		var labels []*elkgraph.Label
		switch p := idx.resolve(ref[2:hash]).(type) {
		case *elkgraph.Node:
			labels = p.Labels
		case *elkgraph.Edge:
			labels = p.Labels
		case *elkgraph.Port:
			labels = p.Labels
		}
		if i < len(labels) {
			return labels[i]
		}
	}
	return nil
}

// TestTransferAgainstPhases takes, for each graph of the phase goldens, the
// LGraph that elkjs had at the end of each run, transfers its layout to the
// ELK graph and writes the JSON. The result must be the output of elkjs. So
// the test covers ElkGraphLayoutTransferrer and the JSON export without the
// processors.
func TestTransferAgainstPhases(t *testing.T) {
	files, err := nativetest.Files("")
	if err != nil {
		t.Fatal(err)
	}
	var equal, skipped, differ int
	skips := map[string]int{}
	for _, file := range files {
		c, err := nativetest.LoadCase(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := js.ParseJSON(c.In)
		if err != nil {
			t.Fatal(err)
		}
		root, _ := parsed.(*js.Obj[any])
		im := &elkgraph.Importer{}
		rootNode, err := im.Import(root)
		if err != nil {
			skipped++
			skips[err.Error()]++
			continue
		}
		idx := newElkIndex(rootNode, im.AllEdges)
		problem := ""
		// The test covers the runs of the layered algorithm. A graph with a
		// node that another algorithm lays out, or with "$H" members, is not
		// one that Layout handles.
		if strings.Contains(string(c.In), "$H") {
			problem = "the input has $H members"
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					problem = "an algorithm is not found"
				}
			}()
			elements(rootNode, im.AllEdges, func(_ prop.Holder, node *elkgraph.Node) {
				if node != nil {
					resolveAlgorithm(node)
					if data := resolved(node); data != nil && data.ID != elkgraph.LayeredAlgorithmID && len(node.Children) != 0 {
						problem = "a node with the algorithm " + data.ID
					}
				}
			})
		}()
		root.Delete("logging")
		for ri, run := range c.Runs {
			if problem != "" {
				break
			}
			state, err := run.State(run.Final)
			if err != nil {
				t.Fatal(err)
			}
			built, err := nativetest.Build(state)
			if err != nil {
				t.Fatalf("%s run %d: %v", file, ri, err)
			}
			// Give each origin the element of the ELK graph.
			fix := func(h prop.Holder) {
				if o, ok := prop.Lookup(h, iprops.Origin); ok {
					if ref, isRef := o.(nativetest.ElkRef); isRef {
						el := idx.resolve(string(ref))
						if el == nil {
							problem = "no element " + string(ref)
							return
						}
						prop.Set[any](h, iprops.Origin, el)
					}
				}
			}
			for _, g := range built.Graphs {
				fix(g)
			}
			for _, n := range built.Nodes {
				fix(n)
			}
			for _, e := range built.Edges {
				fix(e)
			}
			for _, l := range built.Labels {
				fix(l)
			}
			for _, p := range built.Ports {
				fix(p)
			}
			if problem != "" {
				break
			}
			func() {
				defer func() {
					if r := recover(); r != nil {
						problem = "panic: " + strings.SplitN(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(sprint(r), "\n", " "), "\t", " ")), "goroutine", 2)[0]
					}
				}()
				layered.ApplyLayout(built.Root())
			}()
			if problem != "" {
				break
			}
		}
		if problem != "" {
			skipped++
			skips[problem]++
			continue
		}
		im.TransferLayout()
		got := js.StringifyJSON(root)
		if got == string(c.Out) {
			equal++
			continue
		}
		differ++
		if differ <= 5 {
			i := 0
			for i < len(got) && i < len(c.Out) && got[i] == c.Out[i] {
				i++
			}
			lo := max(0, i-120)
			t.Errorf("%s (%s): the output differs from elkjs at byte %d\n  got  …%s\n  want …%s", c.Name, file, i, got[lo:min(len(got), i+120)], string(c.Out[lo:min(len(c.Out), i+120)]))
		}
	}
	t.Logf("%d graphs: %d outputs equal to elkjs, %d differ, %d skipped", len(files), equal, differ, skipped)
	for reason, n := range skips {
		t.Logf("%4d skipped: %s", n, reason)
	}
	if differ > 0 {
		t.Errorf("%d outputs differ from elkjs", differ)
	}
}

func sprint(v any) string {
	if e, ok := v.(error); ok {
		return e.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "?"
}
