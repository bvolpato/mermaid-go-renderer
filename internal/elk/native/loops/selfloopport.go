// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/intermediate/loops/SelfLoopPort.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package loops

import "github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"

// SelfLoopPort is the class SelfLoopPort: a port that is an end of one self
// loop or more.
type SelfLoopPort struct {
	// LPort is the port of this instance (getLPort()).
	LPort *lgraph.LPort
	// HadOnlySelfLoops tells that each edge of the port was a self loop when
	// the instance was made (hadOnlySelfLoops()).
	HadOnlySelfLoops bool
	// IncomingSLEdges has the incoming self loops (getIncomingSLEdges()).
	IncomingSLEdges []*SelfLoopEdge
	// OutgoingSLEdges has the outgoing self loops (getOutgoingSLEdges()).
	OutgoingSLEdges []*SelfLoopEdge
	// Hidden is the field isHidden: the port is removed from its node. Set
	// it with SetHidden.
	Hidden bool
	// HiddenAssigned tells that SetHidden was called. The object of elkjs
	// has its own field isHidden only from the first assignment, and the
	// stored states of the phase goldens show that field only then.
	HiddenAssigned bool
}

// NewSelfLoopPort is new SelfLoopPort(lPort).
func NewSelfLoopPort(lPort *lgraph.LPort) *SelfLoopPort {
	p := &SelfLoopPort{LPort: lPort}

	// Find out whether each edge of the port is a self loop.
	p.HadOnlySelfLoops = true
	for _, edge := range lPort.ConnectedEdges() {
		if !edge.IsSelfLoop() {
			p.HadOnlySelfLoops = false
			break
		}
	}
	return p
}

// IsHidden is isHidden().
func (p *SelfLoopPort) IsHidden() bool { return p.Hidden }

// SetHidden is setHidden(hidden).
func (p *SelfLoopPort) SetHidden(hidden bool) {
	p.Hidden = hidden
	p.HiddenAssigned = true
}

// SLNetFlow is getSLNetFlow(): the number of self loops that enter the port
// minus the number of self loops that leave it.
func (p *SelfLoopPort) SLNetFlow() int {
	return len(p.IncomingSLEdges) - len(p.OutgoingSLEdges)
}
