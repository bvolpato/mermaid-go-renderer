// Ported from the method getIndividualOrInherited(NodeAdapter, IProperty) of
// plugins/org.eclipse.elk.core/src/org/eclipse/elk/core/util/IndividualSpacings.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package internal

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/coreopts"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// getIndividualOrInherited is IndividualSpacings.getIndividualOrInherited(
// node, property): the value of the individual spacings of the node, else
// the value of the graph of the node, else the default value of the
// property.
//
// The individual spacings of a node (the option spacing.individual) are not
// ported, as in lgraph.IndividualOrDefault. A node that has them stops the
// layout.
//
// TODO(core): move to the package that ports org.eclipse.elk.core.util.
func getIndividualOrInherited[T any](node adapters.NodeAdapter, property *prop.Property[T]) T {
	if adapters.HasProperty(node, coreopts.SpacingIndividual) {
		lgraph.Bail("IndividualSpacings: individual spacings are not ported")
	}

	// use the common value
	if graph := node.Graph(); graph != nil {
		if !prop.IsNull(graph.Holder(), property) {
			return adapters.GetProperty(graph, property)
		}
	}

	// if the result is still null, we need the property's default value
	result, _ := property.Default()
	return result
}
