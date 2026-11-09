// Ported from the methods getLabelsBounds(PortAdapter) and
// computeInsidePart, and from the constants DEFAULT_MIN_WIDTH and
// DEFAULT_MIN_HEIGHT, of
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/ElkUtil.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package algorithm

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
)

// TODO(core): move the content of this file to the package that ports
// org.eclipse.elk.core.util.ElkUtil.

const (
	// elkUtilDefaultMinWidth is ElkUtil.DEFAULT_MIN_WIDTH.
	elkUtilDefaultMinWidth = 20.0
	// elkUtilDefaultMinHeight is ElkUtil.DEFAULT_MIN_HEIGHT.
	elkUtilDefaultMinHeight = 20.0
)

// getLabelsBounds is ElkUtil.getLabelsBounds(port): the bounding box of the
// labels of a port, relative to the port. It is an empty rectangle for a
// port without labels.
func getLabelsBounds(port adapters.PortAdapter) *emath.ElkRectangle {
	var bounds *emath.ElkRectangle
	for _, label := range port.Labels() {
		currentLabelBounds := emath.NewElkRectangle(
			label.Position().X,
			label.Position().Y,
			label.Size().X,
			label.Size().Y)
		if bounds == nil {
			bounds = currentLabelBounds
		} else {
			bounds.Union(currentLabelBounds)
		}
	}
	if bounds == nil {
		bounds = &emath.ElkRectangle{}
	}
	return bounds
}

// computeInsidePartOfPort is ElkUtil.computeInsidePart(port,
// portBorderOffset): the length of the part of the labels of the port that
// is inside the node.
func computeInsidePartOfPort(port adapters.PortAdapter, portBorderOffset float64) float64 {
	labelBounds := getLabelsBounds(port)
	return computeInsidePart(emath.NewKVector(labelBounds.X, labelBounds.Y),
		emath.NewKVector(labelBounds.Width, labelBounds.Height), port.Size(), portBorderOffset, port.Side())
}

// computeInsidePart is ElkUtil.computeInsidePart(labelPosition, labelSize,
// portSize, portBorderOffset, portSide).
func computeInsidePart(labelPosition, labelSize, portSize *emath.KVector, portBorderOffset float64,
	portSide core.PortSide) float64 {

	insidePart := 0.0
	switch portSide {
	case core.PortSideNorth:
		insidePart = max(0, labelSize.Y+labelPosition.Y-(portSize.Y+portBorderOffset))
	case core.PortSideSouth:
		insidePart = max(0, -labelPosition.Y-portBorderOffset)
	case core.PortSideEast:
		insidePart = max(0, -labelPosition.X-portBorderOffset)
	case core.PortSideWest:
		insidePart = max(0, labelSize.X+labelPosition.X-(portSize.X+portBorderOffset))
	}
	return insidePart
}
