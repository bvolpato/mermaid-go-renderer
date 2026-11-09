package p1cycles

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestDepthFirstCycleBreaker(t *testing.T) {
	nativetest.RunProcessor(t, "DepthFirstCycleBreaker", func() lgraph.Processor { return NewDepthFirstCycleBreaker() })
}

func TestGreedyCycleBreaker(t *testing.T) {
	nativetest.RunProcessor(t, "GreedyCycleBreaker", func() lgraph.Processor { return NewGreedyCycleBreaker() })
}

func TestGreedyModelOrderCycleBreaker(t *testing.T) {
	nativetest.RunProcessor(t, "GreedyModelOrderCycleBreaker", func() lgraph.Processor { return NewGreedyModelOrderCycleBreaker() })
}

func TestModelOrderCycleBreaker(t *testing.T) {
	nativetest.RunProcessor(t, "ModelOrderCycleBreaker", func() lgraph.Processor { return NewModelOrderCycleBreaker() })
}
