// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/SelfHyperLoopLabels.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Alignment is the enum SelfHyperLoopLabels.Alignment: the alignments that
// the labels of a hyper loop can have. Labels on the north side or the south
// side have one of the first three. Labels on the east side or the west side
// have the alignment TOP.
type Alignment int

// The constants of Alignment, in the order of the Java enum.
const (
	// AlignmentCenter is a centered label on the north or south side.
	AlignmentCenter Alignment = iota
	// AlignmentLeft is a left-aligned label on the north or south side.
	AlignmentLeft
	// AlignmentRight is a right-aligned label on the north or south side.
	AlignmentRight
	// AlignmentTop is a top-aligned label on the east or west side.
	AlignmentTop
)

var namesAlignment = []string{"CENTER", "LEFT", "RIGHT", "TOP"}

// String returns the name of the Java constant.
func (a Alignment) String() string { return prop.EnumName(namesAlignment, int(a)) }

func init() {
	prop.RegisterEnum[Alignment]("org.eclipse.elk.alg.layered.intermediate.loops.SelfHyperLoopLabels$Alignment", namesAlignment)
}

// SelfHyperLoopLabels is the class SelfHyperLoopLabels: the labels of a
// SelfHyperLoop. A side of the node and an alignment on that side give their
// position.
type SelfHyperLoopLabels struct {
	// ID is the public field id. Set it with SetID.
	ID int
	// IDAssigned tells that SetID was called. The object of elkjs has its
	// own field id only from the first assignment, and the stored states of
	// the phase goldens show that field only then.
	IDAssigned bool

	// LLabels has the labels of this instance (getLLabels()).
	LLabels []*lgraph.LLabel
	// Size is the size that the labels need together (getSize()).
	Size emath.KVector
	// Position is the top left corner of the bounding box (getPosition()).
	Position emath.KVector
	// LayoutDirection is the layout direction of the graph.
	LayoutDirection core.Direction
	// LabelLabelSpacing is the space between two labels.
	LabelLabelSpacing float64

	// Placed tells that Side and Alignment have a value. Both are null in
	// Java until the label placer sets them, with the alignment reference
	// port, in one step. Without a value, Side is UNDEFINED and Alignment is
	// CENTER, which no code reads as LEFT, RIGHT, TOP or a real side.
	Placed bool
	// Side is the side of the node that the labels are on (getSide()).
	Side core.PortSide
	// Alignment is the alignment of the labels (getAlignment()).
	Alignment Alignment
	// AlignmentReferenceSLPort is the port that an alignment other than
	// CENTER is relative to (getAlignmentReferenceSLPort()), or nil.
	AlignmentReferenceSLPort *SelfLoopPort
}

// NewSelfHyperLoopLabels is new SelfHyperLoopLabels(slLoop).
func NewSelfHyperLoopLabels(slLoop *SelfHyperLoop) *SelfHyperLoopLabels {
	// The graph gives the layout direction and the spacing.
	lNode := slLoop.SLHolder.LNode
	graph := lNode.Graph()
	if graph == nil {
		lgraph.Bail("SelfHyperLoopLabels: the node of a self loop has no graph")
	}

	return &SelfHyperLoopLabels{
		LayoutDirection:   prop.Get(graph, lopts.Direction),
		LabelLabelSpacing: lgraph.IndividualOrInherited(lNode, lopts.SpacingLabelLabel),
	}
}

// SetID sets the public field id.
func (l *SelfHyperLoopLabels) SetID(id int) {
	l.ID = id
	l.IDAssigned = true
}

////////////////////////////////////////////////////////////////////////////
// LLabel Access

// AddLLabels is addLLabels(newLLabels).
func (l *SelfHyperLoopLabels) AddLLabels(newLLabels []*lgraph.LLabel) {
	for _, newLLabel := range newLLabels {
		l.LLabels = append(l.LLabels, newLLabel)
		l.updateSize(newLLabel)
	}
}

// updateSize is updateSize(newLLabel): the new size after the given label
// was added to this instance.
func (l *SelfHyperLoopLabels) updateSize(newLLabel *lgraph.LLabel) {
	newLLabelSize := &newLLabel.Size

	if l.LayoutDirection.IsHorizontal() {
		// The labels are stacked vertically.
		l.Size.X = max(l.Size.X, newLLabelSize.X)
		l.Size.Y += newLLabelSize.Y

		// Add the spacing between labels after the first label.
		if len(l.LLabels) > 1 {
			l.Size.Y += l.LabelLabelSpacing
		}
	} else {
		// The labels are stacked horizontally.
		l.Size.X += newLLabelSize.X
		l.Size.Y = max(l.Size.Y, newLLabelSize.Y)

		// Add the spacing between labels after the first label.
		if len(l.LLabels) > 1 {
			l.Size.X += l.LabelLabelSpacing
		}
	}
}

// ApplyPlacement is applyPlacement(offset): it gives each label its position
// in the bounding box.
func (l *SelfHyperLoopLabels) ApplyPlacement(offset *emath.KVector) {
	if l.LayoutDirection.IsHorizontal() {
		l.applyPlacementForHorizontalLayout(offset)
	} else {
		l.applyPlacementVerticalForVerticalLayout(offset)
	}
}

func (l *SelfHyperLoopLabels) applyPlacementForHorizontalLayout(offset *emath.KVector) {
	x := l.Position.X
	y := l.Position.Y

	for _, lLabel := range l.LLabels {
		labelPos := &lLabel.Pos

		// The x coordinate depends on the alignment and on the side.
		if l.Alignment == AlignmentLeft || l.Side == core.PortSideEast {
			labelPos.X = x
		} else if l.Alignment == AlignmentRight || l.Side == core.PortSideWest {
			labelPos.X = x + l.Size.X - lLabel.Size.X
		} else {
			// The alignment is CENTER.
			labelPos.X = x + float64((l.Size.X-lLabel.Size.X)/2)
		}

		labelPos.Y = y
		labelPos.Add(offset)

		y += lLabel.Size.Y + l.LabelLabelSpacing
	}
}

func (l *SelfHyperLoopLabels) applyPlacementVerticalForVerticalLayout(offset *emath.KVector) {
	x := l.Position.X
	y := l.Position.Y

	for _, lLabel := range l.LLabels {
		labelPos := &lLabel.Pos

		labelPos.X = x

		// The labels are aligned at the top, except on the north side.
		if l.Side == core.PortSideNorth {
			labelPos.Y = y + l.Size.Y - lLabel.Size.Y
		} else {
			labelPos.Y = y
		}

		labelPos.Add(offset)

		x += lLabel.Size.X + l.LabelLabelSpacing
	}
}

////////////////////////////////////////////////////////////////////////////
// Label Placement

// SetSideAndAlignment is the three calls setSide(side),
// setAlignment(alignment) and setAlignmentReferenceSLPort(reference), which
// the label placer makes together.
func (l *SelfHyperLoopLabels) SetSideAndAlignment(side core.PortSide, alignment Alignment, alignmentReferenceSLPort *SelfLoopPort) {
	l.Placed = true
	l.Side = side
	l.Alignment = alignment
	l.AlignmentReferenceSLPort = alignmentReferenceSLPort
}
