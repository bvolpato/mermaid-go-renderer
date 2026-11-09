// Ported from mermaid 12.2.0, diagrams/sankey/sankeyDB.ts (MIT).

package sankey

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// Link is SankeyLink: a link between two nodes.
type Link struct {
	Source *Node
	Target *Node
	Value  float64
}

// Node is SankeyNode.
type Node struct {
	ID string
}

// GraphNode is a node of the result of GetGraph.
type GraphNode struct {
	ID string `json:"id"`
}

// GraphLink is a link of the result of GetGraph.
type GraphLink struct {
	Source string  `json:"source"`
	Target string  `json:"target"`
	Value  float64 `json:"value"`
}

// Graph is the result of GetGraph.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Links []GraphLink `json:"links"`
}

// DB is the db of the sankey diagram: the module state of sankeyDB.ts.
type DB struct {
	render.CommonDB

	// links holds the links in the order of the text.
	links []*Link
	// nodes holds the nodes in the order of their first use.
	nodes []*Node
	// nodesMap finds a node by its id.
	nodesMap map[string]*Node

	// sanitizeText is common.sanitizeText(text, getConfig()).
	sanitizeText func(text string) string
}

// NewDB returns an empty db. It sanitizes texts with the config of ctx at
// the time of the call, as sankeyDB.ts does with getConfig().
func NewDB(ctx *render.Context) *DB {
	sanitizer := sanitize.New()
	db := &DB{nodesMap: map[string]*Node{}}
	db.sanitizeText = func(text string) string {
		return sanitizer.SanitizeText(text, ctx.Config)
	}
	db.Sanitize = db.sanitizeText
	return db
}

// Clear is clear().
func (db *DB) Clear() {
	db.links = nil
	db.nodes = nil
	db.nodesMap = map[string]*Node{}
	db.ClearCommon()
}

// AddLink is addLink(source, target, value).
func (db *DB) AddLink(source, target *Node, value float64) {
	db.links = append(db.links, &Link{Source: source, Target: target, Value: value})
}

// FindOrCreateNode is findOrCreateNode(ID).
func (db *DB) FindOrCreateNode(id string) *Node {
	id = db.sanitizeText(id)

	node := db.nodesMap[id]
	if node == nil {
		node = &Node{ID: id}
		db.nodesMap[id] = node
		db.nodes = append(db.nodes, node)
	}
	return node
}

// GetNodes is getNodes().
func (db *DB) GetNodes() []*Node { return db.nodes }

// GetLinks is getLinks().
func (db *DB) GetLinks() []*Link { return db.links }

// GetGraph is getGraph().
func (db *DB) GetGraph() *Graph {
	graph := &Graph{Nodes: []GraphNode{}, Links: []GraphLink{}}
	for _, node := range db.nodes {
		graph.Nodes = append(graph.Nodes, GraphNode{ID: node.ID})
	}
	for _, link := range db.links {
		graph.Links = append(graph.Links, GraphLink{Source: link.Source.ID, Target: link.Target.ID, Value: link.Value})
	}
	return graph
}
