package clusters_test

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// TestClusterGolden replays the cases of tools/reference/gen-clusters.mjs:
// the real insertCluster, getUsecaseSystemBoundaryGeometry and
// clusterPaintsTitle of mermaid in Chromium.
func TestClusterGolden(t *testing.T) {
	rutest.RunClusterGolden(t, "testdata/clusters.json.gz", rutest.ClusterFuncs{
		InsertCluster: clusters.InsertCluster,
		Geometry: func(node *ru.Node, labelBBox dom.Rect) map[string]any {
			g := clusters.GetUsecaseSystemBoundaryGeometry(node, labelBBox)
			return map[string]any{
				"boundaryType": g.BoundaryType,
				"width":        g.Width,
				"height":       g.Height,
				"x":            g.X,
				"y":            g.Y,
				"bodyY":        g.BodyY,
				"bodyHeight":   g.BodyHeight,
				"tabHeight":    g.TabHeight,
				"tabWidth":     g.TabWidth,
			}
		},
		PaintsTitle: clusters.ClusterPaintsTitle,
	})
}

// TestPositionCluster checks positionCluster, clear and
// getClusterTitleWidth. The build of mermaid does not have these functions
// (nothing calls them, so the bundler removes them), so there is no golden
// file for them.
func TestPositionCluster(t *testing.T) {
	var g rutest.ClusterGolden
	rutest.LoadGolden(t, "testdata/clusters.json.gz", &g)
	cfg, err := rutest.PatchedConfig(g.BaseConfig, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := rutest.NewProbeContext(cfg, g.CSS[0], "flowchart-v2")
	group := ctx.SVG.Append("g")
	node := ru.NewNode("S")
	node.Label, node.IsGroup, node.Padding = "Title", true, 8
	node.X, node.Y, node.Width, node.Height = 100, 50, 200, 80

	if err := clusters.PositionCluster(ctx, node); err == nil {
		t.Error("PositionCluster before InsertCluster: no error")
	}
	result, err := clusters.InsertCluster(ctx, group, node)
	if err != nil {
		t.Fatal(err)
	}
	node.X, node.Y = 12.5, -7
	if err := clusters.PositionCluster(ctx, node); err != nil {
		t.Fatal(err)
	}
	if got := result.Cluster.AttrOr("transform", ""); got != "translate(12.5, -7)" {
		t.Errorf("transform of the cluster: got %q", got)
	}
	clusters.Clear(ctx)
	if err := clusters.PositionCluster(ctx, node); err == nil {
		t.Error("PositionCluster after Clear: no error")
	}

	measure := ctx.SVG.Append("g")
	width, err := clusters.GetClusterTitleWidth(ctx, measure, node)
	if err != nil {
		t.Fatal(err)
	}
	if !(width > 0) {
		t.Errorf("title width: got %v", width)
	}
	if measure.FirstChild != nil {
		t.Error("GetClusterTitleWidth left the label in the element")
	}
}
