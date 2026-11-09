// Ported from plugins/org.eclipse.elk.graph.json/src/org/eclipse/elk/graph/json/JsonImporter.xtend
// of Eclipse ELK 0.9.1 (EPL-2.0), with the JSON adapter of elkjs 0.9.3 as it
// appears in elk-worker.js.

package elkgraph

import (
	"math"
	"strconv"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Importer is JsonImporter: it builds the ELK graph of a JSON graph and
// writes the layout back into the JSON graph.
//
// The importer handles the inputs that elkjs lays out without an error and
// that have none of the rare features listed in Import. For each other input
// Import returns an error that is ErrUnsupported.
type Importer struct {
	nodes map[ID]*Node
	ports map[ID]*Port
	edges map[ID]*Edge
	// Root is the root node after Import.
	Root *Node
	// AllEdges has each edge in the order of the import.
	AllEdges []*Edge
}

// NodeCount returns the number of nodes of the imported graph, with the
// root.
func (im *Importer) NodeCount() int { return len(im.nodes) }

// stringVal is JsonAdapter.stringVal: the text of a string, a number or a
// boolean.
func stringVal(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case float64:
		return js.Str(x), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	}
	return "", false
}

// asID is JsonAdapter.asId: a string, or a number that is an integer.
func asID(v any) (ID, bool) {
	switch x := v.(type) {
	case string:
		return ID{Str: x}, true
	case float64:
		// The id is Double.intValue(), which clamps. A number outside the
		// range of int is left to elkjs.
		if math.Mod(x, 1) == 0 && math.Abs(x) < 2147483647 {
			return ID{IsInt: true, Int: int(x)}, true
		}
	}
	return ID{}, false
}

func objArray(v any, what string) ([]*js.Obj[any], error) {
	if v == nil {
		return nil, nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, unsupported(what + " is not an array")
	}
	out := make([]*js.Obj[any], len(arr))
	for i, e := range arr {
		o, ok := e.(*js.Obj[any])
		if !ok {
			return nil, unsupported(what + " has an element that is not an object")
		}
		out[i] = o
	}
	return out, nil
}

// Import is JsonImporter.transform: it builds the ELK graph of the JSON graph
// root.
//
// It returns ErrUnsupported for: an element without a valid id, two elements
// of one kind with the same id, the members individualSpacings and sections,
// an edge in the primitive format (source and target instead of
// sources and targets), a member with a value of an unexpected type, an edge
// whose ends do not exist, and an option value that this port does not
// parse.
func (im *Importer) Import(root *js.Obj[any]) (*Node, error) {
	im.nodes = map[ID]*Node{}
	im.ports = map[ID]*Port{}
	im.edges = map[ID]*Edge{}
	node, err := im.transformNode(root, nil)
	if err != nil {
		return nil, err
	}
	im.Root = node
	if err := im.transformEdges(node); err != nil {
		return nil, err
	}
	return node, nil
}

func (im *Importer) id(obj *js.Obj[any]) (ID, string, error) {
	raw, ok := obj.Lookup("id")
	if !ok {
		return ID{}, "", unsupported("an element has no id")
	}
	id, ok := asID(raw)
	if !ok {
		return ID{}, "", unsupported("an id is not a string or an integer")
	}
	s, _ := stringVal(raw)
	return id, s, nil
}

func (im *Importer) transformNode(obj *js.Obj[any], parent *Node) (*Node, error) {
	id, identifier, err := im.id(obj)
	if err != nil {
		return nil, err
	}
	if _, dup := im.nodes[id]; dup {
		return nil, unsupported("two nodes have the id " + id.String())
	}
	node := &Node{Identifier: identifier, Parent: parent, JSON: obj, id: id}
	if parent != nil {
		parent.Children = append(parent.Children, node)
	}
	im.nodes[id] = node
	if err := transformProperties(obj, node); err != nil {
		return nil, err
	}
	if obj.Has("individualSpacings") {
		return nil, unsupported("individualSpacings")
	}
	if err := shapeLayout(obj, &node.X, &node.Y, &node.Width, &node.Height); err != nil {
		return nil, err
	}
	ports, err := objArray(obj.Get("ports"), "ports")
	if err != nil {
		return nil, err
	}
	for _, pobj := range ports {
		if err := im.transformPort(pobj, node); err != nil {
			return nil, err
		}
	}
	if node.Labels, err = im.transformLabels(obj, node); err != nil {
		return nil, err
	}
	children, err := objArray(obj.Get("children"), "children")
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		if _, err := im.transformNode(child, node); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// transformPort is JsonImporter.transformPort.
func (im *Importer) transformPort(obj *js.Obj[any], parent *Node) error {
	id, identifier, err := im.id(obj)
	if err != nil {
		return err
	}
	if _, dup := im.ports[id]; dup {
		return unsupported("two ports have the id " + id.String())
	}
	port := &Port{Identifier: identifier, Parent: parent, JSON: obj, id: id}
	parent.Ports = append(parent.Ports, port)
	im.ports[id] = port
	if err := transformProperties(obj, port); err != nil {
		return err
	}
	if err := shapeLayout(obj, &port.X, &port.Y, &port.Width, &port.Height); err != nil {
		return err
	}
	port.Labels, err = im.transformLabels(obj, port)
	return err
}

// shapeLayout is JsonImporter.transformShapeLayout. A member that is missing
// gives 0. A member that is not a number is left to elkjs.
func shapeLayout(obj *js.Obj[any], x, y, w, h *float64) error {
	for _, f := range []struct {
		key string
		dst *float64
	}{{"x", x}, {"y", y}, {"width", w}, {"height", h}} {
		v, ok := obj.Lookup(f.key)
		if !ok {
			*f.dst = 0
			continue
		}
		n, isNum := v.(float64)
		if !isNum {
			return unsupported("the member " + f.key + " is not a number")
		}
		*f.dst = n
	}
	return nil
}

// transformProperties is JsonImporter.transformProperties: the members of
// layoutOptions, or of properties when there is no layoutOptions object.
func transformProperties(obj *js.Obj[any], holder prop.Holder) error {
	var opts *js.Obj[any]
	for _, key := range []string{"layoutOptions", "properties"} {
		v, ok := obj.Lookup(key)
		if !ok {
			continue
		}
		o, isObj := v.(*js.Obj[any])
		if !isObj {
			if v == any(js.Null) {
				continue
			}
			return unsupported("the member " + key + " is not an object")
		}
		opts = o
		break
	}
	if opts == nil {
		return nil
	}
	for _, key := range opts.Keys() {
		value, ok := stringVal(opts.Get(key))
		if !ok {
			return unsupported("the option " + key + " has a value that is not a string, a number or a boolean")
		}
		option := OptionBySuffix(key)
		if option == nil {
			continue
		}
		if option.Prop == nil {
			// An option of another algorithm, which no ported code reads.
			// The layered algorithm does not read it either.
			if unreadOption(option.Meta.ID) {
				continue
			}
			return unsupported("the option " + option.Meta.ID + " is not ported")
		}
		parsed, set, err := option.ParseValue(value)
		if err != nil {
			return err
		}
		if set {
			prop.SetAny(holder, option.Prop, parsed)
		}
	}
	return nil
}

// unreadOption tells that an option without a Go property belongs to an
// algorithm other than the layered one, so that its value changes nothing
// in a layered layout.
func unreadOption(id string) bool {
	for _, prefix := range []string{
		"org.eclipse.elk.stress.", "org.eclipse.elk.force.", "org.eclipse.elk.mrtree.", "org.eclipse.elk.radial.",
		"org.eclipse.elk.disco.", "org.eclipse.elk.rectpacking.", "org.eclipse.elk.box.", "org.eclipse.elk.polyomino.",
		"org.eclipse.elk.overlapRemoval.", "org.eclipse.elk.structure.", "org.eclipse.elk.processingOrder.",
		"org.eclipse.elk.compaction.", "org.eclipse.elk.underlyingLayoutAlgorithm", "org.eclipse.elk.sporeOverlap.",
		"org.eclipse.elk.sporeCompaction.",
	} {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (im *Importer) transformLabels(obj *js.Obj[any], parent any) ([]*Label, error) {
	labels, err := objArray(obj.Get("labels"), "labels")
	if err != nil {
		return nil, err
	}
	var out []*Label
	for _, lobj := range labels {
		label := &Label{Parent: parent, JSON: lobj}
		if text, ok := lobj.Lookup("text"); ok {
			s, isText := stringVal(text)
			if !isText {
				return nil, unsupported("a label text is not a string")
			}
			label.Text, label.HasText = s, true
		}
		if id, ok := lobj.Lookup("id"); ok {
			if _, isText := stringVal(id); !isText {
				return nil, unsupported("a label id is not a string")
			}
		}
		if err := transformProperties(lobj, label); err != nil {
			return nil, err
		}
		if err := shapeLayout(lobj, &label.X, &label.Y, &label.Width, &label.Height); err != nil {
			return nil, err
		}
		out = append(out, label)
	}
	return out, nil
}

func (im *Importer) shapeByID(v any) (Shape, error) {
	id, ok := asID(v)
	if !ok {
		return nil, unsupported("an edge end is not a string or an integer")
	}
	if n := im.nodes[id]; n != nil {
		return n, nil
	}
	if p := im.ports[id]; p != nil {
		return p, nil
	}
	return nil, unsupported("an edge names a shape that does not exist")
}

// transformEdges is JsonImporter.transformEdges: the edges of a node, then
// the edges of its children.
func (im *Importer) transformEdges(node *Node) error {
	edges, err := objArray(node.JSON.Get("edges"), "edges")
	if err != nil {
		return err
	}
	for _, eobj := range edges {
		// An edge with the member sources or targets has the extended
		// format. Its members source and target are not read.
		if !eobj.Has("sources") && !eobj.Has("targets") {
			return unsupported("the primitive edge format")
		}
		if eobj.Has("sections") {
			return unsupported("an edge with a layout in the input")
		}
		id, identifier, err := im.id(eobj)
		if err != nil {
			return err
		}
		if _, dup := im.edges[id]; dup {
			return unsupported("two edges have the id " + id.String())
		}
		edge := &Edge{Identifier: identifier, JSON: eobj, id: id}
		edge.SetContainingNode(node)
		im.edges[id] = edge
		im.AllEdges = append(im.AllEdges, edge)
		for _, end := range []struct {
			key string
			add func(Shape)
		}{{"sources", edge.AddSource}, {"targets", edge.AddTarget}} {
			v, ok := eobj.Lookup(end.key)
			if !ok {
				continue
			}
			arr, isArr := v.([]any)
			if !isArr {
				return unsupported("the member " + end.key + " of an edge is not an array")
			}
			for _, ref := range arr {
				shape, err := im.shapeByID(ref)
				if err != nil {
					return err
				}
				end.add(shape)
			}
		}
		if len(edge.Sources) == 0 || len(edge.Targets) == 0 {
			return unsupported("an edge without a source or a target")
		}
		if err := transformProperties(eobj, edge); err != nil {
			return err
		}
		if edge.Labels, err = im.transformLabels(eobj, edge); err != nil {
			return err
		}
		// ElkGraphUtil.updateContainment
		best := FindBestEdgeContainment(edge)
		if best == nil {
			return unsupported("an edge without a containing node")
		}
		edge.SetContainingNode(best)
	}
	for _, child := range node.Children {
		if err := im.transformEdges(child); err != nil {
			return err
		}
	}
	return nil
}

func point(x, y float64) *js.Obj[any] {
	p := js.NewObj[any]()
	p.Set("x", x)
	p.Set("y", y)
	return p
}

func transferShape(obj *js.Obj[any], x, y, w, h float64) {
	obj.Set("x", x)
	obj.Set("y", y)
	obj.Set("width", w)
	obj.Set("height", h)
}

// TransferLayout is JsonImporter.transferLayout: it writes the layout of
// each element of the ELK graph into its JSON object.
func (im *Importer) TransferLayout() {
	var walk func(n *Node)
	walk = func(n *Node) {
		transferShape(n.JSON, n.X, n.Y, n.Width, n.Height)
		for _, l := range n.Labels {
			transferShape(l.JSON, l.X, l.Y, l.Width, l.Height)
		}
		for _, port := range n.Ports {
			transferShape(port.JSON, port.X, port.Y, port.Width, port.Height)
			for _, l := range port.Labels {
				transferShape(l.JSON, l.X, l.Y, l.Width, l.Height)
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(im.Root)
	for _, e := range im.AllEdges {
		for _, l := range e.Labels {
			transferShape(l.JSON, l.X, l.Y, l.Width, l.Height)
		}
		if len(e.Sections) != 0 {
			sections := make([]any, len(e.Sections))
			for i, s := range e.Sections {
				sec := js.NewObj[any]()
				sec.Set("id", e.id.String()+"_s"+strconv.Itoa(i))
				sec.Set("startPoint", point(s.StartX, s.StartY))
				sec.Set("endPoint", point(s.EndX, s.EndY))
				if len(s.BendPoints) != 0 {
					bends := make([]any, len(s.BendPoints))
					for k, b := range s.BendPoints {
						bends[k] = point(b.X, b.Y)
					}
					sec.Set("bendPoints", bends)
				}
				if s.IncomingShape != nil {
					sec.Set("incomingShape", s.IncomingShape.shapeID().JSON())
				}
				if s.OutgoingShape != nil {
					sec.Set("outgoingShape", s.OutgoingShape.shapeID().JSON())
				}
				sections[i] = sec
			}
			e.JSON.Set("sections", sections)
		}
		if jps, ok := prop.Lookup(e, coreopts.JunctionPoints); ok && jps != nil && !jps.IsEmpty() {
			points := make([]any, jps.Len())
			for i, p := range jps.Points {
				points[i] = point(p.X, p.Y)
			}
			e.JSON.Set("junctionPoints", points)
		}
		e.JSON.Set("container", e.ContainingNode.Identifier)
	}
}
