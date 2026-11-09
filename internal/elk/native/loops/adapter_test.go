package loops_test

import (
	"slices"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
)

// The port adapter of the package adapters reads the self loops of a port
// through this interface.
var _ adapters.SelfLoopHolder = (*loops.SelfLoopHolder)(nil)

// TestSelfLoopEdgesOfPort checks the method that the port adapter calls: the
// edges of the incoming and of the outgoing self loops of a port, in the
// order of the lists of the SelfLoopPort.
func TestSelfLoopEdgesOfPort(t *testing.T) {
	graph := lgraph.NewLGraph()
	node := lgraph.NewLNode(graph)
	other := lgraph.NewLNode(graph)
	graph.LayerlessNodes = []*lgraph.LNode{node, other}

	port := func(n *lgraph.LNode) *lgraph.LPort {
		p := lgraph.NewLPort()
		p.SetNode(n)
		return p
	}
	edge := func(source, target *lgraph.LPort) *lgraph.LEdge {
		e := lgraph.NewLEdge()
		e.SetSource(source)
		e.SetTarget(target)
		return e
	}
	p0, p1, p2, q := port(node), port(node), port(node), port(other)
	loop1 := edge(p0, p1)
	loop2 := edge(p1, p0)
	loop3 := edge(p0, p1)
	edge(p0, q)
	edge(q, p2)

	if !loops.NeedsSelfLoopProcessing(node) || loops.NeedsSelfLoopProcessing(other) {
		t.Fatalf("NeedsSelfLoopProcessing: got %v for the node with self loops and %v for the other node",
			loops.NeedsSelfLoopProcessing(node), loops.NeedsSelfLoopProcessing(other))
	}
	holder := loops.Install(node)
	if loops.Of(node) != holder {
		t.Fatal("the property selfLoopHolder of the node is not the holder")
	}

	check := func(name string, p *lgraph.LPort, wantIn, wantOut []*lgraph.LEdge, wantOK bool) {
		t.Helper()
		in, out, ok := holder.SelfLoopEdgesOfPort(p)
		if ok != wantOK || !slices.Equal(in, wantIn) || !slices.Equal(out, wantOut) {
			t.Errorf("%s: got %d incoming, %d outgoing, ok %v; want %d incoming, %d outgoing, ok %v",
				name, len(in), len(out), ok, len(wantIn), len(wantOut), wantOK)
		}
	}
	check("p0", p0, []*lgraph.LEdge{loop2}, []*lgraph.LEdge{loop1, loop3}, true)
	check("p1", p1, []*lgraph.LEdge{loop1, loop3}, []*lgraph.LEdge{loop2}, true)
	check("p2", p2, nil, nil, false)
	check("q", q, nil, nil, false)

	// The three loops share their ports, so they are one hyper loop. The
	// port p0 has a regular edge too.
	if len(holder.SLHyperLoops) != 1 || len(holder.SLHyperLoops[0].SLEdges) != 3 {
		t.Fatalf("got %d hyper loops, want one with three edges", len(holder.SLHyperLoops))
	}
	if holder.SLPortMap.Get(p0).HadOnlySelfLoops || !holder.SLPortMap.Get(p1).HadOnlySelfLoops {
		t.Errorf("hadOnlySelfLoops: got %v for p0 and %v for p1, want false and true",
			holder.SLPortMap.Get(p0).HadOnlySelfLoops, holder.SLPortMap.Get(p1).HadOnlySelfLoops)
	}
}
