package p5edges

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestOrthogonalEdgeRouter(t *testing.T) {
	nativetest.RunProcessor(t, "OrthogonalEdgeRouter", func() lgraph.Processor { return NewOrthogonalEdgeRouter() })
}
