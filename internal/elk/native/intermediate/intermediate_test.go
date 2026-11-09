package intermediate

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestLongEdgeSplitter(t *testing.T) {
	nativetest.RunProcessor(t, "LongEdgeSplitter", func() lgraph.Processor { return NewLongEdgeSplitter() })
}

// The graph transformer changes property values in place that elkjs shares
// between two holders. See restoreSharedValues.
func TestGraphTransformerToInputDirection(t *testing.T) {
	runProcessorCanonical(t, "GraphTransformer:TO_INPUT_DIRECTION", func() lgraph.Processor {
		return NewGraphTransformer(ModeToInputDirection)
	})
}

func TestGraphTransformerToInternalLTR(t *testing.T) {
	runProcessorCanonical(t, "GraphTransformer:TO_INTERNAL_LTR", func() lgraph.Processor {
		return NewGraphTransformer(ModeToInternalLTR)
	})
}

func TestReversedEdgeRestorer(t *testing.T) {
	nativetest.RunProcessor(t, "ReversedEdgeRestorer", func() lgraph.Processor { return NewReversedEdgeRestorer() })
}

func TestEdgeAndLayerConstraintEdgeReverser(t *testing.T) {
	nativetest.RunProcessor(t, "EdgeAndLayerConstraintEdgeReverser", func() lgraph.Processor {
		return NewEdgeAndLayerConstraintEdgeReverser()
	})
}

func TestLayerConstraintPreprocessor(t *testing.T) {
	nativetest.RunProcessor(t, "LayerConstraintPreprocessor", func() lgraph.Processor { return NewLayerConstraintPreprocessor() })
}

func TestLayerConstraintPostprocessor(t *testing.T) {
	nativetest.RunProcessor(t, "LayerConstraintPostprocessor", func() lgraph.Processor { return NewLayerConstraintPostprocessor() })
}

// The long edge joiner removes nodes and edges, so the numbers of the Java
// objects that the harness does not know change in three steps (splines and
// breaking points). See canonicalObjects.
func TestLongEdgeJoiner(t *testing.T) {
	runProcessorCanonical(t, "LongEdgeJoiner", func() lgraph.Processor { return NewLongEdgeJoiner() })
}

func TestPortSideProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "PortSideProcessor", func() lgraph.Processor { return NewPortSideProcessor() })
}

func TestPortListSorter(t *testing.T) {
	nativetest.RunProcessor(t, "PortListSorter", func() lgraph.Processor { return NewPortListSorter() })
}

func TestInLayerConstraintProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "InLayerConstraintProcessor", func() lgraph.Processor { return NewInLayerConstraintProcessor() })
}

func TestLayerSizeAndGraphHeightCalculator(t *testing.T) {
	nativetest.RunProcessor(t, "LayerSizeAndGraphHeightCalculator", func() lgraph.Processor {
		return NewLayerSizeAndGraphHeightCalculator()
	})
}

func TestNorthSouthPortPreprocessor(t *testing.T) {
	nativetest.RunProcessor(t, "NorthSouthPortPreprocessor", func() lgraph.Processor { return NewNorthSouthPortPreprocessor() })
}

// The postprocessor removes the dummy nodes, so the numbers of the Java
// objects that the harness does not know change in two steps (end labels).
// See canonicalObjects.
func TestNorthSouthPortPostprocessor(t *testing.T) {
	runProcessorCanonical(t, "NorthSouthPortPostprocessor", func() lgraph.Processor { return NewNorthSouthPortPostprocessor() })
}

func TestHyperedgeDummyMerger(t *testing.T) {
	nativetest.RunProcessor(t, "HyperedgeDummyMerger", func() lgraph.Processor { return NewHyperedgeDummyMerger() })
}
