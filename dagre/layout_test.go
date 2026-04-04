package dagre

import (
	"testing"
)

func TestBasicLayout(t *testing.T) {
	g := NewGraph()
	g.label.RankSep = 50
	g.label.NodeSep = 50
	g.label.EdgeSep = 20
	g.label.RankDir = "TB"

	g.SetNode("A", &NodeLabel{Width: 100, Height: 40})
	g.SetNode("B", &NodeLabel{Width: 100, Height: 40})
	g.SetNode("C", &NodeLabel{Width: 100, Height: 40})

	g.SetEdgeVW("A", "B", &EdgeLabel{MinLen: 1, Weight: 1})
	g.SetEdgeVW("A", "C", &EdgeLabel{MinLen: 1, Weight: 1})
	g.SetEdgeVW("B", "C", &EdgeLabel{MinLen: 1, Weight: 1})

	Layout(g)

	// Basic sanity: all nodes should have coordinates set
	for _, v := range []string{"A", "B", "C"} {
		n := g.Node(v)
		if n == nil {
			t.Fatalf("node %s is nil after layout", v)
		}
		t.Logf("Node %s: x=%.1f y=%.1f w=%.1f h=%.1f rank=%d order=%d",
			v, n.X, n.Y, n.Width, n.Height, n.Rank, n.Order)
	}

	a := g.Node("A")
	b := g.Node("B")
	c := g.Node("C")

	// A should be above B (smaller Y)
	if a.Y >= b.Y {
		t.Errorf("expected A.Y (%.1f) < B.Y (%.1f)", a.Y, b.Y)
	}
	// B should be above C
	if b.Y >= c.Y {
		t.Errorf("expected B.Y (%.1f) < C.Y (%.1f)", b.Y, c.Y)
	}

	// Edges should have points
	for _, e := range g.Edges() {
		el := g.EdgeByKey(e)
		t.Logf("Edge %s->%s: %d points", e.V, e.W, len(el.Points))
		if len(el.Points) < 2 {
			t.Errorf("edge %s->%s has only %d points", e.V, e.W, len(el.Points))
		}
	}
}

func TestLinearGraph(t *testing.T) {
	g := NewGraph()
	g.label.RankSep = 50
	g.label.NodeSep = 50
	g.label.EdgeSep = 20

	g.SetNode("1", &NodeLabel{Width: 80, Height: 30})
	g.SetNode("2", &NodeLabel{Width: 80, Height: 30})
	g.SetNode("3", &NodeLabel{Width: 80, Height: 30})
	g.SetNode("4", &NodeLabel{Width: 80, Height: 30})

	g.SetEdgeVW("1", "2", &EdgeLabel{MinLen: 1, Weight: 1})
	g.SetEdgeVW("2", "3", &EdgeLabel{MinLen: 1, Weight: 1})
	g.SetEdgeVW("3", "4", &EdgeLabel{MinLen: 1, Weight: 1})

	Layout(g)

	// All nodes should be vertically aligned (same X)
	n1 := g.Node("1")
	n2 := g.Node("2")
	n3 := g.Node("3")
	n4 := g.Node("4")

	t.Logf("1: x=%.1f y=%.1f", n1.X, n1.Y)
	t.Logf("2: x=%.1f y=%.1f", n2.X, n2.Y)
	t.Logf("3: x=%.1f y=%.1f", n3.X, n3.Y)
	t.Logf("4: x=%.1f y=%.1f", n4.X, n4.Y)

	// Nodes should be in descending Y order
	if n1.Y >= n2.Y || n2.Y >= n3.Y || n3.Y >= n4.Y {
		t.Error("expected nodes in ascending Y order: 1 < 2 < 3 < 4")
	}

	// All should have the same X coordinate (linear chain)
	dx := n1.X - n2.X
	if dx < -1 || dx > 1 {
		t.Errorf("expected similar X coords, got 1=%.1f 2=%.1f", n1.X, n2.X)
	}
}

func TestBranchingGraph(t *testing.T) {
	g := NewGraph()
	g.label.RankSep = 50
	g.label.NodeSep = 50
	g.label.EdgeSep = 20

	g.SetNode("root", &NodeLabel{Width: 100, Height: 40})
	g.SetNode("left", &NodeLabel{Width: 100, Height: 40})
	g.SetNode("right", &NodeLabel{Width: 100, Height: 40})

	g.SetEdgeVW("root", "left", &EdgeLabel{MinLen: 1, Weight: 1})
	g.SetEdgeVW("root", "right", &EdgeLabel{MinLen: 1, Weight: 1})

	Layout(g)

	root := g.Node("root")
	left := g.Node("left")
	right := g.Node("right")

	t.Logf("root: x=%.1f y=%.1f", root.X, root.Y)
	t.Logf("left: x=%.1f y=%.1f", left.X, left.Y)
	t.Logf("right: x=%.1f y=%.1f", right.X, right.Y)

	// Root should be above both children
	if root.Y >= left.Y {
		t.Errorf("expected root above left")
	}
	if root.Y >= right.Y {
		t.Errorf("expected root above right")
	}

	// Left and right should be on the same rank (same Y)
	dy := left.Y - right.Y
	if dy < -1 || dy > 1 {
		t.Errorf("expected left and right at same Y, got left=%.1f right=%.1f", left.Y, right.Y)
	}

	// Left and right should be separated in X
	if left.X == right.X {
		t.Error("expected left and right to have different X coordinates")
	}
}

func TestLRDirection(t *testing.T) {
	g := NewGraph()
	g.label.RankSep = 50
	g.label.NodeSep = 50
	g.label.EdgeSep = 20
	g.label.RankDir = "LR"

	g.SetNode("A", &NodeLabel{Width: 100, Height: 40})
	g.SetNode("B", &NodeLabel{Width: 100, Height: 40})

	g.SetEdgeVW("A", "B", &EdgeLabel{MinLen: 1, Weight: 1})

	Layout(g)

	a := g.Node("A")
	b := g.Node("B")

	t.Logf("A: x=%.1f y=%.1f", a.X, a.Y)
	t.Logf("B: x=%.1f y=%.1f", b.X, b.Y)

	// In LR mode, A should be to the left of B
	if a.X >= b.X {
		t.Errorf("expected A.X (%.1f) < B.X (%.1f) in LR mode", a.X, b.X)
	}
}
