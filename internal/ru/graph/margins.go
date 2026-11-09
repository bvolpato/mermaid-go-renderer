// Ported from mermaid 12.2.0, utils/subGraphTitleMargins.ts (MIT).

package graph

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// SubGraphTitleMargins is the result of getSubGraphTitleMargins: the
// margins above and below the title of a subgraph, and their sum.
type SubGraphTitleMargins = utils.SubGraphTitleMargins

// GetSubGraphTitleMargins is getSubGraphTitleMargins({ flowchart }): the
// title margins of flowchart.subGraphTitleMargin. config is an object with
// the property flowchart, for example the whole config (ctx.Config). A
// margin that is not set is 0.
func GetSubGraphTitleMargins(config *js.Obj[any]) SubGraphTitleMargins {
	return utils.GetSubGraphTitleMargins(config)
}
