package p4nodes

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p4nodes/internal/p4test"
)

func TestSimpleNodePlacer(t *testing.T) {
	p4test.RunProcessor(t, "SimpleNodePlacer", 9, func() lgraph.Processor { return NewSimpleNodePlacer() })
}

func TestLinearSegmentsNodePlacer(t *testing.T) {
	p4test.RunProcessor(t, "LinearSegmentsNodePlacer", 13, func() lgraph.Processor { return NewLinearSegmentsNodePlacer() })
}

func TestNetworkSimplexPlacer(t *testing.T) {
	p4test.RunProcessor(t, "NetworkSimplexPlacer", 14, func() lgraph.Processor { return NewNetworkSimplexPlacer() })
}

func TestSimpleNodePlacerExtra(t *testing.T) {
	p4test.RunExtra(t, "testdata/simplenodeplacer.jsonl.gz", "SimpleNodePlacer", 30, func() lgraph.Processor { return NewSimpleNodePlacer() })
}

func TestLinearSegmentsNodePlacerExtra(t *testing.T) {
	p4test.RunExtra(t, "testdata/linearsegmentsnodeplacer.jsonl.gz", "LinearSegmentsNodePlacer", 120, func() lgraph.Processor { return NewLinearSegmentsNodePlacer() })
}

func TestNetworkSimplexPlacerExtra(t *testing.T) {
	p4test.RunExtra(t, "testdata/networksimplexplacer.jsonl.gz", "NetworkSimplexPlacer", 200, func() lgraph.Processor { return NewNetworkSimplexPlacer() })
}

// TestRoundToDouble compares roundToDouble with (double) Math.round(y) of
// the worker of elkjs, bit for bit. tools/reference/gen-elk-p4nodes.mjs
// writes the golden.
func TestRoundToDouble(t *testing.T) {
	data, err := os.ReadFile("testdata/roundtodouble.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases [][2]string
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 400 {
		t.Fatalf("the golden has %d values, the test expects at least 400", len(cases))
	}
	for _, c := range cases {
		in, err1 := strconv.ParseUint(c[0], 16, 64)
		want, err2 := strconv.ParseUint(c[1], 16, 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("bad golden entry %v", c)
		}
		y := math.Float64frombits(in)
		if got := math.Float64bits(roundToDouble(y)); got != want {
			t.Errorf("roundToDouble(%v) = %v (%016x), want %v (%016x)", y, math.Float64frombits(got), got, math.Float64frombits(want), want)
		}
	}
}
