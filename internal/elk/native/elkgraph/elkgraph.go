// Ported from the ELK graph model (ElkNode, ElkPort, ElkLabel, ElkEdge,
// ElkEdgeSection in plugins/org.eclipse.elk.graph/src/org/eclipse/elk/graph
// and its impl package) and from
// plugins/org.eclipse.elk.graph/src/org/eclipse/elk/graph/util/ElkGraphUtil.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

// Package elkgraph has the ELK graph: the model that the JSON importer
// builds, that the layout algorithms read and that receives the layout.
//
// The Java model is an EMF model. This package keeps what the layered
// algorithm and the JSON format need: the containment lists and the lists of
// incident edges, each in the order in which EMF fills it.
package elkgraph

import (
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// ID is the id of an element in the JSON graph: a string or an integer. The
// string "1" and the integer 1 are different ids.
type ID struct {
	IsInt bool
	Str   string
	Int   int
}

// String is the text that Java gives for the id in a string concatenation.
func (id ID) String() string {
	if id.IsInt {
		return strconv.Itoa(id.Int)
	}
	return id.Str
}

// JSON returns the id as a JSON value.
func (id ID) JSON() any {
	if id.IsInt {
		return float64(id.Int)
	}
	return id.Str
}

// Shape is ElkConnectableShape: a node or a port, an end of an edge.
type Shape interface {
	prop.Holder
	// ShapeNode is ElkGraphUtil.connectableShapeToNode: the node itself, or
	// the node of a port.
	ShapeNode() *Node
	shapeID() ID
	addOutgoing(e *Edge)
	addIncoming(e *Edge)
}

// Node is ElkNode.
type Node struct {
	prop.MapHolder
	// Identifier is the member id of the JSON object as a string.
	Identifier          string
	X, Y, Width, Height float64
	Parent              *Node
	Children            []*Node
	Ports               []*Port
	Labels              []*Label
	// ContainedEdges has the edges that the node contains, in the order in
	// which they moved into the node.
	ContainedEdges []*Edge
	OutgoingEdges  []*Edge
	IncomingEdges  []*Edge
	// JSON is the object of the node in the JSON graph.
	JSON *js.Obj[any]
	id   ID
}

// Port is ElkPort.
type Port struct {
	prop.MapHolder
	Identifier          string
	X, Y, Width, Height float64
	Parent              *Node
	Labels              []*Label
	OutgoingEdges       []*Edge
	IncomingEdges       []*Edge
	JSON                *js.Obj[any]
	id                  ID
}

// Label is ElkLabel.
type Label struct {
	prop.MapHolder
	// Text is the text. HasText is false for a label without the member
	// text: its Java text is null.
	Text                string
	HasText             bool
	X, Y, Width, Height float64
	// Parent is the *Node, *Port or *Edge that has the label.
	Parent any
	JSON   *js.Obj[any]
}

// Edge is ElkEdge.
type Edge struct {
	prop.MapHolder
	Identifier     string
	ContainingNode *Node
	Sources        []Shape
	Targets        []Shape
	Sections       []*EdgeSection
	Labels         []*Label
	JSON           *js.Obj[any]
	id             ID
}

// EdgeSection is ElkEdgeSection.
type EdgeSection struct {
	StartX, StartY, EndX, EndY float64
	BendPoints                 []emath.KVector
	IncomingShape              Shape
	OutgoingShape              Shape
}

// ShapeNode returns the node.
func (n *Node) ShapeNode() *Node { return n }

// ShapeNode returns the node of the port.
func (p *Port) ShapeNode() *Node { return p.Parent }

func (n *Node) shapeID() ID { return n.id }
func (p *Port) shapeID() ID { return p.id }

func (n *Node) addOutgoing(e *Edge) { n.OutgoingEdges = append(n.OutgoingEdges, e) }
func (n *Node) addIncoming(e *Edge) { n.IncomingEdges = append(n.IncomingEdges, e) }
func (p *Port) addOutgoing(e *Edge) { p.OutgoingEdges = append(p.OutgoingEdges, e) }
func (p *Port) addIncoming(e *Edge) { p.IncomingEdges = append(p.IncomingEdges, e) }

// ElkRef names the node for the property origin: "n:<id>".
func (n *Node) ElkRef() string { return "n:" + n.Identifier }

// ElkIdentifier is getIdentifier().
func (n *Node) ElkIdentifier() string { return n.Identifier }

// ElkRef names the port for the property origin: "p:<id>".
func (p *Port) ElkRef() string { return "p:" + p.Identifier }

// ElkIdentifier is getIdentifier().
func (p *Port) ElkIdentifier() string { return p.Identifier }

// ElkRef names the edge for the property origin: "e:<id>".
func (e *Edge) ElkRef() string { return "e:" + e.Identifier }

// ElkIdentifier is getIdentifier().
func (e *Edge) ElkIdentifier() string { return e.Identifier }

// ElkRef names the label for the property origin: "l:<parent>#<index>".
func (l *Label) ElkRef() string {
	parent, index := "?", -1
	var labels []*Label
	switch p := l.Parent.(type) {
	case *Node:
		parent, labels = p.ElkRef(), p.Labels
	case *Port:
		parent, labels = p.ElkRef(), p.Labels
	case *Edge:
		parent, labels = p.ElkRef(), p.Labels
	}
	for i, x := range labels {
		if x == l {
			index = i
		}
	}
	return "l:" + parent + "#" + strconv.Itoa(index)
}

// ElkIdentifier is getIdentifier(). The harness does not record it.
func (l *Label) ElkIdentifier() string { return "" }

// SetContainingNode is ElkEdge.setContainingNode: it moves the edge to the
// end of the contained edges of the node. It keeps the place of an edge that
// the node contains.
func (e *Edge) SetContainingNode(n *Node) {
	if e.ContainingNode == n {
		return
	}
	if old := e.ContainingNode; old != nil {
		for i, x := range old.ContainedEdges {
			if x == e {
				old.ContainedEdges = append(old.ContainedEdges[:i], old.ContainedEdges[i+1:]...)
				break
			}
		}
	}
	e.ContainingNode = n
	if n != nil {
		n.ContainedEdges = append(n.ContainedEdges, e)
	}
}

// AddSource is getSources().add(shape). The list is unique: a second add of
// a shape does nothing.
func (e *Edge) AddSource(s Shape) {
	for _, x := range e.Sources {
		if x == s {
			return
		}
	}
	e.Sources = append(e.Sources, s)
	s.addOutgoing(e)
}

// AddTarget is getTargets().add(shape).
func (e *Edge) AddTarget(s Shape) {
	for _, x := range e.Targets {
		if x == s {
			return
		}
	}
	e.Targets = append(e.Targets, s)
	s.addIncoming(e)
}

// IsHyperedge is isHyperedge().
func (e *Edge) IsHyperedge() bool { return !(len(e.Sources) <= 1 && len(e.Targets) <= 1) }

// IsConnected is isConnected().
func (e *Edge) IsConnected() bool { return len(e.Sources) != 0 && len(e.Targets) != 0 }

// incidentShapes is ElkGraphUtil.allIncidentShapes: the sources, then the
// targets.
func (e *Edge) incidentShapes() []Shape {
	out := make([]Shape, 0, len(e.Sources)+len(e.Targets))
	out = append(out, e.Sources...)
	return append(out, e.Targets...)
}

// IsHierarchical is isHierarchical().
func (e *Edge) IsHierarchical() bool {
	var common *Node
	first := true
	for _, s := range e.incidentShapes() {
		node := s.ShapeNode()
		if first {
			common, first = node.Parent, false
		} else if common != node.Parent {
			return true
		}
	}
	return false
}

// IsSelfloop is isSelfloop().
func (e *Edge) IsSelfloop() bool {
	var common *Node
	for _, s := range e.incidentShapes() {
		node := s.ShapeNode()
		if common == nil {
			common = node
		} else if common != node {
			return false
		}
	}
	return true
}

// SourceNode is ElkGraphUtil.getSourceNode for an edge with one source.
func (e *Edge) SourceNode() *Node { return e.Sources[0].ShapeNode() }

// TargetNode is ElkGraphUtil.getTargetNode for an edge with one target.
func (e *Edge) TargetNode() *Node { return e.Targets[0].ShapeNode() }

// IsDescendant is ElkGraphUtil.isDescendant(child, ancestor).
func IsDescendant(child, ancestor *Node) bool {
	current := child
	for current.Parent != nil {
		current = current.Parent
		if current == ancestor {
			return true
		}
	}
	return false
}

// ancestors is the AncestorIterator of ElkGraphUtil with the node itself.
func ancestors(n *Node) []*Node {
	var out []*Node
	for ; n != nil; n = n.Parent {
		out = append(out, n)
	}
	return out
}

// FindLowestCommonAncestor is ElkGraphUtil.findLowestCommonAncestor.
func FindLowestCommonAncestor(node1, node2 *Node) *Node {
	a1, a2 := ancestors(node1), ancestors(node2)
	var common *Node
	for i, j := len(a1)-1, len(a2)-1; i >= 0 && j >= 0; i, j = i-1, j-1 {
		if a1[i] != a2[j] {
			break
		}
		common = a1[i]
	}
	return common
}

// FindBestEdgeContainment is ElkGraphUtil.findBestEdgeContainment. The edge
// must have a source or a target. The result is nil when the ends have no
// common ancestor.
func FindBestEdgeContainment(e *Edge) *Node {
	switch len(e.Sources) + len(e.Targets) {
	case 0:
		return nil
	case 1:
		if len(e.Sources) == 0 {
			return e.Targets[0].ShapeNode().Parent
		}
		return e.Sources[0].ShapeNode().Parent
	}
	if len(e.Sources) == 1 && len(e.Targets) == 1 {
		sourceNode := e.Sources[0].ShapeNode()
		targetNode := e.Targets[0].ShapeNode()
		if sourceNode.Parent == targetNode.Parent {
			return sourceNode.Parent
		} else if sourceNode == targetNode.Parent {
			return sourceNode
		} else if targetNode == sourceNode.Parent {
			return targetNode
		}
	}
	shapes := e.incidentShapes()
	common := shapes[0].ShapeNode()
	for _, s := range shapes[1:] {
		incident := s.ShapeNode()
		if incident != common && !IsDescendant(incident, common) {
			if incident.Parent == common.Parent {
				common = incident.Parent
			} else {
				common = FindLowestCommonAncestor(common, incident)
				if common == nil {
					return nil
				}
			}
		}
	}
	return common
}

// AllOutgoingEdges is ElkGraphUtil.allOutgoingEdges(node): the outgoing
// edges of the node, then of each port.
func AllOutgoingEdges(n *Node) []*Edge {
	out := append([]*Edge(nil), n.OutgoingEdges...)
	for _, p := range n.Ports {
		out = append(out, p.OutgoingEdges...)
	}
	return out
}

// AlgorithmData is LayoutAlgorithmData: a layout algorithm that elkjs
// registers. It is the value of the property resolvedAlgorithm.
type AlgorithmData struct {
	ID string
}

// AlgorithmID returns the id of the algorithm.
func (a *AlgorithmData) AlgorithmID() string { return a.ID }

// LayeredAlgorithmID is the id of the layered algorithm.
const LayeredAlgorithmID = "org.eclipse.elk.layered"

// algorithms has the algorithms that elkjs 0.9.3 registers.
var algorithms = []*AlgorithmData{
	{"org.eclipse.elk.box"}, {"org.eclipse.elk.disco"}, {"org.eclipse.elk.fixed"}, {"org.eclipse.elk.force"},
	{LayeredAlgorithmID}, {"org.eclipse.elk.mrtree"}, {"org.eclipse.elk.radial"}, {"org.eclipse.elk.random"},
	{"org.eclipse.elk.rectpacking"}, {"org.eclipse.elk.sporeCompaction"}, {"org.eclipse.elk.sporeOverlap"},
	{"org.eclipse.elk.stress"},
}

// AlgorithmBySuffix is LayoutMetaDataService.getAlgorithmDataBySuffix: the
// algorithm whose id is the suffix or ends with a dot and the suffix. It
// returns nil when no algorithm or more than one matches.
func AlgorithmBySuffix(suffix string) *AlgorithmData {
	if suffix == "" {
		return nil
	}
	var data *AlgorithmData
	for _, d := range algorithms {
		id := d.ID
		if len(id) >= len(suffix) && id[len(id)-len(suffix):] == suffix && (len(suffix) == len(id) || id[len(id)-len(suffix)-1] == '.') {
			if data != nil {
				return nil
			}
			data = d
		}
	}
	return data
}
