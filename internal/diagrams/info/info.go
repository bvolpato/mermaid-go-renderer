// Package info is the info diagram: it shows the mermaid version.
//
// Ported from mermaid 12.2.0, diagrams/info/infoRenderer.ts, infoDb.ts,
// infoParser.ts and the grammar parser/src/language/info/info.langium (MIT).
package info

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

func init() {
	render.Register(&render.Definition{
		ID:    "info",
		New:   func(*render.Context) render.DB { return &db{} },
		Parse: parseDiagram,
		Draw:  draw,
	})
}

type db struct{}

// Info is the AST of the entry rule Info.
type Info struct {
	langium.Common
}

var lexer = langium.MustLexer("info")

// parseDiagram is parser.parse of infoParser.ts: it parses the text and
// keeps nothing of the AST.
func parseDiagram(_ *render.Context, _ render.DB, text string) error {
	_, err := parse(text)
	return err
}

// parse is parse('info', text) of the parser package. The grammar is:
//
//	entry Info:
//	  NEWLINE*
//	  "info" NEWLINE*
//	  ("showInfo" NEWLINE*)?
//	  TitleAndAccessibilities?
//	;
func parse(text string) (*Info, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Info{}
	p.Skip("NEWLINE")
	if _, err := p.Expect("info"); err != nil {
		return nil, err
	}
	p.Skip("NEWLINE")
	if p.Accept("showInfo") != nil {
		p.Skip("NEWLINE")
	}
	if p.AtTitleAndAccessibilities() {
		if err := p.TitleAndAccessibilities(&ast.Common); err != nil {
			return nil, err
		}
	}
	return ast, p.End()
}

func draw(ctx *render.Context, _ render.DB, _ string) error {
	svg := ctx.SelectSvgElement()
	render.ConfigureSvgSize(svg, 100, 400, true)
	group := svg.Append("g")
	group.Append("text").
		Attr("x", 100).
		Attr("y", 40).
		Attr("class", "version").
		Attr("font-size", 32).
		Style("text-anchor", "middle").
		Text("v" + render.Version)
	return nil
}
