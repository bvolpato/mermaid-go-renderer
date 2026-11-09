// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p5edges/orthogonal/direction/RoutingDirection.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package direction has the routing direction strategies of the orthogonal
// edge router: they adapt the routing generator to edges that go from west to
// east, from north to south or from south to north.
package direction

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"

// RoutingDirection is the enum RoutingDirection.
type RoutingDirection int

// The constants of RoutingDirection, in the order of the Java enum.
const (
	RoutingDirectionWestToEast RoutingDirection = iota
	RoutingDirectionNorthToSouth
	RoutingDirectionSouthToNorth
)

var namesRoutingDirection = []string{"WEST_TO_EAST", "NORTH_TO_SOUTH", "SOUTH_TO_NORTH"}

// String returns the name of the Java constant.
func (e RoutingDirection) String() string { return prop.EnumName(namesRoutingDirection, int(e)) }

func init() {
	prop.RegisterEnum[RoutingDirection]("org.eclipse.elk.alg.layered.p5edges.orthogonal.direction.RoutingDirection", namesRoutingDirection)
}
