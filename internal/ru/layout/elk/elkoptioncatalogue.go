// Ported from mermaid 12.2.0,
// rendering-util/layout-algorithms/elk/elkOptionCatalogue.ts (MIT).

package elk

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// The catalogue of upstream lists the ELK options that affect the layout,
// each one commented out. An option that is switched on there applies to
// every ELK diagram. In mermaid 12.2.0 all four objects are empty, and a
// test of upstream asserts that. They are kept here, in the same merge
// positions, so that a later upstream version can be applied as a diff.

// PLACEMENT_OPTIONS is merged over the layout options of the root graph.
var PLACEMENT_OPTIONS = js.NewObj[any]()

// EDGE_ROUTING_OPTIONS is merged over the layout options of the root graph,
// after PLACEMENT_OPTIONS.
var EDGE_ROUTING_OPTIONS = js.NewObj[any]()

// ROOT_EXPERIMENT_OVERRIDES is merged last over the layout options of the
// root graph.
var ROOT_EXPERIMENT_OVERRIDES = js.NewObj[any]()

// SUBGRAPH_EXPERIMENT_OVERRIDES is merged last over the layout options of
// every subgraph.
var SUBGRAPH_EXPERIMENT_OVERRIDES = js.NewObj[any]()
