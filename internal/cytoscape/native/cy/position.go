// Ported from cytoscape 3.34.0 (MIT license): src/collection/position.mjs
// and the functions that mark caches dirty of
// src/collection/dimensions/bounds.mjs.

package cy

// Position is ele.position(). A compound node first updates its bounds: its
// position is the centre of its content.
func (e *Element) Position() Point {
	e.cy.updateCompoundBounds([]*Element{e}, false)
	return e.position
}

// SetPosition is ele.position(newPos) for a node without children.
func (e *Element) SetPosition(newPos Point) {
	// beforePositionSet. A node with children would shift its children: the
	// layouts set the positions of the other nodes only.
	e.dirtyBoundingBoxCache()

	e.position = newPos

	// onSet
	e.dirtyCompoundBoundsCache()
}

// dirtyBoundingBoxCache is ele.dirtyBoundingBoxCache(): it drops the cached
// bounds, and the event bounds tells the renderer that the element needs new
// calculations.
func (e *Element) dirtyBoundingBoxCache() {
	e.bbCache = nil
	e.bbCachePosKey = 0
	e.bodyBounds = nil
	e.labelBounds.all = nil
	e.emitBounds()
}

// emitBounds is the listener of the renderer for the event bounds
// (registerCalculationListeners). A destroyed renderer has no listener.
func (e *Element) emitBounds() {
	if e.cy.destroyed {
		return
	}
	e.rstyle.clean = false
}

// dirtyCompoundBoundsCache is ele.dirtyCompoundBoundsCache(): the compound
// nodes around the element need new bounds.
func (e *Element) dirtyCompoundBoundsCache() {
	if !e.cy.hasCompoundNodes {
		return
	}
	// forEachUp: the element and the nodes that contain it.
	for ele := e; ele != nil; ele = ele.parent {
		if ele.IsParent() {
			ele.compoundBoundsClean = false
			ele.bbCache = nil
			ele.emitBounds()
		}
		if !ele.isNode {
			break
		}
	}
}
