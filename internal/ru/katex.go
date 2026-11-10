// Ported from mermaid 12.2.0, diagrams/common/common.ts (MIT).

package ru

import (
	"regexp"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/katex"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

var reKatexAnnotation = regexp.MustCompile(`<annotation.*</annotation>`)

// RenderKatexSanitized replaces the math expressions in a label and applies
// the sanitizer of the current render. Chromium supports native MathML.
func RenderKatexSanitized(ctx *render.Context, text string) (string, error) {
	if !sanitize.HasKatex(text) {
		return ctx.SanitizeText(text), nil
	}
	output := katex.OutputMathML
	if js.Truthy(ctx.Cfg("forceLegacyMathML")) {
		output = katex.OutputHTMLAndMathML
	}
	lines := sanitize.LineBreakRegex.Split(text, -1)
	for i, line := range lines {
		if sanitize.HasKatex(line) {
			lines[i] = `<div style="display: flex; align-items: center; justify-content: center; white-space: nowrap;">` + line + `</div>`
		} else {
			lines[i] = `<div>` + line + `</div>`
		}
	}
	var renderErr error
	markup := sanitize.KatexRegex.ReplaceAllStringFunc(strings.Join(lines, ""), func(match string) string {
		if renderErr != nil {
			return ""
		}
		expression := match[2 : len(match)-2]
		mathML, err := katex.RenderToStringContext(ctx.Cancel, expression, output)
		if err != nil {
			renderErr = err
			return ""
		}
		mathML = strings.ReplaceAll(mathML, "\n", " ")
		return reKatexAnnotation.ReplaceAllString(mathML, "")
	})
	if renderErr != nil {
		return "", renderErr
	}
	// The import, renderKatexUnsanitized and renderKatexSanitized are async.
	ctx.AwaitN(3)
	return ctx.SanitizeText(markup), nil
}

// CalculateMathMLDimensions measures the temporary HTML label that Mermaid
// attaches to the body. clientWidth and clientHeight are integer CSS pixels.
func CalculateMathMLDimensions(ctx *render.Context, text string) (dom.Rect, error) {
	markup, err := RenderKatexSanitized(ctx, text)
	if err != nil {
		return dom.Rect{}, err
	}
	div := ctx.Body.Append("div").HTML(markup).
		Attr("id", "katex-temp").
		Style("visibility", "hidden").
		Style("position", "absolute").
		Style("top", "0")
	defer div.Remove()
	dim := div.BoundingClientRect()
	ctx.Await()
	return dom.Rect{Width: js.Round(dim.Width), Height: js.Round(dim.Height)}, nil
}
