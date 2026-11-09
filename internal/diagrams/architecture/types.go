// Ported from mermaid 12.2.0, diagrams/architecture/architectureTypes.ts
// (MIT).

package architecture

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

/*=======================================*\
|       Architecture Diagram Types        |
\*=======================================*/

// ArchitectureAlignment is 'vertical' | 'horizontal' | 'bend'.
type ArchitectureAlignment = string

// ArchitectureDirection is 'L' | 'R' | 'T' | 'B'.
type ArchitectureDirection = string

// ArchitectureDirectionPair is a pair of directions without LL, RR, TT and
// BB, which are impossible connections.
type ArchitectureDirectionPair = string

// architectureDirectionName is ArchitectureDirectionName.
func architectureDirectionName(dir ArchitectureDirection) string {
	switch dir {
	case "L":
		return "left"
	case "R":
		return "right"
	case "T":
		return "top"
	case "B":
		return "bottom"
	}
	return ""
}

// architectureDirectionArrow is ArchitectureDirectionArrow[dir](scale).
func architectureDirectionArrow(dir ArchitectureDirection, scale float64) string {
	switch dir {
	case "L":
		return js.Str(scale) + "," + js.Str(scale/2) + " 0," + js.Str(scale) + " 0,0"
	case "R":
		return "0," + js.Str(scale/2) + " " + js.Str(scale) + ",0 " + js.Str(scale) + "," + js.Str(scale)
	case "T":
		return "0,0 " + js.Str(scale) + ",0 " + js.Str(scale/2) + "," + js.Str(scale)
	default:
		return js.Str(scale/2) + ",0 " + js.Str(scale) + "," + js.Str(scale) + " 0," + js.Str(scale)
	}
}

// architectureDirectionArrowShift is
// ArchitectureDirectionArrowShift[dir](orig, arrowSize).
func architectureDirectionArrowShift(dir ArchitectureDirection, orig, arrowSize float64) float64 {
	switch dir {
	case "L", "T":
		return orig - arrowSize + 2
	default:
		return orig - 2
	}
}

func getOppositeArchitectureDirection(x ArchitectureDirection) ArchitectureDirection {
	if isArchitectureDirectionX(x) {
		if x == "L" {
			return "R"
		}
		return "L"
	}
	if x == "T" {
		return "B"
	}
	return "T"
}

func isArchitectureDirection(x string) bool {
	return x == "L" || x == "R" || x == "T" || x == "B"
}

func isArchitectureDirectionX(x ArchitectureDirection) bool {
	return x == "L" || x == "R"
}

func isArchitectureDirectionY(x ArchitectureDirection) bool {
	return x == "T" || x == "B"
}

func isArchitectureDirectionXY(a, b ArchitectureDirection) bool {
	aX_bY := isArchitectureDirectionX(a) && isArchitectureDirectionY(b)
	aY_bX := isArchitectureDirectionY(a) && isArchitectureDirectionX(b)
	return aX_bY || aY_bX
}

func isArchitecturePairXY(pair ArchitectureDirectionPair) bool {
	lhs := pair[0:1]
	rhs := pair[1:2]
	aX_bY := isArchitectureDirectionX(lhs) && isArchitectureDirectionY(rhs)
	aY_bX := isArchitectureDirectionY(lhs) && isArchitectureDirectionX(rhs)
	return aX_bY || aY_bX
}

// isValidArchitectureDirectionPair verifies that the architecture direction
// pair does not contain an invalid match (LL, RR, TT, BB).
func isValidArchitectureDirectionPair(x string) bool {
	return x != "LL" && x != "RR" && x != "TT" && x != "BB"
}

// getArchitectureDirectionPair creates a pair of the directions of each side
// of an edge. The source is always the first character. The second result
// is false when sourceDir and targetDir are the same: upstream then returns
// undefined.
func getArchitectureDirectionPair(sourceDir, targetDir ArchitectureDirection) (ArchitectureDirectionPair, bool) {
	pair := sourceDir + targetDir
	if isValidArchitectureDirectionPair(pair) {
		return pair, true
	}
	return "", false
}

// shiftPositionByArchitectureDirectionPair returns the position next to
// [x, y] on the side that the pair names.
func shiftPositionByArchitectureDirectionPair(pos [2]float64, pair ArchitectureDirectionPair) [2]float64 {
	x, y := pos[0], pos[1]
	lhs := pair[0:1]
	rhs := pair[1:2]
	if isArchitectureDirectionX(lhs) {
		if isArchitectureDirectionY(rhs) {
			return [2]float64{x + sign(lhs == "L", -1, 1), y + sign(rhs == "T", 1, -1)}
		}
		return [2]float64{x + sign(lhs == "L", -1, 1), y}
	}
	if isArchitectureDirectionX(rhs) {
		return [2]float64{x + sign(rhs == "L", 1, -1), y + sign(lhs == "T", 1, -1)}
	}
	return [2]float64{x, y + sign(lhs == "T", 1, -1)}
}

// sign is the conditional operator: cond ? a : b.
func sign(cond bool, a, b float64) float64 {
	if cond {
		return a
	}
	return b
}

// getArchitectureDirectionXYFactors returns, for the directional pair of an
// XY edge, the scale factors that shift the coordinates inwards towards the
// edge: [+/- 1, +/- 1].
func getArchitectureDirectionXYFactors(pair ArchitectureDirectionPair) (float64, float64) {
	if pair == "LT" || pair == "TL" {
		return 1, 1
	} else if pair == "BL" || pair == "LB" {
		return 1, -1
	} else if pair == "BR" || pair == "RB" {
		return -1, -1
	}
	return -1, 1
}

func getArchitectureDirectionAlignment(a, b ArchitectureDirection) ArchitectureAlignment {
	if isArchitectureDirectionXY(a, b) {
		return "bend"
	} else if isArchitectureDirectionX(a) {
		return "horizontal"
	}
	return "vertical"
}

// ArchitectureNode is ArchitectureService | ArchitectureJunction. A nil
// pointer is a property that is undefined.
type ArchitectureNode struct {
	ID string
	// Type is "service" or "junction".
	Type     string
	Edges    []*ArchitectureEdge
	Icon     *string
	IconText *string
	Title    *string
	In       *string
	// Width and Height are the size of the element of a service. The
	// renderer sets them. HasSize is false until then.
	Width, Height float64
	HasSize       bool
}

func isArchitectureService(x *ArchitectureNode) bool {
	return x.Type == "service"
}

func isArchitectureJunction(x *ArchitectureNode) bool {
	return x.Type == "junction"
}

// ArchitectureGroup is a group of the diagram.
type ArchitectureGroup struct {
	ID    string
	Icon  *string
	Title *string
	In    *string
}

// ArchitectureLayoutHint is an align statement. Direction is "row" or
// "column".
type ArchitectureLayoutHint struct {
	Direction string
	Members   []string
}

// ArchitectureEdge is an edge of the diagram.
type ArchitectureEdge struct {
	LhsID    string
	LhsDir   ArchitectureDirection
	LhsInto  bool
	LhsGroup bool
	RhsID    string
	RhsDir   ArchitectureDirection
	RhsInto  bool
	RhsGroup bool
	Title    *string
}

// ArchitectureDirectionPairMap maps a direction pair to the id of the node
// on the other side.
type ArchitectureDirectionPairMap = *js.Map[ArchitectureDirectionPair, string]

// ArchitectureAdjacencyList maps the id of a node to its direction pairs.
type ArchitectureAdjacencyList = *js.Map[string, ArchitectureDirectionPairMap]

// ArchitectureSpatialMap maps the id of a node to its [x, y] cell.
type ArchitectureSpatialMap = *js.Map[string, [2]float64]

// ArchitectureGroupAlignments maps the direction that groups connect from.
//
// Key: `groupA-groupB`, where groupA and groupB are the JSON string ids of
// the two groups in alphabetical order. The value is "vertical" or
// "horizontal".
type ArchitectureGroupAlignments = *js.Map[string, ArchitectureAlignment]

func architectureGroupAlignmentKey(groupA, groupB string) string {
	// [groupA, groupB].sort()
	lowerGroupID, upperGroupID := groupA, groupB
	if js.CompareStrings(groupB, groupA) < 0 {
		lowerGroupID, upperGroupID = groupB, groupA
	}
	return js.StringifyJSON(lowerGroupID) + "-" + js.StringifyJSON(upperGroupID)
}

// ArchitectureDataStructures is the result of getDataStructures.
type ArchitectureDataStructures struct {
	AdjList         ArchitectureAdjacencyList
	SpatialMaps     []ArchitectureSpatialMap
	GroupAlignments ArchitectureGroupAlignments
}

// The cytoscape override types of upstream (EdgeSingularData,
// NodeSingularData) have no port: the elements of cytoscape live in
// JavaScript (internal/cytoscape), and the renderer reads the same values
// from the db.

// d3Element is D3Element: a selection of one element.
type d3Element = *dom.Node
