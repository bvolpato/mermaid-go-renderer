// This file ports no upstream file. It has the typed form of the graph that
// elkjs returns. The field names follow the ELK JSON format
// (https://eclipse.dev/elk/documentation/tooldevelopers/graphdatastructure/jsonformat.html).

package elk

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ID is the id of a graph element. ELK accepts a string or an integer. An
// integer id has its decimal form here.
type ID string

// UnmarshalJSON reads a JSON string or a JSON number.
func (id *ID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*id = ID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("elk: id is not a string or a number: %s", data)
	}
	*id = ID(n.String())
	return nil
}

// Point is a position. Its coordinates are relative to the node that
// contains the edge (Edge.Container), unless the graph sets
// elk.json.edgeCoords.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Label is a label of a node, a port or an edge. X and Y are relative to the
// element for a node or port label, and to the container of the edge for an
// edge label. elkjs writes them for each label. They are 0 when the
// algorithm did not place the label.
type Label struct {
	ID     ID      `json:"id"`
	Text   string  `json:"text"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Port is a port of a node. X and Y are relative to the node.
type Port struct {
	ID     ID       `json:"id"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Width  float64  `json:"width"`
	Height float64  `json:"height"`
	Labels []*Label `json:"labels"`
}

// Section is one piece of the route of an edge. mermaid edges have one
// section. A hyperedge has more.
type Section struct {
	ID            ID      `json:"id"`
	StartPoint    Point   `json:"startPoint"`
	EndPoint      Point   `json:"endPoint"`
	BendPoints    []Point `json:"bendPoints"`
	IncomingShape ID      `json:"incomingShape"`
	OutgoingShape ID      `json:"outgoingShape"`
	// IncomingSections and OutgoingSections connect the sections of a
	// hyperedge.
	IncomingSections []ID `json:"incomingSections"`
	OutgoingSections []ID `json:"outgoingSections"`
}

// Edge is an edge with its route. Sections is empty when the algorithm does
// not route edges (elk.box, elk.rectpacking) or when it skipped the edge.
type Edge struct {
	ID       ID         `json:"id"`
	Sources  []ID       `json:"sources"`
	Targets  []ID       `json:"targets"`
	Sections []*Section `json:"sections"`
	Labels   []*Label   `json:"labels"`
	// Container is the id of the node whose coordinate system the points of
	// the edge use.
	Container      ID      `json:"container"`
	JunctionPoints []Point `json:"junctionPoints"`
}

// Node is a node of the laid-out graph. The root of the graph is a Node too.
// X and Y are relative to the parent node.
type Node struct {
	ID       ID       `json:"id"`
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	Width    float64  `json:"width"`
	Height   float64  `json:"height"`
	Children []*Node  `json:"children"`
	Edges    []*Edge  `json:"edges"`
	Labels   []*Label `json:"labels"`
	Ports    []*Port  `json:"ports"`
}

// Decode reads the JSON that Layout returns. Members that ELK does not
// define, for example the members that mermaid copies from its own nodes,
// are ignored.
func Decode(layoutJSON []byte) (*Node, error) {
	var root Node
	if err := json.Unmarshal(layoutJSON, &root); err != nil {
		return nil, fmt.Errorf("elk: decode the layout: %w", err)
	}
	return &root, nil
}
