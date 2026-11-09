// Ported from plugins/org.eclipse.elk.alg.common/src/org/eclipse/elk/alg/common/nodespacing/cellsystem/ContainerCell.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package cellsystem

// ContainerCell is the abstract class ContainerCell: a cell that has other
// cells and lays them out.
type ContainerCell interface {
	Cell
	// LayoutChildrenHorizontally is layoutChildrenHorizontally(): it sets
	// the x coordinates and the widths of the children.
	LayoutChildrenHorizontally()
	// LayoutChildrenVertically is layoutChildrenVertically(): it sets the y
	// coordinates and the heights of the children.
	LayoutChildrenVertically()
}

// minWidthOfCell is ContainerCell.minWidthOfCell(cell,
// respectContributionFlag). It is 0 for no cell, for a cell that does not
// contribute when the flag is respected, and for an atomic cell whose
// content area has no width.
func minWidthOfCell(cell Cell, respectContributionFlag bool) float64 {
	if cell == nil {
		return 0
	}
	if respectContributionFlag && !cell.IsContributingToMinimumWidth() {
		return 0
	}
	if atomic, ok := cell.(*AtomicCell); ok {
		if atomic.minimumContentAreaSize.X == 0 {
			return 0
		}
	}
	return cell.MinimumWidth()
}

// minHeightOfCell is ContainerCell.minHeightOfCell(cell,
// respectContributionFlag).
func minHeightOfCell(cell Cell, respectContributionFlag bool) float64 {
	if cell == nil {
		return 0
	}
	if respectContributionFlag && !cell.IsContributingToMinimumHeight() {
		return 0
	}
	if atomic, ok := cell.(*AtomicCell); ok {
		if atomic.minimumContentAreaSize.Y == 0 {
			return 0
		}
	}
	return cell.MinimumHeight()
}

// applyHorizontalLayout is ContainerCell.applyHorizontalLayout(cell, x,
// width). It does nothing without a cell.
func applyHorizontalLayout(cell Cell, x, width float64) {
	if cell != nil {
		cellRect := cell.CellRectangle()
		cellRect.X = x
		cellRect.Width = width
	}
}

// applyVerticalLayout is ContainerCell.applyVerticalLayout(cell, y, height).
// It does nothing without a cell.
func applyVerticalLayout(cell Cell, y, height float64) {
	if cell != nil {
		cellRect := cell.CellRectangle()
		cellRect.Y = y
		cellRect.Height = height
	}
}
