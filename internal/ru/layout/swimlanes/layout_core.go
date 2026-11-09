// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/swimlanes/layoutCore.ts (MIT).

package swimlanes

import (
	"fmt"
	"runtime/debug"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// SwimlaneDirection is "TB", "LR", "BT" or "RL".
type SwimlaneDirection = string

func getSwimlaneDirection(data4Layout *ru.LayoutData) SwimlaneDirection {
	if data4Layout.Direction == "" {
		return "TB"
	}
	return data4Layout.Direction
}

// configNumber is `value ?? d` for a number of the config. Upstream uses a
// value that is not a number as it is; here it is converted to a number.
func configNumber(value any, d float64) float64 {
	if js.IsNullish(value) {
		return d
	}
	if n, ok := value.(float64); ok {
		return n
	}
	return js.ToNumber(value)
}

// configBool is the truthiness of `value ?? d` for a flag of the config.
func configBool(value any, d bool) bool {
	if js.IsNullish(value) {
		return d
	}
	return js.Truthy(value)
}

// internalError is a panic of the layout code that is not an error of
// upstream: layout data that the port does not expect. The layout core
// returns it as an error, so that malformed input cannot crash a render.
type internalError struct {
	value any
	stack []byte
}

func (e *internalError) Error() string {
	return fmt.Sprintf("swimlane layout: %v", e.value)
}

// coreTrace receives the state of the layout after each step of the layout
// core. The tests compare these states with upstream. result is the result
// of the Sugiyama phases; it is set for the step "sugiyama" only.
type coreTrace func(step string, result *LayoutResult)

// runSwimlaneLayoutCore is the pure swimlane layout core: it computes the
// positions of the nodes and the points of the edges and writes them into
// the layout data. The nodes must have their sizes.
func runSwimlaneLayoutCore(data4Layout *ru.LayoutData) (direction SwimlaneDirection, err error) {
	return runSwimlaneLayoutCoreTraced(data4Layout, nil)
}

func runSwimlaneLayoutCoreTraced(data4Layout *ru.LayoutData, trace coreTrace) (direction SwimlaneDirection, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(jsError); ok {
				err = e
				return
			}
			err = &internalError{value: r, stack: debug.Stack()}
		}
	}()
	step := func(name string, result *LayoutResult) {
		if trace != nil {
			trace(name, result)
		}
	}

	g := toGraphView(data4Layout)
	nodeGap := configNumber(js.Path(data4Layout.Config, "flowchart", "nodeSpacing"), 40)
	layerGap := configNumber(js.Path(data4Layout.Config, "flowchart", "rankSpacing"), 100)
	ignoreCrossLaneEdges := configBool(js.Path(data4Layout.Config, "swimlane", "ignoreCrossLaneEdges"), true)
	optimizeRanksByCrossings := configBool(js.Path(data4Layout.Config, "swimlane", "optimizeRanksByCrossings"), true)
	automaticLaneOrdering := configBool(js.Path(data4Layout.Config, "swimlane", "automaticLaneOrdering"), false)
	direction = getSwimlaneDirection(data4Layout)

	result := sugiyamaLayout(g, &LayoutOptions{
		NodeGap:                  nodeGap,
		LayerGap:                 layerGap,
		IgnoreCrossLaneEdges:     optOf(ignoreCrossLaneEdges),
		OptimizeRanksByCrossings: optOf(optimizeRanksByCrossings),
		AutomaticLaneOrdering:    optOf(automaticLaneOrdering),
		Direction:                direction,
	})
	step("sugiyama", result)
	writeBackToLayoutData(g, result.Ordered, result.Coordinates, &WriteBackOptions{NodeGap: nodeGap, LayerGap: layerGap})
	step("writeBack", nil)

	// The layout phases above position nodes only; they do not emit edge
	// routing. Reset any edge points carried on the input so that
	// routeEdgesOrthogonal below is the single source of truth for swimlane
	// edge geometry.
	for _, edge := range data4Layout.Edges {
		edge.Points = nil
	}
	routeEdgesOrthogonal(data4Layout, direction)
	step("route", nil)

	for _, edge := range data4Layout.Edges {
		if edge.Curve == "" || edge.Curve == "basis" {
			edge.Curve = "rounded"
		}
	}

	postProcessSwimlaneLayoutTraced(data4Layout, direction, func(name string) { step(name, nil) })

	validateSwimlanesLayout(data4Layout)
	step("end", nil)

	return direction, nil
}
