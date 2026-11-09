package adapters

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// exact is a double that the golden file keeps exact: a JSON number, or one
// of the texts "-0", "NaN", "Infinity" and "-Infinity".
type exact float64

func (e *exact) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		switch s {
		case "-0":
			*e = exact(math.Copysign(0, -1))
		case "NaN":
			*e = exact(math.NaN())
		case "Infinity":
			*e = exact(math.Inf(1))
		case "-Infinity":
			*e = exact(math.Inf(-1))
		default:
			return fmt.Errorf("unknown number %q", s)
		}
		return nil
	}
	f, err := strconv.ParseFloat(string(data), 64)
	*e = exact(f)
	return err
}

type goldenPort struct {
	Side  string `json:"side"`
	X     exact  `json:"x"`
	Y     exact  `json:"y"`
	Index *int   `json:"index"`
}

func (g goldenPort) port(t *testing.T) *lgraph.LPort {
	t.Helper()
	side, ok := prop.ParseEnum[core.PortSide](g.Side)
	if !ok {
		t.Fatalf("unknown port side %q", g.Side)
	}
	port := lgraph.NewLPort()
	port.Side = side
	port.Pos.X = float64(g.X)
	port.Pos.Y = float64(g.Y)
	if g.Index != nil {
		prop.Set(port, lopts.PortIndex, *g.Index)
	}
	return port
}

// compareOrBail calls the comparator and reports whether it stopped with
// lgraph.Bail, which stands for the exception of the Java code.
func compareOrBail(port1, port2 *lgraph.LPort) (result int, stopped bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*lgraph.BailError); !ok {
				panic(r)
			}
			stopped = true
		}
	}()
	return DefaultPortListSorter(port1, port2), false
}

// TestDefaultPortListSorter compares the port comparator with elkjs 0.9.3
// for random pairs of ports. tools/reference/gen-elk-nodespacing.mjs writes
// the golden file testdata/portcomparator.json.gz.
func TestDefaultPortListSorter(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "portcomparator.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			A      goldenPort `json:"a"`
			B      goldenPort `json:"b"`
			Cmp    int        `json:"cmp"`
			Throws bool       `json:"throws"`
		} `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("the golden file has no case")
	}
	failed := 0
	for i, c := range file.Cases {
		got, stopped := compareOrBail(c.A.port(t), c.B.port(t))
		if stopped != c.Throws || !stopped && got != c.Cmp {
			failed++
			if failed <= 5 {
				t.Errorf("case %d: compare(%+v, %+v) is %d (stopped: %v), elkjs has %d (throws: %v)", i, c.A, c.B, got, stopped, c.Cmp, c.Throws)
			}
		}
	}
	t.Logf("%d cases, %d differ from elkjs", len(file.Cases), failed)
	if failed > 0 {
		t.Errorf("%d of %d cases differ from elkjs", failed, len(file.Cases))
	}
}

// TestSortPortList checks that the node adapter sorts the port list only
// for port constraints that fix the order, as LNodeAdapter.sortPortList
// does.
func TestSortPortList(t *testing.T) {
	build := func(constraints core.PortConstraints) *lgraph.LNode {
		node := lgraph.NewLNode(lgraph.NewLGraph())
		prop.Set(node, lopts.PortConstraints, constraints)
		for i, side := range []core.PortSide{core.PortSideWest, core.PortSideNorth, core.PortSideEast, core.PortSideNorth} {
			port := lgraph.NewLPort()
			port.Side = side
			port.Pos.X = float64(10 - i)
			port.ID = i
			port.SetNode(node)
		}
		return node
	}
	ids := func(node *lgraph.LNode) []int {
		var out []int
		for _, port := range node.Ports {
			out = append(out, port.ID)
		}
		return out
	}
	free := build(core.PortConstraintsFixedSide)
	AdaptLNode(free, false).SortPortList()
	if got := fmt.Sprint(ids(free)); got != "[0 1 2 3]" {
		t.Errorf("FIXED_SIDE: the port order is %s, it must not change", got)
	}
	fixed := build(core.PortConstraintsFixedOrder)
	AdaptLNode(fixed, false).SortPortList()
	// North ports by x (the port 3 has x 7, the port 1 has x 9), then east,
	// then west.
	if got := fmt.Sprint(ids(fixed)); got != "[3 1 2 0]" {
		t.Errorf("FIXED_ORDER: the port order is %s, want [3 1 2 0]", got)
	}
}
