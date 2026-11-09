// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/SelfLoopType.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// SelfLoopType is the enum SelfLoopType: the kinds of self loops, by the
// number of node sides that have a port of the loop.
type SelfLoopType int

// The constants of SelfLoopType, in the order of the Java enum.
const (
	// SelfLoopTypeOneSide connects ports that are all on the same side.
	SelfLoopTypeOneSide SelfLoopType = iota
	// SelfLoopTypeTwoSidesCorner connects ports on two adjacent sides.
	SelfLoopTypeTwoSidesCorner
	// SelfLoopTypeTwoSidesOpposing connects ports on two opposing sides.
	SelfLoopTypeTwoSidesOpposing
	// SelfLoopTypeThreeSides connects ports on three sides.
	SelfLoopTypeThreeSides
	// SelfLoopTypeFourSides connects ports on all four sides.
	SelfLoopTypeFourSides
)

var namesSelfLoopType = []string{"ONE_SIDE", "TWO_SIDES_CORNER", "TWO_SIDES_OPPOSING", "THREE_SIDES", "FOUR_SIDES"}

// String returns the name of the Java constant.
func (t SelfLoopType) String() string { return prop.EnumName(namesSelfLoopType, int(t)) }

func init() {
	prop.RegisterEnum[SelfLoopType]("org.eclipse.elk.alg.layered.intermediate.loops.SelfLoopType", namesSelfLoopType)
}

// SelfLoopTypeFromPortSides is SelfLoopType.fromPortSides(portSides): the
// type of a self loop with ports on the given sides. portSides is a set: each
// side is in it one time.
//
// Java throws for the side UNDEFINED, and returns null for a set that is
// empty. The layout stops in both cases.
func SelfLoopTypeFromPortSides(portSides []core.PortSide) SelfLoopType {
	contains := func(side core.PortSide) bool {
		for _, s := range portSides {
			if s == side {
				return true
			}
		}
		return false
	}
	if contains(core.PortSideUndefined) {
		lgraph.Bail("Port sides must not contain UNDEFINED")
	}

	switch len(portSides) {
	case 1:
		return SelfLoopTypeOneSide

	case 2:
		// Find out whether the two sides are opposite to each other.
		eastWest := contains(core.PortSideEast) && contains(core.PortSideWest)
		northSouth := contains(core.PortSideNorth) && contains(core.PortSideSouth)

		if eastWest || northSouth {
			return SelfLoopTypeTwoSidesOpposing
		}
		return SelfLoopTypeTwoSidesCorner

	case 3:
		return SelfLoopTypeThreeSides

	case 4:
		return SelfLoopTypeFourSides
	}
	lgraph.Bail("SelfLoopType: a self loop with ports on %d sides has no type", len(portSides))
	return 0
}
