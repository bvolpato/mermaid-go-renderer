package native

// ArchitectureConfig has the values of db.getConfigField that
// layoutArchitecture of mermaid reads.
type ArchitectureConfig struct {
	IconSize                  float64 `json:"iconSize"`
	FontSize                  float64 `json:"fontSize"`
	Padding                   float64 `json:"padding"`
	IdealEdgeLengthMultiplier float64 `json:"idealEdgeLengthMultiplier"`
	EdgeElasticity            float64 `json:"edgeElasticity"`
	NodeSeparation            float64 `json:"nodeSeparation"`
	NumIter                   float64 `json:"numIter"`
	Randomize                 bool    `json:"randomize"`
	// Seed is the seed of withSeededRandom. With 0 the layout draws from
	// Env.Random.
	Seed float64 `json:"seed"`
}

// ArchitectureNode is a service, a junction or a group of the architecture
// db. An empty string stands for a property that is undefined.
type ArchitectureNode struct {
	ID    string `json:"id"`
	Icon  string `json:"icon,omitempty"`
	Title string `json:"title,omitempty"`
	// In is the id of the group that contains the node.
	In string `json:"in,omitempty"`
}

// ArchitectureEdge is an edge of the architecture db.
type ArchitectureEdge struct {
	LhsID string `json:"lhsId"`
	RhsID string `json:"rhsId"`
	// LhsDir and RhsDir are "L", "R", "T" or "B".
	LhsDir string `json:"lhsDir"`
	RhsDir string `json:"rhsDir"`
	Title  string `json:"title,omitempty"`
}

// AlignmentConstraint is the alignmentConstraint option of fcose.
type AlignmentConstraint struct {
	Horizontal [][]string `json:"horizontal"`
	Vertical   [][]string `json:"vertical"`
}

// RelativePlacement is one entry of the relativePlacementConstraint option
// of fcose: either Left and Right are set, or Top and Bottom.
type RelativePlacement struct {
	Left   string  `json:"left,omitempty"`
	Right  string  `json:"right,omitempty"`
	Top    string  `json:"top,omitempty"`
	Bottom string  `json:"bottom,omitempty"`
	Gap    float64 `json:"gap"`
}

// ArchitectureInput is the input of layoutArchitecture.
type ArchitectureInput struct {
	Config            ArchitectureConfig  `json:"config"`
	Groups            []ArchitectureNode  `json:"groups"`
	Services          []ArchitectureNode  `json:"services"`
	Junctions         []ArchitectureNode  `json:"junctions"`
	Edges             []ArchitectureEdge  `json:"edges"`
	Alignment         AlignmentConstraint `json:"alignment"`
	RelativePlacement []RelativePlacement `json:"relativePlacement"`
}

// Point is a position.
type Point struct {
	X, Y float64
}

// Box is the result of boundingBox() of an element.
type Box struct {
	X1, Y1, X2, Y2, W, H float64
}

// ArchitectureNodeResult is what mermaid reads from a node after the layout.
type ArchitectureNodeResult struct {
	ID string
	// Type is "group", "service" or "junction".
	Type string
	// Index is the index of the node in Groups, Services or Junctions.
	Index int
	// BoundingBox is node.boundingBox(), for a group.
	BoundingBox Box
	// X and Y are node.position(), for a service and a junction. They are
	// NaN for a group.
	X, Y float64
}

// ArchitectureEdgeResult is what drawEdges of mermaid reads from an edge.
type ArchitectureEdgeResult struct {
	// ID is the id in cytoscape: LhsID + "-" + RhsID.
	ID string
	// Index is the index of the edge in ArchitectureInput.Edges.
	Index          int
	SourceEndpoint Point
	Midpoint       Point
	TargetEndpoint Point
	HasRScratch    bool
}

// ArchitectureResult has the nodes and the edges in the order of cy.nodes()
// and cy.edges().
type ArchitectureResult struct {
	Nodes []ArchitectureNodeResult
	Edges []ArchitectureEdgeResult
}
