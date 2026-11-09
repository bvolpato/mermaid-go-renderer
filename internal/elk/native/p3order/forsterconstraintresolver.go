// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p3order/ForsterConstraintResolver.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package p3order

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/iprops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/counting"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// ForsterConstraintResolver is the class ForsterConstraintResolver: it
// detects and resolves violated constraints, after Michael Forster, "A fast
// and simple heuristic for constrained two-level crossing reduction". It
// relies on the assumption that all node groups have well-defined barycenter
// values and are already sorted by these barycenter values.
//
// It must be initialized with counting.Init.
type ForsterConstraintResolver struct {
	counting.DefaultInit

	// constraintsBetweenNonDummies tells whether there are successor
	// constraints between non-dummies.
	constraintsBetweenNonDummies bool
	// layoutUnits has the layout units for handling dummy nodes for north /
	// south ports: for each layout unit its nodes, in the order in which
	// they were added. The Java multimap has no value twice. Each node is
	// added one time, so a slice is the same.
	layoutUnits map[*lgraph.LNode][]*lgraph.LNode
	// barycenterStates has the barycenter values of every node in the
	// graph, indexed by layer.id and node.id.
	barycenterStates [][]*BarycenterState
	// constraintGroups has the constraint groups, indexed by layer.id and
	// node.id.
	constraintGroups [][]*ConstraintGroup
}

// NewForsterConstraintResolver is new ForsterConstraintResolver(
// currentNodeOrder).
func NewForsterConstraintResolver(currentNodeOrder [][]*lgraph.LNode) *ForsterConstraintResolver {
	r := &ForsterConstraintResolver{}
	if len(currentNodeOrder) > 0 && len(currentNodeOrder[0]) > 0 {
		r.constraintsBetweenNonDummies = prop.Get(currentNodeOrder[0][0].Graph(),
			iprops.InLayerSuccessorConstraintsBetweenNonDummies)
	}
	r.barycenterStates = make([][]*BarycenterState, len(currentNodeOrder))
	r.constraintGroups = make([][]*ConstraintGroup, len(currentNodeOrder))
	r.layoutUnits = map[*lgraph.LNode][]*lgraph.LNode{}
	return r
}

// InitAtLayerLevel is initAtLayerLevel(l, nodeOrder).
func (r *ForsterConstraintResolver) InitAtLayerLevel(l int, nodeOrder [][]*lgraph.LNode) {
	r.barycenterStates[l] = make([]*BarycenterState, len(nodeOrder[l]))
	r.constraintGroups[l] = make([]*ConstraintGroup, len(nodeOrder[l]))
}

// InitAtNodeLevel is initAtNodeLevel(l, n, nodeOrder).
func (r *ForsterConstraintResolver) InitAtNodeLevel(l, n int, nodeOrder [][]*lgraph.LNode) {
	r.initAtNodeLevel(nodeOrder[l][n], true)
}

// initAtNodeLevel inits the data structures for the given node. With
// fullInit it also initializes the barycenter state and the layout unit.
// This is needed one time only, when the resolver is first initialized.
func (r *ForsterConstraintResolver) initAtNodeLevel(node *lgraph.LNode, fullInit bool) {
	layerIndex := node.Layer().ID
	nodeIndex := node.ID

	r.constraintGroups[layerIndex][nodeIndex] = r.newConstraintGroup(node)

	if fullInit {
		r.barycenterStates[layerIndex][nodeIndex] = NewBarycenterState(node)

		layoutUnit := prop.Get(node, iprops.InLayerLayoutUnit)
		if layoutUnit != nil {
			r.layoutUnits[layoutUnit] = append(r.layoutUnits[layoutUnit], node)
		}
	}
}

// BarycenterStates is getBarycenterStates().
func (r *ForsterConstraintResolver) BarycenterStates() [][]*BarycenterState {
	return r.barycenterStates
}

// ProcessConstraints is processConstraints(nodes): it finds and handles
// violated in-layer successor constraints. It changes the order of the
// given nodes in place.
func (r *ForsterConstraintResolver) ProcessConstraints(nodes []*lgraph.LNode) {
	// If there are successor constraints between regular (or normal) nodes,
	// we have to apply a two-stage process: in stage 1, we resolve
	// constraints between the normal nodes. In stage 2, we ensure that all
	// of their north / south port dummies are in order and that no layout
	// units overlap. For the latter to work properly, we need the order of
	// normal nodes to be fixed, hence the first stage.
	if r.constraintsBetweenNonDummies {
		r.processConstraints(nodes, true)
		for _, node := range nodes {
			r.initAtNodeLevel(node, false)
		}
	}

	r.processConstraints(nodes, false)
}

func (r *ForsterConstraintResolver) processConstraints(nodes []*lgraph.LNode, onlyBetweenNormalNodes bool) {
	groups := make([]*ConstraintGroup, 0, len(nodes))
	for _, node := range nodes {
		groups = append(groups, r.constraintGroups[node.Layer().ID][node.ID])
	}

	// Build the constraints graph
	r.buildConstraintsGraph(groups, onlyBetweenNormalNodes)

	// Find violated vertices
	for {
		first, second := r.findViolatedConstraint(groups)
		if first == nil {
			break
		}
		groups = r.handleViolatedConstraint(first, second, groups)
	}

	// Apply the determined order
	index := 0
	for _, group := range groups {
		for _, node := range group.Nodes() {
			if index >= len(nodes) {
				lgraph.Bail("ForsterConstraintResolver: the constraint groups have more nodes than the layer")
			}
			nodes[index] = node
			index++
			state := r.stateOf(node)
			state.Barycenter, state.HasBarycenter = group.Barycenter()
		}
	}
	if index != len(nodes) {
		lgraph.Bail("ForsterConstraintResolver: the constraint groups have fewer nodes than the layer")
	}
}

// buildConstraintsGraph builds the constraint graph for the given vertices.
// The constraint graph is created from the predefined in-layer successor
// constraints and the layout units.
func (r *ForsterConstraintResolver) buildConstraintsGraph(groups []*ConstraintGroup, onlyBetweenNormalNodes bool) {
	// Reset the constraint fields
	for _, group := range groups {
		group.ResetOutgoingConstraints()
		group.incomingConstraintsCount = 0
	}

	// Iterate through the vertices, adding the necessary constraints
	var lastNonDummyNode *lgraph.LNode
	for _, group := range groups {
		// at this stage all groups should consist of a single node
		node := group.Node()

		// We may want to skip this
		if onlyBetweenNormalNodes && node.Type != lgraph.NodeTypeNormal {
			continue
		}

		// Add the constraints given by the vertex's node. The read stores
		// the empty default list in the node, as in Java.
		for _, successor := range prop.Get(node, iprops.InLayerSuccessorConstraints) {
			if !onlyBetweenNormalNodes || successor.Type == lgraph.NodeTypeNormal {
				group.addOutgoingConstraint(r.groupOf(successor))
				r.groupOf(successor).incomingConstraintsCount++
			}
		}

		// Check if we're processing a a normal, none-dummy node. If this is
		// the case, we may want to insert additional constraints between
		// the previous normal node's layout unit and the current node's
		// layout unit to be sure they won't mix. This requires that the
		// normal nodes won't change order, however.
		if !onlyBetweenNormalNodes && node.Type == lgraph.NodeTypeNormal {
			// If we already processed another normal, non-dummy node, we
			// need to add constraints from all of that other node's layout
			// unit's vertices to this node's layout unit's vertices
			if lastNonDummyNode != nil {
				for _, lastUnitNode := range r.layoutUnits[lastNonDummyNode] {
					for _, currentUnitNode := range r.layoutUnits[node] {
						r.groupOf(lastUnitNode).addOutgoingConstraint(r.groupOf(currentUnitNode))
						r.groupOf(currentUnitNode).incomingConstraintsCount++
					}
				}
			}

			lastNonDummyNode = node
		}
	}
}

// findViolatedConstraint returns a violated constraint, if any is left. The
// detection is based on the barycenter values of the node groups, so the
// node groups must be sorted by their barycenter values. The two groups are
// returned in the order they should appear in, not in the order that
// violates their constraint. first is nil when no constraint is violated.
func (r *ForsterConstraintResolver) findViolatedConstraint(groups []*ConstraintGroup) (first, second *ConstraintGroup) {
	var activeGroups []*ConstraintGroup

	// Iterate through the constrained vertices
	for _, group := range groups {
		// The Java code unboxes the barycenter here.
		group.barycenterValue()
		group.ResetIncomingConstraints()

		// Find sources of the constraint graph to start the constraints
		// check
		if group.HasOutgoingConstraints() && group.incomingConstraintsCount == 0 {
			activeGroups = append(activeGroups, group)
		}
	}

	// Iterate through the active node groups to find one with violated
	// constraints
	for len(activeGroups) != 0 {
		group := activeGroups[0]
		activeGroups = activeGroups[1:]

		// See if we can find a violated constraint
		if group.HasIncomingConstraints() {
			for _, predecessor := range group.incomingConstraints {
				// The method floatValue of GWT returns the double.
				if predecessor.barycenterValue() == group.barycenterValue() {
					if indexOfGroup(groups, predecessor) > indexOfGroup(groups, group) {
						// The predecessor has equal barycenter, but higher
						// index
						return predecessor, group
					}
				} else if predecessor.barycenterValue() > group.barycenterValue() {
					// The predecessor has greater barycenter and thus also
					// higher index
					return predecessor, group
				}
			}
		}

		// No violated constraints; add outgoing constraints to the
		// respective incoming list
		for _, successor := range group.OutgoingConstraints() {
			successor.incomingConstraintsSet = true
			successor.incomingConstraints = append(successor.incomingConstraints, nil)
			copy(successor.incomingConstraints[1:], successor.incomingConstraints)
			successor.incomingConstraints[0] = group

			if successor.incomingConstraintsCount == len(successor.incomingConstraints) {
				activeGroups = append(activeGroups, successor)
			}
		}
	}

	// No violated constraints found
	return nil, nil
}

func indexOfGroup(groups []*ConstraintGroup, group *ConstraintGroup) int {
	for i, g := range groups {
		if g == group {
			return i
		}
	}
	return -1
}

// handleViolatedConstraint handles the case of a violated constraint. The
// node groups must be sorted by their barycenter values. It returns the list
// of node groups, which is smaller by one element, since two node groups
// have been unified, and which is still sorted by barycenter values.
func (r *ForsterConstraintResolver) handleViolatedConstraint(firstNodeGroup, secondNodeGroup *ConstraintGroup, nodeGroups []*ConstraintGroup) []*ConstraintGroup {
	// Create a new vertex from the two constrain-violating vertices; this
	// also automatically calculates the new vertex's barycenter value
	newNodeGroup := r.newConstraintGroupOf(firstNodeGroup, secondNodeGroup)

	// Iterate through the vertices. Remove the old vertices. Insert the new
	// one according to the barycenter value, thereby keeping the list
	// sorted. Along the way, constraint relationships will be updated. i is
	// the cursor of the list iterator of the Java code.
	alreadyInserted := false
	for i := 0; i < len(nodeGroups); {
		nodeGroup := nodeGroups[i]
		i++

		if nodeGroup == firstNodeGroup || nodeGroup == secondNodeGroup {
			// Remove the two node groups with violated constraint from the
			// list
			nodeGroups = append(nodeGroups[:i-1], nodeGroups[i:]...)
			i--
		} else if !alreadyInserted && nodeGroup.barycenterValue() > newNodeGroup.barycenterValue() {
			// If we haven't inserted the new node group into the list
			// already, do that now. Note: we're not calling next() again.
			// This means that during the next iteration, we will again be
			// looking at the current node group. But then, alreadyInserted
			// will be true and we can look at that node group's outgoing
			// constraints.
			nodeGroups = append(nodeGroups, nil)
			copy(nodeGroups[i:], nodeGroups[i-1:])
			nodeGroups[i-1] = newNodeGroup

			alreadyInserted = true
		} else if nodeGroup.HasOutgoingConstraints() {
			// Check if the vertex has any constraints with the former two
			// vertices
			firstNodeGroupConstraint := nodeGroup.removeOutgoingConstraint(firstNodeGroup)
			secondNodeGroupConstraint := nodeGroup.removeOutgoingConstraint(secondNodeGroup)

			if firstNodeGroupConstraint || secondNodeGroupConstraint {
				nodeGroup.addOutgoingConstraint(newNodeGroup)
				newNodeGroup.incomingConstraintsCount++
			}
		}
	}

	// If we haven't inserted the new node group already, add it to the end
	if !alreadyInserted {
		nodeGroups = append(nodeGroups, newNodeGroup)
	}
	return nodeGroups
}

func (r *ForsterConstraintResolver) groupOf(node *lgraph.LNode) *ConstraintGroup {
	return r.constraintGroups[node.Layer().ID][node.ID]
}

func (r *ForsterConstraintResolver) stateOf(node *lgraph.LNode) *BarycenterState {
	return r.barycenterStates[node.Layer().ID][node.ID]
}

// ConstraintGroup is the class ForsterConstraintResolver.ConstraintGroup: a
// group of one node or more nodes that are placed next to each other.
type ConstraintGroup struct {
	resolver *ForsterConstraintResolver

	// summedWeight is the sum of the node weights.
	summedWeight float64
	// degree is the number of ports relevant to the barycenter calculation.
	degree int
	// nodes has the nodes of the group.
	nodes []*lgraph.LNode
	// outgoingConstraints has the outgoing constraints. The Java field can
	// be null: outgoingConstraintsSet is false then.
	outgoingConstraints    []*ConstraintGroup
	outgoingConstraintsSet bool
	// incomingConstraints has the incoming constraints. The Java field can
	// be null: incomingConstraintsSet is false then.
	incomingConstraints    []*ConstraintGroup
	incomingConstraintsSet bool
	// incomingConstraintsCount is the number of incoming constraints.
	incomingConstraintsCount int
}

// newConstraintGroup is new ConstraintGroup(node).
func (r *ForsterConstraintResolver) newConstraintGroup(node *lgraph.LNode) *ConstraintGroup {
	return &ConstraintGroup{resolver: r, nodes: []*lgraph.LNode{node}}
}

// newConstraintGroupOf is new ConstraintGroup(nodeGroup1, nodeGroup2): the
// concatenation of the two groups. The incoming constraints count is zero.
// The lists of successors are merged, and the incoming count of a successor
// of both groups is made one smaller. The new barycenter comes from the
// barycenters of the two groups.
func (r *ForsterConstraintResolver) newConstraintGroupOf(nodeGroup1, nodeGroup2 *ConstraintGroup) *ConstraintGroup {
	g := &ConstraintGroup{resolver: r}
	// create a combined nodes array
	g.nodes = make([]*lgraph.LNode, 0, len(nodeGroup1.nodes)+len(nodeGroup2.nodes))
	g.nodes = append(g.nodes, nodeGroup1.nodes...)
	g.nodes = append(g.nodes, nodeGroup2.nodes...)

	// Add constraints, taking care not to add any constraints to vertex1 or
	// vertex2 and to decrement the incoming constraints count of those that
	// are successors to both
	if nodeGroup1.outgoingConstraintsSet {
		g.outgoingConstraints = append([]*ConstraintGroup{}, nodeGroup1.outgoingConstraints...)
		g.outgoingConstraintsSet = true
		g.removeOutgoingConstraint(nodeGroup2)
		if nodeGroup2.outgoingConstraintsSet {
			for _, candidate := range nodeGroup2.outgoingConstraints {
				if candidate == nodeGroup1 {
					continue
				} else if indexOfGroup(g.outgoingConstraints, candidate) >= 0 {
					// The candidate was in both vertices' successor list
					candidate.incomingConstraintsCount--
				} else {
					g.outgoingConstraints = append(g.outgoingConstraints, candidate)
				}
			}
		}
	} else if nodeGroup2.outgoingConstraintsSet {
		g.outgoingConstraints = append([]*ConstraintGroup{}, nodeGroup2.outgoingConstraints...)
		g.outgoingConstraintsSet = true
		g.removeOutgoingConstraint(nodeGroup1)
	}

	g.summedWeight = nodeGroup1.summedWeight + nodeGroup2.summedWeight
	g.degree = nodeGroup1.degree + nodeGroup2.degree

	b1, has1 := nodeGroup1.Barycenter()
	b2, has2 := nodeGroup2.Barycenter()
	if g.degree > 0 {
		g.SetBarycenter(g.summedWeight/float64(g.degree), true)
	} else if has1 && has2 {
		g.SetBarycenter((b1+b2)/2, true)
	} else if has1 {
		g.SetBarycenter(b1, true)
	} else if has2 {
		g.SetBarycenter(b2, true)
	}
	return g
}

// SetBarycenter is setBarycenter(barycenter). has is false for the Java
// null.
func (g *ConstraintGroup) SetBarycenter(barycenter float64, has bool) {
	for _, node := range g.nodes {
		state := g.resolver.stateOf(node)
		state.Barycenter, state.HasBarycenter = barycenter, has
	}
}

// Barycenter is getBarycenter(): the barycenter of the group. has is false
// for the Java null.
func (g *ConstraintGroup) Barycenter() (barycenter float64, has bool) {
	state := g.resolver.stateOf(g.nodes[0])
	return state.Barycenter, state.HasBarycenter
}

// barycenterValue is getBarycenter() where the Java code unboxes the value.
// elkjs throws for null, so the layout stops here.
func (g *ConstraintGroup) barycenterValue() float64 {
	barycenter, has := g.Barycenter()
	if !has {
		lgraph.Bail("ForsterConstraintResolver: a constraint group has no barycenter")
	}
	return barycenter
}

// OutgoingConstraints is getOutgoingConstraints(): the list of outgoing
// constraints. It makes the list if there is none.
func (g *ConstraintGroup) OutgoingConstraints() []*ConstraintGroup {
	g.outgoingConstraintsSet = true
	return g.outgoingConstraints
}

// addOutgoingConstraint is getOutgoingConstraints().add(group).
func (g *ConstraintGroup) addOutgoingConstraint(group *ConstraintGroup) {
	g.outgoingConstraintsSet = true
	g.outgoingConstraints = append(g.outgoingConstraints, group)
}

// removeOutgoingConstraint is getOutgoingConstraints().remove(group): it
// removes the first occurrence. It returns true when the list had the
// group.
func (g *ConstraintGroup) removeOutgoingConstraint(group *ConstraintGroup) bool {
	g.outgoingConstraintsSet = true
	i := indexOfGroup(g.outgoingConstraints, group)
	if i < 0 {
		return false
	}
	g.outgoingConstraints = append(g.outgoingConstraints[:i], g.outgoingConstraints[i+1:]...)
	return true
}

// ResetOutgoingConstraints is resetOutgoingConstraints(): it sets the list
// of outgoing constraints to null.
func (g *ConstraintGroup) ResetOutgoingConstraints() {
	g.outgoingConstraints = nil
	g.outgoingConstraintsSet = false
}

// HasOutgoingConstraints is hasOutgoingConstraints().
func (g *ConstraintGroup) HasOutgoingConstraints() bool {
	return g.outgoingConstraintsSet && len(g.outgoingConstraints) > 0
}

// IncomingConstraints is getIncomingConstraints(): the list of incoming
// constraints. It makes the list if there is none.
func (g *ConstraintGroup) IncomingConstraints() []*ConstraintGroup {
	g.incomingConstraintsSet = true
	return g.incomingConstraints
}

// ResetIncomingConstraints is resetIncomingConstraints(): it sets the list
// of incoming constraints to null.
func (g *ConstraintGroup) ResetIncomingConstraints() {
	g.incomingConstraints = nil
	g.incomingConstraintsSet = false
}

// HasIncomingConstraints is hasIncomingConstraints().
func (g *ConstraintGroup) HasIncomingConstraints() bool {
	return g.incomingConstraintsSet && len(g.incomingConstraints) > 0
}

// Nodes is getNodes().
func (g *ConstraintGroup) Nodes() []*lgraph.LNode { return g.nodes }

// Node is getNode(). It may only be used for a group with exactly one node.
func (g *ConstraintGroup) Node() *lgraph.LNode { return g.nodes[0] }
