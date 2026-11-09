// Ported from PortSide.java, PortConstraints.java and Direction.java in
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/options of Eclipse
// ELK 0.9.1 (EPL-2.0).

package core

// Right is PortSide.right(): the next side clockwise.
func (e PortSide) Right() PortSide {
	switch e {
	case PortSideNorth:
		return PortSideEast
	case PortSideEast:
		return PortSideSouth
	case PortSideSouth:
		return PortSideWest
	case PortSideWest:
		return PortSideNorth
	}
	return PortSideUndefined
}

// Left is PortSide.left(): the next side counter-clockwise.
func (e PortSide) Left() PortSide {
	switch e {
	case PortSideNorth:
		return PortSideWest
	case PortSideEast:
		return PortSideNorth
	case PortSideSouth:
		return PortSideEast
	case PortSideWest:
		return PortSideSouth
	}
	return PortSideUndefined
}

// Opposed is PortSide.opposed().
func (e PortSide) Opposed() PortSide {
	switch e {
	case PortSideNorth:
		return PortSideSouth
	case PortSideEast:
		return PortSideWest
	case PortSideSouth:
		return PortSideNorth
	case PortSideWest:
		return PortSideEast
	}
	return PortSideUndefined
}

// AreAdjacent is PortSide.areAdjacent(other).
func (e PortSide) AreAdjacent(other PortSide) bool {
	if e == PortSideUndefined {
		return false
	}
	return e.Left() == other || e.Right() == other
}

// PortSideFromDirection is PortSide.fromDirection(direction).
func PortSideFromDirection(direction Direction) PortSide {
	switch direction {
	case DirectionUp:
		return PortSideNorth
	case DirectionRight:
		return PortSideEast
	case DirectionDown:
		return PortSideSouth
	case DirectionLeft:
		return PortSideWest
	}
	return PortSideUndefined
}

// PortSideIsVertical is PortSide.isVertical(side).
func PortSideIsVertical(side PortSide) bool { return side == PortSideNorth || side == PortSideSouth }

// PortSideIsHorizontal is PortSide.isHorizontal(side).
func PortSideIsHorizontal(side PortSide) bool { return side == PortSideWest || side == PortSideEast }

// IsPosFixed is PortConstraints.isPosFixed().
func (e PortConstraints) IsPosFixed() bool { return e == PortConstraintsFixedPos }

// IsRatioFixed is PortConstraints.isRatioFixed().
func (e PortConstraints) IsRatioFixed() bool { return e == PortConstraintsFixedRatio }

// IsOrderFixed is PortConstraints.isOrderFixed().
func (e PortConstraints) IsOrderFixed() bool {
	return e == PortConstraintsFixedOrder || e == PortConstraintsFixedRatio || e == PortConstraintsFixedPos
}

// IsSideFixed is PortConstraints.isSideFixed().
func (e PortConstraints) IsSideFixed() bool {
	return e != PortConstraintsFree && e != PortConstraintsUndefined
}

// IsHorizontal is Direction.isHorizontal().
func (e Direction) IsHorizontal() bool { return e == DirectionLeft || e == DirectionRight }

// IsVertical is Direction.isVertical().
func (e Direction) IsVertical() bool { return e == DirectionUp || e == DirectionDown }

// Opposite is Direction.opposite().
func (e Direction) Opposite() Direction {
	switch e {
	case DirectionLeft:
		return DirectionRight
	case DirectionRight:
		return DirectionLeft
	case DirectionUp:
		return DirectionDown
	case DirectionDown:
		return DirectionUp
	}
	return DirectionUndefined
}
