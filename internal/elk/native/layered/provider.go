// Ported from plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/LayeredLayoutProvider.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package layered

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/elkgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// Provider is LayeredLayoutProvider: it lays out an ELK node with the
// layered algorithm. ELK keeps one provider and uses it for each node of a
// graph that needs its own run, so one Provider serves one layout.
type Provider struct {
	elkLayered *ElkLayered
	// Imported, when set, is called after the import of each run with the
	// ELK node and its LGraph. Final is called before the layout goes back
	// to the ELK graph. The tests use them.
	Imported func(elkGraph *elkgraph.Node, graph *lgraph.LGraph)
	Final    func(elkGraph *elkgraph.Node, graph *lgraph.LGraph)
}

// NewProvider returns a provider for one layout.
func NewProvider(ctx *lgraph.Context) *Provider {
	return &Provider{elkLayered: NewElkLayered(ctx)}
}

// ElkLayered returns the algorithm of the provider.
func (p *Provider) ElkLayered() *ElkLayered { return p.elkLayered }

// Layout is layout(elkgraph, progressMonitor).
func (p *Provider) Layout(elkGraph *elkgraph.Node) {
	// Import the graph (layeredGraph won't be null since the
	// KGraphImporter always returns an LGraph instance, even though the
	// IGraphImporter interface would allow null as a return value)
	layeredGraph := ImportGraph(elkGraph)
	if p.Imported != nil {
		p.Imported(elkGraph, layeredGraph)
	}

	// Check if hierarchy handling for a compound graph is requested
	if prop.Get(elkGraph, lopts.HierarchyHandling) == core.HierarchyHandlingIncludeChildren {
		// Layout for all hierarchy levels is requested
		p.elkLayered.DoCompoundLayout(layeredGraph)
	} else {
		// Only the top-level graph is processed
		p.elkLayered.DoLayout(layeredGraph)
	}
	if p.Final != nil {
		p.Final(elkGraph, layeredGraph)
	}

	// Apply the layout results to the original graph
	ApplyLayout(layeredGraph)
}
