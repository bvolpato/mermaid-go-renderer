package block

import (
	"sort"
	"testing"

	blockgrammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/block"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// TestGrammarCoverage checks that the cases of the golden file run every
// semantic action of the grammar that a text can reach.
//
// The productions 1 to 6 and 8 to 11 (spaceLines, separator, stop) are not
// part of a rule that the start rule uses. Production 36 (classDef DEFAULT
// CLASSDEF_STYLEOPTS) needs the token DEFAULT after classDef, and the lexer
// gives the tokens CLASSDEF_ID or DEFAULT_CLASSDEF_ID there.
func TestGrammarCoverage(t *testing.T) {
	def, ok := render.Lookup("block")
	if !ok {
		t.Fatal("the block diagram is not registered")
	}
	hits := map[int]int{}
	wrapped := *def
	wrapped.Parse = func(_ *render.Context, db render.DB, text string) error {
		_, err := blockgrammar.Parse(text, db, func(yy any, s *jison.ParserState, production int) (any, error) {
			hits[production]++
			return performAction(yy, s, production)
		})
		return err
	}
	render.Register(&wrapped)
	defer render.Register(def)

	g := loadGolden(t)
	cases, _ := g.Get("cases").([]any)
	for _, item := range cases {
		c := js.AsObj(item)
		// The result of the parse is the subject of TestDBGolden.
		_, _ = parseText(caseText(t, c), siteConfig(c), false, 0)
	}
	want := []int{7, 12, 13, 14, 15, 18, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 37, 38}
	for _, production := range want {
		if hits[production] == 0 {
			t.Errorf("no case runs the action of production %d", production)
		}
	}
	var others []int
	for production := range hits {
		if i := sort.SearchInts(want, production); i == len(want) || want[i] != production {
			others = append(others, production)
		}
	}
	sort.Ints(others)
	// Productions without an action are reduced too.
	t.Logf("%d productions with an action are covered; other reduced productions: %v", len(want), others)
}
