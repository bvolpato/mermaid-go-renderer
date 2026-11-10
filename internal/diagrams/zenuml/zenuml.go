// Ported from @mermaid-js/mermaid-zenuml 1.0.1, src/zenumlRenderer.ts,
// src/parser.ts and src/zenuml-definition.ts (MIT).

package zenuml

import (
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:            "zenuml",
		New:           func(*render.Context) render.DB { return struct{}{} },
		Parse:         func(*render.Context, render.DB, string) error { return nil },
		Draw:          draw,
		DrawUnencoded: true,
	})
}

func draw(ctx *render.Context, _ render.DB, text string) error {
	code := strings.TrimPrefix(js.TrimStart(text), "zenuml")
	result, err := renderToSVG(ctx.Cancel, code, ctx.Engine.Fonts)
	if err != nil {
		return err
	}
	svg := ctx.SVG
	useMaxWidth := js.Path(ctx.Config, "sequence", "useMaxWidth")
	svg.RemoveAttribute("height")
	svg.Style("max-width", nil)
	if js.IsNullish(useMaxWidth) || js.Truthy(useMaxWidth) {
		svg.Attr("width", "100%")
		svg.Attr("style", "max-width: "+js.Str(result.width)+"px;")
	} else {
		svg.Attr("width", result.width)
		svg.Attr("height", result.height)
	}
	svg.Attr("viewBox", result.viewBox)
	svg.HTML(result.innerSVG)
	return nil
}
