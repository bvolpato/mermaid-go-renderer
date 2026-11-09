// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/SelfLoopEdge.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"

// SelfLoopEdge is the class SelfLoopEdge: one self loop edge.
type SelfLoopEdge struct {
	// LEdge is the edge of this instance (getLEdge()).
	LEdge *lgraph.LEdge
	// SLHyperLoop is the hyper loop of the edge (getSLHyperLoop()). It is
	// nil until the holder puts the edge into a hyper loop.
	SLHyperLoop *SelfHyperLoop
	// SLSource is the source port (getSLSource()).
	SLSource *SelfLoopPort
	// SLTarget is the target port (getSLTarget()).
	SLTarget *SelfLoopPort
}

// NewSelfLoopEdge is new SelfLoopEdge(lEdge, slSource, slTarget). It adds
// the new edge to the edge lists of the two ports.
func NewSelfLoopEdge(lEdge *lgraph.LEdge, slSource, slTarget *SelfLoopPort) *SelfLoopEdge {
	e := &SelfLoopEdge{LEdge: lEdge, SLSource: slSource, SLTarget: slTarget}

	slSource.OutgoingSLEdges = append(slSource.OutgoingSLEdges, e)
	slTarget.IncomingSLEdges = append(slTarget.IncomingSLEdges, e)
	return e
}

// SetSLHyperLoop is setSLHyperLoop(slLoop).
func (e *SelfLoopEdge) SetSLHyperLoop(slLoop *SelfHyperLoop) { e.SLHyperLoop = slLoop }
