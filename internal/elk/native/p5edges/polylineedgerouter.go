// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/PolylineEdgeRouter.java
// of Eclipse ELK 0.9.1 (EPL-2.0).
//
// Only the predicate that the orthogonal edge router uses is ported. The
// polyline edge router itself is not.

package p5edges

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// PredExternalWestOrEastPort is PolylineEdgeRouter.PRED_EXTERNAL_WEST_OR_EAST_PORT:
// the node stands for an external port of the west side or the east side.
func PredExternalWestOrEastPort(node *lgraph.LNode) bool {
	extPortSide := prop.Get(node, iprops.ExtPortSide)
	return node.Type == lgraph.NodeTypeExternalPort &&
		(extPortSide == core.PortSideWest || extPortSide == core.PortSideEast)
}

// allNodes is Iterables.all(nodes, predicate): it is true for an empty list.
func allNodes(nodes []*lgraph.LNode, predicate func(*lgraph.LNode) bool) bool {
	for _, node := range nodes {
		if !predicate(node) {
			return false
		}
	}
	return true
}
