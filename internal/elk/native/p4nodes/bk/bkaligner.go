// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/p4nodes/bk/BKAligner.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package bk

import (
	"math"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/gwt"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
)

// BKAligner is the class BKAligner: it builds the blocks of a layout and
// places the nodes inside each block.
type BKAligner struct {
	// layeredGraph is the graph to process.
	layeredGraph *lgraph.LGraph
	// ni has the neighbors of each node and its index in the layer.
	ni *NeighborhoodInformation
}

// NewBKAligner is new BKAligner(layeredGraph, ni).
func NewBKAligner(layeredGraph *lgraph.LGraph, ni *NeighborhoodInformation) *BKAligner {
	return &BKAligner{layeredGraph: layeredGraph, ni: ni}
}

// verticalAlignment is verticalAlignment(bal, markedEdges): it walks the
// graph in the directions of the layout and groups the nodes into blocks. An
// edge between two nodes of one block becomes a straight line.
//
// markedEdges has the edges of type 1 conflicts. Such an edge does not join
// two nodes into one block, so that a long edge stays straight.
func (a *BKAligner) verticalAlignment(bal *BKAlignedLayout, markedEdges map[*lgraph.LEdge]struct{}) {
	// Initialize root and align maps
	for _, layer := range a.layeredGraph.Layers {
		for _, v := range layer.Nodes {
			bal.root[v.ID] = v
			bal.align[v.ID] = v
			bal.innerShift[v.ID] = 0.0
		}
	}

	layers := a.layeredGraph.Layers

	// If the horizontal direction is LEFT, the layers are traversed from
	// right to left
	layerCount := len(layers)
	for li := 0; li < layerCount; li++ {
		layer := layers[li]
		if bal.hdir == HDirectionLeft {
			layer = layers[layerCount-1-li]
		}
		// r denotes the position in layer order where the last block was found
		// It is initialized with -1, since nothing is found and the ordering starts with 0
		r := -1
		nodes := layer.Nodes

		if bal.vdir == VDirectionUp {
			// If the alignment direction is UP, the nodes in a layer are traversed
			// reversely, thus we start at INT_MAX and with the reversed list of nodes.
			r = math.MaxInt32
		}

		// Variable names here are again taken from the paper mentioned above.
		// i denotes the index of the layer and k the position of the node within the layer.
		// m denotes the position of a neighbor in the neighbor list of a node.
		nodeCount := len(nodes)
		for ni := 0; ni < nodeCount; ni++ {
			vik := nodes[ni]
			if bal.vdir == VDirectionUp {
				vik = nodes[nodeCount-1-ni]
			}
			var neighbors []neighbor
			if bal.hdir == HDirectionLeft {
				neighbors = a.ni.rightNeighbors[vik.ID]
			} else {
				neighbors = a.ni.leftNeighbors[vik.ID]
			}

			if len(neighbors) > 0 {

				// When a node has many upper neighbors, consider only the (two) nodes in the
				// middle.
				d := len(neighbors)
				low := gwt.RoundInt(math.Floor((float64(d)+1.0)/2.0)) - 1
				high := gwt.RoundInt(math.Ceil((float64(d)+1.0)/2.0)) - 1

				if bal.vdir == VDirectionUp {
					// Check, whether v_i_k can be added to a block of its upper/lower neighbor(s)
					for m := high; m >= low; m-- {
						if bal.align[vik.ID] == vik {
							umPair := neighbors[m]
							um := umPair.first

							// Again, getEdge won't return null because the neighbor relationship
							// ensures that at least one edge exists
							if _, marked := markedEdges[umPair.second]; !marked && r > a.ni.nodeIndex[um.ID] {
								bal.align[um.ID] = vik
								bal.root[vik.ID] = bal.root[um.ID]
								bal.align[vik.ID] = bal.root[vik.ID]
								bal.od[bal.root[vik.ID].ID] = bal.od[bal.root[vik.ID].ID] && vik.Type == lgraph.NodeTypeLongEdge

								r = a.ni.nodeIndex[um.ID]
							}
						}
					}
				} else {
					// Check, whether vik can be added to a block of its upper/lower neighbor(s)
					for m := low; m <= high; m++ {
						if bal.align[vik.ID] == vik {
							umPair := neighbors[m]
							um := umPair.first

							if _, marked := markedEdges[umPair.second]; !marked && r < a.ni.nodeIndex[um.ID] {
								bal.align[um.ID] = vik
								bal.root[vik.ID] = bal.root[um.ID]
								bal.align[vik.ID] = bal.root[vik.ID]
								bal.od[bal.root[vik.ID].ID] = bal.od[bal.root[vik.ID].ID] && vik.Type == lgraph.NodeTypeLongEdge

								r = a.ni.nodeIndex[um.ID]
							}
						}
					}
				}
			}
		}
	}
}

// insideBlockShift is insideBlockShift(bal): it moves the nodes inside each
// block so that each edge inside a block is a straight line, and it computes
// the size of each block. The original algorithm does not have this step. It
// adds the ports and the node sizes.
func (a *BKAligner) insideBlockShift(bal *BKAlignedLayout) {
	blocks := getBlocks(bal)
	for _, root := range blocks.Keys() {
		// For each block, we place the top left corner of the root node at coordinate (0,0). We
		// then calculate the space required above the top left corner (due to other nodes placed
		// above and to top margins of nodes, including the root node) and the space required below
		// the top left corner. The sum of both becomes the block size, and the y coordinate of each
		// node relative to the block's top border becomes the inner shift of that node.

		// Reserve space for the root node
		spaceAbove := root.Margin.Top
		spaceBelow := root.Size.Y + root.Margin.Bottom
		bal.innerShift[root.ID] = 0.0

		// Iterate over all other nodes of the block
		current := root
		for {
			next := bal.align[current.ID]
			if next == root {
				break
			}
			// Find the edge between the current and the next node
			edge := getEdge(current, next)
			if edge == nil {
				// The Java code fails with a NullPointerException.
				lgraph.Bail("BKAligner: two nodes of a block have no edge between them")
			}

			// Calculate the y coordinate difference between the two nodes required to straighten
			// the edge
			var portPosDiff float64
			if bal.hdir == HDirectionLeft {
				portPosDiff = edge.Target.Pos.Y + edge.Target.Anchor.Y - edge.Source.Pos.Y - edge.Source.Anchor.Y
			} else {
				portPosDiff = edge.Source.Pos.Y + edge.Source.Anchor.Y - edge.Target.Pos.Y - edge.Target.Anchor.Y
			}

			// The current node already has an inner shift value that we need to use as the basis
			// to calculate the next node's inner shift
			nextInnerShift := bal.innerShift[current.ID] + portPosDiff
			bal.innerShift[next.ID] = nextInnerShift

			// Update the space required above and below the root node's top left corner
			spaceAbove = max(spaceAbove, next.Margin.Top-nextInnerShift)
			spaceBelow = max(spaceBelow, nextInnerShift+next.Size.Y+next.Margin.Bottom)

			// The next node is the current node in the next iteration
			current = next
		}

		// Adjust each node's inner shift by the space required above the root node's top left
		// corner (which the inner shifts are relative to at the moment)
		current = root
		for {
			bal.innerShift[current.ID] = bal.innerShift[current.ID] + spaceAbove
			current = bal.align[current.ID]
			if current == root {
				break
			}
		}

		// Remember the block size
		bal.blockSize[root.ID] = spaceAbove + spaceBelow
	}
}
