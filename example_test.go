package mermaid_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
)

func ExampleRender() {
	svg, err := mermaid.Render("flowchart LR\n  A[Start] --> B{Done?}\n  B -->|yes| C[Stop]")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(strings.HasPrefix(svg, `<svg id="my-svg"`), strings.Contains(svg, "Start"))
	// Output: true true
}

func ExampleRender_config() {
	// The site config is the argument of mermaid.initialize.
	svg, err := mermaid.Render("pie\n  \"Dogs\" : 3\n  \"Cats\" : 1",
		mermaid.WithConfigJSON([]byte(`{"theme": "dark"}`)),
		mermaid.WithID("pets"),
	)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(strings.HasPrefix(svg, `<svg id="pets"`))
	// Output: true
}

func ExampleRenderContext() {
	// A deadline bounds the time of a render of input that is not trusted.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := mermaid.RenderContext(ctx, "sequenceDiagram\n  Alice->>Bob: Hello")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(res.Type)
	// Output: sequence
}

func ExampleDetectType() {
	typ, err := mermaid.DetectType("stateDiagram-v2\n  [*] --> Idle")
	fmt.Println(typ, err)
	// Output: stateDiagram <nil>
}
