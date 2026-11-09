package clusters_test

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// TestMalformedNodes feeds nodes with missing and extreme values to every
// cluster shape, with every look. A shape must return, with or without an
// error: it must not panic and must not hang. Upstream has no result to
// compare with for some of these: with the handDrawn look, rough never
// returns for a hachure fill of a rectangle with NaN coordinates.
func TestMalformedNodes(t *testing.T) {
	var g rutest.ClusterGolden
	rutest.LoadGolden(t, "testdata/clusters.json.gz", &g)
	cfg, err := rutest.PatchedConfig(g.BaseConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A fixed generator, so that the cases are the same on every run.
	seed := uint32(0x636c7573)
	rnd := func() float64 {
		seed += 0x6d2b79f5
		x := seed
		x = (x ^ (x >> 15)) * (x | 1)
		x ^= x + (x^(x>>7))*(x|61)
		return float64(x^(x>>14)) / 4294967296
	}
	values := []float64{math.NaN(), 0, -1, 1e-9, 12.5, 250, -250, 3000, math.Inf(1), math.Inf(-1)}
	pick := func() float64 { return values[int(rnd()*float64(len(values)))] }
	shapes := []string{"", "rect", "squareRect", "roundedWithTitle", "noteGroup", "divider", "kanbanSection", "flowGroup", "usecaseSystemBoundary", "swimlane", "noSuchShape"}
	looks := []string{"", "classic", "neo", "handDrawn"}
	labels := []string{"", "Title", "<b>x</b> & **y**", "a\nb<br/>c", "fa:fa-car $$x$$"}

	count := 0
	for _, shape := range shapes {
		for _, look := range looks {
			for i := 0; i < 12; i++ {
				node := ru.NewNode(fmt.Sprintf("n%d", i))
				node.Shape, node.Look, node.IsGroup = shape, look, true
				node.Label = labels[int(rnd()*float64(len(labels)))]
				node.X, node.Y, node.Width, node.Height = pick(), pick(), pick(), pick()
				node.Padding, node.Rx, node.Ry, node.ColorIndex = pick(), pick(), pick(), pick()
				if rnd() < 0.5 {
					node.LabelType = "markdown"
				}
				if rnd() < 0.5 {
					node.CSSClasses, node.CSSClassesSet = "a statediagram-cluster-alt", true
				}
				if rnd() < 0.3 {
					node.CSSStyles = []string{"fill:#f00", "stroke", ":", "stroke-width:px", "stroke-dasharray:a b"}
				}
				if rnd() < 0.5 {
					node.Set("boundaryType", "package")
				}
				if rnd() < 0.5 {
					node.Set("direction", "LR")
				}
				if rnd() < 0.3 {
					node.Set("swimlaneContentTop", pick())
				}
				count++
				done := make(chan string, 1)
				go func() {
					defer func() {
						if p := recover(); p != nil {
							done <- fmt.Sprintf("panic: %v", p)
						}
					}()
					ctx := rutest.NewProbeContext(js.AsObj(js.Clone(cfg)), g.CSS[0], "flowchart-v2")
					group := ctx.SVG.Append("g")
					_, _ = clusters.InsertCluster(ctx, group, node)
					done <- ""
				}()
				select {
				case msg := <-done:
					if msg != "" {
						desc, _ := json.Marshal(map[string]any{"shape": shape, "look": look, "label": node.Label})
						t.Errorf("%s x=%v y=%v w=%v h=%v padding=%v rx=%v: %s", desc, node.X, node.Y, node.Width, node.Height, node.Padding, node.Rx, msg)
					}
				case <-time.After(20 * time.Second):
					t.Fatalf("shape %q look %q x=%v y=%v w=%v h=%v padding=%v rx=%v ry=%v: no result after 20 s", shape, look, node.X, node.Y, node.Width, node.Height, node.Padding, node.Rx, node.Ry)
				}
			}
		}
	}
	t.Logf("%d malformed nodes", count)
}
