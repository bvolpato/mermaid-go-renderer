package nodesize

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

func TestLabelAndNodeSizeProcessor(t *testing.T) {
	nativetest.RunProcessor(t, "LabelAndNodeSizeProcessor", func() lgraph.Processor { return NewLabelAndNodeSizeProcessor() })
}

func TestInnermostNodeMarginCalculator(t *testing.T) {
	nativetest.RunProcessor(t, "InnermostNodeMarginCalculator", func() lgraph.Processor { return NewInnermostNodeMarginCalculator() })
}
