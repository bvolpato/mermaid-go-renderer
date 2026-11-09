package kanban

import (
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// TestKanbanItemAgainstMermaid replays the golden file of the node shape
// kanbanItem: cases recorded from the real insertNode of mermaid in
// Chromium. The cases come from tools/reference/gen-kanban-shape.mjs.
func TestKanbanItemAgainstMermaid(t *testing.T) {
	rutest.RunShapeGolden(t, "testdata/kanbanItem.json.gz")
}
