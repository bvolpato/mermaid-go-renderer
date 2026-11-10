// Tests mermaid 12.2.0, mermaidAPI.ts (renderDiagram), with
// @mermaid-js/mermaid-zenuml 1.0.1, src/zenumlRenderer.ts (MIT).

package zenuml_test

import (
	"strings"
	"testing"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
)

func TestZenUMLLiteralEntities(t *testing.T) {
	svg, err := mermaid.Render("zenuml\n// literal #60;tag#62;\nA.Call()", mermaid.WithPage(mermaid.PageMermaidCLI))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := dom.ParseBody(svg)
	if err != nil {
		t.Fatal(err)
	}
	comment := doc.Body.Select(".comment-text")
	if comment == nil {
		t.Fatal("the comment was not rendered")
	}
	if got := strings.TrimSpace(comment.TextContent()); got != "literal #60;tag#62;" {
		t.Fatalf("comment = %q, want literal #60;tag#62;", got)
	}
}
