package grammar

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/agentflow"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/block"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/c4diagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/classdiagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/erdiagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/flow"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/gantt"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/ishikawa"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/journey"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/kanban"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/mindmap"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/quadrant"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/requirementdiagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/sankey"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/sequencediagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/statediagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/timeline"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/venn"
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/xychart"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
)

// grammars lists every generated grammar by the name of its package.
var grammars = []struct {
	name string
	g    *jison.Grammar
}{
	{"agentflow", agentflow.Grammar},
	{"block", block.Grammar},
	{"c4diagram", c4diagram.Grammar},
	{"classdiagram", classdiagram.Grammar},
	{"erdiagram", erdiagram.Grammar},
	{"flow", flow.Grammar},
	{"gantt", gantt.Grammar},
	{"ishikawa", ishikawa.Grammar},
	{"journey", journey.Grammar},
	{"kanban", kanban.Grammar},
	{"mindmap", mindmap.Grammar},
	{"quadrant", quadrant.Grammar},
	{"requirementdiagram", requirementdiagram.Grammar},
	{"sankey", sankey.Grammar},
	{"sequencediagram", sequencediagram.Grammar},
	{"statediagram", statediagram.Grammar},
	{"timeline", timeline.Grammar},
	{"venn", venn.Grammar},
	{"xychart", xychart.Grammar},
}
