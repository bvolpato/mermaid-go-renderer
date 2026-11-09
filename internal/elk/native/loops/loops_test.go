package loops_test

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/loops/loopstest"
)

func TestSelfLoopPreProcessor(t *testing.T) {
	loopstest.RunProcessor(t, "SelfLoopPreProcessor", func() lgraph.Processor { return loops.NewSelfLoopPreProcessor() })
}

func TestSelfLoopPostProcessor(t *testing.T) {
	loopstest.RunProcessor(t, "SelfLoopPostProcessor", func() lgraph.Processor { return loops.NewSelfLoopPostProcessor() })
}
