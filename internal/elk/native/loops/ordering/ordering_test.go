package ordering_test

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/loopstest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/ordering"
)

func TestSelfLoopPortRestorer(t *testing.T) {
	loopstest.RunProcessor(t, "SelfLoopPortRestorer", func() lgraph.Processor { return ordering.NewSelfLoopPortRestorer() })
}
