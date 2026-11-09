// Package diagrams links every ported diagram type into a program. Import
// it for its side effect: each diagram package registers itself.
package diagrams

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/agentflow"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/architecture"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/block"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/c4"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/class"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/cynefin"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/er"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/eventmodeling"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/flowchart"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/gantt"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/git"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/info"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/ishikawa"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/journey"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/kanban"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/mindmap"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/packet"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/pie"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/quadrant"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/radar"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/railroad"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/requirement"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/sankey"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/sequence"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/state"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/timeline"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/treemap"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/treeview"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/usecase"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/venn"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/wardley"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/xychart"

	// The parts of the unified renderer register themselves: shapes,
	// cluster shapes and layouts. Flowchart, class, state, ER and other
	// diagrams draw through them.
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/clusters"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/cosebilkent"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/dagre"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/elk"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/swimlanes"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/layout/tidytree"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set1"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set2"
	_ "github.com/bvolpato/mermaid-go-renderer/internal/ru/shapes/set3"
)
