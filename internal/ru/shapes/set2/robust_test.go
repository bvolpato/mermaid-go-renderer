package set2

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/ru/rutest"
)

// hostileNodes are node properties that no diagram sets on purpose: sizes
// and paddings that are negative, zero or very large, and labels that are
// empty or long.
var hostileNodes = []map[string]any{
	{"width": -50, "height": -50},
	{"width": 0, "height": 0, "padding": 0, "label": ""},
	{"width": 1e-9, "height": 1e-9},
	{"width": 20000, "height": 3},
	{"width": 3, "height": 20000},
	{"padding": -100},
	{"padding": 5000},
	{"padding": 0, "label": "", "minWidth": 0},
	{"label": strings.Repeat("word ", 200)},
	{"label": strings.Repeat("x", 600), "wrappingWidth": 1},
	{"cssStyles": []string{"stroke-width:-3px", "stroke-dasharray:x y", "fill"}},
	{"cssStyles": []string{"stroke-width:1e9px", "stroke-dasharray:0"}},
}

// TestShapesDoNotPanic feeds hostile nodes to every shape of the package,
// in the three looks. A shape may return an error, as upstream throws for
// some of these nodes. It must not panic, and it must return.
func TestShapesDoNotPanic(t *testing.T) {
	files := map[string]bool{}
	for _, shape := range shapeFiles(t) {
		files[shape] = true
	}
	if len(files) != 25 {
		t.Fatalf("%d golden files, want one for each of the 25 shapes", len(files))
	}
	for shape := range files {
		// One case for each look gives the config and the style sheet.
		byLook := map[string]rutest.ShapeCase{}
		for _, c := range rutest.LoadShapeCases(t, "testdata/"+shape+".json.gz") {
			var node map[string]any
			if err := json.Unmarshal(c.Node, &node); err != nil {
				t.Fatal(err)
			}
			look, _ := node["look"].(string)
			if _, ok := byLook[look]; !ok && c.Error == "" {
				byLook[look] = c
			}
		}
		if len(byLook) != 3 {
			t.Fatalf("%s: cases for %d looks, want 3", shape, len(byLook))
		}
		for look, base := range byLook {
			for i, hostile := range hostileNodes {
				var node map[string]any
				if err := json.Unmarshal(base.Node, &node); err != nil {
					t.Fatal(err)
				}
				for k, v := range hostile {
					node[k] = v
				}
				c := base
				c.Node, _ = json.Marshal(node)
				done := make(chan rutest.ShapeResult, 1)
				go func() { done <- rutest.RunShapeCase(c) }()
				select {
				case r := <-done:
					if strings.HasPrefix(r.Detail, "panic:") {
						t.Errorf("%s, look %s, hostile node %d: %s", shape, look, i, r.Detail)
					}
				case <-time.After(60 * time.Second):
					t.Fatalf("%s, look %s, hostile node %d: no result after 60 s", shape, look, i)
				}
			}
		}
	}
}
